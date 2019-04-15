// +build rt_docker

package docker

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	dockerType "github.com/docker/docker/api/types"
	dockerContainer "github.com/docker/docker/api/types/container"
	dockerFilter "github.com/docker/docker/api/types/filters"
	dockerMount "github.com/docker/docker/api/types/mount"
	dockerNetwork "github.com/docker/docker/api/types/network"
	dockerClient "github.com/docker/docker/client"
	dockerMessage "github.com/docker/docker/pkg/jsonmessage"
	dockerStdCopy "github.com/docker/docker/pkg/stdcopy"
	dockerNat "github.com/docker/go-connections/nat"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/remotecommand"
	criRuntime "k8s.io/kubernetes/pkg/kubelet/apis/cri/runtime/v1alpha2"

	"arhat.dev/aranya/pkg/constant"
	"arhat.dev/aranya/pkg/node/connectivity"
	"arhat.dev/aranya/pkg/node/connectivity/client/runtime"
	"arhat.dev/aranya/pkg/node/connectivity/client/runtimeutil"
)

func NewRuntime(ctx context.Context, config *runtime.Config) (runtime.Interface, error) {
	// dialCtxFunc := func(timeout time.Duration) func(ctx context.Context, network, addr string) (conn net.Conn, e error) {
	// 	return func(ctx context.Context, network, addr string) (conn net.Conn, e error) {
	// 		ctx, cancel := context.WithTimeout(ctx, timeout)
	// 		defer cancel()
	//
	// 		var dialer net.Dialer
	// 		log.Printf("docker dial: %v %v", network, addr)
	// 		return dialer.DialContext(ctx, network, addr)
	// 	}
	// }

	runtimeClient, err := dockerClient.NewClientWithOpts(
		dockerClient.WithHost(config.EndPoints.Runtime.Address),
		// dockerClient.WithDialContext(dialCtxFunc(config.EndPoints.Runtime.DialTimeout)),
		dockerClient.FromEnv,
	)
	if err != nil {
		return nil, err
	}

	imageClient := runtimeClient
	if config.EndPoints.Image.Address != config.EndPoints.Runtime.Address {
		imageClient, err = dockerClient.NewClientWithOpts(
			dockerClient.WithHost(config.EndPoints.Runtime.Address),
			// dockerClient.WithDialContext(dialCtxFunc(config.EndPoints.Image.DialTimeout)),
			dockerClient.FromEnv,
		)
		if err != nil {
			return nil, err
		}
	}

	infoCtx, cancelInfo := context.WithTimeout(ctx, config.EndPoints.Runtime.ActionTimeout)
	defer cancelInfo()

	versions, err := runtimeClient.ServerVersion(infoCtx)
	if err != nil {
		return nil, err
	}

	version := ""
	for _, ver := range versions.Components {
		if strings.ToLower(ver.Name) == "engine" {
			version = ver.Version
		}
	}

	return &dockerRuntime{
		Base:          runtime.NewRuntimeBase(ctx, "docker", version, config),
		imageClient:   imageClient,
		runtimeClient: runtimeClient,
	}, nil
}

type dockerRuntime struct {
	runtime.Base

	runtimeClient dockerClient.ContainerAPIClient
	imageClient   dockerClient.ImageAPIClient
}

