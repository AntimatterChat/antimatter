#!/usr/bin/env python3
"""Reads Go (gotestsum JSON) or jest (--json) results and decides whether the run failed.

    scripts/antimatter/test_results.py go   --label "Server shard 1" server/gotestsum.json
    scripts/antimatter/test_results.py jest --label "Fusion shard 1" webapp/fusion/jest-results.json

Every test runs: nothing is skipped. A failure listed in scripts/antimatter/known-failures.txt (a TEMPORARY
list of tests that already failed before CI was set up) is reported as a warning; any other failure, a
package that didn't build or a missing results file fails the job. A listed test that now passes is reported
so it can be removed from the list.
"""

import argparse
import collections
import fnmatch
import json
import os
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import amlib  # noqa: E402

KNOWN = os.path.join(os.path.dirname(os.path.abspath(__file__)), "known-failures.txt")


def load_known(kind):
    """Entries: 'go <package-suffix> <TestName>' or 'jest <path-glob> <test name substring>'."""
    entries = []
    for raw in amlib.read_lines(KNOWN):
        line = raw.split("  #", 1)[0].strip()
        if not line or line.startswith("#"):
            continue
        parts = line.split(None, 2)
        if len(parts) == 3 and parts[0] == kind:
            entries.append((parts[1], parts[2].strip()))
    return entries


def go_known(entries, package, test):
    for pkg, name in entries:
        if package == pkg or package.endswith("/" + pkg):
            if test == name or test.startswith(name + "/"):
                return True
    return False


def jest_known(entries, path, full_name):
    for glob, name in entries:
        if fnmatch.fnmatchcase(path, "*" + glob) and (name == "*" or name in full_name):
            return True
    return False


def parse_go(paths):
    """Returns (failed tests, broken packages, passed tests, counts) from test2json events."""
    last = {}
    pkg_last = {}
    output = collections.defaultdict(lambda: collections.deque(maxlen=80))
    for path in paths:
        with open(path, encoding="utf-8", errors="replace") as f:
            for line in f:
                try:
                    ev = json.loads(line)
                except ValueError:
                    continue
                action, pkg, test = ev.get("Action"), ev.get("Package", ""), ev.get("Test")
                if action == "output" and pkg:
                    output[(pkg, test or "")].append(ev.get("Output", ""))
                if action not in ("pass", "fail", "skip"):
                    continue
                if test:
                    last[(pkg, test)] = action
                else:
                    pkg_last[pkg] = action
    failed = sorted(k for k, v in last.items() if v == "fail")
    failed_pkgs = {p for p, _ in failed}
    broken = sorted(p for p, v in pkg_last.items() if v == "fail" and p not in failed_pkgs)
    passed = {k for k, v in last.items() if v == "pass"}
    counts = collections.Counter(last.values())
    return failed, broken, passed, counts, output


def parse_jest(paths):
    failed, broken, passed = [], [], set()
    counts = collections.Counter()
    messages = {}
    for path in paths:
        with open(path, encoding="utf-8") as f:
            data = json.load(f)
        for suite in data.get("testResults", []):
            name = os.path.relpath(suite.get("name", ""), amlib.ROOT)
            results = suite.get("assertionResults", [])
            for a in results:
                status = a.get("status")
                counts[status] += 1
                key = (name, a.get("fullName", ""))
                if status == "failed":
                    failed.append(key)
                    messages[key] = "\n".join(a.get("failureMessages", []))[:3000]
                elif status == "passed":
                    passed.add(key)
            if suite.get("status") == "failed" and not any(a.get("status") == "failed" for a in results):
                broken.append(name)
                messages[(name, "")] = (suite.get("message") or "")[:3000]
    return sorted(failed), sorted(broken), passed, counts, messages


