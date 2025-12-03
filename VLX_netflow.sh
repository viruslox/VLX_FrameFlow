#!/bin/bash

PROFILE_NAME=$1
PROFILES_PATH="/etc/systemd/network/profiles"
# Sort all interfaces, pickup only the first wi-fi interface.
WIFI_IF=$(ls /sys/class/net/ | while read iface; do [ -d "/sys/class/net/$iface/wireless" ] && echo $iface && break; done)

## Validate input args
if [ -z "$PROFILE_NAME" ] || [ ! -d "$PROFILES_PATH/$PROFILE_NAME" ] ; then
    echo "[ERR]: Please provide a valid network profile name. Usage: $0 <nome_profilo>"
    echo "[INFO]: Available network profiles:"
    for profile in "$PROFILES_PATH"/*; do
        if [ -d "$profile" ]; then
            echo "  - $(basename "$profile")"
        fi
    done
    exit 1
fi

systemctl stop hostapd
systemctl stop "wpa_supplicant@$WIFI_IF.service"
systemctl disable hostapd
systemctl disable "wpa_supplicant@$WIFI_IF.service"

# Clean systemd network settings
rm -f /etc/systemd/network/*.network /etc/systemd/network/*.netdev

# Copy profiles settings
cp "$PROFILES_PATH/$PROFILE_NAME"/* /etc/systemd/network/

systemctl restart systemd-networkd
    sleep 2

if [ "$PROFILE_NAME" = "normal" ]; then
    ln -sf /run/systemd/resolve/resolv.conf /etc/resolv.conf
    systemctl daemon-reload
    systemctl enable "wpa_supplicant@$WIFI_IF.service"
    systemctl start "wpa_supplicant@$WIFI_IF.service"
elif [ "$PROFILE_NAME" = "ap-bonding" ]; then
    ln -sf /run/systemd/resolve/stub-resolv.conf /etc/resolv.conf
    systemctl daemon-reload
    systemctl enable hostapd
    systemctl start hostapd
fi

systemctl restart systemd-resolved
systemctl restart networkd-dispatcher
/sbin/ip -4 route flush cache
/sbin/ip -6 route flush cache

echo "$PROFILE_NAME Enabled"

exit 0
