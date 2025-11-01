package main

import "fmt"

const (
	sysfsEnable   = "/sys/devices/virtual/android_usb/android0/enable"
	sysfsFile     = "/sys/devices/virtual/android_usb/android0/f_mass_storage/lun/file"
	sysfsFeatures = "/sys/devices/virtual/android_usb/android0/functions"
)

type SysfsBackend struct{}

func (s *SysfsBackend) Name() string {
	return "sysfs"
}

func (s *SysfsBackend) Supported() bool {
	return fileExists(sysfsEnable)
}

func (s *SysfsBackend) Mount(isoPath string, opts MountOptions) error {
	if opts.CDROM {
		logger.Warn("Sysfs backend does not support CDROM mode, ignoring -cdrom flag")
	}
	if opts.ReadWrite {
		logger.Warn("Sysfs backend does not support read-write mode, ignoring -rw flag")
	}

	// Disable USB
	logger.Info("Disabling USB")
	if err := s.setEnabled(false); err != nil {
		return fmt.Errorf("disable USB: %w", err)
	}

	// Set image file
	logger.Info("Setting image file path")
	if err := writeFile(sysfsFile, isoPath); err != nil {
		return fmt.Errorf("set image file: %w", err)
	}

	// Set mass_storage function
	logger.Info("Setting mass_storage function")
	if err := writeFile(sysfsFeatures, "mass_storage"); err != nil {
		return fmt.Errorf("set mass_storage function: %w", err)
	}

	// Enable USB
	logger.Info("Enabling USB")
	if err := s.setEnabled(true); err != nil {
		return fmt.Errorf("enable USB: %w", err)
	}

	return nil
}

func (s *SysfsBackend) Unmount() error {
	// Disable USB
	logger.Info("Disabling USB")
	if err := s.setEnabled(false); err != nil {
		return fmt.Errorf("disable USB: %w", err)
	}

	// Clear image file
	logger.Info("Clearing image file path")
	if err := writeFile(sysfsFile, ""); err != nil {
		return fmt.Errorf("clear image file: %w", err)
	}

	// Reset to MTP
	logger.Info("Resetting to MTP mode")
	if err := writeFile(sysfsFeatures, "mtp"); err != nil {
		return fmt.Errorf("reset to MTP: %w", err)
	}

	// Enable USB
	logger.Info("Enabling USB")
	if err := s.setEnabled(true); err != nil {
		return fmt.Errorf("enable USB: %w", err)
	}

	return nil
}

func (s *SysfsBackend) Status() (*MountStatus, error) {
	file, err := readFile(sysfsFile)
	if err != nil || file == "" {
		return &MountStatus{Mounted: false}, nil
	}

	return &MountStatus{
		Mounted:  true,
		File:     file,
		ReadOnly: true, // sysfs always read-only
		CDROM:    false,
	}, nil
}

func (s *SysfsBackend) setEnabled(enabled bool) error {
	value := "0"
	if enabled {
		value = "1"
	}
	return writeFile(sysfsEnable, value)
}
