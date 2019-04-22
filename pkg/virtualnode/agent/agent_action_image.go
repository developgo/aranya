package agent

import (
	"arhat.dev/aranya/pkg/virtualnode/connectivity"
)

func (b *baseAgent) doImageList(sid uint64) {
	images, err := b.runtime.ListImages()
	if err != nil {
		b.handleError(sid, err)
		return
	}

	if len(images) == 0 {
		if err := b.doPostMsg(connectivity.NewImageMsg(sid, true, nil)); err != nil {
			b.handleError(sid, err)
		}
		return
	}

	lastIndex := len(images) - 1
	for i, p := range images {
		if err := b.doPostMsg(connectivity.NewImageMsg(sid, i == lastIndex, p)); err != nil {
			b.handleError(sid, err)
			return
		}
	}
}
