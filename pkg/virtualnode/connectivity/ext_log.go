package connectivity

import (
	"errors"

	corev1 "k8s.io/api/core/v1"
)

var (
	ErrUnknownLogOption = errors.New("log options unresolved ")
)

func (o *LogOptions) GetResolvedLogOptions() (*corev1.PodLogOptions, error) {
	switch opt := o.GetOptions().(type) {
	case *LogOptions_OptionsV1:
		logOptions := &corev1.PodLogOptions{}
		if err := logOptions.Unmarshal(opt.OptionsV1); err != nil {
			return nil, err
		}
		return logOptions, nil
	}
	return nil, ErrUnknownLogOption
}
