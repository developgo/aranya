package node

import (
	"arhat.dev/aranya/pkg/node/connectivity"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (n *Node) InitializeRemoteDevice() {
	for !n.closing() {
		n.remoteDeviceManager.WaitUntilDeviceConnected()
		msgCh := n.remoteDeviceManager.ConsumeOrphanedMessage()
		for msg := range msgCh {
			switch msg.GetMessage().(type) {
			case *connectivity.Message_DeviceInfo:
				deviceInfo := msg.GetDeviceInfo()
				// update node info
				_ = deviceInfo.GetSpec()
				_ = deviceInfo.GetStatus()

				me, err := n.nodeClient.Get(n.name, metav1.GetOptions{})
				if err != nil {
					n.log.Error(err, "get own node info failed")
					return
				}

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

				me.ResourceVersion = ""

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

				updatedMe, err := n.nodeClient.UpdateStatus(me)
				if err != nil {
					n.log.Error(err, "update node status failed")
					return
				}
				_ = updatedMe
			case *connectivity.Message_PodInfo:
				_ = msg.GetPodInfo()
			case *connectivity.Message_PodData:
				// we don't know how to handle this kind of message, discard
			}
		}
	}
}

func (n *Node) CreateOrUpdatePodInDevice(pod *corev1.Pod) error {
	cmd := connectivity.NewPodCreateOrUpdateCmd(pod)
	msgCh, err := n.remoteDeviceManager.PostCmd(cmd, 0)
	if err != nil {
		return err
	}

	for msg := range msgCh {
		createdPod := msg.GetPodInfo()

		createdPod.GetPhase()
		createdPod.GetSpec()
		createdPod.GetStatus()
	}

	return nil
}

func (n *Node) DeletePodInDevice(namespace, name string) error {
	return nil
}
