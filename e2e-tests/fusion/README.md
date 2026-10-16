# Fusion end-to-end tests

Playwright smoke tests of Antimatter's Fusion web UI, run against a live server. They drive the UI the way people
do, through roles and labels, and cover the main flows:

| Spec | What it checks |
| --- | --- |
| `auth.spec.ts` | Logging in with the form and out again; a wrong password is refused. |
| `navigation.spec.ts` | The communities rail, the channel sidebar, switching channel. |
| `messages.spec.ts` | Sending, editing and deleting a message (and cancelling a delete), replying in a thread, reacting and taking a reaction back, reacting with an emoji found in the picker. |
| `profile.spec.ts` | A click on someone's name opens their profile card and a second click closes it; a right-click opens their menu and a second right-click closes it; the menu's moderation entries for an admin; mentioning someone from the menu. |
| `emoji.spec.ts` | The composer's emoji picker searches and inserts an emoji; the custom emoji dialog opens from the picker. |
| `search.spec.ts` | Finding a message by a word in it. |
| `settings.spec.ts` | Opening the settings and moving between their pages. |
| `phone.spec.ts` | A 390-pixel-wide phone: one pane at a time, the navigation in a drawer, no sideways scrolling, sending a message. |
| `permissions.spec.ts` | A member without admin rights has no System Console entry, is turned away from `/admin_console`, gets no moderation entries in an admin's menu and is refused `/api/v4/config`; an admin opens the System Console. |

## What a run creates, and cleans up

The global setup (`global_setup.ts`) logs the admin in through the API, picks the community, and creates:

- a channel `e2e-<time>` that every test works in;
- a member without admin rights, `e2e-member-<time>`, unless you give one (`AM_E2E_MEMBER_USER`).

After each test, the messages in the test channel are deleted. The global teardown (`global_teardown.ts`) deletes the
channel (or archives it when the server keeps deleted channels, the default), removes the member it created
(deactivates them unless `ServiceSettings.EnableAPIUserDeletion` is on) and logs both sessions out. A run that is
killed half-way may leave its `e2e-*` channel behind.

Running the suite also changes small things for the admin, as using the app does: their recently used emoji, and the
preference that keeps Fusion from offering them a community layout in the test community.

## Settings

| Variable | Default | What it is |
| --- | --- | --- |
| `AM_E2E_BASE_URL` | `http://localhost:8067` | The server. |
| `AM_E2E_USER` | `maya` | An admin's username or email. |
| `AM_E2E_PASSWORD` | `Antimatter-Local-1` | Their password. |
| `AM_E2E_TEAM` | the admin's first community by name | The community (team name) to work in. |
| `AM_E2E_MEMBER_USER`, `AM_E2E_MEMBER_PASSWORD` | unset: a member is created | A member without admin rights, for the permission tests. |
| `AM_E2E_CHROMIUM` | Playwright's own Chromium | The path of a Chromium or Chrome executable to use instead. |

The defaults point at the local test server (amdev, the "Lumen Labs" demo).

## Running it

You need Node.js 20 or later and a running Antimatter server whose default web UI is Fusion.

```sh
cd e2e-tests/fusion
npm ci
npx playwright install chromium   # once; skip it with AM_E2E_CHROMIUM
AM_E2E_BASE_URL=http://localhost:8065 AM_E2E_USER=admin AM_E2E_PASSWORD='...' npx playwright test
```

- One spec: `npx playwright test specs/messages.spec.ts`; one test: `npx playwright test -g "replies in a thread"`.
- Watch it: `npx playwright test --headed`, or step through with `--debug`.
- After a failure, `npx playwright show-report` opens the HTML report, with a screenshot and a trace of each failed
  test (`npx playwright show-trace test-results/<test>/trace.zip`).
- The tests run one at a time (one worker): they share the server, the channel and the admin.

### Against the local test server (amdev)

The defaults already point at amdev (`http://localhost:8067`, maya). Use a demo member for the permission tests, so the
run doesn't leave a deactivated `e2e-member-*` account in the demo:

```sh
AM_E2E_MEMBER_USER=tom AM_E2E_MEMBER_PASSWORD=Antimatter-Local-1 npx playwright test
```

amdev runs a recent build, which may lag behind your branch: a test of a fix that isn't on amdev yet fails there.

### On this machine (memory and worktrees)

The machine has no swap: run Playwright through the memory-capped queue, and don't `npm install` inside a linked
worktree. Playwright 1.64 is installed (offline, from the npm cache) in `~/.cache/amtest/pw-fusion`; link it instead:

```sh
cd e2e-tests/fusion
ln -sfn ~/.cache/amtest/pw-fusion/node_modules node_modules
AM_E2E_MEMBER_USER=tom AM_E2E_MEMBER_PASSWORD=Antimatter-Local-1 MEM=3 ~/.cache/amtest/heavy.sh npx playwright test
```

## In CI

`.github/workflows/antimatter-fusion-e2e.yml` builds the server and Fusion from the commit, starts them on Postgres,
seeds an admin (`e2e-admin`) and a community (`e2e`) with amctl, then runs `npx playwright test` here with
`AM_E2E_BASE_URL`, `AM_E2E_USER` and `AM_E2E_PASSWORD`. The HTML report, the traces and the server log are uploaded
as the `fusion-e2e-report` artifact.

## Writing tests

- Find elements by role and accessible name (`getByRole('button', {name: 'Reply in thread'})`), as a screen reader
  would. Fusion's `.am-msg` (a message) and `.am-author` (its author's name) are the only classes the helpers use.
- Use `lib/fixtures.ts`: `fusion` opens the test channel and sends, finds and acts on messages; `admin` and `member`
  call the REST API; `uniq()` makes a word no other message has.
- Post only in the test channel (it is cleared after each test). Anything else a test creates, it removes itself.
- Keep tests independent of the demo data: create what you need through the API.
- A behaviour that depends on permissions gets both sides: what the member can't see or do, and what the admin can.
