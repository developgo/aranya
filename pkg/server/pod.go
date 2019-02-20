package server

import (
	"github.com/gorilla/mux"
	"net/http"
)

func AddRoutesForPod(m *mux.Router) {
	m.HandleFunc("/containerLogs/{namespace}/{pod}/{container}", PodLogHandler).Methods(http.MethodGet)
	m.HandleFunc("/exec/{namespace}/{pod}/{container}", PodExecHandler).Methods(http.MethodPost)
}

func PodLogHandler(w http.ResponseWriter, r *http.Request) {

}

func PodExecHandler(w http.ResponseWriter, r *http.Request) {

}
