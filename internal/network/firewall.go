package network

import (
	"os"
	"strings"
	"time"

	"github.com/viruslox/vlx_frameflow/internal/sysutils"
)

func ConfigureFirewall(role string) error {
	sysutils.Info("Configuring Firewall...")

	out, _ := sysutils.RunCommand(5*time.Second, "cat", "/etc/default/ufw")
	if !strings.Contains(out, "IPV6=yes") {
		// Native Go file append
		f, err := os.OpenFile("/etc/default/ufw", os.O_APPEND|os.O_WRONLY, 0644)
		if err == nil {
			f.WriteString("IPV6=yes\n")
			f.Close()
		}
	}

	if role == "CLIENT" {
		sysutils.RunCommand(10*time.Second, "ufw", "default", "allow", "routed")
		sysutils.RunCommand(10*time.Second, "ufw", "default", "allow", "outgoing")
		sysutils.RunCommand(10*time.Second, "ufw", "default", "deny", "incoming")

		// Modify /etc/default/ufw using native go read/replace
		contentBytes, err := os.ReadFile("/etc/default/ufw")
		if err == nil {
			content := string(contentBytes)
			content = strings.Replace(content, `DEFAULT_FORWARD_POLICY="DROP"`, `DEFAULT_FORWARD_POLICY="ACCEPT"`, -1)
			content = strings.Replace(content, `DEFAULT_OUTPUT_POLICY="DROP"`, `DEFAULT_OUTPUT_POLICY="ACCEPT"`, -1)
			content = strings.Replace(content, `DEFAULT_INPUT_POLICY="ACCEPT"`, `DEFAULT_INPUT_POLICY="DROP"`, -1)
			os.WriteFile("/etc/default/ufw", []byte(content), 0644)
		}

		sysutils.RunCommand(10*time.Second, "ufw", "allow", "22/tcp")
		sysutils.RunCommand(10*time.Second, "ufw", "allow", "5080/udp")
		sysutils.RunCommand(10*time.Second, "ufw", "allow", "123/udp")
		sysutils.RunCommand(10*time.Second, "ufw", "allow", "8890/udp")
		sysutils.RunCommand(10*time.Second, "ufw", "allow", "1080/tcp")
		sysutils.RunCommand(10*time.Second, "ufw", "allow", "546/udp")
		sysutils.RunCommand(10*time.Second, "ufw", "allow", "547/udp")
		sysutils.RunCommand(10*time.Second, "ufw", "allow", "53")
		sysutils.RunCommand(10*time.Second, "ufw", "allow", "67/udp")
	} else {
		sysutils.RunCommand(10*time.Second, "ufw", "allow", "22/tcp")
	}

	sysutils.Success("Firewall rules applied.")
	return nil
}
