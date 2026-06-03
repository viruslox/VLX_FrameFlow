package telemetry

import (
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func fakeWifiExecCommand(command string, args ...string) *exec.Cmd {
	cs := []string{"-test.run=TestWifiHelperProcess", "--", command}
	cs = append(cs, args...)
	cmd := exec.Command(os.Args[0], cs...)
	cmd.Env = append(os.Environ(), "GO_WANT_HELPER_PROCESS=1")
	// Pass through the mock outputs configured in the environment
	if mockIwOut := os.Getenv("MOCK_IW_OUT"); mockIwOut != "" {
		cmd.Env = append(cmd.Env, "MOCK_IW_OUT="+mockIwOut)
	}
	if mockIwErr := os.Getenv("MOCK_IW_ERR"); mockIwErr != "" {
		cmd.Env = append(cmd.Env, "MOCK_IW_ERR="+mockIwErr)
	}
	if mockIwconfigOut := os.Getenv("MOCK_IWCONFIG_OUT"); mockIwconfigOut != "" {
		cmd.Env = append(cmd.Env, "MOCK_IWCONFIG_OUT="+mockIwconfigOut)
	}
	if mockIwconfigErr := os.Getenv("MOCK_IWCONFIG_ERR"); mockIwconfigErr != "" {
		cmd.Env = append(cmd.Env, "MOCK_IWCONFIG_ERR="+mockIwconfigErr)
	}
	return cmd
}

func TestWifiHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}

	args := os.Args
	for len(args) > 0 {
		if args[0] == "--" {
			args = args[1:]
			break
		}
		args = args[1:]
	}

	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "No command provided to helper process\n")
		os.Exit(1)
	}

	cmd := args[0]
	if cmd == "iw" {
		if os.Getenv("MOCK_IW_ERR") == "1" {
			fmt.Fprint(os.Stderr, "iw error")
			os.Exit(1)
		}
		fmt.Print(os.Getenv("MOCK_IW_OUT"))
		os.Exit(0)
	} else if cmd == "iwconfig" {
		if os.Getenv("MOCK_IWCONFIG_ERR") == "1" {
			fmt.Fprint(os.Stderr, "iwconfig error")
			os.Exit(1)
		}
		fmt.Print(os.Getenv("MOCK_IWCONFIG_OUT"))
		os.Exit(0)
	}

	os.Exit(1)
}

