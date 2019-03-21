package connectivity

import (
	"errors"

	corev1 "k8s.io/api/core/v1"
	criRuntime "k8s.io/kubernetes/pkg/kubelet/apis/cri/runtime/v1alpha2"
)

var (
	ErrUnknownCreateOptions = errors.New("create options unresolved ")
)

func (p *CreateOptions) GetResolvedCreateOptions() (*corev1.PodSpec, map[string]*criRuntime.AuthConfig, map[string][]byte, error) {
	switch opt := p.GetOptions().(type) {
	case *CreateOptions_PodV1_:
		pod := &corev1.PodSpec{}
		if err := pod.Unmarshal(opt.PodV1.GetPodSpec()); err != nil {
			return nil, nil, nil, err
		}

		imageNameToAuthConfigMap := make(map[string]*criRuntime.AuthConfig)
		for imageName, authConfigBytes := range opt.PodV1.GetAuthConfig() {
			authConfig := &criRuntime.AuthConfig{}
			if err := authConfig.Unmarshal(authConfigBytes); err != nil {
				return nil, nil, nil, err
			}
			imageNameToAuthConfigMap[imageName] = authConfig
		}

		return pod, imageNameToAuthConfigMap, opt.PodV1.GetVolumeData(), nil
	}

	return nil, nil, nil, ErrUnknownCreateOptions
}
