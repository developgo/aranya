// +build rt_containerd

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

package containerd

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	goruntime "runtime"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/cio"
	"github.com/containerd/containerd/oci"
	"github.com/containerd/containerd/remotes/docker"
	ociSpecs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/satori/go.uuid"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/remotecommand"

	"arhat.dev/aranya/pkg/connectivity"
	"arhat.dev/aranya/pkg/connectivity/client/runtime"
	"arhat.dev/aranya/pkg/connectivity/client/runtimeutil"
	"arhat.dev/aranya/pkg/constant"
)

func NewRuntime(ctx context.Context, config *runtime.Config) (runtime.Interface, error) {
	runtimeClient, err := containerd.New(config.EndPoints.Runtime.Address,
		containerd.WithDefaultNamespace(config.ManagementNamespace),
		containerd.WithTimeout(config.EndPoints.Runtime.DialTimeout),
	)

	versionQueryCtx, cancel := context.WithTimeout(ctx, config.EndPoints.Runtime.ActionTimeout)
	defer cancel()
	ver, err := runtimeClient.Version(versionQueryCtx)
	if err != nil {
		return nil, err
	}

	// reuse runtime client if same endpoint address provided (most of the time)
	imageClient := runtimeClient
	if config.EndPoints.Runtime.Address != config.EndPoints.Image.Address {
		imageClient, err = containerd.New(config.EndPoints.Image.Address,
			containerd.WithDefaultNamespace(config.ManagementNamespace),
			containerd.WithTimeout(config.EndPoints.Image.DialTimeout),
		)
	}

	return &containerdRuntime{
		Base:          runtime.NewRuntimeBase(ctx, config, "containerd", ver.Version, goruntime.GOOS, goruntime.GOARCH, ""),
		imageClient:   imageClient,
		runtimeClient: runtimeClient,
	}, nil
}

type containerdRuntime struct {
	runtime.Base

	imageClient, runtimeClient *containerd.Client
}

