package server

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	kubeListersCoreV1 "k8s.io/client-go/listers/core/v1"
)

type Resource struct {
	PodLister       kubeListersCoreV1.PodLister
	SecretLister    kubeListersCoreV1.SecretLister
	ConfigMapLister kubeListersCoreV1.ConfigMapLister
}

func (r *Resource) GetPods() []*corev1.Pod {
	pods, err := r.PodLister.List(labels.Everything())
	if err != nil {
		return []*corev1.Pod{}
	}
	return pods
}

func (r *Resource) GetPod(name types.NamespacedName) (*corev1.Pod, error) {
	return r.PodLister.Pods(name.Namespace).Get(name.Name)
}

func (r *Resource) GetSecret(name types.NamespacedName) (*corev1.Secret, error) {
	return r.SecretLister.Secrets(name.Namespace).Get(name.Name)
}

func (r *Resource) GetConfigMap(name types.NamespacedName) (*corev1.ConfigMap, error) {
	return r.ConfigMapLister.ConfigMaps(name.Namespace).Get(name.Name)
}
