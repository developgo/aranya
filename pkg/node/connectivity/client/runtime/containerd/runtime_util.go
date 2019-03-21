package containerd

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"time"

	"k8s.io/klog"

	"google.golang.org/grpc"
	corev1 "k8s.io/api/core/v1"
	criRuntime "k8s.io/kubernetes/pkg/kubelet/apis/cri/runtime/v1alpha2"
	kubeletContainer "k8s.io/kubernetes/pkg/kubelet/container"
	kubeletUtil "k8s.io/kubernetes/pkg/kubelet/util"
)

func dialSvcEndpoint(endpoint string, dialTimeout time.Duration) (*grpc.ClientConn, error) {
	addr, dialer, err := kubeletUtil.GetAddressAndDialer(endpoint)
	if err != nil {
		return nil, err
	}

	dialCtx, cancel := context.WithTimeout(context.Background(), dialTimeout)
	_ = cancel

	conn, err := grpc.DialContext(dialCtx, addr, grpc.WithInsecure(), grpc.WithBlock(), grpc.WithDialer(dialer))
	if err != nil {
		return nil, err
	}

	return conn, nil
}

// getStableKey generates a key (string) to uniquely identify a
// (pod, container) tuple. The key should include the content of the
// container, so that any change to the container generates a new key.
func getStableKey(pod *corev1.Pod, container *corev1.Container) string {
	hash := strconv.FormatUint(kubeletContainer.HashContainer(container), 16)
	return fmt.Sprintf("%s_%s_%s_%s_%s", pod.Name, pod.Namespace, string(pod.UID), container.Name, hash)
}

// determinePodSandboxIP determines the IP address of the given pod sandbox.
func determinePodSandboxIP(podSandbox *criRuntime.PodSandboxStatus) string {
	if podSandbox.Network == nil {
		klog.Warningf("Pod Sandbox status doesn't have network information, cannot report IP")
		return ""
	}
	ip := podSandbox.Network.Ip
	if len(ip) != 0 && net.ParseIP(ip) == nil {
		// ip could be an empty string if runtime is not responsible for the
		// IP (e.g., host networking).
		klog.Warningf("Pod Sandbox reported an unparseable IP %v", ip)
		return ""
	}
	return ip
}
