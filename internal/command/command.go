// Package command validates explicit external command configuration.
package command

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// Validate ensures an external command is allowed and resolves within a trusted boundary.
func Validate(value string, allowed []string, trustedDirs []string) error {
	if value == "" || strings.TrimSpace(value) != value || strings.ContainsAny(value, " \t\n;&|$<>`*?[]{}()") {
		return fmt.Errorf("external command must be one executable name or path")
	}
	if len(allowed) > 0 && !contains(allowed, value) {
		return fmt.Errorf("external command %q is not allowed", value)
	}
	if len(allowed) > 0 && len(trustedDirs) == 0 {
		return fmt.Errorf("trusted command directories are required with an allowlist")
	}
	resolved := value
	if !filepath.IsAbs(value) {
		if filepath.Base(value) != value {
			return fmt.Errorf("external command must be bare or absolute")
		}
		path, err := exec.LookPath(value)
		if err != nil {
			return fmt.Errorf("locate external command %q: %w", value, err)
		}
		resolved = path
	}
	resolved, err := filepath.EvalSymlinks(resolved)
	if err != nil {
		return fmt.Errorf("resolve external command %q: %w", value, err)
	}
	for _, directory := range trustedDirs {
		resolvedDirectory, err := filepath.EvalSymlinks(directory)
		if err != nil {
			return fmt.Errorf("resolve trusted command directory %q: %w", directory, err)
		}
		relative, err := filepath.Rel(resolvedDirectory, resolved)
		if err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) && !filepath.IsAbs(relative) {
			return nil
		}
	}
	if len(trustedDirs) == 0 {
		return nil // ponytail: direct CLI commands are an explicit local-trust choice.
	}
	return fmt.Errorf("external command %q is outside trusted command directories", value)
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
