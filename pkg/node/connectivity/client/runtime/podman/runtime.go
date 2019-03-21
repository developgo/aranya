// +build linux,podman

package podman

import (
	"context"
	"fmt"
	"io"
	"time"

	libpodRuntime "github.com/containers/libpod/libpod"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/remotecommand"
	criRuntime "k8s.io/kubernetes/pkg/kubelet/apis/cri/runtime/v1alpha2"

	"arhat.dev/aranya/pkg/node/connectivity"
	"arhat.dev/aranya/pkg/node/connectivity/client/runtime"
	"arhat.dev/aranya/pkg/node/connectivity/client/runtimeutil"
)

var _ runtime.Interface = &podmanRuntime{}

type podmanRuntime struct {
	ctx    context.Context
	config *runtime.Config

	runtimeActionTimeout time.Duration
	imageActionTimeout   time.Duration
}

func NewPodmanRuntime(ctx context.Context, config *runtime.Config) (runtime.Interface, error) {
	if err := config.Init(); err != nil {
		return nil, err
	}

	return &podmanRuntime{
		ctx:    ctx,
		config: config,

		runtimeActionTimeout: 2 * time.Minute,
		imageActionTimeout:   4 * time.Minute,
	}, nil
}

func (r *podmanRuntime) newRuntime(namespace string) (*libpodRuntime.Runtime, error) {
	return libpodRuntime.NewRuntime(
		libpodRuntime.WithNamespace(namespace),
		// set `pause` image in start command
		libpodRuntime.WithDefaultInfraImage(r.config.PauseImage),
		libpodRuntime.WithDefaultInfraCommand(r.config.PauseCommand),
		// set default proto to pull image
		libpodRuntime.WithDefaultTransport(libpodRuntime.DefaultTransport),
	)
}

func (r *podmanRuntime) CreatePod(namespace, name, uid string, podSpec *corev1.PodSpec, authConfig map[string]*criRuntime.AuthConfig, volumeData map[string][]byte) (*connectivity.Pod, error) {
	ctx, cancelCtx := context.WithTimeout(r.ctx, r.runtimeActionTimeout)
	defer cancelCtx()

	rt, err := r.newRuntime(namespace)
	if err != nil {
		return nil, err
	}

	if rt.ImageRuntime() == nil {
		// should not happen
		return nil, libpodRuntime.ErrRuntimeFinalized
	}

	// ensure image exists per container spec and apply image pull policy
	imageMap, err := ensureImages(rt.ImageRuntime(), podSpec, authConfig)
	if err != nil {
		return nil, err
	}

	// create pod
	podCreateOptions := defaultPodCreateOptions(namespace, name, podSpec)
	podmanPod, err := rt.NewPod(ctx, podCreateOptions...)
	if err != nil {
		return nil, err
	}

	// check `pause` container
	infraCtrID, err := podmanPod.InfraContainerID()
	if err != nil {
		return nil, err
	}

	namespaces := map[string]string{
		"net":  fmt.Sprintf("container:%s", infraCtrID),
		"user": fmt.Sprintf("container:%s", infraCtrID),
		"ipc":  fmt.Sprintf("container:%s", infraCtrID),
		"uts":  fmt.Sprintf("container:%s", infraCtrID),
	}

	// create containers with shared namespaces
	var containers []*libpodRuntime.Container
	for _, apiCtr := range podSpec.Containers {
		createConfig := kubeContainerToCreateConfig(podSpec, &apiCtr, rt, imageMap[apiCtr.Image], namespaces)
		if err != nil {
			return nil, err
		}

		ctr, err := createContainerFromCreateConfig(rt, createConfig, ctx, podmanPod)
		if err != nil {
			return nil, err
		}

		containers = append(containers, ctr)
	}

	// start the containers
	for _, ctr := range containers {
		if err := ctr.Start(ctx, true); err != nil {
			// Making this a hard failure here to avoid a mess
			// the other containers are in created status
			return nil, err
		}
	}

	podStatus, containerStatuses, err := translateLibpodStatusToCriStatus(rt, namespace, name, uid, podmanPod, infraCtrID)
	if err != nil {
		return nil, err
	}

	return connectivity.NewPod(namespace, name, uid, podStatus, containerStatuses), nil
}

func (r *podmanRuntime) DeletePod(namespace, name string, options *connectivity.DeleteOptions) (*connectivity.Pod, error) {
	rt, err := r.newRuntime(namespace)
	if err != nil {
		return nil, err
	}

	pod, err := rt.LookupPod(name)
	if err != nil {
		return nil, err
	}

	timeout := time.Duration(options.GetGraceTime())
	ctx, cancel := context.WithTimeout(r.ctx, timeout)
	defer cancel()

	errMap, err := pod.StopWithTimeout(ctx, true, int(timeout.Seconds()))
	if err != nil {
		return nil, err
	}

	// TODO: check errMap
	_ = errMap

	return nil, nil
}

func (r *podmanRuntime) ListPod(namespace string) ([]*connectivity.Pod, error) {
	rt, err := r.newRuntime(namespace)
	if err != nil {
		return nil, err
	}

	pods, err := rt.Pods()
	var allPodStatus []*connectivity.Pod
	for _, p := range pods {
		infraID, err := p.InfraContainerID()
		if err != nil {
			return nil, err
		}

		podStatus, containerStatuses, err := translateLibpodStatusToCriStatus(rt, namespace, p.Name(), "", p, infraID)
		if err != nil {
			return nil, err
		}

		allPodStatus = append(allPodStatus, connectivity.NewPod(namespace, p.Name(), "", podStatus, containerStatuses))
	}

	return allPodStatus, nil
}

func (r *podmanRuntime) AttachContainer(namespace, name, container string, stdin io.Reader, stdout, stderr io.WriteCloser, resizeCh <-chan remotecommand.TerminalSize) error {
	rt, err := r.newRuntime(namespace)
	if err != nil {
		return err
	}

	target, err := findContainer(rt, name, container)
	if err != nil {
		return err
	}

	// TODO: use more proper detach key
	detachKeys := ""
	return target.Attach(newStreamOptions(stdin, stdout, stderr), detachKeys, resizeCh)
}

func (r *podmanRuntime) ExecInContainer(namespace, name, container string, stdin io.Reader, stdout, stderr io.WriteCloser, resizeCh <-chan remotecommand.TerminalSize, command []string, tty bool) error {
	rt, err := r.newRuntime(namespace)
	if err != nil {
		return err
	}

	target, err := findContainer(rt, name, container)
	if err != nil {
		return err
	}

	return target.Exec(tty, false, nil, command, "", "", newStreamOptions(stdin, stdout, stderr))
}

func (r *podmanRuntime) GetContainerLogs(namespace, name string, stdout, stderr io.WriteCloser, options *corev1.PodLogOptions) error {
	defer func() { _, _ = stdout.Close(), stderr.Close() }()

	rt, err := r.newRuntime(namespace)
	if err != nil {
		return err
	}

	target, err := findContainer(rt, name, options.Container)
	if err != nil {
		return err
	}

	return runtimeutil.ReadLogs(context.Background(), target.LogPath(), options, stdout, stderr)
}

func (r *podmanRuntime) PodPortForward(namespace, name string, in io.Reader, out io.WriteCloser) error {
	rt, err := r.newRuntime(namespace)
	if err != nil {
		return err
	}

	pod, err := rt.LookupPod(name)
	if err != nil {
		return err
	}

	infraID, err := pod.InfraContainerID()
	if err != nil {
		return err
	}

	infraCtr, err := rt.GetContainer(infraID)
	if err != nil {
		return err
	}

	_ = infraCtr
	return nil
}
