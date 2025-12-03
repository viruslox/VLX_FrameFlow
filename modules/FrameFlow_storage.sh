## "VLX FrameFlow - nvme / SSD / eMMc setup"

list_storage_devices() {
    # Exclude the device mounting / (root) to prevent self-cloning issues
    local root_dev
    root_dev=$(lsblk -no PKNAME $(findmnt -n -o SOURCE /) 2>/dev/null)
    lsblk -p -d -n -o NAME | grep -E 'nvme|mmcblk' | grep -v "$root_dev"
}

unmount_device() {
    local dev=$1
    log_info "Unmounting all partitions on $dev..."
    while mount | grep -q "$dev"; do
        for mount_point in $(mount | grep "$dev" | awk '{print $3}' | sort -r); do
            umount "$mount_point"
        done
        sleep 1
    done
}

# Check if the drive already has a VLX-compatible partition scheme
check_existing_layout() {
    local dev=$1
    local layout
    layout=$(sfdisk -d "$dev" 2>/dev/null)
    local linux_parts
    linux_parts=$(echo "$layout" | grep -c 'type=0FC63DAF-8483-4772-8E79-3D69D8477DE4') # Linux FS GUID

    # Check for GPT markers and specific partition GUIDs used in our scheme
    if echo "$layout" | grep -q 'label: gpt' && \
       echo "$layout" | grep -q 'type=C12A7328-F81F-11D2-BA4B-00A0C93EC93B' && \
       [ "$linux_parts" -ge 3 ]; then
        return 0 # Compatible
    else
        return 1 # Not compatible
    fi
}

partition_drive_gpt() {
    local dev=$1
    log_warn "Partitioning $dev (GPT Layout)..."
    
    # 1: EFI (ESP), 2: Boot, 3: Swap, 4: Root, 5: Home
    sfdisk "$dev" << EOF
label: gpt
size=1G,type=C12A7328-F81F-11D2-BA4B-00A0C93EC93B,name="EFI System"
size=1G,type=0FC63DAF-8483-4772-8E79-3D69D8477DE4,name="Linux boot"
size=4G,type=0657FD6D-A4AB-43C4-84E5-0933C84B4F4F,name="Linux swap"
size=44G,type=0FC63DAF-8483-4772-8E79-3D69D8477DE4,name="Linux root"
type=0FC63DAF-8483-4772-8E79-3D69D8477DE4,name="Linux home"
EOF
    partprobe "$dev"
    sleep 2
}

format_partitions() {
    local dev=$1
    local skip_home=$2

    log_info "Formatting partitions..."
    mkfs.vfat -F 32 -n EFI "${dev}p1" || die "Failed to format EFI"
    mkfs.ext4 -F -L boot "${dev}p2"   || die "Failed to format Boot"
    mkswap -f -L swap "${dev}p3"      || die "Failed to format Swap"
    mkfs.ext4 -F -L root "${dev}p4"   || die "Failed to format Root"
    
    if [ "$skip_home" != "true" ]; then
        mkfs.ext4 -F -L home "${dev}p5" || die "Failed to format Home"
    fi
}

