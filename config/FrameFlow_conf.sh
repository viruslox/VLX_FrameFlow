#!/bin/bash

# System Paths
export VLXsuite_DIR="/opt/VLXframeflow"
export VLXlogs_DIR="/opt/VLXflowlogs"
export MEDIAMTX_DIR="/opt/mediamtx"

# External Resources
export GITHUB_URL="https://github.com/viruslox/VLXframeflow.git"
export KEYRING_PAGE_URL="https://www.deb-multimedia.org/pool/main/d/deb-multimedia-keyring/"
export ARMBIAN_KEY_URL="https://beta.armbian.com/armbian.key"

# APT repo files
APTGET_FILE="/etc/apt/sources.list.d/debian.sources"
DEBMLTMEDIA_FILE="/etc/apt/sources.list.d/unofficial-multimedia-packages.sources"
ARMBIAN_FILE="/etc/apt/sources.list.d/armbian-beta.sources"

# Network profiles
export NORM_PROFILE="/etc/systemd/network/profiles/normal"
export AP_PROFILE="/etc/systemd/network/profiles/ap-bonding"
export DISPATCHER_DIR="/etc/networkd-dispatcher/routable.d"

# Desktop environments and bloatware to remove
PKGS_TO_REMOVE=(
    "desktop" "gnome-desktop" "xfce-desktop" "kde-desktop"
    "cinnamon-desktop" "mate-desktop" "lxde-desktop" "lxqt-desktop"
    "qt*" "*gtk*" "adwaita*"
    "cloud-guest-utils" "cloud-init"
)

# Firmware packages
PKGS_FIRMWARE=(
    "firmware-linux" "firmware-linux-free" "firmware-linux-nonfree"
    "firmware-misc-nonfree" "firmware-realtek" "firmware-atheros"
    "firmware-brcm80211" "firmware-iwlwifi"
)

# System utilities
PKGS_SYSTEM=(
    "hostapd" "systemd-resolved" "wireless-tools" "ufw" "postfix"
    "mptcpize" "screen" "tasksel" "git" "jq" "curl" "wget"
    "aptitude" "apt" "dpkg"
)

# Multimedia & Sensor packages
PKGS_MEDIA=(
    "ffmpeg" "libavdevice-dev" "libcamera-dev" "libcamera-tools"
    "libcamera-v4l2" "dov4l" "dv4l" "qv4l2" "v4l-conf" "v4l-utils"
    "uvccapture" "libuvc-dev" "gpsd" "gpsd-clients"
)

# Consolidated install list
PKGS_ALL_INSTALL=(
    "${PKGS_FIRMWARE[@]}"
    "${PKGS_SYSTEM[@]}"
    "${PKGS_MEDIA[@]}"
)

#!/bin/bash

# --- Colors ---
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# --- Logging ---
log_info() { echo -e "${BLUE}[INFO]${NC} $1"; }
log_ok()   { echo -e "${GREEN}[OK]${NC} $1"; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_err()  { echo -e "${RED}[ERR]${NC} $1" >&2; }

die() {
    log_err "$1"
    exit 1
}

# --- Checks ---
check_root() {
    if [ "$EUID" -ne 0 ]; then
        die "Root privileges required."
    fi
}

get_installed_user() {
    if [ -d "$VLXsuite_DIR" ]; then
        ls -ld "$VLXsuite_DIR" | awk '{print $3}'
    else
        echo "root"
    fi
}
