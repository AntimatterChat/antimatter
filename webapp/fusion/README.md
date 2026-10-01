# Classic and Fusion web UIs

Antimatter ships two web UIs:

- **Classic** (`webapp/channels`): the web app inherited from Mattermost.
- **Fusion** (`webapp/fusion`): the new web UI, in preview. It started as a copy of the classic
  web app and diverges from it.

Both talk to the same server API, and both load the same web app plugins.

## Choosing the web UI

| Setting | Env var | Default | Meaning |
| --- | --- | --- | --- |
| `ServiceSettings.DefaultWebUI` | `MM_SERVICESETTINGS_DEFAULTWEBUI` | `classic` | Web UI served to users who didn't pick one: `classic` or `fusion`. |
| `ServiceSettings.AllowUserWebUISelection` | `MM_SERVICESETTINGS_ALLOWUSERWEBUISELECTION` | `true` | Whether users can pick their own web UI. |

Both are in **System Console > Site Configuration > Customization**.

When users may choose, they pick a web UI in **Settings > Display > Web interface**. The choice
is saved for the current browser in the `AMWEBUI` cookie. Opening any page with
`?webui=classic` or `?webui=fusion` does the same, which is also a way back if one UI is broken.

When the Fusion UI isn't deployed (no `client-fusion` directory), the classic web app is always
served and users aren't offered a choice.

## How it's served

- Classic assets live in `client/` and are served under `/static/`.
- Fusion assets live in `client-fusion/` and are served under `/static/fusion/`.
- For every page, the server picks which `root.html` to return from the cookie and the settings
  above, and sets `Vary: Cookie` on the response.
- Fusion's webpack public path is `/static/fusion/`. `window.publicPath` still points at the
  shared `/static/` root, so subpath rewriting and `window.basename` work the same in both UIs.

## Building and running

- `npm run build` in `webapp/` builds the classic web app, then the Fusion UI (one after the
  other, to limit peak memory).
- `make client` in `server/` links `client` and `client-fusion` to the two build outputs.
- `WATCH_FUSION=true npm run run` also rebuilds the Fusion UI on changes.
- Fusion's dev server listens on port 9006; classic's listens on 9005.
- Release packages ship both directories.

Webpack's minifier starts one worker per CPU. On machines with many cores and little RAM,
limit the CPUs the build sees, e.g. `taskset -c 0-5 npm run build`.
