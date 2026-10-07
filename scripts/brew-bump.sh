#!/usr/bin/env bash
# Generate the Homebrew packages for a release and push them to
# akhenakh/homebrew-tap:
#
#   Casks/satsat.rb    macOS  -> installs SatSat.app into /Applications
#   Formula/satsat.rb  Linux  -> installs the satsat command-line binary
#
# Usage: scripts/brew-bump.sh <tag>    (e.g. scripts/brew-bump.sh v1.0.0)
#
# Requires write access to akhenakh/homebrew-tap, either via a
# TAP_GITHUB_TOKEN (a PAT with contents:write on the tap) or an SSH key.
set -euo pipefail

TAG="${1:?usage: $0 <tag>}"
VERSION="${TAG#v}"

BIN="satsat"
CLASS="Satsat"
REPO="akhenakh/gosatsat"
DESC="Native satellite pass tracker"
LICENSE="MIT"

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

curl -fsSL "https://github.com/${REPO}/releases/download/${TAG}/${BIN}_${VERSION}_checksums.txt" \
  -o "$TMP/checksums.txt"

sha() {
  awk -v f="$1" '$2 == f { print $1 }' "$TMP/checksums.txt"
}

DARWIN_AMD64="$(sha "${BIN}_${VERSION}_Darwin_x86_64.tar.gz")"
DARWIN_ARM64="$(sha "${BIN}_${VERSION}_Darwin_arm64.tar.gz")"
LINUX_AMD64="$(sha "${BIN}_${VERSION}_Linux_x86_64.tar.gz")"
LINUX_ARM64="$(sha "${BIN}_${VERSION}_Linux_arm64.tar.gz")"

for v in DARWIN_AMD64 DARWIN_ARM64 LINUX_AMD64 LINUX_ARM64; do
  if [[ -z "${!v}" ]]; then
    echo "error: missing checksum for ${v}" >&2
    exit 1
  fi
done

# macOS: a cask, so `app "SatSat.app"` can install the bundle into
# /Applications. The macOS archives contain nothing but SatSat.app.
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

  depends_on macos: ">= :big_sur"

  app "SatSat.app"
end
EOF

# Linux: a formula for the command-line binary. It is Linux-only because the
# macOS side is served by the cask above; naming it too keeps
# `brew install satsat` working on Linux.
cat > "$TMP/${BIN}-formula.rb" <<EOF
# typed: false
# frozen_string_literal: true

class ${CLASS} < Formula
  desc "${DESC}"
  homepage "https://github.com/${REPO}"
  license "${LICENSE}"

  depends_on :linux

  # url/sha256 stay at the top level so Homebrew can load the formula on every
  # OS (test-bot validates it on macOS too); depends_on :linux above is what
  # keeps it from installing there.
  on_arm do
    url "https://github.com/${REPO}/releases/download/${TAG}/${BIN}_${VERSION}_Linux_arm64.tar.gz"
    sha256 "${LINUX_ARM64}"
  end
  on_intel do
    url "https://github.com/${REPO}/releases/download/${TAG}/${BIN}_${VERSION}_Linux_x86_64.tar.gz"
    sha256 "${LINUX_AMD64}"
  end

  def install
    # app.ResourcePath resolves <exeDir>/Resources, so keep both together.
    libexec.install "${BIN}"
    libexec.install "Resources"
    bin.install_symlink libexec/"${BIN}"
  end

  test do
    system "#{bin}/${BIN}", "--help"
  end
end
EOF

TAP_DIR="$TMP/tap"
if [[ -n "${TAP_GITHUB_TOKEN:-}" ]]; then
  git clone "https://x-access-token:${TAP_GITHUB_TOKEN}@github.com/akhenakh/homebrew-tap.git" "$TAP_DIR"
else
  git clone "git@github.com:akhenakh/homebrew-tap.git" "$TAP_DIR"
fi
mkdir -p "$TAP_DIR/Casks" "$TAP_DIR/Formula"
cp "$TMP/${BIN}.rb" "$TAP_DIR/Casks/${BIN}.rb"
cp "$TMP/${BIN}-formula.rb" "$TAP_DIR/Formula/${BIN}.rb"

git -C "$TAP_DIR" config user.email "akh@inair.space"
git -C "$TAP_DIR" config user.name "Fabrice Aneche"
git -C "$TAP_DIR" add "Casks/${BIN}.rb" "Formula/${BIN}.rb"
git -C "$TAP_DIR" commit -m "Brew cask and formula update for ${BIN} ${TAG}" || true
git -C "$TAP_DIR" push origin main
