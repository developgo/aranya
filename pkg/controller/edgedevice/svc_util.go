package edgedevice

import (
	"fmt"
	"net"
	"strconv"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	aranyav1alpha1 "arhat.dev/aranya/pkg/apis/aranya/v1alpha1"
	"arhat.dev/aranya/pkg/constant"
)

func (r *ReconcileEdgeDevice) createSvcForGrpc(device *aranyav1alpha1.EdgeDevice) (svcObject *corev1.Service, l net.Listener, err error) {
	grpcListenPort := getFreePort()
	if grpcListenPort < 1 {
		return
	}

	// claim this address immediately
	l, err = net.Listen("tcp", fmt.Sprintf(":%s", strconv.FormatInt(int64(grpcListenPort), 10)))
	if err != nil {
		return
	}
	defer func() {
		if err != nil {
			_ = l.Close()
			l = nil
		}
	}()

	svcObject = newServiceForEdgeDevice(device, grpcListenPort)
	err = controllerutil.SetControllerReference(device, svcObject, r.scheme)
	if err != nil {
		log.Error(err, "set svc controller reference failed")
		return
	}

	_, err = controllerutil.CreateOrUpdate(r.ctx, r.client, svcObject, func(existing runtime.Object) error {
		return nil
	})
	if err != nil {
		return
	}

	return
}

func newServiceForEdgeDevice(device *aranyav1alpha1.EdgeDevice, grpcListenPort int32) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:        device.Name,
			Namespace:   device.Namespace,
			Labels:      map[string]string{constant.LabelRole: constant.LabelRoleValueService},
			ClusterName: device.ClusterName,
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{constant.LabelRole: constant.LabelRoleValueController},

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
			Type:      corev1.ServiceTypeClusterIP,
			ClusterIP: "",
		},
	}
}
