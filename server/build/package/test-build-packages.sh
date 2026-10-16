#!/usr/bin/env bash
# Tests build-packages.sh on a dummy release archive: the .deb has the expected layout (links to
# /etc, /var/lib and /var/log, the service, the settings as a conffile, the bundled apps) and
# the .rpm and the renamed archive are written. Needs nfpm and ar (binutils); runs without root.
#
# Usage: test-build-packages.sh
set -euo pipefail

here="$(cd "$(dirname "$0")" && pwd)"
work="$(mktemp -d)"
trap 'rm -rf "${work}"' EXIT
fail() { echo "test-build-packages: FAIL: $*" >&2; exit 1; }

app="${work}/src/antimatter"
mkdir -p "${app}"/{bin,config,client,client-fusion,i18n,templates,fonts,logs,prepackaged_plugins}
printf '#!/bin/sh\necho antimatter\n' > "${app}/bin/antimatter"
printf '#!/bin/sh\necho amctl\n' > "${app}/bin/amctl"
chmod 755 "${app}/bin/antimatter" "${app}/bin/amctl"
echo '{}' > "${app}/config/config.json"
echo '# config.json' > "${app}/config/README.md"
echo '<html></html>' > "${app}/client/root.html"
echo '<html></html>' > "${app}/client-fusion/index.html"
touch "${app}/prepackaged_plugins/app-1.0.0.tar.gz" "${app}/prepackaged_plugins/app-1.0.0.tar.gz.sig"
tar -czf "${work}/antimatter-linux-amd64.tar.gz" -C "${work}/src" antimatter

"${here}/build-packages.sh" "${work}/antimatter-linux-amd64.tar.gz" amd64 1.2.0-rc.1 "${work}/out" >/dev/null

out="${work}/out"
deb="${out}/antimatter_1.2.0~rc.1-1_amd64.deb"
[[ -f "${deb}" ]] || fail "no ${deb##*/}: $(ls "${out}")"
[[ -f "${out}/antimatter-1.2.0~rc.1-1.x86_64.rpm" ]] || fail "no .rpm: $(ls "${out}")"
[[ -f "${out}/antimatter-1.2.0-rc.1-linux-amd64.tar.gz" ]] || fail "no renamed archive: $(ls "${out}")"

(cd "${work}" && ar x "${deb}")
files="$(tar -tvzf "${work}/data.tar.gz")"
control="$(tar -xzOf "${work}/control.tar.gz" ./control ./conffiles)"
expect() { grep -qE -- "$1" <<< "${files}" || fail "the .deb lacks $1"; }
expect '^-rwxr-xr-x .* \./opt/antimatter/bin/antimatter$'
expect '^-rwxr-xr-x .* \./opt/antimatter/bin/amctl$'
expect ' \./opt/antimatter/defaults/config\.json$'
expect ' \./opt/antimatter/client-fusion/index\.html$'
expect ' \./opt/antimatter/prepackaged_plugins/app-1\.0\.0\.tar\.gz\.sig$'
expect ' \./opt/antimatter/config -> /etc/antimatter/config$'
expect ' \./opt/antimatter/data -> /var/lib/antimatter/data$'
expect ' \./opt/antimatter/plugins -> /var/lib/antimatter/plugins$'
expect ' \./opt/antimatter/client/plugins -> /var/lib/antimatter/client-plugins$'
expect ' \./opt/antimatter/logs -> /var/log/antimatter$'
expect ' \./usr/bin/amctl -> /opt/antimatter/bin/amctl$'
expect ' \./usr/lib/systemd/system/antimatter\.service$'
expect '^-rw------- .* \./etc/antimatter/antimatter\.env$'
if grep -qE ' \./opt/antimatter/config/' <<< "${files}"; then fail "the .deb ships files in /opt/antimatter/config"; fi
grep -qx 'Version: 1.2.0~rc.1-1' <<< "${control}" || fail "wrong version: ${control}"
grep -qx '/etc/antimatter/antimatter.env' <<< "${control}" || fail "antimatter.env isn't a conffile"
grep -q '^Recommends: .*postgresql' <<< "${control}" || fail "PostgreSQL isn't recommended"
echo "test-build-packages: OK"
