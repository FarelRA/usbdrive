// Package main implements usbdrive, a tool for mounting disk images as USB mass storage devices on Android.
package main

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/spf13/cobra"
)

var version = "dev" // Injected at build time via -ldflags

var (
	logger *slog.Logger

	// mount flags
	mountRO      bool
	mountRW      bool
	mountCDROM   bool
	mountForce   string
	mountVerbose bool
	mountDryRun  bool
	mountConfig  string

	// unmount flags
	unmountForce   string
	unmountVerbose bool
	unmountDryRun  bool
)

var rootCmd = &cobra.Command{
	Use:   "usbdrive",
	Short: "Mount disk images as USB mass storage",
	Long:  "Usbdrive is a tool for mounting disk images as USB mass storage devices on Android.",
	CompletionOptions: cobra.CompletionOptions{
		DisableDefaultCmd: true,
	},
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		level := slog.LevelError
		if mountVerbose || unmountVerbose {
			level = slog.LevelInfo
		}
		logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
			Level: level,
		}))
	},
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print usbdrive version",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("usbdrive version %s\n", version)
	},
}

var mountCmd = &cobra.Command{
	Use:   "mount [flags] <file>",
	Short: "Mount a disk image as USB device",
	Long:  "Mount a disk image as USB mass storage device. Default mode is read-write.",
	Args:  cobra.MaximumNArgs(1),
	PreRunE: func(cmd *cobra.Command, args []string) error {
		if os.Geteuid() != 0 {
			return fmt.Errorf("must run as root")
		}
		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		var imagePath string
		var readWrite, useCDROM bool
		var forceBackend string

		// Load from config if -c provided
		if mountConfig != "" {
			cfg, err := loadConfig(mountConfig)
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}
			imagePath = cfg.File
			forceBackend = cfg.Backend
			
			switch cfg.Mode {
			case "ro":
				readWrite = false
			case "cdrom":
				useCDROM = true
			default:
				readWrite = true // rw is default
			}
			
			logger.Info("Loaded configuration", "path", mountConfig)
		} else {
			// Use command line args
			if len(args) < 1 {
				return fmt.Errorf("missing file argument")
			}
			imagePath = args[0]
			
			// Default is read-write unless -ro is specified
			if mountRO {
				readWrite = false
			} else if mountRW {
				readWrite = true
			} else {
				readWrite = true // default
			}
			
			useCDROM = mountCDROM
			forceBackend = mountForce
		}

		if useCDROM && readWrite {
			return fmt.Errorf("cannot use -cdrom with -rw (CDROM devices are always read-only)")
		}

		if mountRO && mountRW {
			return fmt.Errorf("cannot use -ro with -rw (conflicting flags)")
		}

		logger.Info("Validating image file", "path", imagePath)
		if err := validateImage(imagePath); err != nil {
			return fmt.Errorf("invalid image file: %w\nHint: Ensure the file exists and is readable", err)
		}

		backend, err := selectBackend(forceBackend)
		if err != nil {
			return err
		}

		// Force read-only for sysfs backend
		if backend.Name() == "sysfs" && readWrite {
			logger.Warn("Sysfs backend only supports read-only mode, forcing -ro")
			readWrite = false
		}

		mode := getMode(readWrite, useCDROM)

		if mountDryRun {
			fileInfo, _ := os.Stat(imagePath)
			fmt.Printf("Dry run: Would mount with the following settings:\n")
			fmt.Printf("  Backend: %s\n", backend.Name())
			fmt.Printf("  File: %s\n", imagePath)
			if fileInfo != nil {
				fmt.Printf("  Size: %d bytes (%.2f MB)\n", fileInfo.Size(), float64(fileInfo.Size())/1024/1024)
			}
			fmt.Printf("  Mode: %s\n", mode)
			
			// Show backend capabilities
			if backend.Name() == "configfs" {
				fmt.Printf("  Capabilities: read-write, cdrom\n")
			} else if backend.Name() == "sysfs" {
				fmt.Printf("  Capabilities: read-only\n")
			} else if backend.Name() == "udc" {
				fmt.Printf("  Capabilities: read-write (always)\n")
			}
			
			// Validate mode compatibility
			if backend.Name() == "sysfs" && (readWrite || useCDROM) {
				fmt.Printf("  WARNING: sysfs backend only supports read-only mode\n")
			}
			if backend.Name() == "udc" && useCDROM {
				fmt.Printf("  WARNING: udc backend does not support CDROM mode\n")
			}
			
			return nil
		}

		logger.Info("Preparing to mount",
			"backend", backend.Name(),
			"file", imagePath,
			"mode", mode,
		)

		opts := MountOptions{
			ReadWrite: readWrite,
			CDROM:     useCDROM,
		}

		if err := backend.Mount(imagePath, opts); err != nil {
			return fmt.Errorf("mount failed: %w\nHint: Try running with -v for verbose output", err)
		}

		logger.Info("Successfully mounted image")
		return nil
	},
}

