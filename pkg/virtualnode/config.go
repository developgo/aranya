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

	aranya "arhat.dev/aranya/pkg/apis/aranya/v1alpha1"
	"arhat.dev/aranya/pkg/connectivity/server"
	"arhat.dev/aranya/pkg/virtualnode/pod"
)

type NodeServiceConfig struct {
	Timers struct {
		StatusSyncInterval time.Duration
	} `json:"timers" yaml:"timers"`
}

type Config struct {
	Connectivity server.Config     `json:"connectivity" yaml:"connectivity"`
	Node         NodeServiceConfig `json:"node" yaml:"node"`
	Pod          pod.Config        `json:"pod" yaml:"pod"`
	Stream       pod.StreamConfig  `json:"stream" yaml:"stream"`
}

func (c *Config) DeepCopy() *Config {
	return &Config{
		Connectivity: c.Connectivity,
		Node:         c.Node,
		Pod:          c.Pod,
		Stream:       c.Stream,
	}
}

// OverrideWith a config with higher priority (config from EdgeDevices)
func (c *Config) OverrideWith(a *aranya.ConnectivityTimers) *Config {
	if a == nil {
		return c
	}

	newConfig := c.DeepCopy()
	if a.ForceNodeStatusSyncInterval != nil {
		newConfig.Connectivity.Timers.ForceNodeStatusSyncInterval = a.ForceNodeStatusSyncInterval.Duration
	}

	if a.ForcePodStatusSyncInterval != nil {
		newConfig.Connectivity.Timers.ForcePodStatusSyncInterval = a.ForcePodStatusSyncInterval.Duration
	}

	if a.UnarySessionTimeout != nil {
		newConfig.Connectivity.Timers.UnarySessionTimeout = a.UnarySessionTimeout.Duration
	}

	return newConfig
}
