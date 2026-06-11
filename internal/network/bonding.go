package network

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/viruslox/vlx_frameflow/internal/sysutils"
)

// GenerateMlvpnConfig creates the MLVPN configuration file based on the role.
func GenerateMlvpnConfig(configFile, updownScript, mlvpnKey, role, mlvpnServerIP string) error {
	var content string

	if role == "CLIENT" {
		content = fmt.Sprintf(`[general]
statuscommand = "%s"
mode = "client"
ip4 = "10.1.10.2/24"
ip4_gateway = "10.1.10.1"
mtu = 1444
tuntap = "tun"
interface_name = "mlvpn0"
timeout = 30
password = "%s"

[mlvpn_link]
bindhost = "0.0.0.0"
remotehost = "%s"
remoteport = 5080
`, updownScript, mlvpnKey, mlvpnServerIP)
	} else {
		content = fmt.Sprintf(`[general]
statuscommand = "%s"
mode = "server"
ip4 = "10.1.10.1/24"
ip4_gateway = "10.1.10.2"
mtu = 1444
tuntap = "tun"
interface_name = "mlvpn0"
timeout = 30
password = "%s"

[mlvpn_link]
bindhost = "0.0.0.0"
bindport = 5080
`, updownScript, mlvpnKey)
	}

	configDir := filepath.Dir(configFile)
	err := os.MkdirAll(configDir, 0755)
	if err != nil {
		return fmt.Errorf("failed to create config dir: %w", err)
	}

	err = os.WriteFile(configFile, []byte(content), 0600)
	if err != nil {
		return fmt.Errorf("failed to write mlvpn config: %w", err)
	}

	return nil
}

// GenerateMlvpnService creates the systemd service file for MLVPN.
func GenerateMlvpnService(serviceFile, mlvpnBin, targetUser, configFile, role string) error {
	var description, execStartPost string
	target := "multi-user.target"

	if role == "CLIENT" {
		description = "VLX FrameFlow MLVPN Bonding Client"
		target = "default.target"
		execStartPost = `
ExecStartPost=/bin/sleep 2
ExecStartPost=-/usr/bin/sudo /usr/sbin/ip rule del ipproto udp dport 8890 table 100
ExecStartPost=/usr/bin/sudo /usr/sbin/ip rule add ipproto udp dport 8890 table 100
ExecStartPost=-/usr/bin/sudo /usr/sbin/ip route add default dev mlvpn0 table 100`
	} else {
		description = "VLX FrameFlow MLVPN Bonding Server"
		execStartPost = ""
	}

	content := fmt.Sprintf(`[Unit]
Description=%s
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=/usr/bin/sudo %s -u %s -c %s%s
Restart=always
RestartSec=3

[Install]
WantedBy=%s
`, description, mlvpnBin, targetUser, configFile, execStartPost, target)

	serviceDir := filepath.Dir(serviceFile)
	err := os.MkdirAll(serviceDir, 0755)
	if err != nil {
		return fmt.Errorf("failed to create service dir: %w", err)
	}

	err = os.WriteFile(serviceFile, []byte(content), 0644)
	if err != nil {
		return fmt.Errorf("failed to write mlvpn service: %w", err)
	}

	return nil
}

// CheckMptcpKernel checks if MPTCP is enabled in the kernel via sysctl.
func CheckMptcpKernel() (bool, error) {
	// Execute `sysctl net.mptcp.enabled`
	output, err := sysutils.RunCommand(10*time.Second, "sysctl", "net.mptcp.enabled")
	if err != nil {
		return false, fmt.Errorf("failed to check mptcp kernel: %w", err)
	}

	// Assuming output like: "net.mptcp.enabled = 1\n"
	if strings.Contains(output, "= 1") {
		return true, nil
	}
	return false, nil
}

func GetProfileVar(file, key string) string {
	content, err := os.ReadFile(file)
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(content), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, key+"=") {
			val := strings.TrimPrefix(line, key+"=")
			val = strings.Trim(val, "\"")
			return val
		}
	}
	return ""
}

func EnsureProfileVar(file, key, value string) error {
	content, err := os.ReadFile(file)
	var lines []string
	if err == nil {
		lines = strings.Split(string(content), "\n")
	}

	found := false
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), key+"=") {
			lines[i] = fmt.Sprintf("%s=%s", key, value)
			found = true
			break
		}
	}
	if !found {
		lines = append(lines, fmt.Sprintf("%s=%s", key, value))
	}

	return os.WriteFile(file, []byte(strings.Join(lines, "\n")), 0644)
}

