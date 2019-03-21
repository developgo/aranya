package podman

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
	"syscall"

	libpodRuntime "github.com/containers/libpod/libpod"
	libpodImage "github.com/containers/libpod/libpod/image"
	libpodNS "github.com/containers/libpod/pkg/namespaces"
	libpodRootless "github.com/containers/libpod/pkg/rootless"
	libpodSpec "github.com/containers/libpod/pkg/spec"
	"github.com/containers/storage"
	"github.com/cri-o/ocicni/pkg/ocicni"
	ociRuntimeSpec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
)

func defaultPodCreateOptions(namespace, name string, podSpec *corev1.PodSpec) []libpodRuntime.PodCreateOption {
	var portmap []ocicni.PortMapping
	for _, ctr := range podSpec.Containers {
		for _, p := range ctr.Ports {
			pm := ocicni.PortMapping{
				HostPort:      p.HostPort,
				ContainerPort: p.ContainerPort,
				Protocol:      strings.ToLower(string(p.Protocol)),
			}
			if p.HostIP != "" {
				logrus.Debug("HostIP on port bindings is not supported")
			}
			portmap = append(portmap, pm)
		}
	}

	return []libpodRuntime.PodCreateOption{
		// pod metadata
		libpodRuntime.WithPodNamespace(namespace),
		libpodRuntime.WithPodName(name),
		// TODO: add pod labels
		// libpodRuntime.WithPodLabels(podSpec.Labels),
		// with `pause` container
		libpodRuntime.WithInfraContainer(),
		// claim ports
		libpodRuntime.WithInfraContainerPorts(portmap),
		// share namespaces (cgroup,ipc,net,uts)
		libpodRuntime.WithPodCgroups(),
		libpodRuntime.WithPodIPC(),
		libpodRuntime.WithPodNet(),
		libpodRuntime.WithPodUTS(),
	}
}

func kubeContainerToCreateConfig(podSpec *corev1.PodSpec, apiCtr *corev1.Container, runtime *libpodRuntime.Runtime, newImage *libpodImage.Image, namespaces map[string]string) *libpodSpec.CreateConfig {
	envs := make(map[string]string)
	for _, e := range apiCtr.Env {
		envs[e.Name] = e.Value
	}

	var mounts []ociRuntimeSpec.Mount
	for _, m := range apiCtr.VolumeMounts {
		mounts = append(mounts, ociRuntimeSpec.Mount{
			Destination: m.MountPath,
		})
	}

	secCtx := determineEffectiveSecurityContext(podSpec, apiCtr)
	return &libpodSpec.CreateConfig{
		Runtime: runtime,
		Image:   apiCtr.Image,
		ImageID: newImage.ID(),
		Name:    apiCtr.Name,
		Tty:     apiCtr.TTY,
		WorkDir: apiCtr.WorkingDir,

		// security opts
		ReadOnlyRootfs: *secCtx.ReadOnlyRootFilesystem,
		Privileged:     *secCtx.Privileged,
		NoNewPrivs:     !*secCtx.AllowPrivilegeEscalation,

		Env:        envs,
		Entrypoint: apiCtr.Command,
		Command:    apiCtr.Args,
		StopSignal: syscall.SIGTERM,
		Mounts:     []ociRuntimeSpec.Mount{},

		IDMappings: &storage.IDMappingOptions{},

		NetMode:    libpodNS.NetworkMode(namespaces["net"]),
		IpcMode:    libpodNS.IpcMode(namespaces["ipc"]),
		UtsMode:    libpodNS.UTSMode(namespaces["uts"]),
		UsernsMode: libpodNS.UsernsMode(namespaces["user"]),
	}
}

func createContainerFromCreateConfig(r *libpodRuntime.Runtime, createConfig *libpodSpec.CreateConfig, ctx context.Context, pod *libpodRuntime.Pod) (*libpodRuntime.Container, error) {
	runtimeSpec, err := libpodSpec.CreateConfigToOCISpec(createConfig)
	if err != nil {
		return nil, err
	}
	// add post stop hook to get notified
	if runtimeSpec.Hooks == nil {
		runtimeSpec.Hooks = &ociRuntimeSpec.Hooks{}
	}

	// point to self
	runtimeSpec.Hooks.Poststop = append(runtimeSpec.Hooks.Poststop, ociRuntimeSpec.Hook{
		Path: os.Args[0],
		Args: os.Args[1:],
		Env:  append(os.Environ(), "MODE=on-container-post-stop"),
	})

	options, err := createConfig.GetContainerCreateOptions(r, pod)
	if err != nil {
		return nil, err
	}

	became, ret, err := joinOrCreateRootlessUserNamespace(createConfig, r)
	if err != nil {
		return nil, err
	}

	if became {
		os.Exit(ret)
	}

	ctr, err := r.NewContainer(ctx, runtimeSpec, options...)
	if err != nil {
		return nil, err
	}

	createConfigJSON, err := json.Marshal(createConfig)
	if err != nil {
		return nil, err
	}

	if err := ctr.AddArtifact("create-config", createConfigJSON); err != nil {
		return nil, err
	}
	return ctr, nil
}

