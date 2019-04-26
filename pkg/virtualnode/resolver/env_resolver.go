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
	"fmt"
	"sort"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/pkg/api/v1/resource"
	"k8s.io/kubernetes/pkg/apis/core/pods"
	"k8s.io/kubernetes/pkg/kubectl/util/fieldpath"
	"k8s.io/kubernetes/third_party/forked/golang/expansion"
)

func ResolveEnv(kubeClient kubernetes.Interface, pod *corev1.Pod, container *corev1.Container) (map[string]string, error) {
	result := make(map[string]string)

	var (
		configMaps = make(map[string]*corev1.ConfigMap)
		secrets    = make(map[string]*corev1.Secret)
		tmpEnv     = make(map[string]string)
		err        error
	)

	// Env will override EnvFrom variables.
	// Process EnvFrom first then allow Env to replace existing values.
	for _, envFrom := range container.EnvFrom {
		switch {
		case envFrom.ConfigMapRef != nil:
			cm := envFrom.ConfigMapRef
			name := cm.Name
			configMap, ok := configMaps[name]
			if !ok {
				optional := cm.Optional != nil && *cm.Optional
				configMap, err = kubeClient.CoreV1().ConfigMaps(pod.Namespace).Get(name, metav1.GetOptions{})
				if err != nil {
					if errors.IsNotFound(err) && optional {
						// ignore error when marked optional
						continue
					}
					return result, err
				}
				configMaps[name] = configMap
			}

			var invalidKeys []string
			for k, v := range configMap.Data {
				if len(envFrom.Prefix) > 0 {
					k = envFrom.Prefix + k
				}
				if errMsgs := validation.IsEnvVarName(k); len(errMsgs) != 0 {
					invalidKeys = append(invalidKeys, k)
					continue
				}
				tmpEnv[k] = v
			}
			if len(invalidKeys) > 0 {
				sort.Strings(invalidKeys)
			}
		case envFrom.SecretRef != nil:
			s := envFrom.SecretRef
			name := s.Name
			secret, ok := secrets[name]
			if !ok {
				optional := s.Optional != nil && *s.Optional
				secret, err = kubeClient.CoreV1().Secrets(pod.Namespace).Get(name, metav1.GetOptions{})
				if err != nil {
					if errors.IsNotFound(err) && optional {
						// ignore error when marked optional
						continue
					}
					return result, err
				}
				secrets[name] = secret
			}

			var invalidKeys []string
			for k, v := range secret.Data {
				if len(envFrom.Prefix) > 0 {
					k = envFrom.Prefix + k
				}
				if errMsgs := validation.IsEnvVarName(k); len(errMsgs) != 0 {
					invalidKeys = append(invalidKeys, k)
					continue
				}
				tmpEnv[k] = string(v)
			}
			if len(invalidKeys) > 0 {
				sort.Strings(invalidKeys)
				// kl.recorder.Eventf(pod, v1.EventTypeWarning, "InvalidEnvironmentVariableNames", "Keys [%s] from the EnvFrom secret %s/%s were skipped since they are considered invalid environment variable names.", strings.Join(invalidKeys, ", "), pod.Namespace, name)
			}
		}
	}

	// Determine the final values of variables:
	//
	// 1.  Determine the final value of each variable:
	//     a.  If the variable's Value is set, expand the `$(var)` references to other
	//         variables in the .Value field; the sources of variables are the declared
	//         variables of the container and the service environment variables
	//     b.  If a source is defined for an environment variable, resolve the source
	// 2.  Create the container's environment in the order variables are declared
	// 3.  Add remaining service environment vars
	var (
		mappingFunc = expansion.MappingFuncFor(tmpEnv)
	)
	for _, envVar := range container.Env {
		runtimeVal := envVar.Value
		if runtimeVal != "" {
			// Step 1a: expand variable references
			runtimeVal = expansion.Expand(runtimeVal, mappingFunc)
		} else if envVar.ValueFrom != nil {
			// Step 1b: resolve alternate env var sources
			switch {
			case envVar.ValueFrom.FieldRef != nil:
				runtimeVal, err = podFieldSelectorRuntimeValue(envVar.ValueFrom.FieldRef, pod, "")
				if err != nil {
					return result, err
				}
			case envVar.ValueFrom.ResourceFieldRef != nil:
				// defaultedPod, defaultedContainer, err := kl.defaultPodLimitsForDownwardAPI(pod, container)
				// if err != nil {
				// 	return result, err
				// }
				runtimeVal, err = containerResourceRuntimeValue(envVar.ValueFrom.ResourceFieldRef, pod, container)
				if err != nil {
					return result, err
				}
			case envVar.ValueFrom.ConfigMapKeyRef != nil:
				cm := envVar.ValueFrom.ConfigMapKeyRef
				name := cm.Name
				key := cm.Key
				optional := cm.Optional != nil && *cm.Optional
				configMap, ok := configMaps[name]
				if !ok {
					configMap, err := kubeClient.CoreV1().ConfigMaps(pod.Namespace).Get(name, metav1.GetOptions{})
					if err != nil {
						if errors.IsNotFound(err) && optional {
							// ignore error when marked optional
							continue
						}
						return result, err
					}
					configMaps[name] = configMap
				}
				runtimeVal, ok = configMap.Data[key]
				if !ok {
					if optional {
						continue
					}
					return result, fmt.Errorf("failed to find key %v in ConfigMap %v/%v", key, pod.Namespace, name)
				}
			case envVar.ValueFrom.SecretKeyRef != nil:
				s := envVar.ValueFrom.SecretKeyRef
				name := s.Name
				key := s.Key
				optional := s.Optional != nil && *s.Optional
				secret, ok := secrets[name]
				if !ok {
					secret, err = kubeClient.CoreV1().Secrets(pod.Namespace).Get(name, metav1.GetOptions{})
					if err != nil {
						if errors.IsNotFound(err) && optional {
							// ignore error when marked optional
							continue
						}
						return result, err
					}
					secrets[name] = secret
				}
				runtimeValBytes, ok := secret.Data[key]
				if !ok {
					if optional {
						continue
					}
					return result, fmt.Errorf("failed to find key %v in Secret %v/%v", key, pod.Namespace, name)
				}
				runtimeVal = string(runtimeValBytes)
			}
		}

		tmpEnv[envVar.Name] = runtimeVal
	}

	// Append the env vars
	for k, v := range tmpEnv {
		result[k] = v
	}

	return result, nil
}

