#!/usr/bin/env python3
"""Summarises the dependency vulnerability scans into one Markdown report.

    scripts/antimatter/vuln_report.py --out DIR [--govulncheck LABEL=file.json ...] [--osv file.json]
                                      [--npm-audit LABEL=file.json ...] [--fail-on-called]
                                      [--allowlist vuln-allowlist.txt]

Inputs are the JSON outputs of `govulncheck -format json`, `osv-scanner scan source --format json` and
`npm audit --json`. Writes DIR/vulnerabilities.md (also appended to the GitHub job summary) and
DIR/vulnerabilities.json.

--fail-on-called exits 1 when govulncheck found a vulnerability in code the server actually calls (the
high-signal case; other findings are reported but don't fail the job). Vulnerabilities in the --allowlist
(scripts/antimatter/vuln-allowlist.txt: those without a fix, each with its reason) are reported, with the
reason, but don't fail it; an entry that no longer matches anything is reported as stale.
"""

import argparse
import collections
import json
import os
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import amlib  # noqa: E402

SEVERITY_ORDER = ["critical", "high", "moderate", "low", "info"]


def read_json_stream(path):
    """govulncheck writes a stream of JSON objects, not one document."""
    text = open(path, encoding="utf-8").read()
    decoder = json.JSONDecoder()
    idx, out = 0, []
    while idx < len(text):
        while idx < len(text) and text[idx].isspace():
            idx += 1
        if idx >= len(text):
            break
        obj, idx = decoder.raw_decode(text, idx)
        out.append(obj)
    return out


def govulncheck(path):
    osvs, findings = {}, collections.defaultdict(lambda: {"level": "module", "fixed": "", "module": "", "version": ""})
    for msg in read_json_stream(path):
        if "osv" in msg:
            osvs[msg["osv"]["id"]] = msg["osv"]
        f = msg.get("finding")
        if not f:
            continue
        entry = findings[f["osv"]]
        trace = f.get("trace") or [{}]
        frame = trace[0]
        level = "called" if frame.get("function") else "imported" if frame.get("package") else "module"
        rank = {"module": 0, "imported": 1, "called": 2}
        if rank[level] >= rank[entry["level"]]:
            entry["level"] = level
        entry["fixed"] = f.get("fixed_version", "") or entry["fixed"]
        entry["module"] = frame.get("module", entry["module"])
        entry["version"] = frame.get("version", entry["version"])
    rows = []
    for vid, e in findings.items():
        o = osvs.get(vid, {})
        rows.append({"id": vid, "level": e["level"], "module": e["module"], "version": e["version"],
                     "fixed": e["fixed"], "summary": o.get("summary") or (o.get("details") or "")[:120],
                     "aliases": o.get("aliases", [])})
    return sorted(rows, key=lambda r: ({"called": 0, "imported": 1, "module": 2}[r["level"]], r["id"]))


def read_allowlist(path):
    """Reads "<vulnerability ID> <reason>" lines; blank lines and # comments are skipped."""
    allowed = {}
    if not path:
        return allowed
    for raw in open(path, encoding="utf-8"):
        line = raw.strip()
        if not line or line.startswith("#"):
            continue
        vid, _, reason = line.partition(" ")
        if not reason.strip():
            raise SystemExit(f"{path}: {vid} has no reason; every allowlisted vulnerability needs one")
        allowed[vid] = reason.strip()
    return allowed


def osv_scanner(path):
    data = json.load(open(path, encoding="utf-8"))
    rows = []
    for result in data.get("results", []):
        source = os.path.relpath(result.get("source", {}).get("path", ""), amlib.ROOT)
        for pkg in result.get("packages", []):
            p = pkg.get("package", {})
            for group in pkg.get("groups", []):
                rows.append({"source": source, "package": p.get("name"), "version": p.get("version"),
                             "ecosystem": p.get("ecosystem"), "ids": group.get("ids", []),
                             "severity": group.get("max_severity", "")})
    return rows


def npm_audit(path):
    data = json.load(open(path, encoding="utf-8"))
    counts = (data.get("metadata") or {}).get("vulnerabilities", {})
    rows = []
    for name, v in (data.get("vulnerabilities") or {}).items():
        titles = [x.get("title", "") for x in v.get("via", []) if isinstance(x, dict)]
        rows.append({"package": name, "severity": v.get("severity", ""), "direct": v.get("isDirect", False),
                     "fix": bool(v.get("fixAvailable")), "titles": titles[:3]})
    rows.sort(key=lambda r: (SEVERITY_ORDER.index(r["severity"]) if r["severity"] in SEVERITY_ORDER else 9,
                             r["package"]))
    return counts, rows


