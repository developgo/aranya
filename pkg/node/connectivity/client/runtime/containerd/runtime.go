package containerd

import (
	"context"
	"fmt"
	"os"
	"time"

	corev1 "k8s.io/api/core/v1"
	utilRuntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/util/flowcontrol"
	"k8s.io/klog"
	criRuntime "k8s.io/kubernetes/pkg/kubelet/apis/cri/runtime/v1alpha2"
	kubeletContainer "k8s.io/kubernetes/pkg/kubelet/container"
	"k8s.io/kubernetes/pkg/kubelet/images"
	"k8s.io/kubernetes/pkg/kubelet/kuberuntime"
	"k8s.io/kubernetes/pkg/kubelet/util/format"

	"arhat.dev/aranya/pkg/node/connectivity"
)

const (
	// MaxContainerBackOff is the max backoff period, exported for the e2e test
	MaxContainerBackOff = 300 * time.Second
	// BackOffPeriod is the period to back off when pod syncing results in an
	// error. It is also used as the base period for the exponential backoff
	// container restarts and image pulls.
	BackOffPeriod = time.Second * 10
	// The api version of kubelet runtime api
	KubeRuntimeAPIVersion = "0.1.0"
)

func NewContainerdRuntime(ctx context.Context, containerdEndpoint string, dialTimeout time.Duration) (*Runtime, error) {
	runtimeScvConn, err := dialSvcEndpoint(containerdEndpoint, dialTimeout)
	if err != nil {
		return nil, err
	}

	imageSvcConn, err := dialSvcEndpoint(containerdEndpoint, dialTimeout)
	if err != nil {
		return nil, err
	}

	runtime := &Runtime{
		ctx:         ctx,
		runtimeName: "default",

		imageActionTimeout:   time.Minute * 2,
		runtimeActionTimeout: time.Minute * 2,

		imageActionBackOff:   flowcontrol.NewBackOff(BackOffPeriod, MaxContainerBackOff),
		runtimeActionBackOff: flowcontrol.NewBackOff(BackOffPeriod, MaxContainerBackOff),

		runtimeSvcClient: criRuntime.NewRuntimeServiceClient(runtimeScvConn),
		imageSvcClient:   criRuntime.NewImageServiceClient(imageSvcConn),

		containerRefManager: kubeletContainer.NewRefManager(),
	}

	typedVersion, err := runtime.remoteVersion(KubeRuntimeAPIVersion)
	if err != nil {
		return nil, err
	}
	if typedVersion.RuntimeApiVersion != KubeRuntimeAPIVersion {
		return nil, kuberuntime.ErrVersionNotSupported
	}
	runtime.runtimeName = typedVersion.RuntimeName

	// ensure pod log dir exists
	if _, err := os.Stat(podLogsRootDirectory); os.IsNotExist(err) {
		if err := os.MkdirAll(podLogsRootDirectory, 0755); err != nil {
			// TODO: log error
		}
	}

	return runtime, nil
}

type Runtime struct {
	ctx         context.Context
	runtimeName string

	imageActionTimeout   time.Duration
	runtimeActionTimeout time.Duration

	imageActionBackOff   *flowcontrol.Backoff
	runtimeActionBackOff *flowcontrol.Backoff

	runtimeSvcClient criRuntime.RuntimeServiceClient
	imageSvcClient   criRuntime.ImageServiceClient

	containerRefManager *kubeletContainer.RefManager
}

