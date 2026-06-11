package mediamtx

import (
	"os"
	"path/filepath"
	"github.com/viruslox/vlx_frameflow/internal/sysutils"
)

// GenerateConfig copies the appropriate MediaMTX configuration template if it doesn't exist
func GenerateConfig() error {
	loadEnv()
	vlxSuiteDir := os.Getenv("VLXsuite_DIR")
	if vlxSuiteDir == "" {
		vlxSuiteDir = "/opt/VLX_FrameFlow"
	}
	configPath := filepath.Join(vlxSuiteDir, "etc", "mediamtx.settings")

	// Skip if already exists
	if _, err := os.Stat(configPath); err == nil {
		return nil
	}

	sysutils.Info("Generating mediamtx.settings...")
	templateName := "mediamtx_client.settings.template"
	if os.Getenv("FRAMEFLOW_ROLE") == "SERVER" {
		templateName = "mediamtx_server.settings.template"
	}

	content, err := os.ReadFile(filepath.Join(vlxSuiteDir, "config", templateName))
	if err != nil {
		content, _ = os.ReadFile(filepath.Join("config", templateName)) // Fallback for tests
	}

	os.MkdirAll(filepath.Join(vlxSuiteDir, "etc"), 0755)
	return os.WriteFile(configPath, content, 0644)
}
