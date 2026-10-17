#!/usr/bin/env bash
# Builds the all-in-one Antimatter .deb and .rpm packages of one architecture from a release
# archive (make package-linux-<arch>), with nfpm (https://nfpm.goreleaser.com), and copies the
# archive next to them under its release name.
#
# Usage: build-packages.sh <antimatter-linux-ARCH.tar.gz> <amd64|arm64> <version> <out-dir>
#   e.g. build-packages.sh dist/antimatter-linux-amd64.tar.gz amd64 1.2.0 release/
# Writes <out-dir>/antimatter-<version>-linux-<arch>.tar.gz, antimatter_<version>-1_<arch>.deb and
# antimatter-<version>-1.<x86_64|aarch64>.rpm (a version like 1.2.0-rc.1 becomes 1.2.0~rc.1 in
# the .deb and .rpm, so that it sorts before 1.2.0; the archive keeps 1.2.0-rc.1).
#
# Package layout:
#   /opt/antimatter                  the archive's content (bin/, client/, client-fusion/, i18n/,
#                                    templates/, fonts/, prepackaged_plugins/), owned by root,
#                                    except the web apps (see scripts/postinstall.sh)
#   /opt/antimatter/defaults         the archive's config/: config.json is copied to
#                                    /etc/antimatter/config at the first install
#   /opt/antimatter/config        -> /etc/antimatter/config (config.json, written by the server)
#   /opt/antimatter/data          -> /var/lib/antimatter/data
#   /opt/antimatter/plugins       -> /var/lib/antimatter/plugins
#   /opt/antimatter/client/plugins -> /var/lib/antimatter/client-plugins
#   /opt/antimatter/logs          -> /var/log/antimatter (file logs are off: logs go to the journal)
#   /etc/antimatter/antimatter.env   AM_* settings for the service (config, kept on upgrades)
#   /usr/lib/systemd/system/antimatter.service, /usr/bin/amctl -> /opt/antimatter/bin/amctl
set -euo pipefail

die() { echo "build-packages: $*" >&2; exit 1; }

[[ $# -eq 4 ]] || die "usage: $0 <archive> <amd64|arm64> <version> <out-dir>"
archive=$1 arch=$2 version=$3 out=$4
case "${arch}" in amd64 | arm64) ;; *) die "unsupported architecture ${arch}" ;; esac
[[ -f "${archive}" ]] || die "no archive ${archive}"
command -v nfpm >/dev/null || die "nfpm is not installed (go install github.com/goreleaser/nfpm/v2/cmd/nfpm@<version>)"

here="$(cd "$(dirname "$0")" && pwd)"
stage="$(mktemp -d)"
trap 'rm -rf "${stage}"' EXIT

out="$(mkdir -p "${out}" && cd "${out}" && pwd)"
mkdir -p "${stage}/root/opt"
tar -xzf "${archive}" -C "${stage}/root/opt"
cp -R "${here}" "${stage}/package"
app="${stage}/root/opt/antimatter"
[[ -x "${app}/bin/antimatter" && -x "${app}/bin/amctl" ]] || die "${archive} has no antimatter/bin/antimatter and bin/amctl"
[[ -f "${app}/config/config.json" ]] || die "${archive} has no antimatter/config/config.json"
[[ -d "${app}/client" && -d "${app}/client-fusion" ]] || die "${archive} lacks a web app (client/, client-fusion/)"

# The directories the server writes become links to /etc, /var/lib and /var/log (see nfpm.yaml).
mv "${app}/config" "${app}/defaults"
rm -rf "${app}/data" "${app}/plugins" "${app}/logs" "${app}/client/plugins"
mkdir -p "${app}/prepackaged_plugins"

export AM_PKG_ARCH="${arch}" AM_PKG_VERSION="${version}"
for packager in deb rpm; do
  (cd "${stage}" && nfpm package --config package/nfpm.yaml --packager "${packager}" --target "${out}/")
done
cp "${archive}" "${out}/antimatter-${version}-linux-${arch}.tar.gz"
ls -l "${out}"
