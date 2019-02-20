package server

import (
	"net/http"
)

func HandleFuncPodLog(w http.ResponseWriter, r *http.Request) {
	log.Info("HandleFuncPodLog")
}

func HandleFuncPodExec(w http.ResponseWriter, r *http.Request) {
	log.Info("HandleFuncPodExec")
}
