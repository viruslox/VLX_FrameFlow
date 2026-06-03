package telemetry

type NetworkInterfaceStats struct {
	RxBytes   uint64 `json:"rx_bytes"`
	TxBytes   uint64 `json:"tx_bytes"`
	OperState string `json:"operstate"`
	IPv4      string `json:"ipv4"`
	IPv4GW    string `json:"ipv4_gw"`
	IPv6      string `json:"ipv6"`
	IPv6GW    string `json:"ipv6_gw"`
}

type SystemUsage struct {
	CPU  float64 `json:"cpu"`
	Ram  float64 `json:"ram"`
	Swap float64 `json:"swap"`
}

type GPSData string

type FFmpegLogs string
