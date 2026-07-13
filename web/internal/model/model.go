// Package model defines the stable JSON contract exposed by nvtop-web.
package model

type Snapshot struct {
	Timestamp string `json:"timestamp"`
	GPUs      []GPU  `json:"gpus"`
}

type GPU struct {
	Index              int       `json:"index"`
	DeviceName         *string   `json:"device_name"`
	GPUClockMHz        *float64  `json:"gpu_clock_mhz"`
	MemClockMHz        *float64  `json:"mem_clock_mhz"`
	TempC              *float64  `json:"temp_c"`
	FanSpeedPct        *float64  `json:"fan_speed_pct"`
	FanSpeedRPM        *float64  `json:"fan_speed_rpm"`
	FanNote            *string   `json:"fan_note,omitempty"`
	PowerDrawW         *float64  `json:"power_draw_w"`
	GPUUtilPct         *float64  `json:"gpu_util_pct"`
	EncodePct          *float64  `json:"encode_pct"`
	DecodePct          *float64  `json:"decode_pct"`
	EncodeDecodeShared bool      `json:"encode_decode_shared"`
	MemUtilPct         *float64  `json:"mem_util_pct"`
	MemTotalBytes      *uint64   `json:"mem_total_bytes"`
	MemUsedBytes       *uint64   `json:"mem_used_bytes"`
	MemFreeBytes       *uint64   `json:"mem_free_bytes"`
	Processes          []Process `json:"processes"`
}

type Process struct {
	PID         *uint64  `json:"pid"`
	Cmdline     *string  `json:"cmdline"`
	Kind        *string  `json:"kind"`
	User        *string  `json:"user"`
	GPUUsagePct *float64 `json:"gpu_usage_pct"`
	GPUMemBytes *uint64  `json:"gpu_mem_bytes"`
	GPUMemPct   *float64 `json:"gpu_mem_pct"`
	EncodePct   *float64 `json:"encode_pct"`
	DecodePct   *float64 `json:"decode_pct"`
}

type HistoryPoint struct {
	TS   int64      `json:"ts"`
	GPUs []GPUPoint `json:"gpus"`
}

type GPUPoint struct {
	Util     *float64 `json:"util"`
	Temp     *float64 `json:"temp"`
	MemUsed  *uint64  `json:"mem_used"`
	MemTotal *uint64  `json:"mem_total"`
	Power    *float64 `json:"power"`
	Enc      *float64 `json:"enc"`
	Dec      *float64 `json:"dec"`
}
