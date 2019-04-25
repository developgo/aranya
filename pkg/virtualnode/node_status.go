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

package virtualnode

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubernetes/pkg/kubelet/util"

	"arhat.dev/aranya/pkg/constant"
)

func (vn *VirtualNode) syncNodeStatus() {
	vn.log.V(10).Info("trying to update node status")
	for i := 0; i < constant.DefaultNodeStatusUpdateRetry; i++ {
		if err := vn.tryUpdateNodeStatus(i); err != nil {
			vn.log.Error(err, "failed to update node status, retry")
		} else {
			vn.log.V(10).Info("update node status success")
			return
		}
	}
	vn.log.Info("update node status exceeds retry count")
}

func (vn *VirtualNode) tryUpdateNodeStatus(tryNumber int) error {
	opts := metav1.GetOptions{}
	if tryNumber == 0 {
		util.FromApiserverCache(&opts)
	}

	oldNode, err := vn.kubeNodeClient.Get(vn.name, opts)
	if err != nil {
		return fmt.Errorf("error getting node %q: %v", vn.name, err)
	}

	// Patch the current status on the API server
	newNode := oldNode.DeepCopy()
	newNode.Status = vn.nodeStatusCache.Get()

	updatedNode, err := vn.kubeNodeClient.UpdateStatus(newNode)
	if err != nil {
		return err
	}

	_ = updatedNode

	return nil
}
