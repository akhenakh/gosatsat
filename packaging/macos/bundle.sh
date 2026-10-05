#!/bin/sh
# Assemble a macOS SatSat.app bundle from a built binary.
#
# Usage: packaging/macos/bundle.sh <binary> <output-dir> [version]
#
# Intended to run from a GoReleaser build post-hook on the darwin targets.
# The version falls back to GORELEASER_CURRENT_TAG, then 0.0.0.
set -eu

BIN="${1:?usage: bundle.sh <binary> <output-dir> [version]}"
OUTDIR="${2:?usage: bundle.sh <binary> <output-dir> [version]}"

VERSION="${3:-${GORELEASER_CURRENT_TAG:-0.0.0}}"
VERSION="${VERSION#v}"

APP="$OUTDIR/SatSat.app"
rm -rf "$APP"
mkdir -p "$APP/Contents/MacOS" "$APP/Contents/Resources"

cp "$BIN" "$APP/Contents/MacOS/satsat"
chmod 0755 "$APP/Contents/MacOS/satsat"

# Inside a .app, Shirei looks for resources in Contents/Resources.
cp Resources/icon.png Resources/icon.icns \
	Resources/sat_marker.png Resources/sat_picto.png \
	"$APP/Contents/Resources/"

cat >"$APP/Contents/Info.plist" <<PLIST
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>CFBundleName</key>
	<string>SatSat</string>
	<key>CFBundleDisplayName</key>
	<string>SatSat</string>
	<key>CFBundleIdentifier</key>
	<string>space.inair.satsat</string>
	<key>CFBundleExecutable</key>
	<string>satsat</string>
	<key>CFBundleIconFile</key>
	<string>icon.icns</string>
	<key>CFBundlePackageType</key>
	<string>APPL</string>
	<key>CFBundleInfoDictionaryVersion</key>
	<string>6.0</string>
	<key>CFBundleShortVersionString</key>
	<string>${VERSION}</string>
	<key>CFBundleVersion</key>
	<string>${VERSION}</string>
	<key>LSMinimumSystemVersion</key>
	<string>11.0</string>
	<key>NSHighResolutionCapable</key>
	<true/>
	<key>NSPrincipalClass</key>
	<string>NSApplication</string>
</dict>
</plist>
PLIST

printf 'APPL????' >"$APP/Contents/PkgInfo"
