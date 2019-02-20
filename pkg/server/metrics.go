package server

import (
	"github.com/gorilla/mux"
	"net/http"
)

func AddRoutesForMetrics(m *mux.Router) {
	m.HandleFunc("/stats/summary", MetricsSummaryHandler).Methods(http.MethodGet)
}

func MetricsSummaryHandler(w http.ResponseWriter, r *http.Request) {

}
