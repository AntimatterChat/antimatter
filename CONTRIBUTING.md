# Contributing to Antimatter

Thank you for your interest in contributing! Bug reports, questions and ideas go to
[AntimatterChat/community](https://github.com/AntimatterChat/community), which collects the issues and
discussions of every Antimatter project. The documentation is at <https://antimatter.chat/docs/>.

## Tests come with every change

Every feature and every bug fix comes with tests that would fail without it: Go tests for the server, jest
tests for the web apps, and a Playwright test for user flows. Changes to security, privacy, permissions
(RBAC), migrations or default settings always get explicit regression tests. [TESTING.md](TESTING.md)
explains the rule in full, what CI checks, and how to run the same checks locally with
`scripts/antimatter/check.sh`.

Before opening a pull request, run at least:

```sh
scripts/antimatter/check.sh policy               # branding, outbound hosts, AM_ names, translations
scripts/antimatter/check.sh test go <packages>   # and/or: test fusion <paths>, test classic <paths>
```

## A few rules

- User-visible text says "Antimatter", never "Mattermost", and new strings are translated into the 37
  supported languages.
- Antimatter never contacts Mattermost, Inc. and ships no telemetry: no new calls to `*.mattermost.com` or
  analytics services.
- Write environment variables with the `AM_` prefix in docs and examples (`MM_` still works as the older name).
- Keep the existing copyright headers and licences (`LICENSE.txt`, `NOTICE.txt`).
