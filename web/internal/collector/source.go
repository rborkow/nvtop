package collector

import (
	"context"
	"time"
)

// Source supplies complete JSON array frames. Implementations must unblock on
// context cancellation or Close.
type Source interface {
	Next(context.Context) ([]byte, error)
	Close() error
}

// Kill is implemented by live process sources and is used by the collector
// watchdog. It is intentionally optional for replay sources.
type Killer interface{ Kill() }

// Runner owns parsing, health state, and restart backoff around a source.
type Runner struct {
	Source     Source
	Interval   time.Duration
	OnSnapshot func(frame []byte, now time.Time) error
	OnStatus   func(status, detail string)
}

func (r Runner) Run(ctx context.Context) error {
	backoff := time.Second
	var healthySince time.Time
	failures := 0
	for {
		frame, err := r.Source.Next(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			status := "stale"
			wait := backoff
			if isNoGPU(err) {
				status, wait = "no_gpu", 30*time.Second
			}
			if r.OnStatus != nil {
				r.OnStatus(status, err.Error())
			}
			healthySince = time.Time{}
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(wait):
			}
			if wait != 30*time.Second && backoff < 30*time.Second {
				backoff *= 2
				if backoff > 30*time.Second {
					backoff = 30 * time.Second
				}
			}
			continue
		}
		now := time.Now()
		if err := r.OnSnapshot(frame, now); err != nil {
			failures++
			if r.OnStatus != nil {
				r.OnStatus("stale", err.Error())
			}
			if failures >= 3 {
				if k, ok := r.Source.(Killer); ok {
					k.Kill()
				}
				failures = 0
			}
			continue
		}
		failures = 0
		if healthySince.IsZero() {
			healthySince = now
		}
		if now.Sub(healthySince) >= 60*time.Second {
			backoff = time.Second
		}
	}
}
