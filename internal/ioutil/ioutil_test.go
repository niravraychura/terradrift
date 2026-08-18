package ioutil

import (
	"bytes"
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
