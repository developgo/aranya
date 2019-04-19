package runtimeutil

import (
	"path/filepath"
	"strings"
)

const (
	DefaultDockerImageDomain    = "docker.io"
	DefaultDockerImageNamespace = "library"
)

// GenerateImageName create a image name with defaults according to provided name
// defaultDomain MUST NOT be empty
func GenerateImageName(defaultDomain, defaultNamespace, name string) string {
	firstSlashIndex := strings.IndexByte(name, '/')
	switch firstSlashIndex {
	case -1:
		// no slash, add default registry
		return filepath.Join(defaultDomain, defaultNamespace, name)
	default:
		prefix := name[:firstSlashIndex]
		if strings.Contains(prefix, ".") {
			return name
		} else {
			return filepath.Join(defaultDomain, name)
		}
	}
}
