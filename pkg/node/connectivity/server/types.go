package server

import (
	"errors"
	"time"

	"arhat.dev/aranya/pkg/node/connectivity"
)

var (
	ErrDeviceNotConnected = errors.New("device not connected ")
)

type Interface interface {
	WaitUntilDeviceConnected()
	ConsumeOrphanedMessage() <-chan *connectivity.Msg
	// send a command to remote device with timeout
	// return a channel of message for this session
	PostCmd(c *connectivity.Cmd, timeout time.Duration) (ch <-chan *connectivity.Msg, err error)
}
