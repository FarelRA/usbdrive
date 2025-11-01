#!/system/bin/sh
# Service script runs on boot
#
# This script automatically mounts a disk image on boot if a configuration
# file exists at $MODDIR/usbdrive.json
#
# Behavior:
# 1. Waits for boot to complete (sys.boot_completed=1)
# 2. Attempts to mount configfs
# 3. Checks if usbdrive binary is installed
# 4. Looks for config file at $MODDIR/usbdrive.json
# 5. If config exists, mounts the image specified in the config
# 6. Logs all operations to system log (tag: usbdrive)
#
# To enable auto-mount on boot:
#   1. Create $MODDIR/usbdrive.json with your desired configuration
#   2. Reboot your device
#
# To disable auto-mount:
#   - Remove or rename $MODDIR/usbdrive.json

MODDIR=${0%/*}
CONFIG="$MODDIR/usbdrive.json"

# Wait for boot to complete
until [ "$(getprop sys.boot_completed)" = "1" ]; do
    sleep 1
done

# Mount configfs
if [ ! -d "/sys/kernel/config/usb_gadget" ]; then
    mount -t configfs configfs /sys/kernel/config 2>/dev/null
    if [ $? -eq 0 ] && [ -d "/sys/kernel/config/usb_gadget" ]; then
        logger -t usbdrive "Mounted configfs"
    fi
fi

# Check if usbdrive binary exists
if [ ! -x "/system/bin/usbdrive" ]; then
    logger -t usbdrive "ERROR: usbdrive binary not found"
    exit 1
fi

# Check if config exists
if [ ! -f "$CONFIG" ]; then
    logger -t usbdrive "No config file found at $CONFIG"
    exit 0
fi

# Auto-mount using config file
logger -t usbdrive "Loading config from $CONFIG"
/system/bin/usbdrive mount -c "$CONFIG" 2>&1 | logger -t usbdrive

if [ $? -eq 0 ]; then
    logger -t usbdrive "Successfully mounted image"
else
    logger -t usbdrive "ERROR: Failed to mount image"
fi
