package store

import (
	"nvtop-web/internal/model"
	"testing"
	"time"
)

func TestRingAndMeta(t *testing.T) {
	s := New(2)
	for i := int64(1); i <= 3; i++ {
		s.SetLatest(model.Snapshot{}, Meta{LastUpdate: time.Unix(i, 0), Status: "ok"})
	}
	h := s.History(0)
	if len(h) != 2 || h[0].TS != 2 || h[1].TS != 3 {
		t.Fatalf("%+v", h)
	}
	if len(s.History(3)) != 1 {
		t.Fatal("since")
	}
	_, m := s.Latest()
	if m.Status != "ok" {
		t.Fatal(m.Status)
	}
}
