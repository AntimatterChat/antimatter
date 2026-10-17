#!/usr/bin/env python3
"""Antimatter policy checks: branding, outbound hosts, AM_ variable names and translations.

    scripts/antimatter/policy_check.py [--check NAME ...] [--update-baseline] [--summary FILE]

Checks (all run by default):
  branding      user-visible English strings (server, Fusion and classic en.json) and the Fusion UI source must
                not say "Mattermost".
  outbound      non-test code must not contain Mattermost service hosts (*.mattermost.com, matterlytics),
                Sentry DSNs or Rudder/Segment telemetry endpoints and keys.
  am-env        Antimatter's docs and compose examples write environment variables as AM_..., not MM_...
  translations  every string of the server and Fusion en.json is translated into the 37 supported languages.

A finding is accepted when it matches scripts/antimatter/policy-allowlist.txt (permanent, commented exceptions)
or scripts/antimatter/policy-baseline.txt (violations that predate the check and still need fixing). Anything
else is a new violation and makes the check fail. --update-baseline rewrites the baseline with the current
findings: only use it to remove fixed entries, never to hide a new violation.
"""

import argparse
import collections
import os
import re
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import amlib  # noqa: E402

HERE = os.path.dirname(os.path.abspath(__file__))
ALLOWLIST = os.path.join(HERE, "policy-allowlist.txt")
BASELINE = os.path.join(HERE, "policy-baseline.txt")

CHECKS = ("branding", "outbound", "am-env", "translations")

CATALOGS = {
    "server/i18n/en.json": "server",
    "webapp/fusion/src/i18n/en.json": "fusion",
    "webapp/channels/src/i18n/en.json": "classic",
}

# The languages every new string must be translated into before it ships (en-AU falls back to en).
LANGUAGES = (
    "ar bg bn ca cs da de el es fa fi fil fr he hi hu id it ja ko ms nb-NO nl pl pt pt-BR ro ru sv ta th tr uk "
    "ur vi zh-CN zh-TW"
).split()
TRANSLATED_CATALOG_DIRS = ("server/i18n", "webapp/fusion/src/i18n")

BRANDING_RE = re.compile(r"mattermost", re.I)
# In source code, only the capitalised word is user-facing text; lower case is imports, ids and packages.
BRANDING_SOURCE_RE = re.compile(r"\bMattermost\b")

OUTBOUND_RES = [
    ("Mattermost host", re.compile(r"\b(?:[a-z0-9-]+\.)*mattermost\.(?:com|cloud|info|dev)\b", re.I)),
    ("Matterlytics telemetry", re.compile(r"matterlytics", re.I)),
    ("Sentry DSN", re.compile(r"https?://[0-9a-f]{16,}@[^\s'\"/]*sentry|\bsentry\.io\b", re.I)),
    ("Segment telemetry", re.compile(r"\b(?:api|cdn)\.segment\.(?:io|com)\b", re.I)),
    ("Rudder telemetry", re.compile(r"rudderstack|rudderlabs|rudder\w*\s*[:=]\s*['\"`][0-9A-Za-z]{20,}['\"`]", re.I)),
]

OUTBOUND_SCOPES = ("server/", "webapp/fusion/src/", "webapp/channels/src/", "webapp/platform/")
OUTBOUND_EXCLUDE_RE = re.compile(r"(^server/(scripts|build|tests)/|^webapp/platform/[^/]+/(lib|dist|build)/|\.d\.ts$)")

FUSION_SOURCE = "webapp/fusion/src/"
FUSION_EXCLUDE_RE = re.compile(r"(^webapp/fusion/src/i18n/|\.d\.ts$)")

