package edgedevice

import (
	"crypto/tls"
	"net"

	aranya "arhat.dev/aranya/pkg/apis/aranya/v1alpha1"
	"arhat.dev/aranya/pkg/constant"
	"github.com/phayes/freeport"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	kubeletApis "k8s.io/kubernetes/pkg/kubelet/apis"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (r *ReconcileEdgeDevice) createNodeObjectForDevice(device *aranya.EdgeDevice) (nodeObj *corev1.Node, listener net.Listener, err error) {
	addresses, err := r.getCurrentNodeAddresses()
	if err != nil {
		return nil, nil, err
	}

	kubeletListenPort, err := freeport.GetFreePort()
	if err != nil {
		return nil, nil, err
	}

	listener, err = NewKubeletListener(r.kubeClient, NodeName(device.Name), int32(kubeletListenPort), addresses)
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

func NewKubeletListener(kubeClient kubernetes.Interface, nodeName string, port int32, nodeAddresses []corev1.NodeAddress) (listener net.Listener, err error) {
	tlsCert, err := GetKubeletServerCert(kubeClient, nodeName, nodeAddresses)
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

	return &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: NodeName(device.Name),
			Labels: map[string]string{
				constant.LabelRole:        constant.LabelRoleValueEdgeDevice,
				constant.LabelName:        device.Name,
				"kubernetes.io/role":      "EdgeDevice",
				kubeletApis.LabelHostname: hostname,
			},
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
