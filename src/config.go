package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type Config struct {
	File    string `json:"file"`
	Mode    string `json:"mode,omitempty"`    // "ro", "rw", "cdrom"
	Backend string `json:"backend,omitempty"` // "configfs", "sysfs"
}

func loadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config file: %w", err)
	}

	// Validate required fields
	if cfg.File == "" {
		return nil, fmt.Errorf("config missing required field: file")
	}

	// Resolve to absolute path
	if !filepath.IsAbs(cfg.File) {
		absPath, err := filepath.Abs(cfg.File)
		if err != nil {
			return nil, fmt.Errorf("resolve absolute path for '%s': %w", cfg.File, err)
		}
		cfg.File = absPath
	}

	// Validate mode if specified
	if cfg.Mode != "" && cfg.Mode != "ro" && cfg.Mode != "rw" && cfg.Mode != "cdrom" {
		return nil, fmt.Errorf("invalid mode: %s (must be ro, rw, or cdrom)", cfg.Mode)
	}

	// Validate backend if specified
	if cfg.Backend != "" && cfg.Backend != "configfs" && cfg.Backend != "sysfs" && cfg.Backend != "legacy" {
		return nil, fmt.Errorf("invalid backend: %s (must be configfs, sysfs, or legacy)", cfg.Backend)
	}

	return &cfg, nil
}
