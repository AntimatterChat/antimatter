#!/usr/bin/env bash
# Runs the server's Go test suite, or one shard of it, with coverage, then judges the results.
#
#   SHARD_INDEX=0 SHARD_TOTAL=6 scripts/antimatter/server-tests.sh
#
# It reuses upstream's shard solver (server/scripts/shard-split.js: whole light packages, the tests of the
# heavy api4/app packages spread by -run filters) and runner (server/scripts/run-shard-tests.sh: gotestsum
# with --rerun-fails, per-shard cover.out). server/prev-gotestsum.json, when present (CI restores it from the
# last run on main), balances the shards by duration.
#
# Environment: SHARD_INDEX (default 0), SHARD_TOTAL (default 1), ENABLE_COVERAGE (default true),
# TEST_DATABASE_POSTGRESQL_DSN and the CI_* service variables (see test-services.yml), AM_RESULTS_LABEL.
# Exit status: 0 when every failure is on the temporary known-failures list (see test_results.py).

set -uo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
cd "$ROOT/server" || exit 1

export SHARD_INDEX="${SHARD_INDEX:-0}" SHARD_TOTAL="${SHARD_TOTAL:-1}"
export ENABLE_COVERAGE="${ENABLE_COVERAGE:-true}" IS_CI="${IS_CI:-true}"
export ENABLE_FULLY_PARALLEL_TESTS="${ENABLE_FULLY_PARALLEL_TESTS:-true}"
export GOTESTSUM_FORMAT="${GOTESTSUM_FORMAT:-testname}"

make setup-go-work >/dev/null || exit 1

# Same package list as upstream's server-test-template: every package with tests, but not the CLI's
# end-to-end tests (cmd/mmctl, run separately upstream).
find ./public/ ./ -name '*_test.go' -not -path './enterprise/*' -not -path './cmd/mmctl/*' -not -path './cmd/amctl/*' \
    | sed 's|/[^/]*$||' | sort -u \
    | sed 's|^\./|github.com/mattermost/mattermost/server/v8/|' \
    | sed 's|github.com/mattermost/mattermost/server/v8/public/|github.com/mattermost/mattermost/server/public/|' \
    | sort -u > all-packages.txt
echo "$(wc -l < all-packages.txt) test packages; running shard $((SHARD_INDEX + 1)) of $SHARD_TOTAL"

rm -f report-*.xml gotestsum-*.json cover-*.out gotestsum.json report.xml cover.out shard-*.txt
node scripts/shard-split.js || exit 1

status=0
if [ -s shard-te-packages.txt ] || [ -s shard-ee-packages.txt ] || [ -s shard-heavy-runs.txt ]; then
    bash scripts/run-shard-tests.sh || status=$?
else
    echo "This shard has no tests."
    exit 0
fi
echo "gotestsum exit status: $status (the known-failures check below decides)"

python3 "$ROOT/scripts/antimatter/test_results.py" go \
    --label "${AM_RESULTS_LABEL:-Server tests (shard $((SHARD_INDEX + 1))/$SHARD_TOTAL)}" gotestsum.json
