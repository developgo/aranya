package edgedevice

import (
	"fmt"
	"net"
	"strconv"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	aranya "arhat.dev/aranya/pkg/apis/aranya/v1alpha1"
	"arhat.dev/aranya/pkg/constant"
)

func (r *ReconcileEdgeDevice) newListener() (l net.Listener, err error) {
	var (
		hostIP string
		port   int32
	)

	// get node ip address
	hostIP, _, err = r.getHostAddress()
	if err != nil {
		return
	}

	// get free port on this node
	port = getFreePort()
	if port < 1 {
		return nil, errNoFreePort
	}

	// claim this address immediately
	return net.Listen("tcp", fmt.Sprintf("%s:%s", hostIP, strconv.FormatInt(int64(port), 10)))
}

func (r *ReconcileEdgeDevice) createNodeObject(device *aranya.EdgeDevice) (nodeObj *corev1.Node, l net.Listener, err error) {
	var (
		hostIP            string
		hostname          string
		kubeletListenPort int32
	)

	// get node ip address
	hostIP, hostname, err = r.getHostAddress()
	if err != nil {
		return
	}

	// get free port on this node
	kubeletListenPort = getFreePort()
	if kubeletListenPort < 1 {
		return nil, nil, errNoFreePort
	}

	// claim this address immediately
	l, err = net.Listen("tcp", fmt.Sprintf("%s:%s", hostIP, strconv.FormatInt(int64(kubeletListenPort), 10)))
	if err != nil {
		return
	}
	defer func() {
		if err != nil {
			_ = l.Close()
		}
	}()

	nodeObj = newNodeForEdgeDevice(device, hostIP, hostname, kubeletListenPort)
	err = controllerutil.SetControllerReference(device, nodeObj, r.scheme)
	if err != nil {
		return
	}

	// create the virtual node object
	_, err = controllerutil.CreateOrUpdate(r.ctx, r.client, nodeObj, func(existing runtime.Object) error { return nil })
	if err != nil {
		return
	}

	return
}

// create a node object in kubernetes, handle it in a dedicated arhat.dev/aranya/pkg/node.Node instance
func newNodeForEdgeDevice(device *aranya.EdgeDevice, hostIP string, hostname string, kubeletPort int32) *corev1.Node {
	return &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:      device.Name,
			Namespace: corev1.NamespaceAll,
			Labels: map[string]string{
				constant.LabelRole: constant.LabelRoleValueNode,
				constant.LabelName: device.Name,
				// TODO: use corev1.LabelHostname in future when controller-runtime updated
				"kubernetes.io/hostname": hostname,
			},
			ClusterName: device.ClusterName,
		},
		Spec: corev1.NodeSpec{
			Taints: []corev1.Taint{{
				Key:    constant.TaintKeyNamespace,
				Value:  constant.CurrentNamespace(),
				Effect: corev1.TaintEffectNoSchedule,
			}},
		},
		Status: corev1.NodeStatus{
			// fill address and port when actually create
			Addresses: []corev1.NodeAddress{{
				Type:    corev1.NodeInternalIP,
				Address: hostIP,
			}, {
				Type:    corev1.NodeHostName,
				Address: hostname,
			}},
			DaemonEndpoints: corev1.NodeDaemonEndpoints{KubeletEndpoint: corev1.DaemonEndpoint{Port: kubeletPort}},
			Phase:           corev1.NodePending,
		},
	}
}
