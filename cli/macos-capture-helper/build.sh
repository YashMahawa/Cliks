#!/usr/bin/env bash
set -euo pipefail
root="$(cd "$(dirname "$0")" && pwd)"
out="${1:-$root/dist/Cliks Capture.app}"
mkdir -p "$out/Contents/MacOS"
cp "$root/Info.plist" "$out/Contents/Info.plist"
arch="$(uname -m)"
if [ "$(sysctl -n hw.optional.arm64 2>/dev/null || true)" = "1" ]; then
  arch="arm64"
fi
swiftc -O -target "${CLIKS_MAC_CAPTURE_TARGET:-${arch}-apple-macosx11.0}" -framework ApplicationServices -framework Foundation "$root/main.swift" -o "$out/Contents/MacOS/cliks-capture"
# Community releases use an ad-hoc identity. Official releases can provide a
# Developer ID through CLIKS_MAC_SIGN_IDENTITY and are notarized separately.
identity="${CLIKS_MAC_SIGN_IDENTITY:--}"
codesign --force --deep --options runtime --sign "$identity" "$out"
