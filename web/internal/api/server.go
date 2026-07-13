package api

import (
	_ "embed"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"nvtop-web/internal/model"
	"nvtop-web/internal/store"
)

//go:embed static/index.html
var indexHTML []byte

func New(s *store.Store, interval time.Duration) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(indexHTML)
	})
	mux.HandleFunc("/api/metrics", func(w http.ResponseWriter, r *http.Request) {
		snap, meta := s.Latest()
		gpus := snap.GPUs
		if gpus == nil {
			gpus = []model.GPU{} // before the first snapshot; null would break consumers
		}
		writeJSON(w, http.StatusOK, map[string]any{"timestamp": snap.Timestamp, "status": meta.Status, "age_seconds": age(meta.LastUpdate), "interval_seconds": interval.Seconds(), "gpus": gpus})
	})
	mux.HandleFunc("/api/history", func(w http.ResponseWriter, r *http.Request) {
		since, _ := strconv.ParseInt(r.URL.Query().Get("since"), 10, 64)
		writeJSON(w, http.StatusOK, map[string]any{"interval_seconds": interval.Seconds(), "points": s.History(since)})
	})
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		_, meta := s.Latest()
		good := meta.Status == "ok" && !meta.LastUpdate.IsZero() && time.Since(meta.LastUpdate) < 3*interval
		if good {
			plainJSON(w, http.StatusOK, map[string]string{"status": "ok"})
			return
		}
		status := meta.Status
		if status == "ok" {
			status = "stale"
		}
		plainJSON(w, http.StatusServiceUnavailable, map[string]string{"status": status, "detail": meta.LastError})
	})
	return mux
}
func age(t time.Time) float64 {
	if t.IsZero() {
		return -1
	}
	return time.Since(t).Seconds()
}

// No CORS header on purpose: the dashboard is same-origin and Home Assistant
// polls server-side, so a wildcard would only let arbitrary web origins pull
// host process data through a LAN browser.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func plainJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