func SetupMptcpProxy() error {
	sysutils.Info("Setting up MPTCP Proxy (Shadowsocks + v2ray-plugin)...")

	sysutils.RunCommand(10*time.Minute, "apt-get", "install", "-y", "jq", "curl", "wget", "mptcpd", "mptcpd-plugins", "mptcpize")

	err := sysutils.InstallShadowsocks()
	if err != nil {
		sysutils.Error("Failed to install shadowsocks")
	}

	_, err = sysutils.RunCommand(10*time.Minute, "apt-get", "install", "-y", "shadowsocks-v2ray-plugin")
	if err == nil {
		sysutils.Success("v2ray-plugin installed via apt.")
	} else {
		sysutils.Warning("v2ray-plugin apt install failed, falling back to manual wget download")
		v2rayArch := "amd64"
		if runtime.GOARCH == "arm64" {
			v2rayArch = "arm64"
		}
		url := fmt.Sprintf("https://github.com/shadowsocks/v2ray-plugin/releases/download/v1.3.2/v2ray-plugin-linux-%s-v1.3.2.tar.gz", v2rayArch)
		cmdStr := fmt.Sprintf("wget %s -O /tmp/v2ray.tar.gz && tar -xf /tmp/v2ray.tar.gz -C /tmp && mv /tmp/v2ray-plugin_linux_%s /usr/local/bin/v2ray-plugin && chmod +x /usr/local/bin/v2ray-plugin", url, v2rayArch)
		_, err = sysutils.RunCommand(10*time.Minute, "bash", "-c", cmdStr)
		if err != nil {
			sysutils.Error("Failed to download and install v2ray-plugin manually")
		} else {
			sysutils.Success("v2ray-plugin installed via manual download.")
		}
	}

	vlxDir := os.Getenv("VLXsuite_DIR")
	if vlxDir == "" {
		vlxDir = "/opt/VLX_FrameFlow"
	}
	settingsFile := filepath.Join(vlxDir, "etc", "frameflow.settings")

	proxyPass := GetProfileVar(settingsFile, "MPTCP_PROXY_PASS")
	if proxyPass == "" {
		proxyPass = "default_mptcp_pass"
		EnsureProfileVar(settingsFile, "MPTCP_PROXY_PASS", fmt.Sprintf("\"%s\"", proxyPass))
	}

	role := os.Getenv("FRAMEFLOW_ROLE")

	os.MkdirAll("/etc/shadowsocks-libev", 0755)

	var serviceFile string
	var jsonContent string
	var serviceContent string

	if role == "CLIENT" {
		serviceFile = "/etc/systemd/user/frameflow-mptcp-proxy.service"

		ssIPs := GetProfileVar(settingsFile, "SHADOWSOCKS_SERVER_IPS")
		if ssIPs == "" {
			ssIPs = GetProfileVar(settingsFile, "MLVPN_SERVER_IP")
		}
		if ssIPs == "" {
			ssIPs = "127.0.0.1"
		}

		// Dynamically format as JSON string or JSON array
		var serverJson string
		if strings.Contains(ssIPs, ",") {
			parts := strings.Split(ssIPs, ",")
			var quoted []string
			for _, p := range parts {
				quoted = append(quoted, fmt.Sprintf(`"%s"`, strings.TrimSpace(p)))
			}
			serverJson = fmt.Sprintf("[%s]", strings.Join(quoted, ", "))
		} else {
			serverJson = fmt.Sprintf(`"%s"`, strings.TrimSpace(ssIPs))
		}

		// Inject serverJson without surrounding quotes in the format string
		jsonContent = fmt.Sprintf(`{
    "server":%s,
    "server_port":8388,
    "local_address":"::",
    "local_port":1080,
    "password":"%s",
    "timeout":300,
    "method":"aes-256-gcm",
    "plugin":"v2ray-plugin",
    "plugin_opts":"mux=8"
}`, serverJson, proxyPass)

		serviceContent = `[Unit]
Description=VLX FrameFlow MPTCP Proxy Client
After=network-online.target
[Service]
Type=simple
ExecStart=mptcpize run ss-redir -c /etc/shadowsocks-libev/mptcp.json
Restart=always
[Install]
WantedBy=default.target
`
	} else {
		serviceFile = "/etc/systemd/system/frameflow-mptcp-proxy.service"
		jsonContent = fmt.Sprintf(`{
    "server":"::",
    "server_port":8388,
    "password":"%s",
    "timeout":300,
    "method":"aes-256-gcm",
    "plugin":"v2ray-plugin",
    "plugin_opts":"server;mux=8"
}`, proxyPass)

		serviceContent = `[Unit]
Description=VLX FrameFlow MPTCP Proxy Server
After=network-online.target
[Service]
Type=simple
ExecStart=mptcpize run ss-server -c /etc/shadowsocks-libev/mptcp.json
Restart=always
[Install]
WantedBy=multi-user.target
`
	}

	os.WriteFile("/etc/shadowsocks-libev/mptcp.json", []byte(jsonContent), 0644)

	os.MkdirAll(filepath.Dir(serviceFile), 0755)
	os.WriteFile(serviceFile, []byte(serviceContent), 0644)

	sysutils.RunCommand(10*time.Second, "systemctl", "disable", "--now", "shadowsocks-libev.service")

	if role == "SERVER" {
		sysutils.RunCommand(10*time.Second, "systemctl", "daemon-reload")
		sysutils.RunCommand(10*time.Second, "systemctl", "enable", "frameflow-mptcp-proxy.service")
	} else {
		targetUser, _ := sysutils.GetInstalledUser()
		if targetUser == "" || targetUser == "root" {
			targetUser = "nobody"
		}
		sysutils.RunCommand(10*time.Second, "su", "-", targetUser, "-c", "XDG_RUNTIME_DIR=/run/user/$(id -u) DBUS_SESSION_BUS_ADDRESS=unix:path=/run/user/$(id -u)/bus systemctl --user daemon-reload")
		sysutils.RunCommand(10*time.Second, "su", "-", targetUser, "-c", "XDG_RUNTIME_DIR=/run/user/$(id -u) DBUS_SESSION_BUS_ADDRESS=unix:path=/run/user/$(id -u)/bus systemctl --user enable frameflow-mptcp-proxy.service")
	}

	sysutils.Success("MPTCP Proxy configured.")
	return nil
}

