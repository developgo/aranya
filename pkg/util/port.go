package util

import (
	"github.com/phayes/freeport"
)

func GetFreePort() int32 {
	port, err := freeport.GetFreePort()
	if err != nil {
		return 0
	}
	return int32(port)
}
