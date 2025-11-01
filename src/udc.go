package main

import (
	"fmt"
	"os"
	"path/filepath"
)

type UDCBackend struct{}

func (u *UDCBackend) Name() string {
	return "udc"
}

func (u *UDCBackend) Supported() bool {
	// Check for UDC gadget interface
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

func (u *UDCBackend) Mount(imagePath string, opts MountOptions) error {
	if opts.CDROM {
		logger.Warn("UDC backend ignores -cdrom flag")
	}

	lunFile, err := u.findLunFile()
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

func (u *UDCBackend) Unmount() error {
	lunFile, err := u.findLunFile()
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

func (u *UDCBackend) Status() (*MountStatus, error) {
	lunFile, err := u.findLunFile()
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
		ReadOnly: false, // UDC always read-write (ro flag is always 0)
		CDROM:    false,
	}, nil
}

func (u *UDCBackend) findLunFile() (string, error) {
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
