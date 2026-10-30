# RK3588 HDMI-RX: full stack enablement (Debian arm64, EDK2 UEFI + GRUB2)

Target: any RK3588/RK3588S board with mainline dts `hdmi_receiver@fdee0000`
present and `status = "okay"` — specifically the **Orange Pi 5 Plus** and
**Radxa Rock 5T**. Verified on kernel 7.2 (vanilla Debian `arm64`
`linux-source-<KVER_MAJOR>`); procedure is version-generic.

## Reference Boot Stack (Recommended)

For a clean, standard, bloatware-free system on RK3588 SBCs, use the standard
UEFI + vanilla Debian `arm64` boot flow rather than vendor-specific OS images:

1. **UEFI Firmware ([edk2-porting/edk2-rk3588](https://github.com/edk2-porting/edk2-rk3588)):**
   - **Orange Pi 5 Plus:** flash `orangepi-5-plus_UEFI_Release_*.img` to SPI NOR (or SD/eMMC).
   - **Radxa Rock 5T:** `edk2-rk3588` does not publish a separate `rock-5t` image; use **`rock-5b-plus` (`rock-5bplus_UEFI_Release_*.img`)**, which is hardware-compatible for UEFI bootstrap on the Rock 5T, and load the mainline `rk3588-rock-5t.dtb` device tree via GRUB2 (Step 6) so the Linux kernel gets the exact Rock 5T hardware description.
   - In the EDK2 menu (`Escape` at boot → `Device Manager` → `Rockchip Platform Configuration` → `ACPI / Device Tree`), set configuration mode to **Device Tree** so GRUB2 can pass the mainline kernel DTB.
2. **OS & Bootloader:** Install **vanilla Debian `arm64`** (standard official Debian `netinst` ISO) using standard **GRUB2 (`grub-efi-arm64`)**.

---

Placeholders: `<KVER>` = `uname -r`, `<KVER_MAJOR>` = source package major
(e.g. `7.2`), `<BOARD_DTS>` = board dts basename, `<ESP_UUID>` = `/boot/efi`
ESP FAT partition UUID.

```bash
KVER=$(uname -r)
KVER_MAJOR=$(echo "$KVER" | cut -d. -f1,2)
BOARD_DTS=rk3588-orangepi-5-plus
#BOARD_DTS=rk3588-rock-5t
ESP_UUID=$(findmnt -n -o UUID /boot/efi)
```
---

## 1. Discover all HDMI-RX-related symbols (video + audio + CEC)

Don't hardcode a symbol list — grep the running config for every match,
so this stays valid across kernel versions:

```bash
grep -iE "hdmirx|snps.*hdmi.*rx" /boot/config-$KVER
```

Typical hits: `CONFIG_VIDEO_SYNOPSYS_HDMIRX`, `CONFIG_VIDEO_SYNOPSYS_HDMIRX_CEC`,
plus any `SND_SOC_*HDMIRX*` / `SND_SOC_ROCKCHIP*HDMIRX*` ALSA audio-capture
entry if present in that kernel's Kconfig tree (audio capture support for
this IP landed after the initial video driver — check for it explicitly,
don't assume absence).

Also check unconditionally for ALSA audio nodes even if not yet in
`/boot/config` (new Kconfig symbol not yet applied):
```bash
grep -riE "hdmirx" drivers/sound/ sound/soc/rockchip/ 2>/dev/null
```
(run from kernel source tree, see step 2)

---

## 2. Kernel source

`linux-headers-*` ships no `.c` — full source required:

```bash
apt install linux-source-$KVER_MAJOR linux-config-$KVER_MAJOR linux-headers-$KVER
cd /usr/src && tar xf linux-source-$KVER_MAJOR.tar.* && cd linux-source-$KVER_MAJOR
```

---

## 3. Config: enable every matched symbol as module

> **Note:** In `Kconfig` files, symbols are declared as `config FOO` (without the
> `CONFIG_` prefix), and in `.config` disabled symbols appear as
> `# CONFIG_FOO is not set` (so a plain `grep -q CONFIG_FOO .config` would
> falsely succeed even when disabled). Use `scripts/config --module` directly:

```bash
cp /boot/config-$KVER .config

# Match symbols from both .config and Kconfig files (stripping any CONFIG_ prefix)
for sym in $( {
  grep -hoE "CONFIG_[A-Z0-9_]*HDMI[A-Z0-9_]*RX[A-Z0-9_]*" .config 2>/dev/null | sed 's/^CONFIG_//'
   awk '/^config [A-Z0-9_]*HDMI[A-Z0-9_]*RX[A-Z0-9_]*/ {print $2}' \
    drivers/media/platform/synopsys/hdmirx/Kconfig \
    sound/soc/rockchip/Kconfig 2>/dev/null
} | sort -u ); do
  scripts/config --module "$sym"
done

# Ensure CEC + HDMI audio codec + V4L2 DV timings dependencies are enabled:
scripts/config --module CONFIG_VIDEO_SYNOPSYS_HDMIRX
scripts/config --module CONFIG_VIDEO_SYNOPSYS_HDMIRX_CEC
scripts/config --module CONFIG_SND_SOC_HDMI_CODEC

make ARCH=arm64 KERNELRELEASE=$KVER olddefconfig
make ARCH=arm64 KERNELRELEASE=$KVER modules_prepare
```

---

## 4. Build — vermagic fix (mandatory)

Debian's `uname -r` (e.g. `$KVER`) never matches the source tree's own
default `kernel.release` (patch level/ABI tag truncated in package name).
Skipping this step produces `Exec format error` at `modprobe`.

```bash
echo "$KVER" > include/config/kernel.release
sed -i "s/#define UTS_RELEASE.*/#define UTS_RELEASE \"$KVER\"/" include/generated/utsrelease.h
cp /usr/src/linux-headers-$KVER/Module.symvers .

make ARCH=arm64 KERNELRELEASE=$KVER M=drivers/media/platform/synopsys/hdmirx modules
modinfo drivers/media/platform/synopsys/hdmirx/synopsys-hdmirx.ko | grep vermagic
# must read exactly: $KVER SMP preempt mod_unload aarch64

# if an audio module was enabled, build it too, same pattern:
make ARCH=arm64 KERNELRELEASE=$KVER M=sound/soc/rockchip modules   # path may vary by kernel version

make ARCH=arm64 KERNELRELEASE=$KVER M=drivers/media/platform/synopsys/hdmirx modules_install
depmod -a $KVER
modprobe synopsys_hdmirx
```

Out-of-tree, unsigned → kernel taint (harmless). Does not survive
`linux-image-*` ABI upgrades; for persistence across kernel upgrades, fold the
symbols into `debian/config/` and rebuild the kernel package or wrap Steps 3–4
into a DKMS module.

---

## 5. Device tree

Mainline dts for supported RK3588 boards already ships the node
correctly wired (`status = "okay"`, `hpd-gpios` set) — no patching
needed on most boards (exceptions exist, e.g. missing `hpd-gpios` on
some vendor boards; check first).

```bash
dtc -I dtb -O dts /usr/lib/linux-image-$KVER/rockchip/$BOARD_DTS.dtb 2>/dev/null | grep -A15 hdmi_receiver
```

Verify what's actually loaded at boot, not what's on disk:

```bash
tr -d '\0' < /proc/device-tree/model; echo
find /sys/firmware/devicetree/base -iname "*hdmi_receiver*"
```

Empty result (or wrong board model, e.g. `Rock 5B+` when booting a `Rock 5T`
via `rock-5bplus_UEFI`) → the UEFI built-in DTB is being used instead of the
OS mainline DTB; go to step 6.

---

## 6. Boot chain

The method to load the right device tree depends on your bootloader environment.

### Case A (Recommended): EDK2 UEFI + GRUB2 on Vanilla Debian `arm64`

`GRUB_DEVICETREE` in `/etc/default/grub` is a no-op on Debian's
`grub-efi-arm64` — no default `/etc/grub.d/` script implements it.

First, stage the mainline `.dtb` onto the EFI System Partition (`/boot/efi`)
and install an `/etc/kernel/postinst.d/` hook so kernel updates automatically
keep `/boot/efi/dtb/rockchip/$BOARD_DTS.dtb` in sync:

```bash
mkdir -p /boot/efi/dtb/rockchip
cp /usr/lib/linux-image-$KVER/rockchip/$BOARD_DTS.dtb /boot/efi/dtb/rockchip/

cat > /etc/kernel/postinst.d/zz-sync-rk3588-dtb << 'EOF'
#!/bin/sh
set -e
KVER="$1"
if [ -d "/usr/lib/linux-image-$KVER/rockchip" ] && mountpoint -q /boot/efi; then
    mkdir -p /boot/efi/dtb/rockchip
    cp -f /usr/lib/linux-image-"$KVER"/rockchip/*.dtb /boot/efi/dtb/rockchip/
fi
EOF
chmod +x /etc/kernel/postinst.d/zz-sync-rk3588-dtb
```

Next, add an explicit GRUB loader snippet `/etc/grub.d/09_devicetree`:

```bash
ESP_UUID=$(findmnt -n -o UUID /boot/efi)

cat > /etc/grub.d/09_devicetree << EOF
#!/bin/sh
exec tail -n +3 \$0
search --no-floppy --fs-uuid --set=dtbroot $ESP_UUID
devicetree (\$dtbroot)/dtb/rockchip/$BOARD_DTS.dtb
EOF
chmod +x /etc/grub.d/09_devicetree
update-grub
grep -A3 devicetree /boot/grub/grub.cfg   # confirm syntax + resolved path
reboot
```

Notes for GRUB2:
- Path in `devicetree` resolves against the **ESP** (`/boot/efi`), because
  GRUB's default `$root` is the `/boot` partition holding `vmlinuz`.
- `exec tail -n +3 \$0` outputs the remainder of the script verbatim into
  `grub.cfg` — keep `\$0` and `\$dtbroot` escaped when generating the file.
- For **Radxa Rock 5T** running `rock-5bplus_UEFI`, setting
  `BOARD_DTS=rk3588-rock-5t` here ensures Linux boots with the exact
  `rk3588-rock-5t.dtb` device tree (including its `hdmi_receiver@fdee0000` node).

### Case B: U-Boot with `extlinux` (`/boot/extlinux/extlinux.conf`)

If using a U-Boot environment that reads `/boot/extlinux/extlinux.conf` instead
of UEFI + GRUB2, ensure the active boot entry points to the mainline `.dtb`:

```bash
cat /boot/extlinux/extlinux.conf
```

Ensure your default boot label contains `fdt` (or `fdtdir`) pointing to the
installed kernel's device tree:
```text
    # Either pointing to the directory:
    fdtdir /usr/lib/linux-image-<KVER>/

    # ...or explicitly to the board DTB file:
    fdt /usr/lib/linux-image-<KVER>/rockchip/rk3588-rock-5t.dtb
```

---

## 7. Verification

```bash
lsmod | grep -iE "hdmirx|hdmi_codec"
dmesg | grep -i hdmirx
v4l2-ctl --list-devices
v4l2-ctl -d /dev/videoN --all      # expect: Video input : 0 (HDMI IN: ok)
arecord -l | grep -i hdmirx        # verify ALSA audio capture device (not aplay -l!)
```

---

## Board-specific caveats (verify per-board before reuse)

- Boot chain (UEFI/GRUB vs plain U-Boot extlinux) is **not** uniform
  across RK3588 boards — steps 5–6 must be re-verified per board.
- Some boards need dts edits (`hpd-gpios`) not covered here.
- 4K@60 EDID negotiation issues are a known upstream driver limitation
  on some boards/sources — check `v4l2-ctl --query-dv-timings` and
  `--show-edid` if resolution negotiation fails after the above.
