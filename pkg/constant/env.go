package constant

import (
	"os"
)

const (
	EnvKeyPodName        = "POD_NAME"
	EnvKeyWatchNamespace = "WATCH_NAMESPACE"
)

func CurrentPodName() string {
	return os.Getenv(EnvKeyPodName)
}

func CurrentNamespace() string {
	ns := os.Getenv(EnvKeyWatchNamespace)
	if ns == "" {
		return "default"
	}

	return ns
}