func SetupMlvpnBonding() error {
	sysutils.Info("Setting up MLVPN Bonding (UDP)...")

	sysutils.InstallMlvpn()

	vlxDir := os.Getenv("VLXsuite_DIR")
	if vlxDir == "" {
		vlxDir = "/opt/VLX_FrameFlow"
	}
	settingsFile := filepath.Join(vlxDir, "etc", "frameflow.settings")

	mlvpnKey := GetProfileVar(settingsFile, "MLVPN_KEY")
	if mlvpnKey == "" {
		mlvpnKey = sysutils.AskInput("Enter MLVPN Password")
		if mlvpnKey == "" {
			mlvpnKey = "default_mlvpn_key"
		}
		EnsureProfileVar(settingsFile, "MLVPN_KEY", fmt.Sprintf("\"%s\"", mlvpnKey))
	}

	configDir := "/etc/mlvpn"
	configFile := filepath.Join(configDir, "mlvpn.conf")
	updownScript := filepath.Join(configDir, "mlvpn_updown.sh")

	role := os.Getenv("FRAMEFLOW_ROLE")
	vpsIP := GetProfileVar(settingsFile, "MLVPN_SERVER_IP")
	if vpsIP == "" {
		vpsIP = "127.0.0.1" // fallback
	}

	GenerateMlvpnConfig(configFile, updownScript, mlvpnKey, role, vpsIP)
	os.Chmod(configFile, 0600)

	updownContent := `#!/bin/sh
DEVICE="$1"
STATUS="$2"
LOG=/tmp/mlvpn_${DEVICE}.log

[ -z "$STATUS" ] || [ -z "$DEVICE" ] || [ -z "$MTU" ] && exit 1

link_up()
{
    ip link set dev $DEVICE mtu $MTU up
    if [ ! -z "$IP4" ]; then
        ip -4 addr add $IP4 dev $DEVICE
    fi
    if [ ! -z "$IP6" ]; then
        ip -6 addr add $IP6 dev $DEVICE
    fi
}
link_down()
{
    ip link set dev $DEVICE down
}
route_add()
{
    family=$1
    route=$2
    via=""
    if [ "$family" = "4" ]; then
        [ -z $IP4_GATEWAY ] || via="via $IP4_GATEWAY"
        ip -4 route add $route $via dev $DEVICE
    elif [ "$family" = "6" ]; then
        [ -z $IP6_GATEWAY ] || via="via $IP6_GATEWAY"
        ip -6 route add $route $via dev $DEVICE
    fi
}

(
TIMESTAMP=$(date "+%Y-%m-%dT%H:%M:%S")
ECHO="echo ${TIMESTAMP} "
[ "$MTU" -gt 1452 ] && (echo "MTU set too high."; exit 1)
[ "$MTU" -lt 100 ] && (echo "MTU set too low."; exit 1)
case "$STATUS" in
    "tuntap_up")
        $ECHO "$DEVICE up"
        link_up
        for r in $IP4_ROUTES; do
            route_add 4 $r
        done
        for r in $IP6_ROUTES; do
            route_add 6 $r
        done
    ;;
    "tuntap_down")
        $ECHO "$DEVICE down"
        link_down
    ;;
    "rtun_up")
        $ECHO "tunnel [$3] is up"
        ;;
    "rtun_down")
        $ECHO "tunnel [$3] is down"
        ;;
esac

) >> $LOG 2>&1

exit 0
`
	os.WriteFile(updownScript, []byte(updownContent), 0700)

	mlvpnBin, _ := sysutils.RunCommand(10*time.Second, "command", "-v", "mlvpn")
	mlvpnBin = strings.TrimSpace(mlvpnBin)
	if mlvpnBin == "" {
		mlvpnBin = "/usr/local/sbin/mlvpn"
	}

	targetUser, _ := sysutils.GetInstalledUser()
	if targetUser == "" || targetUser == "root" {
		targetUser = "nobody"
	}

	sysutils.RunCommand(10*time.Second, "chown", fmt.Sprintf("%s:root", targetUser), configDir, configFile, updownScript)

	var serviceFile string
	if role == "SERVER" {
		serviceFile = "/etc/systemd/system/frameflow-mlvpn.service"
	} else {
		serviceFile = "/etc/systemd/user/frameflow-mlvpn.service"
	}
	GenerateMlvpnService(serviceFile, mlvpnBin, targetUser, configFile, role)

	if role == "SERVER" {
		sysutils.RunCommand(10*time.Second, "systemctl", "daemon-reload")
		sysutils.RunCommand(10*time.Second, "systemctl", "enable", "frameflow-mlvpn.service")
	} else {
		sysutils.RunCommand(10*time.Second, "su", "-", targetUser, "-c", "XDG_RUNTIME_DIR=/run/user/$(id -u) DBUS_SESSION_BUS_ADDRESS=unix:path=/run/user/$(id -u)/bus systemctl --user daemon-reload")
		sysutils.RunCommand(10*time.Second, "su", "-", targetUser, "-c", "XDG_RUNTIME_DIR=/run/user/$(id -u) DBUS_SESSION_BUS_ADDRESS=unix:path=/run/user/$(id -u)/bus systemctl --user enable frameflow-mlvpn.service")
	}

	if role == "CLIENT" {
		if u, err := user.Lookup(targetUser); err == nil && u.Gid != "" {
			confPath := "/etc/sysctl.d/99-ping-frameflow.conf"
			os.WriteFile(confPath, []byte(fmt.Sprintf("net.ipv4.ping_group_range = %s %s\n", u.Gid, u.Gid)), 0644)
			sysutils.RunCommand(10*time.Second, "sysctl", "-p", confPath)
		}

		// Ultimate fallback for non-root ping capabilities
		sysutils.RunCommand(10*time.Second, "setcap", "cap_net_raw+p", "/bin/ping")
		sysutils.RunCommand(10*time.Second, "setcap", "cap_net_raw+p", "/usr/bin/ping")

		sysutils.Success("MLVPN Bonding (Client) configured.")
	} else {
		sysutils.Success("MLVPN Bonding (Server) configured.")
	}

	return nil
}