def main():
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument("kind", choices=("go", "jest"))
    parser.add_argument("results", nargs="+")
    parser.add_argument("--label", default="Tests")
    parser.add_argument("--summary", help="also write the Markdown summary to this file")
    args = parser.parse_args()

    missing = [p for p in args.results if not os.path.exists(p)]
    present = [p for p in args.results if os.path.exists(p)]
    entries = load_known(args.kind)

    if args.kind == "go":
        failed, broken, passed, counts, details = parse_go(present)
        is_known = lambda k: go_known(entries, k[0], k[1])  # noqa: E731
        matches = lambda e, k: go_known([e], k[0], k[1])  # noqa: E731
        fmt = lambda k: f"{k[0].replace('github.com/mattermost/mattermost/server/', '')} {k[1]}"  # noqa: E731
    else:
        failed, broken, passed, counts, details = parse_jest(present)
        is_known = lambda k: jest_known(entries, k[0], k[1])  # noqa: E731
        matches = lambda e, k: jest_known([e], k[0], k[1])  # noqa: E731
        fmt = lambda k: f"{k[0]} > {k[1]}"  # noqa: E731

    known_failed = [k for k in failed if is_known(k)]
    # An entry is fixed when the tests it names ran in this job and none of them failed.
    known_passing = [f"{e[0]} {e[1]}" for e in entries
                     if any(matches(e, k) for k in passed) and not any(matches(e, k) for k in failed)]
    new_failed = [k for k in failed if not is_known(k)]
    total = sum(counts.values())

    ok = not new_failed and not broken and not missing and total > 0
    md = [f"### {args.label}: {'passed' if ok else 'FAILED'}", ""]
    md.append(f"{total} tests: {counts.get('pass', 0) + counts.get('passed', 0)} passed, "
              f"{len(failed)} failed ({len(known_failed)} known), "
              f"{counts.get('skip', 0) + counts.get('pending', 0) + counts.get('skipped', 0)} skipped.")
    md.append("")
    if missing:
        md.append(f"**Missing results file(s)**: {', '.join(missing)} (the run crashed or was killed).")
    if total == 0 and not missing:
        md.append("**No test results were recorded.**")
    if broken:
        md.append(f"**{len(broken)} package(s)/suite(s) failed outside any test** (build error, panic, timeout):")
        md.extend(f"- `{b}`" for b in broken[:50])
        md.append("")
    if new_failed:
        md.append(f"**{len(new_failed)} failing test(s):**")
        md.extend(f"- `{fmt(k)}`" for k in new_failed[:100])
        md.append("")
    if known_failed:
        md.append(f"{len(known_failed)} failure(s) are on the temporary known-failures list "
                  "(`scripts/antimatter/known-failures.txt`); they don't fail the job but still need fixing:")
        md.extend(f"- `{fmt(k)}`" for k in known_failed[:100])
        md.append("")
    if known_passing:
        md.append("These known-failures entries now pass: remove them from "
                  "`scripts/antimatter/known-failures.txt`:")
        md.extend(f"- `{k}`" for k in known_passing)
        md.append("")
    text = "\n".join(md)
    print(text)

    gha = os.environ.get("GITHUB_ACTIONS")
    for k in new_failed[:50]:
        detail = "".join(details.get(k, [])) if args.kind == "go" else details.get(k, "")
        print(f"\n----- {fmt(k)}\n{detail[-3000:]}")
        if gha:
            print(f"::error title={args.label}::{fmt(k)} failed")
    for b in broken[:20]:
        detail = "".join(details.get((b, ""), [])) if args.kind == "go" else details.get((b, ""), "")
        print(f"\n----- {b}\n{detail[-3000:]}")
        if gha:
            print(f"::error title={args.label}::{b} failed outside any test (build error, panic or timeout)")
    if gha:
        for k in known_failed[:50]:
            print(f"::warning title={args.label} (known failure)::{fmt(k)}")

    amlib.append_summary(text)
    if args.summary:
        amlib.write_text(args.summary, text + "\n")
    return 0 if ok else 1


if __name__ == "__main__":
    sys.exit(main())
