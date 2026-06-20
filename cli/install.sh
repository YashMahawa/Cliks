#!/usr/bin/env bash
set -euo pipefail

REPO_URL="${CLIKS_REPO_URL:-https://github.com/YashMahawa/Cliks.git}"
INSTALL_DIR="${CLIKS_INSTALL_DIR:-$HOME/.cliks}"
DEFAULT_BACKEND="${CLIKS_API_URL:-https://139.59.29.207.sslip.io}"

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

echo ""
echo "Cliks installed."
echo "Default backend: $DEFAULT_BACKEND"
echo ""
typ doctor || true

if [ "$(uname -s)" = "Linux" ] && [ -d /dev/input ]; then
  if ! id -nG "$USER" | tr ' ' '\n' | grep -qx input; then
    echo ""
    echo "Linux global capture needs permission to read input-device events."
    echo "Cliks still sends only event type and timing, never key values."
    printf "Add your user to the input group now? [y/N] "
    read -r answer
    case "$answer" in
      y|Y|yes|YES)
        sudo usermod -aG input "$USER"
        echo "Done. Log out and back in before using global capture."
        ;;
      *)
        echo "Skipped. You can run later: sudo usermod -aG input \\$USER"
        ;;
    esac
  fi
fi

echo ""
echo "Create a team on the Cliks website, then run:"
echo "  typ join CLIK-XXXX"
echo "  typ start"