func (r *Runtime) CreatePod(sid uint64, namespace, name string, options *connectivity.CreateOptions) error {
	switch opt := options.GetPod().(type) {
	case *connectivity.CreateOptions_PodV1_:
		var (
			err error

			// cri related
			podSandboxID     string
			podSandboxStatus *criRuntime.PodSandboxStatus
			podSandboxConfig *criRuntime.PodSandboxConfig

			// kubelet related
			podStatus   *kubeletContainer.PodStatus
			pullSecrets []corev1.Secret
		)

		pod := &corev1.Pod{}
		err = pod.Unmarshal(opt.PodV1.GetPod())
		if err != nil {
			return err
		}

		for _, secretBytes := range opt.PodV1.GetPullSecret() {
			secret := &corev1.Secret{}
			err = secret.Unmarshal(secretBytes)
			if err != nil {
				return err
			}
			pullSecrets = append(pullSecrets, *secret)
		}

		podStatus, err = r.GetPodStatus(pod.UID, pod.Name, pod.Namespace)
		if err != nil {
			return err
		}

		// if this is a update command, check if pod exists,
		// aranya will only send pod update when there is
		// a change that requires pod restart
		podActions := computePodActions(pod, podStatus)

		if initContainer := podActions.NextInitContainerToStart; initContainer != nil {

		}

		podIP := ""
		if podStatus != nil {
			podIP = podStatus.IP
		}

		podSandboxConfig, err = translatePodV1ToCRIPodConfig(pod, podActions.Attempt)

		podSandboxID = podActions.SandboxID
		if podActions.CreateSandbox {
			podSandboxID, err = r.createPodSandbox(pod, podActions.Attempt)
			if err != nil {
				return err
			}

			podSandboxStatus, err = r.remotePodSandboxStatus(podSandboxID)
			if err != nil {
				return err
			}
			_ = podSandboxStatus
		}

		for _, idx := range podActions.ContainersToStart {
			container := &pod.Spec.Containers[idx]
			isInBackOff, err := r.doBackOff(pod, container, podStatus, r.runtimeActionBackOff)
			_ = err
			if isInBackOff {
				continue
			}

			if msg, err := r.startContainer(podSandboxID, podSandboxConfig, container, pod, podStatus, pullSecrets, podIP, kubeletContainer.ContainerTypeRegular); err != nil {
				switch err {
				case images.ErrImagePullBackOff:
				default:
					utilRuntime.HandleError(fmt.Errorf("container start failed: %v: %s", err, msg))
				}

				continue
			}
		}
	}

	return nil
}

// If a container is still in backoff, the function will return a brief backoff error and
// a detailed error message.
func (r *Runtime) doBackOff(pod *corev1.Pod, container *corev1.Container, podStatus *kubeletContainer.PodStatus, backOff *flowcontrol.Backoff) (bool, error) {
	var cStatus *kubeletContainer.ContainerStatus
	for _, c := range podStatus.ContainerStatuses {
		if c.Name == container.Name && c.State == kubeletContainer.ContainerStateExited {
			cStatus = c
			break
		}
	}

	if cStatus == nil {
		return false, nil
	}

	// Use the finished time of the latest exited container as the start point to calculate whether to do back-off.
	ts := cStatus.FinishedAt
	// backOff requires a unique key to identify the container.
	key := getStableKey(pod, container)
	if backOff.IsInBackOffSince(key, ts) {
		// if ref, err := kubeletContainer.GenerateContainerRef(pod, container); err == nil {
		// 	// m.recorder.Eventf(ref, v1.EventTypeWarning, events.BackOffStartContainer, "Back-off restarting failed container")
		// }
		err := fmt.Errorf("Back-off %s restarting failed container=%s pod=%s ", backOff.Get(key), container.Name, format.Pod(pod))
		klog.V(3).Infof("%s", err.Error())
		return true, kubeletContainer.ErrCrashLoopBackOff
	}

	backOff.Next(key, ts)
	return false, nil
}

func (r *Runtime) DeletePod(sid uint64, namespace, name string, options *connectivity.DeleteOptions) error {
	deletionCtx, cancel := context.WithTimeout(r.ctx, time.Duration(options.GetGraceTime()))
	_ = cancel

	_, err := r.runtimeSvcClient.RemovePodSandbox(deletionCtx, &criRuntime.RemovePodSandboxRequest{PodSandboxId: ""})
	if err != nil {
		return err
	}

	return nil
}
