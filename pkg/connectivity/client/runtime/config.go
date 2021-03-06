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

package runtime

import (
	"os"
	"path/filepath"
	"sync"
	"time"

	"k8s.io/kubernetes/pkg/kubelet/config"
)

const (
	ContainerLogsDir = "/var/log/containers"
)

type EndPoint struct {
	Address       string        `json:"address" yaml:"address"`
	DialTimeout   time.Duration `json:"dial_timeout" yaml:"dial_timeout"`
	ActionTimeout time.Duration `json:"action_timeout" yaml:"action_timeout"`
}

type Config struct {
	once sync.Once

	DataDir string `json:"data_dir" yaml:"data_dir"`

	// pause image and command
	PauseImage   string `json:"pause_image" yaml:"pause_image"`
	PauseCommand string `json:"pause_command" yaml:"pause_command"`

	// ManagementNamespace the name used to separate container's view
	// used by containerd and podman
	ManagementNamespace string `json:"management_namespace" yaml:"management_namespace"`

	Defaults struct {
		ImageDomain string `json:"image_domain" yaml:"image_domain"`
		ImageRepo   string `json:"image_repo" yaml:"image_repo"`
	} `json:"defaults" yaml:"defaults"`

	// Optional
	EndPoints struct {
		// image endpoint
		Image EndPoint `json:"image" yaml:"image"`
		// runtime endpoint
		Runtime EndPoint `json:"runtime" yaml:"runtime"`
	} `json:"endpoints" yaml:"endpoints"`
}

func (c *Config) PodsDir() string {
	return filepath.Join(c.DataDir, config.DefaultKubeletPodsDirName)
}

func (c *Config) PluginsDir() string {
	return filepath.Join(c.DataDir, config.DefaultKubeletPluginsDirName)
}

func (c *Config) PluginsRegistrationDir() string {
	return filepath.Join(c.DataDir, config.DefaultKubeletPluginsRegistrationDirName)
}

func (c *Config) PluginDir(pluginName string) string {
	return filepath.Join(c.PluginsDir(), pluginName)
}

func (c *Config) PodDir(podUID string) string {
	return filepath.Join(c.PodsDir(), podUID)
}

func (c *Config) PodVolumesDir(podUID string) string {
	return filepath.Join(c.PodDir(podUID), config.DefaultKubeletVolumesDirName)
}

func (c *Config) PodVolumeDir(podUID, pluginName, volumeName string) string {
	return filepath.Join(c.PodVolumesDir(podUID), pluginName, volumeName)
}

func (c *Config) PodPluginsDir(podUID string) string {
	return filepath.Join(c.PodDir(podUID), config.DefaultKubeletPluginsDirName)
}

func (c *Config) PodPluginDir(podUID, pluginName string) string {
	return filepath.Join(c.PodPluginsDir(podUID), pluginName)
}

func (c *Config) PodContainerDir(podUID, containerName string) string {
	return filepath.Join(c.PodDir(podUID), config.DefaultKubeletContainersDirName, containerName)
}

func (c *Config) PodResourcesDir() string {
	return filepath.Join(c.DataDir, config.DefaultKubeletPodResourcesDirName)
}

func (c *Config) ContainerLogsDir() string {
	return ContainerLogsDir
}

func (c *Config) Init() error {
	var err error
	c.once.Do(func() {
		if err = c.ensureAllDir(); err != nil {
			return
		}
	})

	return err
}

func (c *Config) ensureAllDir() error {
	requiredDirs := map[string]os.FileMode{
		c.DataDir:                  0750,
		c.PodsDir():                0750,
		c.PluginsDir():             0750,
		c.PluginsRegistrationDir(): 0750,
		c.PodResourcesDir():        0750,
		c.ContainerLogsDir():       0755,
	}

	for dir, mode := range requiredDirs {
		err := c.ensureDir(dir, mode)
		if err != nil {
			return err
		}
	}

	return nil
}

func (c *Config) ensureDir(dir string, mode os.FileMode) error {
	_, err := os.Stat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			if err := os.MkdirAll(dir, mode); err != nil {
				return err
			}
			return nil
		}

		return err
	}

	return nil
}
