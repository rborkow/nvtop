package collector

import (
	"os"
	"testing"
	"time"

	"nvtop-web/internal/model"
)

func TestRealNASSnapshotParsesAndFilters(t *testing.T) {
	// Canonical fixture captured from the N5 Pro (TrueNAS, Radeon 890M).
	b, err := os.ReadFile("testdata/snapshot_890m_real.json")
	if err != nil {
		t.Fatal(err)
	}
	s, err := Parse(b, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if len(s.GPUs) != 1 {
		t.Fatalf("gpus: %d", len(s.GPUs))
	}
	g := s.GPUs[0]
	if g.DeviceName == nil || *g.DeviceName != "AMD Radeon Graphics" {
		t.Fatalf("device_name: %v", g.DeviceName)
	}
	if g.MemUtilPct == nil || *g.MemUtilPct != 50 {
		t.Fatalf("mem_util: %v", g.MemUtilPct)
	}
	if g.PowerDrawW == nil || *g.PowerDrawW != 8 {
		t.Fatalf("power: %v", g.PowerDrawW)
	}
	if !g.EncodeDecodeShared {
		t.Fatal("expected shared encode/decode on the iGPU")
	}
	if len(g.Processes) != 3 {
		t.Fatalf("processes: %d", len(g.Processes))
	}
	// gpu_usage is null on a one-shot snapshot (needs a sampling delta)
	if g.Processes[0].GPUUsagePct != nil {
		t.Fatalf("expected null gpu_usage, got %v", *g.Processes[0].GPUUsagePct)
	}

	// The supervised nvtop child (pid 838320 in the fixture) filters out.
	FilterPID(&s, 838320)
	if len(s.GPUs[0].Processes) != 2 {
		t.Fatalf("after filter: %d", len(s.GPUs[0].Processes))
	}
	for _, p := range s.GPUs[0].Processes {
		if p.PID != nil && *p.PID == 838320 {
			t.Fatal("self process not filtered")
		}
	}
}

func TestFilterPIDNoMatchAndNilPID(t *testing.T) {
	pid := uint64(9)
	snap := model.Snapshot{GPUs: []model.GPU{{Processes: []model.Process{{PID: nil}, {PID: &pid}}}}}
	FilterPID(&snap, -1) // no child alive → untouched
	if len(snap.GPUs[0].Processes) != 2 {
		t.Fatal("filter with pid<=0 must be a no-op")
	}
	FilterPID(&snap, 12345) // no match → untouched
	if len(snap.GPUs[0].Processes) != 2 {
		t.Fatal("filter without match must keep all")
	}
}
