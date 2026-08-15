package config

import (
	"bufio"
	"log"
	"os"
	"path/filepath"
	"strings"
)

type FrontendConfig struct {
	AuthUser    string
	AuthPass    string
	BackendUser string
	BackendPass string
	BindAddress string
	BindPort    string
	BackendAddr string
	BackendPort string
	ClientCrt   string
	ClientKey   string
	// UseRelay selects how the served UI reaches the FrameFlow API. false =
	// this frontend talks to a local Client (SBC) API directly (api/<module>).
	// true = this frontend fronts a Server (VM), so all module calls must be
	// routed through the relay (api/v1/relay/<module>) to reach the remote SBC.
	UseRelay bool
}

func trimQuotes(s string) string {
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'') {
			return s[1 : len(s)-1]
		}
	}
	return s
}

func LoadConfig(customPath string) *FrontendConfig {
	// Default configuration
	config := &FrontendConfig{
		BindAddress: "",
		BindPort:    "8080",
		BackendAddr: "127.0.0.1",
		BackendPort: "9090",
	}

	// Try reading from file
	var filepaths []string
	if customPath != "" {
		filepaths = []string{customPath}
	} else {
		vlxSuiteDir := os.Getenv("VLXsuite_DIR")
		if vlxSuiteDir == "" {
			vlxSuiteDir = "/opt/VLX_FrameFlow"
		}
		filepaths = []string{filepath.Join(vlxSuiteDir, "etc/frontend.settings"), "bin/frontend.settings", "frontend.settings"}
	}
	var file *os.File
	var err error

	for _, path := range filepaths {
		file, err = os.Open(path)
		if err == nil {
			break
		}
	}

	if customPath != "" && err != nil {
		log.Fatalf("ERROR: Failed to open custom configuration file at %s: %v", customPath, err)
	}

	if err == nil {
		defer file.Close()
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if len(line) == 0 || strings.HasPrefix(line, "#") {
				continue
			}

			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				key := strings.TrimSpace(parts[0])
				key = strings.TrimPrefix(key, "export ")
				key = strings.TrimSpace(key)

				value := strings.TrimSpace(parts[1])
				value = trimQuotes(value)

				if key == "bind address" || key == "bind_address" {
					config.BindAddress = value
				} else if key == "bind port" || key == "bind_port" {
					config.BindPort = value
				} else if key == "backend address" || key == "backend_address" {
					config.BackendAddr = value
				} else if key == "backend port" || key == "backend_port" {
					config.BackendPort = value
				} else if key == "use_relay" {
					v := strings.ToLower(value)
					config.UseRelay = (v == "true" || v == "yes" || v == "1")
				} else if key == "FF_GUI_USER" {
					config.AuthUser = value
				} else if key == "FF_GUI_PASS" {
					config.AuthPass = value
				} else if key == "bkend_user" {
					config.BackendUser = value
				} else if key == "bkend_pass" {
					config.BackendPass = value
				} else if key == "client_crt" {
					config.ClientCrt = value
				} else if key == "client_key" {
					config.ClientKey = value
				}
			}
		}
	}

	// Fallback to Env variables if file not present or partially filled
	if config.AuthUser == "" {
		config.AuthUser = os.Getenv("FF_GUI_USER")
	}
	if config.AuthPass == "" {
		config.AuthPass = os.Getenv("FF_GUI_PASS")
	}
	if config.BackendUser == "" {
		config.BackendUser = os.Getenv("bkend_user")
	}
	if config.BackendPass == "" {
		config.BackendPass = os.Getenv("bkend_pass")
	}
	if config.BindAddress == "" {
		config.BindAddress = os.Getenv("bind_address")
	}
	if envPort := os.Getenv("bind_port"); envPort != "" {
		config.BindPort = envPort
	}
	if envAddr := os.Getenv("backend_address"); envAddr != "" {
		config.BackendAddr = envAddr
	}
	if envBkendPort := os.Getenv("backend_port"); envBkendPort != "" {
		config.BackendPort = envBkendPort
	}
	if envClientCrt := os.Getenv("client_crt"); envClientCrt != "" {
		config.ClientCrt = envClientCrt
	}
	if envClientKey := os.Getenv("client_key"); envClientKey != "" {
		config.ClientKey = envClientKey
	}
	if envUseRelay := os.Getenv("use_relay"); envUseRelay != "" {
		v := strings.ToLower(envUseRelay)
		config.UseRelay = (v == "true" || v == "yes" || v == "1")
	}

	if config.AuthUser == "" || config.AuthPass == "" {
		log.Fatal("ERROR: Insecure configuration. Environment variables FF_GUI_USER and FF_GUI_PASS must be set for authentication, or frontend.settings must be provided with valid credentials.")
	}

	return config
}

