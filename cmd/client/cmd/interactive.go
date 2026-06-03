package cmd

import (
	"fmt"
	"github.com/viruslox/vlx_frameflow/internal/network"
	"github.com/viruslox/vlx_frameflow/internal/sysutils"
	"os"
	"time"
)

func runInteractiveMenu() {
	for {
		fmt.Println("        VLX FrameFlow Setup (CLIENT)    ")
		fmt.Println("========================================")
		fmt.Println("1) Install OS on +64GB drives (eMMc / SSD / nvme)")
		fmt.Println("2) Configure System (Full Setup)")
		fmt.Println("3) Reconfigure System network")
		fmt.Println("4) Update network interfaces")
		fmt.Println("5) Install and configure Client components (Shadowsocks, MLVPN)")
		fmt.Println("6) Complete Clean Up / Roll back")
		fmt.Println("X) Exit")
		fmt.Print("Select: ")

		var opt string
		fmt.Scanln(&opt)

		os.Setenv("FRAMEFLOW_ROLE", "CLIENT")

		switch opt {
		case "1":
			sysutils.ListStorageDevices()
		case "2":
			runSystemSetup()
			runNetworkSetup()
			runApplicationSetup()
		case "3":
			runNetworkSetup()
		case "4":
			if err := network.CreateWifiProfiles("", "", "", ""); err != nil {
				sysutils.Error("Failed to update Wi-Fi profiles: %v", err)
			} else {
				sysutils.Success("Wi-Fi profiles updated successfully.")
			}
			if err := network.CreateNetworkProfiles("", "", "", "", ""); err != nil {
				sysutils.Error("Failed to update wired network profiles: %v", err)
			} else {
				sysutils.Success("Wired network profiles updated successfully.")
			}

			sysutils.Info("Restarting systemd-networkd...")
			if _, err := sysutils.RunCommand(10*time.Second, "sudo", "systemctl", "restart", "systemd-networkd.service"); err != nil {
				sysutils.Error("Failed to restart systemd-networkd: %v", err)
			} else {
				sysutils.Success("Network interfaces updated and applied.")
			}
		case "5":
			setupClientComponents()
		case "6":
			sysutils.RunRollback()
		case "x", "X":
			os.Exit(0)
		default:
			fmt.Println("Invalid")
		}
	}
}