AM_ENV_FILES = (
    "README.md", "CONTRIBUTING.md", "TESTING.md", "server/docker-compose.yaml", "server/docker-compose.pgvector.yml",
)
AM_ENV_GLOBS = ("scripts/antimatter/**", ".github/workflows/antimatter-*.yml", ".github/actions/antimatter-*/*", "e2e-tests/fusion/**/*.md")
MM_VAR_RE = re.compile(r"\bMM_[A-Z][A-Z0-9_]*")
# A line may name MM_ next to its AM_ counterpart, or say it's the older name.
AM_ENV_OK_RE = re.compile(r"\bAM_[A-Z]|older|legacy|still works|alias", re.I)

DEFAULT_MESSAGE_RE = re.compile(r"defaultMessage")
MESSAGE_ID_RE = re.compile(r"""\bid\s*[:=]\s*\{?\s*['"`]([^'"`]+)['"`]""")

Finding = collections.namedtuple("Finding", "check path line text detail")


def norm(text):
    return re.sub(r"\s+", " ", text.strip())[:400]


def fingerprint(f):
    return f"{f.check}\t{f.path}\t{norm(f.text)}"


# ---------------------------------------------------------------------------------------------------- checks

def check_branding(files):
    findings = []
    catalogs = {}
    for path, area in CATALOGS.items():
        full = os.path.join(amlib.ROOT, path)
        if not os.path.exists(full):
            continue
        catalog = amlib.load_catalog(full)
        catalogs[area] = catalog
        for key, value in catalog.items():
            # The web apps' catalogs hold {defaultMessage, description}; the server's a string (or plural forms).
            if isinstance(value, dict):
                value = value.get("defaultMessage", "")
            if isinstance(value, str) and BRANDING_RE.search(value):
                findings.append(Finding("branding", path, 0, f"{key}: {value}", f"{area} catalog"))

    fusion_catalog = catalogs.get("fusion", {})
    for path in files:
        if not path.startswith(FUSION_SOURCE) or FUSION_EXCLUDE_RE.search(path) or amlib.is_test_path(path):
            continue
        if not re.search(r"\.(ts|tsx|js|jsx)$", path):
            continue
        lines = amlib.read_lines(os.path.join(amlib.ROOT, path))
        for i, line in enumerate(lines):
            if not BRANDING_SOURCE_RE.search(line) or amlib.is_comment_line(line):
                continue
            if re.match(r"\s*(import|export \* from)\b", line) or "Copyright" in line:
                continue
            if DEFAULT_MESSAGE_RE.search(line):
                # The catalog, not the defaultMessage, is shown when it has the id; the catalog is checked above.
                window = " ".join(lines[max(0, i - 4):i + 1])
                ids = MESSAGE_ID_RE.findall(window)
                if ids and ids[-1] in fusion_catalog:
                    continue
            findings.append(Finding("branding", path, i + 1, line, "Fusion UI source"))
    return findings


def check_outbound(files):
    findings = []
    for path in files:
        if not path.startswith(OUTBOUND_SCOPES) or not amlib.SOURCE_EXT_RE.search(path):
            continue
        if amlib.is_test_path(path) or OUTBOUND_EXCLUDE_RE.search(path):
            continue
        for i, line in enumerate(amlib.read_lines(os.path.join(amlib.ROOT, path))):
            if amlib.is_comment_line(line):
                continue
            for label, regex in OUTBOUND_RES:
                if regex.search(line):
                    findings.append(Finding("outbound", path, i + 1, line, label))
                    break
    return findings


def check_am_env(files):
    findings = []
    candidates = [p for p in files if p in AM_ENV_FILES or any(amlib.glob_match(p, g) for g in AM_ENV_GLOBS)]
    for path in candidates:
        if path.endswith(("policy-baseline.txt", "policy-allowlist.txt", "policy_check.py")):
            continue
        for i, line in enumerate(amlib.read_lines(os.path.join(amlib.ROOT, path))):
            if MM_VAR_RE.search(line) and not AM_ENV_OK_RE.search(line):
                findings.append(Finding("am-env", path, i + 1, line, "MM_ variable"))
    return findings


