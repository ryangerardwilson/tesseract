#!/usr/bin/env bash
set -euo pipefail

APP=tesseract
REPO="ryangerardwilson/tesseract"
APP_HOME="$HOME/.${APP}"
INSTALL_DIR="$APP_HOME/bin"
APP_DIR="$APP_HOME/app"
PUBLIC_BIN_DIR="$HOME/.local/bin"
PUBLIC_LAUNCHER="$PUBLIC_BIN_DIR/${APP}"
FILENAME="tesseract-linux-x64.tar.gz"
TMUX_CONFIG_DIR="$HOME/.config/tmux"
TMUX_PRIMARY_CONF="$TMUX_CONFIG_DIR/tmux.conf"
TMUX_ROOT_CONF="$HOME/.tmux.conf"
TMUX_LEGACY_SNIPPET_DIR="$HOME/.tmux"
TMUX_LEGACY_SNIPPET_FILE="${TMUX_LEGACY_SNIPPET_DIR}/tm.conf"
SYSTEMD_USER_DIR="$HOME/.config/systemd/user"
TM_RESTORE_SERVICE="$SYSTEMD_USER_DIR/tesseract-restore.service"
TM_SNAPSHOT_SERVICE="$SYSTEMD_USER_DIR/tesseract-snapshot.service"
TM_SNAPSHOT_TIMER="$SYSTEMD_USER_DIR/tesseract-snapshot.timer"

usage() {
  cat <<EOF
${APP} Installer

Usage:
  install.sh                         Install the latest release
  install.sh help                    Show this help
  install.sh version                 Print the latest release version
  install.sh version <ver>           Install a specific release version
  install.sh upgrade                 Upgrade when a newer release exists
  install.sh from <path>             Install from a local binary or source checkout
  install.sh tmux key <key>          Bind the picker shortcut in tmux

EOF
}

die() {
  printf '%s\n' "$1" >&2
  exit 1
}

require_command() {
  command -v "$1" >/dev/null 2>&1 || die "'$1' is required but not installed."
}

normalize_version() {
  local value="$1"
  value="${value#v}"
  [[ -n "$value" ]] || die "empty version"
  printf '%s\n' "$value"
}

installed_command_path() {
  if [[ -x "${INSTALL_DIR}/${APP}" ]]; then
    printf '%s\n' "${INSTALL_DIR}/${APP}"
    return 0
  fi
  if [[ -x "${PUBLIC_LAUNCHER}" ]]; then
    printf '%s\n' "${PUBLIC_LAUNCHER}"
    return 0
  fi
  if command -v "${APP}" >/dev/null 2>&1; then
    command -v "${APP}"
    return 0
  fi
  return 1
}

read_installed_version() {
  local installed_cmd
  installed_cmd="$(installed_command_path)" || return 0
  "$installed_cmd" version 2>/dev/null | head -n 1 | sed 's/^v//' || true
}

get_latest_version() {
  require_command curl
  local release_url tag
  release_url="$(curl -fsSL -o /dev/null -w "%{url_effective}" "https://github.com/${REPO}/releases/latest")" \
    || die "Unable to determine latest release"
  tag="${release_url##*/}"
  tag="${tag#v}"
  [[ -n "$tag" && "$tag" != "latest" ]] || die "Unable to determine latest release"
  printf '%s\n' "$tag"
}

migrate_legacy_state() {
  local legacy="$HOME/.tm"
  if [[ -e "$APP_HOME" || ! -d "$legacy" ]]; then
    return 0
  fi
  mv "$legacy" "$APP_HOME"
}

write_public_launcher() {
  mkdir -p "$PUBLIC_BIN_DIR"
  if [[ -e "$PUBLIC_LAUNCHER" && ! -L "$PUBLIC_LAUNCHER" && ! -f "$PUBLIC_LAUNCHER" ]]; then
    die "Refusing to overwrite non-file launcher: $PUBLIC_LAUNCHER"
  fi
  if [[ -L "$PUBLIC_LAUNCHER" ]]; then
    local resolved
    resolved="$(readlink -f "$PUBLIC_LAUNCHER" 2>/dev/null || true)"
    [[ "$resolved" == "${INSTALL_DIR}/${APP}" ]] || die "Refusing to overwrite existing symlink launcher: $PUBLIC_LAUNCHER"
  elif [[ -f "$PUBLIC_LAUNCHER" ]] && ! grep -Eq '^# Managed by (tm|tesseract)( installer)? local-bin launcher$' "$PUBLIC_LAUNCHER" 2>/dev/null; then
    die "Refusing to overwrite existing launcher: $PUBLIC_LAUNCHER"
  fi
  cat > "$PUBLIC_LAUNCHER" <<EOF
#!/usr/bin/env bash
# Managed by tesseract installer local-bin launcher
set -euo pipefail
exec -a "\$0" "${INSTALL_DIR}/${APP}" "\$@"
EOF
  chmod 755 "$PUBLIC_LAUNCHER"
  local legacy="$PUBLIC_BIN_DIR/tm"
  if [[ -L "$legacy" || ! -e "$legacy" ]]; then
    ln -sfn tesseract "$legacy"
  elif [[ -f "$legacy" ]] && grep -Eq '^# Managed by (tm|tesseract)( installer)? local-bin launcher$' "$legacy"; then
    ln -sfn tesseract "$legacy"
  fi
  local km_launcher="$PUBLIC_BIN_DIR/km"
  if [[ -L "$km_launcher" || ! -e "$km_launcher" ]]; then
    ln -sfn tesseract "$km_launcher"
  elif [[ -f "$km_launcher" ]] && grep -Eq '^# Managed by km( installer)? local-bin launcher$' "$km_launcher"; then
    ln -sfn tesseract "$km_launcher"
  fi
}

