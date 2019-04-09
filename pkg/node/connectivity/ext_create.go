package connectivity

import (
	criRuntime "k8s.io/kubernetes/pkg/kubelet/apis/cri/runtime/v1alpha2"
)

func (p *CreateOptions) GetResolvedImagePullAuthConfig() (map[string]*criRuntime.AuthConfig, error) {
	imageNameToAuthConfigMap := make(map[string]*criRuntime.AuthConfig)
	for imageName, authConfigBytes := range p.GetImagePullAuthConfig() {
		authConfig := &criRuntime.AuthConfig{}
		if err := authConfig.Unmarshal(authConfigBytes); err != nil {
			return nil, err
		}
		imageNameToAuthConfigMap[imageName] = authConfig
	}

	return imageNameToAuthConfigMap, nil
}
