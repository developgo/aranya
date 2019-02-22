package edgedevice

import (
	"github.com/phayes/freeport"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	aranyav1alpha1 "arhat.dev/aranya/pkg/apis/aranya/v1alpha1"
)

func ownerRefFromEdgeDevice(device *aranyav1alpha1.EdgeDevice) []metav1.OwnerReference {
	trueOption := true
	return []metav1.OwnerReference{{
		APIVersion:         device.APIVersion,
		Kind:               device.Kind,
		Name:               device.Name,
		UID:                device.UID,
		Controller:         &trueOption,
		BlockOwnerDeletion: &trueOption,
	}}
}

func containsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

func removeString(slice []string, s string) (result []string) {
	for _, item := range slice {
		if item == s {
			continue
		}
		result = append(result, item)
	}
	return
}

func getFreePort() int32 {
	port, err := freeport.GetFreePort()
	if err != nil {
		return 0
	}
	return int32(port)
}