func joinOrCreateRootlessUserNamespace(createConfig *libpodSpec.CreateConfig, runtime *libpodRuntime.Runtime) (bool, int, error) {
	if os.Geteuid() == 0 {
		return false, 0, nil
	}

	if createConfig.Pod != "" {
		pod, err := runtime.LookupPod(createConfig.Pod)
		if err != nil {
			return false, -1, err
		}
		inspect, err := pod.Inspect()
		for _, ctr := range inspect.Containers {
			prevCtr, err := runtime.LookupContainer(ctr.ID)
			if err != nil {
				return false, -1, err
			}
			s, err := prevCtr.State()
			if err != nil {
				return false, -1, err
			}
			if s != libpodRuntime.ContainerStateRunning && s != libpodRuntime.ContainerStatePaused {
				continue
			}
			data, err := ioutil.ReadFile(prevCtr.Config().ConmonPidFile)
			if err != nil {
				return false, -1, errors.Wrapf(err, "cannot read conmon PID file %q", prevCtr.Config().ConmonPidFile)
			}
			conmonPid, err := strconv.Atoi(string(data))
			if err != nil {
				return false, -1, errors.Wrapf(err, "cannot parse PID %q", data)
			}
			return libpodRootless.JoinDirectUserAndMountNS(uint(conmonPid))
		}
	}

	namespacesStr := []string{string(createConfig.IpcMode), string(createConfig.NetMode), string(createConfig.UsernsMode), string(createConfig.PidMode), string(createConfig.UtsMode)}
	for _, i := range namespacesStr {
		if libpodSpec.IsNS(i) {
			return libpodRootless.JoinNSPath(libpodSpec.NS(i))
		}
	}

	type namespace interface {
		IsContainer() bool
		Container() string
	}
	namespaces := []namespace{createConfig.IpcMode, createConfig.NetMode, createConfig.UsernsMode, createConfig.PidMode, createConfig.UtsMode}
	for _, i := range namespaces {
		if i.IsContainer() {
			ctr, err := runtime.LookupContainer(i.Container())
			if err != nil {
				return false, -1, err
			}
			pid, err := ctr.PID()
			if err != nil {
				return false, -1, err
			}
			if pid == 0 {
				if createConfig.Pod != "" {
					continue
				}
				return false, -1, errors.Errorf("dependency container %s is not running", ctr.ID())
			}
			return libpodRootless.JoinNS(uint(pid))
		}
	}
	return libpodRootless.BecomeRootInUserNS()
}

func determineEffectiveSecurityContext(podSpec *corev1.PodSpec, container *corev1.Container) *corev1.SecurityContext {
	effectiveSc := securityContextFromPodSecurityContext(podSpec)
	containerSc := container.SecurityContext

	if effectiveSc == nil && containerSc == nil {
		return &corev1.SecurityContext{}
	}
	if effectiveSc != nil && containerSc == nil {
		return effectiveSc
	}
	if effectiveSc == nil && containerSc != nil {
		return containerSc
	}

	if containerSc.SELinuxOptions != nil {
		effectiveSc.SELinuxOptions = new(corev1.SELinuxOptions)
		*effectiveSc.SELinuxOptions = *containerSc.SELinuxOptions
	}

	if containerSc.Capabilities != nil {
		effectiveSc.Capabilities = new(corev1.Capabilities)
		*effectiveSc.Capabilities = *containerSc.Capabilities
	}

	if containerSc.Privileged != nil {
		effectiveSc.Privileged = new(bool)
		*effectiveSc.Privileged = *containerSc.Privileged
	}

	if containerSc.RunAsUser != nil {
		effectiveSc.RunAsUser = new(int64)
		*effectiveSc.RunAsUser = *containerSc.RunAsUser
	}

	if containerSc.RunAsGroup != nil {
		effectiveSc.RunAsGroup = new(int64)
		*effectiveSc.RunAsGroup = *containerSc.RunAsGroup
	}

	if containerSc.RunAsNonRoot != nil {
		effectiveSc.RunAsNonRoot = new(bool)
		*effectiveSc.RunAsNonRoot = *containerSc.RunAsNonRoot
	}

	if containerSc.ReadOnlyRootFilesystem != nil {
		effectiveSc.ReadOnlyRootFilesystem = new(bool)
		*effectiveSc.ReadOnlyRootFilesystem = *containerSc.ReadOnlyRootFilesystem
	}

	if containerSc.AllowPrivilegeEscalation != nil {
		effectiveSc.AllowPrivilegeEscalation = new(bool)
		*effectiveSc.AllowPrivilegeEscalation = *containerSc.AllowPrivilegeEscalation
	}

	if containerSc.ProcMount != nil {
		effectiveSc.ProcMount = new(corev1.ProcMountType)
		*effectiveSc.ProcMount = *containerSc.ProcMount
	}

	return effectiveSc
}

func securityContextFromPodSecurityContext(podSpec *corev1.PodSpec) *corev1.SecurityContext {
	if podSpec.SecurityContext == nil {
		return nil
	}

	synthesized := &corev1.SecurityContext{}

	if podSpec.SecurityContext.SELinuxOptions != nil {
		synthesized.SELinuxOptions = &corev1.SELinuxOptions{}
		*synthesized.SELinuxOptions = *podSpec.SecurityContext.SELinuxOptions
	}
	if podSpec.SecurityContext.RunAsUser != nil {
		synthesized.RunAsUser = new(int64)
		*synthesized.RunAsUser = *podSpec.SecurityContext.RunAsUser
	}

	if podSpec.SecurityContext.RunAsGroup != nil {
		synthesized.RunAsGroup = new(int64)
		*synthesized.RunAsGroup = *podSpec.SecurityContext.RunAsGroup
	}

	if podSpec.SecurityContext.RunAsNonRoot != nil {
		synthesized.RunAsNonRoot = new(bool)
		*synthesized.RunAsNonRoot = *podSpec.SecurityContext.RunAsNonRoot
	}

	return synthesized
}