func TestGetFirstWifiInterface(t *testing.T) {
	// Setup temporary directory to mock /sys/class/net/
	tempDir, err := os.MkdirTemp("", "mock_sys_class_net")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Save original and restore after test
	originalSysClassNetPath := sysClassNetPath
	originalNetInterfaces := netInterfaces
	defer func() {
		sysClassNetPath = originalSysClassNetPath
		netInterfaces = originalNetInterfaces
	}()
	sysClassNetPath = tempDir

	tests := []struct {
		name     string
		mockNet  func() ([]net.Interface, error)
		setup    func(dir string)
		expected string
	}{
		{
			name: "netInterfaces returns error",
			mockNet: func() ([]net.Interface, error) {
				return nil, errors.New("mock network error")
			},
			setup:    func(dir string) {},
			expected: "wlan0", // Should return default on error
		},
		{
			name: "No interfaces exist",
			mockNet: func() ([]net.Interface, error) {
				return []net.Interface{}, nil
			},
			setup:    func(dir string) {},
			expected: "",
		},
		{
			name: "Ignores lo interface",
			mockNet: func() ([]net.Interface, error) {
				return []net.Interface{{Name: "lo"}}, nil
			},
			setup: func(dir string) {
				os.MkdirAll(filepath.Join(dir, "lo", "wireless"), 0755)
			},
			expected: "",
		},
		{
			name: "Interfaces lack wireless or phy80211",
			mockNet: func() ([]net.Interface, error) {
				return []net.Interface{{Name: "eth0"}}, nil
			},
			setup: func(dir string) {
				os.MkdirAll(filepath.Join(dir, "eth0"), 0755)
			},
			expected: "",
		},
		{
			name: "Successfully finds interface with wireless subdirectory",
			mockNet: func() ([]net.Interface, error) {
				return []net.Interface{{Name: "wlan0"}, {Name: "eth0"}}, nil
			},
			setup: func(dir string) {
				os.MkdirAll(filepath.Join(dir, "wlan0", "wireless"), 0755)
				os.MkdirAll(filepath.Join(dir, "eth0"), 0755)
			},
			expected: "wlan0",
		},
		{
			name: "Successfully finds interface with phy80211 subdirectory",
			mockNet: func() ([]net.Interface, error) {
				return []net.Interface{{Name: "wlan1"}}, nil
			},
			setup: func(dir string) {
				os.MkdirAll(filepath.Join(dir, "wlan1", "phy80211"), 0755)
			},
			expected: "wlan1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clean up the temp dir for each test case
			os.RemoveAll(tempDir)
			os.MkdirAll(tempDir, 0755)

			// Reset sysClassNetPath for each test
			sysClassNetPath = tempDir

			// Mock net.Interfaces
			netInterfaces = tt.mockNet

			tt.setup(tempDir)

			got := getFirstWifiInterface()
			if got != tt.expected {
				t.Errorf("getFirstWifiInterface() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestGetWifiMode_Caching(t *testing.T) {
	// Setup temporary directory to mock /sys/class/net/
	tempDir, err := os.MkdirTemp("", "mock_sys_class_net_wifi_caching")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	originalSysClassNetPath := sysClassNetPath
	originalNetInterfaces := netInterfaces
	sysClassNetPath = tempDir
	netInterfaces = func() ([]net.Interface, error) {
		return []net.Interface{{Name: "wlan0"}}, nil
	}
	defer func() {
		sysClassNetPath = originalSysClassNetPath
		netInterfaces = originalNetInterfaces
	}()

	originalExecCommand := execCommand
	execCommand = fakeWifiExecCommand
	defer func() { execCommand = originalExecCommand }()

	// Mock environment for iw
	os.Setenv("MOCK_IW_OUT", "Interface wlan0\n\ttype managed")
	defer os.Unsetenv("MOCK_IW_OUT")

	// Ensure interface exists
	os.MkdirAll(filepath.Join(tempDir, "wlan0", "wireless"), 0755)

	// Reset cache
	wifiModeMutex.Lock()
	wifiModeCacheTime = time.Time{}
	wifiModeCache = ""
	wifiModeMutex.Unlock()

	// 1. Initial call - should populate cache
	mode1 := GetWifiMode()
	if mode1 != "Managed" {
		t.Errorf("Initial call: GetWifiMode() = %q, want %q", mode1, "Managed")
	}

	// 2. Change mock output, but call within duration - should return cached value
	os.Setenv("MOCK_IW_OUT", "Interface wlan0\n\ttype monitor")
	mode2 := GetWifiMode()
	if mode2 != "Managed" {
		t.Errorf("Cached call: GetWifiMode() = %q, want %q", mode2, "Managed")
	}

	// 3. Temporarily set cache duration to 0 and call - should refresh
	originalDuration := WifiCacheDuration
	WifiCacheDuration = 0
	defer func() { WifiCacheDuration = originalDuration }()

	mode3 := GetWifiMode()
	if mode3 != "Monitor" {
		t.Errorf("Refreshed call: GetWifiMode() = %q, want %q", mode3, "Monitor")
	}
}

func TestGetWifiMode(t *testing.T) {
	// Setup temporary directory to mock /sys/class/net/
	tempDir, err := os.MkdirTemp("", "mock_sys_class_net_wifi_mode")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	originalSysClassNetPath := sysClassNetPath
	originalNetInterfaces := netInterfaces
	sysClassNetPath = tempDir
	netInterfaces = func() ([]net.Interface, error) {
		return []net.Interface{{Name: "wlan0"}}, nil
	}
	defer func() {
		sysClassNetPath = originalSysClassNetPath
		netInterfaces = originalNetInterfaces
	}()

	originalExecCommand := execCommand
	execCommand = fakeWifiExecCommand
	defer func() { execCommand = originalExecCommand }()

	// Clean up environment variables after tests
	defer func() {
		os.Unsetenv("MOCK_IW_OUT")
		os.Unsetenv("MOCK_IW_ERR")
		os.Unsetenv("MOCK_IWCONFIG_OUT")
		os.Unsetenv("MOCK_IWCONFIG_ERR")
	}()

	tests := []struct {
		name         string
		setupFs      func()
		mockEnv      map[string]string
		expectedMode string
	}{
		{
			name:         "No interface found",
			setupFs:      func() { os.RemoveAll(tempDir); os.MkdirAll(tempDir, 0755) },
			mockEnv:      nil,
			expectedMode: "Not found",
		},
		{
			name: "iw succeeds with type managed",
			setupFs: func() {
				os.RemoveAll(tempDir)
				os.MkdirAll(filepath.Join(tempDir, "wlan0", "wireless"), 0755)
			},
			mockEnv: map[string]string{
				"MOCK_IW_OUT": `Interface wlan0
	ifindex 3
	wdev 0x1
	addr 00:11:22:33:44:55
	type managed
	wiphy 0
	txpower 31.00 dBm`,
				"MOCK_IW_ERR":       "",
				"MOCK_IWCONFIG_OUT": "",
				"MOCK_IWCONFIG_ERR": "",
			},
			expectedMode: "Managed",
		},
		{
			name: "iw succeeds with type AP",
			setupFs: func() {
				os.RemoveAll(tempDir)
				os.MkdirAll(filepath.Join(tempDir, "wlan0", "wireless"), 0755)
			},
			mockEnv: map[string]string{
				"MOCK_IW_OUT": `Interface wlan0
	type AP
`,
			},
			expectedMode: "Master",
		},
		{
			name: "iw succeeds with other types (monitor)",
			setupFs: func() {
				os.RemoveAll(tempDir)
				os.MkdirAll(filepath.Join(tempDir, "wlan0", "wireless"), 0755)
			},
			mockEnv: map[string]string{
				"MOCK_IW_OUT": `Interface wlan0
	type monitor
`,
			},
			expectedMode: "Monitor",
		},
		{
			name: "iw succeeds with other types (ibss)",
			setupFs: func() {
				os.RemoveAll(tempDir)
				os.MkdirAll(filepath.Join(tempDir, "wlan0", "wireless"), 0755)
			},
			mockEnv: map[string]string{
				"MOCK_IW_OUT": `Interface wlan0
	type ibss
`,
			},
			expectedMode: "Ad-Hoc",
		},
		{
			name: "iw succeeds with other types (mesh point)",
			setupFs: func() {
				os.RemoveAll(tempDir)
				os.MkdirAll(filepath.Join(tempDir, "wlan0", "wireless"), 0755)
			},
			mockEnv: map[string]string{
				"MOCK_IW_OUT": `Interface wlan0
	type mesh point
`,
			},
			expectedMode: "Mesh",
		},
		{
			name: "iw succeeds with other types (wds)",
			setupFs: func() {
				os.RemoveAll(tempDir)
				os.MkdirAll(filepath.Join(tempDir, "wlan0", "wireless"), 0755)
			},
			mockEnv: map[string]string{
				"MOCK_IW_OUT": `Interface wlan0
	type wds
`,
			},
			expectedMode: "Repeater",
		},
		{
			name: "iw succeeds with unknown type",
			setupFs: func() {
				os.RemoveAll(tempDir)
				os.MkdirAll(filepath.Join(tempDir, "wlan0", "wireless"), 0755)
			},
			mockEnv: map[string]string{
				"MOCK_IW_OUT": `Interface wlan0
	type unknown_type_test
`,
			},
			expectedMode: "unknown_type_test",
		},
		{
			name: "iw fails, iwconfig succeeds",
			setupFs: func() {
				os.RemoveAll(tempDir)
				os.MkdirAll(filepath.Join(tempDir, "wlan0", "wireless"), 0755)
			},
			mockEnv: map[string]string{
				"MOCK_IW_ERR": "1", // iw fails
				"MOCK_IWCONFIG_OUT": `wlan0     IEEE 802.11  ESSID:"TestNetwork"
          Mode:Managed  Frequency:2.412 GHz  Access Point: 00:11:22:33:44:55
          Bit Rate=72.2 Mb/s   Tx-Power=31 dBm
`,
			},
			expectedMode: "Managed",
		},
		{
			name: "Both commands fail",
			setupFs: func() {
				os.RemoveAll(tempDir)
				os.MkdirAll(filepath.Join(tempDir, "wlan0", "wireless"), 0755)
			},
			mockEnv: map[string]string{
				"MOCK_IW_ERR":       "1", // iw fails
				"MOCK_IWCONFIG_ERR": "1", // iwconfig fails
			},
			expectedMode: "Not found",
		},
		{
			name: "iw succeeds but type not found, iwconfig succeeds",
			setupFs: func() {
				os.RemoveAll(tempDir)
				os.MkdirAll(filepath.Join(tempDir, "wlan0", "wireless"), 0755)
			},
			mockEnv: map[string]string{
				"MOCK_IW_OUT": `Interface wlan0
	ifindex 3
	wdev 0x1
	addr 00:11:22:33:44:55
	wiphy 0
	txpower 31.00 dBm`, // No "type " line
				"MOCK_IWCONFIG_OUT": `wlan0     IEEE 802.11  ESSID:"TestNetwork"
          Mode:Master  Frequency:2.412 GHz  Access Point: 00:11:22:33:44:55
          Bit Rate=72.2 Mb/s   Tx-Power=31 dBm
`,
			},
			expectedMode: "Master",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupFs()

			// Clear environment
			os.Unsetenv("MOCK_IW_OUT")
			os.Unsetenv("MOCK_IW_ERR")
			os.Unsetenv("MOCK_IWCONFIG_OUT")
			os.Unsetenv("MOCK_IWCONFIG_ERR")

			// Set mock outputs for this test
			for k, v := range tt.mockEnv {
				os.Setenv(k, v)
			}

			// Reset cache for testing
			wifiModeMutex.Lock()
			wifiModeCacheTime = time.Time{}
			wifiModeMutex.Unlock()

			mode := GetWifiMode()
			if mode != tt.expectedMode {
				t.Errorf("GetWifiMode() = %q, want %q", mode, tt.expectedMode)
			}
		})
	}
}
