# Testing Antimatter

Antimatter aims to be software people can trust with their conversations, so its code is checked on every
push: linters, the full test suites with coverage, security and supply-chain scans, and checks of
Antimatter's own rules (no Mattermost branding, no calls to Mattermost's servers). This page says what runs,
where, and how to run the same checks on your machine.

## The rule: every change comes with its tests

- **Every feature and every bug fix comes with tests that would fail without it**: Go unit or integration
  tests for the server and plugins, jest / React Testing Library tests for Fusion and the classic web app,
  and a Playwright test when it's a user flow. A change isn't finished until its tests are.
- **Security, privacy, permissions (RBAC), migrations and defaults always get explicit regression tests**:
  who can do what with the default roles, who may promote whom, what a migration grants, what a new setting
  defaults to, which outside hosts the code contacts. An update must never, say, turn every user into an
  admin, and a test must catch it if it would.
- **Fix a failing test; don't hide it.** Fix the test or the code, whichever is wrong. Delete a test only
  when the feature it covers was removed, and say so in the commit. Never add a test to
  `scripts/antimatter/known-failures.txt` to turn a red build green: that list is temporary and only holds
  tests that already failed before CI was set up.
- New user-visible strings say "Antimatter", never "Mattermost", and are translated into the 37 supported
  languages before they ship (the policy check enforces both).

## Running the checks yourself

Everything goes through one script, `scripts/antimatter/check.sh`, which runs what CI runs:

| Command | What it does | Needs | Time |
| --- | --- | --- | --- |
| `check.sh policy` | Branding, outbound hosts, `AM_` names, translations | Python 3 | seconds |
| `check.sh lint workflows` | actionlint and yamllint on `.github/workflows` | `check.sh tools` | seconds |
| `check.sh lint web` | eslint, stylelint and tsc for Fusion, classic and the platform packages | `webapp/node_modules` | 10-20 min |
| `check.sh lint server` | golangci-lint with upstream's configuration (`server/.golangci.yml`) | Go | 10-20 min |
| `check.sh test go ./channels/app/... -run TestFoo` | Go tests of the packages you name (from `server/`) | Go, Postgres | varies |
| `check.sh test fusion src/fusion/teams` | jest on the paths you name (`classic` for `webapp/channels`) | `webapp/node_modules` | varies |
| `check.sh test all` | Every suite: Go, Fusion, classic, platform | all of the above | 2-3 hours |
| `check.sh coverage [go\|web\|all]` | The suites with coverage, then the summary and HTML reports | as `test all` | 2-3 hours |
| `check.sh security` | gitleaks (working tree), govulncheck, osv-scanner, npm audit | `check.sh tools`, Go, npm | 5-10 min |
| `check.sh reports` | Dependency, vulnerability and privacy reports | `check.sh tools`, Go | 5-10 min |
| `check.sh e2e` | The Fusion Playwright suite against a running server | a server, Node | 5-15 min |
| `check.sh services up` / `down` | Starts or stops the Go tests' Postgres, MinIO, Inbucket and Azurite | Docker | 1 min |
| `check.sh tools` | Installs the pinned scanners in `~/.cache/antimatter/tools/bin` | Linux x86_64 | 1 min |
| `check.sh all` | policy, lint, security, reports, then `test all` | everything | 3+ hours |

Reports are written to `.am-reports/` (ignored by git; set `AM_REPORT_DIR` to change it).

**Prerequisites.** Go (the version in `server/go.mod`), Node and npm (`.nvmrc`), Python 3, Docker for the
test services, and `webapp/node_modules` (`cd webapp && npm ci`). `check.sh tools` installs actionlint,
yamllint, shellcheck, gitleaks, osv-scanner and govulncheck at pinned versions, checking each download's
SHA-256 (Linux x86_64; elsewhere, install the same versions and put them on `PATH`).

**Database for the Go tests.** `check.sh services up` starts the services on localhost (Postgres on 5432;
move it with `AM_TEST_PG_PORT`). To use your own Postgres instead, set `TEST_DATABASE_POSTGRESQL_DSN`, for
example `postgres://mmuser:mostest_password@localhost:5433/mattermost_test?sslmode=disable`.

