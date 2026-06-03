package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/viruslox/vlx_frameflow/internal/cameraman"
)

var cameramanCmd = &cobra.Command{
	Use:   "cameraman <VxAy> <start|stop|status> | devlist",
	Short: "Manages video encoding pipelines",
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if os.Geteuid() == 0 {
			fmt.Println("Error: cameraman command must not be run as root. Please run as the dedicated user or via the vlx_frameflow alias without sudo.")
			os.Exit(1)
		}

		if len(args) == 0 {
			cmd.Help()
			return
		}

		action := ""
		cameraID := args[0]

		// If just "status", show all
		if len(args) == 1 && cameraID == "status" {
			out, err := cameraman.StatusAllStreams()
			if err != nil {
				fmt.Println(err)
			} else {
				fmt.Println(out)
			}
			return
		}

		// If just "devlist", show devices
		if len(args) == 1 && cameraID == "devlist" {
			out, err := cameraman.ListDevices()
			if err != nil {
				fmt.Println(err)
			} else {
				fmt.Println(out)
			}
			return
		}

		if len(args) < 2 {
			fmt.Println("Insufficient arguments. Expected: cameraman <VxAy> <start|stop|status>")
			os.Exit(1)
		}

		action = args[1]

		vidID, audID, err := cameraman.ParseCameraID(cameraID)
		if err != nil {
			fmt.Printf("Error parsing camera ID: %v\n", err)
			os.Exit(1)
		}

		switch action {
		case "start":
				fmt.Printf("Starting stream %s...\n", cameraID)
				err = cameraman.StartStream(cameraID, vidID, audID)
				if err != nil {
					fmt.Printf("Error starting stream: %v\n", err)
					os.Exit(1)
				}
				fmt.Printf("Stream %s started successfully.\n", cameraID)
		case "stop":
			fmt.Printf("Stopping stream %s...\n", cameraID)
			err := cameraman.StopStream(cameraID)
			if err != nil {
				fmt.Printf("Error stopping stream: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("Stream %s stopped successfully.\n", cameraID)

		case "status":
			out, err := cameraman.StatusStream(cameraID)
			if err != nil {
				fmt.Printf("Error checking status for stream %s: %v\n", cameraID, err)
			} else {
				fmt.Println(out)
			}

		default:
			fmt.Printf("Unknown action: %s. Use start, stop, or status.\n", action)
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.AddCommand(cameramanCmd)
}
