package constant

import (
	"time"
)

const (
	DefaultNodeStatusSyncInterval = 10 * time.Second
	DefaultPodReSyncInterval      = time.Minute
)

// stream defaults
const (
	DefaultStreamIdleTimeout     = 4 * time.Hour
	DefaultStreamCreationTimeout = 30 * time.Second
)
