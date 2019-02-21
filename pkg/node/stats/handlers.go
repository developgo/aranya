package stats

import (
	"encoding/json"
	"net/http"
)

func (m *Manager) HandleStatsSummary(w http.ResponseWriter, r *http.Request) {
	log.Info("HandleStatsSummary")

	metrics, err := m.GetStatsSummary()
	if err != nil {

	}

	metricsJsonBytes, err := json.Marshal(metrics)
	if err != nil {
		return
	}

	if _, err := w.Write(metricsJsonBytes); err != nil {
		return
	}
}
