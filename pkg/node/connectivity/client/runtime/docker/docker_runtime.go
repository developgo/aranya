// +build rt_docker

package docker

import (
	"context"
	"io"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/remotecommand"
	criRuntime "k8s.io/kubernetes/pkg/kubelet/apis/cri/runtime/v1alpha2"

	"arhat.dev/aranya/pkg/node/connectivity"
	"arhat.dev/aranya/pkg/node/connectivity/client/runtime"
	"arhat.dev/aranya/pkg/node/connectivity/client/runtimeutil"
)

type dockerRuntime struct {
	ctx    context.Context
	config *runtime.Config

	remoteImageService   *runtimeutil.RemoteImageService
	remoteRuntimeService *runtimeutil.RemoteRuntimeService

	runtimeActionTimeout time.Duration
	imageActionTimeout   time.Duration
}

func NewRuntime(ctx context.Context, config *runtime.Config) (runtime.Interface, error) {
	if err := config.Init(); err != nil {
		return nil, err
	}

	imageEndpoint := config.EndPoints.Image
	imageSvc, err := runtimeutil.NewRemoteImageService(imageEndpoint.Address, imageEndpoint.DialTimeout)
	if err != nil {
		return nil, err
	}

	runtimeEndpoint := config.EndPoints.Runtime
	runtimeSvc, err := runtimeutil.NewRemoteRuntimeService(runtimeEndpoint.Address, runtimeEndpoint.DialTimeout)
	if err != nil {
		return nil, err
	}

	return &dockerRuntime{
		ctx:    ctx,
		config: config,

		remoteImageService:   imageSvc,
		remoteRuntimeService: runtimeSvc,

		runtimeActionTimeout: config.EndPoints.Runtime.ActionTimeout,
		imageActionTimeout:   config.EndPoints.Image.ActionTimeout,
	}, nil
}

func (r *dockerRuntime) CreatePod(
	namespace, name string,
	containers map[string]*connectivity.ContainerSpec,
	authConfig map[string]*criRuntime.AuthConfig,
	volumeData map[string]*connectivity.NamedData,
	hostVolumes map[string]string,
) (*connectivity.Pod, error) {
	return nil, nil
}

func (r *dockerRuntime) DeletePod(namespace, name string, options *connectivity.DeleteOptions) (*connectivity.Pod, error) {
	return nil, nil
}

func (r *dockerRuntime) ListPod(namespace, name string) ([]*connectivity.Pod, error) {
	return nil, nil
}

func (r *dockerRuntime) ExecInContainer(namespace, name, container string, stdin io.Reader, stdout, stderr io.WriteCloser, resizeCh <-chan remotecommand.TerminalSize, command []string, tty bool) error {
	return nil
}

func (r *dockerRuntime) AttachContainer(namespace, name, container string, stdin io.Reader, stdout, stderr io.WriteCloser, resizeCh <-chan remotecommand.TerminalSize) error {
	return nil
}

func (r *dockerRuntime) GetContainerLogs(namespace, name string, stdout, stderr io.WriteCloser, options *corev1.PodLogOptions) error {
	return nil
}

func (r *dockerRuntime) PortForward(namespace, name string, ports []int32, in io.Reader, out io.WriteCloser) error {
	return nil
}
