package connectivity

import (
	"errors"

	corev1 "k8s.io/api/core/v1"
)

var (
	ErrUnknownExecOption = errors.New("exec options unresolved ")
)

func (o *ExecOptions) GetResolvedExecOptions() (*corev1.PodExecOptions, error) {
	switch opt := o.GetOptions().(type) {
	case *ExecOptions_OptionsV1:
		execOption := &corev1.PodExecOptions{}
		if err := execOption.Unmarshal(opt.OptionsV1); err != nil {
			return nil, err
		}
	}

	return nil, ErrUnknownExecOption
}
