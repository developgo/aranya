/*
Copyright 2019 The arhat.dev Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package resolver

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/pkg/credentialprovider"
	"k8s.io/kubernetes/pkg/credentialprovider/secrets"
	"k8s.io/kubernetes/pkg/util/parsers"

	"arhat.dev/aranya/pkg/virtualnode/connectivity"
)

func ResolveImagePullAuthConfig(kubeClient kubernetes.Interface, pod *corev1.Pod) (map[string]*connectivity.AuthConfig, error) {
	secret := make([]corev1.Secret, len(pod.Spec.ImagePullSecrets))
	for i, secretRef := range pod.Spec.ImagePullSecrets {
		s, err := kubeClient.CoreV1().Secrets(pod.Namespace).Get(secretRef.Name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		secret[i] = *s
	}

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
