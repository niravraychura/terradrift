// Package ioutil provides bounded readers and writers for untrusted I/O.
package ioutil

import (
	"bytes"
	"fmt"
	"io"
	"os"
)

// LimitedBuffer caps how many bytes are retained while still consuming Write calls.
type LimitedBuffer struct {
	Buffer    *bytes.Buffer
	Remaining int
}

// Write implements io.Writer. Excess bytes are discarded after Remaining is exhausted.
func (buffer *LimitedBuffer) Write(p []byte) (int, error) {
	originalLen := len(p)
	if buffer.Remaining <= 0 {
		return originalLen, nil
	}
	if len(p) > buffer.Remaining {
		p = p[:buffer.Remaining]
	}
	written, err := buffer.Buffer.Write(p)
	buffer.Remaining -= written
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
