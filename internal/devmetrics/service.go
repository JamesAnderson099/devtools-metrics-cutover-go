package devmetrics

import (
	"encoding/json"
	"errors"
	"net/http"
)

type Service struct {
	Pipeline EventPipeline
	Client   *InfraiClient
}

func (s Service) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /events", s.handleEvent)
	mux.HandleFunc("GET /chart", s.handleChart)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	return mux
}

func (s Service) handleEvent(w http.ResponseWriter, r *http.Request) {
	var event DeveloperEvent
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&event); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid event JSON"})
		return
	}
	point, err := s.Pipeline.Record(r.Context(), event)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"accepted": true, "metric": point.Name, "value": point.Value})
}

func (s Service) handleChart(w http.ResponseWriter, r *http.Request) {
	metricName := r.URL.Query().Get("metric")
	if metricName == "" {
		metricName = "devtools.build.completed"
	}
	business, err := s.Client.QueryMetric(r.Context(), metricName, "sum")
	if err != nil {
		writeServiceError(w, err)
		return
	}
	spend, err := s.Client.UsageTimeseries(r.Context())
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"metric":   metricName,
		"business": json.RawMessage(business),
		"spend":    json.RawMessage(spend),
	})
}

func writeServiceError(w http.ResponseWriter, err error) {
	var infraiErr *InfraiError
	if errors.As(err, &infraiErr) && infraiErr.HTTPStatus >= 400 && infraiErr.HTTPStatus < 500 {
		writeJSON(w, infraiErr.HTTPStatus, map[string]string{"error": infraiErr.Code})
		return
	}
	if errors.As(err, &infraiErr) {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "upstream request failed"})
		return
	}
	writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
