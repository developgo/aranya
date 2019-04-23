package runtimeutil

import (
	"arhat.dev/aranya/pkg/virtualnode/connectivity"
)

var (
	ErrNotFound      = connectivity.NewNotFoundError("")
	ErrNotSupported  = connectivity.NewNotSupportedError("")
	ErrAlreadyExists = connectivity.NewAlreadyExistsError("")
)
