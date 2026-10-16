#!/usr/bin/env python3
"""Outdated and unmaintained dependencies of the server (Go) and the web apps (npm).

    scripts/antimatter/deps_report.py --out DIR [--skip-go] [--skip-npm] [--stale-years 2]

For the direct dependencies only:
  Go   `go list -m -u -json all` in server/ and server/public: newer version available, deprecated module
       (// Deprecated: in its go.mod), no release for more than --stale-years, and, for github.com modules,
       an archived repository (GitHub API; uses $GITHUB_TOKEN when set, unauthenticated otherwise).
  npm  the direct dependencies of every workspace in webapp/package.json, at the version in package-lock.json,
       against the npm registry: newer version available, installed version deprecated, no publish for more
       than --stale-years.

Writes DIR/dependencies.md (also appended to the GitHub job summary) and DIR/dependencies.json. Report only:
never fails on findings (exit 1 only when it couldn't run at all).
"""

import argparse
import concurrent.futures
import datetime
import json
import os
import re
import subprocess
import sys
import urllib.error
import urllib.parse
import urllib.request

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import amlib  # noqa: E402

NOW = datetime.datetime.now(datetime.timezone.utc)
NOTES = []


def parse_time(value):
    if not value:
        return None
    try:
        return datetime.datetime.fromisoformat(value.replace("Z", "+00:00"))
    except ValueError:
        return None


def age_years(when):
    return (NOW - when).days / 365.25 if when else None


def http_json(url, headers=None):
    req = urllib.request.Request(url, headers={"User-Agent": "antimatter-deps-report", **(headers or {})})
    with urllib.request.urlopen(req, timeout=60) as resp:
        return json.load(resp)


# ---------------------------------------------------------------------------------------------------- Go

def go_direct_modules(module_dir):
    """Direct requirements of the go.mod in module_dir."""
    direct = set()
    in_block = False
    for line in amlib.read_lines(os.path.join(module_dir, "go.mod")):
        s = line.strip()
        if s.startswith("require ("):
            in_block = True
            continue
        if in_block and s == ")":
            in_block = False
            continue
        m = re.match(r"^(?:require\s+)?(\S+)\s+v\S+(\s*//\s*indirect)?", s) if (in_block or s.startswith("require ")) else None
        if m and not m.group(2):
            direct.add(m.group(1))
    return direct


def go_list(module_dir):
    env = dict(os.environ, GOWORK="off", GOFLAGS="-mod=readonly")
    out = subprocess.run(["go", "list", "-m", "-u", "-json", "all"], cwd=module_dir, env=env,
                         capture_output=True, text=True, check=True).stdout
    decoder = json.JSONDecoder()
    mods, idx = [], 0
    while idx < len(out):
        while idx < len(out) and out[idx].isspace():
            idx += 1
        if idx >= len(out):
            break
        obj, idx = decoder.raw_decode(out, idx)
        mods.append(obj)
    return mods


def github_repo_state(path, token):
    m = re.match(r"^github\.com/([^/]+)/([^/]+)", path)
    if not m:
        return None
    headers = {"Accept": "application/vnd.github+json"}
    if token:
        headers["Authorization"] = f"Bearer {token}"
    try:
        data = http_json(f"https://api.github.com/repos/{m.group(1)}/{m.group(2)}", headers)
    except (urllib.error.URLError, TimeoutError, ValueError):
        return None
    return {"archived": bool(data.get("archived")), "pushed_at": data.get("pushed_at")}


def report_go(stale_years, token):
    rows = []
    for module_dir in ("server", "server/public"):
        full = os.path.join(amlib.ROOT, module_dir)
        direct = go_direct_modules(full)
        for mod in go_list(full):
            if mod.get("Main") or mod["Path"] not in direct:
                continue
            update = mod.get("Update") or {}
            latest_time = parse_time(update.get("Time") or mod.get("Time"))
            rows.append({
                "module": module_dir, "name": mod["Path"], "version": mod.get("Version", ""),
                "latest": update.get("Version", mod.get("Version", "")),
                "outdated": bool(update), "deprecated": mod.get("Deprecated", ""),
                "last_release": latest_time.date().isoformat() if latest_time else "",
                "stale": bool(latest_time and age_years(latest_time) > stale_years),
                "replaced": bool(mod.get("Replace")),
            })
    merged = {}
    for r in rows:  # a module required by both server and server/public is listed once
        key = (r["name"], r["version"])
        if key in merged:
            merged[key]["module"] += ", " + r["module"]
        else:
            merged[key] = r
    rows = list(merged.values())
    unique = sorted({r["name"] for r in rows if r["name"].startswith("github.com/")})
    with concurrent.futures.ThreadPoolExecutor(8) as pool:
        states = dict(zip(unique, pool.map(lambda p: github_repo_state(p, token), unique)))
    unknown = sum(1 for p in unique if states.get(p) is None)
    if unknown:
        NOTES.append(f"GitHub didn't answer for {unknown} of {len(unique)} Go repositories (rate limit without "
                     "GITHUB_TOKEN?): their archived state is unknown.")
    for r in rows:
        st = states.get(r["name"])
        r["archived"] = bool(st and st["archived"])
        r["last_push"] = (st or {}).get("pushed_at", "") or ""
        pushed = parse_time(r["last_push"])
        if r["stale"] and pushed and age_years(pushed) <= stale_years:
            r["stale"] = False  # no tagged release, but the repository is still active
    return rows


# ---------------------------------------------------------------------------------------------------- npm

