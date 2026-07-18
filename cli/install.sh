#!/usr/bin/env bash
# Cliks one-line installer — designed for non-technical users.
# Installs the CLI and prepares native audio/capture access.
set -euo pipefail

REPO_URL="${CLIKS_REPO_URL:-https://github.com/YashMahawa/Cliks.git}"
INSTALL_DIR="${CLIKS_INSTALL_DIR:-$HOME/.cliks}"
BIN_DIR="${CLIKS_BIN_DIR:-$HOME/.local/bin}"
DEFAULT_BACKEND="${CLIKS_API_URL:-https://139.59.29.207.sslip.io}"
REQUIRED_VERSION="${CLIKS_REQUIRED_VERSION:-0.6.6}"
CAPTURE_APP_DIR="${CLIKS_CAPTURE_APP_DIR:-$HOME/Applications/Cliks Capture.app}"
# When piped from curl, default to non-interactive auto setup.
AUTO_YES="${CLIKS_AUTO_YES:-}"
if [ -z "$AUTO_YES" ]; then
  if [ ! -t 0 ]; then
    AUTO_YES=1
  else
    AUTO_YES=0
  fi
fi

say() { printf '%s\n' "$*"; }
ok() { printf '  ✓ %s\n' "$*"; }
tip() { printf '  · %s\n' "$*"; }

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

say "Installing Cliks..."
say ""

# Prefer a small native release. Source compilation remains a fallback for
# unreleased branches and unusual architectures.
PREBUILT=0
version_at_least() {
  awk -v got="$1" -v need="$2" 'BEGIN {
    split(got, g, "."); split(need, n, ".");
    for (i = 1; i <= 3; i++) {
      g[i] += 0; n[i] += 0;
      if (g[i] > n[i]) exit 0;
      if (g[i] < n[i]) exit 1;
    }
    exit 0;
  }'
}
install_prebuilt() {
  local os arch asset url tmp downloaded_version
  case "$(uname -s)" in
    Darwin) os="macos" ;;
    Linux) os="linux" ;;
    *) return 1 ;;
  esac
  case "$(uname -m)" in
    x86_64|amd64) arch="amd64" ;;
    arm64|aarch64) arch="arm64" ;;
    *) return 1 ;;
  esac
  asset="cliks-${os}-${arch}.tar.gz"
  url="https://github.com/YashMahawa/Cliks/releases/latest/download/${asset}"
  tmp="$(mktemp -d 2>/dev/null || mktemp -d -t cliks)"
  if curl -fL --retry 2 --connect-timeout 10 "$url" -o "$tmp/$asset" 2>/dev/null && \
     tar -xzf "$tmp/$asset" -C "$tmp" && [ -x "$tmp/cliks" ]; then
    downloaded_version="$($tmp/cliks version 2>/dev/null || true)"
    if ! version_at_least "$downloaded_version" "$REQUIRED_VERSION"; then
      tip "Latest release is Cliks ${downloaded_version:-unknown}; ${REQUIRED_VERSION}+ is required for embedded sounds — using source fallback"
      rm -rf "$tmp"
      return 1
    fi
    mkdir -p "$BIN_DIR"
    install -m 755 "$tmp/cliks" "$BIN_DIR/cliks"
    if [ "$os" = "macos" ] && [ -d "$tmp/Cliks Capture.app" ]; then
      mkdir -p "$(dirname "$CAPTURE_APP_DIR")"
      rm -rf "$CAPTURE_APP_DIR"
      cp -R "$tmp/Cliks Capture.app" "$CAPTURE_APP_DIR"
      ok "Installed isolated Cliks Capture app"
    fi
    if [ "$os" = "linux" ] && [ -x "$tmp/cliks-capture-helper" ] && [ -f "$tmp/cliks-capture.service" ] && command -v systemctl >/dev/null 2>&1; then
      if command -v sudo >/dev/null 2>&1; then
        sudo mkdir -p /usr/local/libexec
        sudo install -m 755 "$tmp/cliks-capture-helper" /usr/local/libexec/cliks-capture-helper
        sudo install -m 644 "$tmp/cliks-capture.service" /etc/systemd/system/cliks-capture.service
        sudo mkdir -p /etc/systemd/system/cliks-capture.service.d
        printf '[Service]\nEnvironment=CLIKS_CAPTURE_UID=%s\nEnvironment="CLIKS_CAPTURE_CLIENT_EXE=%s"\n' "$(id -u)" "$(readlink -f "$BIN_DIR/cliks")" | sudo tee /etc/systemd/system/cliks-capture.service.d/user.conf >/dev/null
        sudo systemctl daemon-reload
        sudo systemctl enable --now cliks-capture.service
        ok "Installed privacy-isolated input helper"
      fi
    fi
    PREBUILT=1
    ok "Downloaded native Cliks release"
  fi
  rm -rf "$tmp"
  [ "$PREBUILT" = "1" ]
}

