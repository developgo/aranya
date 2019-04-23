package client

import (
	"time"
)

type Config struct {
	Log struct {
		Level int    `json:"level" yaml:"level"`
		Dir   string `json:"dir" yaml:"dir"`
	} `json:"log" yaml:"log"`

	Features struct {
		AllowHostExec bool `json:"allow_host_exec" yaml:"allow_host_exec"`
	} `json:"features" yaml:"features"`

	Timers struct {
		NodeStatusSyncInterval time.Duration `json:"node_status_sync_interval" yaml:"node_status_sync_interval"`
		PodStatusSyncInterval  time.Duration `json:"pod_status_sync_interval" yaml:"pod_status_sync_interval"`
	} `json:"timers" yaml:"timers"`

	Timeouts struct {
	} `json:"timeouts" yaml:"timeouts"`
}
