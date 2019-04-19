package pod

import (
	"io"
	"net/http"
	"strings"

	"k8s.io/apimachinery/pkg/types"
	kubeletportforward "k8s.io/kubernetes/pkg/kubelet/server/portforward"
	kubeletremotecommand "k8s.io/kubernetes/pkg/kubelet/server/remotecommand"

	"arhat.dev/aranya/pkg/constant"
	"arhat.dev/aranya/pkg/node/util"
)

func (m *Manager) getPodUIDInCache(namespace, name string, podUID types.UID) types.UID {
	if podUID != "" {
		return podUID
	}

	pod, ok := m.podCache.GetByName(namespace, name)
	if ok {
		return pod.UID
	}
	return ""
}

// HandlePodContainerLog
func (m *Manager) HandlePodContainerLog(w http.ResponseWriter, r *http.Request) {
	namespace, podName, opt, err := util.GetParamsForContainerLog(r)
	if err != nil {
		log.Error(err, "parse container log options failed")

		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	podUID := m.getPodUIDInCache(namespace, podName, "")
	if podUID == "" {
		http.Error(w, "target pod not found", http.StatusBadRequest)
		return
	}

	logReader, err := m.getContainerLogs(podUID, opt)
	if err != nil {
		log.Error(err, "failed to get container logs")
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	defer func() { _ = logReader.Close() }()

	// read until EOF (err = nil)
	w.WriteHeader(http.StatusOK)
	if _, err := io.Copy(w, logReader); err != nil {
		log.Error(err, "failed to send container log response")
		return
	}
}

// HandlePodExec
func (m *Manager) HandlePodExec(w http.ResponseWriter, r *http.Request) {
	namespace, podName, uid, containerName, cmd := util.GetParamsForExec(r)
	podUID := m.getPodUIDInCache(namespace, podName, uid)
	if podUID == "" {
		http.Error(w, "target pod not found", http.StatusBadRequest)
		return
	}

	streamOptions := util.NewRemoteCommandOptions(r)

	errCh := make(chan error, 1)
	kubeletremotecommand.ServeExec(
		// http context
		w, r,
		// wrapped pod executor
		m.handleExecInContainer(errCh),
		// namespaced pod name
		"", // unused
		// unique id of pod
		podUID,
		// container to execute in
		containerName,
		// commands to execute
		cmd,
		// stream options
		streamOptions,
		// timeout options
		constant.DefaultStreamIdleTimeout, constant.DefaultStreamCreationTimeout,
		// supported protocols
		strings.Split(r.Header.Get("X-Stream-Protocol-Version"), ","))

	for err := range errCh {
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
		}
	}
}

// HandlePodAttach
func (m *Manager) HandlePodAttach(w http.ResponseWriter, r *http.Request) {
	namespace, podName, uid, containerName, _ := util.GetParamsForExec(r)
	podUID := m.getPodUIDInCache(namespace, podName, uid)
	if podUID == "" {
		http.Error(w, "target pod not found", http.StatusBadRequest)
		return
	}

	streamOptions := util.NewRemoteCommandOptions(r)

	errCh := make(chan error, 1)
	kubeletremotecommand.ServeAttach(
		// http context
		w, r,
		// wrapped pod attacher
		m.handleAttachContainer(errCh),
		// namespaced pod name (not used)
		"", // unused
		// unique id of pod
		podUID,
		// container to execute in
		containerName,
		// stream options
		streamOptions,
		// timeout options
		constant.DefaultStreamIdleTimeout, constant.DefaultStreamCreationTimeout,
		// supported protocols
		strings.Split(r.Header.Get("X-Stream-Protocol-Version"), ","))

	for err := range errCh {
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
		}
	}
}

// HandlePodPortForward
func (m *Manager) HandlePodPortForward(w http.ResponseWriter, r *http.Request) {
	namespace, podName, uid := util.GetParamsForPortForward(r)
	podUID := m.getPodUIDInCache(namespace, podName, uid)
	if podUID == "" {
		http.Error(w, "target pod not found", http.StatusBadRequest)
		return
	}

	portForwardOptions, err := kubeletportforward.NewV4Options(r)
	if err != nil {
		log.Error(err, "parse portforward options failed")
		return
	}

	errCh := make(chan error, 1)
	kubeletportforward.ServePortForward(
		// http context
		w, r,
		// wrapped pod port forwarder
		m.handlePortForward(errCh),
		// namespaced pod name (not used)
		"",
		// unique id of pod
		podUID,
		// port forward options (ports)
		portForwardOptions,
		// timeout options
		constant.DefaultStreamIdleTimeout, constant.DefaultStreamCreationTimeout,
		// supported protocols
		strings.Split(r.Header.Get("X-Stream-Protocol-Version"), ","))

	for err := range errCh {
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
		}
	}
}
