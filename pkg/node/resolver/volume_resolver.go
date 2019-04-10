package resolver

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeClient "k8s.io/client-go/kubernetes"
)

func ResolveVolume(kubeClient kubeClient.Interface, pod *corev1.Pod) (volumeData map[string]map[string][]byte, hostVolume map[string]string, err error) {
	volumeData = make(map[string]map[string][]byte)
	hostVolume = make(map[string]string)

	configMapClient := kubeClient.CoreV1().ConfigMaps(pod.Namespace)
	secretClient := kubeClient.CoreV1().Secrets(pod.Namespace)

	for _, vol := range pod.Spec.Volumes {
		switch {
		case vol.HostPath != nil:
			hostVolume[vol.Name] = vol.HostPath.Path
		case vol.ConfigMap != nil:
			optional := vol.ConfigMap.Optional != nil && *vol.ConfigMap.Optional

			configMap, err := configMapClient.Get(vol.ConfigMap.Name, metav1.GetOptions{})
			if err != nil {
				if errors.IsNotFound(err) && optional {
					continue
				}
				return nil, nil, err
			}

			namedData := make(map[string][]byte)

			for dataName, data := range configMap.Data {
				namedData[dataName] = []byte(data)
			}

			for dataName, data := range configMap.BinaryData {
				namedData[dataName] = data
			}

			volumeData[vol.Name] = namedData
		case vol.Secret != nil:
			optional := vol.Secret.Optional != nil && *vol.Secret.Optional

			secret, err := secretClient.Get(vol.Secret.SecretName, metav1.GetOptions{})
			if err != nil {
				if errors.IsNotFound(err) && optional {
					continue
				}
				return nil, nil, err
			}

			namedData := make(map[string][]byte)

			for dataName, dataVal := range secret.Data {
				namedData[dataName] = dataVal
			}

			volumeData[vol.Name] = namedData
		}
	}
	return
}
