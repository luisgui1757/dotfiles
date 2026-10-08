#!/usr/bin/env bash
# Build and launch the installer from this trusted checkout. No system installs.
set -euo pipefail

fail() { printf 'Dotfiles bootstrap: %s\n' "$*" >&2; exit 1; }
[[ $# == 0 || ( $# == 1 && $1 == machine ) ]] ||
  fail "run without arguments for the menu; automation uses 'machine' with JSON on stdin"

checkout="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
launch_directory=$PWD
case "$(uname -s):$(uname -m)" in
  Darwin:arm64) platform=darwin-arm64 ;;
  Linux:x86_64) platform=linux-amd64 ;;
  Linux:aarch64|Linux:arm64) platform=linux-arm64 ;;
  *) fail 'supported targets are Apple Silicon macOS, Linux amd64/arm64, and native Windows amd64 (use installer-bootstrap.ps1)' ;;
esac

version='' filename='' digest='' size='' seen=' '
while IFS=$'\t' read -r row_version row_platform row_filename row_digest row_size extra; do
  [[ $row_version == \#* || $row_version == version ]] && continue
  [[ $row_version =~ ^go[0-9]+\.[0-9]+\.[0-9]+$ &&
     $row_platform =~ ^(darwin-arm64|linux-amd64|linux-arm64|windows-amd64)$ &&
     $row_digest =~ ^[a-f0-9]{64}$ && $row_size =~ ^[1-9][0-9]*$ && -z $extra ]] ||
    fail 'invalid toolchain pin metadata'
  case "$row_platform:$row_filename" in
    windows-amd64:"$row_version.$row_platform.zip") ;;
    darwin-arm64:"$row_version.$row_platform.tar.gz"|linux-*:"$row_version.$row_platform.tar.gz") ;;
    *) fail 'toolchain filename does not match its version and platform' ;;
  esac
  [[ $seen != *" $row_platform "* ]] || fail 'duplicate toolchain target'
  seen+="$row_platform "
  [[ $row_platform == "$platform" ]] || continue
  [[ -z $version ]] || fail 'duplicate toolchain target'
  version=$row_version filename=$row_filename digest=$row_digest size=$row_size
done < "$checkout/installer/bootstrap-toolchain.tsv"
[[ -n $version ]] || fail 'toolchain target is missing'
[[ $(awk '$1 == "go" { print "go" $2 }' "$checkout/installer/go.mod") == "$version" ]] ||
  fail 'toolchain pin must match installer/go.mod'