if ! is_termux; then
  install_prebuilt || tip "No matching release found — using the source fallback"
fi

# --- git ---
if [ "$PREBUILT" = "0" ] && ! command -v git >/dev/null 2>&1; then
  if is_termux; then
    say "Installing git..."
    if command -v pkg >/dev/null 2>&1; then
      pkg install -y git
    else
      apt-get update
      apt-get install -y git
    fi
  else
    say "Cliks needs git. Install git, then rerun this command."
    exit 1
  fi
fi

# --- go ---
install_go() {
  say "Installing Go (needed to build Cliks)..."
  case "$(uname -s)" in
    Darwin)
      if ! command -v brew >/dev/null 2>&1; then
        say "Installing Homebrew..."
        NONINTERACTIVE=1 /bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"
        if [ -x /opt/homebrew/bin/brew ]; then
          eval "$(/opt/homebrew/bin/brew shellenv)"
        elif [ -x /usr/local/bin/brew ]; then
          eval "$(/usr/local/bin/brew shellenv)"
        fi
      fi
      brew install go
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
        say "Install Go from https://go.dev/dl/ then rerun this script."
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
        say "Install Go from https://go.dev/dl/, reopen this shell, then rerun."
        exit 1
      fi
      # Common Windows Go install path
      export PATH="/c/Program Files/Go/bin:$HOME/go/bin:$PATH"
      ;;
    *)
      say "Install Go from https://go.dev/dl/ then rerun this script."
      exit 1
      ;;
  esac
}

if [ "$PREBUILT" = "0" ] && ! command -v go >/dev/null 2>&1; then
  install_go
fi

# Refresh PATH for common Go locations after fresh install.
export PATH="$HOME/go/bin:/usr/local/go/bin:/c/Program Files/Go/bin:$PATH"

if [ "$PREBUILT" = "0" ] && ! command -v go >/dev/null 2>&1; then
  say "Go is installed but not on PATH yet. Open a new terminal and rerun this command."
  exit 1
fi
if [ "$PREBUILT" = "0" ]; then
  ok "Go ready"
fi

# --- platform helpers (Linux audio/notifications; desktop builds include audio) ---
install_system_deps() {
  case "$(uname -s)" in
    Darwin)
      ok "Spatial audio (built into Cliks)"
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
        sudo pacman -S --needed --noconfirm mpv xclip wl-clipboard libnotify
      elif command -v apt-get >/dev/null 2>&1; then
        sudo apt-get update
        sudo apt-get install -y mpv xclip wl-clipboard pulseaudio-utils libnotify-bin || \
          sudo apt-get install -y mpv
      elif command -v dnf >/dev/null 2>&1; then
        sudo dnf install -y mpv xclip wl-clipboard pulseaudio-utils libnotify || sudo dnf install -y mpv
      elif command -v zypper >/dev/null 2>&1; then
        sudo zypper install -y mpv xclip wl-clipboard pulseaudio-utils libnotify-tools || sudo zypper install -y mpv
      elif command -v apk >/dev/null 2>&1; then
        sudo apk add mpv xclip wl-clipboard libnotify
      fi
      if command -v mpv >/dev/null 2>&1; then
        ok "Spatial audio (mpv)"
      else
        tip "mpv not installed — Cliks will use basic system sound if available"
      fi
      ;;
    MINGW*|MSYS*|CYGWIN*)
      ok "Spatial audio (built into Cliks)"
      ;;
  esac
}

install_system_deps

