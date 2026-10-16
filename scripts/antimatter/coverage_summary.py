#!/usr/bin/env python3
"""Coverage statistics for the server (Go) and the web apps (jest), without any third-party service.

    scripts/antimatter/coverage_summary.py --out DIR [--go cover.out ...] \
        [--jest fusion=webapp/fusion/coverage/coverage-summary.json] [--jest classic=...] [--jest platform=...]

Go profiles from several shards are merged (a block counts as covered when any shard covered it) into
DIR/server-cover.out, which `go tool cover -html` turns into the HTML report. jest json-summary files come from
merge_jest_coverage.js. Writes DIR/coverage.md (also appended to the GitHub job summary), DIR/coverage.json
and one SVG badge per area (DIR/badges/*.svg).
"""

import argparse
import collections
import datetime
import json
import os
import re
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import amlib  # noqa: E402

GO_MODULES = (
    ("github.com/mattermost/mattermost/server/v8/", "server/"),
    ("github.com/mattermost/mattermost/server/public/", "server/public/"),
)

# Server code written for Antimatter (whole directories; Antimatter's changes inside upstream packages are
# counted with those packages).
ANTIMATTER_SERVER_DIRS = ("server/libre/", "server/cmd/antimatter/", "server/cmd/amctl/")

AREA_LABELS = {
    "server": "Server (Go)",
    "fusion": "Fusion web UI (webapp/fusion)",
    "classic": "Classic web UI (webapp/channels)",
    "platform": "Shared web packages (webapp/platform)",
}

Stat = collections.namedtuple("Stat", "total covered")


def pct(stat):
    return 100.0 * stat.covered / stat.total if stat.total else 0.0


def add(a, b):
    return Stat(a.total + b.total, a.covered + b.covered)


ZERO = Stat(0, 0)


# ---------------------------------------------------------------------------------------------------- Go

def go_file_path(name):
    for prefix, local in GO_MODULES:
        if name.startswith(prefix):
            return local + name[len(prefix):]
    return name


def merge_go(profiles, out_path):
    """Merges cover profiles: per block, the highest count wins. Returns {local file path: Stat}."""
    blocks = {}
    mode = "set"
    for path in profiles:
        for line in amlib.read_lines(path):
            if line.startswith("mode:"):
                mode = line.split(":", 1)[1].strip()
                continue
            m = re.match(r"^(.+):(\d+\.\d+,\d+\.\d+) (\d+) (\d+)$", line)
            if not m:
                continue
            key = (m.group(1), m.group(2))
            stmts, count = int(m.group(3)), int(m.group(4))
            prev = blocks.get(key)
            if prev is None or count > prev[1]:
                blocks[key] = (stmts, count)
    with open(out_path, "w", encoding="utf-8") as f:
        f.write(f"mode: {mode}\n")
        for (name, pos), (stmts, count) in sorted(blocks.items()):
            f.write(f"{name}:{pos} {stmts} {count}\n")
    files = collections.defaultdict(lambda: ZERO)
    for (name, _), (stmts, count) in blocks.items():
        files[go_file_path(name)] = add(files[go_file_path(name)], Stat(stmts, stmts if count > 0 else 0))
    return dict(files)


# ---------------------------------------------------------------------------------------------------- jest

def load_jest_summary(path):
    """Returns {repo-relative file path: Stat} from an istanbul json-summary (statements)."""
    with open(path, encoding="utf-8") as f:
        data = json.load(f)
    files = {}
    for name, entry in data.items():
        if name == "total":
            continue
        st = entry.get("statements", {})
        rel = os.path.relpath(name, amlib.ROOT) if os.path.isabs(name) else name
        rel = re.sub(r"^.*?(webapp/)", r"\1", rel)
        files[rel] = Stat(int(st.get("total", 0)), int(st.get("covered", 0)))
    return files


# ---------------------------------------------------------------------------------------------------- grouping

def group(files, depth, prefix=""):
    groups = collections.defaultdict(lambda: ZERO)
    for path, stat in files.items():
        if not path.startswith(prefix):
            continue
        parts = path[len(prefix):].split("/")
        key = prefix + "/".join(parts[:depth]) if len(parts) > depth else prefix + "/".join(parts[:-1] or parts)
        groups[key] = add(groups[key], stat)
    return dict(groups)


def total(files, prefixes=None):
    t = ZERO
    for path, stat in files.items():
        if prefixes is None or path.startswith(prefixes):
            t = add(t, stat)
    return t


def table(rows, header="Area"):
    out = [f"| {header} | Statements | Covered | Coverage |", "| --- | ---: | ---: | ---: |"]
    for name, stat in rows:
        out.append(f"| {amlib.md_escape(name)} | {stat.total} | {stat.covered} | {pct(stat):.1f}% |")
    return out


