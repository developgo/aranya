package util

import (
	"github.com/gorilla/mux"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
	"k8s.io/kubernetes/pkg/apis/core/v1/validation"
	"net/http"
)

const (
	PathParamNamespace = "namespace"
	PathParamPodID     = "podID"
	PathParamUID       = "uid"
	PathParamContainer = "containerName"
)

func GetParamsForExec(req *http.Request) (namespace, podID string, uid types.UID, containerName string, command []string) {
	pathVars := mux.Vars(req)
	return pathVars[PathParamNamespace], pathVars[PathParamPodID], types.UID(pathVars[PathParamUID]),
		pathVars[PathParamContainer], req.URL.Query()[corev1.ExecCommandParam]
}

func GetParamsForPortForward(req *http.Request) (namespace, podID string, uid types.UID) {
	pathVars := mux.Vars(req)
	return pathVars[PathParamNamespace], pathVars[PathParamPodID], types.UID(pathVars[PathParamUID])
}

func GetParamsForContainerLog(req *http.Request) (namespace, podID, containerName string, opt *corev1.PodLogOptions, err error) {
	pathVars := mux.Vars(req)

	query := req.URL.Query()
	// legacy
	if t := req.FormValue("tail"); t != "" {
		query["tailLines"] = []string{t}
		// "all" is the same as omitting tail
		if t == "all" {
			delete(query, "tailLines")
		}
	}

	logOptions := &corev1.PodLogOptions{}
	if err = legacyscheme.ParameterCodec.DecodeParameters(query, corev1.SchemeGroupVersion, logOptions); err != nil {
		return
	}

	logOptions.TypeMeta = metav1.TypeMeta{}
	if errs := validation.ValidatePodLogOptions(logOptions); len(errs) > 0 {
		err = errs[0]
		return
	}

	return pathVars[PathParamNamespace], pathVars[PathParamPodID], pathVars[PathParamContainer], logOptions, nil
}
