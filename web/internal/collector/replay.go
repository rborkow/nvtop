package collector

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"
)

type ReplaySource struct {
	frames   [][]byte
	interval time.Duration
	next     int
	first    bool
}

func NewReplaySource(path string, interval time.Duration) (*ReplaySource, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	framer := NewFramer(f)
	var frames [][]byte
	for {
		frame, e := framer.Next()
		if e == io.EOF {
			break
		}
		if e != nil {
			return nil, e
		}
		frames = append(frames, frame)
	}
	if len(frames) == 0 {
		return nil, fmt.Errorf("replay file has no complete JSON frames")
	}
	return &ReplaySource{frames: frames, interval: interval, first: true}, nil
}

func (r *ReplaySource) Next(ctx context.Context) ([]byte, error) {
	if !r.first {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(r.interval):
		}
	}
	r.first = false
	f := append([]byte(nil), r.frames[r.next]...)
	r.next = (r.next + 1) % len(r.frames)
	return f, nil
}
func (r *ReplaySource) Close() error { return nil }