def check_translations(_files):
    findings = []
    for directory in TRANSLATED_CATALOG_DIRS:
        en_path = os.path.join(amlib.ROOT, directory, "en.json")
        if not os.path.exists(en_path):
            continue
        en = amlib.load_catalog(en_path)
        for lang in LANGUAGES:
            rel = f"{directory}/{lang}.json"
            full = os.path.join(amlib.ROOT, rel)
            catalog = amlib.load_catalog(full) if os.path.exists(full) else {}
            for key in en:
                if not catalog.get(key):
                    findings.append(Finding("translations", rel, 0, key, "missing translation"))
    return findings


RUNNERS = {
    "branding": check_branding,
    "outbound": check_outbound,
    "am-env": check_am_env,
    "translations": check_translations,
}

ADVICE = {
    "branding": (
        'User-visible text says "Antimatter", never "Mattermost" (teams are "communities", Boards is "Kanban"). '
        "Reword the string (and its 37 translations). Keep \"Mattermost\" only for copyright lines, "
        '"Mattermost, Inc." named as the company running a service, or a compatibility note, and then add an '
        "allowlist entry with a comment saying why."
    ),
    "outbound": (
        "Antimatter never contacts Mattermost, Inc. and ships no telemetry: remove the URL, host or key, point "
        "links at https://antimatter.chat/docs/ or https://github.com/AntimatterChat/community, or make it an "
        "empty default the admin configures. Only detection constants (e.g. model.IsMattermostServiceURL) may "
        "name these hosts, with an allowlist entry."
    ),
    "am-env": (
        "Write environment variables as AM_... in docs and compose examples; mention MM_... only as the older "
        "name that still works (on the same line)."
    ),
    "translations": (
        "Translate every new or changed string into all 37 languages before pushing (server/i18n and "
        "webapp/fusion/src/i18n)."
    ),
}


# ---------------------------------------------------------------------------------------------------- lists

def load_allowlist():
    entries = []
    pending_comment = False
    for n, raw in enumerate(amlib.read_lines(ALLOWLIST), 1):
        line = raw.strip()
        if not line:
            pending_comment = False
            continue
        if line.startswith("#"):
            pending_comment = True
            continue
        parts = line.split(None, 2)
        if len(parts) != 3 or parts[0] not in CHECKS:
            raise SystemExit(f"{ALLOWLIST}:{n}: expected '<check> <path-glob> <regex>', got: {raw}")
        if not pending_comment:
            raise SystemExit(f"{ALLOWLIST}:{n}: every allowlist entry needs a comment line above it saying why")
        entries.append((parts[0], parts[1], re.compile(parts[2]), n))
    return entries


def load_baseline():
    if not os.path.exists(BASELINE):
        return set()
    return {line for line in amlib.read_lines(BASELINE) if line and not line.startswith("#")}


def allowed(f, allowlist):
    for check, glob, regex, _ in allowlist:
        if check == f.check and amlib.glob_match(f.path, glob) and regex.search(f.text):
            return True
    return False


BASELINE_HEADER = """\
# Policy violations that predate the checks in policy_check.py. Each one still needs fixing: fix it, then
# regenerate this file with `scripts/antimatter/policy_check.py --update-baseline` to drop it.
# Never add a new violation here; fix it, or add a commented entry to policy-allowlist.txt if it's legitimate.
# Format: check<TAB>path<TAB>text (the offending line with whitespace collapsed, or "key: value" for catalogs).
"""


def write_baseline(findings):
    lines = sorted({fingerprint(f) for f in findings})
    amlib.write_text(BASELINE, BASELINE_HEADER + "\n".join(lines) + "\n")


# ---------------------------------------------------------------------------------------------------- main

