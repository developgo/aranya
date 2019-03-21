package containerd

import (
	"context"
	"errors"
	"fmt"

	criRuntime "k8s.io/kubernetes/pkg/kubelet/apis/cri/runtime/v1alpha2"
)

// Version returns the runtime name, runtime version and runtime API version.
func (r *Runtime) remoteVersion(apiVersion string) (*criRuntime.VersionResponse, error) {
	ctx, cancel := context.WithTimeout(r.ctx, r.runtimeActionTimeout)
	defer cancel()

	typedVersion, err := r.runtimeSvcClient.Version(ctx, &criRuntime.VersionRequest{
		Version: apiVersion,
	})

	if err != nil {
		return nil, err
	}

	if typedVersion.Version == "" || typedVersion.RuntimeName == "" || typedVersion.RuntimeApiVersion == "" || typedVersion.RuntimeVersion == "" {
		return nil, fmt.Errorf("not all fields are set in VersionResponse (%q)", *typedVersion)
	}

	return typedVersion, err
}

// remoteStatus returns the status of the runtime.
func (r *Runtime) remoteStatus() (*criRuntime.RuntimeStatus, error) {
	ctx, cancel := context.WithTimeout(r.ctx, r.runtimeActionTimeout)
	defer cancel()

	resp, err := r.runtimeSvcClient.Status(ctx, &criRuntime.StatusRequest{})
	if err != nil {
		return nil, err
	}

	if resp.Status == nil || len(resp.Status.Conditions) < 2 {
		errorMessage := "RuntimeReady or NetworkReady condition are not set"
		return nil, errors.New(errorMessage)
	}

	return resp.Status, nil
}
