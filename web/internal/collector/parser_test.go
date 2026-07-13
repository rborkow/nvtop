package collector

import (
	"os"
	"strings"
	"testing"
	"time"
)

func TestParseFixtures(t *testing.T) {
	cases := []struct {
		name   string
		gpus   int
		shared bool
	}{{"snapshot_890m.json", 1, true}, {"snapshot_dual_gpu.json", 2, true}}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			b, err := os.ReadFile("testdata/" + tc.name)
			if err != nil {
				t.Fatal(err)
			}
			s, err := Parse(b, time.Unix(100, 0))
			if err != nil {
				t.Fatal(err)
			}
			if len(s.GPUs) != tc.gpus || s.GPUs[0].EncodeDecodeShared != tc.shared || s.GPUs[0].FanNote == nil {
				t.Fatal("unexpected first GPU")
			}
			if tc.name == "snapshot_890m.json" && (len(s.GPUs[0].Processes) != 3 || s.GPUs[0].Processes[2].Cmdline != nil) {
				t.Fatal("process nulls")
			}
			if tc.name == "snapshot_dual_gpu.json" && s.GPUs[1].FanSpeedRPM == nil {
				t.Fatal("rpm fan")
			}
		})
	}
}
func TestNumberHelpers(t *testing.T) {
	s := "42%"
	if v := number(&s, "%"); v == nil || *v != 42 {
		t.Fatal(v)
	}
	f := "212F"
	if v := temperature(&f); v == nil || *v != 100 {
		t.Fatal(v)
	}
	bad := "wat"
	if number(&bad, "%") != nil {
		t.Fatal("bad value")
	}
}

func TestBrokenFrameReturnsError(t *testing.T) {
	b, err := os.ReadFile("testdata/stream_torture.txt")
	if err != nil {
		t.Fatal(err)
	}
	f := NewFramer(strings.NewReader(string(b)))
	if _, err := f.Next(); err != nil {
		t.Fatal(err)
	}
	broken, err := f.Next()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Parse(broken, time.Now()); err == nil {
		t.Fatal("expected malformed frame error")
	}
}

func TestSanitizeControlBytesInCmdline(t *testing.T) {
	// nvtop does not escape control bytes in cmdlines; a hostile process argv
	// with a raw 0x01 must not make the whole frame unparseable.
	frame := []byte("[\n  {\n   \"device_name\": \"GPU\",\n   \"gpu_util\": \"10%\",\n   \"processes\" : [\n     {\n       \"pid\": \"7\",\n       \"cmdline\": \"evil\x01arg\x1f\",\n       \"kind\": \"compute\",\n       \"user\": \"x\",\n       \"gpu_usage\": \"5%\",\n       \"gpu_mem_bytes_alloc\": \"1\",\n       \"gpu_mem_usage\": \"1%\",\n       \"encode\": null,\n       \"decode\": null\n     }\n   ]\n  }\n]\n")
	s, err := Parse(frame, time.Now())
	if err != nil {
		t.Fatalf("sanitizer failed to rescue frame: %v", err)
	}
	if len(s.GPUs) != 1 || len(s.GPUs[0].Processes) != 1 {
		t.Fatalf("unexpected shape: %+v", s.GPUs)
	}
	got := *s.GPUs[0].Processes[0].Cmdline
	if got != "evil\x01arg\x1f" {
		t.Fatalf("cmdline mangled: %q", got)
	}
}
