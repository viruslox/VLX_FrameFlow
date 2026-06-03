package network

import (
	"fmt"
	"time"

	"github.com/viruslox/vlx_frameflow/internal/sysutils"
)

func ClientStart() error {
	sysutils.Info("Starting client components...")
	if out, err := sysutils.RunCommand(10*time.Second, "systemctl", "--user", "start", "frameflow-mptcp-proxy.service"); err != nil {
		return fmt.Errorf("failed to start frameflow-mptcp-proxy: %w, output: %s", err, out)
	}
	if out, err := sysutils.RunCommand(10*time.Second, "systemctl", "--user", "start", "frameflow-mlvpn.service"); err != nil {
		return fmt.Errorf("failed to start frameflow-mlvpn: %w, output: %s", err, out)
	}
	sysutils.Success("Client components started.")
	return nil
}

func ClientStop() error {
	sysutils.Info("Stopping client components...")
	if out, err := sysutils.RunCommand(10*time.Second, "systemctl", "--user", "stop", "frameflow-mptcp-proxy.service"); err != nil {
		return fmt.Errorf("failed to stop frameflow-mptcp-proxy: %w, output: %s", err, out)
	}
	if out, err := sysutils.RunCommand(10*time.Second, "systemctl", "--user", "stop", "frameflow-mlvpn.service"); err != nil {
		return fmt.Errorf("failed to stop frameflow-mlvpn: %w, output: %s", err, out)
	}
	sysutils.Info("Killing possible orphan processes...")
	sysutils.RunCommand(10*time.Second, "pkill", "-f", "ss-redir")
	sysutils.RunCommand(10*time.Second, "pkill", "-f", "mlvpn")
	sysutils.RunCommand(10*time.Second, "pkill", "-f", "v2ray-plugin")
	sysutils.Success("Client components stopped and orphans killed.")
	return nil
}

func ClientStatus() (string, error) {
	sysutils.Info("Checking client components status...")
	out1, _ := sysutils.RunCommand(10*time.Second, "systemctl", "--user", "status", "frameflow-mptcp-proxy.service", "--no-pager")
	out3, _ := sysutils.RunCommand(10*time.Second, "systemctl", "--user", "status", "frameflow-mlvpn.service", "--no-pager")
	return out1 + "\n" + out3, nil
}
