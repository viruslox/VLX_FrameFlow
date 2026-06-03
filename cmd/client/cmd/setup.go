package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/viruslox/vlx_frameflow/internal/network"
	"github.com/viruslox/vlx_frameflow/internal/services/mediamtx"
	"github.com/viruslox/vlx_frameflow/internal/sysutils"
)

func runSystemSetup() error {
	sysutils.Info("System conf")

	if sysutils.AskConfirmation("Enable and start SSH service?") {
		sysutils.RunCommand(10*time.Second, "systemctl", "enable", "--now", "ssh")
	}

	if sysutils.AskConfirmation("Update APT repositories?") {
		sysutils.SystemUpdateRepos()
	}

	role := os.Getenv("FRAMEFLOW_ROLE")
	if role == "CLIENT" {
		fmt.Println("Please select desired default boot mode:")
		fmt.Println("1) Multi-user (Command Line / Headless) - [Recommended for performance]")
		fmt.Println("2) Graphical (Desktop GUI) - [Requires a Desktop Environment installed]")

		var bootChoice string
		fmt.Print("Select [1]: ")
		fmt.Scanln(&bootChoice)
		if bootChoice == "" {
			bootChoice = "1"
		}

		if bootChoice == "2" {
			sysutils.Info("Setting default to Graphical Target")
			sysutils.RunCommand(10*time.Second, "systemctl", "set-default", "graphical.target")
		} else {
			sysutils.Info("Setting default to Multi-user Target")
			sysutils.RunCommand(10*time.Second, "systemctl", "set-default", "multi-user.target")
		}

		fmt.Println("It is suggested to remove GUI packages (Gnome, XFCE, KDE, etc) to optimize performance.")
		fmt.Println("WARNING: If you plan to use 'Graphical Target' (GUI), answer NO (N) here.")

		var removeGuiChoice string
		fmt.Print("Do you want to remove these packages? (y/N) ")
		fmt.Scanln(&removeGuiChoice)

		if removeGuiChoice == "y" || removeGuiChoice == "Y" {
			sysutils.RemoveBloatware([]string{})
		}

		sysutils.RestorePackages()
	}

	if sysutils.AskConfirmation("Reinstall systemd?") {
		sysutils.RunCommand(10*time.Minute, "apt", "-y", "install", "--reinstall", "systemd")
	}

	if sysutils.AskConfirmation("Install dependencies?") {
		sysutils.InstallDependencies(nil, nil, nil)
	}

	if sysutils.AskConfirmation("Configure kernel sysctl (dmesg_restrict)?") {
		os.WriteFile("/etc/sysctl.d/99-disable-dmesg-restrict.conf", []byte("kernel.dmesg_restrict=0\n"), 0644)
		sysutils.RunCommand(10*time.Second, "sysctl", "--system")
	}

	if sysutils.AskConfirmation("Set NTP and Timezone?") {
		sysutils.RunCommand(10*time.Second, "timedatectl", "set-ntp", "true")
		sysutils.RunCommand(10*time.Second, "systemctl", "restart", "systemd-timesyncd")
		sysutils.RunCommand(10*time.Second, "timedatectl", "set-timezone", "Europe/Rome")
	}
	return nil
}


func runNetworkSetup() error {
	sysutils.Info("Network conf")
	if sysutils.AskConfirmation("Configure network features and tools?") {
		network.ConfigureNetworkFeatures()
	}

	if sysutils.AskConfirmation("Configure firewall (UFW)?") {
		network.ConfigureFirewall(os.Getenv("FRAMEFLOW_ROLE"))
	}

	if os.Getenv("FRAMEFLOW_ROLE") == "CLIENT" {
		network.CreateAPProfile("", "")
		network.CreateManagedProfile("", "", 0)
	}

	if sysutils.AskConfirmation("Enable network settings and reload systemd?") {
		network.EnableNetworkSettings()
	}

	if sysutils.AskConfirmation("Check Kernel MPTCP Support?") {
		network.CheckMptcpKernel()
	}

	if sysutils.AskConfirmation("Setup MPTCP Proxy (Shadowsocks + V2Ray)?") {
		network.SetupMptcpProxy()
	}

	if sysutils.AskConfirmation("Setup MLVPN Bonding (UDP)?") {
		network.SetupMlvpnBonding()
	}

	return nil
}

func runApplicationSetup() error {
	sysutils.Info("Applications conf")

	if sysutils.AskConfirmation("Update suite code from GitHub?") {
		sysutils.UpdateSuiteCode()
	}

	if sysutils.AskConfirmation("Install MediaMTX?") {
		mediamtx.Install()
	}

	if sysutils.AskConfirmation("Setup maintenance cron job?") {
		sysutils.SetupMaintenanceCron()
	}
	return nil
}

func setupClientComponents() error {
	sysutils.Info("Cleaning out existing Client components (Shadowsocks, MLVPN)...")

	sysutils.RunCommand(10*time.Second, "systemctl", "disable", "--now", "shadowsocks-libev.service")
	sysutils.RunCommand(10*time.Second, "systemctl", "disable", "--now", "frameflow-mptcp-proxy.service")
	sysutils.RunCommand(10*time.Second, "systemctl", "disable", "--now", "frameflow-mlvpn.service")

	sysutils.InstallShadowsocks()
	sysutils.InstallMlvpn()
	network.SetupMptcpProxy()
	network.SetupMlvpnBonding()

	sysutils.Success("Client components successfully re-installed and configured.")
	return nil
}
