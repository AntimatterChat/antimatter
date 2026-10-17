# config.json

This is the configuration file of your Antimatter server. The server reads it at startup and saves
the changes made in the System Console back to it. It holds passwords and keys: keep it readable by
the server's user only. Settings left out of it take their default value.

You can change the settings in three ways:

- In the System Console: open the profile menu and choose *System Console*, or go to
  `https://<your site>/admin_console`.
- With `amctl`, for example `amctl config set TeamSettings.EnableOpenServer false`.
- With environment variables: write `AM_`, then the section, then the key, in capital letters and
  separated by underscores. For example, `SqlSettings.DataSource` is `AM_SQLSETTINGS_DATASOURCE`
  and `ServiceSettings.SiteURL` is `AM_SERVICESETTINGS_SITEURL`. A variable wins over this file and
  locks the setting in the System Console. The .deb and .rpm packages read these variables from
  `/etc/antimatter/antimatter.env`. The older `MM_` prefix still works; when both are set, the
  `AM_` variable wins.

When `AM_CONFIG` is set to a `postgres://` URL, the server keeps its configuration in the database
and doesn't use this file.

Configuration guide: https://antimatter.chat/docs/admins/configuration.html
Installation guide: https://antimatter.chat/docs/admins/install.html
