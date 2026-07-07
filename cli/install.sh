#!/usr/bin/env bash
set -euo pipefail

REPO_URL="${CLIKS_REPO_URL:-https://github.com/YashMahawa/Cliks.git}"
INSTALL_DIR="${CLIKS_INSTALL_DIR:-$HOME/.cliks}"
BIN_DIR="${CLIKS_BIN_DIR:-$HOME/.local/bin}"
DEFAULT_BACKEND="${CLIKS_API_URL:-https://139.59.29.207.sslip.io}"

is_termux() {
  case "${PREFIX:-}:${TERMUX_VERSION:-}:$(uname -o 2>/dev/null || true):$HOME" in
    *com.termux*|*Android*) return 0 ;;
    *) return 1 ;;
  esac
}

if is_termux; then
  BIN_DIR="${CLIKS_BIN_DIR:-${PREFIX:-$HOME/../usr}/bin}"
fi

case "$(uname -s)" in
  MINGW*|MSYS*|CYGWIN*)
    BIN_DIR="${CLIKS_BIN_DIR:-$HOME/bin}"
    ;;
esac

if ! command -v git >/dev/null 2>&1; then
  if is_termux; then
    echo "Git is not installed. Installing it with Termux package manager..."
    if command -v pkg >/dev/null 2>&1; then
      pkg install -y git
    else
      apt-get update
      apt-get install -y git
    fi
  else
    echo "Cliks needs git to install or update the CLI."
    echo "Install git, then rerun this script."
    exit 1
  fi
fi

install_system_deps() {
  case "$(uname -s)" in
    Darwin)
      if ! command -v brew >/dev/null 2>&1; then
        echo "Homebrew was not found. Installing Homebrew..."
        NONINTERACTIVE=1 /bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"
        if [ -x /opt/homebrew/bin/brew ]; then
          eval "$(/opt/homebrew/bin/brew shellenv)"
        elif [ -x /usr/local/bin/brew ]; then
          eval "$(/usr/local/bin/brew shellenv)"
        fi
      fi
      if ! command -v mpv >/dev/null 2>&1; then
        echo "Installing mpv for macOS spatial audio..."
        brew install mpv
      fi
      ;;
    Linux)
      if is_termux; then
        if command -v pkg >/dev/null 2>&1; then
          pkg install -y mpv termux-api
        else
          apt-get update
          apt-get install -y mpv termux-api
        fi
      elif command -v pacman >/dev/null 2>&1; then
        sudo pacman -S --needed --noconfirm mpv xclip wl-clipboard
      elif command -v apt-get >/dev/null 2>&1; then
        sudo apt-get update
        sudo apt-get install -y mpv xclip wl-clipboard pulseaudio-utils
      elif command -v dnf >/dev/null 2>&1; then
        sudo dnf install -y mpv xclip wl-clipboard pulseaudio-utils
      elif command -v zypper >/dev/null 2>&1; then
        sudo zypper install -y mpv xclip wl-clipboard pulseaudio-utils
      elif command -v apk >/dev/null 2>&1; then
        sudo apk add mpv xclip wl-clipboard
      fi
      ;;
    MINGW*|MSYS*|CYGWIN*)
      if ! command -v winget.exe >/dev/null 2>&1 && ! command -v choco.exe >/dev/null 2>&1 && ! command -v scoop >/dev/null 2>&1; then
        echo "No package manager found. Installing Scoop..."
        powershell.exe -NoProfile -ExecutionPolicy Bypass -Command "Set-ExecutionPolicy -ExecutionPolicy RemoteSigned -Scope CurrentUser -Force; Invoke-RestMethod -Uri https://get.scoop.sh | Invoke-Expression"
        export PATH="$HOME/scoop/shims:$PATH"
      fi

      if command -v winget.exe >/dev/null 2>&1; then
        if ! command -v mpv >/dev/null 2>&1; then
          echo "Installing mpv for Windows spatial audio..."
          winget.exe install --id mpv.mpv -e --accept-package-agreements --accept-source-agreements
        fi
      elif command -v choco.exe >/dev/null 2>&1; then
        if ! command -v mpv >/dev/null 2>&1; then
          echo "Installing mpv for Windows spatial audio..."
          choco.exe install mpv -y
        fi
      elif command -v scoop >/dev/null 2>&1; then
        if ! command -v mpv >/dev/null 2>&1; then
          echo "Installing mpv for Windows spatial audio..."
          scoop install mpv
        fi
      fi
      ;;
  esac
}

install_go() {
  echo "Go is not installed. Cliks will try to install it now."
  case "$(uname -s)" in
    Darwin)
      if ! command -v brew >/dev/null 2>&1; then
        echo "Homebrew was not found. Installing Homebrew..."
        NONINTERACTIVE=1 /bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"
        if [ -x /opt/homebrew/bin/brew ]; then
          eval "$(/opt/homebrew/bin/brew shellenv)"
        elif [ -x /usr/local/bin/brew ]; then
          eval "$(/usr/local/bin/brew shellenv)"
        fi
      fi
      if command -v brew >/dev/null 2>&1; then
        brew install go
      else
        echo "Failed to install Homebrew. Install Go from https://go.dev/dl/, then rerun this script."
        exit 1
      fi
      ;;
    Linux)
      if is_termux; then
        if command -v pkg >/dev/null 2>&1; then
          pkg install -y golang
        else
          apt-get update
          apt-get install -y golang
        fi
      elif command -v pacman >/dev/null 2>&1; then
        sudo pacman -S --needed --noconfirm go
      elif command -v apt-get >/dev/null 2>&1; then
        sudo apt-get update
        sudo apt-get install -y golang-go
      elif command -v dnf >/dev/null 2>&1; then
        sudo dnf install -y golang
      elif command -v zypper >/dev/null 2>&1; then
        sudo zypper install -y go
      elif command -v apk >/dev/null 2>&1; then
        sudo apk add go
      else
        echo "Could not find a supported package manager to install Go automatically."
        echo "Install Go from https://go.dev/dl/, then rerun this script."
        exit 1
      fi
      ;;
    MINGW*|MSYS*|CYGWIN*)
      if command -v winget.exe >/dev/null 2>&1; then
        winget.exe install --id GoLang.Go -e --accept-package-agreements --accept-source-agreements
      elif command -v choco.exe >/dev/null 2>&1; then
        choco.exe install golang -y
      elif command -v scoop >/dev/null 2>&1; then
        scoop install go
      else
        echo "Could not find a package manager to install Go automatically."
        echo "Install Go from https://go.dev/dl/, reopen this shell, then rerun this script."
        exit 1
      fi
      ;;
    *)
      echo "Install Go from https://go.dev/dl/, then rerun this script."
      exit 1
      ;;
  esac
}

if ! command -v go >/dev/null 2>&1; then
  install_go
fi

if ! command -v go >/dev/null 2>&1; then
  echo "Go still was not found on PATH after installation."
  echo "Open a new terminal or add Go to PATH, then rerun this script."
  exit 1
fi

install_system_deps

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
    if is_termux; then
      echo "Termux usually includes this directory by default. If this shell was opened before install, run:"
      echo "  hash -r"
    fi
    ;;
esac

"$BIN_DIR/cliks" set api.url "$DEFAULT_BACKEND"

echo ""
echo "Cliks installed."
echo "Default backend: $DEFAULT_BACKEND"
echo "Command installed at: $BIN_DIR/cliks"
echo ""
"$BIN_DIR/cliks" doctor || true

if [ "$(uname -s)" = "Linux" ] && ! is_termux && [ -d /dev/input ]; then
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
