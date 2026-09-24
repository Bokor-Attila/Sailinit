#!/bin/sh
# Installs the latest sailinit release on macOS or Linux.
#
#   curl -fsSL https://raw.githubusercontent.com/Bokor-Attila/Sailinit/main/install.sh | sh
#
# Set SAILINIT_INSTALL_DIR to install somewhere other than /usr/local/bin.

set -eu

REPO="Bokor-Attila/Sailinit"
BINARY="sailinit"

err() {
    printf 'sailinit install: %s\n' "$*" >&2
    exit 1
}

need() {
    command -v "$1" >/dev/null 2>&1 || err "required command not found: $1"
}

detect_asset() {
    case "$(uname -s)" in
        Darwin) os="macos" ;;
        Linux) os="linux" ;;
        *) err "unsupported OS: $(uname -s)" ;;
    esac

    case "$(uname -m)" in
        x86_64 | amd64) arch="amd64" ;;
        arm64 | aarch64) arch="arm64" ;;
        *) err "unsupported architecture: $(uname -m)" ;;
    esac

    # An x86_64 shell under Rosetta still runs on Apple Silicon; install the native build.
    if [ "$os" = "macos" ] && [ "$arch" = "amd64" ] &&
        [ "$(sysctl -n sysctl.proc_translated 2>/dev/null || echo 0)" = "1" ]; then
        arch="arm64"
    fi

    printf '%s-%s-%s' "$BINARY" "$os" "$arch"
}

sha256() {
    if command -v sha256sum >/dev/null 2>&1; then
        sha256sum "$1" | cut -d ' ' -f 1
    elif command -v shasum >/dev/null 2>&1; then
        shasum -a 256 "$1" | cut -d ' ' -f 1
    else
        err "no sha256sum or shasum available to verify the download"
    fi
}

main() {
    need curl
    need uname

    asset="$(detect_asset)"
    install_dir="${SAILINIT_INSTALL_DIR:-/usr/local/bin}"
    base_url="https://github.com/${REPO}/releases/latest/download"

    tmp="$(mktemp -d)"
    trap 'rm -rf "$tmp"' EXIT INT TERM

    printf 'Downloading %s...\n' "$asset" >&2
    curl -fsSL "${base_url}/${asset}" -o "${tmp}/${asset}" || err "download failed: ${base_url}/${asset}"
    curl -fsSL "${base_url}/sha256sums.txt" -o "${tmp}/sha256sums.txt" || err "checksum download failed"

    expected="$(awk -v f="$asset" '$2 == f || $2 == "*" f { print $1 }' "${tmp}/sha256sums.txt")"
    [ -n "$expected" ] || err "no checksum listed for ${asset}"
    actual="$(sha256 "${tmp}/${asset}")"
    [ "$expected" = "$actual" ] || err "checksum mismatch for ${asset} (expected ${expected}, got ${actual})"

    chmod +x "${tmp}/${asset}"

    sudo=""
    if [ ! -d "$install_dir" ]; then
        mkdir -p "$install_dir" 2>/dev/null || sudo="sudo"
    elif [ ! -w "$install_dir" ]; then
        sudo="sudo"
    fi
    if [ -n "$sudo" ]; then
        need sudo
        printf 'Installing to %s requires sudo.\n' "$install_dir" >&2
        sudo mkdir -p "$install_dir"
    fi

    $sudo mv "${tmp}/${asset}" "${install_dir}/${BINARY}"

    printf 'Installed %s to %s/%s\n' "$("${install_dir}/${BINARY}" --version 2>/dev/null || echo "$BINARY")" "$install_dir" "$BINARY" >&2

    case ":${PATH}:" in
        *":${install_dir}:"*) ;;
        *) printf 'Note: %s is not on your PATH.\n' "$install_dir" >&2 ;;
    esac
}

# Everything runs from main so a truncated download never executes half a script.
main "$@"
