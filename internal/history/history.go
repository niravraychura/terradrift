// Package history stores local scan report history for self-hosted trend views.
package history

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/niravraychura/terradrift/internal/report"
)

const fileTimestampFormat = "20060102T150405.000000000Z"

// Entry is a persisted scan report.
type Entry struct {
	Report report.DriftReport
}

// Write stores a scan report as a secure JSON file in directory.
func Write(directory string, scanReport report.DriftReport) (string, error) {
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
	path := filepath.Join(directory, time.Now().UTC().Format(fileTimestampFormat)+"-"+string(scanReport.Status)+".json")
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return "", fmt.Errorf("create history report %s: %w", path, err)
	}
	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		return "", fmt.Errorf("write history report %s: %w", path, err)
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
	matches, err := filepath.Glob(filepath.Join(directory, "*.json"))
	if err != nil {
		return nil, fmt.Errorf("list history reports %s: %w", directory, err)
	}
	sort.Sort(sort.Reverse(sort.StringSlice(matches)))
	if len(matches) > limit {
		matches = matches[:limit]
	}
	entries := make([]Entry, 0, len(matches))
	for _, path := range matches {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read history report %s: %w", path, err)
		}
		var scanReport report.DriftReport
		if err := json.Unmarshal(data, &scanReport); err != nil {
			return nil, fmt.Errorf("parse history report %s: %w", path, err)
		}
		entries = append(entries, Entry{Report: scanReport})
	}
	return entries, nil
}

// Prune removes all but the newest keep history reports.
func Prune(directory string, keep int) error {
	if keep <= 0 {
		return fmt.Errorf("history retention must be greater than zero")
	}
	matches, err := filepath.Glob(filepath.Join(directory, "*.json"))
	if err != nil {
		return fmt.Errorf("list history reports %s: %w", directory, err)
	}
	sort.Sort(sort.Reverse(sort.StringSlice(matches)))
	for _, path := range matches[keep:] {
		if err := os.Remove(path); err != nil {
			return fmt.Errorf("remove history report %s: %w", path, err)
		}
	}
	return nil
}
