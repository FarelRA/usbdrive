package main

import (
	"fmt"
	"os"
	"path/filepath"
)

type ConfigFSBackend struct{}

func (c *ConfigFSBackend) Name() string {
	return "configfs"
}

func (c *ConfigFSBackend) Supported() bool {
	mountPoint := findMountPoint("configfs")
	if mountPoint == "" {
		return false
	}
	gadgetPath := filepath.Join(mountPoint, "usb_gadget")
	return dirExists(gadgetPath)
}

func (c *ConfigFSBackend) Mount(imagePath string, opts MountOptions) error {
	gadgetRoot, err := c.findGadgetRoot()
	if err != nil {
		return fmt.Errorf("find gadget: %w", err)
	}
	logger.Info("Found USB gadget", "path", gadgetRoot)

	configRoot, err := c.findConfigRoot(gadgetRoot)
	if err != nil {
		return fmt.Errorf("find config: %w", err)
	}

	udc, err := c.getUDC(gadgetRoot)
	if err != nil {
		return fmt.Errorf("get UDC: %w", err)
	}
	logger.Info("Current UDC controller", "udc", udc)

	// Disable UDC
	logger.Info("Disabling UDC")
	if err := c.setUDC(gadgetRoot, ""); err != nil {
		return fmt.Errorf("disable UDC: %w", err)
	}

	// Ensure UDC is re-enabled even on failure
	defer func() {
		if udc != "" {
			logger.Info("Re-enabling UDC (cleanup)", "udc", udc)
			c.setUDC(gadgetRoot, udc)
		}
	}()

	functionRoot := filepath.Join(gadgetRoot, "functions")
	massStorageRoot := filepath.Join(functionRoot, "mass_storage.0")
	lunRoot := filepath.Join(massStorageRoot, "lun.0")

	// Create mass storage function if needed
	if !dirExists(massStorageRoot) {
		logger.Info("Creating mass storage function")
		if err := os.MkdirAll(massStorageRoot, 0755); err != nil {
			return fmt.Errorf("create mass_storage function: %w", err)
		}
	}

	// Link to config if needed
	configLink := filepath.Join(configRoot, "mass_storage.0")
	if !pathExists(configLink) {
		logger.Info("Linking mass storage to config")
		if err := os.Symlink(massStorageRoot, configLink); err != nil {
			return fmt.Errorf("link mass_storage to config: %w", err)
		}
	}

	// Clear existing file
	lunFile := filepath.Join(lunRoot, "file")
	if err := writeFile(lunFile, ""); err != nil {
		return fmt.Errorf("clear lun file: %w", err)
	}

	// Set CDROM flag
	cdromValue := "0"
	if opts.CDROM {
		cdromValue = "1"
	}
	logger.Info("Setting CDROM flag", "value", cdromValue)
	if err := writeFile(filepath.Join(lunRoot, "cdrom"), cdromValue); err != nil {
		return fmt.Errorf("set cdrom flag: %w", err)
	}

	// Set read-only flag
	roValue := "1"
	if opts.ReadWrite {
		roValue = "0"
	}
	logger.Info("Setting read-only flag", "value", roValue)
	if err := writeFile(filepath.Join(lunRoot, "ro"), roValue); err != nil {
		return fmt.Errorf("set ro flag: %w", err)
	}

	// Mount the image
	logger.Info("Writing image path to LUN")
	if err := writeFile(lunFile, imagePath); err != nil {
		return fmt.Errorf("mount image: %w", err)
	}

	// Verify mount succeeded
	logger.Info("Verifying mount")
	if err := verifyMount(lunFile, imagePath); err != nil {
		return fmt.Errorf("verify mount: %w", err)
	}

	logger.Info("Mount verified successfully")
	return nil
}

