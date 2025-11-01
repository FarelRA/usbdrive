package main

import (
	"fmt"
	"os"
	"path/filepath"
)

type LegacyBackend struct{}

func (l *LegacyBackend) Name() string {
	return "legacy"
}

func (l *LegacyBackend) Supported() bool {
	// Check for legacy UDC gadget interface
	udcDir := "/sys/class/udc"
	if !dirExists(udcDir) {
		return false
	}

	entries, err := os.ReadDir(udcDir)
	if err != nil {
		return false
	}

	// Look for any UDC with gadget/lun0/file
	for _, entry := range entries {
		lunFile := filepath.Join(udcDir, entry.Name(), "device/gadget/lun0/file")
		if fileExists(lunFile) {
			return true
		}
	}

	return false
}

func (l *LegacyBackend) Mount(imagePath string, opts MountOptions) error {
	if opts.CDROM || opts.ReadWrite {
		logger.Warn("Legacy backend ignores -cdrom and -rw flags")
	}

	lunFile, err := l.findLunFile()
	if err != nil {
		return fmt.Errorf("find lun file: %w", err)
	}

	// Clear existing file
	logger.Info("Clearing LUN file")
	if err := writeFile(lunFile, ""); err != nil {
		return fmt.Errorf("clear lun file: %w", err)
	}

	// Mount the image
	logger.Info("Writing image path to LUN")
	if err := writeFile(lunFile, imagePath); err != nil {
		return fmt.Errorf("mount image: %w", err)
	}

	// Verify mount
	logger.Info("Verifying mount")
	mountedPath, err := readFile(lunFile)
	if err != nil {
		return fmt.Errorf("verify mount: %w", err)
	}
	if mountedPath != imagePath {
		return fmt.Errorf("verify mount: expected %s, got %s", imagePath, mountedPath)
	}

	logger.Info("Mount verified successfully")
	return nil
}

func (l *LegacyBackend) Unmount() error {
	lunFile, err := l.findLunFile()
	if err != nil {
		return fmt.Errorf("find lun file: %w", err)
	}

	// Clear the file
	logger.Info("Clearing LUN file")
	if err := writeFile(lunFile, ""); err != nil {
		return fmt.Errorf("clear lun file: %w", err)
	}

	// Verify unmount
	logger.Info("Verifying unmount")
	content, err := readFile(lunFile)
	if err != nil {
		return fmt.Errorf("verify unmount: %w", err)
	}
	if content != "" {
		return fmt.Errorf("verify unmount: LUN file not empty")
	}

	logger.Info("Unmount verified successfully")
	return nil
}

func (l *LegacyBackend) Status() (*MountStatus, error) {
	lunFile, err := l.findLunFile()
	if err != nil {
		return &MountStatus{Mounted: false}, nil
	}

	file, err := readFile(lunFile)
	if err != nil || file == "" {
		return &MountStatus{Mounted: false}, nil
	}

	return &MountStatus{
		Mounted:  true,
		File:     file,
		ReadOnly: true, // legacy always read-only
		CDROM:    false,
	}, nil
}

func (l *LegacyBackend) findLunFile() (string, error) {
	udcDir := "/sys/class/udc"
	entries, err := os.ReadDir(udcDir)
	if err != nil {
		return "", fmt.Errorf("read udc dir: %w", err)
	}

	for _, entry := range entries {
		lunFile := filepath.Join(udcDir, entry.Name(), "device/gadget/lun0/file")
		if fileExists(lunFile) {
			return lunFile, nil
		}
	}

	return "", fmt.Errorf("no lun file found")
}
