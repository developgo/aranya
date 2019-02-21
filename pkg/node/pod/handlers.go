package pod

import (
	"io"
	"net/http"
	"strings"

	"k8s.io/kubernetes/pkg/kubelet/server/portforward"
	kubeletrc "k8s.io/kubernetes/pkg/kubelet/server/remotecommand"

	"arhat.dev/aranya/pkg/node/util"
)

func (m *Manager) HandleNodeLog(w http.ResponseWriter, r *http.Request) {
	log.Info("HandlePodContainerLog")
}

// HandlePodContainerLog
// GET /containerLogs/{namespace}/{pod}/{container}
func (m *Manager) HandlePodContainerLog(w http.ResponseWriter, r *http.Request) {
	log.Info("HandlePodContainerLog")

	namespace, podID, container, opt, err := util.GetParamsForContainerLog(r)
	if err != nil {
		log.Error(err, "parse container log options failed")
		return
	}

	logReader, err := m.GetContainerLogs(namespace, podID, container, opt)
	if err != nil {
		log.Error(err, "Get container log failed", "Pod.Namespace", namespace, "Pod.Name", podID, "Container.Name", container)
		return
	}

	// read until EOF (err = nil)
	if _, err := io.Copy(w, logReader); err != nil {
		log.Error(err, "Send container log response failed")
		return
	}
}

// HandlePodExec
func (m *Manager) HandlePodExec(w http.ResponseWriter, r *http.Request) {
	log.Info("HandlePodExec")

	namespace, podID, uid, containerName, cmd := util.GetParamsForExec(r)

	kubeletrc.ServeExec(
		// http context
		w, r,
		// edge pod executor provided by Manager (implements ExecInContainer)
		m,
		// namespaced pod name
		util.GetFullPodName(namespace, podID),
		// unique id of pod
		uid,
		// container to execute in
		containerName,
		// commands to execute
		cmd,
		// stream options
		util.NewRemoteCommandOptions(r),
		// timeout options
		idleTimeout, streamCreationTimeout,
		// supported protocols
		strings.Split(r.Header.Get("X-Stream-Protocol-Version"), ","))
}

// HandlePodAttach
func (m *Manager) HandlePodAttach(w http.ResponseWriter, r *http.Request) {
	log.Info("HandlePodAttach")
	namespace, podID, uid, containerName, _ := util.GetParamsForExec(r)
	kubeletrc.ServeAttach(
		// http context
		w, r,
		// edge pod executor provided by Manager (implements ExecInContainer)
		m,
		// namespaced pod name
		util.GetFullPodName(namespace, podID),
		// unique id of pod
		uid,
		// container to execute in
		containerName,
		// stream options
		util.NewRemoteCommandOptions(r),
		// timeout options
		idleTimeout, streamCreationTimeout,
		// supported protocols
		strings.Split(r.Header.Get("X-Stream-Protocol-Version"), ","))
}

func (m *Manager) HandlePodPortForward(w http.ResponseWriter, r *http.Request) {
	log.Info("HandlePodAttach")
	namespace, podID, uid := util.GetParamsForPortForward(r)

	portForwardOptions, err := portforward.NewV4Options(r)
	if err != nil {
		log.Error(err, "parse portforward options failed")
		return
	}

	portforward.ServePortForward(
		// http context
		w, r,
		// edge pod executor provided by Manager (implements ExecInContainer)
		m,
		// namespaced pod name
		util.GetFullPodName(namespace, podID),
		// unique id of pod
		uid,
		// port forward options (ports)
		portForwardOptions,
		// timeout options
		idleTimeout, streamCreationTimeout,
		// supported protocols
		strings.Split(r.Header.Get("X-Stream-Protocol-Version"), ","))
}
