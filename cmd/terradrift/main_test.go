package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func executeCommand(args ...string) (string, string, error) {
	var stdout, stderr bytes.Buffer
	cmd := newRootCommand(&stdout, &stderr)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return stdout.String(), stderr.String(), err
}

func TestScanRequiresDirectory(t *testing.T) {
	_, _, err := executeCommand("scan")
	if err == nil || !strings.Contains(err.Error(), "terraform directory is required") {
		t.Fatalf("expected missing directory error, got %v", err)
	}
}

func TestScanRejectsNonexistentDirectory(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing")
	_, _, err := executeCommand("scan", "--directory", missing)
	if err == nil || !strings.Contains(err.Error(), "terraform directory does not exist") {
		t.Fatalf("expected nonexistent directory error, got %v", err)
	}
}

func TestScanRejectsFilePath(t *testing.T) {
	file := filepath.Join(t.TempDir(), "main.tf")
	if err := os.WriteFile(file, []byte("terraform {}\n"), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	_, _, err := executeCommand("scan", "-d", file)
	if err == nil || !strings.Contains(err.Error(), "terraform path is not a directory") {
		t.Fatalf("expected not directory error, got %v", err)
	}
}

func TestScanValidDirectory(t *testing.T) {
	dir := t.TempDir()
	stdout, _, err := executeCommand("scan", "-d", dir)
	if err != nil {
		t.Fatalf("expected valid directory, got %v", err)
	}
	absDir, err := filepath.Abs(dir)
	if err != nil {
		t.Fatalf("abs fixture: %v", err)
	}
	if !strings.Contains(stdout, "TerraDrift scan initialized") || !strings.Contains(stdout, "Terraform directory: "+absDir) {
		t.Fatalf("unexpected stdout: %q", stdout)
	}
}

func TestLogLevelSupported(t *testing.T) {
	for _, level := range []string{"debug", "info", "warn", "error"} {
		_, _, err := executeCommand("--log-level", level, "scan", "-d", t.TempDir())
		if err != nil {
			t.Fatalf("expected log level %q to be supported: %v", level, err)
		}
	}
}

func TestLogLevelUnsupported(t *testing.T) {
	_, _, err := executeCommand("--log-level", "trace", "scan", "-d", t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "unsupported log level") {
		t.Fatalf("expected unsupported log level error, got %v", err)
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) {
	return 0, errors.New("write failed")
}

func TestScanReturnsOutputWriteError(t *testing.T) {
	cmd := newRootCommand(failingWriter{}, &bytes.Buffer{})
	cmd.SetArgs([]string{"scan", "-d", t.TempDir()})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "write scan output") {
		t.Fatalf("expected output write error, got %v", err)
	}
}
