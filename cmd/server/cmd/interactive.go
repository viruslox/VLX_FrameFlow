package cmd

import (
	"fmt"
	"github.com/viruslox/vlx_frameflow/internal/sysutils"
	"os"
)

func runInteractiveMenu() {
	for {
		fmt.Println("        VLX FrameFlow Setup (SERVER)    ")
		fmt.Println("========================================")
		fmt.Println("1) Install and configure Server components (Shadowsocks, MLVPN)")
		fmt.Println("2) Complete Clean Up / Roll back")
		fmt.Println("3) Exit")
		fmt.Print("Select: ")

		var opt string
		fmt.Scanln(&opt)

		os.Setenv("FRAMEFLOW_ROLE", "SERVER")

		switch opt {
		case "1":
			setupServerComponents()
		case "2":
			sysutils.RunRollback()
		case "3":
			os.Exit(0)
		default:
			fmt.Println("Invalid")
		}
	}
}