def main():
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument("--out", required=True)
    parser.add_argument("--govulncheck", action="append", default=[])
    parser.add_argument("--osv")
    parser.add_argument("--npm-audit", action="append", default=[])
    parser.add_argument("--fail-on-called", action="store_true")
    parser.add_argument("--allowlist")
    args = parser.parse_args()

    allowed = read_allowlist(args.allowlist)
    md = ["## Dependency vulnerabilities", ""]
    result = {}
    called_total = 0
    allowed_seen = set()

    for spec in args.govulncheck:
        label, _, path = spec.partition("=")
        if not os.path.exists(path):
            md.append(f"**govulncheck ({label}) didn't produce a result.**\n")
            continue
        rows = govulncheck(path)
        for r in rows:
            if r["id"] in allowed:
                allowed_seen.add(r["id"])
                # Only advisories without a fix may be allowlisted: one with a fixed version is updated instead.
                if not r["fixed"]:
                    r["allowlisted"] = allowed[r["id"]]
        result[f"govulncheck:{label}"] = rows
        called = [r for r in rows if r["level"] == "called" and "allowlisted" not in r]
        called_allowed = sum(1 for r in rows if r["level"] == "called" and "allowlisted" in r)
        called_total += len(called)
        md += [f"### govulncheck: {label}", "",
               f"{len(called)} vulnerabilities in code the server calls"
               + (f" (and {called_allowed} allowlisted)" if called_allowed else "") + ", "
               f"{sum(1 for r in rows if r['level'] == 'imported')} in imported packages whose vulnerable code "
               f"isn't called, {sum(1 for r in rows if r['level'] == 'module')} in required modules only.", ""]
        if rows:
            md += ["| ID | Reach | Module | Version | Fixed in | Summary |", "| --- | --- | --- | --- | --- | --- |"]
            md += [f"| [{r['id']}](https://pkg.go.dev/vuln/{r['id']}) | {r['level']}"
                   f"{' (allowlisted)' if 'allowlisted' in r else ''} | `{r['module']}` | "
                   f"{r['version']} | {r['fixed'] or 'no fix'} | {amlib.md_escape(r['summary'][:120])} |"
                   for r in rows]
            md.append("")

    if args.osv and os.path.exists(args.osv):
        rows = osv_scanner(args.osv)
        result["osv-scanner"] = rows
        by_source = collections.Counter(r["source"] for r in rows)
        md += ["### osv-scanner (lockfiles)", "",
               f"{len(rows)} known vulnerabilities across "
               + (", ".join(f"`{s}`: {n}" for s, n in sorted(by_source.items())) or "the scanned lockfiles") + ".",
               ""]
        if rows:
            md += ["<details><summary>Details</summary>", "", "| Lockfile | Package | Version | IDs | CVSS |",
                   "| --- | --- | --- | --- | --- |"]
            md += [f"| `{r['source']}` | `{r['package']}` | {r['version']} | {', '.join(r['ids'][:3])} | "
                   f"{r['severity'] or '?'} |" for r in sorted(rows, key=lambda r: (r['source'], r['package']))]
            md += ["", "</details>", ""]

    for spec in args.npm_audit:
        label, _, path = spec.partition("=")
        if not os.path.exists(path):
            continue
        counts, rows = npm_audit(path)
        result[f"npm-audit:{label}"] = {"counts": counts, "packages": rows}
        md += [f"### npm audit: {label}", "",
               ", ".join(f"{counts.get(s, 0)} {s}" for s in SEVERITY_ORDER) + ".", ""]
        serious = [r for r in rows if r["severity"] in ("critical", "high")]
        if serious:
            md += ["| Package | Severity | Direct | Fix available | Advisory |", "| --- | --- | --- | --- | --- |"]
            md += [f"| `{r['package']}` | {r['severity']} | {'yes' if r['direct'] else 'no'} | "
                   f"{'yes' if r['fix'] else 'no'} | {amlib.md_escape('; '.join(r['titles']))[:150]} |"
                   for r in serious]
            md.append("")

    if allowed and args.govulncheck:
        md += ["### Allowlisted vulnerabilities", "",
               "Reported without failing the job (`scripts/antimatter/vuln-allowlist.txt`):", ""]
        md += [f"- [{vid}](https://pkg.go.dev/vuln/{vid}): {amlib.md_escape(reason)}"
               + ("" if vid in allowed_seen else " **Stale: govulncheck no longer reports it; remove the entry.**")
               + (" **It has a fix now: update the module (the entry is ignored).**"
                  if any(r["id"] == vid and r["fixed"] for rows in result.values() for r in rows) else "")
               for vid, reason in sorted(allowed.items())]
        md.append("")

    if args.fail_on_called and called_total:
        md.append(f"**{called_total} vulnerabilit{'y is' if called_total == 1 else 'ies are'} reachable from the "
                  "server's code**: update the module to the fixed version (`go get <module>@<version>` in "
                  "server/, then `make modules-tidy`).")
    text = "\n".join(md)
    amlib.write_text(os.path.join(args.out, "vulnerabilities.md"), text + "\n")
    amlib.write_text(os.path.join(args.out, "vulnerabilities.json"), json.dumps(result, indent=2) + "\n")
    amlib.append_summary(text)
    print(text)
    return 1 if args.fail_on_called and called_total else 0


if __name__ == "__main__":
    sys.exit(main())
