package network

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// GenerateNetworkdProfile creates a systemd-networkd profile for a wired interface.
func GenerateNetworkdProfile(iface, tableID, systemdNetworkDir string) error {
	content := fmt.Sprintf(`[Match]
Name=%s

[Link]
EnergyEfficientEthernet=false
RequiredForOnline=routable

[Network]
DHCP=yes
IPMasquerade=yes
Table=%s
SourceRouting=yes

[DHCPv4]
RouteMetric=1%s

[IPv6AcceptRA]
RouteMetric=1%s
`, iface, tableID, tableID, tableID)

	err := os.MkdirAll(systemdNetworkDir, 0755)
	if err != nil {
		return fmt.Errorf("failed to create systemd network dir: %w", err)
	}

	filename := fmt.Sprintf("10-%s.network", iface)
	filePath := filepath.Join(systemdNetworkDir, filename)

	err = os.WriteFile(filePath, []byte(content), 0644)
	if err != nil {
		return fmt.Errorf("failed to write network profile: %w", err)
	}

	return nil
}

// GenerateDispatcherUpScript creates the networkd-dispatcher up script for MPTCP routing.
func GenerateDispatcherUpScript(iface, tableID, dispatcherOnDir string) error {
	content := fmt.Sprintf(`#!/bin/sh

IP_CMD="${IP_CMD:-$(command -v ip)}"

if [ "$IFACE" = "%s" ]; then
    # IPv4 Config
    IPV4_ADDR4=$("$IP_CMD" -4 addr show dev "$IFACE" | grep -oP '(?<=inet\s)\d+(\.\d+){3}')
    if [ -n "$IPV4_ADDR4" ]; then
        "$IP_CMD" mptcp endpoint add $IPV4_ADDR4 dev $IFACE subflow
        "$IP_CMD" -4 rule add table main suppress_prefixlength 0 priority 310%s 2>/dev/null || true
        "$IP_CMD" -4 rule add from "$IPV4_ADDR4" lookup %s priority 320%s

        GW4=$("$IP_CMD" -4 route show default dev "$IFACE" | grep -oP '(?<=via\s)\S+')
        if [ -n "$GW4" ]; then
             "$IP_CMD" route del default dev "$IFACE" scope link 2>/dev/null || true
             "$IP_CMD" route add default via "$GW4" dev "$IFACE" metric 110 2>/dev/null || true
             "$IP_CMD" -4 route add default via "$GW4" dev "$IFACE" table %s 2>/dev/null
        fi

        "$IP_CMD" -4 route show dev "$IFACE" | while read -r subnet rest; do
            case "$rest" in
                *kernel*)
                    "$IP_CMD" -4 route add "$subnet" dev "$IFACE" table %s 2>/dev/null
                    ;;
            esac
        done
    fi

    # IPv6 Config
    IPV6_ADDR6=$("$IP_CMD" -6 addr show dev "$IFACE" | grep -oP '(?<=inet6\s)[\da-f:]+(?=\/)' | head -n1)
    if [ -n "$IPV6_ADDR6" ]; then
        "$IP_CMD" -6 mptcp endpoint add $IPV6_ADDR6 dev $IFACE subflow
        "$IP_CMD" -6 rule add table main suppress_prefixlength 0 priority 310%s 2>/dev/null || true
        "$IP_CMD" -6 rule add from "$IPV6_ADDR6" lookup %s priority 320%s

        sleep 2
        GW6=$("$IP_CMD" -6 route show default dev "$IFACE" table main | grep -oP '(?<=via\s)\S+' | head -n1)
        if [ -n "$GW6" ]; then
             "$IP_CMD" -6 route add default via "$GW6" dev "$IFACE" table %s 2>/dev/null
        fi

        "$IP_CMD" -6 route show dev "$IFACE" | while read -r subnet rest; do
            case "$rest" in
                *kernel*)
                    "$IP_CMD" -6 route add "$subnet" dev "$IFACE" table %s 2>/dev/null
                    ;;
            esac
        done
    fi
fi
`, iface, tableID, tableID, tableID, tableID, tableID, tableID, tableID, tableID, tableID, tableID)

	err := os.MkdirAll(dispatcherOnDir, 0755)
	if err != nil {
		return fmt.Errorf("failed to create dispatcher on dir: %w", err)
	}

	filename := fmt.Sprintf("30-%s-mptcp-subflow.sh", iface)
	filePath := filepath.Join(dispatcherOnDir, filename)

	err = os.WriteFile(filePath, []byte(content), 0755)
	if err != nil {
		return fmt.Errorf("failed to write dispatcher up script: %w", err)
	}

	return nil
}

