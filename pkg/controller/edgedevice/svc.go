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
	"net"

	"github.com/phayes/freeport"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	aranya "arhat.dev/aranya/pkg/apis/aranya/v1alpha1"
	"arhat.dev/aranya/pkg/constant"
)

func (r *ReconcileEdgeDevice) createGRPCSvcObjectForDevice(device *aranya.EdgeDevice, oldSvcObj *corev1.Service) (svcObject *corev1.Service, l net.Listener, err error) {
	grpcListenPort, err := freeport.GetFreePort()
	if err != nil {
		return nil, nil, err
	}

	// claim this address immediately
	l, err = net.Listen("tcp", GetListenAllAddress(int32(grpcListenPort)))
	if err != nil {
		return nil, nil, err
	}
	defer func() {
		if err != nil {
			_ = l.Close()
		}
	}()

	if oldSvcObj != nil {
		err = r.client.Delete(r.ctx, oldSvcObj, client.GracePeriodSeconds(0))
		if err != nil {
			return nil, nil, err
		}
	}

	svcObject = newServiceForEdgeDevice(device, int32(grpcListenPort))
	err = controllerutil.SetControllerReference(device, svcObject, r.scheme)
	if err != nil {
		return nil, nil, err
	}

	err = r.client.Create(r.ctx, svcObject)
	if err != nil {
		return nil, nil, err
	}

	return svcObject, l, nil
}

func newServiceForEdgeDevice(device *aranya.EdgeDevice, grpcListenPort int32) *corev1.Service {
	labels := make(map[string]string)
	for k, v := range device.Labels {
		labels[k] = v
	}
	labels[constant.LabelRole] = constant.LabelRoleValueService

	selector := map[string]string{
		constant.LabelRole: constant.LabelRoleValueController,
	}

	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:        device.Name,
			Namespace:   device.Namespace,
			Labels:      labels,
			ClusterName: device.ClusterName,
		},
		Spec: corev1.ServiceSpec{
			Selector: selector,
			// setup port for grpc server served by virtual node,
			// with mqtt we don't need to expose service
			Ports: []corev1.ServicePort{{
				Name:     "grpc",
				Protocol: corev1.ProtocolTCP,
				Port:     8080,
				TargetPort: intstr.IntOrString{
					Type:   intstr.Int,
					IntVal: grpcListenPort,
				},
			}},
			Type: corev1.ServiceTypeClusterIP,
			// do not allocate cluster ip since it's target is aranya's host node,
			// and only one grpc backend
			ClusterIP: corev1.ClusterIPNone,
		},
	}
}
