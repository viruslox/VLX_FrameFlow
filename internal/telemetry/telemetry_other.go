//go:build !linux

package telemetry

func GetNetworkInterfaces() map[string]NetworkInterfaceStats {
	return make(map[string]NetworkInterfaceStats)
}

func GetSystemUsage() SystemUsage {
	return SystemUsage{}
}

func GetGPSData() string {
	return ""
}

func GetFFmpegLogs() string {
	return ""
}
