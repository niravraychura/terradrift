package config

import (
	"encoding/json"
	"fmt"
	"os"
)

const DefaultPath = ".terradrift.json"

// Config stores repeatable TerraDrift CLI settings for local and CI usage.
type Config struct {
	Directory   string `json:"directory"`
	Output      string `json:"output"`
	Timeout     string `json:"timeout"`
	RedactPaths bool   `json:"redact_paths"`
}

// Default returns a safe bootstrap configuration.
func Default() Config {
	return Config{Directory: ".", Output: "table", Timeout: "5m", RedactPaths: false}
}

// Load reads a TerraDrift JSON configuration file.
func Load(path string) (Config, error) {
	if path == "" {
		path = DefaultPath
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read config %s: %w", path, err)
	}
	cfg := Default()
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse config %s: %w", path, err)
	}
	return cfg, nil
}

// WriteDefault writes the default configuration to path without overwriting existing files.
func WriteDefault(path string) error {
	if path == "" {
		path = DefaultPath
	}
	data, err := json.MarshalIndent(Default(), "", "  ")
	if err != nil {
		return fmt.Errorf("encode default config: %w", err)
	}
	data = append(data, '\n')
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		if os.IsExist(err) {
			return fmt.Errorf("config already exists: %s", path)
		}
		return fmt.Errorf("write config %s: %w", path, err)
	}
	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		return fmt.Errorf("write config %s: %w", path, err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close config %s: %w", path, err)
	}
	return nil
}
