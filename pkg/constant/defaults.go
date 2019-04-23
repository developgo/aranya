package constant

import (
	"time"
)

// Defaults intervals
const (
	DefaultNodeStatusSyncInterval = 10 * time.Second
	DefaultPodReSyncInterval      = time.Minute
)

// Default timeouts
const (
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
