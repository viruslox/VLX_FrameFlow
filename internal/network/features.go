package network

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/viruslox/vlx_frameflow/internal/sysutils"
)

func ConfigureNetworkFeatures() error {
	sysutils.Info("Installing network packages...")

	// Install packages
	pkgs := []string{"ufw", "hostapd", "systemd-timesyncd", "networkd-dispatcher", "wpasupplicant", "iproute2"}
	args := append([]string{"install", "-y"}, pkgs...)
	_, err := sysutils.RunCommand(10*time.Minute, "apt-get", args...)
	if err != nil {
		sysutils.RunCommand(10*time.Minute, "apt-get", "update")
		sysutils.RunCommand(10*time.Minute, "apt-get", args...)
	}

	// Purge netplan if present
	out, _ := sysutils.RunCommand(10*time.Second, "dpkg", "-l")
	if strings.Contains(out, "netplan") {
		// Just a simple approximation to purge netplan packages
		sysutils.RunCommand(10*time.Minute, "apt-get", "purge", "-y", "netplan.io")
	}

	// Create directories
	os.MkdirAll("/etc/networkd-dispatcher/routable.d", 0755)
	os.MkdirAll("/etc/networkd-dispatcher/off.d", 0755)

	sysctlFile := "/etc/sysctl.d/97-forwarding.conf"
	if _, err := os.Stat(sysctlFile); os.IsNotExist(err) {
		content := `net.ipv4.ip_forward=1
net.ipv6.conf.all.forwarding=1
net.ipv4.conf.all.arp_ignore=1
net.ipv4.conf.all.arp_announce=2
`
		os.WriteFile(sysctlFile, []byte(content), 0644)
		sysutils.RunCommand(10*time.Second, "sysctl", "-p", sysctlFile)
	}

	return nil
}

func EnableNetworkSettings() error {
	sysutils.Info("Applying settings...")

	sysutils.RunCommand(10*time.Second, "systemctl", "disable", "NetworkManager")
	sysutils.RunCommand(10*time.Second, "systemctl", "mask", "NetworkManager")

	sysutils.RunCommand(10*time.Second, "systemctl", "daemon-reload")
	sysutils.RunCommand(10*time.Second, "systemctl", "enable", "systemd-networkd", "systemd-resolved", "networkd-dispatcher")

	waitOnlineDir := "/etc/systemd/system/systemd-networkd-wait-online.service.d"
	os.MkdirAll(waitOnlineDir, 0755)

	waitOnlineBin := "/lib/systemd/systemd-networkd-wait-online"
	if _, err := os.Stat("/usr/lib/systemd/systemd-networkd-wait-online"); err == nil {
		waitOnlineBin = "/usr/lib/systemd/systemd-networkd-wait-online"
	}

	overrideContent := fmt.Sprintf(`[Service]
ExecStart=
ExecStart=%s --timeout=2 --any
`, waitOnlineBin)

	os.WriteFile(waitOnlineDir+"/override.conf", []byte(overrideContent), 0644)
	sysutils.RunCommand(10*time.Second, "systemctl", "enable", "systemd-networkd-wait-online.service")

	sysutils.RunCommand(10*time.Second, "ufw", "reload")
	sysutils.RunCommand(10*time.Second, "ufw", "--force", "enable")

	sysutils.Success("Network settings applied.")
	return nil
}

func ClientReset() error {
	sysutils.Info("Stopping client components...")

	services := []string{"frameflow-mptcp-proxy.service", "frameflow-mlvpn.service"}
	for _, svc := range services {
		sysutils.RunCommand(10*time.Second, "systemctl", "--user", "stop", svc)
	}
	sysutils.RunCommand(10*time.Second, "pkill", "-f", "ss-redir")
	sysutils.RunCommand(10*time.Second, "pkill", "-f", "mlvpn")
	sysutils.RunCommand(10*time.Second, "pkill", "-f", "v2ray-plugin")

	sysutils.Info("Reconfiguring network interfaces...")

	EnableNetworkSettings()
	sysutils.RunCommand(10*time.Second, "sudo", "systemctl", "restart", "systemd-networkd.service")

	return nil
}
