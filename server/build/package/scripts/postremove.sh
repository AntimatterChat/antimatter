#!/bin/sh
# antimatter package: runs after the files are removed (deb postrm, rpm %postun). The data in
# /var/lib/antimatter, the configuration in /etc/antimatter, the database and the antimatter user
# are kept: delete them by hand to remove Antimatter completely.
set -e

if [ -d /run/systemd/system ]; then
    systemctl daemon-reload >/dev/null 2>&1 || true
fi
case "$1" in
    remove | 0)
        echo "Antimatter was removed; its data (/var/lib/antimatter), configuration (/etc/antimatter) and database are kept."
        ;;
esac
exit 0
