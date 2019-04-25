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

package pod

import (
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/gorilla/mux"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
	"k8s.io/kubernetes/pkg/apis/core/v1/validation"
	kubeletportforward "k8s.io/kubernetes/pkg/kubelet/server/portforward"
	kubeletremotecommand "k8s.io/kubernetes/pkg/kubelet/server/remotecommand"

	"arhat.dev/aranya/pkg/constant"
)

const (
	PathParamNamespace = "namespace"
	PathParamPodName   = "name"
	PathParamPodUID    = "uid"
	PathParamContainer = "container"
)

func getParamsForExec(req *http.Request) (namespace, podName string, uid types.UID, containerName string, command []string) {
	pathVars := mux.Vars(req)
	return pathVars[PathParamNamespace], pathVars[PathParamPodName], types.UID(pathVars[PathParamPodUID]),
		pathVars[PathParamContainer], req.URL.Query()[corev1.ExecCommandParam]
}

func getParamsForPortForward(req *http.Request) (namespace, podName string, uid types.UID) {
	pathVars := mux.Vars(req)
	return pathVars[PathParamNamespace], pathVars[PathParamPodName], types.UID(pathVars[PathParamPodUID])
}

func getParamsForContainerLog(req *http.Request) (namespace, podName string, logOptions *corev1.PodLogOptions, err error) {
	pathVars := mux.Vars(req)

	namespace = pathVars[PathParamNamespace]
	if namespace == "" {
		err = errors.New("missing namespace")
		return
	}

	podName = pathVars[PathParamPodName]
	if podName == "" {
		err = errors.New("missing pod name")
		return
	}

	containerName := pathVars[PathParamContainer]
	if containerName == "" {
		err = errors.New("missing container name")
		return
	}

	query := req.URL.Query()
	// backwards compatibility for the "tail" query parameter
	if tail := req.FormValue("tail"); len(tail) > 0 {
		query["tailLines"] = []string{tail}
		// "all" is the same as omitting tail
		if tail == "all" {
			delete(query, "tailLines")
		}
	}
	query.Get("tailLines")

	// container logs on the kubelet are locked to the v1 API version of PodLogOptions
	logOptions = &corev1.PodLogOptions{}
	if err = legacyscheme.ParameterCodec.DecodeParameters(query, corev1.SchemeGroupVersion, logOptions); err != nil {
		return
	}

	logOptions.TypeMeta = metav1.TypeMeta{}
	if errs := validation.ValidatePodLogOptions(logOptions); len(errs) > 0 {
		err = errors.New("invalid request")
		return
	}

	logOptions.Container = containerName
	return
}

