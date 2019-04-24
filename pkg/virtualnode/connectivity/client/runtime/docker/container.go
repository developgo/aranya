package docker

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	dockerType "github.com/docker/docker/api/types"
	dockerContainer "github.com/docker/docker/api/types/container"
	dockerFilter "github.com/docker/docker/api/types/filters"
	dockerMount "github.com/docker/docker/api/types/mount"
	dockerNetwork "github.com/docker/docker/api/types/network"
	dockerClient "github.com/docker/docker/client"
	dockerNat "github.com/docker/go-connections/nat"

	"arhat.dev/aranya/pkg/constant"
	"arhat.dev/aranya/pkg/virtualnode/connectivity"
	"arhat.dev/aranya/pkg/virtualnode/connectivity/client/runtimeutil"
)

func translateRestartPolicy(policy connectivity.RestartPolicy) dockerContainer.RestartPolicy {
	switch policy {
	case connectivity.RestartAlways:
		return dockerContainer.RestartPolicy{Name: "always"}
	case connectivity.RestartOnFailure:
		return dockerContainer.RestartPolicy{Name: "on-failure"}
	case connectivity.RestartNever:
		return dockerContainer.RestartPolicy{Name: "no"}
	}

	return dockerContainer.RestartPolicy{Name: "always"}
}

