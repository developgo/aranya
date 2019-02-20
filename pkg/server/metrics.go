package server

import (
	"net/http"
)

func (s *Server) HandleFuncMetricsSummary(w http.ResponseWriter, r *http.Request) {
	s.httpLogger.Info("HandleFuncMetricsSummary")
}
