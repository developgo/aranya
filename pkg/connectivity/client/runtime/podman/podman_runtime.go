// +build linux,rt_podman

/*
Copyright 2019 The arhat.dev Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package podman

import (
	"context"
	"errors"
	"fmt"
	"io"
	goruntime "runtime"
	"time"

	libpodRuntime "github.com/containers/libpod/libpod"
	podmanVersion "github.com/containers/libpod/version"
	"k8s.io/client-go/tools/remotecommand"

	"arhat.dev/aranya/pkg/connectivity"
	"arhat.dev/aranya/pkg/connectivity/client/runtime"
	"arhat.dev/aranya/pkg/connectivity/client/runtimeutil"
)

type podmanRuntime struct {
	runtime.Base
}

func NewRuntime(ctx context.Context, config *runtime.Config) (runtime.Interface, error) {
	if err := config.Init(); err != nil {
		return nil, err
	}

	runtimeClient, err := libpodRuntime.NewRuntime(
		libpodRuntime.WithNamespace(config.ManagementNamespace),
		// set `pause` image in start command
		libpodRuntime.WithDefaultInfraImage(config.PauseImage),
		libpodRuntime.WithDefaultInfraCommand(config.PauseCommand),
		// set default proto to pull image
		libpodRuntime.WithDefaultTransport(libpodRuntime.DefaultTransport),
	)
	if err != nil {
		return nil, err
	}

	imageClient := runtimeClient.ImageRuntime()
	if imageClient != nil {
		return nil, errors.New("empty image client")
	}

	return &podmanRuntime{
		Base: runtime.NewRuntimeBase(ctx, config, "podman", podmanVersion.Version, goruntime.GOOS, goruntime.GOARCH, ""),
	}, nil
}

func (r *podmanRuntime) newRuntime() (*libpodRuntime.Runtime, error) {
	return libpodRuntime.NewRuntime(
		libpodRuntime.WithNamespace(r.ManagementNamespace),
		// set `pause` image in start command
		libpodRuntime.WithDefaultInfraImage(r.PauseImage),
		libpodRuntime.WithDefaultInfraCommand(r.PauseCommand),
		// set default proto to pull image
		libpodRuntime.WithDefaultTransport(libpodRuntime.DefaultTransport),
	)
}

func (r *podmanRuntime) CreatePod(options *connectivity.CreateOptions) (*connectivity.PodStatus, error) {
	ctx, cancelCtx := r.RuntimeActionContext()
	defer cancelCtx()

	rt, err := r.newRuntime()
	if err != nil {
		return nil, err
	}

	if rt.ImageRuntime() == nil {
		// should not happen
		return nil, libpodRuntime.ErrRuntimeFinalized
	}

	// ensure image exists per container spec and apply image pull policy
	imageMap, err := ensureImages(rt.ImageRuntime(), options.GetContainers(), options.GetImagePullAuthConfig())
	if err != nil {
		return nil, err
	}

	// create pod
	podmanPod, err := rt.NewPod(ctx, defaultPodCreateOptions(options.GetPodUid(), options.GetContainers())...)
	if err != nil {
		return nil, err
	}

	// check `pause` container
	infraCtrID, err := podmanPod.InfraContainerID()
	if err != nil {
		return nil, err
	}

	// share infrastructure container namespaces
	namespaces := map[string]string{
		"net":  fmt.Sprintf("container:%s", infraCtrID),
		"user": fmt.Sprintf("container:%s", infraCtrID),
		"ipc":  fmt.Sprintf("container:%s", infraCtrID),
		"uts":  fmt.Sprintf("container:%s", infraCtrID),
	}

	// create containers with shared namespaces
	var libpodContainers []*libpodRuntime.Container
	for containerName, containerSpec := range options.GetContainers() {
		createConfig, err := r.translateContainerSpecToPodmanCreateConfig(
			options.GetPodUid(), containerName, containerSpec,
			options.GetHostVolumes(), options.GetVolumeData(),
			rt, imageMap, namespaces)
		if err != nil {
			return nil, err
		}

		ctr, err := createContainerFromCreateConfig(rt, createConfig, ctx, podmanPod)
		if err != nil {
			return nil, err
		}

		libpodContainers = append(libpodContainers, ctr)
	}

	// start the containers
	for _, ctr := range libpodContainers {
		if err := ctr.Start(ctx, true); err != nil {
			// Making this a hard failure here to avoid a mess
			// the other containers are in created status
			return nil, err
		}
	}

	// podStatus, containerStatuses, err := translateLibpodStatusToCriStatus(rt, options.GetPodUid(), podmanPod, infraCtrID)
	// if err != nil {
	// 	return nil, err
	// }

	return connectivity.NewPodStatus(options.GetPodUid(), nil), nil
}

func (r *podmanRuntime) DeletePod(options *connectivity.DeleteOptions) (*connectivity.PodStatus, error) {
	rt, err := r.newRuntime()
	if err != nil {
		return nil, err
	}

	ctx, cancel := r.RuntimeActionContext()
	defer cancel()

	pod, err := rt.LookupPod(options.GetPodUid())
	if err != nil {
		return nil, err
	}

	timeout := time.Duration(options.GetGraceTime())

	errMap, err := pod.StopWithTimeout(ctx, true, int(timeout.Seconds()))
	if err != nil {
		return nil, err
	}

	// TODO: check errMap
	_ = errMap

	return nil, nil
}

func (r *podmanRuntime) ListPods(options *connectivity.ListOptions) ([]*connectivity.PodStatus, error) {
	// rt, err := r.newRuntime()
	// if err != nil {
	// 	return nil, err
	// }

	// pods, err := rt.Pods()
	var allPodStatus []*connectivity.PodStatus
	// for _, p := range pods {
	// 	infraID, err := p.InfraContainerID()
	// 	if err != nil {
	// 		return nil, err
	// 	}
	//
	// 	podStatus, containerStatuses, err := translateLibpodStatusToCriStatus(rt, p.Name(), p, infraID)
	// 	if err != nil {
	// 		return nil, err
	// 	}
	//
	// 	allPodStatus = append(allPodStatus, connectivity.NewPodStatus(p.Labels()[constant.ContainerLabelPodUID], podStatus, containerStatuses))
	// }

	return allPodStatus, nil
}

func (r *podmanRuntime) AttachContainer(podUID, container string, stdin io.Reader, stdout, stderr io.WriteCloser, resizeCh <-chan remotecommand.TerminalSize) error {
	rt, err := r.newRuntime()
	if err != nil {
		return err
	}

	target, err := findContainer(rt, podUID, container)
	if err != nil {
		return err
	}

	// TODO: use more proper detach key
	detachKeys := ""
	return target.Attach(newStreamOptions(stdin, stdout, stderr), detachKeys, resizeCh)
}

func (r *podmanRuntime) ExecInContainer(podUID, container string, stdin io.Reader, stdout, stderr io.WriteCloser, resizeCh <-chan remotecommand.TerminalSize, command []string, tty bool) error {
	rt, err := r.newRuntime()
	if err != nil {
		return err
	}

	target, err := findContainer(rt, podUID, container)
	if err != nil {
		return err
	}

	return target.Exec(tty, false, nil, command, "", "", newStreamOptions(stdin, stdout, stderr))
}

func (r *podmanRuntime) GetContainerLogs(podUID string, options *connectivity.LogOptions, stdout, stderr io.WriteCloser) error {
	defer func() { _, _ = stdout.Close(), stderr.Close() }()

	rt, err := r.newRuntime()
	if err != nil {
		return err
	}

	target, err := findContainer(rt, podUID, options.Container)
	if err != nil {
		return err
	}

	return runtimeutil.ReadLogs(context.Background(), target.LogPath(), nil, stdout, stderr)
}

func (r *podmanRuntime) PortForward(podUID string, protocol string, port int32, in io.Reader, out io.WriteCloser) error {
	rt, err := r.newRuntime()
	if err != nil {
		return err
	}

	pod, err := rt.LookupPod(podUID)
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
