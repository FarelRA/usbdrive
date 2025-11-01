# usbdrive

Mount disk images as USB mass storage devices on Android using USB gadget mode.

Usbdrive allows you to present ISO files, disk images, or any block device image to a computer as if your Android device were a USB flash drive or CD-ROM. This is useful for installing operating systems, running live Linux distributions, or transferring large files without copying them to your device's storage.

## Features

**Modern Go implementation** - Built with Go for reliability and performance. The entire tool is a single static binary with no dependencies, making it easy to deploy and use.

**Dual backend support** - Automatically detects and uses either ConfigFS (modern kernel interface) or Sysfs (legacy Android USB interface). ConfigFS is preferred and offers more features, while Sysfs provides compatibility with older devices.

**Proper error handling** - Every operation is validated with clear error messages. The tool checks file existence, permissions, and kernel support before attempting operations, providing helpful hints when something goes wrong.

**Status checking** - Query the current mount state at any time to see which image is mounted, what mode it's using, and which backend is active.

**Read-write support** - Mount images as writable when using ConfigFS backend, allowing the connected computer to modify the image file. This is useful for creating persistent live USB installations.

**CDROM mode** - Present images as CD-ROM devices instead of removable disks. This is essential for booting certain operating system installers that expect optical media.

**Clean CLI** - Intuitive command structure with subcommands (mount, unmount, status) and flags for options. Includes dry-run mode for previewing operations and verbose mode for debugging.

## Installation

### Magisk Module (Recommended)

Usbdrive is distributed as a Magisk module for easy installation on rooted Android devices.

1. Download the latest `usbdrive-VERSION.zip` from the releases page
2. Open Magisk Manager and navigate to the Modules section
3. Tap "Install from storage" and select the downloaded ZIP file
4. Wait for the installation to complete
5. Reboot your device to activate the module
6. After reboot, open a terminal emulator and run `usbdrive --help` to verify installation

The module automatically installs the correct binary for your device architecture (ARM, ARM64, x86, or x86_64).

### Standalone Binary

If you prefer not to use Magisk or need manual installation:

1. Download the appropriate binary for your architecture from the releases page:
   - `usbdrive-VERSION-arm64-v8a.zip` (most modern devices)
   - `usbdrive-VERSION-armeabi-v7a.zip` (older 32-bit ARM devices)
   - `usbdrive-VERSION-x86_64.zip` (x86 64-bit devices)
   - `usbdrive-VERSION-x86.zip` (x86 32-bit devices)

2. Extract and install:
   ```bash
   unzip usbdrive-VERSION-arm64-v8a.zip
   su
   mount -o remount,rw /system
   cp usbdrive /system/bin/
   chmod 755 /system/bin/usbdrive
   mount -o remount,ro /system
   ```

3. Verify installation:
   ```bash
   usbdrive --help
   ```

Note: Standalone installation does not include auto-mount on boot. You'll need to manually run usbdrive commands.

## Usage

### Basic Operations

The most common use case is mounting an ISO or disk image. By default, images are mounted in read-write mode:

```bash
usbdrive mount /sdcard/ubuntu.iso
```

To mount as read-only, use the `-ro` flag:

```bash
usbdrive mount -ro /sdcard/ubuntu.iso
```

To check what's currently mounted and which backend is being used:

```bash
usbdrive status
```

When you're done, unmount the image to disconnect it from the host computer:

```bash
usbdrive unmount
```

### Read-Write Mode

Read-write mode is the default. The host computer can modify the image file:

```bash
usbdrive mount /sdcard/data.img
```

Note: Sysfs backend only supports read-only mode and will automatically force `-ro` if needed.

### CDROM Mode

Some operating system installers expect to boot from optical media. Use the `-cdrom` flag to present your image as a CD-ROM device instead of a removable disk. This is particularly useful for Windows installers:

```bash
usbdrive mount -cdrom /sdcard/windows.iso
```

