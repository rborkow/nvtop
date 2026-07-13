package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"nvtop-web/internal/api"
	"nvtop-web/internal/collector"
	"nvtop-web/internal/store"
)

func main() {
	listen := flag.String("listen", env("NVTOP_WEB_LISTEN", ":8080"), "HTTP listen address")
	interval := flag.Duration("interval", durationEnv("NVTOP_WEB_INTERVAL", 2*time.Second), "snapshot interval")
	history := flag.Duration("history", durationEnv("NVTOP_WEB_HISTORY", time.Hour), "history duration")
	nvtopPath := flag.String("nvtop-path", env("NVTOP_WEB_NVTOP_PATH", "/usr/local/bin/nvtop"), "path to nvtop")
	replay := flag.String("replay", env("NVTOP_WEB_REPLAY", ""), "replay concatenated snapshot file")
	healthcheck := flag.Bool("healthcheck", false, "check this instance's /healthz endpoint")
	flag.Parse()
	if *healthcheck {
		os.Exit(runHealthcheck(*listen))
	}
	if *interval <= 0 || *history <= 0 {
		log.Fatal("interval and history must be positive")
	}
	// Clamp to sane bounds: nvtop itself samples no faster than 100ms, and an
	// extreme history/interval ratio would allocate a giant ring up front.
	if *interval < 100*time.Millisecond {
		log.Printf("interval %s too small; clamping to 100ms", *interval)
		*interval = 100 * time.Millisecond
	}
	if *history < *interval {
		*history = *interval
	}

	capacity := int(*history / *interval)
	const maxCapacity = 100_000
	if capacity > maxCapacity {
		log.Printf("history/interval = %d points; capping ring at %d", capacity, maxCapacity)
		capacity = maxCapacity
	}
	s := store.New(capacity)
	var source collector.Source
	var err error
	if *replay != "" {
		source, err = collector.NewReplaySource(*replay, *interval)
	} else {
		source = collector.NewExecSource(*nvtopPath, *interval)
	}
	if err != nil {
		log.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	runner := collector.Runner{Source: source, Interval: *interval,
		OnSnapshot: func(frame []byte, now time.Time) error {
			snapshot, err := collector.Parse(frame, now)
			if err != nil {
				log.Printf("invalid nvtop frame: %s: %v", truncate(frame, 1024), err)
				return err
			}
			if live, ok := source.(*collector.ExecSource); ok {
				collector.FilterPID(&snapshot, live.Pid())
			}
			s.SetLatest(snapshot, store.Meta{LastUpdate: now, Status: "ok"})
			return nil
		},
		OnStatus: func(status, detail string) {
			snap, meta := s.Latest()
			meta.Status, meta.LastError = status, detail
			s.SetMeta(meta)
			_ = snap
		},
	}
	go func() {
		if err := runner.Run(ctx); err != nil && ctx.Err() == nil {
			log.Printf("collector stopped: %v", err)
		}
	}()

	server := &http.Server{Addr: *listen, Handler: api.New(s, *interval), ReadTimeout: 5 * time.Second, WriteTimeout: 10 * time.Second, IdleTimeout: 60 * time.Second}
	serverErr := make(chan error, 1)
	go func() { serverErr <- server.ListenAndServe() }()
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	select {
	case sig := <-signals:
		log.Printf("received %s; shutting down", sig)
		if live, ok := source.(*collector.ExecSource); ok {
			live.Terminate(3 * time.Second)
		}
		cancel()
		shutdown, stop := context.WithTimeout(context.Background(), 5*time.Second)
		defer stop()
		_ = server.Shutdown(shutdown)
	case err := <-serverErr:
		if err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
func durationEnv(key string, fallback time.Duration) time.Duration {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	d, err := time.ParseDuration(value)
	if err != nil {
		log.Printf("invalid %s=%q; using %s", key, value, fallback)
		return fallback
	}
	return d
}
func truncate(b []byte, n int) string {
	if len(b) > n {
		return string(b[:n]) + "…"
	}
	return string(b)
}

func runHealthcheck(listen string) int {
	host, port, err := net.SplitHostPort(listen)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if host == "" || host == "0.0.0.0" || host == "::" {
		host = "127.0.0.1"
	}
	if strings.Contains(host, ":") {
		host = "[" + host + "]"
	}
	client := http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get("http://" + host + ":" + port + "/healthz")
	if err != nil {
		return 1
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusOK {
		return 0
	}
	return 1
}