func (r *containerdRuntime) CreatePod(options *connectivity.CreateOptions) (pod *connectivity.PodStatus, err *connectivity.Error) {
	imagePullCtx, cancelPull := r.ImageActionContext()
	defer cancelPull()

	// ensure pause image (infra image to claim namespaces) exists
	var plainErr error
	pauseImageMap, plainErr := r.ensureImages(imagePullCtx, map[string]*connectivity.ContainerSpec{
		"pause": {
			Image:           r.PauseImage,
			ImagePullPolicy: string(corev1.PullIfNotPresent),
		},
	}, nil)
	if plainErr != nil {
		return nil, &connectivity.Error{Kind: connectivity.ErrCommon, Description: plainErr.Error()}
	}

	pauseImage := pauseImageMap["pause"]

	// ensure all images exists
	images, err := r.ensureImages(imagePullCtx, options.GetContainers(), options.GetImagePullAuthConfig())
	if err != nil {
		return nil, err
	}

	createCtx, cancelCreate := r.RuntimeActionContext()
	defer cancelCreate()
	// create infra container to claim namespaces
	infraSpecOpts := []oci.SpecOpts{
		oci.WithRootFSReadonly(),
		oci.WithImageConfig(pauseImage),
	}

	if r.PauseCommand != "" {
		infraSpecOpts = append(infraSpecOpts, oci.WithProcessArgs(r.PauseCommand))
	}
	if options.GetHostNetwork() {
		infraSpecOpts = append(infraSpecOpts, oci.WithHostNamespace(ociSpecs.NetworkNamespace))
	}
	if options.GetHostPid() {
		infraSpecOpts = append(infraSpecOpts, oci.WithHostNamespace(ociSpecs.PIDNamespace))
	}
	if options.GetHostIpc() {
		infraSpecOpts = append(infraSpecOpts, oci.WithHostNamespace(ociSpecs.IPCNamespace))
	}
	if options.GetHostname() != "" {
		infraSpecOpts = append(infraSpecOpts, oci.WithHostname(options.GetHostname()))
	}

	pauseContainerID := runtimeutil.GetContainerName(options.Namespace, options.Name, constant.ContainerNamePause)
	pauseContainer, err := r.runtimeClient.NewContainer(
		createCtx,
		pauseContainerID,
		containerd.WithImage(pauseImage),
		containerd.WithImageStopSignal(pauseImage, "SIGTERM"),
		containerd.WithNewSnapshot(pauseContainerID, pauseImage),
		containerd.WithNewSpec(infraSpecOpts...))
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			e := pauseContainer.Delete(context.Background(), containerd.WithSnapshotCleanup)
			if e != nil {
				r.Log().Error(e, "failed to delete pause container")
			}
		}
	}()

	spec, err := pauseContainer.Spec(createCtx)
	if err != nil {
		return nil, err
	}

	// common oci spec opts for all containers in pod
	commonOCISpecOpts := []oci.SpecOpts{
		oci.WithDefaultSpec(),
		oci.WithDefaultUnixDevices,
	}

	if spec.Linux != nil {
		nsCount := 0
		for _, ns := range spec.Linux.Namespaces {
			switch ns.Type {
			// share net, ipc, user, uts namespaces
			case ociSpecs.NetworkNamespace, ociSpecs.IPCNamespace, ociSpecs.UserNamespace, ociSpecs.UTSNamespace:
				commonOCISpecOpts = append(commonOCISpecOpts, oci.WithLinuxNamespace(ociSpecs.LinuxNamespace{
					Type: ns.Type,
					Path: ns.Path,
				}))
				nsCount++
			}
		}
		if nsCount != 4 {
			return nil, errors.New("required shared namespaces not fulfilled")
		}
	}

	// create containers
	for ctrName, container := range options.GetContainers() {
		image := images[container.GetImage()]

		containerID := runtimeutil.GetContainerName(options.Namespace, options.Name, ctrName)
		var specOpts []oci.SpecOpts

		if container.GetTty() {
			specOpts = append(specOpts, oci.WithTTY)
		}

		if container.GetPrivileged() {
			specOpts = append(specOpts, oci.WithPrivileged)
		}

		if container.GetAllowNewPrivileges() {
			specOpts = append(specOpts, oci.WithNewPrivileges)
		}

		if container.GetWorkingDir() != "" {
			specOpts = append(specOpts, oci.WithProcessCwd(container.GetWorkingDir()))
		}

		// TODO: expose port
		// for portName, port := range container.GetPorts() {
		// 	specOpts = append(specOpts)
		// }

		var envs []string
		for k, v := range container.GetEnvs() {
			envs = append(envs, k+"="+v)
		}
		specOpts = append(specOpts, oci.WithEnv(envs))

		var mounts []ociSpecs.Mount
		for volumeName, mountOpt := range container.GetVolumeMounts() {
			source := ""
			if hostPath, isHostVol := options.GetHostVolumes()[volumeName]; isHostVol {
				source = hostPath
			}

			if volData, isVolData := options.GetVolumeData()[volumeName]; isVolData && volData.GetDataMap() != nil {
				dataMap := volData.GetDataMap()

				dir := r.PodVolumeDir(options.GetPodUid(), "native", volumeName)
				if err = os.MkdirAll(dir, 0755); err != nil {
					return nil, err
				}
				source, err = mountOpt.Ensure(dir, dataMap)
				if err != nil {
					return nil, err
				}
			}

			mounts = append(mounts, ociSpecs.Mount{
				Source:      source,
				Destination: mountOpt.GetMountPath(),
				Type:        mountOpt.GetType(),
				Options:     mountOpt.GetOptions(),
			})
		}
		specOpts = append(specOpts, oci.WithMounts(mounts))
		specOpts = append(specOpts, commonOCISpecOpts...)

		ctr, err := r.runtimeClient.NewContainer(createCtx, containerID,
			containerd.WithContainerLabels(runtimeutil.ContainerLabels(options.GetNamespace(), options.GetName(), options.GetPodUid(), ctrName)),
			containerd.WithImage(image),
			containerd.WithImageStopSignal(image, "SIGTERM"),
			containerd.WithNewSnapshot(containerID, image),
			containerd.WithNewSpec(specOpts...))
		if err != nil {
			return nil, err
		}

		defer func() {
			if err != nil {
				e := ctr.Delete(context.Background(), containerd.WithSnapshotCleanup)
				if e != nil {
					r.Log().Error(e, "failed to delete container")
				}
			}
		}()
	}

	return connectivity.NewPodStatus(options.GetPodUid(), nil), nil
}

func (r *containerdRuntime) DeletePod(options *connectivity.DeleteOptions) (*connectivity.PodStatus, *connectivity.Error) {
	return nil, errors.New("method not implemented")
}

func (r *containerdRuntime) ListPods(options *connectivity.ListOptions) ([]*connectivity.PodStatus, *connectivity.Error) {
	return nil, errors.New("method not implemented")
}

