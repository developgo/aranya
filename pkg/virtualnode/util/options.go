package util

import (
	"net/http"

	corev1 "k8s.io/api/core/v1"
	kubeletrc "k8s.io/kubernetes/pkg/kubelet/server/remotecommand"
)

func NewRemoteCommandOptions(req *http.Request) *kubeletrc.Options {
	return &kubeletrc.Options{
		TTY:    req.FormValue(corev1.ExecTTYParam) == "1",
		Stdin:  req.FormValue(corev1.ExecStdinParam) == "1",
		Stdout: req.FormValue(corev1.ExecStdoutParam) == "1",
		Stderr: req.FormValue(corev1.ExecStderrParam) == "1",
	}
}