def main():
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument("--check", action="append", choices=CHECKS, help="run only this check (repeatable)")
    parser.add_argument("--update-baseline", action="store_true", help="rewrite policy-baseline.txt")
    parser.add_argument("--summary", help="also write a Markdown summary to this file")
    parser.add_argument("--list-baseline", action="store_true", help="print the baseline findings still present")
    args = parser.parse_args()
    checks = args.check or list(CHECKS)

    files = amlib.tracked_files()
    allowlist = load_allowlist()
    baseline = load_baseline()

    results = {}
    for check in checks:
        results[check] = [f for f in RUNNERS[check](files) if not allowed(f, allowlist)]

    if args.update_baseline:
        kept = [line for line in baseline if line.split("\t", 1)[0] not in checks]
        write_baseline([f for fs in results.values() for f in fs])
        if kept:
            with open(BASELINE, "a", encoding="utf-8") as fh:
                fh.write("\n".join(sorted(kept)) + "\n")
            # keep the file sorted
            body = sorted(load_baseline())
            amlib.write_text(BASELINE, BASELINE_HEADER + "\n".join(body) + "\n")
        print(f"Wrote {os.path.relpath(BASELINE, amlib.ROOT)}")
        return 0

    failed = False
    md = ["## Antimatter policy checks", "", "| Check | New violations | Known (baseline, to fix) | Fixed since baseline |",
          "| --- | ---: | ---: | ---: |"]
    details = []
    for check in checks:
        current = results[check]
        prints = {fingerprint(f) for f in current}
        new = [f for f in current if fingerprint(f) not in baseline]
        known = len(prints & baseline)
        fixed = len({b for b in baseline if b.startswith(check + "\t")} - prints)
        md.append(f"| {check} | {len(new)} | {known} | {fixed} |")
        if new:
            failed = True
            print(f"\n{check}: {len(new)} new violation(s)")
            details.append(f"### {check}: new violations\n")
            details.append(ADVICE[check] + "\n")
            for f in new[:200]:
                where = f"{f.path}:{f.line}" if f.line else f.path
                print(f"  {where}: [{f.detail}] {norm(f.text)[:200]}")
                if os.environ.get("GITHUB_ACTIONS"):
                    title = f"Antimatter policy: {check}"
                    loc = f"file={f.path},line={f.line}" if f.line else f"file={f.path}"
                    print(f"::error {loc},title={title}::{f.detail}: {norm(f.text)[:180]}")
                details.append(f"- `{where}` ({f.detail}): {amlib.md_escape(norm(f.text)[:200])}")
            if len(new) > 200:
                print(f"  ... and {len(new) - 200} more")
            print(f"  What to do: {ADVICE[check]}")
            details.append("")
        else:
            print(f"{check}: OK ({known} known violation(s) still to fix, {fixed} fixed since the baseline)")
        if fixed:
            print(f"  {fixed} baseline entr{'y is' if fixed == 1 else 'ies are'} fixed: run "
                  "scripts/antimatter/policy_check.py --update-baseline to drop them.")

    if args.list_baseline:
        for check in checks:
            for f in results[check]:
                if fingerprint(f) in baseline:
                    where = f"{f.path}:{f.line}" if f.line else f.path
                    print(f"{check}\t{where}\t{norm(f.text)[:200]}")

    md.append("")
    md.extend(details)
    if failed:
        md.append("If a finding is legitimate (copyright, \"Mattermost, Inc.\" as the company, a detection constant), "
                  "add an entry with a comment explaining why to `scripts/antimatter/policy-allowlist.txt`. Re-run "
                  "locally with `scripts/antimatter/check.sh policy`.")
        print("\nIf a finding is legitimate (copyright, \"Mattermost, Inc.\" as the company, a detection constant), "
              "add an entry with a comment explaining why to scripts/antimatter/policy-allowlist.txt.\n"
              "Re-run locally with: scripts/antimatter/check.sh policy")
    text = "\n".join(md)
    amlib.append_summary(text)
    if args.summary:
        amlib.write_text(args.summary, text + "\n")
    return 1 if failed else 0


if __name__ == "__main__":
    sys.exit(main())