// GetBondingStatus returns a formatted string containing the status of the bonding components.
func GetBondingStatus() string {
	role := os.Getenv("FRAMEFLOW_ROLE")
	var err error

	// Check MPTCP Proxy (Shadowsocks)
	if role == "SERVER" {
		_, err = sysutils.RunCommand(10*time.Second, "systemctl", "is-active", "--quiet", "frameflow-mptcp-proxy.service")
	} else {
		_, err = sysutils.RunCommand(10*time.Second, "systemctl", "--user", "is-active", "--quiet", "frameflow-mptcp-proxy.service")
	}
	mptcpStatus := "\033[32mActive\033[0m"
	if err != nil {
		mptcpStatus = "\033[31mInactive\033[0m"
	}
	res := fmt.Sprintf("MPTCP Proxy (Shadowsocks): %s\n", mptcpStatus)

	// Check MLVPN Tunnel (mlvpn0)
	if role == "SERVER" {
		_, err = sysutils.RunCommand(10*time.Second, "systemctl", "is-active", "--quiet", "frameflow-mlvpn.service")
	} else {
		_, err = sysutils.RunCommand(10*time.Second, "systemctl", "--user", "is-active", "--quiet", "frameflow-mlvpn.service")
	}
	mlvpnStatus := "\033[32mConnected\033[0m"
	if err != nil {
		mlvpnStatus = "\033[31mDisconnected\033[0m"
	}
	res += fmt.Sprintf("MLVPN Tunnel (mlvpn0): %s\n", mlvpnStatus)

	return res
}
