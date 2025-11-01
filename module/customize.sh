#!/system/bin/sh

ui_print "- Installing usbdrive"

# Pre-flight checks
ui_print "- Running pre-flight checks"

# Check Magisk version
if [ -z "$MAGISK_VER_CODE" ] || [ "$MAGISK_VER_CODE" -lt 20400 ]; then
    abort "! Magisk 20.4+ required (current: ${MAGISK_VER_CODE:-unknown})"
fi

# Check for USB gadget support
if [ ! -d "/sys/kernel/config" ] && [ ! -d "/sys/class/android_usb" ]; then
    ui_print "! WARNING: No USB gadget interface detected"
    ui_print "! Device may not support USB gadget mode"
    ui_print "! Functionality may be limited"
fi

# Detect architecture
api_level_arch_detect

# Check if architecture is supported
if [ ! -d "$MODPATH/libs/$ABI" ]; then
    abort "! Architecture $ABI not supported"
fi

# Install binary
ui_print "- Installing binary for $ABI"
mkdir -p "$MODPATH/system/bin"
cp -af "$MODPATH/libs/$ABI/"* "$MODPATH/system/bin"

# Verify binary was copied
if [ ! -f "$MODPATH/system/bin/usbdrive" ]; then
    abort "! Failed to install usbdrive binary"
fi

# Cleanup
rm -rf "$MODPATH/libs"

# Set permissions
chcon -R u:object_r:system_file:s0 "$MODPATH/system" 2>/dev/null
chmod 755 "$MODPATH/system/bin/usbdrive"

ui_print "- Installation complete"
ui_print "- Run 'usbdrive --help' for usage"
