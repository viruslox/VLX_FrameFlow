package network

import (
	"crypto/rand"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/viruslox/vlx_frameflow/internal/sysutils"
)

func getInstalledUser() string {
	vlxSuiteDir := os.Getenv("VLXsuite_DIR")
	if vlxSuiteDir == "" {
		vlxSuiteDir = "/opt/VLX_FrameFlow"
	}
	confFile := filepath.Join(vlxSuiteDir, "config", "FrameFlow_conf.sh")
	data, err := os.ReadFile(confFile)
	if err == nil {
		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			if strings.HasPrefix(line, "DEDICATED_USER=") {
				val := strings.TrimPrefix(line, "DEDICATED_USER=")
				val = strings.Trim(val, "\"'")
				return val
			}
		}
	}
	return "frameflow"
}

func generateSecurePassword() string {
	const chars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "FrameFlow123!" // fallback if rand fails
	}
	for i, b := range bytes {
		bytes[i] = chars[b%byte(len(chars))]
	}
	return string(bytes)
}

func getAccesspointPass() string {
	vlxSuiteDir := os.Getenv("VLXsuite_DIR")
	if vlxSuiteDir == "" {
		vlxSuiteDir = "/opt/VLX_FrameFlow"
	}

	settingsPath := filepath.Join(vlxSuiteDir, "etc", "frameflow.settings")
	data, err := os.ReadFile(settingsPath)
	if err == nil {
		var newLines []string
		var foundPass string
		var passFound bool

		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			trimmedLine := strings.TrimSpace(line)
			if strings.HasPrefix(trimmedLine, "AP_PASSWORD=") {
				passFound = true
				val := strings.TrimPrefix(trimmedLine, "AP_PASSWORD=")
				val = strings.Trim(val, "\"'")
				if val != "" {
					foundPass = val
					newLines = append(newLines, line)
				} else {
					foundPass = generateSecurePassword()
					newLines = append(newLines, fmt.Sprintf(`AP_PASSWORD="%s"`, foundPass))
				}
			} else {
				newLines = append(newLines, line)
			}
		}

		if passFound {
			if foundPass != "" {
				// If we generated a new password because it was empty, write it back
				if len(lines) > 0 {
					os.WriteFile(settingsPath, []byte(strings.Join(newLines, "\n")), 0644)
				}
				return foundPass
			}
		} else {
			// AP_PASSWORD is not in the file, append it
			foundPass = generateSecurePassword()
			newLines = append(newLines, fmt.Sprintf(`AP_PASSWORD="%s"`, foundPass))
			if len(lines) > 0 {
				os.WriteFile(settingsPath, []byte(strings.Join(newLines, "\n")), 0644)
			}
			return foundPass
		}
	}

	// Fallback to generating if the file doesn't exist
	pass := generateSecurePassword()
	os.WriteFile(settingsPath, []byte(fmt.Sprintf(`AP_PASSWORD="%s"`+"\n", pass)), 0644)
	return pass
}

// GetFirstWifiInterface discovers and returns the first available wireless interface name (e.g., wlan0).
// It searches /sys/class/net and looks for the "wireless" subdirectory.
func GetFirstWifiInterface(sysClassNetDir string) (string, error) {
	if sysClassNetDir == "" {
		sysClassNetDir = "/sys/class/net"
	}

	entries, err := os.ReadDir(sysClassNetDir)
	if err != nil {
		return "", fmt.Errorf("failed to read %s: %w", sysClassNetDir, err)
	}

	for _, entry := range entries {
		if entry.IsDir() || entry.Type()&os.ModeSymlink != 0 {
			wirelessPath := filepath.Join(sysClassNetDir, entry.Name(), "wireless")
			info, err := os.Stat(wirelessPath)
			if err == nil && info.IsDir() {
				return entry.Name(), nil
			}
		}
	}

	return "", fmt.Errorf("no wireless interface found")
}