func (r *dockerRuntime) findContainer(podUID, container string) (*dockerType.Container, *connectivity.Error) {
	findCtx, cancelFind := r.RuntimeActionContext()
	defer cancelFind()

	containers, err := r.runtimeClient.ContainerList(findCtx, dockerType.ContainerListOptions{
		Quiet: true,
		Filters: dockerFilter.NewArgs(
			dockerFilter.Arg("label", constant.ContainerLabelPodUID+"="+podUID),
			dockerFilter.Arg("label", constant.ContainerLabelPodContainer+"="+container),
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

func (r *dockerRuntime) createPauseContainer(ctx context.Context, options *connectivity.CreateOptions) (ctrInfo *dockerType.ContainerJSON, ns map[string]string, netSettings map[string]*dockerNetwork.EndpointSettings, err *connectivity.Error) {
	if _, err = r.findContainer(options.PodUid, constant.ContainerNamePause); err != runtimeutil.ErrNotFound {
		return nil, nil, nil, runtimeutil.ErrAlreadyExists
	}

	var (
		exposedPorts = make(dockerNat.PortSet)
		portBindings = make(dockerNat.PortMap)
		plainErr     error
	)

	for _, port := range options.Ports {
		ctrPort, plainErr := dockerNat.NewPort(port.Protocol, strconv.FormatInt(int64(port.ContainerPort), 10))
		if plainErr != nil {
			return nil, nil, nil, connectivity.NewCommonError(plainErr.Error())
		}

		exposedPorts[ctrPort] = struct{}{}
		portBindings[ctrPort] = []dockerNat.PortBinding{{
			HostPort: strconv.FormatInt(int64(port.GetHostPort()), 10),
		}}
	}

	pauseContainerName := runtimeutil.GetContainerName(options.Namespace, options.Name, constant.ContainerNamePause)
	pauseContainer, plainErr := r.runtimeClient.ContainerCreate(ctx,
		&dockerContainer.Config{
			ExposedPorts: exposedPorts,
			Hostname:     options.Hostname,
			Image:        r.PauseImage,
			Labels:       runtimeutil.ContainerLabels(options.Namespace, options.Name, options.PodUid, constant.ContainerNamePause),
		},
		&dockerContainer.HostConfig{
			Resources: dockerContainer.Resources{
				MemorySwap: 0,
				CPUShares:  2,
			},
			PortBindings:  portBindings,
			RestartPolicy: translateRestartPolicy(options.GetRestartPolicy()),
			NetworkMode: func() dockerContainer.NetworkMode {
				if options.HostNetwork {
					return "host"
				}
				return "default"
			}(),
			IpcMode: func() dockerContainer.IpcMode {
				if options.HostIpc {
					return "host"
				}
				return "shareable"
			}(),
			PidMode: func() dockerContainer.PidMode {
				if options.HostPid {
					return "host"
				}
				return "container"
			}(),
		},
		&dockerNetwork.NetworkingConfig{
			EndpointsConfig: map[string]*dockerNetwork.EndpointSettings{},
		}, pauseContainerName)
	if plainErr != nil {
		return nil, nil, nil, connectivity.NewCommonError(plainErr.Error())
	}

	plainErr = r.runtimeClient.ContainerStart(ctx, pauseContainer.ID, dockerType.ContainerStartOptions{})
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
	endpointSettings map[string]*dockerNetwork.EndpointSettings,
) (ctrID string, err *connectivity.Error) {
	if _, err = r.findContainer(options.PodUid, container); err != runtimeutil.ErrNotFound {
		return "", runtimeutil.ErrAlreadyExists
	}

	var (
		plainErr         error
		containerVolumes = make(map[string]struct{})
		containerBinds   []string
		containerMounts  []dockerMount.Mount
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

		if volData, isVolData := volumeData[volName]; isVolData && volData.GetDataMap() != nil {
			dataMap := volData.GetDataMap()

			dir := r.PodVolumeDir(options.PodUid, "native", volName)
			if plainErr = os.MkdirAll(dir, 0755); plainErr != nil {
				return "", connectivity.NewCommonError(plainErr.Error())
			}

			source, plainErr = volMountSpec.Ensure(dir, dataMap)
			if plainErr != nil {
				return "", connectivity.NewCommonError(plainErr.Error())
			}
		}

		containerMounts = append(containerMounts, dockerMount.Mount{
			Type:     "bind",
			Source:   source,
			Target:   filepath.Join(volMountSpec.MountPath, volMountSpec.SubPath),
			ReadOnly: volMountSpec.ReadOnly,
		})
	}

	containerConfig := &dockerContainer.Config{
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

	hostConfig := &dockerContainer.HostConfig{
		Resources: dockerContainer.Resources{MemorySwap: 0, CPUShares: 2},

		Binds:  containerBinds,
		Mounts: containerMounts,

		RestartPolicy: translateRestartPolicy(options.RestartPolicy),

		// share namespaces
		NetworkMode: dockerContainer.NetworkMode(namespaces["net"]),
		IpcMode:     dockerContainer.IpcMode(namespaces["ipc"]),
		UTSMode:     dockerContainer.UTSMode(namespaces["uts"]),
		UsernsMode:  dockerContainer.UsernsMode(namespaces["user"]),
		// shared only when it's host (do not add `pid` ns to `namespaces`)
		PidMode: dockerContainer.PidMode(namespaces["pid"]),

		// security options
		Privileged:     spec.Security.GetPrivileged(),
		CapAdd:         spec.Security.GetCapsAdd(),
		CapDrop:        spec.Security.GetCapsDrop(),
		ReadonlyRootfs: spec.Security.GetReadOnlyRootfs(),
	}
	networkingConfig := &dockerNetwork.NetworkingConfig{
		EndpointsConfig: endpointSettings,
	}

	ctr, plainErr := r.runtimeClient.ContainerCreate(ctx, containerConfig, hostConfig, networkingConfig, containerName)
	if plainErr != nil {
		return "", connectivity.NewCommonError(plainErr.Error())
	}

	return ctr.ID, nil
}

func (r *dockerRuntime) deleteContainer(containerID string, timeout time.Duration) *connectivity.Error {
	// stop with best effort
	_ = r.runtimeClient.ContainerStop(context.Background(), containerID, &timeout)
	err := r.runtimeClient.ContainerRemove(context.Background(), containerID, dockerType.ContainerRemoveOptions{
		RemoveVolumes: true,
		Force:         true,
	})
	if err != nil && !dockerClient.IsErrNotFound(err) {
		return connectivity.NewCommonError(err.Error())
	}

	return nil
}
