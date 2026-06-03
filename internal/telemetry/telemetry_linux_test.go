//go:build linux

package telemetry

import (
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetNetworkInterfaces_ErrorPath(t *testing.T) {
	// Save the original readFile function and restore it after the test
	originalReadFile := readFile
	t.Cleanup(func() {
		readFile = originalReadFile
	})

	// Mock readFile to return an error when accessing /proc/net/dev
	readFile = func(name string) ([]byte, error) {
		if name == "/proc/net/dev" {
			return nil, errors.New("mock error reading /proc/net/dev")
		}
		// Return empty data for other files to avoid crashes
		return []byte(""), nil
	}

	stats := GetNetworkInterfaces()

	// Assert the mock data is populated on error as per line 77 logic in telemetry_linux.go
	assert.Contains(t, stats, "eth0", "Expected eth0 mock data")
	assert.Equal(t, uint64(1000), stats["eth0"].RxBytes)
	assert.Equal(t, uint64(2000), stats["eth0"].TxBytes)

	assert.Contains(t, stats, "wlan0", "Expected wlan0 mock data")
	assert.Equal(t, uint64(500), stats["wlan0"].RxBytes)
	assert.Equal(t, uint64(100), stats["wlan0"].TxBytes)
}

func TestGetNetworkInterfaces_ParsePath(t *testing.T) {
	// Save the original readFile function and restore it after the test
	originalReadFile := readFile
	t.Cleanup(func() {
		readFile = originalReadFile
	})

	mockProcNetDev := `Inter-|   Receive                                                |  Transmit
 face |bytes    packets errs drop fifo frame compressed multicast|bytes    packets errs drop fifo colls carrier compressed
    lo:    1234      10    0    0    0     0          0         0     5678      10    0    0    0     0       0          0
  eth0: 1000000    1000    0    0    0     0          0         0  2000000    2000    0    0    0     0       0          0
 wlan0:  500000     500    0    0    0     0          0         0   100000     100    0    0    0     0       0          0
`

	mockProcNetRoute := `Iface	Destination	Gateway 	Flags	RefCnt	Use	Metric	Mask		MTU	Window	IRTT
eth0	00000000	0100A8C0	0003	0	0	100	00000000	0	0	0
wlan0	00000000	0100A8C0	0003	0	0	600	00000000	0	0	0
eth0	0000A8C0	00000000	0001	0	0	100	00FFFFFF	0	0	0
`

	mockProcNetIpv6Route := `00000000000000000000000000000000 00 00000000000000000000000000000000 00 fe80000000000000021122fffe334455 00000100 00000000 00000000 00000001 eth0
fe800000000000000000000000000000 40 00000000000000000000000000000000 00 00000000000000000000000000000000 00000100 00000000 00000000 00000001 eth0
`

	// Mock readFile to return mock content based on the file name
	readFile = func(name string) ([]byte, error) {
		switch name {
		case "/proc/net/dev":
			return []byte(mockProcNetDev), nil
		case "/proc/net/route":
			return []byte(mockProcNetRoute), nil
		case "/proc/net/ipv6_route":
			return []byte(mockProcNetIpv6Route), nil
		case "/sys/class/net/eth0/operstate":
			return []byte("up\n"), nil
		case "/sys/class/net/wlan0/operstate":
			return []byte("down\n"), nil
		case "/sys/class/net/lo/operstate":
			return []byte("unknown\n"), nil
		default:
			if strings.HasPrefix(name, "/sys/class/net/") && strings.HasSuffix(name, "/operstate") {
				return []byte("unknown\n"), nil
			}
			return []byte(""), os.ErrNotExist
		}
	}

	stats := GetNetworkInterfaces()

	// Assert parsing results for eth0
	assert.Contains(t, stats, "eth0")
	assert.Equal(t, uint64(1000000), stats["eth0"].RxBytes)
	assert.Equal(t, uint64(2000000), stats["eth0"].TxBytes)
	assert.Equal(t, "UP", stats["eth0"].OperState)
	assert.Equal(t, "192.168.0.1", stats["eth0"].IPv4GW)
	assert.Equal(t, "fe80::211:22ff:fe33:4455", stats["eth0"].IPv6GW) // net.IP parsing

	// Assert parsing results for wlan0
	assert.Contains(t, stats, "wlan0")
	assert.Equal(t, uint64(500000), stats["wlan0"].RxBytes)
	assert.Equal(t, uint64(100000), stats["wlan0"].TxBytes)
	assert.Equal(t, "DOWN", stats["wlan0"].OperState)
	assert.Equal(t, "192.168.0.1", stats["wlan0"].IPv4GW)
	assert.Equal(t, "", stats["wlan0"].IPv6GW)

	// Assert parsing results for lo
	assert.Contains(t, stats, "lo")
	assert.Equal(t, uint64(1234), stats["lo"].RxBytes)
	assert.Equal(t, uint64(5678), stats["lo"].TxBytes)
	assert.Equal(t, "UNKNOWN", stats["lo"].OperState)
	assert.Equal(t, "", stats["lo"].IPv4GW)
	assert.Equal(t, "", stats["lo"].IPv6GW)
}

func TestGetNetworkInterfacesLinux(t *testing.T) {
	// Original basic test that runs without mocks against real system files (if available)
	// It mainly ensures the function doesn't panic.
	GetNetworkInterfaces()
}

func TestGetSystemUsage_ErrorPath(t *testing.T) {
	// Save the original readFile function and restore it after the test
	originalReadFile := readFile
	t.Cleanup(func() {
		readFile = originalReadFile
	})

	// Mock readFile to return an error when accessing /proc/stat and /proc/meminfo
	readFile = func(name string) ([]byte, error) {
		if name == "/proc/stat" {
			return nil, errors.New("mock error reading /proc/stat")
		}
		if name == "/proc/meminfo" {
			return nil, errors.New("mock error reading /proc/meminfo")
		}
		return []byte(""), nil
	}

	usage := GetSystemUsage()

	// Assert the mock data is populated on error as per telemetry_linux.go logic
	assert.Equal(t, 25.5, usage.CPU, "Expected mock CPU usage data")
	assert.Equal(t, 40.2, usage.Ram, "Expected mock Ram usage data")
	assert.Equal(t, 10.5, usage.Swap, "Expected mock Swap usage data")
}

func TestParseIPv4Hex(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Typical IPv4 - 192.168.0.1",
			input:    "0100A8C0",
			expected: "192.168.0.1",
		},
		{
			name:     "Localhost - 127.0.0.1",
			input:    "0100007F",
			expected: "127.0.0.1",
		},
		{
			name:     "All zeros - 0.0.0.0",
			input:    "00000000",
			expected: "0.0.0.0",
		},
		{
			name:     "All ones - 255.255.255.255",
			input:    "FFFFFFFF",
			expected: "255.255.255.255",
		},
		{
			name:     "Invalid hex string",
			input:    "invalidx",
			expected: "",
		},
		{
			name:     "Non-hex string",
			input:    "non-hex string",
			expected: "",
		},
		{
			name:     "Invalid length - too short",
			input:    "0100A8",
			expected: "",
		},
		{
			name:     "Invalid length - too long",
			input:    "0100A8C0FF",
			expected: "",
		},
		{
			name:     "Empty string",
			input:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseIPv4Hex(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParseIPv6Hex(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Typical IPv6 - fe80::211:22ff:fe33:4455",
			input:    "fe80000000000000021122fffe334455",
			expected: "fe80::211:22ff:fe33:4455",
		},
		{
			name:     "All zeros - ::",
			input:    "00000000000000000000000000000000",
			expected: "::",
		},
		{
			name:     "Invalid hex string",
			input:    "invalidhexstring0000000000000000",
			expected: "",
		},
		{
			name:     "Invalid length - too short",
			input:    "fe80",
			expected: "",
		},
		{
			name:     "Invalid length - too long",
			input:    "fe80000000000000021122fffe33445500",
			expected: "",
		},
		{
			name:     "Empty string",
			input:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseIPv6Hex(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
