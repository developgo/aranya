package node

import (
	"context"
	"fmt"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	kubeletContainer "k8s.io/kubernetes/pkg/kubelet/container"

	"arhat.dev/aranya/pkg/node/connectivity"
	"arhat.dev/aranya/pkg/node/resolver"
)

func (n *Node) InitializeRemoteDevice() {
	for !n.closing() {
		connCtx, cancel := context.WithCancel(n.ctx)
		wg := &sync.WaitGroup{}

		<-n.connectivityManager.DeviceConnected()
		n.log.Info("device connected")

		wg.Add(1)
		go func() {
			defer wg.Done()

			for msg := range n.connectivityManager.GlobalMessages() {
				n.handleGlobalMsg(msg)
			}
		}()

		n.log.Info("sync pods cache")
		if err := n.generateCacheForPodsInDevice(); err != nil {
			n.log.Error(err, "failed to sync pods cache")
			goto waitForDeviceDisconnect
		}
		n.log.Info("sync pods cache success")

		n.log.Info("sync node cache")
		if err := n.generateCacheForNodeInDevice(); err != nil {
			n.log.Error(err, "failed to sync node cache")
			goto waitForDeviceDisconnect
		}
		n.log.Info("sync node cache success")

		// sync node status after device has been connected
		go wait.Until(n.syncNodeStatus, 10*time.Second, connCtx.Done())

	waitForDeviceDisconnect:
		wg.Wait()
		cancel()
	}
}

// generate in cluster pod cache for remote device
func (n *Node) generateCacheForPodsInDevice() error {
	msgCh, err := n.connectivityManager.PostCmd(n.ctx, connectivity.NewPodListCmd("", "", true))
	if err != nil {
		return err
	}

	var podStatuses []*kubeletContainer.PodStatus
	for msg := range msgCh {
		if err := msg.Error(); err != nil {
			return err
		}

		switch podMsg := msg.GetMsg().(type) {
		case *connectivity.Msg_Pod:
			podStatus, err := podMsg.Pod.GetResolvedKubePodStatus()
			if err != nil {
				return err
			}
			podStatuses = append(podStatuses, podStatus)
		}
	}

	for _, podStatus := range podStatuses {
		apiPod, err := n.podManager.GetMirrorPod(podStatus.Namespace, podStatus.Name)
		if err != nil {
			if !errors.IsNotFound(err) {
				n.log.Error(err, "failed to get pod for pod cache sync")
				return err
			}

			n.deletePodInDevice(podStatus.ID)
			continue
		}

		if apiPod == nil {
			return fmt.Errorf("nil pod object")
		}

		// ok, process and generate pod cache

		apiStatus := n.GenerateAPIPodStatus(apiPod, podStatus)
		n.podStatusManager.SetPodStatus(apiPod, apiStatus)

		// generate cache
		pod := apiPod.DeepCopy()
		pod.Status = apiStatus
		n.podCache.Update(pod, podStatus)
	}

	return nil
}

// generate in cluster node cache for remote device
func (n *Node) generateCacheForNodeInDevice() error {
	msgCh, err := n.connectivityManager.PostCmd(n.ctx, connectivity.NewNodeCmd())
	if err != nil {
		return err
	}

	for msg := range msgCh {
		if err := msg.Error(); err != nil {
			return err
		}

		switch nodeMsg := msg.GetMsg().(type) {
		case *connectivity.Msg_Node:
			if err := n.updateNodeCache(nodeMsg); err != nil {
				return err
			}
		}
	}

	return nil
}

func (n *Node) handleGlobalMsg(msg *connectivity.Msg) {
	switch m := msg.GetMsg().(type) {
	case *connectivity.Msg_Ack:
		switch m.Ack.GetValue().(type) {
		case *connectivity.Ack_Error:
			n.log.Error(msg.Error(), "received error from remote device")
		}
	case *connectivity.Msg_Node:
		if err := n.updateNodeCache(m); err != nil {
			n.log.Error(err, "failed to update node cache")
		}
	case *connectivity.Msg_Pod:
		podStatus, err := m.Pod.GetResolvedKubePodStatus()
		if err != nil {
			n.log.Error(err, "failed to resolve pod status")
			return
		}

		apiPod, err := n.podManager.GetMirrorPod(podStatus.Namespace, podStatus.Name)
		if err != nil {
			if errors.IsNotFound(err) {
				n.deletePodInDevice(podStatus.ID)
				return
			}
			n.log.Error(err, "get mirror pod failed")
			return
		}
		n.podCache.Update(apiPod, podStatus)
	case *connectivity.Msg_Data:
		// we don't know how to handle this kind of message, discard
	}
}

func (n *Node) updateNodeCache(node *connectivity.Msg_Node) error {
	apiNode, err := node.Node.GetResolvedCoreV1Node()
	if err != nil {
		n.log.Error(err, "failed to resolve node")
		return err
	}

	me, err := n.nodeClient.Get(n.name, metav1.GetOptions{})
	if err != nil {
		n.log.Error(err, "failed to get node info")
		return err
	}
	if me == nil {
		n.log.Error(nil, "empty node object")
		return err
	}

	n.nodeStatusCache.Update(apiNode.Status)
	return nil
}

func (n *Node) createPodInDevice(pod *corev1.Pod) {
	containerEnvs := make(map[string]map[string]string)
	for _, ctr := range pod.Spec.Containers {
		envs, err := resolver.ResolveEnv(n.kubeClient, pod, &ctr)
		if err != nil {
			n.log.Error(err, "failed to resolve pod envs")
			return
		}
		containerEnvs[ctr.Name] = envs
	}

	secrets := make([]corev1.Secret, len(pod.Spec.ImagePullSecrets))
	for i, secretRef := range pod.Spec.ImagePullSecrets {
		s, err := n.kubeClient.CoreV1().Secrets(pod.Namespace).Get(secretRef.Name, metav1.GetOptions{})
		if err != nil {
			n.log.Error(err, "failed to get image pull secret")
			return
		}
		secrets[i] = *s
	}

	imagePullSecrets, err := resolver.ResolveImagePullSecret(pod, secrets)
	if err != nil {
		n.log.Error(err, "failed to resolve image pull secret")
		return
	}

	volumeData, hostVolume, err := resolver.ResolveVolume(n.kubeClient, pod)
	if err != nil {
		n.log.Error(err, "failed to resolve volume")
		return
	}

	podCreateCmd := connectivity.NewPodCreateCmd(pod, imagePullSecrets, containerEnvs, volumeData, hostVolume)
	msgCh, err := n.connectivityManager.PostCmd(n.ctx, podCreateCmd)
	if err != nil {
		n.log.Error(err, "failed to post pod create command")
	}

	for msg := range msgCh {
		if err := msg.Error(); err != nil {
			n.log.Error(err, "failed to create pod in device")
			return
		}

		switch podMsg := msg.GetMsg().(type) {
		case *connectivity.Msg_Pod:
			status, err := podMsg.Pod.GetResolvedKubePodStatus()
			if err != nil {
				n.log.Error(err, "failed to get resolved kube pod status")
				return
			}

			n.podCache.Update(pod, status)
		}
	}
}

func (n *Node) deletePodInDevice(podUID types.UID) {
	podDeleteCmd := connectivity.NewPodDeleteCmd(string(podUID), time.Minute)
	msgCh, err := n.connectivityManager.PostCmd(n.ctx, podDeleteCmd)
	if err != nil {
		n.log.Error(err, "failed to post pod delete command")
	}

	for msg := range msgCh {
		if err := msg.Error(); err != nil {
			n.log.Error(err, "failed to delete pod in device")
			return
		}
	}
}
