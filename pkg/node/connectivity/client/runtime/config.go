package runtime

import (
	"os"
	"sync"
	"time"
)

type EndPoint struct {
	Address       string        `json:"address" yaml:"address"`
	DialTimeout   time.Duration `json:"dial_timeout" yaml:"dial_timeout"`
	ActionTimeout time.Duration `json:"action_timeout" yaml:"action_timeout"`
}

type Config struct {
	once sync.Once

	PauseImage         string `json:"pause_image" yaml:"pause_image"`
	PauseCommand       string `json:"pause_command" yaml:"pause_command"`
	VolumeDataDir      string `json:"volume_data_dir" yaml:"volume_data_dir"`
	ContainerNamespace string `json:"container_namespace" yaml:"container_namespace"`

	// Optional
	EndpointDialTimeout time.Duration `json:"endpoint_dial_timeout" yaml:"endpoint_dial_timeout"`
	EndPoints           struct {
		Image   EndPoint `json:"image" yaml:"image"`
		Runtime EndPoint `json:"runtime" yaml:"runtime"`
	} `json:"endpoints" yaml:"endpoints"`
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
	for _, dir := range []string{c.VolumeDataDir} {
		err := c.ensureDir(dir)
		if err != nil {
			return err
		}
	}

	return nil
}

func (c *Config) ensureDir(dir string) error {
	_, err := os.Stat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			if err := os.MkdirAll(dir, 0755); err != nil {
				return err
			}
			return nil
		}

		return err
	}

	return nil
}
