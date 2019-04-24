// +build !linux

package client

import (
	"arhat.dev/aranya/pkg/virtualnode/connectivity"
)

func setSystemInfo(info *connectivity.NodeSystemInfo) *connectivity.NodeSystemInfo {
	return info
}
