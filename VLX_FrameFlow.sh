#!/bin/bash

echo "VLX FrameFlow"

BASE_DIR="$(dirname "$(readlink -f "$0")")"

# Imports
source "$BASE_DIR/config/FrameFlow_conf.sh"
source "$BASE_DIR/modules/FrameFlow_system.sh"
source "$BASE_DIR/modules/FrameFlow_packages.sh"
source "$BASE_DIR/modules/FrameFlow_network.sh"
source "$BASE_DIR/modules/FrameFlow_storage.sh"

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

# Checking if the user own the FrameFlow Suite
get_installed_user() {
    if [ -d "$VLXsuite_DIR" ]; then
        ls -ld "$VLXsuite_DIR" | awk '{print $3}'
    else
        echo "root"
    fi
}

# Root OR (Profile + Sudoers)
check_permissions() {
    # If root jump next
    if [ "$EUID" -eq 0 ]; then
        return 0
    fi

    local user_profile="$HOME/.frameflow_profile"
    local sudoers_file="/etc/sudoers.d/90-$USER"
    if [ -f "$user_profile" ] && [ -f "$sudoers_file" ]; then
        # echo "[INFO]: Accepted non-privileged user: $USER"
        return 0
    fi

    echo "[ERR]: Requirement: Root privileges OR a fully configured user setup."
    echo "If You are allowed to, try: 'sudo passwd root' , then login as root"
    
    die "Please launch again this script as root."
}

check_permissions
clear

run_system_setup() {
    log_info "System conf"
    systemctl enable --now ssh
    read -p "Fully update OS? (y/N) " a
    [[ "$a" =~ ^[yY]$ ]] && { system_update_repos; remove_bloatware;}
    restore_packages
    systemctl set-default multi-user.target
    apt -y install --reinstall systemd
    install_dependencies
    configure_kernel_sysctl
}

run_techuser_setup() {
    ## Reorder passwd file and get unprileged users list
    pwck -s
    userlist=($(awk -F: '($3>=1000)&&($1!="nobody")&&($NF!="/usr/sbin/nologin")&&($NF!="/bin/false"){print $1}' /etc/passwd))
    for i in "${!userlist[@]}"; do
        echo "[$i] ${userlist[$i]}"
    done
    echo "[N] Create new dedicated user"
    echo "[X] Do nothing"
    echo ""
    read -p "Enter your choice and press <Enter>: " CHOICE

    # Handle invalid (non-numeric or out-of-bounds) input
    if [[ "$CHOICE" =~ ^[nN]$ ]]; then
        read -p "Create new dedicated username [default: frameflow]: " answnewuser
        answnewuser=${answnewuser:-frameflow}
        setup_service_user $answnewuser
    elif [[ ! "$CHOICE" =~ ^[0-9]+$ ]] || [ "$CHOICE" -ge "${#userlist[@]}" ]; then
        exit 1
    else
        answnewuser=${userlist[$CHOICE]}
        setup_service_user $answnewuser
    fi
    read -p "Fully replace existing sudo config setting it up for $answnewuser? (y/N) " b
    [[ "$b" =~ ^[yY]$ ]] && setup_sudo_user "$answnewuser"
}

run_network_setup() {
    log_info "Network conf"
    configure_network_features
    configure_firewall
    create_wifi_profiles
    create_network_profiles
    enable_network_settings
}

run_application_setup() {
    log_info "Applications conf"
    update_suite_code
    install_mediamtx
    setup_maintenance_cron
}

echo "1) Install OS on +64GB drives (eMMc / SSD / nvme)"
echo "2) Configure System (Full Setup)"
echo "3) Update network interfaces"
echo "4) Create/Reconfigure FrameFlow user"
echo "5) Exit"
read -p "Select: " OPT

case "$OPT" in
    1) run_storage_installation_wizard ;;
    2) run_system_setup ; run_techuser_setup; run_network_setup ;run_application_setup ;;
    3) create_wifi_profiles; create_network_profiles ;;
    4) run_techuser_setup ;;
    5) exit 0 ;;
    *) echo "Invalid"; exit 1 ;;
esac

exit 0
