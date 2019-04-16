package pod

import (
	"io"
	"net/http"
	"strings"

	"k8s.io/apimachinery/pkg/types"
	kubeletportforward "k8s.io/kubernetes/pkg/kubelet/server/portforward"
	kubeletremotecommand "k8s.io/kubernetes/pkg/kubelet/server/remotecommand"

	"arhat.dev/aranya/pkg/node/util"
)

func (m *Manager) getPodUIDInCache(namespace, name string, podUID types.UID) types.UID {
	if podUID != "" {
		return podUID
	} else {
		pod, _ := m.podCache.GetByName(namespace, name)
		return pod.UID
	}
}

// HandlePodContainerLog
func (m *Manager) HandlePodContainerLog(w http.ResponseWriter, r *http.Request) {
	log.Info("HandlePodContainerLog")

	namespace, podName, podUID, container, opt, err := util.GetParamsForContainerLog(r)
	if err != nil {
		log.Error(err, "parse container log options failed")
		return
	}

	logReader, err := m.GetContainerLogs(m.getPodUIDInCache(namespace, podName, podUID), container, opt)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		log.Error(err, "failed to get container logs")
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
	log.Info("HandlePodExec")

	namespace, podName, uid, containerName, cmd := util.GetParamsForExec(r)
	streamOptions := util.NewRemoteCommandOptions(r)

	kubeletremotecommand.ServeExec(
		// http context
		w, r,
		// edge pod executor provided by Manager (implements ExecInContainer)
		kubeletremotecommand.Executor(m),
		// namespaced pod name
		"", // unused
		// unique id of pod
		m.getPodUIDInCache(namespace, podName, uid),
		// container to execute in
		containerName,
		// commands to execute
		cmd,
		// stream options
		streamOptions,
		// timeout options
		idleTimeout, streamCreationTimeout,
		// supported protocols
		strings.Split(r.Header.Get("X-Stream-Protocol-Version"), ","))
}

// HandlePodAttach
func (m *Manager) HandlePodAttach(w http.ResponseWriter, r *http.Request) {
	log.Info("HandlePodAttach")
	namespace, podName, uid, containerName, _ := util.GetParamsForExec(r)
	streamOptions := util.NewRemoteCommandOptions(r)

	kubeletremotecommand.ServeAttach(
		// http context
		w, r,
		// edge pod executor provided by Manager (implements ExecInContainer)
		kubeletremotecommand.Attacher(m),
		// namespaced pod name (not used)
		"", // unused
		// unique id of pod
		m.getPodUIDInCache(namespace, podName, uid),
		// container to execute in
		containerName,
		// stream options
		streamOptions,
		// timeout options
		idleTimeout, streamCreationTimeout,
		// supported protocols
		strings.Split(r.Header.Get("X-Stream-Protocol-Version"), ","))
}

// HandlePodPortForward
func (m *Manager) HandlePodPortForward(w http.ResponseWriter, r *http.Request) {
	log.Info("HandlePodAttach")
	namespace, podName, uid := util.GetParamsForPortForward(r)

	portForwardOptions, err := kubeletportforward.NewV4Options(r)
	if err != nil {
		log.Error(err, "parse portforward options failed")
		return
	}

	kubeletportforward.ServePortForward(
		// http context
		w, r,
		// edge pod executor provided by Manager (implements ExecInContainer)
		kubeletportforward.PortForwarder(m),
		// namespaced pod name (not used)
		"",
		// unique id of pod
		m.getPodUIDInCache(namespace, podName, uid),
		// port forward options (ports)
		portForwardOptions,
		// timeout options
		idleTimeout, streamCreationTimeout,
		// supported protocols
		strings.Split(r.Header.Get("X-Stream-Protocol-Version"), ","))
}