func (r *dockerRuntime) CreatePod(options *connectivity.CreateOptions) (pod *connectivity.Pod, err error) {
	imagePullCtx, cancelPull := r.ImageActionContext()
	defer cancelPull()

	// ensure pause image (infra image to claim namespaces) exists
	_, err = r.ensureImages(imagePullCtx, map[string]*connectivity.ContainerSpec{
		"pause": {
			Image:           r.PauseImage,
			ImagePullPolicy: string(corev1.PullIfNotPresent),
		},
	}, nil)
	if err != nil {
		return nil, err
	}

	authConfig, err := options.GetResolvedImagePullAuthConfig()
	if err != nil {
		return nil, err
	}
	// ensure all images exists
	_, err = r.ensureImages(imagePullCtx, options.GetContainers(), authConfig)
	if err != nil {
		return nil, err
	}

	createCtx, cancelCreate := r.RuntimeActionContext()
	defer cancelCreate()

	pauseContainerSpec, ns, networkSettings, err := r.createPauseContainer(
		createCtx, options.GetNamespace(), options.GetName(), options.GetPodUid(),
		options.GetHostNetwork(), options.GetHostPid(), options.GetHostIpc(), options.GetHostname())
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			// delete pause container if any error happened
			e := r.deleteContainer(pauseContainerSpec.ID, 0)
			if e != nil {
				r.Log().Error(e, "failed to delete pause container after start failure")
			}
		}
	}()

	var containersCreated []string
	for containerName, containerSpec := range options.GetContainers() {
		ctrID, err := r.createContainer(
			createCtx, options.GetNamespace(), options.GetName(), options.GetPodUid(),
			containerName, options.GetHostname(), ns, containerSpec, options.GetVolumeData(),
			options.GetHostVolumes(), networkSettings)
		if err != nil {
			return nil, err
		}
		containersCreated = append(containersCreated, ctrID)
	}

	for _, ctrID := range containersCreated {
		err = r.runtimeClient.ContainerStart(createCtx, ctrID, dockerType.ContainerStartOptions{})
		if err != nil {
			return nil, err
		}

		defer func() {
			if err != nil {
				e := r.deleteContainer(ctrID, 0)
				if e != nil {
					r.Log().Error(e, "failed to delete container after start failure")
				}
			}
		}()
	}

	containersStatus := make([]*criRuntime.ContainerStatus, len(containersCreated))
	for i, ctrID := range containersCreated {
		ctrInfo, err := r.runtimeClient.ContainerInspect(createCtx, ctrID)
		if err != nil {
			return nil, err
		}
		containersStatus[i] = translateDockerContainerStatusToCRIContainerStatus(&ctrInfo)
	}

	return connectivity.NewPod(options.GetPodUid(), translateDockerContainerStatusToCRISandboxStatus(pauseContainerSpec), containersStatus), nil
}

func (r *dockerRuntime) DeletePod(options *connectivity.DeleteOptions) (pod *connectivity.Pod, err error) {
	deleteCtx, cancelDelete := r.RuntimeActionContext()
	defer cancelDelete()

	infraCtr, err := r.findContainer(options.GetPodUid(), constant.ContainerNamePause)
	if err != nil {
		return nil, err
	}

	timeout := time.Duration(options.GraceTime)
	now := time.Now()
	err = r.runtimeClient.ContainerStop(deleteCtx, infraCtr.ID, &timeout)
	if err != nil {
		return nil, err
	}

	err = r.runtimeClient.ContainerRemove(deleteCtx, infraCtr.ID, dockerType.ContainerRemoveOptions{
		RemoveVolumes: true,
		Force:         options.GetGraceTime() == 0,
	})
	if err != nil {
		return nil, err
	}

	containers, err := r.runtimeClient.ContainerList(deleteCtx, dockerType.ContainerListOptions{
		Quiet: true,
		Filters: dockerFilter.NewArgs(
			dockerFilter.Arg("label", constant.ContainerLabelPodUID+"="+options.GetPodUid()),
			dockerFilter.Arg("label", constant.ContainerLabelPodContainerRole+"="+constant.ContainerRoleWork),
		),
	})
	if err != nil {
		return nil, err
	}

	for _, ctr := range containers {
		timeout = timeout - time.Since(now)

		err := r.runtimeClient.ContainerStop(deleteCtx, ctr.ID, &timeout)
		if err != nil {
			return nil, err
		}

		err = r.runtimeClient.ContainerRemove(deleteCtx, ctr.ID, dockerType.ContainerRemoveOptions{
			RemoveVolumes: true,
			Force:         options.GetGraceTime() == 0,
		})
		if err != nil {
			return nil, err
		}
		now = time.Now()
	}

	return connectivity.NewPod(options.GetPodUid(), nil, nil), nil
}

