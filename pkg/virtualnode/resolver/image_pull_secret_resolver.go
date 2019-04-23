package resolver

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/kubernetes/pkg/credentialprovider"
	"k8s.io/kubernetes/pkg/credentialprovider/secrets"
	"k8s.io/kubernetes/pkg/util/parsers"

	"arhat.dev/aranya/pkg/virtualnode/connectivity"
)

func ResolveImagePullSecret(pod *corev1.Pod, secret []corev1.Secret) (map[string]*connectivity.AuthConfig, error) {
	imageNameToAuthConfigMap := make(map[string]*connectivity.AuthConfig)

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
			imageNameToAuthConfigMap[apiCtr.Image] = &connectivity.AuthConfig{
				Username:      authConfig.Username,
				Password:      authConfig.Password,
				Auth:          authConfig.Auth,
				ServerAddress: authConfig.ServerAddress,
				IdentityToken: authConfig.IdentityToken,
				RegistryToken: authConfig.RegistryToken,
			}
		}
	}

	return imageNameToAuthConfigMap, nil
}
