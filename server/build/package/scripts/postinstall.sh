#!/bin/sh
# antimatter package: runs after the files are installed or upgraded (deb postinst, rpm %post).
# Creates the antimatter system user and the directories the server writes, installs the default
# configuration at the first install and enables the service without starting it.
set -e

# deb: "configure" [<previous version>]; rpm: 1 (install) or 2 (upgrade).
first=
case "$1" in
    configure) [ -n "$2" ] || first=1 ;;
    1) first=1 ;;
    2) ;;
    *) exit 0 ;;
esac

user=antimatter
nologin=/usr/sbin/nologin
[ -x "$nologin" ] || nologin=/sbin/nologin
getent group "$user" >/dev/null 2>&1 || groupadd --system "$user"
getent passwd "$user" >/dev/null 2>&1 ||
    useradd --system --gid "$user" --home-dir /var/lib/antimatter --no-create-home \
        --shell "$nologin" --comment "Antimatter server" "$user"

# /opt/antimatter/{data,plugins,client/plugins,logs,config} link to these.
install -d -o "$user" -g "$user" -m 0750 /var/lib/antimatter /var/lib/antimatter/data \
    /var/lib/antimatter/plugins /var/lib/antimatter/client-plugins /var/log/antimatter \
    /etc/antimatter/config
# The server saves the System Console's changes to config.json: it's created once, from the
# package's defaults, and belongs to the server from then on.
config=/etc/antimatter/config/config.json
if [ ! -e "$config" ]; then
    install -o "$user" -g "$user" -m 0600 /opt/antimatter/defaults/config.json "$config"
fi
install -o root -g root -m 0644 /opt/antimatter/defaults/README.md /etc/antimatter/config/README.md
chmod 0600 /etc/antimatter/antimatter.env
# The server rewrites the web apps' assets when the site URL has a path (https://example.com/chat).
chown -R "$user:$user" /opt/antimatter/client /opt/antimatter/client-fusion

if [ -d /run/systemd/system ]; then
    systemctl daemon-reload >/dev/null 2>&1 || true
    if [ -n "$first" ]; then
        systemctl enable antimatter.service >/dev/null 2>&1 || true
    fi
fi

if [ -n "$first" ]; then
    cat <<'MSG'

Antimatter is installed in /opt/antimatter and enabled, but not started. Next steps:
  1. Create a PostgreSQL (15 or newer) user and database for it:
       sudo -u postgres psql -c "CREATE USER antimatter WITH PASSWORD 'a-long-password'"
       sudo -u postgres psql -c "CREATE DATABASE antimatter OWNER antimatter"
  2. Set the database in /etc/antimatter/antimatter.env:
       AM_SQLSETTINGS_DATASOURCE=postgres://antimatter:a-long-password@localhost:5432/antimatter?sslmode=disable&connect_timeout=10
     and the site URL (AM_SERVICESETTINGS_SITEURL=https://chat.example.com).
  3. Start it:   sudo systemctl start antimatter
     Logs:       journalctl -u antimatter -f
  4. Open http://<this server>:8065 and create the first account: it becomes the system admin.
Settings saved in the System Console are in /etc/antimatter/config/config.json, files and apps in
/var/lib/antimatter. Guide: https://antimatter.chat/docs/admins/install.html
MSG
else
    echo "Antimatter was upgraded; restart it to run the new version: sudo systemctl restart antimatter"
fi
exit 0
