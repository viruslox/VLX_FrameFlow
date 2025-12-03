#!/bin/bash

remove_bloatware() {
    log_info "Removing packages that might interfere with FrameFlow"
    apt-get purge -y "${PKGS_TO_REMOVE[@]}"
    apt-get autoremove -y
    aptitude -y purge '~c'
    log_ok "Cleaned."
}

restore_packages() {
    if [ -f /root/pkg.list ]; then
        read -p "Found a list of packages previously installed. Reinstall them? (y/N) " getthelist;
        if [[ "$getthelist" =~ [yY] ]]; then
            xargs -a /root/pkg.list apt-get -y install
        fi
    fi
}

install_dependencies() {
    log_info "Installing dependencies"
    apt --fix-broken install -y
    apt-get update -y
    apt-get install -y "${PKGS_ALL_INSTALL[@]}"
    log_ok "Installed."
}

update_suite_code() {
    log_info "Updating source code..."
    mkdir -p "$VLXsuite_DIR"
    local user=$(get_installed_user)

    if [ -d "$VLXsuite_DIR/.git" ]; then
        cd "$VLXsuite_DIR" || return
        sudo -u "$user" git reset --hard
        sudo -u "$user" git pull --no-verify "$GITHUB_URL"
    else
        chown -R "$user:$user" "$VLXsuite_DIR"
        sudo -u "$user" git clone "$GITHUB_URL" "$VLXsuite_DIR"
    fi
    chmod 700 "$VLXsuite_DIR"/*.sh "$VLXsuite_DIR"/config/FrameFlow_maintenance.sh
    log_ok "Code updated."
}

install_mediamtx() {
    log_info "Checking MediaMTX..."
    mkdir -p "$MEDIAMTX_DIR"
    cd "$MEDIAMTX_DIR" || return

    if [ -f "mediamtx" ]; then
        ./mediamtx --upgrade
    else
        local url=$(wget -qO- https://api.github.com/repos/bluenviron/mediamtx/releases/latest | grep "browser_download_url.*linux_arm64.tar.gz" | cut -d '"' -f 4)
        if [ -n "$url" ]; then
            wget -q "$url" -O mediamtx.tar.gz && tar -zxf mediamtx.tar.gz && rm mediamtx.tar.gz
            local user=$(get_installed_user)
            chown -R "$user:$user" "$MEDIAMTX_DIR"
            chmod 700 "$MEDIAMTX_DIR/mediamtx"
        else
            log_err "MediaMTX download failed."
        fi
    fi
    log_ok "MediaMTX ready."
}
