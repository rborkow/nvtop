package collector

import (
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"nvtop-web/internal/model"
)

type rawGPU struct {
	DeviceName   *string         `json:"device_name"`
	GPUClock     *string         `json:"gpu_clock"`
	MemClock     *string         `json:"mem_clock"`
	Temp         *string         `json:"temp"`
	FanSpeed     *string         `json:"fan_speed"`
	PowerDraw    *string         `json:"power_draw"`
	GPUUtil      *string         `json:"gpu_util"`
	EncodeDecode *string         `json:"encode_decode"`
	Encode       *string         `json:"encode"`
	Decode       *string         `json:"decode"`
	MemUtil      *string         `json:"mem_util"`
	MemTotal     *string         `json:"mem_total"`
	MemUsed      *string         `json:"mem_used"`
	MemFree      *string         `json:"mem_free"`
	Processes    json.RawMessage `json:"processes"`
}

type rawProcess struct {
	PID              *string `json:"pid"`
	Cmdline          *string `json:"cmdline"`
	Kind             *string `json:"kind"`
	User             *string `json:"user"`
	GPUUsage         *string `json:"gpu_usage"`
	GPUMemBytesAlloc *string `json:"gpu_mem_bytes_alloc"`
	GPUMemUsage      *string `json:"gpu_mem_usage"`
	EncodeDecode     *string `json:"encode_decode"`
	Encode           *string `json:"encode"`
	Decode           *string `json:"decode"`
}

func Parse(frame []byte, now time.Time) (model.Snapshot, error) {
	frame = sanitizeFrame(frame)
	var raw []rawGPU
	if err := json.Unmarshal(frame, &raw); err != nil {
		return model.Snapshot{}, err
	}
	// A null shared encode/decode value still identifies an iGPU layout. Keep
	// presence separately because a *string cannot distinguish null from absent.
	var keys []map[string]json.RawMessage
	if err := json.Unmarshal(frame, &keys); err != nil {
		return model.Snapshot{}, err
	}
	s := model.Snapshot{Timestamp: now.UTC().Format(time.RFC3339), GPUs: make([]model.GPU, len(raw))}
	for i, g := range raw {
		out := model.GPU{Index: i, DeviceName: g.DeviceName, GPUClockMHz: number(g.GPUClock, "MHz"), MemClockMHz: number(g.MemClock, "MHz"), TempC: temperature(g.Temp), PowerDrawW: number(g.PowerDraw, "W"), GPUUtilPct: number(g.GPUUtil, "%"), MemUtilPct: number(g.MemUtil, "%"), MemTotalBytes: uintNumber(g.MemTotal), MemUsedBytes: uintNumber(g.MemUsed), MemFreeBytes: uintNumber(g.MemFree)}
		if g.FanSpeed != nil {
			switch {
			case *g.FanSpeed == "CPU Fan":
				n := "CPU Fan"
				out.FanNote = &n
			case strings.HasSuffix(strings.TrimSpace(*g.FanSpeed), "RPM"):
				out.FanSpeedRPM = number(g.FanSpeed, "RPM")
			default:
				out.FanSpeedPct = number(g.FanSpeed, "%")
			}
		}
		if _, shared := keys[i]["encode_decode"]; shared {
			out.EncodePct, out.DecodePct, out.EncodeDecodeShared = number(g.EncodeDecode, "%"), number(g.EncodeDecode, "%"), true
		} else {
			out.EncodePct, out.DecodePct = number(g.Encode, "%"), number(g.Decode, "%")
		}
		if len(g.Processes) > 0 && string(g.Processes) != "null" {
			var processes []rawProcess
			if err := json.Unmarshal(g.Processes, &processes); err != nil {
				return model.Snapshot{}, err
			}
			out.Processes = make([]model.Process, len(processes))
			for j, p := range processes {
				q := model.Process{PID: uintNumber(p.PID), Cmdline: p.Cmdline, Kind: p.Kind, User: p.User, GPUUsagePct: number(p.GPUUsage, "%"), GPUMemBytes: uintNumber(p.GPUMemBytesAlloc), GPUMemPct: number(p.GPUMemUsage, "%")}
				if p.EncodeDecode != nil {
					q.EncodePct, q.DecodePct = number(p.EncodeDecode, "%"), number(p.EncodeDecode, "%")
				} else {
					q.EncodePct, q.DecodePct = number(p.Encode, "%"), number(p.Decode, "%")
				}
				out.Processes[j] = q
			}
		}
		s.GPUs[i] = out
	}
	return s, nil
}

// sanitizeFrame escapes raw control bytes inside JSON strings. nvtop's
// snapshot emitter escapes \n \b \f \r \t \\ " but passes other control bytes
// through verbatim, so a GPU process whose argv contains e.g. 0x01 would make
// every frame invalid JSON forever. Escaping here keeps one hostile cmdline
// from poisoning all telemetry.
func sanitizeFrame(frame []byte) []byte {
	inString, escaped, dirty := false, false, false
	for _, b := range frame {
		if inString && b < 0x20 {
			dirty = true
			break
		}
		switch {
		case inString && escaped:
			escaped = false
		case inString && b == '\\':
			escaped = true
		case b == '"':
			inString = !inString
		}
	}
	if !dirty {
		return frame
	}
	out := make([]byte, 0, len(frame)+16)
	inString, escaped = false, false
	const hex = "0123456789abcdef"
	for _, b := range frame {
		if inString && b < 0x20 {
			out = append(out, '\\', 'u', '0', '0', hex[b>>4], hex[b&0xf])
			continue
		}
		switch {
		case inString && escaped:
			escaped = false
		case inString && b == '\\':
			escaped = true
		case b == '"':
			inString = !inString
		}
		out = append(out, b)
	}
	return out
}

func number(s *string, suffix string) *float64 {
	if s == nil {
		return nil
	}
	v, err := strconv.ParseFloat(strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(*s), suffix)), 64)
	if err != nil {
		return nil
	}
	return &v
}

func uintNumber(s *string) *uint64 {
	if s == nil {
		return nil
	}
	v, err := strconv.ParseUint(strings.TrimSpace(*s), 10, 64)
	if err != nil {
		return nil
	}
	return &v
}

func temperature(s *string) *float64 {
	if s == nil {
		return nil
	}
	v := strings.TrimSpace(*s)
	if strings.HasSuffix(v, "F") {
		n := number(&v, "F")
		if n != nil {
			x := (*n - 32) * 5 / 9
			return &x
		}
		return nil
	}
	return number(&v, "C")
}
