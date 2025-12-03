#!/bin/bash

echo "VLX FrameFlow - nvme / SSD / eMMc setup"

list_storage_devices() {
    SYSTEM_DEVICE_NAME=$(lsblk -dno PKNAME $(findmnt -n -o SOURCE /))
    lsblk -pdno NAME | grep -v "$SYSTEM_DEVICE_NAME"
}

unmount_device() {
    local dev=$1
    while mount | grep -q "$dev"; do
        umount $(mount | grep "$dev" | awk '{print $3}' | head -n 1)
    done
}

partition_drive_gpt() {
    local dev=$1
    log_warn "Partitioning $dev..."
    sfdisk "$dev" << EOF
label: gpt
size=1G,type=C12A7328-F81F-11D2-BA4B-00A0C93EC93B,name="EFI"
size=1G,type=0FC63DAF-8483-4772-8E79-3D69D8477DE4,name="Boot"
size=4G,type=0657FD6D-A4AB-43C4-84E5-0933C84B4F4F,name="Swap"
size=44G,type=0FC63DAF-8483-4772-8E79-3D69D8477DE4,name="Root"
type=0FC63DAF-8483-4772-8E79-3D69D8477DE4,name="Home"
EOF
    partprobe "$dev" && sleep 2
}

format_partitions() {
    local dev=$1; local skip_home=$2
    mkfs.vfat -F 32 -n EFI "${dev}p1"
    mkfs.ext4 -F -L boot "${dev}p2"
    mkswap -f -L swap "${dev}p3"
    mkfs.ext4 -F -L root "${dev}p4"
    [ "$skip_home" != "true" ] && mkfs.ext4 -F -L home "${dev}p5"
}

clone_current_os() {
    local dev=$1; local skip_home=$2
    local mnt="/mnt/installer"
    mkdir -p "$mnt"
    mount "${dev}p4" "$mnt"
    mkdir -p "$mnt/boot/efi"
    mount "${dev}p2" "$mnt/boot"
    mount "${dev}p1" "$mnt/boot/efi"

    local rsync_opts=(-aAX --delete --exclude={/dev/*,/proc/*,/sys/*,/tmp/*,/run/*,/mnt/*})
    if [ "$skip_home" == "true" ]; then
        rsync_opts+=(--exclude=/home/*)
    else
        mkdir -p "$mnt/home" && mount "${dev}p5" "$mnt/home"
    fi

    log_info "Cloning OS..."
    rsync "${rsync_opts[@]}" / "$mnt/"

    # Fstab
    local p1uuid=$(lsblk -f -n -o UUID "${dev}p1")
    local p2uuid=$(lsblk -f -n -o UUID "${dev}p2")
    local p3uuid=$(lsblk -f -n -o UUID "${dev}p3")
    local p4uuid=$(lsblk -f -n -o UUID "${dev}p4")
    local p5uuid=$(lsblk -f -n -o UUID "${dev}p5")

    cat <<EOF > "$mnt/etc/fstab"
proc /proc proc defaults 0 0
UUID=$p1uuid /boot/efi vfat defaults 0 2
UUID=$p2uuid /boot ext4 defaults 0 2
UUID=$p4uuid / ext4 errors=remount-ro 0 1
UUID=$p3uuid none swap sw 0 0
UUID=$p5uuid /home ext4 defaults 0 2
EOF

    umount -R "$mnt"
    log_ok "Done."
}

run_storage_installation_wizard() {
    log_info "Starting Storage Wizard..."
    mapfile -t DEVS < <(list_storage_devices)
    [ ${#DEVS[@]} -eq 0 ] && die "No devices found."

    echo "Devices:"; for i in "${!DEVS[@]}"; do echo "[$i] ${DEVS[$i]}"; done
    read -p "Select [0-$((${#DEVS[@]}-1))]: " IDX
    local dev="${DEVS[$IDX]}"
    [ -b "$dev" ] || die "Invalid."

    unmount_device "$dev"
    partition_drive_gpt "$dev"
    format_partitions "$dev" "false"
    clone_current_os "$dev" "false"
    log_ok "Install complete."
}
