package connectivity

import (
	"errors"

	corev1 "k8s.io/api/core/v1"
)

var (
	ErrUnknownPortForwardOption = errors.New("port forward options unresolved ")
)

func (o PortForwardOptions) GetResolvedOptions() ([]int32, error) {
	switch opt := o.GetOptions().(type) {
	case *PortForwardOptions_OptionsV1:
		options := &corev1.PodPortForwardOptions{}
		if err := options.Unmarshal(opt.OptionsV1); err != nil {
			return nil, err
		}

		return options.Ports, nil
	}

	return nil, ErrUnknownPortForwardOption
}
