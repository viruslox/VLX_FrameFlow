package cmd

import (
	"os"
	"time"

	"github.com/viruslox/vlx_frameflow/internal/network"
	"github.com/viruslox/vlx_frameflow/internal/sysutils"
)

func setupServerComponents() error {
	sysutils.InstallShadowsocks()
	sysutils.InstallMlvpn()
	if err := network.SetupMptcpProxy(); err != nil {
		sysutils.Error("MPTCP proxy setup failed: %v", err)
	}
	if err := network.SetupMlvpnBonding(); err != nil {
		sysutils.Error("MLVPN bonding setup failed: %v", err)
		sysutils.Error("No MLVPN tunnel services were created. Fix the issue above and re-run setup. "+
			"Peer names in %s must be lowercase, DNS-label safe (letters, digits, internal hyphens).", network.ServerPeersPath())
		return err
	}

	sysutils.Info("The following ports are required for the server components:")
	sysutils.Info("- 8889/tcp (mediamtx WEBRTC)")
	sysutils.Info("- 8322/tcp (mediamtx RTSP)")
	sysutils.Info("- 8189/udp (mediamtx WEBRTC)")
	sysutils.Info("- 8890/udp (mediamtx SRT)")
	sysutils.Info("- 5080/udp (MLVPN bonding tunnel)")
	sysutils.Info("- 8388 (Shadowsocks: MPTCP TCP aggregator)")

	if err := os.MkdirAll("/etc/ufw", 0755); err == nil {
		sysutils.Info("Configuring UFW firewall rules...")
		sysutils.RunCommand(10*time.Second, "ufw", "allow", "8889/tcp")
		sysutils.RunCommand(10*time.Second, "ufw", "allow", "8322/tcp")
		sysutils.RunCommand(10*time.Second, "ufw", "allow", "8189/udp")
		sysutils.RunCommand(10*time.Second, "ufw", "allow", "8890/udp")
		sysutils.RunCommand(10*time.Second, "ufw", "allow", "5080/udp")
		sysutils.RunCommand(10*time.Second, "ufw", "allow", "8388")
		sysutils.Success("Firewall rules applied.")
	}

	return nil
}
