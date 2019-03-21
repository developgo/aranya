package connectivity

import (
	"time"
)

type Config struct {
	Method         string        `json:"method" yaml:"method"`
	DialTimeout    time.Duration `json:"dial_timeout" yaml:"dial_timeout"`
	ServerEndpoint string        `json:"server_endpoint" yaml:"server_endpoint"`
	TLSCert        string        `json:"tls_cert" yaml:"tls_cert"`
	TLSKey         string        `json:"tls_key" yaml:"tls_key"`
}