sha256() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum | awk '{print $1}'
  elif command -v shasum >/dev/null 2>&1; then
    shasum -a 256 | awk '{print $1}'
  else
    fail 'SHA-256 verification requires sha256sum or shasum'
  fi
}
cache="${XDG_CACHE_HOME:-$HOME/.cache}/dotfiles/bootstrap"
[[ $cache == /* ]] || fail 'cache directory must be absolute'
umask 077
mkdir -p "$cache"
[[ ! -L $cache && -O $cache ]] || fail 'bootstrap cache must be a directory owned by this user'
stage="$(mktemp -d "$cache/.prepare.XXXXXXXX")"
locked=0
cleanup() {
  rm -rf "$stage"
  if [[ $locked == 1 ]]; then rmdir "$cache/lock"; fi
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM
if ! mkdir "$cache/lock" 2>/dev/null; then
  fail "another bootstrap holds $cache/lock; after confirming no bootstrap is running, remove that empty directory and retry"
fi
locked=1
archive="$cache/$filename"
if [[ ! -f $archive ]]; then
  printf 'Downloading verified %s toolchain…\n' "$version" >&2
  url="https://go.dev/dl/$filename"
  if command -v curl >/dev/null 2>&1; then
    curl --fail --location --silent --show-error --proto '=https' --proto-redir '=https' \
      --connect-timeout 30 --max-time 600 --output "$stage/archive" "$url"
  elif command -v wget >/dev/null 2>&1; then
    wget --https-only --timeout=30 --tries=2 -q -O "$stage/archive" "$url"
  else
    fail 'download requires curl or wget'
  fi
  [[ $(wc -c < "$stage/archive" | tr -d ' ') == "$size" && $(sha256 < "$stage/archive") == "$digest" ]] ||
    fail 'Go archive checksum or size mismatch; nothing was extracted or executed'
  mv "$stage/archive" "$archive"
fi
[[ ! -L $archive && $(wc -c < "$archive" | tr -d ' ') == "$size" && $(sha256 < "$archive") == "$digest" ]] ||
  fail "cached Go archive is damaged; remove $archive and retry"
# Re-extract the verified cached archive before executing compiler code. A
# cached compiler's version string cannot prove its bytes are still authentic.
toolchain="$stage/toolchain"
mkdir "$toolchain"
tar -xzf "$archive" -C "$toolchain"
[[ -x $toolchain/go/bin/go ]] || fail 'verified archive has no Go executable'
go="$toolchain/go/bin/go"
# Ignore personal Go settings/workspaces and keep all compiler writes in this cache.
compiler() (
  export GOENV=off GOWORK=off GOTOOLCHAIN=local CGO_ENABLED=0 GOFLAGS='' GOEXPERIMENT='' GO111MODULE=on GOFIPS140=off
  export GOCACHEPROG=''
  export GOROOT="$toolchain/go" GOPATH="$cache/gopath" GOCACHE="$cache/go-build"
  # Go's telemetry directory ignores GOENV. Isolate its config root as well.
  export HOME="$cache/compiler-home" XDG_CONFIG_HOME="$cache/compiler-config" GOTMPDIR="$stage"
  export GOOS="${platform%-*}" GOARCH="${platform#*-}" GOAMD64=v1 GOARM64=v8.0
  export GOPROXY=https://proxy.golang.org GOSUMDB=sum.golang.org
  unset GOPRIVATE GONOPROXY GONOSUMDB
  "$go" "$@"
)
[[ $(compiler version) == "go version $version ${platform%-*}/${platform#*-}" ]] || fail 'cached compiler version mismatch'
cd "$checkout/installer"
compiler mod verify >&2
source_identity() {
  compiler list -mod=readonly -deps -f '{{if .Module}}{{if .Module.Main}}{{range .GoFiles}}{{$.Dir}}/{{.}}{{println}}{{end}}{{range .SFiles}}{{$.Dir}}/{{.}}{{println}}{{end}}{{range .HFiles}}{{$.Dir}}/{{.}}{{println}}{{end}}{{range .SysoFiles}}{{$.Dir}}/{{.}}{{println}}{{end}}{{range .EmbedFiles}}{{$.Dir}}/{{.}}{{println}}{{end}}{{end}}{{end}}' ./cmd/dotfiles > "$stage/inputs"
  {
    printf '%s\n' "$version" "$platform" "$digest" "$(sha256 < "$checkout/scripts/installer-bootstrap.sh")"
    printf '%s\n' "$checkout/installer/go.mod" "$checkout/installer/go.sum" >> "$stage/inputs"
    LC_ALL=C sort -u "$stage/inputs" | while IFS= read -r path; do
      [[ -n $path ]] || continue
      [[ $path == "$checkout/installer/"* && -f $path && ! -L $path ]] || fail 'unexpected or redirected build input'
      printf '%s\t%s\n' "${path#"$checkout/installer/"}" "$(sha256 < "$path")"
    done
  } | sha256
}
identity="$(source_identity)"
binary="$cache/dotfiles-$identity"
if [[ ! -f $binary ]]; then
  printf 'Building Dotfiles from the trusted checkout…\n' >&2
  compiler build -mod=readonly -trimpath -buildvcs=false -pgo=off -o "$stage/dotfiles" ./cmd/dotfiles
  [[ $(source_identity) == "$identity" ]] || fail 'checkout changed while building; retry'
  sha256 < "$stage/dotfiles" > "$stage/dotfiles.sha256"
  mv "$stage/dotfiles.sha256" "$binary.sha256"
  mv "$stage/dotfiles" "$binary"
fi
[[ ! -L $binary && -x $binary && -f $binary.sha256 && $(sha256 < "$binary") == "$(cat "$binary.sha256")" ]] ||
  fail "cached installer is damaged; remove $binary and retry"
export DOTFILES_CHECKOUT="$checkout"
cd "$launch_directory"
cleanup
trap - EXIT INT TERM
exec "$binary" "$@"
