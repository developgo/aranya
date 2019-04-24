package constant

import (
	"os"

	corev1 "k8s.io/api/core/v1"
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
		return corev1.NamespaceDefault
	}
	return ns
}
