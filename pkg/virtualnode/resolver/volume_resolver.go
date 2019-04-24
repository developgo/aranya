package resolver

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeClient "k8s.io/client-go/kubernetes"

	"arhat.dev/aranya/pkg/virtualnode/connectivity"
)

func ResolveVolumeData(kubeClient kubeClient.Interface, pod *corev1.Pod) (volumeData map[string]*connectivity.NamedData, err error) {
	volumeData = make(map[string]*connectivity.NamedData)

	configMapClient := kubeClient.CoreV1().ConfigMaps(pod.Namespace)
	secretClient := kubeClient.CoreV1().Secrets(pod.Namespace)

	for _, vol := range pod.Spec.Volumes {
		switch {
		case vol.ConfigMap != nil:
			optional := vol.ConfigMap.Optional != nil && *vol.ConfigMap.Optional

			configMap, err := configMapClient.Get(vol.ConfigMap.Name, metav1.GetOptions{})
			if err != nil {
				if errors.IsNotFound(err) && optional {
					continue
				}
				return nil, err
			}

			namedData := &connectivity.NamedData{DataMap: make(map[string][]byte)}

			for dataName, data := range configMap.Data {
				namedData.DataMap[dataName] = []byte(data)
			}

			for dataName, data := range configMap.BinaryData {
				namedData.DataMap[dataName] = data
			}

			volumeData[vol.Name] = namedData
		case vol.Secret != nil:
			optional := vol.Secret.Optional != nil && *vol.Secret.Optional

			secret, err := secretClient.Get(vol.Secret.SecretName, metav1.GetOptions{})
			if err != nil {
				if errors.IsNotFound(err) && optional {
					continue
				}
				return nil, err
			}

			namedData := &connectivity.NamedData{DataMap: make(map[string][]byte)}
			for dataName, data := range secret.StringData {
				namedData.DataMap[dataName] = []byte(data)
			}

			for dataName, dataVal := range secret.Data {
				namedData.DataMap[dataName] = dataVal
			}

			volumeData[vol.Name] = namedData
		}
	}

	return volumeData, nil
}
