package ioutil

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLimitedBufferCapsRetainedBytes(t *testing.T) {
	var buffer bytes.Buffer
	writer := &LimitedBuffer{Buffer: &buffer, Remaining: 4}
	if n, err := writer.Write([]byte("abcdef")); err != nil || n != 6 {
		t.Fatalf("Write() = %d, %v", n, err)
	}
	if buffer.String() != "abcd" {
		t.Fatalf("retained %q, want abcd", buffer.String())
	}
	if !writer.Truncated {
		t.Fatal("expected Truncated to be set")
	}
}

func TestLimitedBufferMarksTruncationOnExactThenExtra(t *testing.T) {
	var buffer bytes.Buffer
	writer := &LimitedBuffer{Buffer: &buffer, Remaining: 2}
	if _, err := writer.Write([]byte("ab")); err != nil || writer.Truncated {
		t.Fatalf("exact write should not truncate: err=%v truncated=%t", err, writer.Truncated)
	}
	if _, err := writer.Write([]byte("c")); err != nil || !writer.Truncated {
		t.Fatalf("expected truncation after budget exhausted: err=%v truncated=%t", err, writer.Truncated)
	}
}

func TestLimitedWriterMarksTruncation(t *testing.T) {
	writer := &LimitedWriter{W: io.Discard, Remaining: 1}
	if _, err := writer.Write([]byte("ab")); err != nil || !writer.Truncated {
		t.Fatalf("expected truncation, err=%v truncated=%t", err, writer.Truncated)
	}
}

func TestReadLimitedFileRejectsOversized(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "file")
	if err := os.WriteFile(path, []byte("hello"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadLimitedFile(path, 4); err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("expected exceeds error, got %v", err)
	}
	data, err := ReadLimitedFile(path, 5)
	if err != nil || string(data) != "hello" {
		t.Fatalf("ReadLimitedFile() = %q, %v", data, err)
	}
}
