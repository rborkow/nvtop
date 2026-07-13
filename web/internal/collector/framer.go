package collector

import (
	"bufio"
	"errors"
	"io"
)

const maxFrameSize = 8 << 20

// Framer extracts complete top-level JSON arrays from a noisy byte stream.
type Framer struct {
	r *bufio.Reader
}

func NewFramer(r io.Reader) *Framer { return &Framer{r: bufio.NewReader(r)} }

func (f *Framer) Next() ([]byte, error) {
	var frame []byte
	square, curly := 0, 0
	inString, escaped, discarding := false, false, false
	for {
		b, err := f.r.ReadByte()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil, io.EOF
			}
			return nil, err
		}
		if len(frame) == 0 && !discarding {
			if b != '[' {
				continue
			}
			square, curly, inString, escaped = 1, 0, false, false
			frame = append(frame, b)
			continue
		}
		if !discarding {
			frame = append(frame, b)
		}
		if inString {
			if escaped {
				escaped = false
			} else if b == '\\' {
				escaped = true
			} else if b == '"' {
				inString = false
			}
			continue
		}
		if b == '"' {
			inString = true
			continue
		}
		switch b {
		case '[':
			square++
		case ']':
			if square > 0 {
				square--
			}
		case '{':
			curly++
		case '}':
			if curly > 0 {
				curly--
			}
		}
		if !discarding && len(frame) > maxFrameSize {
			frame = nil
			discarding = true
		}
		if square == 0 && curly == 0 {
			if discarding {
				discarding = false
				frame = nil
				continue
			}
			return frame, nil
		}
	}
}
