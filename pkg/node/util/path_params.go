package util

import (
	"errors"
	"net/http"

	"github.com/gorilla/mux"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
	"k8s.io/kubernetes/pkg/apis/core/v1/validation"
)

const (
	PathParamNamespace = "namespace"
	PathParamPodName   = "name"
	PathParamPodUID    = "uid"
	PathParamContainer = "container"
)

func GetParamsForExec(req *http.Request) (namespace, podName string, uid types.UID, containerName string, command []string) {
	pathVars := mux.Vars(req)
	return pathVars[PathParamNamespace], pathVars[PathParamPodName], types.UID(pathVars[PathParamPodUID]),
		pathVars[PathParamContainer], req.URL.Query()[corev1.ExecCommandParam]
}

func GetParamsForPortForward(req *http.Request) (namespace, podName string, uid types.UID) {
	pathVars := mux.Vars(req)
	return pathVars[PathParamNamespace], pathVars[PathParamPodName], types.UID(pathVars[PathParamPodUID])
}

func GetParamsForContainerLog(req *http.Request) (namespace, podName string, logOptions *corev1.PodLogOptions, err error) {
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
