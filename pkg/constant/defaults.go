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

package constant

import (
	"time"
)

// Default file and dirs
const (
	// aranya defaults
	DefaultAranyaConfigFile = "/etc/aranya/config.yaml"
	DefaultAranyaLogDir     = "/var/log/aranya"

	// arhat defaults
	DefaultArhatConfigFile     = "/etc/arhat/config.yaml"
	DefaultArhatLogDir         = "/var/log/arhat"
	DefaultArhatDataDir        = "/var/lib/arhat/data"
	DefaultPauseImage          = "k8s.gcr.io/pause:3.1"
	DefaultPauseCommand        = "/pause"
	DefaultManagementNamespace = "container.arhat.dev"
)

// Defaults intervals
const (
	DefaultNodeStatusSyncInterval = 10 * time.Second
	DefaultPodReSyncInterval      = time.Minute
)

// Default timeouts
const (
	DefaultUnarySessionTimeout   = time.Minute
	DefaultStreamIdleTimeout     = 4 * time.Hour
	DefaultStreamCreationTimeout = 30 * time.Second
)

// Default retry times
const (
	DefaultNodeStatusUpdateRetry = 5
)

// Default channel size
const (
	DefaultConnectivityMsgChannelSize = 2
)