install_assets() {
  "${INSTALL_DIR}/${APP}" __install-assets "$APP_DIR"
}

write_tmux_primary_conf() {
  mkdir -p "$TMUX_CONFIG_DIR"
  "${INSTALL_DIR}/${APP}" __render-tmux-config "$TMUX_PRIMARY_CONF" "$tmux_index_key" "$PUBLIC_LAUNCHER"
}

remove_legacy_tmux_files() {
  rm -f "$TMUX_LEGACY_SNIPPET_FILE"
  rmdir "$TMUX_LEGACY_SNIPPET_DIR" 2>/dev/null || true
  rm -f "$TMUX_CONFIG_DIR/tm.conf"
  if [[ -f "$TMUX_ROOT_CONF" ]] && grep -Eq '^# Managed by tesseract installer\.|^# Managed tesseract index jump$' "$TMUX_ROOT_CONF"; then
    rm -f "$TMUX_ROOT_CONF"
  fi
}

write_systemd_user_units() {
  mkdir -p "$SYSTEMD_USER_DIR"
  cat > "$TM_RESTORE_SERVICE" <<EOF
[Unit]
Description=Restore tmux sessions managed by tesseract
# graphical-session-pre is reached only after uwsm has imported PATH.
# Restoring earlier starts tmux without mise, so grok and codex are not found.
After=graphical-session-pre.target

[Service]
Type=oneshot
ExecStart=%h/.local/bin/tesseract restore sessions

[Install]
WantedBy=graphical-session.target
EOF

  cat > "$TM_SNAPSHOT_SERVICE" <<EOF
[Unit]
Description=Save tmux sessions managed by tesseract
After=tesseract-restore.service

[Service]
Type=oneshot
ExecStart=%h/.local/bin/tesseract snapshot sessions
EOF

  cat > "$TM_SNAPSHOT_TIMER" <<EOF
[Unit]
Description=Periodically save tmux sessions managed by tesseract

[Timer]
OnActiveSec=15min
OnUnitActiveSec=15min
AccuracySec=5s
Unit=tesseract-snapshot.service

[Install]
WantedBy=timers.target
EOF
}

activate_systemd_user_units() {
  [[ "${TM_INSTALL_ACTIVATE_SYSTEMD:-1}" == "1" ]] || return 0
  command -v systemctl >/dev/null 2>&1 || return 0
  systemctl --user daemon-reload >/dev/null 2>&1 || return 0
  systemctl --user disable --now tm-restore.service tm-snapshot.timer >/dev/null 2>&1 || true
  rm -f "$SYSTEMD_USER_DIR/tm-restore.service" "$SYSTEMD_USER_DIR/tm-snapshot.service" "$SYSTEMD_USER_DIR/tm-snapshot.timer"
  systemctl --user enable tesseract-restore.service tesseract-snapshot.timer >/dev/null 2>&1 || true
  systemctl --user reset-failed tesseract-restore.service tesseract-snapshot.service >/dev/null 2>&1 || true
  systemctl --user start tesseract-restore.service >/dev/null 2>&1 || true
  systemctl --user start tesseract-snapshot.timer >/dev/null 2>&1 || true
}

finalize_install() {
  install_assets
  write_public_launcher
  write_tmux_primary_conf
  remove_legacy_tmux_files
  write_systemd_user_units
  activate_systemd_user_units
}

print_manual_shell_steps() {
  if [[ ":$PATH:" != *":$PUBLIC_BIN_DIR:"* ]]; then
    printf '%s\n' "Manually add to ~/.bashrc if needed: export PATH=$PUBLIC_BIN_DIR:\$PATH"
    printf '%s\n' "Reload your shell: source ~/.bashrc"
  fi
}

install_from_binary() {
  local binary_path="$1"
  migrate_legacy_state
  mkdir -p "$INSTALL_DIR"
  cp "$binary_path" "${INSTALL_DIR}/${APP}"
  chmod 755 "${INSTALL_DIR}/${APP}"
  finalize_install
}

