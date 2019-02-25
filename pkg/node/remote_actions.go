package node

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (n *Node) InitializeDevice() {
	me, err := n.nodeClient.Get(n.name, metav1.GetOptions{})
	if err != nil {
		log.Error(err, "get own node info failed")
		return
	}

	// TODO: wait edge device to connect and get info

	me.Status.NodeInfo = corev1.NodeSystemInfo{
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
	}

	updatedMe, err := n.nodeClient.UpdateStatus(me)
	if err != nil {
		log.Error(err, "update node status failed")
		return
	}
	_ = updatedMe
}

func (n *Node) CreateOrUpdatePodInDevice(pod *corev1.Pod) error {
	return nil
}

func (n *Node) DeletePodInDevice(namespace, name string) error {
	return nil
}
