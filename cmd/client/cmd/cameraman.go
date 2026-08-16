package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/viruslox/vlx_frameflow/internal/cameraman"
)

var cameramanCmd = &cobra.Command{
	Use:   "cameraman <VxAy> [NN] start | cameraman <NN> stop|status | status | devlist",
	Short: "Manages combined hardware AV encoding pipelines",
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if os.Geteuid() == 0 {
			fmt.Println("Error: cameraman command must not be run as root. Please run as the dedicated user or via the vlx_frameflow alias without sudo.")
			os.Exit(1)
		}

		// Single-word forms: aggregate status / device listing.
		if len(args) == 1 {
			switch args[0] {
			case "status":
				out, err := cameraman.StatusAllStreams()
				if err != nil {
					fmt.Println(err)
				} else {
					fmt.Println(out)
				}
			case "devlist":
				out, err := cameraman.ListDevices()
				if err != nil {
					fmt.Println(err)
				} else {
					fmt.Println(out)
				}
			default:
				fmt.Println("Insufficient arguments. Expected: cameraman <VxAy> [NN] start | cameraman <NN> stop|status")
				os.Exit(1)
			}
			return
		}

		// The action is always the final argument.
		action := args[len(args)-1]

		switch action {
		case "start":
			// cameraman <VxAy> start          (auto-assign NN)
			// cameraman <VxAy> <NN> start
			cameraID := args[0]
			slot := ""
			if len(args) == 3 {
				slot = args[1]
			} else if len(args) != 2 {
				fmt.Println("Usage: cameraman <VxAy> [NN] start")
				os.Exit(1)
			}
			if _, err := cameraman.ParseCameraID(cameraID); err != nil {
				fmt.Printf("Error parsing camera ID: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("Starting stream %s...\n", cameraID)
			if err := cameraman.StartStream(cameraID, slot); err != nil {
				fmt.Printf("Error starting stream: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("Stream %s started successfully.\n", cameraID)

		case "stop":
			// cameraman <NN> stop
			if len(args) != 2 {
				fmt.Println("Usage: cameraman <NN> stop")
				os.Exit(1)
			}
			slot := args[0]
			fmt.Printf("Stopping slot %s...\n", slot)
			if err := cameraman.StopStream(slot); err != nil {
				fmt.Printf("Error stopping stream: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("Slot %s stopped successfully.\n", slot)

		case "status":
			// cameraman <NN> status
			if len(args) != 2 {
				fmt.Println("Usage: cameraman <NN> status")
				os.Exit(1)
			}
			out, err := cameraman.StatusStream(args[0])
			if err != nil {
				fmt.Printf("Error checking status for slot %s: %v\n", args[0], err)
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
