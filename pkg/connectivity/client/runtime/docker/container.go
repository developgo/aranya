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

package docker

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	dockertype "github.com/docker/docker/api/types"
	dockercontainer "github.com/docker/docker/api/types/container"
	dockerfilter "github.com/docker/docker/api/types/filters"
	dockermount "github.com/docker/docker/api/types/mount"
	dockernetwork "github.com/docker/docker/api/types/network"
	dockerclient "github.com/docker/docker/client"
	dockernat "github.com/docker/go-connections/nat"

	"arhat.dev/aranya/pkg/connectivity"
	"arhat.dev/aranya/pkg/connectivity/client/runtimeutil"
	"arhat.dev/aranya/pkg/constant"
)

func translateRestartPolicy(policy connectivity.RestartPolicy) dockercontainer.RestartPolicy {
	switch policy {
	case connectivity.RestartAlways:
		return dockercontainer.RestartPolicy{Name: "always"}
	case connectivity.RestartOnFailure:
		return dockercontainer.RestartPolicy{Name: "on-failure"}
	case connectivity.RestartNever:
		return dockercontainer.RestartPolicy{Name: "no"}
	}

	return dockercontainer.RestartPolicy{Name: "always"}
}

func (r *dockerRuntime) findContainer(podUID, container string) (*dockertype.Container, *connectivity.Error) {
	findCtx, cancelFind := r.RuntimeActionContext()
	defer cancelFind()

	containers, err := r.runtimeClient.ContainerList(findCtx, dockertype.ContainerListOptions{
		Quiet: true,
		Filters: dockerfilter.NewArgs(
			dockerfilter.Arg("label", constant.ContainerLabelPodUID+"="+podUID),
			dockerfilter.Arg("label", constant.ContainerLabelPodContainer+"="+container),
		),
	})
	if err != nil {
		return nil, connectivity.NewCommonError(err.Error())
	}

	if len(containers) != 1 {
		return nil, runtimeutil.ErrNotFound
	}

	return &containers[0], nil
}

func (r *dockerRuntime) createPauseContainer(ctx context.Context, options *connectivity.CreateOptions) (ctrInfo *dockertype.ContainerJSON, ns map[string]string, netSettings map[string]*dockernetwork.EndpointSettings, err *connectivity.Error) {
	if _, err = r.findContainer(options.PodUid, constant.ContainerNamePause); err != runtimeutil.ErrNotFound {
		return nil, nil, nil, runtimeutil.ErrAlreadyExists
	}

	var (
		exposedPorts = make(dockernat.PortSet)
		portBindings = make(dockernat.PortMap)
		plainErr     error
	)

	for _, port := range options.Ports {
		ctrPort, plainErr := dockernat.NewPort(port.Protocol, strconv.FormatInt(int64(port.ContainerPort), 10))
		if plainErr != nil {
			return nil, nil, nil, connectivity.NewCommonError(plainErr.Error())
		}

		exposedPorts[ctrPort] = struct{}{}
		portBindings[ctrPort] = []dockernat.PortBinding{{
			HostPort: strconv.FormatInt(int64(port.HostPort), 10),
		}}
	}

	pauseContainerName := runtimeutil.GetContainerName(options.Namespace, options.Name, constant.ContainerNamePause)
	pauseContainer, plainErr := r.runtimeClient.ContainerCreate(ctx,
		&dockercontainer.Config{
			ExposedPorts: exposedPorts,
			Hostname:     options.Hostname,
			Image:        r.PauseImage,
			Labels:       runtimeutil.ContainerLabels(options.Namespace, options.Name, options.PodUid, constant.ContainerNamePause),
		},
		&dockercontainer.HostConfig{
			Resources: dockercontainer.Resources{
				MemorySwap: 0,
				CPUShares:  2,
			},
			PortBindings:  portBindings,
			RestartPolicy: translateRestartPolicy(options.RestartPolicy),
			NetworkMode: func() dockercontainer.NetworkMode {
				if options.HostNetwork {
					return "host"
				}
				return "default"
			}(),
			IpcMode: func() dockercontainer.IpcMode {
				if options.HostIpc {
					return "host"
				}
				return "shareable"
			}(),
			PidMode: func() dockercontainer.PidMode {
				if options.HostPid {
					return "host"
				}
				return "container"
			}(),
		},
		&dockernetwork.NetworkingConfig{
			EndpointsConfig: map[string]*dockernetwork.EndpointSettings{},
		}, pauseContainerName)
	if plainErr != nil {
		return nil, nil, nil, connectivity.NewCommonError(plainErr.Error())
	}

	plainErr = r.runtimeClient.ContainerStart(ctx, pauseContainer.ID, dockertype.ContainerStartOptions{})
	if plainErr != nil {
		return nil, nil, nil, connectivity.NewCommonError(plainErr.Error())
	}

	pauseContainerSpec, plainErr := r.runtimeClient.ContainerInspect(ctx, pauseContainer.ID)
	if plainErr != nil {
		return nil, nil, nil, connectivity.NewCommonError(plainErr.Error())
	}

	containerNS := fmt.Sprintf("container:%s", pauseContainer.ID)
	ns = map[string]string{
		"net":  containerNS,
		"ipc":  containerNS,
		"uts":  containerNS,
		"user": containerNS,
	}

	return &pauseContainerSpec, ns, pauseContainerSpec.NetworkSettings.Networks, nil
}

