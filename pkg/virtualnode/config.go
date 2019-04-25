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

package virtualnode

import (
	"time"
)

type NodeServiceConfig struct {
	Timers struct {
		StatusSyncInterval time.Duration
	} `json:"timers" yaml:"timers"`
}

type PodServiceConfig struct {
	Timers struct {
		ReSyncInterval time.Duration
	} `json:"timers" yaml:"timers"`
}

type StreamServiceConfig struct {
	Timers struct {
		CreationTimeout time.Duration `json:"creation_timeout" yaml:"creation_timeout"`
		IdleTimeout     time.Duration `json:"idle_timeout" yaml:"idle_timeout"`
	} `json:"timers" yaml:"timers"`
}

type ConnectivityServerConfig struct {
	Timers struct {
		UnarySessionTimeout         time.Duration `json:"unary_session_timeout" yaml:"unary_session_timeout"`
		StreamSessionTimeout        time.Duration `json:"stream_session_timeout" yaml:"stream_session_timeout"`
		ForceNodeStatusSyncInterval time.Duration `json:"force_node_status_sync_interval" yaml:"force_node_status_sync_interval"`
		ForcePodStatusSyncInterval  time.Duration `json:"force_pod_status_sync_interval" yaml:"force_pod_status_sync_interval"`
	} `json:"timers" yaml:"timers"`
}

type Config struct {
	Connectivity ConnectivityServerConfig `json:"connectivity" yaml:"connectivity"`

	Services struct {
		Node   NodeServiceConfig   `json:"node" yaml:"node"`
		Pod    PodServiceConfig    `json:"pod" yaml:"pod"`
		Stream StreamServiceConfig `json:"stream" yaml:"stream"`
	} `json:"services" yaml:"services"`
}
