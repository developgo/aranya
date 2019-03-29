package runtime

import (
	"errors"
)

var (
	ErrRuntimeNotSupported = errors.New("this runtime is not supported ")
)
