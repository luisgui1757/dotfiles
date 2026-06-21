#!/usr/bin/env bash
# Install a pinned chezmoi release archive without executing a remote installer.
set -euo pipefail

version="${CHEZMOI_VERSION:?CHEZMOI_VERSION is required}"
bin_dir="${1:-$HOME/.local/bin}"
version_no_v="${version#v}"

case "$(uname -s):$(uname -m)" in
    Linux:x86_64|Linux:amd64)
        os="linux"
        arch="amd64"
        expected="${CHEZMOI_LINUX_X86_64_SHA256:?CHEZMOI_LINUX_X86_64_SHA256 is required}"
        ;;
    Linux:aarch64|Linux:arm64)
        os="linux"
        arch="arm64"
        expected="${CHEZMOI_LINUX_ARM64_SHA256:?CHEZMOI_LINUX_ARM64_SHA256 is required}"
        ;;
    Darwin:x86_64|Darwin:amd64)
        os="darwin"
        arch="amd64"
        expected="${CHEZMOI_DARWIN_X86_64_SHA256:?CHEZMOI_DARWIN_X86_64_SHA256 is required}"
        ;;
    Darwin:arm64|Darwin:aarch64)
        os="darwin"
        arch="arm64"
        expected="${CHEZMOI_DARWIN_ARM64_SHA256:?CHEZMOI_DARWIN_ARM64_SHA256 is required}"
        ;;
    *)
        echo "FAIL: unsupported chezmoi release platform: $(uname -s)/$(uname -m)" >&2
        exit 1
        ;;
esac

asset="chezmoi_${version_no_v}_${os}_${arch}.tar.gz"
url="https://github.com/twpayne/chezmoi/releases/download/${version}/${asset}"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

curl -fsSL "$url" -o "$tmp/$asset"
if command -v sha256sum >/dev/null 2>&1; then
    printf '%s  %s\n' "$expected" "$tmp/$asset" | sha256sum -c -
elif command -v shasum >/dev/null 2>&1; then
    actual="$(shasum -a 256 "$tmp/$asset" | awk '{print $1}')"
    if [[ "$actual" != "$expected" ]]; then
        echo "FAIL: checksum mismatch for $asset: got $actual expected $expected" >&2
        exit 1
    fi
else
    echo "FAIL: sha256sum or shasum is required to verify $asset" >&2
    exit 1
fi

tar -xzf "$tmp/$asset" -C "$tmp"
source_bin="$tmp/chezmoi"
if [[ ! -f "$source_bin" ]]; then
    source_bin="$(find "$tmp" -type f -name chezmoi -print -quit)"
fi
if [[ -z "$source_bin" || ! -f "$source_bin" ]]; then
    echo "FAIL: chezmoi binary missing from $asset" >&2
    exit 1
fi

mkdir -p "$bin_dir"
install -m 0755 "$source_bin" "$bin_dir/chezmoi"
"$bin_dir/chezmoi" --version
