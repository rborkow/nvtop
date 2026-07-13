package store

import (
	"sync"
	"time"

	"nvtop-web/internal/model"
)

type Meta struct {
	LastUpdate time.Time `json:"last_update"`
	Status     string    `json:"status"`
	LastError  string    `json:"last_error"`
}

type Store struct {
	mu          sync.RWMutex
	latest      model.Snapshot
	meta        Meta
	history     []model.HistoryPoint
	next, count int
}

func New(capacity int) *Store {
	if capacity < 1 {
		capacity = 1
	}
	return &Store{history: make([]model.HistoryPoint, capacity), meta: Meta{Status: "starting"}}
}

func (s *Store) SetLatest(snapshot model.Snapshot, meta Meta) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.latest, s.meta = snapshot, meta
	p := model.HistoryPoint{TS: meta.LastUpdate.Unix(), GPUs: make([]model.GPUPoint, len(snapshot.GPUs))}
	for i, g := range snapshot.GPUs {
		p.GPUs[i] = model.GPUPoint{Util: g.GPUUtilPct, Temp: g.TempC, MemUsed: g.MemUsedBytes, MemTotal: g.MemTotalBytes, Power: g.PowerDrawW, Enc: g.EncodePct, Dec: g.DecodePct}
	}
	s.history[s.next] = p
	s.next = (s.next + 1) % len(s.history)
	if s.count < len(s.history) {
		s.count++
	}
}

func (s *Store) SetMeta(meta Meta) { s.mu.Lock(); s.meta = meta; s.mu.Unlock() }
func (s *Store) Latest() (model.Snapshot, Meta) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.latest, s.meta
}
func (s *Store) History(sinceUnix int64) []model.HistoryPoint {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]model.HistoryPoint, 0, s.count)
	start := (s.next - s.count + len(s.history)) % len(s.history)
	for i := 0; i < s.count; i++ {
		p := s.history[(start+i)%len(s.history)]
		if p.TS >= sinceUnix {
			p.GPUs = append([]model.GPUPoint(nil), p.GPUs...)
			out = append(out, p)
		}
	}
	return out
}
