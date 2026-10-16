#!/usr/bin/env bash
# Installs the open-source command-line tools used by the Antimatter checks, at pinned versions, and checks
# each download against its SHA-256 before installing it. Linux x86_64 only; elsewhere, install the same
# versions yourself and put them on PATH.
#
#   scripts/antimatter/install-tools.sh [tool...]     (default: all)
#
# Tools go to $AM_TOOLS_DIR (default ~/.cache/antimatter/tools/bin; CI sets it under $RUNNER_TEMP).

set -euo pipefail

AM_TOOLS_DIR="${AM_TOOLS_DIR:-$HOME/.cache/antimatter/tools/bin}"
mkdir -p "$AM_TOOLS_DIR"

ACTIONLINT_VERSION=1.7.12
ACTIONLINT_SHA256=8aca8db96f1b94770f1b0d72b6dddcb1ebb8123cb3712530b08cc387b349a3d8
GITLEAKS_VERSION=8.30.1
GITLEAKS_SHA256=551f6fc83ea457d62a0d98237cbad105af8d557003051f41f3e7ca7b3f2470eb
OSV_SCANNER_VERSION=2.7.0
OSV_SCANNER_SHA256=160c6253e2922e9e0a6a9771c5c4c40a4b3c6090b34a3cdd5e80c96cc2e4218b
SHELLCHECK_VERSION=0.11.0
SHELLCHECK_SHA256=8c3be12b05d5c177a04c29e3c78ce89ac86f1595681cab149b65b97c4e227198
GOVULNCHECK_VERSION=v1.1.4
YAMLLINT_VERSION=1.37.1

if [ "$(uname -s)/$(uname -m)" != "Linux/x86_64" ]; then
    echo "install-tools.sh only downloads Linux x86_64 binaries; install the tools yourself on $(uname -s)/$(uname -m)." >&2
    exit 1
fi

work="$(mktemp -d "${RUNNER_TEMP:-${TMPDIR:-$HOME/.cache}}/am-tools.XXXXXX")"
trap 'rm -rf "$work"' EXIT

fetch() { # url sha256 output
    curl -fsSL --retry 3 -o "$3" "$1"
    echo "$2  $3" | sha256sum -c --quiet - || { echo "checksum mismatch for $1" >&2; exit 1; }
}

install_actionlint() {
    fetch "https://github.com/rhysd/actionlint/releases/download/v${ACTIONLINT_VERSION}/actionlint_${ACTIONLINT_VERSION}_linux_amd64.tar.gz" \
        "$ACTIONLINT_SHA256" "$work/actionlint.tgz"
    tar -xzf "$work/actionlint.tgz" -C "$AM_TOOLS_DIR" actionlint
}

install_gitleaks() {
    fetch "https://github.com/gitleaks/gitleaks/releases/download/v${GITLEAKS_VERSION}/gitleaks_${GITLEAKS_VERSION}_linux_x64.tar.gz" \
        "$GITLEAKS_SHA256" "$work/gitleaks.tgz"
    tar -xzf "$work/gitleaks.tgz" -C "$AM_TOOLS_DIR" gitleaks
}

install_osv-scanner() {
    fetch "https://github.com/google/osv-scanner/releases/download/v${OSV_SCANNER_VERSION}/osv-scanner_linux_amd64" \
        "$OSV_SCANNER_SHA256" "$work/osv-scanner"
    install -m 0755 "$work/osv-scanner" "$AM_TOOLS_DIR/osv-scanner"
}

install_shellcheck() {
    fetch "https://github.com/koalaman/shellcheck/releases/download/v${SHELLCHECK_VERSION}/shellcheck-v${SHELLCHECK_VERSION}.linux.x86_64.tar.xz" \
        "$SHELLCHECK_SHA256" "$work/shellcheck.txz"
    tar -xJf "$work/shellcheck.txz" -C "$work"
    install -m 0755 "$work/shellcheck-v${SHELLCHECK_VERSION}/shellcheck" "$AM_TOOLS_DIR/shellcheck"
}

install_govulncheck() {
    # The Go module proxy and checksum database verify the download.
    GOBIN="$AM_TOOLS_DIR" GOFLAGS='' go install "golang.org/x/vuln/cmd/govulncheck@${GOVULNCHECK_VERSION}"
}

install_yamllint() {
    local venv="$AM_TOOLS_DIR/../yamllint-venv"
    python3 -m venv "$venv"
    "$venv/bin/pip" install --quiet --disable-pip-version-check "yamllint==${YAMLLINT_VERSION}"
    ln -sf "$venv/bin/yamllint" "$AM_TOOLS_DIR/yamllint"
}

tools=("$@")
[ ${#tools[@]} -gt 0 ] || tools=(actionlint gitleaks osv-scanner shellcheck govulncheck yamllint)
for tool in "${tools[@]}"; do
    echo "installing $tool into $AM_TOOLS_DIR"
    "install_$tool"
done

if [ -n "${GITHUB_PATH:-}" ]; then
    echo "$AM_TOOLS_DIR" >> "$GITHUB_PATH"
fi
