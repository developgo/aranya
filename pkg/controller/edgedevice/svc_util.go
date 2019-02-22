package edgedevice

import (
	"arhat.dev/aranya/pkg/constant"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	aranyav1alpha1 "arhat.dev/aranya/pkg/apis/aranya/v1alpha1"
	"arhat.dev/aranya/pkg/node/util"
)

func newServiceForEdgeDevice(device *aranyav1alpha1.EdgeDevice, grpcListenPort int32) *corev1.Service {
	svcName := util.GetServiceName(device.Name)

	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:            svcName,
			Namespace:       device.Namespace,
			OwnerReferences: ownerRefFromEdgeDevice(device),
			Labels:          map[string]string{constant.LabelType: constant.LabelTypeValueService},
			ClusterName:     device.ClusterName,
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{constant.LabelType: constant.LabelTypeValueController},

			// setup port for grpc server served by virtual node,
			// with mqtt we don't need to expose a port
			Ports: []corev1.ServicePort{{
				Name:     "grpc",
				Protocol: corev1.ProtocolTCP,
				Port:     8080,
				TargetPort: intstr.IntOrString{
					Type:   intstr.Int,
					IntVal: grpcListenPort,
				},
			}},
			ClusterIP: "",
			Type:      corev1.ServiceTypeClusterIP,
		},
	}
}
