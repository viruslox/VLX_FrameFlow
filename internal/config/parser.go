package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	LogLevel          string   `yaml:"logLevel"`
	LogDestinations   []string `yaml:"logDestinations"`
	SysLogPrefix      string   `yaml:"sysLogPrefix"`
	ReadTimeout       string   `yaml:"readTimeout"`
	WriteTimeout      string   `yaml:"writeTimeout"`
	WriteQueueSize    int      `yaml:"writeQueueSize"`
	UDPMaxPayloadSize int      `yaml:"udpMaxPayloadSize"`

	RunOnConnect        string `yaml:"runOnConnect"`
	RunOnConnectRestart bool   `yaml:"runOnConnectRestart"`
	RunOnDisconnect     string `yaml:"runOnDisconnect"`

	AuthMethod        string `yaml:"authMethod"`
	AuthInternalUsers []struct {
		User        string   `yaml:"user"`
		Ips         []string `yaml:"ips"`
		Permissions []struct {
			Action string `yaml:"action"`
			Path   string `yaml:"path"`
		} `yaml:"permissions"`
	} `yaml:"authInternalUsers"`

	Api     bool `yaml:"api"`
	Metrics bool `yaml:"metrics"`
	Pprof   bool `yaml:"pprof"`

	Playback       bool   `yaml:"playback"`
	Rtsp           bool   `yaml:"rtsp"`
	Rtmp           bool   `yaml:"rtmp"`
	RtmpAddress    string `yaml:"rtmpAddress"`
	RtmpEncryption string `yaml:"rtmpEncryption"`
	RtmpsAddress   string `yaml:"rtmpsAddress"`
	RtmpServerKey  string `yaml:"rtmpServerKey"`
	RtmpServerCert string `yaml:"rtmpServerCert"`

	Hls        bool   `yaml:"hls"`
	Webrtc     bool   `yaml:"webrtc"`
	Srt        bool   `yaml:"srt"`
	SrtAddress string `yaml:"srtAddress"`

	PathDefaults struct {
		Source               string `yaml:"source"`
		SourceFingerprint    string `yaml:"sourceFingerprint"`
		SourceOnDemand       bool   `yaml:"sourceOnDemand"`
		MaxReaders           int    `yaml:"maxReaders"`
		SrtReadPassphrase    string `yaml:"srtReadPassphrase"`
		Fallback             string `yaml:"fallback"`
		UseAbsoluteTimestamp bool   `yaml:"useAbsoluteTimestamp"`
		OverridePublisher    bool   `yaml:"overridePublisher"`
		SrtPublishPassphrase string `yaml:"srtPublishPassphrase"`

		RunOnInit                  string `yaml:"runOnInit"`
		RunOnInitRestart           bool   `yaml:"runOnInitRestart"`
		RunOnDemand                string `yaml:"runOnDemand"`
		RunOnDemandRestart         bool   `yaml:"runOnDemandRestart"`
		RunOnDemandStartTimeout    string `yaml:"runOnDemandStartTimeout"`
		RunOnDemandCloseAfter      string `yaml:"runOnDemandCloseAfter"`
		RunOnUnDemand              string `yaml:"runOnUnDemand"`
		RunOnReady                 string `yaml:"runOnReady"`
		RunOnReadyRestart          bool   `yaml:"runOnReadyRestart"`
		RunOnNotReady              string `yaml:"runOnNotReady"`
		RunOnRead                  string `yaml:"runOnRead"`
		RunOnReadRestart           bool   `yaml:"runOnReadRestart"`
		RunOnUnread                string `yaml:"runOnUnread"`
		RunOnRecordSegmentCreate   string `yaml:"runOnRecordSegmentCreate"`
		RunOnRecordSegmentComplete string `yaml:"runOnRecordSegmentComplete"`
	} `yaml:"pathDefaults"`

	Paths map[string]struct {
		RunOnReadyRestart bool   `yaml:"runOnReadyRestart"`
		RunOnReady        string `yaml:"runOnReady,omitempty"`
	} `yaml:"paths"`
}

func Parse(filePath string) (*Config, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("error reading config file: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("error parsing yaml: %w", err)
	}

	return &config, nil
}