func (r *dockerRuntime) createContainer(
	ctx context.Context,
	options *connectivity.CreateOptions,
	container string,
	namespaces map[string]string,
	spec *connectivity.ContainerSpec,
	endpointSettings map[string]*dockernetwork.EndpointSettings,
) (ctrID string, err *connectivity.Error) {
	if _, err = r.findContainer(options.PodUid, container); err != runtimeutil.ErrNotFound {
		return "", runtimeutil.ErrAlreadyExists
	}

	var (
		plainErr         error
		containerVolumes = make(map[string]struct{})
		containerBinds   []string
		containerMounts  []dockermount.Mount
		envs             []string
		hostPaths        = options.HostPaths
		volumeData       = options.VolumeData
		containerName    = runtimeutil.GetContainerName(options.Namespace, options.Name, container)
	)

	// generalize to avoid panic
	if hostPaths == nil {
		hostPaths = make(map[string]string)
	}

	if volumeData == nil {
		volumeData = make(map[string]*connectivity.NamedData)
	}

	for k, v := range spec.Envs {
		envs = append(envs, k+"="+v)
	}

	for volName, volMountSpec := range spec.Mounts {
		containerVolumes[volMountSpec.MountPath] = struct{}{}

		source := ""
		hostPath, isHostVol := hostPaths[volName]
		if isHostVol {
			source = hostPath
		}

		if volData, isVolData := volumeData[volName]; isVolData {
			if dataMap := volData.GetDataMap(); dataMap != nil {
				dir := r.PodVolumeDir(options.PodUid, "native", volName)
				if plainErr = os.MkdirAll(dir, 0755); plainErr != nil {
					return "", connectivity.NewCommonError(plainErr.Error())
				}

				source, plainErr = volMountSpec.Ensure(dir, dataMap)
				if plainErr != nil {
					return "", connectivity.NewCommonError(plainErr.Error())
				}
			}
		}

		containerMounts = append(containerMounts, dockermount.Mount{
			Type:     "bind",
			Source:   source,
			Target:   filepath.Join(volMountSpec.MountPath, volMountSpec.SubPath),
			ReadOnly: volMountSpec.ReadOnly,
		})
	}

	containerConfig := &dockercontainer.Config{
		Hostname:   options.Hostname,
		Labels:     runtimeutil.ContainerLabels(options.Namespace, options.Name, options.PodUid, container),
		Image:      spec.Image,
		Env:        envs,
		Tty:        spec.Tty,
		OpenStdin:  spec.Stdin,
		StdinOnce:  spec.StdinOnce,
		Volumes:    containerVolumes,
		StopSignal: "SIGTERM",
		Entrypoint: spec.Command,
		Cmd:        spec.Args,
		WorkingDir: spec.WorkingDir,
	}

	hostConfig := &dockercontainer.HostConfig{
		Resources: dockercontainer.Resources{MemorySwap: 0, CPUShares: 2},

		Binds:  containerBinds,
		Mounts: containerMounts,

		RestartPolicy: translateRestartPolicy(options.RestartPolicy),

		// share namespaces
		NetworkMode: dockercontainer.NetworkMode(namespaces["net"]),
		IpcMode:     dockercontainer.IpcMode(namespaces["ipc"]),
		UTSMode:     dockercontainer.UTSMode(namespaces["uts"]),
		UsernsMode:  dockercontainer.UsernsMode(namespaces["user"]),
		// shared only when it's host (do not add `pid` ns to `namespaces`)
		PidMode: dockercontainer.PidMode(namespaces["pid"]),

		// security options
		Privileged:     spec.Security.GetPrivileged(),
		CapAdd:         spec.Security.GetCapsAdd(),
		CapDrop:        spec.Security.GetCapsDrop(),
		ReadonlyRootfs: spec.Security.GetReadOnlyRootfs(),
	}
	networkingConfig := &dockernetwork.NetworkingConfig{
		EndpointsConfig: endpointSettings,
	}

	ctr, plainErr := r.runtimeClient.ContainerCreate(ctx, containerConfig, hostConfig, networkingConfig, containerName)
	if plainErr != nil {
		return "", connectivity.NewCommonError(plainErr.Error())
	}

	return ctr.ID, nil
}

func (r *dockerRuntime) deleteContainer(containerID string) *connectivity.Error {
	// stop with best effort
	timeout := time.Duration(0)
	_ = r.runtimeClient.ContainerStop(context.Background(), containerID, &timeout)
	err := r.runtimeClient.ContainerRemove(context.Background(), containerID, dockertype.ContainerRemoveOptions{
		RemoveVolumes: true,
		Force:         true,
	})
	if err != nil && !dockerclient.IsErrNotFound(err) {
		return connectivity.NewCommonError(err.Error())
	}

	return nil
}
