#!/usr/bin/env bash
# Generate the Homebrew cask for a release and push it to
# akhenakh/homebrew-tap.
#
#   Casks/satsat.rb    macOS -> installs SatSat.app into /Applications
#
# Linux is not served by Homebrew; Linux users grab the release archives from
# GitHub. Any stale Formula/satsat.rb left in the tap is removed.
#
# Usage: scripts/brew-bump.sh <tag>    (e.g. scripts/brew-bump.sh v1.0.0)
#
# Requires write access to akhenakh/homebrew-tap, either via a
# TAP_GITHUB_TOKEN (a PAT with contents:write on the tap) or an SSH key.
set -euo pipefail

TAG="${1:?usage: $0 <tag>}"
VERSION="${TAG#v}"

BIN="satsat"
REPO="akhenakh/gosatsat"
DESC="Native satellite pass tracker"

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

curl -fsSL "https://github.com/${REPO}/releases/download/${TAG}/${BIN}_${VERSION}_checksums.txt" \
  -o "$TMP/checksums.txt"

sha() {
  awk -v f="$1" '$2 == f { print $1 }' "$TMP/checksums.txt"
}

DARWIN_AMD64="$(sha "${BIN}_${VERSION}_Darwin_x86_64.tar.gz")"
DARWIN_ARM64="$(sha "${BIN}_${VERSION}_Darwin_arm64.tar.gz")"

for v in DARWIN_AMD64 DARWIN_ARM64; do
  if [[ -z "${!v}" ]]; then
    echo "error: missing checksum for ${v}" >&2
    exit 1
  fi
done

# The macOS archives contain nothing but SatSat.app, so `app "SatSat.app"`
# installs the whole bundle into /Applications.
cat > "$TMP/${BIN}.rb" <<EOF
# typed: false
# frozen_string_literal: true

cask "${BIN}" do
  version "${VERSION}"
  sha256 arm:   "${DARWIN_ARM64}",
         intel: "${DARWIN_AMD64}"

  on_arm do
    url "https://github.com/${REPO}/releases/download/${TAG}/${BIN}_${VERSION}_Darwin_arm64.tar.gz"
  end
  on_intel do
    url "https://github.com/${REPO}/releases/download/${TAG}/${BIN}_${VERSION}_Darwin_x86_64.tar.gz"
  end

  name "SatSat"
  desc "${DESC}"
  homepage "https://github.com/${REPO}"

  depends_on :macos

  app "SatSat.app"

  # The release bundle is unsigned; give it an ad-hoc signature at install time
  # so macOS has a stable code identity for it.
  postflight_steps do
    run "/usr/bin/codesign", args: ["--force", "--sign", "-", "{{appdir}}/SatSat.app"]
  end
end
EOF

TAP_DIR="$TMP/tap"
if [[ -n "${TAP_GITHUB_TOKEN:-}" ]]; then
  git clone "https://x-access-token:${TAP_GITHUB_TOKEN}@github.com/akhenakh/homebrew-tap.git" "$TAP_DIR"
else
  git clone "git@github.com:akhenakh/homebrew-tap.git" "$TAP_DIR"
fi
mkdir -p "$TAP_DIR/Casks"
cp "$TMP/${BIN}.rb" "$TAP_DIR/Casks/${BIN}.rb"
# The Linux formula is gone; drop it from the tap if a previous release left it.
rm -f "$TAP_DIR/Formula/${BIN}.rb"

git -C "$TAP_DIR" config user.email "akh@inair.space"
git -C "$TAP_DIR" config user.name "Fabrice Aneche"
git -C "$TAP_DIR" add -A
git -C "$TAP_DIR" commit -m "Brew cask update for ${BIN} ${TAG}" || true
git -C "$TAP_DIR" push origin main
