# Antimatter

Antimatter is a self-hosted team chat server: channels, direct messages, threads, file sharing,
search, voice and video calls, and integrations through webhooks, slash commands, bots and plugins.
It is a fully open source fork of [Mattermost](https://github.com/mattermost/mattermost): every
feature is available without a license key, and it works with the Mattermost mobile and desktop
apps.

- Source code and issues: [github.com/AntimatterChat](https://github.com/AntimatterChat)
- Server: [github.com/AntimatterChat/antimatter](https://github.com/AntimatterChat/antimatter)
- Docker image with the bundled plugins:
  [github.com/AntimatterChat/antimatter-docker](https://github.com/AntimatterChat/antimatter-docker)

## Running Antimatter

The easiest way is the Docker image (`ghcr.io/antimatterchat/antimatter`); see the
antimatter-docker repository for a ready-to-use compose file.

To run a release archive (`antimatter-<os>-<arch>.tar.gz`):

1. Set up a PostgreSQL database and user.
2. Unpack the archive, e.g. into `/opt/antimatter`, and point `SqlSettings.DataSource` in
   `config/config.json` (or the `MM_SQLSETTINGS_DATASOURCE` environment variable) at the database.
3. Start the server with `bin/antimatter`, as an unprivileged user, and open
   http://localhost:8065. The first account created becomes the system administrator.

`bin/amctl` manages a running server from the command line (`bin/amctl --local` on the server
itself, with `ServiceSettings.EnableLocalMode`).

## Building from source

The server is written in Go (`server/`) and the web app in React (`webapp/`). `make build` and
`make package-linux` in `server/` build the server and its release archives; see
[CONTRIBUTING.md](CONTRIBUTING.md) for a development setup.

## License

Antimatter is free software. The server is licensed under the GNU Affero General Public License
v3.0; the admin tools and configuration files (`server/templates/`, `server/i18n/`,
`server/public/`, `webapp/`) are licensed under the Apache License 2.0. See
[LICENSE.txt](LICENSE.txt) and [NOTICE.txt](NOTICE.txt).

Antimatter is based on Mattermost, Copyright 2015-present Mattermost, Inc. Antimatter is not
affiliated with or endorsed by Mattermost, Inc.; Mattermost is a trademark of Mattermost, Inc.
