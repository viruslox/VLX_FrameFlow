package telemetry

import (
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

var (
	sysClassNetPath   = "/sys/class/net/"
	wifiModeCache     string
	wifiModeCacheTime time.Time
	wifiModeMutex     sync.Mutex
	WifiCacheDuration = 10 * time.Second
	netInterfaces     = net.Interfaces
)

// GetWifiMode returns the current mode of the first wireless interface.
// The result is cached for 10 seconds to avoid expensive fork/execs in the polling loop.
func GetWifiMode() string {
	wifiModeMutex.Lock()
	defer wifiModeMutex.Unlock()

	if time.Since(wifiModeCacheTime) < WifiCacheDuration {
		return wifiModeCache
	}

	mode := getWifiModeUncached()

	wifiModeCache = mode
	wifiModeCacheTime = time.Now()

	return mode
}

func getWifiModeUncached() string {
	iface := getFirstWifiInterface()
	if iface == "" {
		return "Not found"
	}

	// Try iw first
	cmd := execCommand("iw", "dev", iface, "info")
	out, err := cmd.Output()
	if err == nil {
		lines := strings.Split(string(out), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "type ") {
				t := strings.TrimPrefix(line, "type ")
				switch strings.ToLower(t) {
				case "managed":
					return "Managed"
				case "ap":
					return "Master"
				case "monitor":
					return "Monitor"
				case "ibss":
					return "Ad-Hoc"
				case "mesh point":
					return "Mesh"
				case "wds":
					return "Repeater"
				default:
					return t
				}
			}
		}
	}

	// Fallback to iwconfig
	cmd = execCommand("iwconfig", iface)
	out, err = cmd.Output()
	if err == nil {
		s := string(out)
		idx := strings.Index(s, "Mode:")
		if idx != -1 {
			modeStr := s[idx+5:]
			spaceIdx := strings.IndexAny(modeStr, " \n")
			if spaceIdx != -1 {
				modeStr = modeStr[:spaceIdx]
			}
			return modeStr
		}
	}

	return "Not found"
}

func getFirstWifiInterface() string {
	ifaces, err := netInterfaces()
	if err != nil {
		return "wlan0"
	}

	for _, e := range ifaces {
		iface := e.Name
		if iface == "lo" {
			continue
		}
		if _, err := os.Stat(filepath.Join(sysClassNetPath, iface, "wireless")); err == nil {
			return iface
		}
		if _, err := os.Stat(filepath.Join(sysClassNetPath, iface, "phy80211")); err == nil {
			return iface
		}
	}
	return ""
}