func (r *dockerRuntime) ListPod(options *connectivity.ListOptions) ([]*connectivity.Pod, error) {
	listCtx, cancelList := r.RuntimeActionContext()
	defer cancelList()

	filter := dockerFilter.NewArgs()
	if !options.GetAll() {
		if options.GetNamespace() != "" {
			filter.Add("label", constant.ContainerLabelPodNamespace+"="+options.GetNamespace())
		}

		if options.GetName() != "" {
			filter.Add("label", constant.ContainerLabelPodName+"="+options.GetName())
		}
	}

	containers, err := r.runtimeClient.ContainerList(listCtx, dockerType.ContainerListOptions{
		All:     options.GetAll(),
		Quiet:   true,
		Filters: filter,
	})
	if err != nil {
		return nil, err
	}

	var (
		results []*connectivity.Pod
		// podUID -> pause container
		pauseContainers = make(map[string]dockerType.Container)
		// podUID -> containers
		podContainers = make(map[string][]dockerType.Container)
	)

	for _, ctr := range containers {
		podUID, hasUID := ctr.Labels[constant.ContainerLabelPodUID]
		if !hasUID {
			continue
		}

		_, isInfra := ctr.Labels[constant.ContainerRoleInfra]
		if isInfra {
			pauseContainers[podUID] = ctr
			continue
		}

		podContainers[podUID] = append(podContainers[podUID], ctr)
	}

	for podUID, pauseContainer := range pauseContainers {
		pauseCtrSpec, err := r.runtimeClient.ContainerInspect(listCtx, pauseContainer.ID)
		if err != nil {
			return nil, err
		}

		var containerStatus []*criRuntime.ContainerStatus
		for _, ctr := range podContainers[podUID] {
			ctrInfo, err := r.runtimeClient.ContainerInspect(listCtx, ctr.ID)
			if err != nil {
				return nil, err
			}
			containerStatus = append(containerStatus, translateDockerContainerStatusToCRIContainerStatus(&ctrInfo))
		}
		results = append(results, connectivity.NewPod(podUID, translateDockerContainerStatusToCRISandboxStatus(&pauseCtrSpec), containerStatus))
	}

	return results, nil
}

func (r *dockerRuntime) ExecInContainer(podUID, container string, stdin io.Reader, stdout, stderr io.WriteCloser, resizeCh <-chan remotecommand.TerminalSize, command []string, tty bool) error {
	ctr, err := r.findContainer(podUID, container)
	if err != nil {
		return err
	}

	execCtx, cancelExec := r.ActionContext()
	defer cancelExec()
	resp, err := r.runtimeClient.ContainerExecCreate(execCtx, ctr.ID, dockerType.ExecConfig{
		Tty:          tty,
		AttachStdin:  stdin != nil,
		AttachStdout: stdout != nil,
		AttachStderr: stderr != nil,
		Cmd:          command,
	})
	if err != nil {
		return err
	}

	attachResp, err := r.runtimeClient.ContainerExecAttach(execCtx, resp.ID, dockerType.ExecStartCheck{Tty: tty})
	if err != nil {
		return err
	}
	defer func() { _ = attachResp.Conn.Close() }()

	wg := &sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()

		_, err := io.Copy(attachResp.Conn, stdin)
		if err != nil {
			r.Log().Error(err, "exception when copy to attach stream")
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()

		_, err := dockerStdCopy.StdCopy(stdout, stderr, attachResp.Reader)
		if err != nil {
			r.Log().Error(err, "exception when copy from attach stream")
		}
	}()

	wg.Wait()

	return nil
}

func (r *dockerRuntime) AttachContainer(podUID, container string, stdin io.Reader, stdout, stderr io.WriteCloser, resizeCh <-chan remotecommand.TerminalSize) error {
	ctr, err := r.findContainer(podUID, container)
	if err != nil {
		return err
	}

	attachCtx, cancelAttach := r.ActionContext()
	defer cancelAttach()
	resp, err := r.runtimeClient.ContainerAttach(attachCtx, ctr.ID, dockerType.ContainerAttachOptions{
		Stream: true,
		Stdin:  stdin != nil,
		Stdout: stdout != nil,
		Stderr: stderr != nil,
	})
	if err != nil {
		return err
	}
	defer func() { _ = resp.Conn.Close() }()

	wg := &sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()

		if stdin != nil {
			_, err := io.Copy(resp.Conn, stdin)
			if err != nil {
				r.Log().Error(err, "exception when copy to attach stream")
			}
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()

		_, err := dockerStdCopy.StdCopy(stdout, stderr, resp.Reader)
		if err != nil {
			r.Log().Error(err, "exception when copy from attach stream")
		}
	}()

	wg.Wait()

	return nil
}

