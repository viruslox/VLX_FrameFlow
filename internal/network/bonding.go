package network

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"strconv"
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

// SetupMlvpnBonding dispatches to the multi-peer server path when a peer
// registry (etc/peers.yaml) is present on a SERVER, and otherwise runs the
// historical single-tunnel path (unchanged) for the CLIENT and for servers
// without a registry.
func SetupMlvpnBonding() error {
	if os.Getenv("FRAMEFLOW_ROLE") == "SERVER" {
		peersPath := ServerPeersPath()
		if _, err := os.Stat(peersPath); err == nil {
			return setupMlvpnBondingServerMulti(peersPath)
		}
	}
	return setupMlvpnBondingLegacy()
}

func setupMlvpnBondingLegacy() error {
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

	if role == "CLIENT" {
		id := LoadClientTunnelIdentity(settingsFile)
		GenerateMlvpnClientConfig(configFile, updownScript, mlvpnKey, vpsIP, id)
	} else {
		GenerateMlvpnConfig(configFile, updownScript, mlvpnKey, role, vpsIP)
	}
	os.Chmod(configFile, 0600)

	os.WriteFile(updownScript, []byte(mlvpnUpdownScript), 0700)

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

// GetBondingStatus returns a formatted string containing the status of the
// bonding components. On a SERVER with a peer registry it reports per-peer
// tunnel status; otherwise it uses the historical single-tunnel report.
func GetBondingStatus() string {
	if os.Getenv("FRAMEFLOW_ROLE") == "SERVER" {
		peersPath := ServerPeersPath()
		if _, err := os.Stat(peersPath); err == nil {
			return getBondingStatusServerMulti(peersPath)
		}
	}
	return getBondingStatusLegacy()
}

func getBondingStatusLegacy() string {
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

// mlvpnUpdownScript is the interface up/down helper invoked by mlvpn via its
// statuscommand. It is generic across peers (it acts on $DEVICE supplied by
// mlvpn at runtime) and is shared by the legacy and multi-peer paths.
const mlvpnUpdownScript = `#!/bin/sh
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

// GenerateMlvpnServerPeerConfig writes the server-side mlvpn config for a
// single peer. The interface name, address pair and bind port come from the
// peer's slot-derived (or explicitly overridden) fields.
func GenerateMlvpnServerPeerConfig(configFile, updownScript, key string, p MlvpnPeer) error {
	content := fmt.Sprintf(`[general]
statuscommand = "%s"
mode = "server"
ip4 = "%s/24"
ip4_gateway = "%s"
mtu = 1444
tuntap = "tun"
interface_name = "%s"
timeout = 30
password = "%s"

[mlvpn_link]
bindhost = "0.0.0.0"
bindport = %d
`, updownScript, p.ServerTunIP, p.ClientTunIP, PeerInterface(p.Slot), key, p.Port)

	if err := os.MkdirAll(filepath.Dir(configFile), 0755); err != nil {
		return fmt.Errorf("failed to create config dir: %w", err)
	}
	if err := os.WriteFile(configFile, []byte(content), 0600); err != nil {
		return fmt.Errorf("failed to write mlvpn peer config: %w", err)
	}
	return nil
}

// GenerateMlvpnServerServiceTemplate writes the templated systemd unit
// (frameflow-mlvpn@.service) instantiated once per peer slot. The %i specifier
// resolves to the slot, selecting /etc/mlvpn/mlvpn<slot>.conf.
func GenerateMlvpnServerServiceTemplate(serviceFile, mlvpnBin, targetUser string) error {
	content := fmt.Sprintf(`[Unit]
Description=VLX FrameFlow MLVPN Bonding Server (peer %%i)
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=/usr/bin/sudo %s -u %s -c /etc/mlvpn/mlvpn%%i.conf
Restart=always
RestartSec=3

[Install]
WantedBy=multi-user.target
`, mlvpnBin, targetUser)

	if err := os.MkdirAll(filepath.Dir(serviceFile), 0755); err != nil {
		return fmt.Errorf("failed to create service dir: %w", err)
	}
	if err := os.WriteFile(serviceFile, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write mlvpn service template: %w", err)
	}
	return nil
}

// setupMlvpnBondingServerMulti provisions one mlvpn instance per peer from the
// registry: a templated unit, per-slot config files, per-peer key generation,
// and per-peer firewall openings.
func setupMlvpnBondingServerMulti(peersPath string) error {
	sysutils.Info("Setting up MLVPN Bonding (UDP, multi-peer server)...")
	sysutils.InstallMlvpn()

	peers, err := LoadPeers(peersPath)
	if err != nil {
		return fmt.Errorf("failed to load peer registry: %w", err)
	}
	if len(peers) == 0 {
		sysutils.Warning("Peer registry %s contains no peers; no MLVPN tunnels will be provisioned.", peersPath)
	}

	configDir := "/etc/mlvpn"
	updownScript := filepath.Join(configDir, "mlvpn_updown.sh")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create %s: %w", configDir, err)
	}
	os.WriteFile(updownScript, []byte(mlvpnUpdownScript), 0700)

	mlvpnBin, _ := sysutils.RunCommand(10*time.Second, "command", "-v", "mlvpn")
	mlvpnBin = strings.TrimSpace(mlvpnBin)
	if mlvpnBin == "" {
		mlvpnBin = "/usr/local/sbin/mlvpn"
	}

	targetUser, _ := sysutils.GetInstalledUser()
	if targetUser == "" || targetUser == "root" {
		targetUser = "nobody"
	}

	serviceFile := "/etc/systemd/system/frameflow-mlvpn@.service"
	if err := GenerateMlvpnServerServiceTemplate(serviceFile, mlvpnBin, targetUser); err != nil {
		return fmt.Errorf("failed to write templated mlvpn unit: %w", err)
	}

	// Retire the legacy single-instance unit to avoid an mlvpn0 / udp-5080
	// collision with peer slot 0.
	sysutils.RunCommand(10*time.Second, "systemctl", "disable", "--now", "frameflow-mlvpn.service")

	chownTargets := []string{configDir, updownScript}
	for i := range peers {
		p := &peers[i]
		key, err := EnsurePeerKey(peersPath, p.Slot, p.Key)
		if err != nil {
			return fmt.Errorf("peer slot %d: %w", p.Slot, err)
		}
		p.Key = key

		peerConf := filepath.Join(configDir, fmt.Sprintf("mlvpn%d.conf", p.Slot))
		if err := GenerateMlvpnServerPeerConfig(peerConf, updownScript, key, *p); err != nil {
			return fmt.Errorf("peer slot %d: failed to write config: %w", p.Slot, err)
		}
		os.Chmod(peerConf, 0600)
		chownTargets = append(chownTargets, peerConf)
	}

	chownArgs := append([]string{fmt.Sprintf("%s:root", targetUser)}, chownTargets...)
	sysutils.RunCommand(10*time.Second, "chown", chownArgs...)

	sysutils.RunCommand(10*time.Second, "systemctl", "daemon-reload")
	for _, p := range peers {
		sysutils.RunCommand(10*time.Second, "systemctl", "enable", PeerServiceInstance(p.Slot))
		sysutils.RunCommand(10*time.Second, "ufw", "allow", fmt.Sprintf("%d/udp", p.Port))
		sysutils.Success("MLVPN peer slot %d (%s): %s  srv %s  cli %s  udp/%d",
			p.Slot, p.Name, PeerInterface(p.Slot), p.ServerTunIP, p.ClientTunIP, p.Port)
	}

	sysutils.Success("MLVPN Bonding (multi-peer server) configured for %d peer(s).", len(peers))
	return nil
}

// getBondingStatusServerMulti reports MPTCP proxy status plus per-peer tunnel
// status from the registry.
func getBondingStatusServerMulti(peersPath string) string {
	_, err := sysutils.RunCommand(10*time.Second, "systemctl", "is-active", "--quiet", "frameflow-mptcp-proxy.service")
	mptcpStatus := "\033[32mActive\033[0m"
	if err != nil {
		mptcpStatus = "\033[31mInactive\033[0m"
	}
	res := fmt.Sprintf("MPTCP Proxy (Shadowsocks): %s\n", mptcpStatus)

	peers, err := LoadPeers(peersPath)
	if err != nil {
		res += fmt.Sprintf("MLVPN Peers: \033[31merror loading registry: %v\033[0m\n", err)
		return res
	}
	if len(peers) == 0 {
		res += "MLVPN Peers: none configured\n"
		return res
	}
	for _, p := range peers {
		_, e := sysutils.RunCommand(10*time.Second, "systemctl", "is-active", "--quiet", PeerServiceInstance(p.Slot))
		st := "\033[32mConnected\033[0m"
		if e != nil {
			st = "\033[31mDisconnected\033[0m"
		}
		res += fmt.Sprintf("MLVPN Peer slot %d (%s) [%s]: %s\n", p.Slot, p.Name, PeerInterface(p.Slot), st)
	}
	return res
}

// GenerateMlvpnClientConfig writes the client-side mlvpn config using a
// configurable tunnel identity. The underlay remote host (the server's public
// address, from MLVPN_SERVER_IP) is distinct from the in-tunnel gateway. The
// local interface stays mlvpn0 (a client has a single tunnel), so the service
// unit's routing ExecStartPost is unaffected.
func GenerateMlvpnClientConfig(configFile, updownScript, mlvpnKey, remoteHost string, id ClientTunnelIdentity) error {
	content := fmt.Sprintf(`[general]
statuscommand = "%s"
mode = "client"
ip4 = "%s/24"
ip4_gateway = "%s"
mtu = 1444
tuntap = "tun"
interface_name = "mlvpn0"
timeout = 30
password = "%s"

[mlvpn_link]
bindhost = "0.0.0.0"
remotehost = "%s"
remoteport = %d
`, updownScript, id.ClientTunIP, id.ServerTunIP, mlvpnKey, remoteHost, id.RemotePort)

	if err := os.MkdirAll(filepath.Dir(configFile), 0755); err != nil {
		return fmt.Errorf("failed to create config dir: %w", err)
	}
	if err := os.WriteFile(configFile, []byte(content), 0600); err != nil {
		return fmt.Errorf("failed to write mlvpn client config: %w", err)
	}
	return nil
}

// LoadClientTunnelIdentity reads the client's MLVPN tunnel identity from the
// settings file: MLVPN_SLOT plus optional explicit overrides
// (MLVPN_CLIENT_TUN_IP, MLVPN_SERVER_TUN_IP, MLVPN_REMOTE_PORT). Absent or
// empty values derive from the slot; an absent slot defaults to 0, reproducing
// the historical single-client tunnel.
func LoadClientTunnelIdentity(settingsFile string) ClientTunnelIdentity {
	slot := 0
	if s := strings.TrimSpace(GetProfileVar(settingsFile, "MLVPN_SLOT")); s != "" {
		if n, err := strconv.Atoi(s); err == nil {
			slot = n
		}
	}
	clientIP := strings.TrimSpace(GetProfileVar(settingsFile, "MLVPN_CLIENT_TUN_IP"))
	serverIP := strings.TrimSpace(GetProfileVar(settingsFile, "MLVPN_SERVER_TUN_IP"))
	port := 0
	if s := strings.TrimSpace(GetProfileVar(settingsFile, "MLVPN_REMOTE_PORT")); s != "" {
		if n, err := strconv.Atoi(s); err == nil {
			port = n
		}
	}
	return DeriveClientTunnelIdentity(slot, clientIP, serverIP, port)
}
