#!/bin/sh
# Complete the macOS SatSat.app bundle around an already-built binary.
#
# Usage: packaging/macos/bundle.sh <binary> [version]
#
# The darwin builds emit their binary into the bundle layout already:
#
#	<...>/SatSat.app/Contents/MacOS/satsat
#
# so the bundle root is derived from the binary path and the binary is left in
# place. Everything else (Resources, Info.plist, PkgInfo) is written here.
#
# Intended to run from a GoReleaser build post-hook on the darwin targets, from
# the repository root. The version falls back to GORELEASER_CURRENT_TAG, then
# 0.0.0.
set -eu

BIN="${1:?usage: bundle.sh <binary> [version]}"

VERSION="${2:-${GORELEASER_CURRENT_TAG:-0.0.0}}"
VERSION="${VERSION#v}"

CONTENTS="$(dirname "$(dirname "$BIN")")"

mkdir -p "$CONTENTS/Resources"

# Inside a .app, Shirei looks for resources in Contents/Resources.
cp Resources/icon.png Resources/icon.icns \
	Resources/sat_marker.png Resources/sat_picto.png \
	"$CONTENTS/Resources/"

cat >"$CONTENTS/Info.plist" <<PLIST
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

printf 'APPL????' >"$CONTENTS/PkgInfo"