**Memory.** The server's tests, golangci-lint, tsc, webpack and jest each need several GiB. On a machine
where `~/.cache/amtest/heavy.sh` exists, `check.sh` runs every heavy step through it (one heavy job at a
time, memory-capped; `AM_HEAVY_MEM` sets the cap in GiB, default 5). Elsewhere they run directly: run one at
a time on a small machine. Prefer targeted runs (`test go <package> -run <Test>`, `test fusion <path>`); CI
runs the full suites.

### The Fusion end-to-end suite (Playwright)

The suite lives in `e2e-tests/fusion` and reads three variables:

```sh
export AM_E2E_BASE_URL=http://localhost:8065   # the server to test
export AM_E2E_USER=e2e-admin                   # a system admin of that server
export AM_E2E_PASSWORD='...'
scripts/antimatter/check.sh e2e                # or: cd e2e-tests/fusion && npx playwright test
```

`check.sh e2e` installs the suite's dependencies and Chromium the first time. Extra arguments go to
Playwright, e.g. `check.sh e2e --headed tests/login.spec.ts`. Run it against a test server, never a
production one: the suite creates and changes data.

## Continuous integration

GitHub Actions runs Antimatter's workflows (`.github/workflows/antimatter-*.yml`). They use GitHub's own
features (CodeQL, dependency review, artifacts, job summaries) and open-source tools run inside the jobs: no
third-party service, account or token.

| Workflow | When | What fails it |
| --- | --- | --- |
| **Antimatter lint** | push to `main`, pull requests | policy violations, golangci-lint, `make vet`, the plugin API checker, server translations (`make i18n-verify`), eslint, stylelint, tsc, `en.json` out of sync with the code, actionlint, yamllint errors |
| **Antimatter tests** | push to `main`, pull requests | a failing Go or jest test that isn't on the known-failures list, a package that doesn't build |
| **Antimatter security** | push to `main`, pull requests, Mondays | CodeQL alerts (Security tab), a vulnerable function the server calls (govulncheck), a secret (gitleaks), a pull request adding a high or critical vulnerability (dependency review) |
| **Antimatter Fusion e2e** | push to `main`, pull requests touching the server, the web app or the suite, Mondays | a failing Playwright test |
| **Antimatter reports** | Mondays, on demand | nothing: it's a report |

**Tests.** The Go tests run in 6 shards against Postgres, MinIO, Inbucket, Redis and Azurite
(`scripts/antimatter/test-services.yml`), using upstream's shard solver; test durations from the last run on
`main` balance the shards. jest runs in 3 shards per web app. A failure listed in
`scripts/antimatter/known-failures.txt` is reported as a warning in the job summary; when a listed test
passes again, the summary says so and its line must be removed.

**Coverage.** The "Coverage report" job of Antimatter tests writes the statistics in its summary (server,
Fusion and its own UI in `src/fusion`, classic, platform packages, Antimatter-only server code, and per
package or directory) and uploads the `antimatter-coverage` artifact: HTML reports, `coverage.json`, the merged
Go profile and SVG badges.

**Weekly report.** Antimatter reports gathers, in one job summary and the `antimatter-report` artifact:
coverage, known vulnerabilities in the dependencies (govulncheck, osv-scanner, npm audit), outdated,
deprecated, archived or unmaintained dependencies (no release for over two years), secrets anywhere in the
git history, and the **privacy report**: every outside host the code names, sorted into what the server or
the browser may contact on its own, default settings, links people click, and placeholders.

**Policy checks** (`scripts/antimatter/policy_check.py`): violations that predate the checks are listed in
`scripts/antimatter/policy-baseline.txt` (still to fix: fix them, then run
`scripts/antimatter/policy_check.py --update-baseline` to drop them). Legitimate mentions, such as copyright
lines, "Mattermost, Inc." named as the company running a service, or detection constants, go in
`scripts/antimatter/policy-allowlist.txt`, each with a comment saying why. Anything else fails the build,
with a message saying what to do.

**Upstream's workflows.** The Mattermost workflows inherited from upstream are kept to ease merges. Those
that need Mattermost's infrastructure or secrets, or that Antimatter's workflows replace, only run when
`github.repository_owner == 'mattermost'`. The few that work here still run (tools CI, the API reference
build, the nightly race detector and its cache warm-up).