def badge(label, value):
    colour = "#4c1" if value >= 80 else "#a3c51c" if value >= 60 else "#dfb317" if value >= 40 else "#e05d44"
    text = f"{value:.0f}%"
    lw, vw = 7 * len(label) + 10, 7 * len(text) + 10
    return (
        f'<svg xmlns="http://www.w3.org/2000/svg" width="{lw + vw}" height="20" role="img" '
        f'aria-label="{label}: {text}"><rect width="{lw}" height="20" fill="#555"/>'
        f'<rect x="{lw}" width="{vw}" height="20" fill="{colour}"/>'
        f'<g fill="#fff" font-family="Verdana,DejaVu Sans,sans-serif" font-size="11" text-anchor="middle">'
        f'<text x="{lw / 2}" y="14">{label}</text><text x="{lw + vw / 2}" y="14">{text}</text></g></svg>\n'
    )


def main():
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument("--out", required=True)
    parser.add_argument("--go", nargs="*", default=[])
    parser.add_argument("--jest", action="append", default=[], help="area=path/to/coverage-summary.json")
    args = parser.parse_args()

    os.makedirs(os.path.join(args.out, "badges"), exist_ok=True)
    areas = collections.OrderedDict()
    detail = {}

    go_profiles = [p for p in args.go if os.path.exists(p)]
    if go_profiles:
        files = merge_go(go_profiles, os.path.join(args.out, "server-cover.out"))
        areas["server"] = total(files)
        detail["server"] = files
    for spec in args.jest:
        area, _, path = spec.partition("=")
        if os.path.exists(path):
            files = load_jest_summary(path)
            areas[area] = total(files)
            detail[area] = files

    if not areas:
        print("No coverage data found.", file=sys.stderr)
        return 1

    md = ["## Coverage", ""]
    md += table([(AREA_LABELS.get(a, a), s) for a, s in areas.items()])
    md.append("")
    result = {
        "generated": datetime.datetime.now(datetime.timezone.utc).isoformat(timespec="seconds"),
        "commit": os.environ.get("GITHUB_SHA", ""),
        "areas": {a: {"statements": s.total, "covered": s.covered, "percent": round(pct(s), 2)}
                  for a, s in areas.items()},
        "groups": {},
    }

    if "server" in detail:
        files = detail["server"]
        antimatter = total(files, ANTIMATTER_SERVER_DIRS)
        md += ["### Server", "", f"Antimatter-only code ({', '.join(ANTIMATTER_SERVER_DIRS)}): "
               f"{antimatter.covered}/{antimatter.total} statements, **{pct(antimatter):.1f}%**.", ""]
        groups = group(files, 2, "server/")
        groups.pop("server/public", None)
        groups.update(group(files, 1, "server/public/"))
        rows = sorted(groups.items(), key=lambda kv: -kv[1].total)
        md += ["<details><summary>Per package area</summary>", ""] + table(rows, "Package area") + ["", "</details>", ""]
        result["groups"]["server"] = {k: {"statements": v.total, "covered": v.covered} for k, v in rows}
        result["areas"]["server-antimatter-only"] = {
            "statements": antimatter.total, "covered": antimatter.covered, "percent": round(pct(antimatter), 2)}

    for area, prefix, depth in (("fusion", "webapp/fusion/src/", 1), ("classic", "webapp/channels/src/", 1),
                                ("platform", "webapp/platform/", 1)):
        if area not in detail:
            continue
        files = detail[area]
        groups = group(files, depth, prefix)
        if area == "fusion":
            own = total(files, ("webapp/fusion/src/fusion/",))
            md += ["### Fusion", "", f"Fusion's own UI (webapp/fusion/src/fusion): {own.covered}/{own.total} "
                   f"statements, **{pct(own):.1f}%**.", ""]
            result["areas"]["fusion-own-ui"] = {"statements": own.total, "covered": own.covered,
                                                "percent": round(pct(own), 2)}
            groups.update({k: v for k, v in group(files, 2, "webapp/fusion/src/fusion/").items()})
        else:
            md += [f"### {AREA_LABELS[area]}", ""]
        rows = sorted(groups.items(), key=lambda kv: -kv[1].total)
        md += ["<details><summary>Per directory</summary>", ""] + table(rows, "Directory") + ["", "</details>", ""]
        result["groups"][area] = {k: {"statements": v.total, "covered": v.covered} for k, v in rows}

    for area, values in result["areas"].items():
        with open(os.path.join(args.out, "badges", f"coverage-{area}.svg"), "w", encoding="utf-8") as f:
            f.write(badge(area, values["percent"]))

    md.append("Full HTML reports are in this run's coverage artifacts.")
    text = "\n".join(md)
    amlib.write_text(os.path.join(args.out, "coverage.md"), text + "\n")
    amlib.write_text(os.path.join(args.out, "coverage.json"), json.dumps(result, indent=2) + "\n")
    amlib.append_summary(text)
    print(text)
    return 0


if __name__ == "__main__":
    sys.exit(main())
