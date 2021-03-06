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

package edgedevice

import (
	"fmt"
	"sync"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"arhat.dev/aranya/pkg/constant"
	"arhat.dev/aranya/pkg/virtualnode"
)

var (
	globalNodeAddresses []*corev1.NodeAddress
	globalHostNodeName  string
	globalMutex         sync.RWMutex
)

func (r *ReconcileEdgeDevice) getCurrentNodeAddresses() (hostNodeName string, addresses []corev1.NodeAddress, err error) {
	hostNodeName, addresses = func() (string, []corev1.NodeAddress) {
		globalMutex.RLock()
		defer globalMutex.RUnlock()

		if len(globalNodeAddresses) == 0 {
			return "", nil
		}

		result := make([]corev1.NodeAddress, len(globalNodeAddresses))
		for i, addr := range globalNodeAddresses {
			result[i] = *addr
		}
		return globalHostNodeName, result
	}()
	if addresses != nil {
		return hostNodeName, addresses, nil
	}

	globalMutex.Lock()
	defer globalMutex.Unlock()

	var thisPod *corev1.Pod
	thisPod, err = r.kubeClient.CoreV1().Pods(constant.AranyaNamespace()).Get(constant.ThisPodName(), metav1.GetOptions{})
	if err != nil {
		return "", nil, err
	}

	thisNode := &corev1.Node{}
	err = r.client.Get(r.ctx, types.NamespacedName{Name: thisPod.Spec.NodeName}, thisNode)
	if err != nil {
		return "", nil, err
	}

	globalHostNodeName = thisNode.Name
	for _, addr := range thisNode.Status.Addresses {
		globalNodeAddresses = append(globalNodeAddresses, addr.DeepCopy())
	}

	if len(globalNodeAddresses) == 0 {
		return "", nil, fmt.Errorf("failed to get node addresses")
	}

	result := make([]corev1.NodeAddress, len(globalNodeAddresses))
	for i, addr := range globalNodeAddresses {
		result[i] = *addr
	}

	return globalHostNodeName, result, nil
}

func (r *ReconcileEdgeDevice) cleanupVirtualNode(reqLog logr.Logger, namespace, name string) (err error) {
	virtualnode.Delete(name)

	needToDeleteNodeObj := true
	nodeObj := &corev1.Node{}
	err = r.client.Get(r.ctx, types.NamespacedName{Namespace: corev1.NamespaceAll, Name: name}, nodeObj)
	if err != nil {
		if errors.IsNotFound(err) {
			needToDeleteNodeObj = false
		} else {
			reqLog.Error(err, "failed to get node object")
			return err
		}
	}

	if needToDeleteNodeObj {
		err = r.client.Delete(r.ctx, nodeObj)
		if err != nil {
			reqLog.Error(err, "failed to delete node object")
			return err
		}
	}

	needToDeleteSvcObj := true
	svcObj := &corev1.Service{}
	err = r.client.Get(r.ctx, types.NamespacedName{Namespace: namespace, Name: name}, svcObj)
	if err != nil {
		if errors.IsNotFound(err) {
			needToDeleteSvcObj = false
		} else {
			reqLog.Error(err, "failed to get svc object")
			return err
		}
	}

	if needToDeleteSvcObj {
		err = r.client.Delete(r.ctx, svcObj)
		if err != nil {
			reqLog.Error(err, "failed to delete svc object")
			return err
		}
	}

	return nil
}
