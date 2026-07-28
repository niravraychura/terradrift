// Package command validates explicit external command configuration.
package command

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

func Validate(value string, allowed []string, trustedDirs []string) error {
	if value == "" || strings.TrimSpace(value) != value || strings.ContainsAny(value, " \t\n;&|$<>`*?[]{}()") {
		return fmt.Errorf("external command must be one executable name or path")
	}
	if len(allowed) > 0 && !contains(allowed, value) {
		return fmt.Errorf("external command %q is not allowed", value)
	}
	if !filepath.IsAbs(value) {
		if filepath.Base(value) != value {
			return fmt.Errorf("external command must be bare or absolute")
		}
		if _, err := exec.LookPath(value); err != nil {
			return fmt.Errorf("locate external command %q: %w", value, err)
		}
		return nil
	}
	for _, directory := range trustedDirs {
		relative, err := filepath.Rel(directory, value)
		if err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) && !filepath.IsAbs(relative) {
			return nil
		}
	}
	return fmt.Errorf("absolute external command %q is outside trusted command directories", value)
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
