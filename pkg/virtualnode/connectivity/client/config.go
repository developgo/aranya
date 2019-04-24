package client

import (
	"time"
)

type NodeConfig struct {
	Timers struct {
		NodeStatusSyncInterval time.Duration `json:"status_sync_interval" yaml:"status_sync_interval"`
	} `json:"timers" yaml:"timers"`
}

type PodConfig struct {
	MaxPodCount int `json:"max_pod_count" yaml:"max_pod_count"`
	Timers      struct {
		PodStatusSyncInterval time.Duration `json:"status_sync_interval" yaml:"status_sync_interval"`
	} `json:"timers" yaml:"timers"`
}

type Config struct {
	Log struct {
		Level int    `json:"level" yaml:"level"`
		Dir   string `json:"dir" yaml:"dir"`
	} `json:"log" yaml:"log"`

	Features struct {
		AllowHostExec bool `json:"allow_host_exec" yaml:"allow_host_exec"`
	} `json:"features" yaml:"features"`

	Node NodeConfig `json:"node" yaml:"node"`
	Pod  PodConfig  `json:"pod" yaml:"pod"`
}
