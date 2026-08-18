// Package history stores local scan report history for self-hosted trend views.
package history

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/niravraychura/terradrift/internal/report"
)

const fileTimestampFormat = "20060102T150405.000000000Z"

const (
	maxReportBytes  = 32 << 20
	maxHistoryFiles = 1000
)

// Entry is a persisted scan report.
type Entry struct {
	Report report.DriftReport
}

// Write stores a scan report as a secure JSON file in directory.
func Write(directory string, scanReport report.DriftReport) (string, error) {
	return write(directory, scanReport, false)
}

// WriteCompressed stores a gzip-compressed scan report with the same security guarantees as Write.
func WriteCompressed(directory string, scanReport report.DriftReport) (string, error) {
	return write(directory, scanReport, true)
}

func write(directory string, scanReport report.DriftReport, compressed bool) (string, error) {
	if directory == "" {
		return "", fmt.Errorf("history directory is required")
	}
	if info, err := os.Lstat(directory); err == nil && info.Mode()&os.ModeSymlink != 0 {
		return "", fmt.Errorf("history directory must not be a symlink: %s", directory)
	}
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return "", fmt.Errorf("create history directory %s: %w", directory, err)
	}
	if err := os.Chmod(directory, 0o700); err != nil {
		return "", fmt.Errorf("secure history directory %s: %w", directory, err)
	}
	data, err := json.MarshalIndent(scanReport, "", "  ")
	if err != nil {
		return "", fmt.Errorf("encode history report: %w", err)
	}
	data = append(data, '\n')
	if len(data) > maxReportBytes {
		return "", fmt.Errorf("history report exceeds %d bytes", maxReportBytes)
	}
	suffix := ".json"
	if compressed {
		suffix += ".gz"
	}
	path := filepath.Join(directory, time.Now().UTC().Format(fileTimestampFormat)+"-"+string(scanReport.Status)+suffix)
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return "", fmt.Errorf("create history report %s: %w", path, err)
	}
	writer := io.Writer(file)
	var gzipWriter *gzip.Writer
	if compressed {
		gzipWriter = gzip.NewWriter(file)
		writer = gzipWriter
	}
	if _, err := writer.Write(data); err != nil {
		_ = file.Close()
		return "", fmt.Errorf("write history report %s: %w", path, err)
	}
	if gzipWriter != nil {
		if err := gzipWriter.Close(); err != nil {
			_ = file.Close()
			return "", fmt.Errorf("close compressed history report %s: %w", path, err)
		}
	}
	if err := file.Close(); err != nil {
		return "", fmt.Errorf("close history report %s: %w", path, err)
	}
	return path, nil
}

// LoadRecent reads up to limit recent scan reports from directory newest-first.
func LoadRecent(directory string, limit int) ([]Entry, error) {
	if directory == "" || limit <= 0 {
		return nil, nil
	}
	matches, err := reportPaths(directory)
	if err != nil {
		return nil, err
	}
	sort.Sort(sort.Reverse(sort.StringSlice(matches)))
	entries := make([]Entry, 0, len(matches))
	for _, path := range matches {
		if len(entries) == limit {
			break
		}
		data, err := readReport(path)
		if err != nil {
			log.Print("skipped unreadable history report")
			continue
		}
		var scanReport report.DriftReport
		if err := json.Unmarshal(data, &scanReport); err != nil {
			log.Print("skipped malformed history report")
			continue
		}
		entries = append(entries, Entry{Report: scanReport})
	}
	return entries, nil
}

func readReport(path string) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("history report must not be a symlink")
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()
	var reader io.Reader = file
	if filepath.Ext(path) == ".gz" {
		gzipReader, err := gzip.NewReader(file)
		if err != nil {
			return nil, err
		}
		defer func() { _ = gzipReader.Close() }()
		reader = gzipReader
	}
	data, err := io.ReadAll(io.LimitReader(reader, maxReportBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxReportBytes {
		return nil, fmt.Errorf("history report exceeds %d bytes", maxReportBytes)
	}
	return data, nil
}

// Prune removes all but the newest keep history reports.
func Prune(directory string, keep int) error {
	if keep <= 0 {
		return fmt.Errorf("history retention must be greater than zero")
	}
	matches, err := reportPaths(directory)
	if err != nil {
		return err
	}
	sort.Sort(sort.Reverse(sort.StringSlice(matches)))
	for _, path := range matches[keep:] {
		if err := os.Remove(path); err != nil {
			return fmt.Errorf("remove history report %s: %w", path, err)
		}
	}
	return nil
}

func reportPaths(directory string) ([]string, error) {
	info, err := os.Lstat(directory)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("inspect history directory %s: %w", directory, err)
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("history directory must be a non-symlink directory: %s", directory)
	}
	file, err := os.Open(directory)
	if err != nil {
		return nil, fmt.Errorf("open history directory %s: %w", directory, err)
	}
	defer func() { _ = file.Close() }()
	paths := make([]string, 0)
	for {
		entries, err := file.ReadDir(100)
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") && !strings.HasSuffix(entry.Name(), ".json.gz") {
				continue
			}
			if len(paths) == maxHistoryFiles {
				return nil, fmt.Errorf("history directory exceeds %d report files", maxHistoryFiles)
			}
			paths = append(paths, filepath.Join(directory, entry.Name()))
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("list history reports %s: %w", directory, err)
		}
	}
	return paths, nil
}
