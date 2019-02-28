package node

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"arhat.dev/aranya/pkg/node/connectivity"
)

func (n *Node) InitializeRemoteDevice() {
	for !n.closing() {
		n.remoteManager.WaitUntilDeviceConnected()
		msgCh := n.remoteManager.ConsumeOrphanedMessage()
		for msg := range msgCh {
			switch msg.GetMsg().(type) {
			case *connectivity.Msg_DeviceInfo:
				deviceInfo := msg.GetDeviceInfo()
				// update node info
				spec := corev1.PodSpec{}
				status := corev1.PodStatus{}

				_ = spec.Unmarshal(deviceInfo.GetSpec())
				_ = status.Unmarshal(deviceInfo.GetStatus())

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

				updatedMe, err := n.nodeClient.UpdateStatus(me)
				if err != nil {
					n.log.Error(err, "update node status failed")
					return
				}
				_ = updatedMe

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
						break
					}
				}
			case *connectivity.Msg_PodInfo:
				_ = msg.GetPodInfo()
			case *connectivity.Msg_PodData:
				// we don't know how to handle this kind of message, discard
			}
		}
	}
}

func (n *Node) CreateOrUpdatePodInDevice(pod *corev1.Pod) error {
	cmd := connectivity.NewPodCreateOrUpdateCmd(pod)
	msgCh, err := n.remoteManager.PostCmd(cmd, 0)
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
	cmd := connectivity.NewPodDeleteCmd(namespace, name, 0)
	msgCh, err := n.remoteManager.PostCmd(cmd, 0)
	if err != nil {
		return err
	}

	for msg := range msgCh {
		_ = msg.GetPodInfo()
	}
	return nil
}
