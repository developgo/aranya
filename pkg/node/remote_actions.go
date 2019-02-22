package node

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (s *Node) SetupDevice() {
	me, err := s.nodeClient.Get(s.name, metav1.GetOptions{})
	if err != nil {
		log.Error(err, "get own node info failed")
		return
	}

	// TODO: wait edge device to connect and get info

	me.Status = corev1.NodeStatus{
		NodeInfo: corev1.NodeSystemInfo{
			MachineID:               "",
			SystemUUID:              "",
			BootID:                  "",
			KernelVersion:           "",
			OSImage:                 "",
			ContainerRuntimeVersion: "",
			KubeletVersion:          "",
			KubeProxyVersion:        "",
			OperatingSystem:         "linux",
			Architecture:            "arm",
		},
	}

	updatedMe, err := s.nodeClient.UpdateStatus(me)
	if err != nil {
		log.Error(err, "update node status failed")
		return
	}
	_ = updatedMe
}

func (s *Node) CreateOrUpdatePodInDevice(pod *corev1.Pod) error {
	return nil
}

func (s *Node) DeletePodInDevice(namespace, name string) error {
	return nil
}
