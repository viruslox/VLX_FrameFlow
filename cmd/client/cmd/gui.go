package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/viruslox/vlx_frameflow/internal/security"
	"github.com/viruslox/vlx_frameflow/internal/sysutils"
)

var guiCmd = &cobra.Command{
	Use:   "gui",
	Short: "GUI related commands",
}

var guiAddClientCmd = &cobra.Command{
	Use:   "add-client [clientName]",
	Short: "Generate and export signed Client Certificates",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		clientName := args[0]
		sysutils.Info("Generating client certificates for %s...", clientName)

		// Check if running in production or development
		secDir := "/opt/VLX_FrameFlow/certs/"
		if _, err := os.Stat(secDir); os.IsNotExist(err) {
			secDir = "." // Fallback to current dir for dev/testing
		}

		caCertPath := filepath.Join(secDir, "ca.crt")
		caKeyPath := filepath.Join(secDir, "ca.key")

		caCertPEM, err := os.ReadFile(caCertPath)
		if err != nil {
			sysutils.Error("Failed to read CA certificate: %v", err)
			return
		}

		caKeyPEM, err := os.ReadFile(caKeyPath)
		if err != nil {
			sysutils.Error("Failed to read CA private key: %v", err)
			return
		}

		clientCertPEM, clientKeyPEM, err := security.GenerateClientCert(caCertPEM, caKeyPEM, clientName)
		if err != nil {
			sysutils.Error("Failed to generate client certificate: %v", err)
			return
		}

		certFile := fmt.Sprintf("%s.crt", clientName)
		keyFile := fmt.Sprintf("%s.key", clientName)

		err = os.WriteFile(certFile, clientCertPEM, 0644)
		if err != nil {
			sysutils.Error("Failed to write client certificate: %v", err)
			return
		}

		err = os.WriteFile(keyFile, clientKeyPEM, 0600)
		if err != nil {
			sysutils.Error("Failed to write client private key: %v", err)
			return
		}

		sysutils.Success("Client certificates generated successfully: %s, %s", certFile, keyFile)
	},
}

func init() {
	rootCmd.AddCommand(guiCmd)
	guiCmd.AddCommand(guiAddClientCmd)
}
