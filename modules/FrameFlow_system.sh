#!/bin/bash

system_update_repos() {
    log_info "Configuring APT repositories..."

    # Multimedia Keyring
    if ! [ "$(dpkg -l | grep multimedia-keyring)" ]; then
        local keyring=$(curl -s "$KEYRING_PAGE_URL" | grep -o 'deb-multimedia-keyring_[0-9.]*_all\.deb' | sort -V | tail -n 1)
        if [ -n "$keyring" ]; then
            wget -q "${KEYRING_PAGE_URL}${keyring}" -O "/tmp/$keyring"
            dpkg -i "/tmp/$keyring" && rm "/tmp/$keyring"
        fi
    fi

    # Armbian Keyring
    wget -qO- "$ARMBIAN_KEY_URL" | gpg --dearmor | tee /usr/share/keyrings/armbian.gpg > /dev/null

    apt -y modernize-sources

    # Reconfiguring APT
    cat <<EOF > $APTGET_FILE
Types: deb
URIs: https://deb.debian.org/debian/
Suites: testing testing-updates experimental
Components: main contrib non-free non-free-firmware
Signed-By: /usr/share/keyrings/debian-archive-keyring.gpg

Types: deb
URIs: http://security.debian.org/debian-security/
Suites: testing-security
Components: main contrib non-free non-free-firmware
Signed-By: /usr/share/keyrings/debian-archive-keyring.gpg
EOF

    cat <<EOF > $DEBMLTMEDIA_FILE
Types: deb
URIs: https://www.deb-multimedia.org/
Suites: testing
Components: main non-free
Signed-By: /usr/share/keyrings/deb-multimedia-keyring.pgp
EOF

    cat <<EOF > $ARMBIAN_FILE
Types: deb
URIs: https://beta.armbian.com/
Suites: sid
Components: main sid-utils sid-desktop
Signed-By: /usr/share/keyrings/armbian.gpg
EOF

    apt-get update -y
    apt-get install -y aptitude apt dpkg
    log_ok "Repos updated."
}

create_user_profile() {
    local user=$1
    local home_dir="/home/$user"
    local profile="$home_dir/.frameflow_profile"

    log_info "Generating default profile at $profile"
    cat <<EOF > "$profile"
# VLX FrameFlow User Profile
VLXsuite_DIR="$VLXsuite_DIR"
VLXlogs_DIR="$VLXlogs_DIR"
MEDIAMTX_DIR="$MEDIAMTX_DIR"
ENABLED_DEVICES=0
RTSP_URL="#rtsps://<host>:<port>/<path>/<key>"
SRT_URL="#srt://<host>:<port>?streamid=publish:<path>/<key>"
AUDIODEV='card.*USB'
#API_URL="http://your-server-ip:3000/update-gps"
#AUTH_TOKEN="<token>"
EOF

    chown "$user:$user" "$profile"
    log_ok "Profile created."
}

setup_sudo_user() {
    local user="${1:-frameflow}"
    local sudo_file="/etc/sudoers.d/90-$user"

    log_info "Purging existing sudoers"
    rm -fv /etc/sudoers.d/* 2>/dev/null
    
    log_info "Setting up sudoers for user: $user"
    cat <<EOF > "$sudo_file"
$user ALL=(ALL) NOPASSWD: $VLXsuite_DIR/VLX_FrameFlow.sh
$user ALL=(ALL) NOPASSWD: $VLXsuite_DIR/VLX_netflow.sh

$user ALL=(ALL) NOPASSWD: /usr/bin/systemctl restart systemd-networkd
$user ALL=(ALL) NOPASSWD: /usr/bin/systemctl restart systemd-resolved
$user ALL=(ALL) NOPASSWD: /usr/bin/systemctl start hostapd
$user ALL=(ALL) NOPASSWD: /usr/bin/systemctl stop hostapd
EOF

    chmod 0440 "$sudo_file"
    if command -v visudo >/dev/null; then
        visudo -c -f "$sudo_file" || log_warn "Sudoers syntax check failed!"
    fi

    log_ok "Sudoers updated."
}

setup_service_user() {
    local user="${1:-frameflow}"
    log_info "Setting up user: $user"

    if ! id "$user" &>/dev/null; then
        adduser --home "/home/$user" --shell /bin/bash --gecos "VLX FrameFlow tech user" "$user"
    fi

    usermod -a -G crontab,dialout,tty,video,audio,plugdev,netdev,i2c,bluetooth "$user"
    loginctl enable-linger "$user"

    mkdir -p "$VLXsuite_DIR" "$VLXlogs_DIR" "$MEDIAMTX_DIR"
    chown -Rf "$user:$user" "$VLXsuite_DIR" "$VLXlogs_DIR" "$MEDIAMTX_DIR"

    create_user_profile "$user"
    log_ok "User configured."
}

configure_kernel_sysctl() {
    echo "kernel.dmesg_restrict=0" > /etc/sysctl.d/99-disable-dmesg-restrict.conf
    sysctl --system
}

setup_maintenance_cron() {
    log_info "Installing cron job..."
    local cron_script="$VLXsuite_DIR/config/FrameFlow_maintenance.sh"
    local cron_job="@reboot $cron_script start 2>&1"

    if ! crontab -l 2>/dev/null | grep -qF "$cron_script"; then
        (crontab -l 2>/dev/null; echo "$cron_job") | crontab -
        log_ok "Cron job added."
    fi
}