type BackendConfig struct {
	BindAddress    string
	BindPort       string
	Accounts       map[string]string
	AllowedOrigins []string
	ServerCrt      string
	ServerKey      string
	// RelayClientHost/Port are the address of the *remote* Client API on the
	// far side of the MLVPN tunnel, used by the Server's relay proxy. These are
	// distinct from BindAddress/BindPort, which are where this Server's own
	// relay listens locally. Defaults target the standard MLVPN client.
	RelayClientHost string
	RelayClientPort string
}

func LoadBackendConfig(customPath string) *BackendConfig {
		config := &BackendConfig{
		BindAddress:     "",
		BindPort:        "9090",
		Accounts:        make(map[string]string),
		AllowedOrigins:  []string{},
		RelayClientHost: "10.1.10.2",
		RelayClientPort: "9090",
	}

	var filepaths []string
	if customPath != "" {
		filepaths = []string{customPath}
	} else {
		vlxSuiteDir := os.Getenv("VLXsuite_DIR")
		if vlxSuiteDir == "" {
			vlxSuiteDir = "/opt/VLX_FrameFlow"
		}
		filepaths = []string{filepath.Join(vlxSuiteDir, "etc/frameflow.settings"), "bin/frameflow.settings", "frameflow.settings"}
	}
	var file *os.File
	var err error

	for _, path := range filepaths {
		file, err = os.Open(path)
		if err == nil {
			break
		}
	}

	if customPath != "" && err != nil {
		log.Fatalf("ERROR: Failed to open custom configuration file at %s: %v", customPath, err)
	}

	users := make(map[string]string)
	passes := make(map[string]string)

	if err == nil {
		defer file.Close()
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if len(line) == 0 || strings.HasPrefix(line, "#") {
				continue
			}

			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				key := strings.TrimSpace(parts[0])
				key = strings.TrimPrefix(key, "export ")
				key = strings.TrimSpace(key)

				value := strings.TrimSpace(parts[1])
				value = trimQuotes(value)

				if key == "bind address" || key == "bind_address" {
					config.BindAddress = value
				} else if key == "bind port" || key == "bind_port" {
					config.BindPort = value
				} else if key == "allowed_origins" {
					origins := strings.Split(value, ",")
					for _, o := range origins {
						trimmed := strings.TrimSpace(o)
						trimmed = trimQuotes(trimmed)
						if trimmed != "" {
							config.AllowedOrigins = append(config.AllowedOrigins, trimmed)
						}
					}
				} else if strings.HasPrefix(key, "bkend_user") {
					id := strings.TrimPrefix(key, "bkend_user")
					users[id] = value
				} else if strings.HasPrefix(key, "bkend_pass") {
					id := strings.TrimPrefix(key, "bkend_pass")
					passes[id] = value
				} else if key == "server_crt" {
					config.ServerCrt = value
				} else if key == "server_key" {
					config.ServerKey = value
				} else if key == "relay_client_host" {
					config.RelayClientHost = value
				} else if key == "relay_client_port" {
					config.RelayClientPort = value
				}
			}
		}
	}

	for id, user := range users {
		if pass, ok := passes[id]; ok {
			config.Accounts[user] = pass
		}
	}

	// Fallback to Env variables if file not present or partially filled
	if config.BindAddress == "" {
		config.BindAddress = os.Getenv("bind_address")
	}
	if envPort := os.Getenv("bind_port"); envPort != "" {
		config.BindPort = envPort
	}
	if envOrigins := os.Getenv("allowed_origins"); envOrigins != "" {
		origins := strings.Split(envOrigins, ",")
		for _, o := range origins {
			trimmed := strings.TrimSpace(o)
			trimmed = trimQuotes(trimmed)
			if trimmed != "" {
				config.AllowedOrigins = append(config.AllowedOrigins, trimmed)
			}
		}
	}

	if envServerCrt := os.Getenv("server_crt"); envServerCrt != "" {
		config.ServerCrt = envServerCrt
	}
	if envServerKey := os.Getenv("server_key"); envServerKey != "" {
		config.ServerKey = envServerKey
	}
	if envRelayHost := os.Getenv("relay_client_host"); envRelayHost != "" {
		config.RelayClientHost = envRelayHost
	}
	if envRelayPort := os.Getenv("relay_client_port"); envRelayPort != "" {
		config.RelayClientPort = envRelayPort
	}

	// The SERVER role runs only the localhost relay, which sits behind the
	// frontend's own auth and needs no backend accounts of its own. Requiring
	// bkend_* there would wrongly impose a Client-only concept on the Server,
	// so the accounts check applies to non-SERVER roles only.
	if os.Getenv("FRAMEFLOW_ROLE") != "SERVER" && len(config.Accounts) == 0 {
		log.Fatal("ERROR: Insecure configuration. At least one bkend_userX/bkend_passX pair must be provided in frameflow.settings.")
	}

	return config
}