func (c *ConfigFSBackend) Unmount() error {
	gadgetRoot, err := c.findGadgetRoot()
	if err != nil {
		return fmt.Errorf("find gadget: %w", err)
	}

	udc, err := c.getUDC(gadgetRoot)
	if err != nil {
		return fmt.Errorf("get UDC: %w", err)
	}

	// Disable UDC
	logger.Info("Disabling UDC")
	if err := c.setUDC(gadgetRoot, ""); err != nil {
		return fmt.Errorf("disable UDC: %w", err)
	}

	// Ensure UDC is re-enabled even on failure
	defer func() {
		if udc != "" {
			logger.Info("Re-enabling UDC (cleanup)", "udc", udc)
			c.setUDC(gadgetRoot, udc)
		}
	}()

	massStorageRoot := filepath.Join(gadgetRoot, "functions", "mass_storage.0")
	lunFile := filepath.Join(massStorageRoot, "lun.0", "file")

	// Clear the file
	logger.Info("Clearing LUN file")
	if err := writeFile(lunFile, ""); err != nil {
		return fmt.Errorf("clear lun file: %w", err)
	}

	// Verify unmount
	logger.Info("Verifying unmount")
	if err := verifyUnmount(lunFile); err != nil {
		return fmt.Errorf("verify unmount: %w", err)
	}

	logger.Info("Unmount verified successfully")
	return nil
}

func (c *ConfigFSBackend) Status() (*MountStatus, error) {
	gadgetRoot, err := c.findGadgetRoot()
	if err != nil {
		return &MountStatus{Mounted: false}, nil
	}

	massStorageRoot := filepath.Join(gadgetRoot, "functions", "mass_storage.0")
	if !dirExists(massStorageRoot) {
		return &MountStatus{Mounted: false}, nil
	}

	lunRoot := filepath.Join(massStorageRoot, "lun.0")
	lunFile := filepath.Join(lunRoot, "file")

	file, err := readFile(lunFile)
	if err != nil || file == "" {
		return &MountStatus{Mounted: false}, nil
	}

	cdrom, _ := readFile(filepath.Join(lunRoot, "cdrom"))
	ro, _ := readFile(filepath.Join(lunRoot, "ro"))

	return &MountStatus{
		Mounted:  true,
		File:     file,
		ReadOnly: ro == "1",
		CDROM:    cdrom == "1",
	}, nil
}

func (c *ConfigFSBackend) findGadgetRoot() (string, error) {
	mountPoint := findMountPoint("configfs")
	if mountPoint == "" {
		return "", fmt.Errorf("configfs not mounted")
	}

	gadgetDir := filepath.Join(mountPoint, "usb_gadget")
	if !dirExists(gadgetDir) {
		return "", fmt.Errorf("usb_gadget directory not found")
	}
	
	entries, err := os.ReadDir(gadgetDir)
	if err != nil {
		return "", fmt.Errorf("read gadget dir: %w", err)
	}

	// Look for existing active gadget
	for _, entry := range entries {
		if entry.Name()[0] == '.' {
			continue
		}

		gadgetPath := filepath.Join(gadgetDir, entry.Name())
		udcFile := filepath.Join(gadgetPath, "UDC")

		if udc, _ := readFile(udcFile); udc != "" {
			return gadgetPath, nil
		}
	}

	return "", fmt.Errorf("no active gadget found")
}

func (c *ConfigFSBackend) findConfigRoot(gadgetRoot string) (string, error) {
	configDir := filepath.Join(gadgetRoot, "configs")
	entries, err := os.ReadDir(configDir)
	if err != nil {
		return "", fmt.Errorf("read configs: %w", err)
	}

	for _, entry := range entries {
		if entry.Name()[0] != '.' {
			return filepath.Join(configDir, entry.Name()), nil
		}
	}

	return "", fmt.Errorf("no config found")
}

func (c *ConfigFSBackend) getUDC(gadgetRoot string) (string, error) {
	udcFile := filepath.Join(gadgetRoot, "UDC")
	udc, err := readFile(udcFile)
	if err != nil {
		return "", err
	}
	return udc, nil
}

func (c *ConfigFSBackend) setUDC(gadgetRoot, udc string) error {
	udcFile := filepath.Join(gadgetRoot, "UDC")
	return writeFile(udcFile, udc)
}
