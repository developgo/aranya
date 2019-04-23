package docker

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"time"

	dockerType "github.com/docker/docker/api/types"
	dockerContainer "github.com/docker/docker/api/types/container"
	dockerFilter "github.com/docker/docker/api/types/filters"
	dockerMount "github.com/docker/docker/api/types/mount"
	dockerNetwork "github.com/docker/docker/api/types/network"
	dockerNat "github.com/docker/go-connections/nat"

	"arhat.dev/aranya/pkg/constant"
	"arhat.dev/aranya/pkg/virtualnode/connectivity"
	"arhat.dev/aranya/pkg/virtualnode/connectivity/client/runtimeutil"
)

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

func (r *dockerRuntime) createPauseContainer(
	ctx context.Context,
	podNamespace, podName, podUID string,
	hostNetwork, hostPID, hostIPC bool, hostname string,
) (ctrInfo *dockerType.ContainerJSON, ns map[string]string, netSettings map[string]*dockerNetwork.EndpointSettings, err *connectivity.Error) {
	if _, err = r.findContainer(podUID, constant.ContainerNamePause); err != runtimeutil.ErrNotFound {
		return nil, nil, nil, runtimeutil.ErrAlreadyExists
	}

	var plainErr error
	pauseContainerName := runtimeutil.GetContainerName(podNamespace, podName, constant.ContainerNamePause)
	pauseContainer, plainErr := r.runtimeClient.ContainerCreate(ctx,
		&dockerContainer.Config{
			Hostname: hostname,
			Image:    r.PauseImage,
			Labels:   runtimeutil.ContainerLabels(podNamespace, podName, podUID, constant.ContainerNamePause),
		},
		&dockerContainer.HostConfig{
			Resources: dockerContainer.Resources{
				MemorySwap: 0,
				CPUShares:  2,
			},
			NetworkMode: func() dockerContainer.NetworkMode {
				if hostNetwork {
					return "host"
				}
				return "default"
			}(),
			IpcMode: func() dockerContainer.IpcMode {
				if hostIPC {
					return "host"
				}
				return "shareable"
			}(),
			PidMode: func() dockerContainer.PidMode {
				if hostPID {
					return "host"
				}
				return "container"
			}(),
			// UsernsMode: "host",
			// UTSMode:    "host",
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
	podNamespace, podName, podUID, container, hostname string,
	namespaces map[string]string,
	spec *connectivity.ContainerSpec,
	volumeData map[string]*connectivity.NamedData,
	hostVolumes map[string]string,
	endpointSettings map[string]*dockerNetwork.EndpointSettings,
) (ctrID string, err *connectivity.Error) {
	if _, err = r.findContainer(podUID, container); err != runtimeutil.ErrNotFound {
		return "", runtimeutil.ErrAlreadyExists
	}

	var (
		exposedPorts     = make(dockerNat.PortSet)
		portBindings     = make(dockerNat.PortMap)
		containerVolumes = make(map[string]struct{})
		containerBinds   []string
		containerMounts  []dockerMount.Mount
		envs             []string
		containerName    = runtimeutil.GetContainerName(podNamespace, podName, container)
		plainErr         error
	)

	for _, port := range spec.GetPorts() {
		ctrPort, err := dockerNat.NewPort(port.GetProtocol(), strconv.FormatInt(int64(port.GetContainerPort()), 10))
		if err != nil {
			return "", nil
		}
		exposedPorts[ctrPort] = struct{}{}
		portBindings[ctrPort] = []dockerNat.PortBinding{{
			HostIP:   port.GetHostIp(),
			HostPort: strconv.FormatInt(int64(port.GetHostPort()), 10),
		}}
	}

	for k, v := range spec.GetEnvs() {
		envs = append(envs, k+"="+v)
	}

	for volName, volMountSpec := range spec.GetVolumeMounts() {
		containerVolumes[volMountSpec.GetMountPath()] = struct{}{}

		source := ""
		hostPath, isHostVol := hostVolumes[volName]
		if isHostVol {
			source = hostPath
		}

		if volData, isVolData := volumeData[volName]; isVolData && volData.GetDataMap() != nil {
			dataMap := volData.GetDataMap()

			dir := r.PodVolumeDir(podUID, "native", volName)
			if plainErr = os.MkdirAll(dir, 0755); err != nil {
				return "", connectivity.NewCommonError(plainErr.Error())
			}
			source, plainErr = volMountSpec.Ensure(dir, dataMap)
			if plainErr != nil {
				return "", connectivity.NewCommonError(plainErr.Error())
			}
		}

		containerMounts = append(containerMounts, dockerMount.Mount{
			Type:     dockerMount.Type(volMountSpec.GetType()),
			Source:   source,
			Target:   volMountSpec.GetMountPath(),
			ReadOnly: volMountSpec.GetReadOnly(),
		})
	}
	containerConfig := &dockerContainer.Config{
		Hostname:     hostname,
		ExposedPorts: exposedPorts,
		Labels:       runtimeutil.ContainerLabels(podNamespace, podName, podUID, container),
		Image:        spec.GetImage(),
		Env:          envs,
		Tty:          spec.GetTty(),
		OpenStdin:    spec.GetStdin(),
		Volumes:      containerVolumes,
		StopSignal:   "SIGTERM",
		Entrypoint:   spec.GetCommand(),
		Cmd:          spec.GetArgs(),
		WorkingDir:   spec.GetWorkingDir(),
	}
	hostConfig := &dockerContainer.HostConfig{
		Binds:        containerBinds,
		Privileged:   spec.GetPrivileged(),
		PortBindings: portBindings,
		Mounts:       containerMounts,
		Resources: dockerContainer.Resources{
			MemorySwap: 0,
			CPUShares:  2,
		},
		NetworkMode: dockerContainer.NetworkMode(namespaces["net"]),
		IpcMode:     dockerContainer.IpcMode(namespaces["ipc"]),
		UTSMode:     dockerContainer.UTSMode(namespaces["uts"]),
		UsernsMode:  dockerContainer.UsernsMode(namespaces["user"]),
		// shared only when it's host
		PidMode: dockerContainer.PidMode(namespaces["pid"]),
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
	if err != nil {
		return connectivity.NewCommonError(err.Error())
	}

	return nil
}