func (r *dockerRuntime) GetContainerLogs(podUID string, options *corev1.PodLogOptions, stdout, stderr io.WriteCloser) error {
	ctr, err := r.findContainer(podUID, options.Container)
	if err != nil {
		return err
	}

	logCtx, cancelLog := r.ActionContext()
	defer cancelLog()
	return runtimeutil.ReadLogs(logCtx, ctr.LogPath, options, stdout, stderr)
}

func (r *dockerRuntime) PortForward(podUID string, ports []int32, in io.Reader, out io.WriteCloser) error {
	return errors.New("method not implemented")
}

func (r *dockerRuntime) ensureImages(ctx context.Context, containers map[string]*connectivity.ContainerSpec, authConfig map[string]*criRuntime.AuthConfig) (map[string]*dockerType.ImageSummary, error) {
	imageMap := make(map[string]*dockerType.ImageSummary)
	imageToPull := make([]string, 0)

	for _, ctr := range containers {
		if ctr.GetImagePullPolicy() == string(corev1.PullAlways) {
			imageToPull = append(imageToPull, ctr.GetImage())
			continue
		}

		image, err := r.getImage(ctx, ctr.Image)
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
		authStr := ""
		if authConfig != nil {
			config, hasCred := authConfig[imageName]
			if hasCred {
				authCfg := dockerType.AuthConfig{
					Username:      config.GetUsername(),
					Password:      config.GetPassword(),
					ServerAddress: config.GetServerAddress(),
					IdentityToken: config.GetIdentityToken(),
					RegistryToken: config.GetRegistryToken(),
				}
				encodedJSON, err := json.Marshal(authCfg)
				if err != nil {
					panic(err)
				}
				authStr = base64.URLEncoding.EncodeToString(encodedJSON)
			}
		}

		out, err := r.imageClient.ImagePull(ctx, imageName, dockerType.ImagePullOptions{
			RegistryAuth: authStr,
		})
		if err != nil {
			return nil, err
		}
		err = func() error {
			defer func() { _ = out.Close() }()
			decoder := json.NewDecoder(out)
			for {
				var msg dockerMessage.JSONMessage
				err := decoder.Decode(&msg)
				if err == io.EOF {
					break
				}
				if err != nil {
					return err
				}
				if msg.Error != nil {
					return msg.Error
				}
			}
			return nil
		}()
		if err != nil {
			return nil, err
		}

		image, err := r.getImage(ctx, imageName)
		if err != nil {
			return nil, err
		}
		imageMap[imageName] = image
	}

	return imageMap, nil
}

func (r *dockerRuntime) getImage(ctx context.Context, imageName string) (*dockerType.ImageSummary, error) {
	imageList, err := r.imageClient.ImageList(ctx, dockerType.ImageListOptions{
		Filters: dockerFilter.NewArgs(dockerFilter.Arg("reference", imageName)),
	})
	if err != nil {
		return nil, err
	}

	if len(imageList) == 0 {
		return nil, errors.New("failed to find local image")
	}

	return &imageList[0], nil
}

func (r *dockerRuntime) findContainer(podUID, container string) (*dockerType.ContainerJSON, error) {
	findCtx, cancelFind := r.RuntimeActionContext()
	defer cancelFind()

	ctrName := runtimeutil.GetContainerName(podUID, container)
	ctr, err := r.runtimeClient.ContainerInspect(findCtx, ctrName)
	if err != nil {
		return nil, err
	}

	if ctr.ID == "" {
		return nil, errors.New("container not found")
	}

	return &ctr, err
}