// GenerateDispatcherDownScript creates the networkd-dispatcher down script for MPTCP routing cleanup.
func GenerateDispatcherDownScript(iface, tableID, dispatcherOffDir string) error {
	content := fmt.Sprintf(`#!/bin/sh

IP_CMD="${IP_CMD:-$(command -v ip)}"

if [ "$IFACE" = "%s" ]; then
    RULES=$("$IP_CMD" rule show)
    while IFS= read -r line || [ -n "$line" ]; do
        case "$line" in
            *"lookup %s"*) "$IP_CMD" rule del lookup %s 2>/dev/null ;;
            *"310%s:"*) "$IP_CMD" rule del priority 310%s 2>/dev/null ;;
        esac
    done <<< "$RULES"

    RULES6=$("$IP_CMD" -6 rule show)
    while IFS= read -r line || [ -n "$line" ]; do
        case "$line" in
            *"lookup %s"*) "$IP_CMD" -6 rule del lookup %s 2>/dev/null ;;
            *"310%s:"*) "$IP_CMD" -6 rule del priority 310%s 2>/dev/null ;;
        esac
    done <<< "$RULES6"

    MPTCP_ENDPOINTS=$("$IP_CMD" mptcp endpoint show)
    while IFS= read -r line || [ -n "$line" ]; do
        case "$line" in
            *"dev $IFACE"*)
                rest="${line#*id }"
                id="${rest%%%% *}"
                if [ -n "$id" ]; then
                    "$IP_CMD" mptcp endpoint delete id "$id"
                fi
                ;;
        esac
    done <<< "$MPTCP_ENDPOINTS"

    MPTCP6_ENDPOINTS=$("$IP_CMD" -6 mptcp endpoint show)
    while IFS= read -r line || [ -n "$line" ]; do
        case "$line" in
            *"dev $IFACE"*)
                rest="${line#*id }"
                id="${rest%%%% *}"
                if [ -n "$id" ]; then
                    "$IP_CMD" -6 mptcp endpoint delete id "$id"
                fi
                ;;
        esac
    done <<< "$MPTCP6_ENDPOINTS"
    "$IP_CMD" route flush table %s 2>/dev/null
    "$IP_CMD" -6 route flush table %s 2>/dev/null
fi
`, iface, tableID, tableID, tableID, tableID, tableID, tableID, tableID, tableID, tableID, tableID)

	err := os.MkdirAll(dispatcherOffDir, 0755)
	if err != nil {
		return fmt.Errorf("failed to create dispatcher off dir: %w", err)
	}

	filename := fmt.Sprintf("30-%s-mptcp-stop.sh", iface)
	filePath := filepath.Join(dispatcherOffDir, filename)

	err = os.WriteFile(filePath, []byte(content), 0755)
	if err != nil {
		return fmt.Errorf("failed to write dispatcher down script: %w", err)
	}

	return nil
}

// CreateNetworkProfiles generates interface profiles for wired connections.
func CreateNetworkProfiles(sysClassNetDir, iproute2Dir, dispatcherOnDir, dispatcherOffDir, systemdNetworkDir string) error {
	fmt.Println("[INFO] Generating interface profiles...")

	if sysClassNetDir == "" {
		sysClassNetDir = "/sys/class/net"
	}
	if iproute2Dir == "" {
		iproute2Dir = "/etc/iproute2"
	}
	if dispatcherOnDir == "" {
		dispatcherOnDir = "/etc/networkd-dispatcher/routable.d" // standard dispatcher path based on features
	}
	// Looking at the bash script, NORM_PROFILE and DISPATCHER_ON_DIR were globals or passed.
	// The bash says `mkdir -p "$DISPATCHER_ON_DIR" "$DISPATCHER_OFF_DIR"`
	// Let's use env vars if provided, else default
	if envOn := os.Getenv("DISPATCHER_ON_DIR"); envOn != "" {
		dispatcherOnDir = envOn
	}
	if dispatcherOffDir == "" {
		dispatcherOffDir = "/etc/networkd-dispatcher/off.d"
	}
	if envOff := os.Getenv("DISPATCHER_OFF_DIR"); envOff != "" {
		dispatcherOffDir = envOff
	}
	if systemdNetworkDir == "" {
		systemdNetworkDir = "/etc/systemd/network"
	}
	if envNet := os.Getenv("SYSTEMD_NETWORK"); envNet != "" {
		systemdNetworkDir = envNet
	}

	os.MkdirAll(iproute2Dir, 0755)
	os.MkdirAll(dispatcherOnDir, 0755)
	os.MkdirAll(dispatcherOffDir, 0755)

	rtTablesFile := filepath.Join(iproute2Dir, "rt_tables")
	// Restore default content
	defaultRtTables := `#
# reserved values
#
255	local
254	main
253	default
0	unspec
#
# local
#
#1	inr.ruhep
`
	os.WriteFile(rtTablesFile, []byte(defaultRtTables), 0644)

	entries, err := os.ReadDir(sysClassNetDir)
	if err != nil {
		return err
	}

	ifaceIndex := 1

	rtf, err := os.OpenFile(rtTablesFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer rtf.Close()

	for _, entry := range entries {
		iface := entry.Name()

		// Skip loopback, bonds, tun interfaces, and wireless
		if iface == "lo" || strings.Contains(iface, "bond") || strings.Contains(iface, "tun") || strings.Contains(iface, "mlvpn") {
			continue
		}

		wirelessPath := filepath.Join(sysClassNetDir, iface, "wireless")
		if info, err := os.Stat(wirelessPath); err == nil && info.IsDir() {
			continue
		}

		tableID := fmt.Sprintf("%d", ifaceIndex*10)

		GenerateNetworkdProfile(iface, tableID, systemdNetworkDir)
		GenerateDispatcherUpScript(iface, tableID, dispatcherOnDir)
		GenerateDispatcherDownScript(iface, tableID, dispatcherOffDir)

		rtf.WriteString(fmt.Sprintf("%s    T_%s\n", tableID, iface))

		onScript := filepath.Join(dispatcherOnDir, fmt.Sprintf("30-%s-mptcp-subflow.sh", iface))
		offScript := filepath.Join(dispatcherOffDir, fmt.Sprintf("30-%s-mptcp-stop.sh", iface))
		os.Chmod(onScript, 0755)
		os.Chmod(offScript, 0755)

		ifaceIndex++
	}

	fmt.Println("[SUCCESS] Interface profiles created.")
	return nil
}
