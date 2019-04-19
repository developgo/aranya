package agent

import (
	"arhat.dev/aranya/pkg/node/connectivity"
)

func (c *baseAgent) doImageList(sid uint64) {
	images, err := c.runtime.ListImages()
	if err != nil {
		c.handleError(sid, err)
		return
	}

	if len(images) == 0 {
		if err := c.doPostMsg(connectivity.NewImageMsg(sid, true, nil)); err != nil {
			c.handleError(sid, err)
		}
		return
	}

	lastIndex := len(images) - 1
	for i, p := range images {
		if err := c.doPostMsg(connectivity.NewImageMsg(sid, i == lastIndex, p)); err != nil {
			c.handleError(sid, err)
			return
		}
	}
}
