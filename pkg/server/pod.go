package server

import (
	"net/http"
)

func (s *Server) HandleFuncPodLog(w http.ResponseWriter, r *http.Request) {
	s.httpLogger.Info("HandleFuncPodLog")
}

func (s *Server) HandleFuncPodExec(w http.ResponseWriter, r *http.Request) {
	s.httpLogger.Info("HandleFuncPodExec")
}
