#!/usr/bin/env bash
# One entry point for Antimatter's checks, the same ones CI runs. See TESTING.md.
#
#   scripts/antimatter/check.sh <target> [args]
#
#   policy                      branding, outbound hosts, AM_ names, translations (seconds)
#   lint [server|web|workflows] golangci-lint, eslint + tsc, actionlint + yamllint (default: all of them)
#   test go <packages...>       Go tests, e.g. `test go ./channels/app/... -run TestFoo` (from server/)
#   test fusion|classic [args]  jest, e.g. `test fusion src/fusion/teams`
#   test all                    every suite: Go (about 1-2 hours), Fusion, classic and platform jest
#   coverage [go|web|all]       the full suites with coverage, then the coverage summary in the reports dir
#   security                    govulncheck, osv-scanner, npm audit, gitleaks (secrets in the working tree)
#   reports                     dependency, vulnerability and privacy reports (+ coverage if already run)
#   e2e [playwright args]       the Fusion Playwright suite against $AM_E2E_BASE_URL
#   services up|down            start/stop the Go tests' Postgres, MinIO, Inbucket and Azurite (docker compose)
#   tools                       install the pinned scanners (actionlint, gitleaks, osv-scanner...)
#   all                         policy, lint, security, reports, then `test all`
#
# Heavy steps go through ~/.cache/amtest/heavy.sh when it exists (memory-capped queue, one job at a time);
# set AM_HEAVY_MEM (GiB, default 5) to change the cap. Reports land in $AM_REPORT_DIR (default .am-reports/).

set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
HERE="$ROOT/scripts/antimatter"
export AM_TOOLS_DIR="${AM_TOOLS_DIR:-$HOME/.cache/antimatter/tools/bin}"
export PATH="$AM_TOOLS_DIR:$PATH"
[ -d "$HOME/.cache/amtest/bin" ] && export PATH="$PATH:$HOME/.cache/amtest/bin"
REPORT_DIR="${AM_REPORT_DIR:-$ROOT/.am-reports}"
mkdir -p "$REPORT_DIR"

if [ -z "${GITHUB_TOKEN:-}" ] && command -v gh >/dev/null 2>&1; then
    GITHUB_TOKEN="$(gh auth token 2>/dev/null || true)"
    export GITHUB_TOKEN
fi

heavy() {
    if [ -x "$HOME/.cache/amtest/heavy.sh" ]; then
        MEM="${AM_HEAVY_MEM:-5}" "$HOME/.cache/amtest/heavy.sh" "$@"
    else
        "$@"
    fi
}

need() {
    command -v "$1" >/dev/null 2>&1 && return 0
    echo "$1 is not installed: run \`scripts/antimatter/check.sh tools\` (or install it yourself)." >&2
    return 1
}

section() { printf '\n=== %s\n' "$*"; }

# ---------------------------------------------------------------------------------------------------- targets

run_policy() {
    section "Antimatter policy checks"
    python3 "$HERE/policy_check.py" --summary "$REPORT_DIR/policy.md"
}

lint_server() {
    section "golangci-lint (server)"
    (cd "$ROOT/server" && heavy make golangci-lint)
}

lint_web() {
    local ws
    for ws in fusion channels; do
        section "eslint + stylelint + tsc ($ws)"
        (cd "$ROOT/webapp/$ws" && heavy env NODE_OPTIONS=--max-old-space-size=4096 npm run check \
            && heavy env NODE_OPTIONS=--max-old-space-size=4096 npm run check-types)
    done
    section "eslint + tsc (platform packages)"
    (cd "$ROOT/webapp" && heavy npm run check --workspace=platform/client --workspace=platform/types \
        --workspace=platform/components --workspace=platform/shared \
        && heavy npm run check-types --workspace=platform/components --workspace=platform/shared)
}

lint_workflows() {
    section "actionlint + yamllint (.github/workflows)"
    need actionlint && need yamllint
    (cd "$ROOT" && actionlint && yamllint .github/workflows/)
}

run_lint() {
    case "${1:-all}" in
        server) lint_server ;;
        web) lint_web ;;
        workflows) lint_workflows ;;
        all) lint_workflows; lint_web; lint_server ;;
        *) echo "unknown lint target: $1" >&2; exit 2 ;;
    esac
}

