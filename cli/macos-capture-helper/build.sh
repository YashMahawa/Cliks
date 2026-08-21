#!/usr/bin/env bash
set -euo pipefail
root="$(cd "$(dirname "$0")" && pwd)"
out="${1:-$root/dist/Cliks Capture.app}"

if ! command -v swiftc >/dev/null 2>&1; then
  echo "WARNING: Swift compiler (swiftc) is not installed. Unable to compile Cliks Capture.app." >&2
  echo "Action required: Run 'xcode-select --install' to install Xcode Command Line Tools." >&2
  exit 1
fi

mkdir -p "$out/Contents/MacOS"
cp "$root/Info.plist" "$out/Contents/Info.plist"
swiftc -O -framework ApplicationServices -framework Foundation "$root/main.swift" -o "$out/Contents/MacOS/cliks-capture"

# Community releases use an ad-hoc identity. Official releases provide a
# Developer ID through CLIKS_MAC_SIGN_IDENTITY or APPLE_DEVELOPER_ID.
identity="${CLIKS_MAC_SIGN_IDENTITY:-${APPLE_DEVELOPER_ID:--}}"
echo "Signing $out with identity: $identity"
codesign --force --deep --options runtime --timestamp --sign "$identity" "$out"

# Submit for Apple Notarization if credentials are provided in CI / release environment
if [ -n "${APPLE_ID:-}" ] && [ -n "${APPLE_PASSWORD:-}" ] && [ -n "${APPLE_TEAM_ID:-}" ]; then
  echo "Submitting $out for Apple Notarization using Apple ID..."
  zip_out="$(mktemp -t cliks-capture-XXXXXX.zip)"
  ditto -c -k --keepParent "$out" "$zip_out"
  xcrun notarytool submit "$zip_out" --apple-id "$APPLE_ID" --password "$APPLE_PASSWORD" --team-id "$APPLE_TEAM_ID" --wait
  rm -f "$zip_out"
  echo "Stapling notarization ticket to $out..."
  xcrun stapler staple "$out" || echo "Stapling completed or skipped"
elif [ -n "${APPLE_KEY_ID:-}" ] && [ -n "${APPLE_ISSUER_ID:-}" ] && [ -n "${APPLE_KEY_FILE:-}" ]; then
  echo "Submitting $out for Apple Notarization using API Key..."
  zip_out="$(mktemp -t cliks-capture-XXXXXX.zip)"
  ditto -c -k --keepParent "$out" "$zip_out"
  xcrun notarytool submit "$zip_out" --key "$APPLE_KEY_FILE" --key-id "$APPLE_KEY_ID" --issuer "$APPLE_ISSUER_ID" --wait
  rm -f "$zip_out"
  echo "Stapling notarization ticket to $out..."
  xcrun stapler staple "$out" || echo "Stapling completed or skipped"
elif [ -n "${APPLE_NOTARY_PROFILE:-}" ]; then
  echo "Submitting $out for Apple Notarization using Keychain profile $APPLE_NOTARY_PROFILE..."
  zip_out="$(mktemp -t cliks-capture-XXXXXX.zip)"
  ditto -c -k --keepParent "$out" "$zip_out"
  xcrun notarytool submit "$zip_out" --keychain-profile "$APPLE_NOTARY_PROFILE" --wait
  rm -f "$zip_out"
  echo "Stapling notarization ticket to $out..."
  xcrun stapler staple "$out" || echo "Stapling completed or skipped"
else
  echo "Apple Notarization skipped (no notarization credentials provided)."
fi

