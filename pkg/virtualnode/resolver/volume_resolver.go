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
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"arhat.dev/aranya/pkg/connectivity"
)

func ResolveVolumeData(kubeClient kubernetes.Interface, pod *corev1.Pod) (volumeData map[string]*connectivity.NamedData, err error) {
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