func (r *dockerRuntime) createPauseContainer(
	ctx context.Context,
	podNamespace, podName, podUID string,
	hostNetwork, hostPID, hostIPC bool, hostname string,
) (ctrInfo *dockerType.ContainerJSON, ns map[string]string, netSettings map[string]*dockerNetwork.EndpointSettings, err error) {
	pauseContainerName := runtimeutil.GetContainerName(podUID, constant.ContainerNamePause)
	containerNsName := fmt.Sprintf("container:%s", pauseContainerName)
	pauseContainer, err := r.runtimeClient.ContainerCreate(ctx,
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
				return dockerContainer.NetworkMode(containerNsName)
			}(),
			IpcMode: func() dockerContainer.IpcMode {
				if hostIPC {
					return "host"
				}
				return dockerContainer.IpcMode(containerNsName)
			}(),
			PidMode: func() dockerContainer.PidMode {
				if hostPID {
					return "host"
				}
				return dockerContainer.PidMode(containerNsName)
			}(),
			UsernsMode: dockerContainer.UsernsMode(":" + pauseContainerName),
			UTSMode:    dockerContainer.UTSMode(":" + pauseContainerName),
		},
		&dockerNetwork.NetworkingConfig{
			EndpointsConfig: map[string]*dockerNetwork.EndpointSettings{},
		}, pauseContainerName)
	if err != nil {
		return nil, nil, nil, err
	}

	err = r.runtimeClient.ContainerStart(ctx, pauseContainer.ID, dockerType.ContainerStartOptions{})
	if err != nil {
		return nil, nil, nil, err
	}

	pauseContainerSpec, err := r.runtimeClient.ContainerInspect(ctx, pauseContainer.ID)
	if err != nil {
		return nil, nil, nil, err
	}

	ns = map[string]string{
		"net":  string(pauseContainerSpec.HostConfig.NetworkMode),
		"ipc":  string(pauseContainerSpec.HostConfig.IpcMode),
		"uts":  string(pauseContainerSpec.HostConfig.UTSMode),
		"user": string(pauseContainerSpec.HostConfig.UsernsMode),
	}
	// DO NOT share pid ns if it's not host
	if pauseContainerSpec.HostConfig.PidMode.IsHost() {
		ns["pid"] = "host"
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
) (ctrID string, err error) {
	containerName := runtimeutil.GetContainerName(podUID, container)
	var (
		exposedPorts     = make(dockerNat.PortSet)
		portBindings     = make(dockerNat.PortMap)
		containerVolumes = make(map[string]struct{})
		containerMounts  []dockerMount.Mount
		envs             []string
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

		if volData, isVolData := volumeData[volName]; isVolData && volData.GetData() != nil {
			dataMap := volData.GetData()

			dir := r.PodVolumeDir(podUID, "native", volName)
			if err = os.MkdirAll(dir, 0755); err != nil {
				return "", err
			}
			source, err = volMountSpec.Ensure(dir, dataMap)
			if err != nil {
				return "", err
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
		Privileged:   spec.GetPrivileged(),
		PortBindings: portBindings,
		AutoRemove:   true,
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

	ctr, err := r.runtimeClient.ContainerCreate(ctx, containerConfig, hostConfig, networkingConfig, containerName)
	if err != nil {
		return "", err
	}
	return ctr.ID, nil
}

func (r *dockerRuntime) deleteContainer(containerID string, timeout time.Duration) error {
	err := r.runtimeClient.ContainerStop(context.Background(), containerID, &timeout)
	if err != nil {
		return err
	}

	return r.runtimeClient.ContainerRemove(context.Background(), containerID, dockerType.ContainerRemoveOptions{
		RemoveVolumes: true,
		Force:         true,
	})
}

func dockerNetworkNamespacePath(ctrInfo dockerType.ContainerJSON) (string, error) {
	if ctrInfo.State.Pid == 0 {
		// Docker reports pid 0 for an exited container.
		return "", fmt.Errorf("cannot find network namespace for the terminated container %q", ctrInfo.ID)
	}
	return fmt.Sprintf("/proc/%v/ns/net", ctrInfo.State.Pid), nil
}
