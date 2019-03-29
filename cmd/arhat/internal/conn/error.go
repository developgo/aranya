package conn

import (
	"errors"
)

var (
	ErrConnectivityMethodNotSupported = errors.New("this connectivity method not supported ")
)
