package node

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (n *Node) InitializeDevice() {
	me, err := n.nodeClient.Get(n.name, metav1.GetOptions{})
	if err != nil {
		n.log.Error(err, "get own node info failed")
		return
	}

	// TODO: wait edge device to connect and get node info
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

	// TODO:
	//   	- get on device pods
	// 		-

	podOnDevice := make([]*corev1.Pod, 0)
	podToDelete := make([]*corev1.Pod, 0)
	for _, p := range podOnDevice {
		if _, err := n.podManager.GetMirrorPod(p.Namespace, p.Name); err != nil {
			if errors.IsNotFound(err) {
				// The current pod does not exist in Kubernetes, so we mark it for deletion.
				podToDelete = append(podToDelete, p)
				continue
			}
			n.log.Error(err, "get mirror pod failed")
			return
		}
	}

	me.ResourceVersion = ""

	updatedMe, err := n.nodeClient.UpdateStatus(me)
	if err != nil {
		n.log.Error(err, "update node status failed")
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
