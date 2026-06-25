#!/usr/bin/env bash
set -euo pipefail

REPO_URL="${CLIKS_REPO_URL:-https://github.com/YashMahawa/Cliks.git}"
INSTALL_DIR="${CLIKS_INSTALL_DIR:-$HOME/.cliks}"
BIN_DIR="${CLIKS_BIN_DIR:-$HOME/.local/bin}"
DEFAULT_BACKEND="${CLIKS_API_URL:-https://139.59.29.207.sslip.io}"

case "$(uname -s)" in
  MINGW*|MSYS*|CYGWIN*)
    BIN_DIR="${CLIKS_BIN_DIR:-$HOME/bin}"
    ;;
esac

if ! command -v git >/dev/null 2>&1; then
  echo "Cliks needs git to install or update the CLI."
  echo "Install git, then rerun this script."
  exit 1
fi

if ! command -v go >/dev/null 2>&1; then
  echo "Cliks needs Go to build the CLI from source."
  echo "Install Go, then rerun this script."
  exit 1
fi

if [ -d "$INSTALL_DIR/.git" ]; then
  git -C "$INSTALL_DIR" pull --ff-only
else
  rm -rf "$INSTALL_DIR"
  git clone "$REPO_URL" "$INSTALL_DIR"
fi

cd "$INSTALL_DIR"
cd cli
go build -o dist/cliks .

mkdir -p "$BIN_DIR"
cat > "$BIN_DIR/cliks" <<EOF
#!/usr/bin/env sh
exec "$INSTALL_DIR/cli/dist/cliks" "\$@"
EOF
chmod +x "$BIN_DIR/cliks"

case ":$PATH:" in
  *":$BIN_DIR:"*) ;;
  *)
    echo ""
    echo "Add this directory to PATH if 'cliks' is not found in new terminals:"
    echo "  $BIN_DIR"
    ;;
esac

"$BIN_DIR/cliks" set api.url "$DEFAULT_BACKEND"

echo ""
echo "Cliks installed."
echo "Default backend: $DEFAULT_BACKEND"
echo "Command installed at: $BIN_DIR/cliks"
echo ""
"$BIN_DIR/cliks" doctor || true

if [ "$(uname -s)" = "Linux" ] && [ -d /dev/input ]; then
  if ! id -nG "${USER:-$(id -un)}" | tr ' ' '\n' | grep -qx input; then
    echo ""
    echo "Linux global capture needs permission to read input-device events."
    echo "Cliks still sends only event type and coarse timing, never key values."
    printf "Add your user to the input group now? [y/N] "
    read -r answer
    case "$answer" in
      y|Y|yes|YES)
        sudo usermod -aG input "${USER:-$(id -un)}"
        echo "Done. Log out and back in before using global capture."
        ;;
      *)
        echo "Skipped. You can run later: sudo usermod -aG input \\$USER"
        ;;
    esac
  fi
fi

case "$(uname -s)" in
  Darwin)
    echo ""
    echo "macOS global capture needs Accessibility permission for your terminal app."
    echo "Open System Settings > Privacy & Security > Accessibility, allow the terminal, then run:"
    echo "  cliks capture-test"
    ;;
  MINGW*|MSYS*|CYGWIN*)
    echo ""
    echo "Windows note: this installer is for Git Bash/MSYS-style shells."
    echo "If PowerShell cannot find cliks, add this to your user PATH:"
    echo "  $BIN_DIR"
    ;;
esac

echo ""
echo "Create a team on the Cliks website, then run:"
echo "  cliks join CLIK-XXXXXX"
echo "  cliks start"