// containerResourceRuntimeValue returns the value of the provided container resource
func containerResourceRuntimeValue(fs *corev1.ResourceFieldSelector, pod *corev1.Pod, container *corev1.Container) (string, error) {
	containerName := fs.ContainerName
	if len(containerName) == 0 {
		return resource.ExtractContainerResourceValue(fs, container)
	}
	return resource.ExtractResourceValueByContainerName(fs, pod, containerName)
}

// podFieldSelectorRuntimeValue returns the runtime value of the given
// selector for a pod.
func podFieldSelectorRuntimeValue(fs *corev1.ObjectFieldSelector, pod *corev1.Pod, podIP string) (string, error) {
	internalFieldPath, _, err := pods.ConvertDownwardAPIFieldLabel(fs.APIVersion, fs.FieldPath, "")
	if err != nil {
		return "", err
	}
	switch internalFieldPath {
	case "spec.nodeName":
		return pod.Spec.NodeName, nil
	case "spec.serviceAccountName":
		return pod.Spec.ServiceAccountName, nil
	case "status.hostIP":
		// hostIP, err := kl.getHostIPAnyWay()
		// if err != nil {
		// 	return "", err
		// }
		// return hostIP.String(), nil
	case "status.podIP":
		return podIP, nil
	}
	return fieldpath.ExtractFieldPathAsString(pod, internalFieldPath)
}
