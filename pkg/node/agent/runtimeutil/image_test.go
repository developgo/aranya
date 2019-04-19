package runtimeutil

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGenerateImageName(t *testing.T) {
	dockerHubImages := map[string]string{
		"alpine:latest":           "docker.io/library/alpine:latest",
		"library/alpine:latest":   "docker.io/library/alpine:latest",
		"user/foo:test":           "docker.io/user/foo:test",
		"docker.io/user/foo:test": "docker.io/user/foo:test",
	}
	for name, expectedName := range dockerHubImages {
		assert.Equal(t, expectedName, GenerateImageName(DefaultDockerImageDomain, DefaultDockerImageNamespace, name))
	}

	gcrImages := map[string]string{
		"k8s.gcr.io/pause:3.1": "k8s.gcr.io/pause:3.1",
		"pause:3.1":            "k8s.gcr.io/pause:3.1",
	}
	for name, expectedName := range gcrImages {
		assert.Equal(t, expectedName, GenerateImageName("k8s.gcr.io", "", name))
	}
}
