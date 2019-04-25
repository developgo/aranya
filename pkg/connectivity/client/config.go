/*
Copyright 2019 The arhat.dev Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

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
