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
	"crypto/tls"
	"net"

	"github.com/cloudflare/cfssl/csr"
	"github.com/phayes/freeport"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	kubeletapis "k8s.io/kubernetes/pkg/kubelet/apis"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	aranya "arhat.dev/aranya/pkg/apis/aranya/v1alpha1"
	"arhat.dev/aranya/pkg/constant"
)

func (r *ReconcileEdgeDevice) createNodeObjectForDevice(device *aranya.EdgeDevice) (nodeObj *corev1.Node, listener net.Listener, err error) {
	hostNodeName, addresses, err := r.getCurrentNodeAddresses()
	if err != nil {
		return nil, nil, err
	}

	kubeletListenPort, err := freeport.GetFreePort()
	if err != nil {
		return nil, nil, err
	}

	listener, err = newKubeletListener(r.kubeClient, hostNodeName, device, int32(kubeletListenPort), addresses)
	if err != nil {
		return nil, nil, err
	}

	defer func() {
		if err != nil {
			_ = listener.Close()
		}
	}()

	nodeObj = newNodeForEdgeDevice(device, addresses, int32(kubeletListenPort))
	err = controllerutil.SetControllerReference(device, nodeObj, r.scheme)
	if err != nil {
		return nil, nil, err
	}

	// create the node object
	_, err = controllerutil.CreateOrUpdate(r.ctx, r.client, nodeObj, func(existing runtime.Object) error { return nil })
	if err != nil {
		return nil, nil, err
	}

	return nodeObj, listener, nil
}

func newKubeletListener(kubeClient kubernetes.Interface, hostNodeName string, deviceObj *aranya.EdgeDevice, port int32, nodeAddresses []corev1.NodeAddress) (listener net.Listener, err error) {
	certInfo := deviceObj.Spec.CertInfo
	csrName := csr.Name{
		C:  certInfo.Country,
		ST: certInfo.State,
		L:  certInfo.Locality,
		O:  certInfo.Organisation,
		OU: certInfo.OrganisationUnit,
	}
	tlsCert, err := getKubeletServerCert(kubeClient, hostNodeName, deviceObj.Name, csrName, nodeAddresses)
	if err != nil {
		return nil, err
	}

	// claim this port immediately
	listener, err = net.Listen("tcp", GetListenAllAddress(port))
	if err != nil {
		return
	}

	return tls.NewListener(listener, &tls.Config{Certificates: []tls.Certificate{*tlsCert}}), nil
}

// create a node object in kubernetes, handle it in a dedicated arhat.dev/aranya/pkg/node.Node instance
func newNodeForEdgeDevice(device *aranya.EdgeDevice, addresses []corev1.NodeAddress, kubeletPort int32) *corev1.Node {
	hostname := ""
	for _, addr := range addresses {
		if addr.Type == corev1.NodeHostName {
			hostname = addr.Address
		}
	}

	labels := map[string]string{
		// this label can be overridden
		"kubernetes.io/role": "EdgeDevice",
	}

	for k, v := range device.Labels {
		labels[k] = v
	}

	labels[constant.LabelRole] = constant.LabelRoleValueEdgeDevice
	labels[constant.LabelName] = device.Name
	if hostname != "" {
		labels[kubeletapis.LabelHostname] = hostname
	}

	return &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:        device.Name,
			Annotations: device.Annotations,
			Labels:      labels,
			ClusterName: device.ClusterName,
		},
		Spec: corev1.NodeSpec{
			Taints: []corev1.Taint{{
				Key:    constant.TaintKeyNamespace,
				Value:  device.Namespace,
				Effect: corev1.TaintEffectNoSchedule,
			}},
		},
		Status: corev1.NodeStatus{
			// fill address and port when actually create
			Addresses:       addresses,
			DaemonEndpoints: corev1.NodeDaemonEndpoints{KubeletEndpoint: corev1.DaemonEndpoint{Port: kubeletPort}},
		},
	}
}