var unmountCmd = &cobra.Command{
	Use:   "unmount [flags]",
	Short: "Unmount currently mounted image",
	Long:  "Unmount currently mounted disk image.",
	Args:  cobra.NoArgs,
	PreRunE: func(cmd *cobra.Command, args []string) error {
		if os.Geteuid() != 0 {
			return fmt.Errorf("must run as root")
		}
		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		backend, err := selectBackend(unmountForce)
		if err != nil {
			return err
		}

		if unmountDryRun {
			fmt.Printf("Dry run: Would unmount using backend: %s\n", backend.Name())
			
			// Show current status if available
			status, err := backend.Status()
			if err == nil && status.Mounted {
				fmt.Printf("  Currently mounted: %s\n", status.File)
				fmt.Printf("  Current mode: %s\n", getMode(!status.ReadOnly, status.CDROM))
			} else {
				fmt.Printf("  Status: No image currently mounted\n")
			}
			
			return nil
		}

		logger.Info("Preparing to unmount", "backend", backend.Name())

		if err := backend.Unmount(); err != nil {
			return fmt.Errorf("unmount failed: %w\nHint: Try running with -v for verbose output", err)
		}

		logger.Info("Successfully unmounted image")
		return nil
	},
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show current mount status",
	Long:  "Show current mount status including backend, file, and mount mode.",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		backends := []Backend{&ConfigFSBackend{}, &SysfsBackend{}}

		for _, backend := range backends {
			if !backend.Supported() {
				continue
			}

			status, err := backend.Status()
			if err != nil {
				logger.Warn("Failed to get status", "backend", backend.Name(), "error", err)
				continue
			}

			fmt.Printf("Backend: %s\n", backend.Name())
			if status.Mounted {
				fmt.Printf("Status: Mounted\n")
				fmt.Printf("File: %s\n", status.File)
				fmt.Printf("Mode: %s\n", getMode(!status.ReadOnly, status.CDROM))
			} else {
				fmt.Printf("Status: Not mounted\n")
			}
			return nil
		}

		fmt.Println("No active USB gadget found")
		return nil
	},
}

func main() {
	// Disable auto-generated commands
	rootCmd.CompletionOptions.DisableDefaultCmd = true
	rootCmd.SetHelpCommand(&cobra.Command{Hidden: true})

	// Mount flags
	mountCmd.Flags().SortFlags = false
	mountCmd.Flags().StringVarP(&mountConfig, "config", "c", "", "load configuration from file")
	
	mountCmd.Flags().BoolVar(&mountRW, "rw", false, "mount as read-write (default)")
	mountCmd.Flags().BoolVar(&mountRO, "ro", false, "mount as read-only")
	mountCmd.Flags().BoolVar(&mountCDROM, "cdrom", false, "mount as CDROM device")
	
	mountCmd.Flags().StringVarP(&mountForce, "force", "f", "", "force backend: configfs or sysfs")
	mountCmd.Flags().BoolVarP(&mountDryRun, "dry-run", "n", false, "preview operation without executing")
	mountCmd.Flags().BoolVarP(&mountVerbose, "verbose", "v", false, "verbose output")

	// Unmount flags
	unmountCmd.Flags().SortFlags = false
	unmountCmd.Flags().StringVarP(&unmountForce, "force", "f", "", "force backend: configfs or sysfs")
	unmountCmd.Flags().BoolVarP(&unmountDryRun, "dry-run", "n", false, "preview operation without executing")
	unmountCmd.Flags().BoolVarP(&unmountVerbose, "verbose", "v", false, "verbose output")

	// Add commands
	cobra.EnableCommandSorting = false
	rootCmd.AddCommand(mountCmd)
	rootCmd.AddCommand(unmountCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(versionCmd)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func selectBackend(force string) (Backend, error) {
	backends := []Backend{&ConfigFSBackend{}, &UDCBackend{}, &SysfsBackend{}}

	if force != "" {
		for _, b := range backends {
			if b.Name() == force {
				if b.Supported() {
					return b, nil
				}
				return nil, fmt.Errorf("backend '%s' is not supported on this device\nHint: Check if /sys/kernel/config/usb_gadget (configfs), /sys/class/android_usb (sysfs), or /sys/class/udc/*/device/gadget (udc) exists", force)
			}
		}
		return nil, fmt.Errorf("unknown backend '%s'\nHint: Valid backends are 'configfs', 'sysfs', or 'udc'", force)
	}

	for _, b := range backends {
		if b.Supported() {
			return b, nil
		}
	}

	return nil, fmt.Errorf("no supported USB gadget backend found\nHint: Your kernel may not support USB gadget mode. Check if configfs, android_usb, or UDC gadget is available")
}

func getMode(rw, cdrom bool) string {
	if cdrom {
		return "cdrom"
	}
	if rw {
		return "read-write"
	}
	return "read-only"
}
