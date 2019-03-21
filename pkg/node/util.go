package node

import (
	"sync"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/kubernetes/pkg/credentialprovider"
	"k8s.io/kubernetes/pkg/credentialprovider/secrets"
	criRuntime "k8s.io/kubernetes/pkg/kubelet/apis/cri/runtime/v1alpha2"
	kubeletContainer "k8s.io/kubernetes/pkg/kubelet/container"
	"k8s.io/kubernetes/pkg/util/parsers"
)

func newPodCache() *PodCache {
	return &PodCache{
		m: make(map[types.NamespacedName]*PodPair),
	}
}

type PodPair struct {
	pod    *corev1.Pod
	status *kubeletContainer.PodStatus
}

type PodCache struct {
	// pod full name to kubeletContainer.PodPair
	m  map[types.NamespacedName]*PodPair
	mu sync.RWMutex
}

func (c *PodCache) Update(pod *corev1.Pod, status *kubeletContainer.PodStatus) {
	c.mu.Lock()
	defer c.mu.Unlock()

	name := types.NamespacedName{Namespace: pod.Namespace, Name: pod.Name}
	c.m[name] = &PodPair{
		pod:    pod,
		status: status,
	}
}

func (c *PodCache) Get(name types.NamespacedName) (*corev1.Pod, *kubeletContainer.PodStatus) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	podPair, ok := c.m[name]
	if !ok {
		return nil, nil
	}

	return podPair.pod, podPair.status
}

func ParseImagePullSecret(pod corev1.Pod, secret []corev1.Secret) (map[string]*criRuntime.AuthConfig, error) {
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