func (r *containerdRuntime) ExecInContainer(podUID, container string, stdin io.Reader, stdout, stderr io.WriteCloser, resizeCh <-chan remotecommand.TerminalSize, command []string, tty bool) *connectivity.Error {
	timeoutCtx, cancel := r.RuntimeActionContext()
	defer cancel()

	ctr, err := r.findContainer(timeoutCtx, podUID, container)
	if err != nil {
		return err
	}

	spec, err := ctr.Spec(timeoutCtx)
	if err != nil {
		return err
	}

	task, err := ctr.Task(timeoutCtx, nil)
	if err != nil {
		return err
	}

	pspec := spec.Process
	pspec.Terminal = tty
	pspec.Args = command

	cioOpts := []cio.Opt{cio.WithStreams(stdin, stdout, stderr), cio.WithFIFODir("fifo-dir")}
	if tty {
		cioOpts = append(cioOpts, cio.WithTerminal)
	}
	ioCreator := cio.NewCreator(cioOpts...)

	execCtx, cancelExec := r.ActionContext()
	defer cancelExec()

	process, err := task.Exec(execCtx, uuid.NewV1().String(), pspec, ioCreator)
	if err != nil {
		return err
	}

	statusC, err := process.Wait(execCtx)
	if err != nil {
		return err
	}

	go func() {
		for size := range resizeCh {
			if err := process.Resize(execCtx, uint32(size.Width), uint32(size.Height)); err != nil {
				r.Log().Error(err, "failed to resize process tty size")
			}
		}
	}()

	if err := process.Start(execCtx); err != nil {
		return err
	}

	// wait for the process
	status := <-statusC
	_, _, err = status.Result()
	if err != nil {
		r.Log().Error(err, "exception from exec process")
		return err
	}

	return nil
}

func (r *containerdRuntime) AttachContainer(podUID, container string, stdin io.Reader, stdout, stderr io.WriteCloser, resizeCh <-chan remotecommand.TerminalSize) *connectivity.Error {
	timeoutCtx, cancel := r.RuntimeActionContext()
	defer cancel()

	ctr, err := r.findContainer(timeoutCtx, podUID, container)
	if err != nil {
		return err
	}

	attachCtx, cancelAttach := r.ActionContext()
	defer cancelAttach()
	task, err := ctr.Task(timeoutCtx, cio.NewAttach(cio.WithStreams(stdin, stdout, stderr)))
	if err != nil {
		return err
	}
	defer func() { _, _ = task.Delete(attachCtx) }()

	statusC, err := task.Wait(attachCtx)
	if err != nil {
		return err
	}

	go func() {
		for size := range resizeCh {
			if err := task.Resize(attachCtx, uint32(size.Width), uint32(size.Height)); err != nil {
				r.Log().Error(err, "failed to resize process tty size")
			}
		}
	}()

	// wait for the task
	status := <-statusC
	_, _, err = status.Result()
	if err != nil {
		r.Log().Error(err, "exception from attached process")
		return err
	}

	return nil
}

func (r *containerdRuntime) GetContainerLogs(podUID string, options *connectivity.LogOptions, stdout, stderr io.WriteCloser) *connectivity.Error {
	return errors.New("method not implemented")
}

func (r *containerdRuntime) PortForward(podUID string, protocol string, port int32, in io.Reader, out io.WriteCloser) *connectivity.Error {
	return nil
}

func (r *containerdRuntime) ensureImages(ctx context.Context, containers map[string]*connectivity.ContainerSpec, authConfig map[string]*connectivity.AuthConfig) (map[string]containerd.Image, error) {
	imageMap := make(map[string]containerd.Image)
	imageToPull := make([]string, 0)

	for _, ctr := range containers {
		if ctr.GetImagePullPolicy() == string(corev1.PullAlways) {
			imageToPull = append(imageToPull, ctr.GetImage())
			continue
		}

		image, err := r.imageClient.GetImage(ctx, ctr.GetImage())
		if err == nil {
			// image exists
			switch ctr.GetImagePullPolicy() {
			case string(corev1.PullNever), string(corev1.PullIfNotPresent):
				imageMap[ctr.GetImage()] = image
			}
		} else {
			// image does not exist
			switch ctr.GetImagePullPolicy() {
			case string(corev1.PullNever):
				return nil, err
			case string(corev1.PullIfNotPresent):
				imageToPull = append(imageToPull, ctr.GetImage())
			}
		}
	}

	for _, imageName := range imageToPull {
		pullOpts := []containerd.RemoteOpt{containerd.WithPullUnpack}
		if authConfig != nil {
			config, hasCred := authConfig[imageName]
			if hasCred {
				pullOpts = append(pullOpts, containerd.WithResolver(docker.NewResolver(docker.ResolverOptions{
					Authorizer: docker.NewAuthorizer(http.DefaultClient, func(host string) (username, password string, err error) {
						return config.GetUsername(), config.GetPassword(), nil
					}),
				})))
			}
		}

		image, err := r.imageClient.Pull(ctx, imageName, pullOpts...)
		if err != nil {
			return nil, err
		}
		imageMap[imageName] = image
	}

	return imageMap, nil
}

func (r *containerdRuntime) createContainer(
	ctx context.Context,
	podNamespace, podName, podUID, container, hostname string,
	namespaces map[string]string,
	spec *connectivity.ContainerSpec,
	volumeData map[string]*connectivity.NamedData,
	hostVolumes map[string]string,
) {

}