// CreateAPProfile generates the systemd-networkd profile for an access point.
func CreateAPProfile(iface, apProfileDir string) error {
	dnsServers := os.Getenv("DNS_SERVERS")
	if dnsServers == "" {
		dnsServers = "8.8.8.8 1.1.1.1"
	}

	dnsServersV6 := os.Getenv("DNS_SERVERS_V6")
	if dnsServersV6 == "" {
		dnsServersV6 = "2001:4860:4860::8888 2606:4700:4700::1111"
	}

	content := fmt.Sprintf(`[Match]
Name=%s
WLANMode=ap

[Network]
Address=192.168.168.1/24
Address=fd42:42:42::1/64
DHCPServer=yes
IPv6SendRA=yes
IPMasquerade=yes

[DHCPServer]
EmitRouter=yes
EmitDNS=yes
DNS=%s
PoolOffset=100
PoolSize=50
MaxLeaseTimeSec=12h

[IPv6SendRA]
RouterPreference=high
DNS=%s
Managed=no
OtherConfig=yes

[Link]
WiFiPowerSave=disable
`, iface, dnsServers, dnsServersV6)

	err := os.MkdirAll(apProfileDir, 0755)
	if err != nil {
		return fmt.Errorf("failed to create ap profile dir: %w", err)
	}

	filename := fmt.Sprintf("40-%s-ap.network", iface)
	filePath := filepath.Join(apProfileDir, filename)

	err = os.WriteFile(filePath, []byte(content), 0644)
	if err != nil {
		return fmt.Errorf("failed to write ap profile: %w", err)
	}

	return nil
}

// CreateHostapdConfig generates the hostapd.conf file.
func CreateHostapdConfig(iface, hostapdDir, hostname, password string) error {
	content := fmt.Sprintf(`interface=%s
driver=nl80211
hw_mode=g
channel=6
ieee80211n=1
wmm_enabled=1
country_code=IT
macaddr_acl=0
auth_algs=1
ignore_broadcast_ssid=0
ssid=VLX_%s
wpa=2
wpa_passphrase=%s
wpa_key_mgmt=WPA-PSK
wpa_pairwise=TKIP CCMP
rsn_pairwise=CCMP
ieee80211w=0
wpa_group_rekey=86400
`, iface, hostname, password)

	err := os.MkdirAll(hostapdDir, 0755)
	if err != nil {
		return fmt.Errorf("failed to create hostapd dir: %w", err)
	}

	filePath := filepath.Join(hostapdDir, "hostapd.conf")

	err = os.WriteFile(filePath, []byte(content), 0644)
	if err != nil {
		return fmt.Errorf("failed to write hostapd config: %w", err)
	}

	return nil
}

// CreateManagedProfile generates the systemd-networkd profile for a managed interface.
func CreateManagedProfile(iface, outputFile string, metricOffset int) error {
	content := fmt.Sprintf(`[Match]
Name=%s

[Network]
DHCP=yes
IPMasquerade=yes
WPAConfigFile=/etc/wpa_supplicant/wpa_supplicant-%s.conf

[DHCPv4]
RouteMetric=%d

[IPv6AcceptRA]
RouteMetric=%d

[Link]
WiFiPowerSave=disable
`, iface, iface, 200+metricOffset, 200+metricOffset)

	dir := filepath.Dir(outputFile)
	err := os.MkdirAll(dir, 0755)
	if err != nil {
		return fmt.Errorf("failed to create managed profile dir: %w", err)
	}

	err = os.WriteFile(outputFile, []byte(content), 0644)
	if err != nil {
		return fmt.Errorf("failed to write managed profile: %w", err)
	}

	return nil
}