Note that CDROM devices are always read-only, so you cannot combine `-cdrom` with `-rw`.

### Debugging and Testing

If something isn't working, enable verbose output to see detailed information about what the tool is doing:

```bash
usbdrive mount -v /sdcard/ubuntu.iso
```

To preview what would happen without actually mounting anything, use dry-run mode:

```bash
usbdrive mount -n /sdcard/ubuntu.iso
```

This shows you which backend would be used, the file path, and the mount mode.

### Backend Selection

Usbdrive automatically selects the best available backend (preferring ConfigFS over Sysfs and Legacy). If you need to force a specific backend for testing or compatibility reasons:

```bash
# Force ConfigFS (modern, supports all features)
usbdrive mount -f configfs /sdcard/ubuntu.iso

# Force Sysfs (legacy, read-only only)
usbdrive mount -f sysfs /sdcard/ubuntu.iso

# Force Legacy (UDC gadget, read-write only)
usbdrive mount -f legacy /sdcard/ubuntu.iso
```

### Configuration File

For automatic mounting on boot or to avoid typing long commands, you can use a JSON configuration file:

```bash
usbdrive mount -c /path/to/config.json
```

## Configuration File

Create a JSON config file for automatic mounting on boot:

```json
{
  "file": "/sdcard/ubuntu.iso",
  "mode": "rw",
  "backend": "configfs"
}
```

**Mode options:** `rw` (read-write, default), `ro` (read-only), `cdrom`  
**Backend options:** `configfs` (modern), `sysfs` (legacy), `legacy` (UDC)

The Magisk module includes an example config file at `/data/adb/modules/usbdrive/usbdrive.json.example`. Copy and edit it to enable auto-mount on boot:

```bash
# Copy example and edit
cp /data/adb/modules/usbdrive/usbdrive.json.example /data/adb/modules/usbdrive/usbdrive.json
vi /data/adb/modules/usbdrive/usbdrive.json

# Or create from scratch
cat > /data/adb/modules/usbdrive/usbdrive.json << 'EOF'
{
  "file": "/sdcard/ubuntu.iso",
  "mode": "rw"
}
EOF

# Mount using config
usbdrive mount -c /data/adb/modules/usbdrive/usbdrive.json
```

## Requirements

- Rooted Android device
- Kernel with USB gadget support (configfs or android_usb)
- USB OTG cable

## Backends

### ConfigFS (Preferred)
- Modern USB gadget interface
- Supports read-write and CDROM modes
- Available on most recent kernels
- Path: `/sys/kernel/config/usb_gadget`

### Sysfs (Android USB)
- Android-specific USB interface
- Read-only only
- Common on older Android devices
- Path: `/sys/devices/virtual/android_usb/android0`

### Legacy (UDC Gadget)
- Legacy USB Device Controller interface
- Supports read-write mode
- Found on some Qualcomm devices
- Path: `/sys/class/udc/*/device/gadget`

## Building

```bash
# Install Go 1.21+
./build.sh
```

This builds and packages:
- Magisk module with all architectures (`usbdrive-VERSION.zip`)
- Standalone binaries for each architecture:
  - ARM (armeabi-v7a)
  - ARM64 (arm64-v8a)
  - x86
  - x86_64
- SHA256 checksums for all files

All artifacts are placed in the `out/` directory.

## About

This project is a complete rewrite of [isodrive-magisk](https://github.com/nitanmarcel/isodrive-magisk) in Go. It maintains the same core functionality while adding modern error handling, better CLI design, and improved reliability.

Key differences from the original:
- Rewritten in Go (from C++)
- Three backend support (ConfigFS, Sysfs, Legacy UDC)
- Proper error handling and validation
- File existence checking before mount
- Status command to check current state
- Better CLI with subcommands
- Verbose and dry-run modes
- Read-write mode by default
- Automatic backend detection
- No memory leaks
- Smaller binary size
- Better logging
- Graceful cleanup

## License

GPL-3.0
