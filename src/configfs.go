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
	// Check if we can access the configfs mount point
	return dirExists(mountPoint)
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
	mountedPath, err := readFile(lunFile)
	if err != nil {
		return fmt.Errorf("verify mount: failed to read LUN file: %w", err)
	}
	if mountedPath != imagePath {
		return fmt.Errorf("verify mount: expected %s, got %s", imagePath, mountedPath)
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
	
	// Create usb_gadget directory if it doesn't exist
	if !dirExists(gadgetDir) {
		if err := os.MkdirAll(gadgetDir, 0755); err != nil {
			return "", fmt.Errorf("create usb_gadget dir: %w", err)
		}
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

	// No active gadget found, create one
	gadgetPath := filepath.Join(gadgetDir, "g1")
	if !dirExists(gadgetPath) {
		logger.Info("Creating new USB gadget", "path", gadgetPath)
		if err := os.MkdirAll(gadgetPath, 0755); err != nil {
			return "", fmt.Errorf("create gadget: %w", err)
		}
		
		// Set basic USB device descriptors
		if err := writeFile(filepath.Join(gadgetPath, "idVendor"), "0x18d1"); err != nil {
			return "", fmt.Errorf("set idVendor: %w", err)
		}
		if err := writeFile(filepath.Join(gadgetPath, "idProduct"), "0x4e26"); err != nil {
			return "", fmt.Errorf("set idProduct: %w", err)
		}
		
		// Create strings
		stringsDir := filepath.Join(gadgetPath, "strings/0x409")
		if err := os.MkdirAll(stringsDir, 0755); err != nil {
			return "", fmt.Errorf("create strings dir: %w", err)
		}
		if err := writeFile(filepath.Join(stringsDir, "serialnumber"), "123456"); err != nil {
			return "", fmt.Errorf("set serialnumber: %w", err)
		}
		if err := writeFile(filepath.Join(stringsDir, "manufacturer"), "Android"); err != nil {
			return "", fmt.Errorf("set manufacturer: %w", err)
		}
		if err := writeFile(filepath.Join(stringsDir, "product"), "USB Drive"); err != nil {
			return "", fmt.Errorf("set product: %w", err)
		}
		
		// Create config
		configDir := filepath.Join(gadgetPath, "configs/c.1")
		if err := os.MkdirAll(configDir, 0755); err != nil {
			return "", fmt.Errorf("create config dir: %w", err)
		}
		configStringsDir := filepath.Join(configDir, "strings/0x409")
		if err := os.MkdirAll(configStringsDir, 0755); err != nil {
			return "", fmt.Errorf("create config strings dir: %w", err)
		}
		if err := writeFile(filepath.Join(configStringsDir, "configuration"), "Config 1"); err != nil {
			return "", fmt.Errorf("set configuration: %w", err)
		}
		
		// Enable the gadget with first available UDC
		udcList, err := os.ReadDir(filepath.Join(mountPoint, "../devices"))
		if err == nil && len(udcList) > 0 {
			for _, udc := range udcList {
				if udc.Name()[0] != '.' {
					if err := writeFile(filepath.Join(gadgetPath, "UDC"), udc.Name()); err == nil {
						logger.Info("Enabled USB gadget", "udc", udc.Name())
						break
					}
				}
			}
		}
	}

	return gadgetPath, nil
}

func (c *ConfigFSBackend) findConfigRoot(gadgetRoot string) (string, error) {
	configDir := filepath.Join(gadgetRoot, "configs")
	
	// Create configs directory if it doesn't exist
	if !dirExists(configDir) {
		if err := os.MkdirAll(configDir, 0755); err != nil {
			return "", fmt.Errorf("create configs dir: %w", err)
		}
	}
	
	entries, err := os.ReadDir(configDir)
	if err != nil {
		return "", fmt.Errorf("read configs: %w", err)
	}

	for _, entry := range entries {
		if entry.Name()[0] != '.' {
			return filepath.Join(configDir, entry.Name()), nil
		}
	}

	// No config found, use c.1 (should have been created by findGadgetRoot)
	return filepath.Join(configDir, "c.1"), nil
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
