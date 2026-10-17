#!/usr/bin/env bash
# Downloads the bundled apps of the release archives and packages (their prepackaged_plugins/):
# the latest GitHub release of every app listed in antimatter-docker's plugins.json, its bundle
# (*.tar.gz) and the bundle's signature (*.tar.gz.sig), checked against the Antimatter plugin
# public key (build/plugin-production-public-key.gpg).
#
# Usage: fetch-apps.sh <out-dir> [plugins.json]
#   Needs gh (GH_TOKEN in CI), python3 and gpg. Without a plugins.json file, it reads the one on
#   antimatter-docker's main branch (AM_APPS_LIST_REPO, AM_APPS_LIST_REF override it).
#   Only the apps named in AM_BUNDLED_APPS (repository names, space-separated) are bundled; by
#   default Whiteboard, Voice channels, Calls, Agents, Todo, Polls, Notes, GIFs and Memes (Memes adds its
#   tab to the GIFs picker). The others can be
#   installed later from their own releases.
#
# An app takes its latest release, or its newest prerelease when it has no release yet (with a
# warning). Apps without any release, and bundles without a signature (the server only installs
# signed prepackaged apps), are skipped with a warning; a bad signature fails. <out-dir>/apps.txt
# lists what was bundled.
set -euo pipefail

warn() { echo "::warning title=Bundled apps::$*"; }
die() { echo "::error title=Bundled apps::$*"; exit 1; }

[[ $# -ge 1 ]] || die "usage: $0 <out-dir> [plugins.json]"
out=$1 list=${2:-}
here="$(cd "$(dirname "$0")" && pwd)"
public_key="${here}/../plugin-production-public-key.gpg"
for tool in gh python3 gpg; do command -v "${tool}" >/dev/null || die "${tool} is not installed"; done

work="$(mktemp -d)"
export GNUPGHOME="${work}/gnupg"
mkdir -m 700 "${GNUPGHOME}"
trap 'gpgconf --kill all >/dev/null 2>&1 || true; rm -rf "${work}"' EXIT
gpg --batch --quiet --import "${public_key}"

if [[ -z "${list}" ]]; then
  list="${work}/plugins.json"
  gh api -H "Accept: application/vnd.github.raw+json" \
    "repos/${AM_APPS_LIST_REPO:-AntimatterChat/antimatter-docker}/contents/plugins.json?ref=${AM_APPS_LIST_REF:-main}" > "${list}"
fi

mkdir -p "${out}"
: > "${out}/apps.txt"
count=0
bundled="${AM_BUNDLED_APPS:-antimatter-plugin-whiteboard antimatter-plugin-voice-channels antimatter-plugin-calls antimatter-plugin-agents antimatter-plugin-todo antimatter-plugin-polls antimatter-plugin-notes antimatter-plugin-gifs antimatter-plugin-memes}"
repos="$(python3 -c '
import json, sys
wanted = set(sys.argv[2].split())
print("\n".join(p["repo"] for p in json.load(open(sys.argv[1])) if p["repo"].split("/")[-1] in wanted))
' "${list}" "${bundled}")"
for name in ${bundled}; do
  grep -q "/${name}$" <<<"${repos}" || warn "${name} is not in plugins.json: not bundled"
done
for repo in ${repos}; do
  tag="$(gh release view --repo "${repo}" --json tagName --jq .tagName 2>/dev/null || true)"
  if [[ -z "${tag}" ]]; then
    tag="$(gh release list --repo "${repo}" --exclude-drafts --limit 50 --json tagName \
      --jq '[.[] | select(.tagName | startswith("v"))][0].tagName // empty' 2>/dev/null || true)"
    if [[ -z "${tag}" ]]; then
      warn "${repo} has no release yet: not bundled"
      continue
    fi
    warn "${repo} has no stable release: bundling its prerelease ${tag}"
  fi

  dir="${work}/${repo//\//_}"
  mkdir -p "${dir}"
  gh release download "${tag}" --repo "${repo}" --dir "${dir}" --pattern '*.tar.gz' --pattern '*.tar.gz.sig'
  shopt -s nullglob
  bundles=("${dir}"/*.tar.gz)
  shopt -u nullglob
  if [[ ${#bundles[@]} -eq 0 ]]; then
    warn "${repo} ${tag} has no .tar.gz bundle: not bundled"
    continue
  fi
  for bundle in "${bundles[@]}"; do
    name="$(basename "${bundle}")"
    if [[ ! -f "${bundle}.sig" ]]; then
      warn "${repo} ${tag}: ${name} is unsigned, and the server only installs signed bundled apps: not bundled"
      continue
    fi
    gpg --batch --quiet --verify "${bundle}.sig" "${bundle}" 2>/dev/null \
      || die "${repo} ${tag}: the signature of ${name} doesn't match the Antimatter plugin key"
    cp "${bundle}" "${bundle}.sig" "${out}/"
    echo "${repo} ${tag} ${name}" >> "${out}/apps.txt"
    count=$((count + 1))
  done
done

echo "Bundled ${count} app(s):"
cat "${out}/apps.txt"
