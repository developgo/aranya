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

	PauseImage          string `json:"pause_image" yaml:"pause_image"`
	PauseCommand        string `json:"pause_command" yaml:"pause_command"`
	RootDir             string `json:"root_dir" yaml:"root_dir"`
	ManagementNamespace string `json:"management_namespace" yaml:"management_namespace"`

	// Optional
	EndpointDialTimeout time.Duration `json:"endpoint_dial_timeout" yaml:"endpoint_dial_timeout"`
	EndPoints           struct {
		Image   EndPoint `json:"image" yaml:"image"`
		Runtime EndPoint `json:"runtime" yaml:"runtime"`
	} `json:"endpoints" yaml:"endpoints"`
}

func (c *Config) PodsDir() string {
	return filepath.Join(c.RootDir, config.DefaultKubeletPodsDirName)
}

func (c *Config) PluginsDir() string {
	return filepath.Join(c.RootDir, config.DefaultKubeletPluginsDirName)
}

func (c *Config) PluginsRegistrationDir() string {
	return filepath.Join(c.RootDir, config.DefaultKubeletPluginsRegistrationDirName)
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
	return filepath.Join(c.RootDir, config.DefaultKubeletPodResourcesDirName)
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
		c.RootDir:                  0750,
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
