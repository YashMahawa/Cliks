#!/usr/bin/env bash
set -euo pipefail

REPO_URL="${CLIKS_REPO_URL:-https://github.com/YashMahawa/Cliks.git}"
INSTALL_DIR="${CLIKS_INSTALL_DIR:-$HOME/.cliks}"

if ! command -v node >/dev/null 2>&1; then
  echo "Cliks needs Node.js 20 or newer. Install Node first, then rerun this script."
  exit 1
fi

if [ -d "$INSTALL_DIR/.git" ]; then
  git -C "$INSTALL_DIR" pull --ff-only
else
  rm -rf "$INSTALL_DIR"
  git clone "$REPO_URL" "$INSTALL_DIR"
fi

cd "$INSTALL_DIR"
npm install
npm --workspace @cliks/cli run build
npm link --workspace @cliks/cli

echo "Cliks installed. Try: typ join CLIK-LOCAL && typ start"
