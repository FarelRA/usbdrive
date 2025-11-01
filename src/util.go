package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func findMountPoint(fsType string) string {
	file, err := os.Open("/proc/mounts")
	if err != nil {
		return ""
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) >= 3 && fields[0] == fsType {
			return fields[1]
		}
	}

	// Fallback for Android
	if fsType == "configfs" {
		if dirExists("/sys/kernel/config") {
			return "/sys/kernel/config"
		}
		if dirExists("/config") {
			return "/config"
		}
	}

	return ""
}

func readFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

func writeFile(path, content string) error {
	return os.WriteFile(path, []byte(content+"\n"), 0644)
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func pathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func validateImage(path string) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("invalid path: %w", err)
	}

	// Check for sensitive paths
	if err := validateSafePath(absPath); err != nil {
		return err
	}

	info, err := os.Stat(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("file does not exist: %s", absPath)
		}
		return fmt.Errorf("cannot access file: %w", err)
	}

	if info.IsDir() {
		return fmt.Errorf("path is a directory: %s", absPath)
	}

	if info.Size() == 0 {
		return fmt.Errorf("file is empty: %s", absPath)
	}

	// Check if file is readable
	file, err := os.Open(absPath)
	if err != nil {
		return fmt.Errorf("file not readable: %w", err)
	}
	file.Close()

	return nil
}

func validateSafePath(path string) error {
	// Disallow system directories
	dangerousPaths := []string{
		"/system", "/sys", "/proc", "/dev",
		"/etc", "/bin", "/sbin", "/boot",
		"/root", "/data/system", "/data/data",
	}

	for _, dangerous := range dangerousPaths {
		if strings.HasPrefix(path, dangerous) {
			return fmt.Errorf("cannot mount files from system directory: %s", dangerous)
		}
	}

	// Check if it's a symlink to a dangerous location
	if linkTarget, err := os.Readlink(path); err == nil {
		if filepath.IsAbs(linkTarget) {
			return validateSafePath(linkTarget)
		}
		// Resolve relative symlink
		dir := filepath.Dir(path)
		resolved := filepath.Join(dir, linkTarget)
		return validateSafePath(resolved)
	}

	return nil
}

// verifyMount checks if the file was successfully mounted
func verifyMount(lunFile, expectedPath string) error {
	mountedPath, err := readFile(lunFile)
	if err != nil {
		return fmt.Errorf("failed to read LUN file: %w", err)
	}
	if mountedPath != expectedPath {
		return fmt.Errorf("expected %s, got %s", expectedPath, mountedPath)
	}
	return nil
}

// verifyUnmount checks if the file was successfully unmounted
func verifyUnmount(lunFile string) error {
	content, err := readFile(lunFile)
	if err != nil {
		return err
	}
	if content != "" {
		return fmt.Errorf("LUN file not empty")
	}
	return nil
}