# --- source fallback ---
if [ "$PREBUILT" = "0" ]; then
  if [ -d "$INSTALL_DIR/.git" ]; then
    git -C "$INSTALL_DIR" pull --ff-only
  else
    rm -rf "$INSTALL_DIR"
    git clone "$REPO_URL" "$INSTALL_DIR"
  fi
  ok "Source ready"
  cd "$INSTALL_DIR/cli"
  go build -o dist/cliks .
  ok "Built cliks"
  mkdir -p "$BIN_DIR"
  cat > "$BIN_DIR/cliks" <<EOF
#!/usr/bin/env sh
exec "$INSTALL_DIR/cli/dist/cliks" "\$@"
EOF
  chmod +x "$BIN_DIR/cliks"
  if [ "$(uname -s)" = "Darwin" ] && command -v swiftc >/dev/null 2>&1; then
    chmod +x macos-capture-helper/build.sh
    macos-capture-helper/build.sh "$CAPTURE_APP_DIR"
  fi
  if [ "$(uname -s)" = "Linux" ] && ! is_termux && command -v systemctl >/dev/null 2>&1; then
    go build -o dist/cliks-capture-helper ./linux-capture-helper
    sudo mkdir -p /usr/local/libexec
    sudo install -m 755 dist/cliks-capture-helper /usr/local/libexec/cliks-capture-helper
    sudo install -m 644 linux-capture-helper/cliks-capture.service /etc/systemd/system/cliks-capture.service
    sudo mkdir -p /etc/systemd/system/cliks-capture.service.d
    printf '[Service]\nEnvironment=CLIKS_CAPTURE_UID=%s\nEnvironment="CLIKS_CAPTURE_CLIENT_EXE=%s"\n' "$(id -u)" "$(readlink -f "$INSTALL_DIR/cli/dist/cliks")" | sudo tee /etc/systemd/system/cliks-capture.service.d/user.conf >/dev/null
    sudo systemctl daemon-reload
    sudo systemctl enable --now cliks-capture.service
  fi
fi

# --- PATH for new terminals ---
ensure_path_export() {
  local export_line='export PATH="$HOME/.local/bin:$PATH" # cliks'
  case "$(uname -s)" in
    MINGW*|MSYS*|CYGWIN*)
      export_line="export PATH=\"$BIN_DIR:\$PATH\" # cliks"
      ;;
  esac
  if is_termux; then
    return 0
  fi
  local rc
  for rc in "$HOME/.bashrc" "$HOME/.zshrc" "$HOME/.profile" "$HOME/.bash_profile"; do
    if [ -f "$rc" ] || [ "$rc" = "$HOME/.profile" ]; then
      if [ ! -f "$rc" ]; then
        touch "$rc" 2>/dev/null || continue
      fi
      if ! grep -q '# cliks' "$rc" 2>/dev/null; then
        printf '\n%s\n' "$export_line" >> "$rc" 2>/dev/null || true
      fi
    fi
  done
}

export PATH="$BIN_DIR:$PATH"
ensure_path_export
ok "Command: $BIN_DIR/cliks"

installed_version="$($BIN_DIR/cliks version 2>/dev/null || true)"
if ! version_at_least "$installed_version" "$REQUIRED_VERSION"; then
  say "Install stopped: Cliks ${REQUIRED_VERSION}+ is required, but ${installed_version:-an unknown version} was installed."
  say "The updater will not silently leave the broken non-embedded sound build in place."
  exit 1
fi
ok "Version $installed_version (bundled sounds included)"

"$BIN_DIR/cliks" set api.url "$DEFAULT_BACKEND" >/dev/null 2>&1 || true

# Cliks deliberately does not add desktop users to the Linux input group or
# grant per-user ACLs anymore. Those permissions would let unrelated programs
# under that account inspect raw device events.

# --- final guided setup ---
say ""
say "Running easy setup..."
"$BIN_DIR/cliks" setup || true

say ""
say "Cliks is ready."
say "Default backend: $DEFAULT_BACKEND"
say ""
say "Next steps:"
say "  1. Create a team on the Cliks website"
say "  2. cliks join CLIK-XXXXXX"
say "  3. Keep typing — teammates only hear soft clicks, never your keys"
say ""
if ! command -v cliks >/dev/null 2>&1; then
  tip "If 'cliks' is not found, open a new terminal or run: export PATH=\"$BIN_DIR:\$PATH\""
fi
