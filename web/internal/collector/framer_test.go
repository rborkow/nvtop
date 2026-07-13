package collector

import (
	"bytes"
	"io"
	"math/rand"
	"os"
	"testing"
)

type chunkReader struct {
	chunks [][]byte
	n      int
}

func (r *chunkReader) Read(p []byte) (int, error) {
	if r.n >= len(r.chunks) {
		return 0, io.EOF
	}
	n := copy(p, r.chunks[r.n])
	r.n++
	return n, nil
}

func TestFramerTorture(t *testing.T) {
	b, err := os.ReadFile("testdata/stream_torture.txt")
	if err != nil {
		t.Fatal(err)
	}
	rng := rand.New(rand.NewSource(44))
	var chunks [][]byte
	for len(b) > 0 {
		n := rng.Intn(17) + 1
		if n > len(b) {
			n = len(b)
		}
		chunks = append(chunks, append([]byte(nil), b[:n]...))
		b = b[n:]
	}
	f := NewFramer(&chunkReader{chunks: chunks})
	var frames [][]byte
	for {
		x, err := f.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		frames = append(frames, x)
	}
	if len(frames) != 3 {
		t.Fatalf("frames=%d", len(frames))
	}
	if !bytes.Contains(frames[0], []byte("Torture GPU A")) || !bytes.Contains(frames[1], []byte("Broken frame")) || !bytes.Contains(frames[2], []byte("Torture GPU B")) {
		t.Fatal("wrong frames")
	}
}
