#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")"

# Validate module.prop exists
if [ ! -f "module/module.prop" ]; then
    echo "Error: module/module.prop not found"
    exit 1
fi

# Extract version from module.prop
VERSION=$(grep '^version=' module/module.prop | cut -d'=' -f2)
if [ -z "$VERSION" ]; then
    echo "Error: Could not extract version from module.prop"
    exit 1
fi

echo "Building usbdrive v$VERSION..."

build() {
    local abi=$1 goarch=$2
    echo "  $abi"
    mkdir -p ../libs/$abi
    GOOS=linux GOARCH=$goarch CGO_ENABLED=0 \
        go build -trimpath -ldflags="-s -w -X main.version=$VERSION" -o ../libs/$abi/usbdrive .
}

pushd src
build armeabi-v7a arm
build arm64-v8a arm64
build x86 386
build x86_64 amd64
popd

echo "Packaging module..."
rm -rf out
mkdir -p out
cp -r module out/
cp -r libs out/module/

# Create module zip (without module/ prefix in zip)
(cd out/module && zip -qr9 ../usbdrive-$VERSION.zip *)

# Package standalone binaries
echo "Packaging standalone binaries..."
for abi in armeabi-v7a arm64-v8a x86 x86_64; do
    (cd out/module/libs/$abi && zip -q9 "../../../usbdrive-$VERSION-$abi.zip" usbdrive)
done

# Cleanup
rm -rf out/module

echo "Generating checksums..."
(cd out && sha256sum *.zip > checksums.sha256)

echo "✓ Build complete:"
echo "  - Module: out/usbdrive-$VERSION.zip"
echo "  - Binaries: out/usbdrive-$VERSION-*.zip"
echo "  - Checksums: out/checksums.sha256"