func getRemoteCommandOptions(req *http.Request) *kubeletremotecommand.Options {
	return &kubeletremotecommand.Options{
		TTY:    req.FormValue(corev1.ExecTTYParam) == "1",
		Stdin:  req.FormValue(corev1.ExecStdinParam) == "1",
		Stdout: req.FormValue(corev1.ExecStdoutParam) == "1",
		Stderr: req.FormValue(corev1.ExecStderrParam) == "1",
	}
}

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
	httpLog := m.log.WithValues("type", "http", "action", "log")

	namespace, podName, opt, err := getParamsForContainerLog(r)
	if err != nil {
		httpLog.Error(err, "parse container log options failed")
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	podUID := m.getPodUIDInCache(namespace, podName, "")
	if podUID == "" {
		httpLog.Info("pod not found for log", "podUID", podUID)
		http.Error(w, "target pod not found", http.StatusNotFound)
		return
	}

	logReader, err := m.doGetContainerLogs(podUID, opt)
	if err != nil {
		httpLog.Error(err, "failed to get container logs")
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	defer func() { _ = logReader.Close() }()

	// read until EOF (err = nil)
	w.WriteHeader(http.StatusOK)
	if _, err := io.Copy(w, logReader); err != nil {
		httpLog.Error(err, "failed to send container log response")
		return
	}
}

// HandlePodExec
func (m *Manager) HandlePodExec(w http.ResponseWriter, r *http.Request) {
	httpLog := m.log.WithValues("type", "http", "action", "exec")

	namespace, podName, uid, containerName, cmd := getParamsForExec(r)
	podUID := m.getPodUIDInCache(namespace, podName, uid)
	if podUID == "" {
		httpLog.Info("pod not found for exec", "podUID", podUID)
		http.Error(w, "target pod not found", http.StatusNotFound)
		return
	}

	httpLog.Info("starting to serve exec")
	kubeletremotecommand.ServeExec(
		w, r, /* http context */
		m.doHandleExecInContainer(), /* wrapped pod executor */
		"",                          /* pod name (unused) */
		podUID,                      /* unique id of pod */
		containerName,               /* container name to execute in*/
		cmd,                         /* commands to execute */
		getRemoteCommandOptions(r),  /* stream options */
		// timeout options
		constant.DefaultStreamIdleTimeout, constant.DefaultStreamCreationTimeout,
		// supported protocols
		strings.Split(r.Header.Get("X-Stream-Protocol-Version"), ","))
}

// HandlePodAttach
func (m *Manager) HandlePodAttach(w http.ResponseWriter, r *http.Request) {
	httpLog := m.log.WithValues("type", "http", "action", "attach")

	namespace, podName, uid, containerName, _ := getParamsForExec(r)
	podUID := m.getPodUIDInCache(namespace, podName, uid)
	if podUID == "" {
		httpLog.Info("pod not found for attach", "podUID", podUID)
		http.Error(w, "target pod not found", http.StatusNotFound)
		return
	}

	httpLog.Info("starting to serve attach")
	kubeletremotecommand.ServeAttach(
		w, r, /* http context */
		m.doHandleAttachContainer(), /* wrapped pod attacher */
		"",                          /* pod name (not used) */
		podUID,                      /* unique id of pod */
		containerName,               /* container to execute in */
		getRemoteCommandOptions(r),  /* stream options */
		// timeout options
		constant.DefaultStreamIdleTimeout, constant.DefaultStreamCreationTimeout,
		// supported protocols
		strings.Split(r.Header.Get("X-Stream-Protocol-Version"), ","))
}

// HandlePodPortForward
func (m *Manager) HandlePodPortForward(w http.ResponseWriter, r *http.Request) {
	httpLog := m.log.WithValues("type", "http", "action", "portforward")

	namespace, podName, uid := getParamsForPortForward(r)

	httpLog.Info("trying to get portforward options")
	portForwardOptions, err := kubeletportforward.NewV4Options(r)
	if err != nil {
		httpLog.Error(err, "failed to parse portforward options")
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	podUID := m.getPodUIDInCache(namespace, podName, uid)
	if podUID == "" {
		httpLog.Info("pod not found for port forward", "podUID", podUID)
		http.Error(w, "target pod not found", http.StatusNotFound)
		return
	}

	// build port protocol map
	pod, ok := m.podCache.GetByID(podUID)
	if !ok {
		httpLog.Info("pod not found for port forward", "podUID", podUID)
		http.Error(w, "target pod not found", http.StatusNotFound)
		return
	}

	portProto := make(map[int32]string)
	for _, port := range portForwardOptions.Ports {
		// defaults to tcp
		portProto[port] = "tcp"
	}
	for _, ctr := range pod.Spec.Containers {
		for _, ctrPort := range ctr.Ports {
			portProto[ctrPort.ContainerPort] = strings.ToLower(string(ctrPort.Protocol))
		}
	}

	httpLog.Info("starting to serve port forward")
	kubeletportforward.ServePortForward(
		w, r, /* http context */
		m.doHandlePortForward(portProto), /* wrapped pod port forwarder */
		"",                               /* pod name (not used) */
		podUID,                           /* unique id of pod */
		portForwardOptions,               /* port forward options (ports) */
		// timeout options
		constant.DefaultStreamIdleTimeout, constant.DefaultStreamCreationTimeout,
		// supported protocols
		strings.Split(r.Header.Get("X-Stream-Protocol-Version"), ","))
}
