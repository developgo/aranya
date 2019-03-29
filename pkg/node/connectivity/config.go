package connectivity

import (
	"time"
)

type TLSConfig struct {
	Cert string `json:"cert" yaml:"cert"`
	Key  string `json:"key" yaml:"key"`
}

type Config struct {
	Server struct {
		Address     string        `json:"address" yaml:"address"`
		DialTimeout time.Duration `json:"dial_timeout" yaml:"dial_timeout"`

		TLS *TLSConfig `json:"tls" yaml:"tls"`
	} `json:"server" yaml:"server"`
}
