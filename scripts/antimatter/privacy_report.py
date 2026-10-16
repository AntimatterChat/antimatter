#!/usr/bin/env python3
"""Privacy report: every outside address the shipped code names, grouped by host.

    scripts/antimatter/privacy_report.py --out DIR

Scans the non-test server code (Go, email templates), the web apps (Fusion, classic, platform packages) and
the English catalogs for http(s) URLs, WebSocket URLs and well-known service domains. Each host is classified:

  default setting   the default value of a System Console setting (model/config.go): contacted when the
                    feature is used, unless the admin changes it
  in code           a URL in code: may be contacted on the server's or the browser's own initiative (review)
  link              a link shown to people (href, Markdown link, catalog text): opened only when clicked
  not contacted     placeholders, examples, XML namespaces and spec references

It also lists the settings that hold an outside address and are empty by default (user-configured: nothing is
contacted until an admin fills them in). Writes DIR/privacy.md (also appended to the GitHub job summary) and
DIR/privacy.json. Report only: the outbound-host policy check is what fails the build.
"""

import argparse
import collections
import json
import os
import re
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import amlib  # noqa: E402

SCOPES = ("server/", "webapp/fusion/src/", "webapp/channels/src/", "webapp/platform/")
EXCLUDE_RE = re.compile(r"(^server/(scripts|build|tests|i18n)/|^webapp/platform/[^/]+/(lib|dist|build)/|\.d\.ts$|"
                        r"^webapp/(fusion|channels)/src/i18n/(?!en\.json))")
SCANNED_EXT_RE = re.compile(r"\.(go|ts|tsx|js|jsx|mjs|html)$")
CATALOGS = ("server/i18n/en.json", "webapp/fusion/src/i18n/en.json", "webapp/channels/src/i18n/en.json")
CONFIG_GO = "server/public/model/config.go"

URL_RE = re.compile(r"\b(?:https?|wss?)://([A-Za-z0-9.-]+\.[A-Za-z]{2,}|localhost|\d+\.\d+\.\d+\.\d+)(?::\d+)?[^\s'\"`)<>\]]*")
BARE_DOMAIN_RE = re.compile(r"['\"`]((?:[a-z0-9-]+\.)+(?:googleapis|gstatic|gravatar|giphy|tenor|sentry|segment|"
                            r"rudderstack|matterlytics|mattermost|cloudflare|jsdelivr|unpkg|amazonaws|"
                            r"azure|openai|anthropic)\.(?:com|io|net|org|ai))['\"`]")

NOT_CONTACTED = re.compile(
    r"(^|\.)(example\.(com|org|net)|localhost|test|invalid|local|w3\.org|xmlsoap\.org|purl\.org|adobe\.com|"
    r"openxmlformats\.org|ietf\.org|rfc-editor\.org|schema\.org|json-schema\.org|xmpp\.org|microsoft\.com/office|"
    r"yourcompany\.com|yourdomain\.com|domain\.com|company\.com|mysite\.com|myserver\.com|server\.com)$"
    r"|\.example$|^(\d+\.\d+\.\d+\.\d+)$"
)
# Generators of sample and load-test content: their URLs end up in made-up messages, never requested.
SAMPLE_PATH_RE = re.compile(r"(textgeneration|sampledata|command_loadtest|loadtest)")
LINK_RE = re.compile(r"href|\]\(https?:|<a\b|window\.open|Link\b|link\b|ExternalLink|openExternal|helpText|"
                     r"help_text|defaultMessage|placeholder", re.I)

KNOWN_SERVICES = {
    "mattermost.com": "Mattermost, Inc. (must never be contacted)",
    "googleapis.com": "Google APIs (Google sign-in when enabled, Firebase push)",
    "google.com": "Google",
    "gstatic.com": "Google static content",
    "microsoftonline.com": "Microsoft sign-in (when Entra ID SSO is enabled)",
    "microsoft.com": "Microsoft",
    "gravatar.com": "Gravatar (profile pictures)",
    "giphy.com": "GIPHY", "tenor.com": "Tenor / Google GIFs",
    "github.com": "GitHub", "githubusercontent.com": "GitHub content",
    "gitlab.com": "GitLab",
    "apple.com": "Apple", "push.apple.com": "Apple push service",
    "youtube.com": "YouTube embeds", "vimeo.com": "Vimeo embeds",
    "openai.com": "OpenAI", "anthropic.com": "Anthropic",
    "sentry.io": "Sentry (telemetry)", "segment.io": "Segment (telemetry)", "rudderstack.com": "Rudder (telemetry)",
    "jsdelivr.net": "jsDelivr CDN", "unpkg.com": "unpkg CDN", "cloudflare.com": "Cloudflare",
    "amazonaws.com": "Amazon Web Services (S3 when configured)",
    "antimatter.chat": "Antimatter's website",
}

Hit = collections.namedtuple("Hit", "host url path line kind")


def base_domain(host):
    parts = host.lower().split(".")
    return ".".join(parts[-2:]) if len(parts) >= 2 else host


def service_of(host):
    host = host.lower()
    for domain, label in sorted(KNOWN_SERVICES.items(), key=lambda kv: -len(kv[0])):
        if host == domain or host.endswith("." + domain):
            return label
    return ""


