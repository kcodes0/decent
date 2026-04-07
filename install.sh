#!/bin/sh

set -eu

VERSION="${DECENT_VERSION:-v0.0.3}"
REPO="${DECENT_REPO:-kcodes0/decent}"
INSTALL_DIR="${DECENT_INSTALL_DIR:-$HOME/.local/bin}"
SOURCE_DIR="${DECENT_SOURCE_DIR:-}"
TMP_DIR="$(mktemp -d)"

cleanup() {
  rm -rf "$TMP_DIR"
}
trap cleanup EXIT INT TERM

need_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "decent install: required command '$1' was not found." >&2
    exit 1
  fi
}

download() {
  url="$1"
  out="$2"
  if command -v curl >/dev/null 2>&1; then
    curl -fsSL "$url" -o "$out" && return 0
    return 1
  fi
  if command -v wget >/dev/null 2>&1; then
    wget -qO "$out" "$url" && return 0
    return 1
  fi
  echo "decent install: curl or wget is required." >&2
  exit 1
}

append_path_hint() {
  shell_name="$(basename "${SHELL:-sh}")"
  rc_file=""
  case "$shell_name" in
    zsh) rc_file="$HOME/.zshrc" ;;
    bash) rc_file="$HOME/.bashrc" ;;
    *)
      if [ -f "$HOME/.zshrc" ]; then
        rc_file="$HOME/.zshrc"
      elif [ -f "$HOME/.bashrc" ]; then
        rc_file="$HOME/.bashrc"
      else
        rc_file="$HOME/.profile"
      fi
      ;;
  esac

  line="export PATH=\"$INSTALL_DIR:\$PATH\""
  if [ ! -f "$rc_file" ]; then
    touch "$rc_file"
  fi
  if ! grep -F "$line" "$rc_file" >/dev/null 2>&1; then
    printf '\n# Added by decent installer\n%s\n' "$line" >> "$rc_file"
    echo "Added $INSTALL_DIR to PATH in $rc_file"
  fi
}

need_cmd go
need_cmd tar
need_cmd mktemp

ARCHIVE="$TMP_DIR/decent.tar.gz"
SRC_DIR="$TMP_DIR/src"
TAG_URL="https://codeload.github.com/$REPO/tar.gz/refs/tags/$VERSION"
MAIN_URL="https://codeload.github.com/$REPO/tar.gz/refs/heads/main"

echo "Installing decent $VERSION..."
if [ -n "$SOURCE_DIR" ]; then
  SOURCE_ROOT="$SOURCE_DIR"
elif download "$TAG_URL" "$ARCHIVE"; then
  mkdir -p "$SRC_DIR"
  tar -xzf "$ARCHIVE" -C "$SRC_DIR"
  SOURCE_ROOT="$(find "$SRC_DIR" -mindepth 1 -maxdepth 1 -type d | head -n 1)"
elif download "$MAIN_URL" "$ARCHIVE"; then
  echo "Could not download tag $VERSION, falling back to main." >&2
  mkdir -p "$SRC_DIR"
  tar -xzf "$ARCHIVE" -C "$SRC_DIR"
  SOURCE_ROOT="$(find "$SRC_DIR" -mindepth 1 -maxdepth 1 -type d | head -n 1)"
else
  need_cmd git
  echo "Could not download an archive. Falling back to git clone." >&2
  SOURCE_ROOT="$TMP_DIR/repo"
  if git clone --depth 1 --branch "$VERSION" "https://github.com/$REPO.git" "$SOURCE_ROOT" >/dev/null 2>&1; then
    :
  elif git clone --depth 1 --branch main "https://github.com/$REPO.git" "$SOURCE_ROOT" >/dev/null 2>&1; then
    :
  else
    git clone --depth 1 "https://github.com/$REPO.git" "$SOURCE_ROOT" >/dev/null
  fi
fi

mkdir -p "$INSTALL_DIR"

(
  cd "$SOURCE_ROOT"
  echo "Building decent..."
  GOBIN="$INSTALL_DIR" go install ./cmd/decent ./cmd/decent-node
)

append_path_hint

echo
echo "decent is installed in $INSTALL_DIR"
echo "Open a new shell, or run:"
echo "  export PATH=\"$INSTALL_DIR:\$PATH\""
echo
echo "Then check it with:"
echo "  decent version"
