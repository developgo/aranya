package resolver

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/kubernetes/pkg/credentialprovider"
	"k8s.io/kubernetes/pkg/credentialprovider/secrets"
	criRuntime "k8s.io/kubernetes/pkg/kubelet/apis/cri/runtime/v1alpha2"
	"k8s.io/kubernetes/pkg/util/parsers"
)

func ResolveImagePullSecret(pod *corev1.Pod, secret []corev1.Secret) (map[string]*criRuntime.AuthConfig, error) {
	imageNameToAuthConfigMap := make(map[string]*criRuntime.AuthConfig)

	keyring, err := secrets.MakeDockerKeyring(secret, credentialprovider.NewDockerKeyring())
	if err != nil {
		return nil, err
	}

	for _, apiCtr := range pod.Spec.Containers {
		repoToPull, _, _, err := parsers.ParseImageName(apiCtr.Image)
		if err != nil {
			return nil, err
		}

		creds, withCredentials := keyring.Lookup(repoToPull)
		if !withCredentials {
			// pull without credentials
		}

		for _, currentCreds := range creds {
			authConfig := credentialprovider.LazyProvide(currentCreds)
			auth := &criRuntime.AuthConfig{
				Username:      authConfig.Username,
				Password:      authConfig.Password,
				Auth:          authConfig.Auth,
				ServerAddress: authConfig.ServerAddress,
				IdentityToken: authConfig.IdentityToken,
				RegistryToken: authConfig.RegistryToken,
			}

			imageNameToAuthConfigMap[apiCtr.Image] = auth
		}
	}

	return imageNameToAuthConfigMap, nil
}