// CacheNMConnections simulates caching NetworkManager connections to get SSIDs and PSKs.
// In Go, since we need to parse NetworkManager connections from /etc/NetworkManager/system-connections,
// we'll implement a basic version. For simplicity and to match the bash behavior without complex ini parsing,
// we can read files and use simple string matching or regex.
// To exactly match the bash logic structure, we'll keep the function signature simple.
func CacheNMConnections(nmDir string) ([]string, []string) {
	if nmDir == "" {
		nmDir = "/etc/NetworkManager/system-connections"
	}
	var ssids []string
	var psks []string

	entries, err := os.ReadDir(nmDir)
	if err != nil {
		return ssids, psks
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			content, err := os.ReadFile(filepath.Join(nmDir, entry.Name()))
			if err == nil {
				lines := strings.Split(string(content), "\n")
				isWifi := false
				ssid := ""
				psk := ""
				for _, line := range lines {
					line = strings.TrimSpace(line)
					if strings.HasPrefix(line, "type=wifi") {
						isWifi = true
					}
					if strings.HasPrefix(line, "ssid=") {
						ssid = strings.TrimPrefix(line, "ssid=")
					}
					if strings.HasPrefix(line, "psk=") {
						psk = strings.TrimPrefix(line, "psk=")
					}
				}
				if isWifi && ssid != "" && psk != "" {
					ssids = append(ssids, ssid)
					psks = append(psks, psk)
				}
			}
		}
	}
	return ssids, psks
}

// WriteCachedNMConnections writes the cached connections to a wpa_supplicant config file.
func WriteCachedNMConnections(wpaConf string, ssids []string, psks []string) error {
	f, err := os.OpenFile(wpaConf, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0600)
	if err != nil {
		return err
	}
	defer f.Close()

	contentBytes, err := os.ReadFile(wpaConf)
	contentStr := ""
	if err == nil {
		contentStr = string(contentBytes)
	}

	var newNetworks strings.Builder
	for i, ssid := range ssids {
		escapedSsid := strings.ReplaceAll(ssid, "\\", "\\\\")
		escapedSsid = strings.ReplaceAll(escapedSsid, "\"", "\\\"")
		psk := psks[i]
		escapedPsk := strings.ReplaceAll(psk, "\\", "\\\\")
		escapedPsk = strings.ReplaceAll(escapedPsk, "\"", "\\\"")

		searchStr := fmt.Sprintf("ssid=\"%s\"", escapedSsid)
		if !strings.Contains(contentStr, searchStr) && !strings.Contains(newNetworks.String(), searchStr) {
			newNetworks.WriteString("network={\n")
			newNetworks.WriteString(fmt.Sprintf("    ssid=\"%s\"\n", escapedSsid))
			newNetworks.WriteString(fmt.Sprintf("    psk=\"%s\"\n", escapedPsk))
			newNetworks.WriteString("}\n")
		}
	}

	if newNetworks.Len() > 0 {
		_, err = f.WriteString(newNetworks.String())
		if err != nil {
			return err
		}
	}
	return nil
}

// OverrideHostapdService creates the hostapd service override.
func OverrideHostapdService(systemdSystemDir string) error {
	if systemdSystemDir == "" {
		systemdSystemDir = "/etc/systemd/system"
	}
	serviceFile := filepath.Join(systemdSystemDir, "hostapd.service")
	content := `[Unit]
Description=Access point and authentication server for Wi-Fi and Ethernet
After=network.target
ConditionFileNotEmpty=/etc/hostapd/hostapd.conf

[Service]
Type=exec
Restart=on-failure
RestartSec=2
Environment=DAEMON_CONF=/etc/hostapd/hostapd.conf
EnvironmentFile=-/etc/default/hostapd
ExecStart=/usr/sbin/hostapd -P /run/hostapd.pid $DAEMON_OPTS ${DAEMON_CONF}

[Install]
WantedBy=multi-user.target
`
	err := os.MkdirAll(systemdSystemDir, 0755)
	if err != nil {
		return err
	}
	err = os.WriteFile(serviceFile, []byte(content), 0644)
	if err == nil {
		sysutils.RunCommand(10*time.Second, "systemctl", "daemon-reload")
	}
	return err
}

