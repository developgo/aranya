// +build !linux

package agent

import (
	corev1 "k8s.io/api/core/v1"
)

func systemInfo() *corev1.NodeSystemInfo {
	return &corev1.NodeSystemInfo{}
}