def npm_direct_deps():
    lock = json.load(open(os.path.join(amlib.ROOT, "webapp", "package-lock.json"), encoding="utf-8"))
    packages = lock.get("packages", {})
    workspaces = {k for k in packages if k and "node_modules" not in k}
    local_names = {packages[w].get("name") for w in workspaces}
    deps = {}
    for ws in [""] + sorted(workspaces):
        manifest = json.load(open(os.path.join(amlib.ROOT, "webapp", ws, "package.json"), encoding="utf-8"))
        for kind in ("dependencies", "devDependencies"):
            for name in manifest.get(kind, {}):
                if name in local_names:
                    continue
                entry = packages.get(f"{ws}/node_modules/{name}" if ws else "") or {}
                if not entry.get("version"):
                    entry = packages.get(f"node_modules/{name}", {})
                version = entry.get("version")
                if not version or entry.get("link"):
                    continue
                d = deps.setdefault((name, version), {"name": name, "version": version, "used_by": set(),
                                                      "dev": True})
                d["used_by"].add(ws or "webapp")
                if kind == "dependencies":
                    d["dev"] = False
    return list(deps.values())


def npm_registry(name):
    url = "https://registry.npmjs.org/" + urllib.parse.quote(name, safe="@")
    try:
        return http_json(url, {"Accept": "application/vnd.npm.install-v1+json"})
    except (urllib.error.URLError, TimeoutError, ValueError):
        return None


def report_npm(stale_years):
    deps = npm_direct_deps()
    names = sorted({d["name"] for d in deps})
    with concurrent.futures.ThreadPoolExecutor(16) as pool:
        docs = dict(zip(names, pool.map(npm_registry, names)))
    rows = []
    for d in deps:
        doc = docs.get(d["name"]) or {}
        latest = (doc.get("dist-tags") or {}).get("latest", "")
        info = (doc.get("versions") or {}).get(d["version"], {})
        modified = parse_time(doc.get("modified"))
        rows.append({
            "name": d["name"], "version": d["version"], "latest": latest,
            "outdated": bool(latest and latest != d["version"]),
            "deprecated": info.get("deprecated", "") or ((doc.get("versions") or {}).get(latest, {}).get("deprecated", "")),
            "last_publish": modified.date().isoformat() if modified else "",
            "stale": bool(modified and age_years(modified) > stale_years),
            "dev": d["dev"], "used_by": sorted(d["used_by"]), "found": bool(doc),
        })
    return sorted(rows, key=lambda r: r["name"])


# ---------------------------------------------------------------------------------------------------- output

def section(title, rows, name_key, date_key, stale_years, date_label="Last release"):
    flagged = [r for r in rows if r["deprecated"] or r["stale"] or r.get("archived")]
    outdated = [r for r in rows if r["outdated"]]
    md = [f"### {title}", "",
          f"{len(rows)} direct dependencies: {len(outdated)} have a newer version, "
          f"{sum(1 for r in rows if r['deprecated'])} deprecated, "
          f"{sum(1 for r in rows if r.get('archived'))} archived, "
          f"{sum(1 for r in rows if r['stale'])} without a release for more than {stale_years:g} years.", ""]
    strong = [r for r in flagged if r["deprecated"] or r.get("archived")]
    weak = [r for r in flagged if not (r["deprecated"] or r.get("archived"))]
    for heading, group in (("**Deprecated or archived** (replace them):", strong),
                           (f"**No release for more than {stale_years:g} years** (check they're still maintained):",
                            weak)):
        if not group:
            continue
        md += [heading, "", f"| Dependency | Version | Latest | {date_label} | Why |", "| --- | --- | --- | --- | --- |"]
        for r in sorted(group, key=lambda r: r[name_key]):
            why = []
            if r["deprecated"]:
                why.append("deprecated: " + amlib.md_escape(str(r["deprecated"])[:120]))
            if r.get("archived"):
                why.append("repository archived")
            if r["stale"]:
                why.append(f"nothing published since {r[date_key]}")
            md.append(f"| `{r[name_key]}` | {r['version']} | {r['latest']} | {r[date_key]} | {'; '.join(why)} |")
        md.append("")
    if outdated:
        md += ["<details><summary>Newer versions available</summary>", "",
               "| Dependency | Version | Latest |", "| --- | --- | --- |"]
        md += [f"| `{r[name_key]}` | {r['version']} | {r['latest']} |" for r in sorted(outdated, key=lambda r: r[name_key])]
        md += ["", "</details>", ""]
    return md


def main():
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument("--out", required=True)
    parser.add_argument("--skip-go", action="store_true")
    parser.add_argument("--skip-npm", action="store_true")
    parser.add_argument("--stale-years", type=float, default=2)
    args = parser.parse_args()

    result, md = {}, ["## Dependencies: outdated and unmaintained", ""]
    errors = []
    if not args.skip_go:
        try:
            result["go"] = report_go(args.stale_years, os.environ.get("GITHUB_TOKEN"))
            md += section("Server (Go modules)", result["go"], "name", "last_release", args.stale_years)
        except (OSError, subprocess.CalledProcessError) as e:
            errors.append(f"Go: {e}")
    if not args.skip_npm:
        try:
            result["npm"] = report_npm(args.stale_years)
            md += section("Web apps (npm)", result["npm"], "name", "last_publish", args.stale_years,
                          "Last registry update")
        except OSError as e:
            errors.append(f"npm: {e}")
    for e in errors:
        md.append(f"**Could not check {e}**")
    md += [f"Note: {n}" for n in NOTES]
    text = "\n".join(md)
    amlib.write_text(os.path.join(args.out, "dependencies.md"), text + "\n")
    amlib.write_text(os.path.join(args.out, "dependencies.json"), json.dumps(result, indent=2, default=list) + "\n")
    amlib.append_summary(text)
    print(text)
    return 1 if errors and not result else 0


if __name__ == "__main__":
    sys.exit(main())