clone_current_os() {
    local target_dev=$1
    local skip_home=$2
    local mnt="/mnt/temp_installer"
    
    mkdir -p "$mnt"
    
    # Mount sequence mimicking the old script
    mount "${target_dev}p4" "$mnt"
    mkdir -p "$mnt/boot"
    mount "${target_dev}p2" "$mnt/boot"
    mkdir -p "$mnt/boot/efi" "$mnt/boot/firmware"
    mount "${target_dev}p1" "$mnt/boot/efi"
    # Some distros use /boot/firmware for EFI partition too
    mount "${target_dev}p1" "$mnt/boot/firmware" 2>/dev/null || true
    
    if [ "$skip_home" != "true" ]; then
        mkdir -p "$mnt/home"
        mount "${target_dev}p5" "$mnt/home"
    fi

    log_info "Cloning OS via rsync (this will take time)..."
    
    local rsync_opts=(
        -aAXv --delete
        --exclude=/dev/* --exclude=/proc/* --exclude=/sys/* --exclude=/tmp/* --exclude=/run/* --exclude=/mnt/* --exclude=/media/* --exclude=/lost-found
    )
    
    if [ "$skip_home" == "true" ]; then
        rsync_opts+=(--exclude=/home/*)
    fi

    rsync "${rsync_opts[@]}" / "$mnt/"
    local rsync_status=$?
    
    # Exit code 23/24 are partial transfers (usually harmless on live systems)
    if [ $rsync_status -ne 0 ] && [ $rsync_status -ne 23 ] && [ $rsync_status -ne 24 ]; then
        log_err "Rsync failed with code $rsync_status"
        return 1
    fi
    sync

    log_info "Configuring Boot and Fstab..."
    
    # 1. Fix Symlinks (Critical for Armbian/RPi)
    cd "$mnt/boot/" || return
    ln -sf efi firmware 2>/dev/null

    # 2. Get UUIDs
    local p1uuid=$(lsblk -f -n -o UUID "${target_dev}p1")
    local p2uuid=$(lsblk -f -n -o UUID "${target_dev}p2")
    local p3uuid=$(lsblk -f -n -o UUID "${target_dev}p3")
    local p4uuid=$(lsblk -f -n -o UUID "${target_dev}p4")
    local p5uuid=$(lsblk -f -n -o UUID "${target_dev}p5")

    # 3. Update cmdline.txt (Critical for booting from NVMe)
    if [ -f "$mnt/boot/efi/cmdline.txt" ]; then
        cp -p "$mnt/boot/efi/cmdline.txt" "$mnt/boot/efi/cmdline.txt.BK"
        # Replace root=UUID=... or root=PARTUUID=... with new UUID
        sed -i "s/root=[^ ]*/root=UUID=$p4uuid/" "$mnt/boot/efi/cmdline.txt"
        
        # Fallback if sed didn't match standard patterns or file is empty
        if ! grep -q "root=UUID=$p4uuid" "$mnt/boot/efi/cmdline.txt"; then
             echo "console=serial0,115200 console=tty1 root=UUID=$p4uuid rootfstype=ext4 fsck.repair=yes rootwait nosplash debug --verbose cfg80211.ieee80211_regdom=IT consoleblank=0" > "$mnt/boot/efi/cmdline.txt"
        fi
    fi

    # 4. Generate new FSTAB
    cat <<EOF > "$mnt/etc/fstab"
proc /proc proc defaults 0 0
UUID=$p1uuid /boot/efi vfat defaults 0 2
UUID=$p2uuid /boot ext4 defaults 0 2
UUID=$p4uuid / ext4 errors=remount-ro 0 1
UUID=$p5uuid /home ext4 defaults 0 2
UUID=$p3uuid none swap sw 0 0
EOF

    cd /
    sync
    umount -R "$mnt"
    rmdir "$mnt" 2>/dev/null
    log_ok "Cloning and Boot Configuration Complete."
}

run_storage_installation_wizard() {
    log_info "Initializing Storage Installer..."
    
    mapfile -t DEVS < <(list_storage_devices)
    if [ ${#DEVS[@]} -eq 0 ]; then
        die "No NVMe/eMMC devices found."
    fi

    echo "Available devices:"
    for i in "${!DEVS[@]}"; do
        echo "[$i] ${DEVS[$i]}"
    done
    
    read -p "Select target [0-$((${#DEVS[@]}-1))]: " CHOICE
    if [[ ! "$CHOICE" =~ ^[0-9]+$ ]] || [ "$CHOICE" -ge "${#DEVS[@]}" ]; then
        die "Invalid selection."
    fi

    local device="${DEVS[$CHOICE]}"
    unmount_device "$device"

    local skip_home="false"
    
    # Intelligent check: Does it look like a VLX drive already?
    if check_existing_layout "$device"; then
        log_info "Found existing VLX partition scheme."
        read -p "Preserve existing /home partition data? (y/N) " ans
        if [[ "$ans" =~ ^[yY]$ ]]; then
            skip_home="true"
        fi
    fi

    if [ "$skip_home" == "false" ]; then
        read -p "WARNING: $device will be COMPLETELY WIPED. Type 'ok' to proceed: " confirm
        if [ "$confirm" != "ok" ]; then
            log_info "Operation cancelled."
            return 0
        fi
        partition_drive_gpt "$device"
    fi

    format_partitions "$device" "$skip_home"
    clone_current_os "$device" "$skip_home"
    
    log_ok "Installation complete. Please reboot without SD card."
}
