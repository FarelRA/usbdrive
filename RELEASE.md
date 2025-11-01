# Release Guide

## Building a Release

```bash
./build.sh
```

This generates:
- `out/usbdrive-VERSION.zip` - Magisk module (all architectures)
- `out/usbdrive-VERSION-arm64-v8a.zip` - Standalone ARM64 binary
- `out/usbdrive-VERSION-armeabi-v7a.zip` - Standalone ARM binary
- `out/usbdrive-VERSION-x86_64.zip` - Standalone x86_64 binary
- `out/usbdrive-VERSION-x86.zip` - Standalone x86 binary
- `out/checksums.sha256` - SHA256 checksums for all files

## Creating a GitHub Release

1. Update version in `module/module.prop`
2. Commit changes: `git commit -am "Release vX.Y.Z"`
3. Create tag: `git tag vX.Y.Z`
4. Push: `git push && git push --tags`
5. GitHub Actions will automatically create the release

## Manual Release

If creating a release manually:

1. Build: `./build.sh`
2. Create GitHub release with tag `vX.Y.Z`
3. Upload all files from `out/` directory
4. Include checksums in release notes

## Verifying Downloads

Users can verify downloads using:

```bash
sha256sum -c checksums.sha256
```

Or for individual files:

```bash
sha256sum usbdrive-3.0.0-arm64-v8a.zip
# Compare with checksums.sha256
```
