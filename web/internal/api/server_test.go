package api

import (
	"net/http"
	"net/http/httptest"
	"nvtop-web/internal/model"
	"nvtop-web/internal/store"
	"strings"
	"testing"
	"time"
)

func TestRoutes(t *testing.T) {
	s := store.New(3)
	s.SetLatest(model.Snapshot{Timestamp: "2026-01-01T00:00:00Z", GPUs: []model.GPU{{Index: 0}}}, store.Meta{LastUpdate: time.Now(), Status: "ok"})
	h := New(s, time.Second)
	for _, tc := range []struct {
		path string
		code int
	}{{"/", 200}, {"/api/metrics", 200}, {"/api/history?since=0", 200}, {"/healthz", 200}} {
		r := httptest.NewRequest(http.MethodGet, tc.path, nil)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != tc.code {
			t.Fatalf("%s: %d", tc.path, w.Code)
		}
		if strings.HasPrefix(tc.path, "/api/") && w.Header().Get("Access-Control-Allow-Origin") != "" {
			t.Fatal("unexpected CORS header: API must stay same-origin")
		}
	}
	s.SetMeta(store.Meta{Status: "no_gpu", LastError: "missing"})
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if w.Code != 503 {
		t.Fatal(w.Code)
	}
}
