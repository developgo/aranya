package edgedevice

import (
	"fmt"
	"net"
	"strconv"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	aranyav1alpha1 "arhat.dev/aranya/pkg/apis/aranya/v1alpha1"
	"arhat.dev/aranya/pkg/constant"
	"arhat.dev/aranya/pkg/node/util"
)

func (r *ReconcileEdgeDevice) createGrpcSvcIfUsed(device *aranyav1alpha1.EdgeDevice) (svcObject *corev1.Service, l net.Listener, err error) {
	switch device.Spec.Connectivity.Method {
	case aranyav1alpha1.DeviceConnectViaGRPC:
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
			}
		}()

		svcObject = newServiceForEdgeDevice(device, grpcListenPort)
		err = controllerutil.SetControllerReference(device, svcObject, r.scheme)
		if err != nil {
			log.Error(err, "set svc controller reference failed")
			return
		}

		err = r.client.Create(r.ctx, svcObject)
		if err != nil {
			return
		}

		return
	}

	return
}

func newServiceForEdgeDevice(device *aranyav1alpha1.EdgeDevice, grpcListenPort int32) *corev1.Service {
	svcName := util.GetServiceName(device.Name)

	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:            svcName,
			Namespace:       device.Namespace,
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