source_build_version() {
  local source_path="$1"
  local described=""
  if command -v git >/dev/null 2>&1 && git -C "$source_path" rev-parse --is-inside-work-tree >/dev/null 2>&1; then
    described="$(git -C "$source_path" describe --tags --dirty --always --match 'v[0-9]*' 2>/dev/null || true)"
    described="${described#v}"
  fi
  if [[ -z "$described" ]]; then
    described="0.0.0"
  fi
  printf '%s\n' "$described"
}

install_from_source() {
  local source_path="$1"
  migrate_legacy_state
  local build_version
  require_command go
  build_version="$(source_build_version "$source_path")"
  mkdir -p "$INSTALL_DIR"
  (cd "$source_path" && GOWORK=off go build \
    -ldflags "-X github.com/ryangerardwilson/tesseract/internal/version.Version=${build_version}" \
    -o "${INSTALL_DIR}/${APP}" ./cmd/tesseract)
  chmod 755 "${INSTALL_DIR}/${APP}"
  finalize_install
}

install_release() {
  local requested
  requested="$(normalize_version "$1")"
  require_command curl
  require_command tar
  local os_name arch asset_url tmp_dir archive binary_path
  os_name="$(uname -s)"
  arch="$(uname -m)"
  [[ "$os_name" == "Linux" ]] || die "Only Linux release installs are currently packaged."
  [[ "$arch" == "x86_64" || "$arch" == "amd64" ]] || die "Only Linux x64 release installs are currently packaged."
  asset_url="https://github.com/${REPO}/releases/download/v${requested}/${FILENAME}"
  tmp_dir="$(mktemp -d "${TMPDIR:-/tmp}/${APP}_install_XXXXXX")"
  archive="$tmp_dir/$FILENAME"
  curl -fsSL "$asset_url" -o "$archive" || die "Unable to download ${asset_url}"
  tar -xzf "$archive" -C "$tmp_dir"
  binary_path="$(find "$tmp_dir" -type f -name "$APP" -perm -u+x | head -n 1)"
  [[ -n "$binary_path" ]] || die "Release archive did not contain executable ${APP}"
  install_from_binary "$binary_path"
  rm -rf "$tmp_dir"
  print_manual_shell_steps
  "${PUBLIC_LAUNCHER}" version
}

upgrade_release() {
  local latest installed
  latest="$(get_latest_version)"
  installed="$(read_installed_version)"
  if [[ -n "$installed" && "$installed" == "$latest" && -x "${INSTALL_DIR}/${APP}" ]]; then
    finalize_install
    print_manual_shell_steps
    printf '%s %s already installed\n' "$APP" "$latest"
    return 0
  fi
  install_release "$latest"
}

tmux_index_key=${TMUX_INDEX_KEY:-M-i}
show_latest=false
upgrade=false
from_path=""
requested_version=""
install_action=false

if [[ $# -eq 0 ]]; then
  install_action=true
fi

while [[ $# -gt 0 ]]; do
  case "$1" in
    help)
      usage
      exit 0
      ;;
    version)
      if [[ -n "${2:-}" && "${2:0:1}" != "-" ]]; then
        requested_version="$2"
        install_action=true
        shift 2
      else
        show_latest=true
        shift
      fi
      ;;
    upgrade)
      upgrade=true
      install_action=true
      shift
      ;;
    from)
      [[ -n "${2:-}" ]] || die "from requires a path"
      from_path="$2"
      install_action=true
      shift 2
      ;;
    tmux)
      [[ "${2:-}" == "key" && -n "${3:-}" ]] || die "tmux key requires a tmux key name"
      tmux_index_key="$3"
      shift 3
      ;;
    *)
      die "unknown installer command: $1"
      ;;
  esac
done

if $show_latest; then
  [[ "$upgrade" == false && -z "$from_path" && -z "$requested_version" ]] || die "version cannot be combined with install commands"
  get_latest_version
  exit 0
fi

if [[ -n "$from_path" ]]; then
  [[ "$upgrade" == false && -z "$requested_version" ]] || die "from cannot be combined with upgrade or version <ver>"
  [[ -e "$from_path" ]] || die "from path does not exist: $from_path"
  if [[ -d "$from_path" ]]; then
    install_from_source "$from_path"
  elif [[ -f "$from_path" ]]; then
    install_from_binary "$from_path"
  else
    die "unsupported from path: $from_path"
  fi
  print_manual_shell_steps
  "${PUBLIC_LAUNCHER}" version
  exit 0
fi

if $upgrade; then
  [[ -z "$requested_version" ]] || die "upgrade cannot be combined with version <ver>"
  upgrade_release
  exit 0
fi

if [[ -n "$requested_version" ]]; then
  install_release "$requested_version"
  exit 0
fi

if $install_action; then
  install_release "$(get_latest_version)"
  exit 0
fi

if [[ -x "${INSTALL_DIR}/${APP}" ]]; then
  finalize_install
  print_manual_shell_steps
  printf '%s install refreshed\n' "$APP"
  exit 0
fi

install_release "$(get_latest_version)"
