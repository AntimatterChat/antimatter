#!/bin/sh
# antimatter package: runs before the files are removed (deb prerm, rpm %preun). Stops and
# disables the service on removal, not on upgrades.
set -e

# deb: "remove", "upgrade", ...; rpm: 0 (erase) or 1 (upgrade).
case "$1" in
    remove | 0) ;;
    *) exit 0 ;;
esac

if [ -d /run/systemd/system ]; then
    systemctl disable --now antimatter.service >/dev/null 2>&1 || true
fi
exit 0