def config_defaults():
    """URLs that are default values of settings, and URL-like settings that are empty by default."""
    defaults, empty = set(), []
    for line in amlib.read_lines(os.path.join(amlib.ROOT, CONFIG_GO)):
        for m in URL_RE.finditer(line):
            if re.search(r"Default\w*\s*=|NewPointer\(", line):
                defaults.add(m.group(0))
        m = re.search(r"\b(\w+)\.(\w*(?:URL|Url|Server|Endpoint|Host|Address|Link)\w*)\s*=\s*NewPointer\(\"\"\)", line)
        if m:
            empty.append(m.group(2))
    return defaults, sorted(set(empty))


def classify(host, url, path, line, defaults):
    if NOT_CONTACTED.search(host.lower()) or SAMPLE_PATH_RE.search(path):
        return "not contacted"
    if url in defaults or (path == CONFIG_GO and re.search(r"Default\w*\s*=", line)):
        return "default setting"
    if path.endswith(".json") or path.startswith("server/templates/") or LINK_RE.search(line):
        return "link"
    return "in code"


def scan(defaults):
    hits = []
    files = [p for p in amlib.tracked_files() if p.startswith(SCOPES) and not EXCLUDE_RE.search(p)]
    for path in files:
        if amlib.is_test_path(path) or not (SCANNED_EXT_RE.search(path) or path in CATALOGS):
            continue
        for i, line in enumerate(amlib.read_lines(os.path.join(amlib.ROOT, path)), 1):
            if path.endswith((".go", ".ts", ".tsx", ".js", ".jsx", ".mjs")) and amlib.is_comment_line(line):
                continue
            found = [(m.group(1), m.group(0)) for m in URL_RE.finditer(line)]
            found += [(m.group(1), m.group(1)) for m in BARE_DOMAIN_RE.finditer(line)]
            for host, url in found:
                hits.append(Hit(host.lower(), url, path, i, classify(host, url, path, line, defaults)))
    return hits


ORDER = ["in code", "default setting", "link", "not contacted"]


def main():
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument("--out", required=True)
    args = parser.parse_args()

    defaults, empty_settings = config_defaults()
    hits = scan(defaults)
    by_host = collections.defaultdict(list)
    for h in hits:
        by_host[h.host].append(h)

    hosts = []
    for host, hs in by_host.items():
        kinds = collections.Counter(h.kind for h in hs)
        kind = next(k for k in ORDER if kinds.get(k))
        sample = next(h for h in hs if h.kind == kind)
        hosts.append({"host": host, "kind": kind, "kinds": dict(kinds), "count": len(hs), "service": service_of(host),
                      "example": f"{sample.path}:{sample.line}", "url": sample.url[:120],
                      "files": sorted({h.path for h in hs})[:20]})
    hosts.sort(key=lambda r: (ORDER.index(r["kind"]), base_domain(r["host"]), r["host"]))

    md = ["## Privacy: outside addresses in the code", "",
          f"{len(hosts)} hosts named in shipped code ({len(hits)} occurrences). \"In code\" and \"default setting\" "
          "are the ones Antimatter (server or browser) may contact on its own; review them. Links are opened only "
          "when someone clicks them.", ""]
    counts = collections.Counter(r["kind"] for r in hosts)
    md += ["| Kind | Hosts |", "| --- | ---: |"] + [f"| {k} | {counts.get(k, 0)} |" for k in ORDER] + [""]
    mm = [r for r in hosts if r["host"] == "mattermost.com" or r["host"].endswith(".mattermost.com")]
    if mm:
        md.append(f"**{len(mm)} Mattermost hosts are still named** ({sum(r['count'] for r in mm)} occurrences); "
                  "see the outbound policy check's baseline for the list to fix.")
        md.append("")
    for kind in ORDER[:3]:
        rows = [r for r in hosts if r["kind"] == kind]
        if not rows:
            continue
        open_tag = "" if kind != "link" else "<details><summary>Links (" + str(len(rows)) + " hosts)</summary>\n"
        md += [f"### {kind.capitalize()}", ""] + ([open_tag] if open_tag else [])
        md += ["| Host | Service | Uses | Example |", "| --- | --- | ---: | --- |"]
        md += [f"| `{r['host']}` | {amlib.md_escape(r['service'])} | {r['count']} | `{r['example']}` |" for r in rows]
        md += ([""] + ["</details>"] if open_tag else []) + [""]
    if empty_settings:
        md += ["### User-configured addresses", "",
               "These settings name an outside address but are empty by default: nothing is contacted until an "
               "admin fills them in.", "", ", ".join(f"`{s}`" for s in empty_settings), ""]

    text = "\n".join(md)
    amlib.write_text(os.path.join(args.out, "privacy.md"), text + "\n")
    amlib.write_text(os.path.join(args.out, "privacy.json"),
                     json.dumps({"hosts": hosts, "empty_url_settings": empty_settings}, indent=2) + "\n")
    amlib.append_summary(text)
    print(text)
    return 0


if __name__ == "__main__":
    sys.exit(main())
