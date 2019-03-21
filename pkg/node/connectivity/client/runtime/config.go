package runtime

import (
	"os"
	"sync"
)

type Config struct {
	once sync.Once

	PauseImage    string `json:"pause_image"`
	PauseCommand  string `json:"pause_command"`
	VolumeDataDir string `json:"volume_data_dir"`
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
