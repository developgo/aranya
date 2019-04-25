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

package pod

import (
	"time"
)

type Config struct {
	Timers struct {
		ReSyncInterval time.Duration `json:"resync_interval" yaml:"resync_interval"`
	} `json:"timers" yaml:"timers"`
}

type StreamConfig struct {
	Timers struct {
		CreationTimeout time.Duration `json:"creation_timeout" yaml:"creation_timeout"`
		IdleTimeout     time.Duration `json:"idle_timeout" yaml:"idle_timeout"`
	} `json:"timers" yaml:"timers"`
}