test_env() {
    export TEST_DATABASE_POSTGRESQL_DSN="${TEST_DATABASE_POSTGRESQL_DSN:-postgres://mmuser:mostest_password@localhost:${AM_TEST_PG_PORT:-5432}/mattermost_test?sslmode=disable&connect_timeout=10}"
    export CI_MINIO_HOST="${CI_MINIO_HOST:-localhost}" CI_MINIO_PORT="${CI_MINIO_PORT:-${AM_TEST_MINIO_PORT:-9000}}"
    export CI_INBUCKET_HOST="${CI_INBUCKET_HOST:-localhost}" CI_INBUCKET_PORT="${CI_INBUCKET_PORT:-${AM_TEST_INBUCKET_PORT:-9001}}"
    export CI_INBUCKET_SMTP_PORT="${CI_INBUCKET_SMTP_PORT:-${AM_TEST_SMTP_PORT:-10025}}"
    export CI_AZURITE_HOST="${CI_AZURITE_HOST:-localhost}" CI_AZURITE_PORT="${CI_AZURITE_PORT:-${AM_TEST_AZURITE_PORT:-10000}}"
    # The default SMTP server, localhost:10025, is Inbucket's. Only a moved port needs an override, and that one
    # beats the tests that point SMTP somewhere unreachable (they fail then).
    if [ "$CI_INBUCKET_SMTP_PORT" != 10025 ] && [ -z "${AM_EMAILSETTINGS_SMTPPORT:-}" ]; then
        export AM_EMAILSETTINGS_SMTPSERVER="${AM_EMAILSETTINGS_SMTPSERVER:-$CI_INBUCKET_HOST}"
        export AM_EMAILSETTINGS_SMTPPORT="$CI_INBUCKET_SMTP_PORT"
    fi
}

jest_run() { # workspace dir, extra args...
    local dir="$1"; shift
    (cd "$ROOT/webapp/$dir" && heavy env TZ=Etc/UTC LC_ALL=en_US.UTF-8 LANG=en_US.UTF-8 \
        NODE_OPTIONS=--max-old-space-size=4096 npx jest --maxWorkers=2 "$@")
}

run_test() {
    local what="${1:-}"; shift || true
    case "$what" in
        go)
            [ $# -gt 0 ] || { echo "usage: check.sh test go <packages...> [go test flags]" >&2; exit 2; }
            test_env
            (cd "$ROOT/server" && make setup-go-work >/dev/null && heavy go test -count=1 "$@")
            ;;
        fusion) jest_run fusion "$@" ;;
        classic) jest_run channels "$@" ;;
        all)
            echo "Running every suite: expect 1-2 hours for Go, 20-40 minutes per web app." >&2
            ENABLE_COVERAGE=false run_go_suite
            jest_run fusion; jest_run channels
            (cd "$ROOT/webapp" && heavy npm run test --workspace=platform/client --workspace=platform/components \
                --workspace=platform/shared)
            ;;
        *) echo "usage: check.sh test go <packages...> | fusion [args] | classic [args] | all" >&2; exit 2 ;;
    esac
}

run_go_suite() {
    section "Go server tests (needs \`check.sh services up\`)"
    test_env
    heavy env ENABLE_COVERAGE="${ENABLE_COVERAGE:-true}" "$HERE/server-tests.sh"
}

