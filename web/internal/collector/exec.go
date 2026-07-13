package collector

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"
)

type noGPUError struct{}

func (noGPUError) Error() string { return "no GPU visible — check /dev/dri device mapping" }
func isNoGPU(err error) bool     { var e noGPUError; return errors.As(err, &e) }

type ExecSource struct {
	path     string
	interval time.Duration
	mu       sync.Mutex
	cmd      *exec.Cmd
	frames   chan []byte
	done     chan error
	output   bytes.Buffer
	stderr   []string
}

func NewExecSource(path string, interval time.Duration) *ExecSource {
	return &ExecSource{path: path, interval: interval}
}

func (e *ExecSource) startLocked() error {
	if e.cmd != nil {
		return nil
	}
	tenths := int(math.Round(float64(e.interval) / float64(100*time.Millisecond)))
	if tenths < 1 {
		tenths = 1
	}
	cmd := exec.Command(e.path, "-s", "-l", "-d", fmt.Sprint(tenths))
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	if err = cmd.Start(); err != nil {
		return err
	}
	e.cmd, e.frames, e.done = cmd, make(chan []byte, 2), make(chan error, 1)
	e.output.Reset()
	e.stderr = nil
	frames, done := e.frames, e.done
	// cmd.Wait closes the pipes, so it must not run until both readers hit
	// EOF — otherwise a fast-exiting child's "No GPU to monitor." line can be
	// lost before the tee captures it.
	stdoutDone := make(chan struct{})
	stderrDone := make(chan struct{})
	go func() {
		defer close(stdoutDone)
		tee := io.TeeReader(stdout, &lockedBuffer{mu: &e.mu, b: &e.output})
		f := NewFramer(tee)
		for {
			frame, er := f.Next()
			if er != nil {
				return
			}
			select {
			case frames <- frame:
			default:
			}
		}
	}()
	go func() {
		defer close(stderrDone)
		s := bufio.NewScanner(stderr)
		for s.Scan() {
			e.mu.Lock()
			e.stderr = append(e.stderr, s.Text())
			if len(e.stderr) > 10 {
				e.stderr = e.stderr[len(e.stderr)-10:]
			}
			e.mu.Unlock()
		}
	}()
	go func() {
		<-stdoutDone
		<-stderrDone
		done <- cmd.Wait()
	}()
	return nil
}

func (e *ExecSource) Next(ctx context.Context) ([]byte, error) {
	e.mu.Lock()
	err := e.startLocked()
	frames, done := e.frames, e.done
	e.mu.Unlock()
	if err != nil {
		return nil, err
	}
	timer := time.NewTimer(5 * e.interval)
	defer timer.Stop()
	select {
	case f := <-frames:
		return f, nil
	case err := <-done:
		e.mu.Lock()
		noGPU := strings.Contains(e.output.String(), "No GPU to monitor.")
		detail := strings.Join(e.stderr, "; ")
		e.cmd = nil
		e.mu.Unlock()
		if noGPU {
			return nil, noGPUError{}
		}
		if err == nil {
			err = fmt.Errorf("nvtop exited")
		}
		if detail != "" {
			err = fmt.Errorf("nvtop exited: %w (%s)", err, detail)
		}
		return nil, err
	case <-timer.C:
		e.Kill()
		return nil, fmt.Errorf("nvtop produced no valid frame for %s", 5*e.interval)
	case <-ctx.Done():
		e.Kill()
		return nil, ctx.Err()
	}
}

func (e *ExecSource) Kill() {
	e.mu.Lock()
	cmd := e.cmd
	e.mu.Unlock()
	if cmd != nil && cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
}
func (e *ExecSource) Close() error { e.Kill(); return nil }

// Terminate gives nvtop an orderly SIGTERM before the final SIGKILL fallback.
func (e *ExecSource) Terminate(grace time.Duration) {
	e.mu.Lock()
	cmd, done := e.cmd, e.done
	e.mu.Unlock()
	if cmd == nil || cmd.Process == nil {
		return
	}
	_ = cmd.Process.Signal(syscall.SIGTERM)
	if done == nil {
		return
	}
	select {
	case <-done:
	case <-time.After(grace):
		e.Kill()
	}
}

// lockedBuffer captures only the first outputCap bytes of the child's stdout:
// it exists solely to detect the "No GPU to monitor." line nvtop prints at
// startup, so capping it keeps a long-running healthy child from growing the
// buffer without bound.
const outputCap = 4096

type lockedBuffer struct {
	mu *sync.Mutex
	b  *bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if room := outputCap - b.b.Len(); room > 0 {
		if len(p) > room {
			b.b.Write(p[:room])
		} else {
			b.b.Write(p)
		}
	}
	return len(p), nil
}
