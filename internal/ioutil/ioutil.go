// Package ioutil provides bounded readers and writers for untrusted I/O.
package ioutil

import (
	"bytes"
	"fmt"
	"io"
	"os"
)

// LimitedBuffer caps how many bytes are retained while still consuming Write calls.
// Truncated is set when any bytes beyond Remaining are discarded.
type LimitedBuffer struct {
	Buffer    *bytes.Buffer
	Remaining int
	Truncated bool
}

// Write implements io.Writer. Excess bytes are discarded after Remaining is exhausted.
func (buffer *LimitedBuffer) Write(p []byte) (int, error) {
	originalLen := len(p)
	if buffer.Remaining <= 0 {
		if originalLen > 0 {
			buffer.Truncated = true
		}
		return originalLen, nil
	}
	if len(p) > buffer.Remaining {
		p = p[:buffer.Remaining]
		buffer.Truncated = true
	}
	written, err := buffer.Buffer.Write(p)
	buffer.Remaining -= written
	return originalLen, err
}

// LimitedWriter forwards writes to W until Remaining bytes are consumed.
// Further bytes are discarded and Truncated is set; Write still reports success
// for the full input length so command pipes keep draining.
type LimitedWriter struct {
	W         io.Writer
	Remaining int64
	Truncated bool
}

// Write implements io.Writer.
func (writer *LimitedWriter) Write(p []byte) (int, error) {
	originalLen := len(p)
	if writer.Remaining <= 0 {
		if len(p) > 0 {
			writer.Truncated = true
		}
		return originalLen, nil
	}
	if int64(len(p)) > writer.Remaining {
		p = p[:writer.Remaining]
		writer.Truncated = true
	}
	written, err := writer.W.Write(p)
	writer.Remaining -= int64(written)
	return originalLen, err
}

// ReadLimitedFile reads a file and rejects contents larger than maximum bytes.
func ReadLimitedFile(path string, maximum int64) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()
	data, err := io.ReadAll(io.LimitReader(file, maximum+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > maximum {
		return nil, fmt.Errorf("file exceeds %d bytes", maximum)
	}
	return data, nil
}
