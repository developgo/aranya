// +build rt_docker

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
	"io"
	"io/ioutil"
	"net"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	dockerType "github.com/docker/docker/api/types"
	dockerFilter "github.com/docker/docker/api/types/filters"
	dockerClient "github.com/docker/docker/client"
	dockerCopy "github.com/docker/docker/pkg/stdcopy"

	"arhat.dev/aranya/pkg/connectivity"
	"arhat.dev/aranya/pkg/connectivity/client/runtime"
	"arhat.dev/aranya/pkg/connectivity/client/runtimeutil"
	"arhat.dev/aranya/pkg/constant"
)

func NewRuntime(ctx context.Context, config *runtime.Config) (runtime.Interface, error) {
	dialCtxFunc := func(timeout time.Duration) func(ctx context.Context, network, addr string) (conn net.Conn, e error) {
		return func(ctx context.Context, network, addr string) (conn net.Conn, e error) {
			ctx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()

			var dialer net.Dialer
			if filepath.IsAbs(addr) {
				network = "unix"
				idx := strings.LastIndexByte(addr, ':')
				if idx != -1 {
					addr = addr[:idx]
				}
			}
			return dialer.DialContext(ctx, network, addr)
		}
	}

	runtimeClient, err := dockerClient.NewClientWithOpts(
		dockerClient.WithHost(config.EndPoints.Runtime.Address),
		dockerClient.WithDialContext(dialCtxFunc(config.EndPoints.Runtime.DialTimeout)),
		dockerClient.FromEnv,
	)
	if err != nil {
		return nil, err
	}

	imageClient := runtimeClient
	if config.EndPoints.Image.Address != config.EndPoints.Runtime.Address {
		imageClient, err = dockerClient.NewClientWithOpts(
			dockerClient.WithHost(config.EndPoints.Runtime.Address),
			dockerClient.WithDialContext(dialCtxFunc(config.EndPoints.Image.DialTimeout)),
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
		Base:          runtime.NewRuntimeBase(ctx, config, "docker", version, versions.Os, versions.Arch, versions.KernelVersion),
		imageClient:   imageClient,
		runtimeClient: runtimeClient,
	}, nil
}

type dockerRuntime struct {
	runtime.Base

	runtimeClient dockerClient.ContainerAPIClient
	imageClient   dockerClient.ImageAPIClient
}

func (r *dockerRuntime) CreatePod(options *connectivity.CreateOptions) (pod *connectivity.PodStatus, err *connectivity.Error) {
	createLog := r.Log().WithValues("action", "create", "namespace", options.GetNamespace(), "name", options.GetName(), "uid", options.GetPodUid())

	// ensure pause image (infra image to claim namespaces) exists
	_, err = r.ensureImages(map[string]*connectivity.ContainerSpec{
		"pause": {
			Image:           r.PauseImage,
			ImagePullPolicy: connectivity.ImagePullIfNotPresent,
		},
	}, nil)
	if err != nil {
		createLog.Error(err, "failed to ensure pause image")
		return nil, err
	}

	// ensure all images exists
	_, err = r.ensureImages(options.GetContainers(), options.GetImagePullAuthConfig())
	if err != nil {
		createLog.Error(err, "failed to ensure container images")
		return nil, err
	}

	createCtx, cancelCreate := r.RuntimeActionContext()
	defer cancelCreate()

	pauseContainerInfo, ns, networkSettings, err := r.createPauseContainer(createCtx, options)
	if err != nil {
		createLog.Error(err, "failed to create pause container")
		return nil, err
	}
	defer func() {
		if err != nil {
			// delete pause container if any error happened
			createLog.Info("delete pause container due to error")
			e := r.deleteContainer(pauseContainerInfo.ID, 0)
			if e != nil {
				createLog.Error(e, "failed to delete pause container after start failure")
			}
		}
	}()

	var containersCreated []string
	for containerName, containerSpec := range options.GetContainers() {
		ctrID, err := r.createContainer(createCtx, options, containerName, ns, containerSpec, networkSettings)
		if err != nil {
			createLog.Error(err, "failed to create container", "container", containerName)
			return nil, err
		}
		containersCreated = append(containersCreated, ctrID)
	}

	for _, ctrID := range containersCreated {
		plainErr := r.runtimeClient.ContainerStart(createCtx, ctrID, dockerType.ContainerStartOptions{})
		if plainErr != nil {
			createLog.Error(plainErr, "failed to start container", "containerID", ctrID)
			return nil, connectivity.NewCommonError(plainErr.Error())
		}

		defer func() {
			if err != nil {
				createLog.Info("delete container due to error", "containerID", ctrID)
				if e := r.deleteContainer(ctrID, 0); e != nil {
					createLog.Error(e, "failed to delete container after start failure")
				}
			}
		}()
	}

	containersInfo := make([]*dockerType.ContainerJSON, len(containersCreated))
	for i, ctrID := range containersCreated {
		ctrInfo, plainErr := r.runtimeClient.ContainerInspect(createCtx, ctrID)
		if plainErr != nil {
			createLog.Error(plainErr, "failed to inspect docker container")
			return nil, connectivity.NewCommonError(plainErr.Error())
		}
		containersInfo[i] = &ctrInfo
	}

	createLog.Info("success")
	return r.translatePodStatus(pauseContainerInfo, containersInfo), nil
}

func (r *dockerRuntime) DeletePod(options *connectivity.DeleteOptions) (pod *connectivity.PodStatus, err *connectivity.Error) {
	deleteLog := r.Log().WithValues("action", "delete", "options", options)

	deleteCtx, cancelDelete := r.RuntimeActionContext()
	defer cancelDelete()

	deleteLog.Info("trying to list containers for pod delete")
	var plainErr error
	containers, plainErr := r.runtimeClient.ContainerList(deleteCtx, dockerType.ContainerListOptions{
		Quiet: true,
		Filters: dockerFilter.NewArgs(
			dockerFilter.Arg("label", constant.ContainerLabelPodUID+"="+options.PodUid),
		),
	})
	if plainErr != nil {
		deleteLog.Error(plainErr, "failed to list containers")
		return nil, connectivity.NewCommonError(plainErr.Error())
	}

	// delete work containers first
	now := time.Now()
	timeout := time.Duration(options.GraceTime)

	pauseCtrIndex := -1
	for i, ctr := range containers {
		// find pause container
		if ctr.Labels[constant.ContainerLabelPodContainerRole] == constant.ContainerRoleInfra {
			pauseCtrIndex = i
			break
		}
	}
	lastIndex := len(containers) - 1
	// swap pause container to last
	deleteLog.Info("original containers", "containers", containers)
	if pauseCtrIndex != -1 {
		containers[lastIndex], containers[pauseCtrIndex] = containers[pauseCtrIndex], containers[lastIndex]
	}
	deleteLog.Info("swapped containers", "containers", containers)

	for _, ctr := range containers {
		timeout = timeout - time.Since(now)
		now = time.Now()

		if timeout < 0 {
			timeout = 0
		}

		deleteLog.Info("trying to delete container", "timeout", timeout)
		err = r.deleteContainer(ctr.ID, timeout)
		if err != nil {
			deleteLog.Error(err, "failed to delete container")
			return nil, err
		}
	}

	r.Log().Info("pod deleted")
	return connectivity.NewPodStatus(options.GetPodUid(), nil), nil
}

func (r *dockerRuntime) ListPods(options *connectivity.ListOptions) ([]*connectivity.PodStatus, *connectivity.Error) {
	listLog := r.Log().WithValues("action", "list", "options", options)

	listCtx, cancelList := r.RuntimeActionContext()
	defer cancelList()

	filter := dockerFilter.NewArgs()
	if !options.All {
		if options.Namespace != "" {
			filter.Add("label", constant.ContainerLabelPodNamespace+"="+options.Namespace)
		}

		if options.Name != "" {
			filter.Add("label", constant.ContainerLabelPodName+"="+options.Name)
		}
	}

	listLog.Info("listing containers")
	containers, err := r.runtimeClient.ContainerList(listCtx, dockerType.ContainerListOptions{
		All:     options.All,
		Quiet:   true,
		Filters: filter,
	})
	if err != nil {
		listLog.Error(err, "failed to list containers")
		return nil, connectivity.NewCommonError(err.Error())
	}

	var (
		results []*connectivity.PodStatus
		// podUID -> pause container
		pauseContainers = make(map[string]dockerType.Container)
		// podUID -> containers
		podContainers = make(map[string][]dockerType.Container)
	)

	for _, ctr := range containers {
		podUID, hasUID := ctr.Labels[constant.ContainerLabelPodUID]
		if !hasUID {
			// not the container created by us
			continue
		}

		role, hasRole := ctr.Labels[constant.ContainerLabelPodContainerRole]
		if hasRole && role == constant.ContainerRoleInfra {
			pauseContainers[podUID] = ctr
			continue
		}

		podContainers[podUID] = append(podContainers[podUID], ctr)
	}

	// one pause container represents on Pod
	for podUID, pauseContainer := range pauseContainers {
		pauseCtrSpec, err := r.runtimeClient.ContainerInspect(listCtx, pauseContainer.ID)
		if err != nil {
			listLog.Error(err, "failed to inspect pause container")
			return nil, connectivity.NewCommonError(err.Error())
		}

		var containersInfo []*dockerType.ContainerJSON
		for _, ctr := range podContainers[podUID] {
			ctrInfo, err := r.runtimeClient.ContainerInspect(listCtx, ctr.ID)
			if err != nil {
				listLog.Error(err, "failed to inspect work container")
				return nil, connectivity.NewCommonError(err.Error())
			}
			containersInfo = append(containersInfo, &ctrInfo)
		}

		results = append(results, r.translatePodStatus(&pauseCtrSpec, containersInfo))
	}

	return results, nil
}

func (r *dockerRuntime) ExecInContainer(podUID, container string, stdin io.Reader, stdout, stderr io.WriteCloser, resizeCh <-chan *connectivity.TtyResizeOptions, command []string, tty bool) *connectivity.Error {
	execLog := r.Log().WithValues("uid", podUID, "container", container, "action", "exec")

	execLog.Info("trying to find container")
	ctr, err := r.findContainer(podUID, container)
	if err != nil {
		execLog.Error(err, "failed to find container")
		return runtimeutil.ErrNotFound
	}

	execCtx, cancelExec := r.ActionContext()
	defer cancelExec()

	var plainErr error
	execLog.Info("trying to exec create")
	resp, plainErr := r.runtimeClient.ContainerExecCreate(execCtx, ctr.ID, dockerType.ExecConfig{
		Tty:          tty,
		AttachStdin:  stdin != nil,
		AttachStdout: stdout != nil,
		AttachStderr: stderr != nil,
		Cmd:          command,
	})
	if plainErr != nil {
		execLog.Error(plainErr, "failed to exec create")
		return connectivity.NewCommonError(plainErr.Error())
	}

	execLog.Info("trying to exec attach")
	attachResp, plainErr := r.runtimeClient.ContainerExecAttach(execCtx, resp.ID, dockerType.ExecStartCheck{Tty: tty})
	if plainErr != nil {
		execLog.Error(plainErr, "failed to exec attach")
		return connectivity.NewCommonError(plainErr.Error())
	}
	defer func() { _ = attachResp.Conn.Close() }()

	if stdin != nil {
		go func() {
			execLog.Info("starting write routine")
			defer execLog.Info("finished write routine")

			_, err := io.Copy(attachResp.Conn, stdin)
			if err != nil {
				execLog.Error(err, "exception happened in write routine")
			}
		}()
	}

	go func() {
		defer execLog.Info("finished tty resize routine")

		for {
			select {
			case size, more := <-resizeCh:
				if !more {
					return
				}

				err := r.runtimeClient.ContainerExecResize(execCtx, resp.ID, dockerType.ResizeOptions{
					Height: uint(size.GetRows()),
					Width:  uint(size.GetCols()),
				})
				if err != nil {
					// DO NOT break here
					execLog.Error(err, "exception happened in tty resize routine")
				}
			case <-execCtx.Done():
				return
			}
		}
	}()

	// Here, we will only wait for the output
	// since input (stdin) and resize (tty) are optional
	// and kubectl doesn't have a detach option, so the stdout will always be there
	// once this function call returned, base_runtime will close everything related

	execLog.Info("starting read routine")
	defer execLog.Info("finished read routine")

	var stdOut, stdErr io.Writer
	stdOut, stdErr = stdout, stderr
	if stdout == nil {
		stdOut = ioutil.Discard
	}
	if stderr == nil {
		stdErr = ioutil.Discard
	}

	if tty {
		_, plainErr = io.Copy(stdOut, attachResp.Reader)
	} else {
		_, plainErr = dockerCopy.StdCopy(stdOut, stdErr, attachResp.Reader)
	}

	if plainErr != nil {
		execLog.Error(plainErr, "exception happened in read routine")
	}

	return nil
}

func (r *dockerRuntime) AttachContainer(podUID, container string, stdin io.Reader, stdout, stderr io.WriteCloser, resizeCh <-chan *connectivity.TtyResizeOptions) *connectivity.Error {
	attachLog := r.Log().WithValues("action", "attach", "uid", podUID, "container", container)

	attachLog.Info("trying to find container")
	ctr, err := r.findContainer(podUID, container)
	if err != nil {
		attachLog.Error(err, "failed to find container")
		return runtimeutil.ErrNotFound
	}

	attachCtx, cancelAttach := r.ActionContext()
	defer cancelAttach()

	var plainErr error
	attachLog.Info("trying to attach")
	resp, plainErr := r.runtimeClient.ContainerAttach(attachCtx, ctr.ID, dockerType.ContainerAttachOptions{
		Stream: true,
		Stdin:  stdin != nil,
		Stdout: stdout != nil,
		Stderr: stderr != nil,
	})
	if plainErr != nil {
		attachLog.Error(plainErr, "failed to attach")
		return connectivity.NewCommonError(plainErr.Error())
	}
	defer func() { _ = resp.Conn.Close() }()

	if stdin != nil {
		go func() {
			attachLog.Info("starting write routine")
			defer attachLog.Info("finished write routine")

			_, err := io.Copy(resp.Conn, stdin)
			if err != nil {
				attachLog.Error(err, "exception happened in write routine")
			}
		}()
	}

	go func() {
		attachLog.Info("starting tty resize routine")
		defer attachLog.Info("finished tty resize routine")

		for {
			select {
			case size, more := <-resizeCh:
				if !more {
					return
				}

				err := r.runtimeClient.ContainerResize(attachCtx, ctr.ID, dockerType.ResizeOptions{
					Height: uint(size.GetRows()),
					Width:  uint(size.GetCols()),
				})
				if err != nil {
					attachLog.Error(err, "exception happened in tty resize routine")
				}
			case <-attachCtx.Done():
				return
			}
		}
	}()

	attachLog.Info("starting read routine")
	defer attachLog.Info("finished read routine")

	if stderr != nil {
		_, plainErr = dockerCopy.StdCopy(stdout, stderr, resp.Reader)
	} else {
		_, plainErr = io.Copy(stdout, resp.Reader)
	}
	if plainErr != nil {
		attachLog.Error(plainErr, "exception happened in read routine")
	}

	return nil
}

func (r *dockerRuntime) GetContainerLogs(podUID string, options *connectivity.LogOptions, stdout, stderr io.WriteCloser) *connectivity.Error {
	logLog := r.Log().WithValues("action", "log", "uid", podUID, "stdout", stdout != nil, "stderr", stderr != nil, "options", options)

	logLog.Info("trying to find container")
	ctr, err := r.findContainer(podUID, options.Container)
	if err != nil {
		logLog.Error(err, "failed to find container")
		return runtimeutil.ErrNotFound
	}

	logCtx, cancelLog := r.ActionContext()
	defer cancelLog()

	var since, tail string
	if options.Since > 0 {
		since = time.Unix(0, options.Since).Format(time.RFC3339Nano)
	}

	if options.TailLines > 0 {
		tail = strconv.FormatInt(options.TailLines, 10)
	}

	var plainErr error
	logReader, plainErr := r.runtimeClient.ContainerLogs(logCtx, ctr.ID, dockerType.ContainerLogsOptions{
		ShowStdout: stdout != nil,
		ShowStderr: stderr != nil,
		Since:      since,
		Timestamps: options.Timestamp,
		Follow:     options.Follow,
		Tail:       tail,
		Details:    false,
	})
	if plainErr != nil {
		logLog.Error(plainErr, "failed to read container logs")
		return connectivity.NewCommonError(plainErr.Error())
	}

	_, plainErr = dockerCopy.StdCopy(stdout, stderr, logReader)
	if plainErr != nil {
		logLog.Error(plainErr, "exception happened in logs")
		return connectivity.NewCommonError(plainErr.Error())
	}

	return nil
}

func (r *dockerRuntime) PortForward(podUID string, protocol string, port int32, in io.Reader, out io.WriteCloser) *connectivity.Error {
	pfLog := r.Log().WithValues("action", "portforward", "proto", protocol, "port", port, "uid", podUID)

	pfLog.Info("trying to find pause container")
	pauseCtr, err := r.findContainer(podUID, constant.ContainerNamePause)
	if err != nil {
		pfLog.Error(err, "failed to find pause container")
		return runtimeutil.ErrNotFound
	}

	if pauseCtr.NetworkSettings == nil {
		pfLog.Info("pause container network settings empty")
		return connectivity.NewCommonError("pause container network settings empty")
	}

	address := ""
	for name, network := range pauseCtr.NetworkSettings.Networks {
		if name == "bridge" {
			address = network.IPAddress
			break
		}
	}

	if address == "" {
		pfLog.Info("bridge ip address not found")
		return connectivity.NewCommonError("bridge ip address not found")
	}

	// TODO: evaluate more efficient way to get traffic redirected
	var plainErr error
	conn, plainErr := net.Dial(protocol, fmt.Sprintf("%s:%s", address, strconv.FormatInt(int64(port), 10)))
	if plainErr != nil {
		pfLog.Error(plainErr, "failed to dial target")
		return connectivity.NewCommonError(plainErr.Error())
	}
	defer func() { _ = conn.Close() }()

	go func() {
		pfLog.Info("starting write routine")
		defer pfLog.Info("finished write routine")

		if _, err := io.Copy(conn, in); err != nil {
			pfLog.Error(err, "exception happened in write routine")
			return
		}
	}()

	pfLog.Info("starting read routine")
	defer pfLog.Info("finished write routine")

	if _, err := io.Copy(out, conn); err != nil {
		pfLog.Error(err, "exception happened in read routine")
	}

	return nil
}
