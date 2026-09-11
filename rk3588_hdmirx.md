# RK3588 HDMI-RX: full stack enablement (Debian, UEFI+GRUB2)

Target: any RK3588/RK3588S board with mainline dts `hdmi_receiver@fdee0000`
present and `status = "okay"`. Verified on kernel 7.2 (Debian
`linux-source-<KVER_MAJOR>`); procedure is version-generic.

Placeholders: `<KVER>` = `uname -r`, `<KVER_MAJOR>` = source package major
(e.g. `7.2`), `<BOARD_DTS>` = board dts basename, `<ESP_UUID>` = ESP FAT
partition UUID.

---

## 1. Discover all HDMI-RX-related symbols (video + audio + CEC)

Don't hardcode a symbol list — grep the running config for every match,
so this stays valid across kernel versions:

```bash
grep -iE "hdmirx|snps.*hdmi.*rx" /boot/config-<KVER>
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
apt install linux-source-<KVER_MAJOR> linux-config-<KVER_MAJOR>
cd /usr/src && tar xf linux-source-<KVER_MAJOR>.tar.* && cd linux-source-<KVER_MAJOR>
```

---

## 3. Config: enable every matched symbol as module

```bash
cp /boot/config-<KVER> .config

for sym in $(grep -ioE "CONFIG_[A-Z0-9_]*HDMI[A-Z0-9_]*RX[A-Z0-9_]*" .config \
             drivers/media/platform/synopsys/hdmirx/Kconfig \
             sound/soc/rockchip/Kconfig 2>/dev/null | cut -d: -f2 | sort -u); do
  scripts/config --module "$sym"
done

# CEC + audio are usually separate symbols, not auto-selected — verify:
scripts/config --module CONFIG_VIDEO_SYNOPSYS_HDMIRX_CEC
grep -q CONFIG_SND_SOC_HDMI_CODEC .config || scripts/config --module CONFIG_SND_SOC_HDMI_CODEC

make ARCH=arm64 olddefconfig
make ARCH=arm64 modules_prepare
```

---

## 4. Build — vermagic fix (mandatory)

Debian's `uname -r` (e.g. `<KVER>`) never matches the source tree's own
`kernel.release` (patch level truncated in package name). Skipping this
step produces `Exec format error` at `modprobe`.

```bash
echo "<KVER>" > include/config/kernel.release
sed -i "s/#define UTS_RELEASE.*/#define UTS_RELEASE \"<KVER>\"/" include/generated/utsrelease.h
cp /usr/src/linux-headers-<KVER>/Module.symvers .

make ARCH=arm64 KERNELRELEASE=<KVER> M=drivers/media/platform/synopsys/hdmirx modules
modinfo drivers/media/platform/synopsys/hdmirx/synopsys-hdmirx.ko | grep vermagic
# must read exactly: <KVER> SMP preempt mod_unload aarch64

# if an audio module was enabled, build it too, same pattern:
make ARCH=arm64 KERNELRELEASE=<KVER> M=sound/soc/rockchip modules   # path may vary by kernel version

sudo make ARCH=arm64 KERNELRELEASE=<KVER> M=drivers/media/platform/synopsys/hdmirx modules_install
sudo depmod -a <KVER>
sudo modprobe synopsys_hdmirx
```

Out-of-tree, unsigned → kernel taint (harmless). Does not survive
`linux-image-*` upgrades; for persistence, fold the symbols into
`debian/config/` and rebuild the kernel package instead.

---

## 5. Device tree

Mainline dts for supported RK3588 boards already ships the node
correctly wired (`status = "okay"`, `hpd-gpios` set) — no patching
needed on most boards (exceptions exist, e.g. missing `hpd-gpios` on
some vendor boards; check first).

```bash
dtc -I dtb -O dts /usr/lib/linux-image-<KVER>/rockchip/<BOARD_DTS>.dtb 2>/dev/null | grep -A15 hdmi_receiver
```

Verify what's actually loaded at boot, not what's on disk:

```bash
cat /proc/device-tree/model
find /sys/firmware/devicetree/base -iname "*hdmi_receiver*"
```

Empty result → wrong dtb is being loaded; go to step 6.

---

## 6. Boot chain (UEFI + GRUB2)

`GRUB_DEVICETREE` in `/etc/default/grub` is a no-op on Debian's
`grub-efi-arm64` — no script implements it. Add an explicit loader:

```bash
blkid <ESP_partition_device>   # get <ESP_UUID>

cat > /etc/grub.d/09_devicetree << EOF
#!/bin/sh
exec tail -n +3 \$0
search --no-floppy --fs-uuid --set=dtbroot <ESP_UUID>
devicetree (\$dtbroot)/dtb/rockchip/<BOARD_DTS>.dtb
EOF
chmod +x /etc/grub.d/09_devicetree
update-grub
grep -A3 devicetree /boot/grub/grub.cfg   # confirm syntax + resolved path
reboot
```

Notes:
- Path in `devicetree` must resolve against the **ESP**, not `/boot` —
  GRUB's default search root is the partition holding `vmlinuz`, which
  is typically a different partition than the ESP holding `dtb/`.
- `exec tail -n +3 $0` outputs the remainder of the script verbatim into
  `grub.cfg` — do not wrap the `devicetree` line in `cat <<EOF`, it will
  be copied literally and break parsing.
- If `u-boot-efi-dtb` is installed, it resyncs `/boot/efi/dtb/` from
  `linux-image-*` on every kernel update; confirm the synced dtb still
  contains the node after upgrades.

---

## 7. Verification

```bash
lsmod | grep -iE "hdmirx|hdmi_codec"
dmesg | grep -i hdmirx
v4l2-ctl --list-devices
v4l2-ctl -d /dev/videoN --all      # expect: Video input : 0 (HDMI IN: ok)
aplay -l | grep -i hdmirx          # if audio capture module present
```

---

## Board-specific caveats (verify per-board before reuse)

- Boot chain (UEFI/GRUB vs plain U-Boot extlinux) is **not** uniform
  across RK3588 boards — steps 5–6 must be re-verified per board.
- Some boards need dts edits (`hpd-gpios`) not covered here.
- 4K@60 EDID negotiation issues are a known upstream driver limitation
  on some boards/sources — check `v4l2-ctl --query-dv-timings` and
  `--show-edid` if resolution negotiation fails after the above.
