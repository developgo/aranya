package containerd

import (
	"arhat.dev/aranya/pkg/node/connectivity/client/runtime"
	"context"
	"errors"
	"time"

	"k8s.io/client-go/util/flowcontrol"
	criRuntime "k8s.io/kubernetes/pkg/kubelet/apis/cri/runtime/v1alpha2"
	kubeletContainer "k8s.io/kubernetes/pkg/kubelet/container"
)

const (
	KubeRuntimeAPIVersion = "0.1.0"
)

var (
	// ErrVersionNotSupported is returned when the api version of runtime interface is not supported
	ErrVersionNotSupported = errors.New("Runtime api version is not supported")
)

func NewRuntime(ctx context.Context, config *runtime.Config) (*Runtime, error) {
	runtimeScvConn, err := dialSvcEndpoint(config.RuntimeSvcEndpoint, config.EndpointDialTimeout)
	if err != nil {
		return nil, err
	}

	imageSvcConn, err := dialSvcEndpoint(config.ImageSvcEndpoint, config.EndpointDialTimeout)
	if err != nil {
		return nil, err
	}

	runtime := &Runtime{
		ctx:         ctx,
		runtimeName: "default",

		imageActionTimeout:   time.Minute * 2,
		runtimeActionTimeout: time.Minute * 2,

		runtimeSvcClient: criRuntime.NewRuntimeServiceClient(runtimeScvConn),
		imageSvcClient:   criRuntime.NewImageServiceClient(imageSvcConn),

		containerRefManager: kubeletContainer.NewRefManager(),
	}

	typedVersion, err := runtime.remoteVersion(KubeRuntimeAPIVersion)
	if err != nil {
		return nil, err
	}
	if typedVersion.RuntimeApiVersion != KubeRuntimeAPIVersion {
		return nil, ErrVersionNotSupported
	}
	runtime.runtimeName = typedVersion.RuntimeName
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