run_coverage() {
    local what="${1:-all}" go_profiles=() jest_args=()
    if [ "$what" = go ] || [ "$what" = all ]; then
        ENABLE_COVERAGE=true run_go_suite || true
        go_profiles=("$ROOT/server/cover.out")
    fi
    if [ "$what" = web ] || [ "$what" = all ]; then
        local ws area
        for ws in fusion channels; do
            area=$([ "$ws" = channels ] && echo classic || echo fusion)
            jest_run "$ws" --coverage --coverageReporters=json --coverageDirectory="$REPORT_DIR/jest-$area" || true
            node "$HERE/merge_jest_coverage.js" "$REPORT_DIR/coverage-$area" "$REPORT_DIR/jest-$area/coverage-final.json"
            jest_args+=(--jest "$area=$REPORT_DIR/coverage-$area/coverage-summary.json")
        done
    fi
    python3 "$HERE/coverage_summary.py" --out "$REPORT_DIR" --go "${go_profiles[@]}" "${jest_args[@]}"
    if [ ${#go_profiles[@]} -gt 0 ] && [ -f "$REPORT_DIR/server-cover.out" ]; then
        (cd "$ROOT/server" && go tool cover -html="$REPORT_DIR/server-cover.out" -o "$REPORT_DIR/server-coverage.html")
    fi
    echo "Coverage reports: $REPORT_DIR"
}

scan_vulns() {
    section "Dependency vulnerabilities"
    need govulncheck && need osv-scanner
    local gv=()
    for mod in server server/public; do
        local name="${mod//\//-}"
        (cd "$ROOT/$mod" && heavy sh -c "govulncheck -format json ./... > '$REPORT_DIR/govulncheck-$name.json'") || true
        gv+=(--govulncheck "$mod=$REPORT_DIR/govulncheck-$name.json")
    done
    osv-scanner scan source -L "$ROOT/webapp/package-lock.json" -L "$ROOT/server/go.mod" -L "$ROOT/server/public/go.mod" \
        --format json > "$REPORT_DIR/osv-scanner.json" 2>/dev/null || true
    (cd "$ROOT/webapp" && npm audit --package-lock-only --json > "$REPORT_DIR/npm-audit.json" 2>/dev/null || true)
    (cd "$ROOT/webapp" && npm audit --package-lock-only --omit=dev --json > "$REPORT_DIR/npm-audit-prod.json" 2>/dev/null || true)
    python3 "$HERE/vuln_report.py" --out "$REPORT_DIR" "${gv[@]}" --osv "$REPORT_DIR/osv-scanner.json" \
        --npm-audit "all dependencies=$REPORT_DIR/npm-audit.json" \
        --npm-audit "shipped (no dev dependencies)=$REPORT_DIR/npm-audit-prod.json" --fail-on-called
}

scan_secrets() {
    section "Secrets (gitleaks)"
    need gitleaks
    gitleaks dir "$ROOT" --config "$ROOT/.gitleaks.toml" --redact --no-banner \
        --report-format json --report-path "$REPORT_DIR/gitleaks.json"
}

run_security() {
    local status=0
    scan_secrets || status=1
    scan_vulns || status=1
    return $status
}

run_reports() {
    section "Dependencies (outdated, unmaintained)"
    python3 "$HERE/deps_report.py" --out "$REPORT_DIR"
    section "Privacy (outside addresses)"
    python3 "$HERE/privacy_report.py" --out "$REPORT_DIR"
    scan_vulns || true
    echo "Reports: $REPORT_DIR"
}

run_e2e() {
    local dir="$ROOT/e2e-tests/fusion"
    [ -f "$dir/package.json" ] || { echo "No Playwright suite in e2e-tests/fusion yet." >&2; exit 1; }
    : "${AM_E2E_BASE_URL:=http://localhost:8065}"
    [ -n "${AM_E2E_USER:-}" ] && [ -n "${AM_E2E_PASSWORD:-}" ] \
        || { echo "Set AM_E2E_USER and AM_E2E_PASSWORD (an admin of the server at $AM_E2E_BASE_URL)." >&2; exit 2; }
    export AM_E2E_BASE_URL AM_E2E_USER AM_E2E_PASSWORD
    if [ ! -d "$dir/node_modules" ]; then
        (cd "$dir" && if [ -f package-lock.json ]; then heavy npm ci; else heavy npm install; fi)
        (cd "$dir" && npx playwright install chromium)
    fi
    (cd "$dir" && heavy npx playwright test "$@")
}

run_services() {
    local compose=(docker compose -f "$HERE/test-services.yml")
    case "${1:-up}" in
        up)
            "${compose[@]}" up -d --wait
            "${compose[@]}" exec -T minio sh -c 'mkdir -p /data/mattermost-test'
            ;;
        down) "${compose[@]}" down -v ;;
        *) echo "usage: check.sh services up|down" >&2; exit 2 ;;
    esac
}

target="${1:-}"; shift || true
case "$target" in
    policy) run_policy ;;
    lint) run_lint "$@" ;;
    test) run_test "$@" ;;
    coverage) run_coverage "$@" ;;
    security) run_security ;;
    reports) run_reports ;;
    e2e) run_e2e "$@" ;;
    services) run_services "$@" ;;
    tools) "$HERE/install-tools.sh" "$@" ;;
    all)
        status=0
        run_policy || status=1
        run_lint || status=1
        run_security || status=1
        run_reports || status=1
        run_test all || status=1
        exit $status
        ;;
    *) sed -n '2,22p' "$0" | sed 's/^# \{0,1\}//'; exit 2 ;;
esac