// CreateWifiProfiles generates Wi-Fi profiles for all wireless interfaces.
func CreateWifiProfiles(sysClassNetDir, systemdNetworkDir, normProfileDir, wpaSupplicantDir string) error {
	fmt.Println("[INFO] Generating Wi-Fi profiles...")
	if sysClassNetDir == "" {
		sysClassNetDir = "/sys/class/net"
	}
	if systemdNetworkDir == "" {
		systemdNetworkDir = "/etc/systemd/network"
	}
	if normProfileDir == "" {
		normProfileDir = "" // NORM_PROFILE equivalent, usually maybe /etc/systemd/network or another dir depending on bash context
		// Wait, NORM_PROFILE is just an env variable. Let's get it from env or default
		if envNorm := os.Getenv("NORM_PROFILE"); envNorm != "" {
			normProfileDir = envNorm
		} else {
			normProfileDir = "/etc/systemd/network/profiles/normal" // fallback
		}
	}
	if wpaSupplicantDir == "" {
		wpaSupplicantDir = "/etc/wpa_supplicant"
	}

	entries, err := os.ReadDir(sysClassNetDir)
	if err != nil {
		return err
	}

	var interfaces []string
	for _, entry := range entries {
		if entry.IsDir() || entry.Type()&os.ModeSymlink != 0 {
			wirelessPath := filepath.Join(sysClassNetDir, entry.Name(), "wireless")
			info, err := os.Stat(wirelessPath)
			if err == nil && info.IsDir() {
				interfaces = append(interfaces, entry.Name())
			}
		} else {
			// Workaround for sysfs entries that aren't reported as dirs but point to dirs
			wirelessPath := filepath.Join(sysClassNetDir, entry.Name(), "wireless")
			info, err := os.Stat(wirelessPath)
			if err == nil && info.IsDir() {
				interfaces = append(interfaces, entry.Name())
			}
		}
	}

	if len(interfaces) == 0 {
		fmt.Println("[WARNING] No wireless interfaces found.")
		return nil
	}

	sort.Strings(interfaces)

	ssids, psks := CacheNMConnections("")

	metricOffset := 1
	for i, iface := range interfaces {
		if i == 0 {
			managedProf := filepath.Join(normProfileDir, fmt.Sprintf("20-%s-managed.network", iface))
			CreateManagedProfile(iface, managedProf, metricOffset)

			// copy it
			destManagedProf := filepath.Join(systemdNetworkDir, fmt.Sprintf("20-%s-managed.network", iface))
			content, _ := os.ReadFile(managedProf)
			os.MkdirAll(systemdNetworkDir, 0755)
			os.WriteFile(destManagedProf, content, 0644)

			wpaConf := filepath.Join(wpaSupplicantDir, fmt.Sprintf("wpa_supplicant-%s.conf", iface))
			os.MkdirAll(wpaSupplicantDir, 0755)
			WriteCachedNMConnections(wpaConf, ssids, psks)
			os.Chmod(wpaConf, 0600)

			// systemctl enable
			cmd := exec.Command("systemctl", "enable", fmt.Sprintf("wpa_supplicant@%s.service", iface))
			cmd.Run()

			cmd = exec.Command("systemctl", "unmask", "hostapd")
			cmd.Run()

			apProfileDir := os.Getenv("AP_PROFILE")
			if apProfileDir == "" {
				apProfileDir = "/etc/systemd/network/profiles/ap-bonding"
			}
			CreateAPProfile(iface, apProfileDir)

			hostname, _ := os.Hostname()
			pass := getAccesspointPass()
			hostapdDir := os.Getenv("HOSTAPD_DIR")
			if hostapdDir == "" {
				hostapdDir = "/etc/hostapd"
			}
			CreateHostapdConfig(iface, hostapdDir, hostname, pass)

		} else {
			managedProf := filepath.Join(systemdNetworkDir, fmt.Sprintf("20-%s-managed.network", iface))
			CreateManagedProfile(iface, managedProf, metricOffset)

			wpaConf := filepath.Join(wpaSupplicantDir, fmt.Sprintf("wpa_supplicant-%s.conf", iface))
			os.MkdirAll(wpaSupplicantDir, 0755)
			WriteCachedNMConnections(wpaConf, ssids, psks)
			os.Chmod(wpaConf, 0600)

			cmd := exec.Command("systemctl", "enable", fmt.Sprintf("wpa_supplicant@%s.service", iface))
			cmd.Run()
		}
		metricOffset++
	}

	OverrideHostapdService("")
	fmt.Println("[SUCCESS] Wi-Fi profiles generated.")
	return nil
}
