# Antimatter

Antimatter is a self-hosted team chat server: channels, direct messages, threads, file sharing,
search, voice and video calls, and integrations through webhooks, slash commands, bots and plugins.
It is a fully open source fork of [Mattermost](https://github.com/mattermost/mattermost): every
feature is available without a license key, and it works with the Mattermost mobile and desktop
apps.

- Source code: [github.com/AntimatterChat](https://github.com/AntimatterChat)
- Bug reports, feature requests and questions: [AntimatterChat/community](https://github.com/AntimatterChat/community)
- Server: [github.com/AntimatterChat/antimatter](https://github.com/AntimatterChat/antimatter)
- Docker image with the bundled plugins:
  [github.com/AntimatterChat/antimatter-docker](https://github.com/AntimatterChat/antimatter-docker)
- Documentation for users, admins and developers: [antimatter.chat/docs](https://antimatter.chat/docs/)

## Running Antimatter

The easiest way is the Docker image (`ghcr.io/antimatterchat/antimatter`); see the
antimatter-docker repository for a ready-to-use compose file.

Each release (`vX.Y.Z` tag) on the [releases page](https://github.com/AntimatterChat/antimatter/releases)
has, for Linux `amd64` and `arm64`, a release archive (`antimatter-X.Y.Z-linux-<arch>.tar.gz`) and
all-in-one `.deb` and `.rpm` packages with the bundled apps, plus a `SHA256SUMS` file. The
packages install the server in `/opt/antimatter` with a systemd service, `antimatter.service`
(enabled, not started), running as the `antimatter` user; the database goes in
`/etc/antimatter/antimatter.env` (`AM_SQLSETTINGS_DATASOURCE`), the data in `/var/lib/antimatter`,
the logs in the journal. PostgreSQL isn't included:

```sh
sudo apt install ./antimatter_X.Y.Z-1_amd64.deb        # Debian, Ubuntu
sudo dnf install ./antimatter-X.Y.Z-1.x86_64.rpm       # Fedora, RHEL, Rocky, Alma
```

To run a release archive instead:

1. Set up a PostgreSQL database and user.
2. Unpack the archive, e.g. into `/opt/antimatter`, and point `SqlSettings.DataSource` in
   `config/config.json` (or the `AM_SQLSETTINGS_DATASOURCE` environment variable) at the database.
3. Start the server with `bin/antimatter`, as an unprivileged user, and open
   http://localhost:8065. The first account created becomes the system administrator.

`bin/amctl` manages a running server from the command line (`bin/amctl --local` on the server
itself, with `ServiceSettings.EnableLocalMode`).

## Building from source

The server is written in Go (`server/`) and the web app in React (`webapp/`). `make build` and
`make package-linux` in `server/` build the server and its release archives, and
`server/build/package/build-packages.sh` the `.deb` and `.rpm` packages (the
`antimatter-release.yml` workflow runs them all on release tags); see
[CONTRIBUTING.md](CONTRIBUTING.md) for a development setup.

## Feedback and support

Found a bug in the Antimatter server or its web apps, or missing a feature? Please [open an
issue](https://github.com/AntimatterChat/community/issues/new/choose) in the [Antimatter community
repository](https://github.com/AntimatterChat/community), where the feedback for every part of
Antimatter is gathered. Questions and ideas are welcome in its
[Discussions](https://github.com/AntimatterChat/community/discussions). Please report security
problems privately, as explained in the [security
policy](https://github.com/AntimatterChat/.github/blob/main/SECURITY.md).

## License

Antimatter is free software. The server is licensed under the GNU Affero General Public License
v3.0; the admin tools and configuration files (`server/templates/`, `server/i18n/`,
`server/public/`, `webapp/`) are licensed under the Apache License 2.0. See
[LICENSE.txt](LICENSE.txt) and [NOTICE.txt](NOTICE.txt).

Antimatter is based on Mattermost, Copyright 2015-present Mattermost, Inc. Antimatter is not
affiliated with or endorsed by Mattermost, Inc.; Mattermost is a trademark of Mattermost, Inc.
