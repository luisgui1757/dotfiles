#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
OLD_COMMIT="015617362830280bf85c7142e69d0681d376d453"
OLD_TAG_OBJECT="a3b4d6d7b6d289959cac68d76faec96219b3e310"
REAL_GIT="$(command -v git)"
REAL_CHEZMOI="$(command -v chezmoi || true)"
FIXTURE_PLATFORM="${TEST_UPGRADE_PLATFORM:-Linux}"
WORK="$(mktemp -d)"
WORK="$(cd "$WORK" && pwd -P)"
trap 'chmod -R u+w "$WORK" 2>/dev/null || true; rm -rf "$WORK"' EXIT

fail() {
    echo "FAIL: $*" >&2
    exit 1
}

refresh_payload_digest() {
    local recovery="$1" file="$2"
    chmod u+w "$recovery/payload.sha256"
    python3 - "$recovery" "$file" <<'PY'
import hashlib
import pathlib
import sys

recovery = pathlib.Path(sys.argv[1])
name = sys.argv[2]
payload = recovery / name
digest = hashlib.sha256(payload.read_bytes()).hexdigest()
manifest = recovery / "payload.sha256"
lines = manifest.read_text().splitlines()
matches = [index for index, line in enumerate(lines) if line.endswith(f"  {name}")]
if len(matches) != 1:
    raise SystemExit(f"manifest identity is not unique: {name}")
lines[matches[0]] = f"{digest}  {name}"
manifest.write_text("\n".join(lines) + "\n")
PY
    chmod 400 "$recovery/payload.sha256"
}

[[ -n "$REAL_CHEZMOI" ]] || {
    echo "SKIP: chezmoi is required for the exact v0.1.0 upgrade test"
    exit 0
}
git -C "$REPO_ROOT" cat-file -e "$OLD_COMMIT^{commit}" 2>/dev/null || {
    fail "exact v0.1.0 commit is unavailable; CI checkout must fetch release history"
}
[[ "$(git -C "$REPO_ROOT" rev-parse 'v0.1.0^{commit}')" == "$OLD_COMMIT" ]] || \
    fail "v0.1.0 tag does not peel to the reviewed baseline"
[[ "$(git -C "$REPO_ROOT" rev-parse v0.1.0)" == "$OLD_TAG_OBJECT" ]] || \
    fail "v0.1.0 tag object drifted"

old_checkout="$WORK/dotfiles-v0.1.0"
new_checkout="$WORK/dotfiles-v0.3.0"
home="$WORK/home"
bin="$WORK/bin"
mkdir -p "$home" "$bin"

git clone -q --no-checkout "$REPO_ROOT" "$old_checkout"
git -C "$old_checkout" checkout -q --detach "$OLD_COMMIT"
git -C "$old_checkout" remote set-url origin \
    https://github.com/luisgui1757/dotfiles.git

mkdir -p "$new_checkout/scripts"
mkdir -p "$new_checkout/nvim"
printf '%s\n' 'release-upgrade-fixture' > "$new_checkout/nvim/.fixture"
cp "$REPO_ROOT/scripts/upgrade-v0.1.0.sh" "$new_checkout/scripts/"
cp "$REPO_ROOT/flake.lock" "$new_checkout/"
cp -R "$REPO_ROOT/home" "$new_checkout/"
cp "$REPO_ROOT/uninstall.sh" "$new_checkout/"
cp "$REPO_ROOT/setup.sh" "$new_checkout/setup-real.sh"
cat > "$new_checkout/setup.sh" <<'SETUP'
#!/usr/bin/env bash
set -euo pipefail
root="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"
"$root/setup-real.sh" "$@"
if [[ -n "${TEST_SETUP_SIGNAL:-}" ]]; then
    kill -s "$TEST_SETUP_SIGNAL" "$PPID"
    sleep 1
    exit 143
fi
if [[ "${TEST_SETUP_FAIL:-}" == "1" ]]; then
    exit 42
fi
SETUP
chmod +x "$new_checkout/setup.sh" "$new_checkout/setup-real.sh" \
    "$new_checkout/scripts/upgrade-v0.1.0.sh"
(
    cd "$new_checkout"
    git init -q
    git config user.name test
    git config user.email test@example.invalid
    git add .
    git commit -qm fixture
    git tag -am fixture v0.3.0
    git remote add origin https://github.com/luisgui1757/dotfiles.git
)
new_commit="$(git -C "$new_checkout" rev-parse HEAD)"
new_tag_object="$(git -C "$new_checkout" rev-parse v0.3.0)"

cat > "$bin/git" <<'GIT'
#!/usr/bin/env bash
set -euo pipefail
if [[ "$*" == *"ls-remote --tags https://github.com/luisgui1757/dotfiles.git"* ]]; then
    case "$*" in
        *refs/tags/v0.1.0*)
            printf '%s\t%s\n' \
                "$TEST_OLD_TAG_OBJECT" refs/tags/v0.1.0 \
                "$TEST_OLD_COMMIT" 'refs/tags/v0.1.0^{}'
            ;;
        *refs/tags/v0.3.0*)
            printf '%s\t%s\n' \
                "$TEST_NEW_TAG_OBJECT" refs/tags/v0.3.0 \
                "$TEST_NEW_COMMIT" 'refs/tags/v0.3.0^{}'
            ;;
        *) exit 91 ;;
    esac
    exit 0
fi
exec "$TEST_REAL_GIT" "$@"
GIT

cat > "$bin/uname" <<'UNAME'
#!/usr/bin/env bash
case "${1:-}" in
    -s) echo "$TEST_FIXTURE_PLATFORM" ;;
    -m)
        if [[ "$TEST_FIXTURE_PLATFORM" == "Darwin" ]]; then echo arm64; else echo x86_64; fi
        ;;
    *) echo "$TEST_FIXTURE_PLATFORM" ;;
esac
UNAME

cat > "$bin/getent" <<'GETENT'
#!/usr/bin/env bash
if [[ "${1:-}" == "passwd" ]]; then
    printf '%s:x:501:20:test:%s:/bin/zsh\n' "$2" "$TEST_HOME"
    exit 0
fi
exit 1
GETENT

cat > "$bin/dscl" <<'DSCL'
#!/usr/bin/env bash
if [[ "$*" == *"NFSHomeDirectory"* ]]; then
    printf 'NFSHomeDirectory: %s\n' "$TEST_HOME"
    exit 0
fi
exit 1
DSCL

cat > "$bin/brew" <<'BREW'
#!/usr/bin/env bash
set -euo pipefail
case "${1:-}" in
    --repository)
        echo "$TEST_BREW_REPOSITORY"
        ;;
    shellenv)
        printf 'export PATH=%q:$PATH\n' "$(dirname "$0")"
        ;;
    list)
        case "${2:-}" in
            --formula|--cask)
                if [[ "$#" -eq 3 && "${3:-}" == "--versions" ]]; then exit 0; fi
                exit 1
                ;;
            *) exit 1 ;;
        esac
        ;;
    uninstall)
        printf '%s\n' "$*" >> "$TEST_BREW_LOG"
        ;;
    *) exit 1 ;;
esac
BREW

cat > "$bin/sudo" <<'SUDO'
#!/usr/bin/env bash
set -euo pipefail
if [[ "${1:-}" == "-H" ]]; then
    shift
fi
if [[ "${1:-}" == "env" ]]; then
    shift
    exec env "$@"
fi
if [[ "${1:-}" == "mv" ]]; then
    exit 0
fi
exec "$@"
SUDO

cat > "$bin/nix" <<'NIX'
#!/usr/bin/env bash
set -euo pipefail
if [[ " $* " == *"#darwin-uninstaller"* ]]; then
    rm -f "$TEST_BIN/darwin-rebuild"
    printf '%s\n' "$*" >> "$TEST_NIX_LOG"
    exit 0
fi
case "${1:-}" in
    --version)
        echo 'nix (fixture) 2.34.0'
        ;;
    store)
        [[ "${2:-}" == "info" ]]
        ;;
    eval)
        printf '%s\n%s\n' \
            1111111111111111111111111111111111111111 \
            sha256-AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=
        ;;
    run)
        if [[ " $* " == *" uninstall "* ]]; then
            rm -rf \
                "${XDG_STATE_HOME:-$HOME/.local/state}/home-manager" \
                "${XDG_STATE_HOME:-$HOME/.local/state}/nix/profiles/home-manager" \
                "${XDG_DATA_HOME:-$HOME/.local/share}/home-manager"
        elif [[ "$TEST_FIXTURE_PLATFORM" == "Darwin" ]]; then
            if [[ "${TEST_MUTATE_RELEASE_SOURCE:-}" == "1" ]]; then
                printf '%s\n' post-validation-drift >> "$TEST_NEW_CHECKOUT/home/dot_zshrc"
            fi
            cat > "$TEST_BIN/darwin-rebuild" <<'DARWIN'
#!/usr/bin/env bash
exit 0
DARWIN
            chmod +x "$TEST_BIN/darwin-rebuild"
        else
            if [[ "${TEST_MUTATE_RELEASE_SOURCE:-}" == "1" ]]; then
                printf '%s\n' post-validation-drift >> "$TEST_NEW_CHECKOUT/home/dot_zshrc"
            fi
            mkdir -p \
                "${XDG_STATE_HOME:-$HOME/.local/state}/nix/profiles" \
                "${XDG_STATE_HOME:-$HOME/.local/state}/home-manager" \
                "${XDG_DATA_HOME:-$HOME/.local/share}/home-manager"
            ln -sfn "$TEST_NEW_CHECKOUT" \
                "${XDG_STATE_HOME:-$HOME/.local/state}/nix/profiles/home-manager"
        fi
        printf '%s\n' "$*" >> "$TEST_NIX_LOG"
        ;;
    *)
        echo "unexpected nix fixture invocation: $*" >&2
        exit 92
        ;;
esac
NIX
chmod +x "$bin/git" "$bin/uname" "$bin/getent" "$bin/dscl" "$bin/brew" \
    "$bin/sudo" "$bin/nix"

export HOME="$home"
export XDG_CONFIG_HOME="$home/.config"
export XDG_CACHE_HOME="$home/.cache"
export XDG_DATA_HOME="$home/.local/share"
export XDG_STATE_HOME="$home/.local/state"
export TEST_HOME="$home"
export TEST_FIXTURE_PLATFORM="$FIXTURE_PLATFORM"
export TEST_BIN="$bin"
export TEST_BREW_REPOSITORY="$WORK/homebrew"
export TEST_BREW_LOG="$WORK/brew.log"
export TEST_REAL_GIT="$REAL_GIT"
export TEST_OLD_TAG_OBJECT="$OLD_TAG_OBJECT"
export TEST_OLD_COMMIT="$OLD_COMMIT"
export TEST_NEW_TAG_OBJECT="$new_tag_object"
export TEST_NEW_COMMIT="$new_commit"
export TEST_NEW_CHECKOUT="$new_checkout"
export TEST_NIX_LOG="$WORK/nix.log"
mkdir -p "$TEST_BREW_REPOSITORY/Library"
chezmoi_bin_dir="$(dirname "$REAL_CHEZMOI")"
export PATH="$bin:$chezmoi_bin_dir:/usr/bin:/bin:/usr/sbin:/sbin"

mkdir -p \
    "$HOME/.config/gh-dash" \
    "$HOME/.config/lsd" \
    "$HOME/.config/nvim" \
    "$HOME/.config" \
    "$HOME/Library/Application Support/com.mitchellh.ghostty" \
    "$HOME/Library/Application Support/lazygit"

"$REAL_CHEZMOI" --source "$old_checkout/home" --destination "$HOME" init
while IFS= read -r historical_target; do
    [[ -n "$historical_target" ]] || continue
    mkdir -p "$(dirname "$historical_target")"
done < <("$REAL_CHEZMOI" --source "$old_checkout/home" --destination "$HOME" \
    managed --path-style absolute --include files,symlinks)

"$REAL_CHEZMOI" --source "$old_checkout/home" --destination "$HOME" \
    --no-tty --force apply --include files,symlinks
old_hash="$(shasum -a 256 "$HOME/.zshrc" | awk '{print $1}')"

if "$new_checkout/scripts/upgrade-v0.1.0.sh" --preflight-only "$new_checkout" \
    > "$WORK/in-place.out" 2>&1; then
    fail "in-place release migration unexpectedly passed"
fi
grep -F "in-place migration is forbidden" "$WORK/in-place.out" >/dev/null || \
    fail "in-place rejection was not explicit"
[[ "$(shasum -a 256 "$HOME/.zshrc" | awk '{print $1}')" == "$old_hash" ]] || \
    fail "in-place preflight changed live config"
echo "ok  : in-place checkout migration fails before mutation"

preflight_output="$("$new_checkout/scripts/upgrade-v0.1.0.sh" \
    --preflight-only "$old_checkout")"
grep -F "preflight passed; no live state changed" <<<"$preflight_output" >/dev/null
[[ "$(shasum -a 256 "$HOME/.zshrc" | awk '{print $1}')" == "$old_hash" ]] || \
    fail "read-only preflight changed live config"
echo "ok  : exact tag, identity, Nix, and historical config preflight is write-free"

printf '%s\n' local-change >> "$old_checkout/home/dot_zshrc"
if "$new_checkout/scripts/upgrade-v0.1.0.sh" --preflight-only "$old_checkout" \
    > "$WORK/dirty.out" 2>&1; then
    fail "tracked v0.1.0 source drift unexpectedly passed"
fi
grep -F "old checkout has tracked or untracked changes" "$WORK/dirty.out" >/dev/null
git -C "$old_checkout" restore home/dot_zshrc
touch "$old_checkout/untracked-collision"
if "$new_checkout/scripts/upgrade-v0.1.0.sh" --preflight-only "$old_checkout" \
    > "$WORK/untracked.out" 2>&1; then
    fail "untracked v0.1.0 source drift unexpectedly passed"
fi
grep -F "old checkout has tracked or untracked changes" "$WORK/untracked.out" >/dev/null
rm "$old_checkout/untracked-collision"
echo "ok  : tracked and untracked historical checkout drift fail before mutation"

set +e
TEST_MUTATE_RELEASE_SOURCE=1 TEST_SETUP_FAIL=1 \
    "$new_checkout/scripts/upgrade-v0.1.0.sh" --apply "$old_checkout" \
    > "$WORK/frozen-source.out" 2>&1
frozen_source_rc=$?
set -e
[[ "$frozen_source_rc" -eq 42 ]] || {
    cat "$WORK/frozen-source.out" >&2
    fail "post-validation source mutation returned $frozen_source_rc instead of 42"
}
frozen_source_recovery="$(awk -F': ' '/^Recovery directory:/ { print $2 }' "$WORK/frozen-source.out")"
grep -F post-validation-drift "$new_checkout/home/dot_zshrc" >/dev/null ||
    fail "source-mutation fixture did not alter the retained checkout"
if grep -F post-validation-drift "$frozen_source_recovery/new-release/home/dot_zshrc" >/dev/null; then
    fail "post-validation checkout drift changed frozen release bytes"
fi
[[ "$(< "$frozen_source_recovery/stage")" == "rolled-back" ]] ||
    fail "frozen-source failure did not complete automatic rollback"
[[ "$(shasum -a 256 "$HOME/.zshrc" | awk '{print $1}')" == "$old_hash" ]] ||
    fail "frozen-source failure did not restore v0.1.0 config"
git -C "$new_checkout" restore home/dot_zshrc
echo "ok  : post-validation checkout drift cannot change published or rollback bytes"

set +e
TEST_SETUP_FAIL=1 "$new_checkout/scripts/upgrade-v0.1.0.sh" --apply "$old_checkout" \
    > "$WORK/failure.out" 2>&1
failure_rc=$?
set -e
[[ "$failure_rc" -eq 42 ]] || {
    cat "$WORK/failure.out" >&2
    fail "injected post-publication failure returned $failure_rc instead of 42"
}
failure_recovery="$(awk -F': ' '/^Recovery directory:/ { print $2 }' "$WORK/failure.out")"
[[ -d "$failure_recovery" ]] || fail "failure did not retain recovery material"
[[ "$(< "$failure_recovery/stage")" == "rolled-back" ]] || {
    cat "$WORK/failure.out" >&2
    fail "failure did not complete automatic rollback"
}
[[ "$(shasum -a 256 "$HOME/.zshrc" | awk '{print $1}')" == "$old_hash" ]] || \
    fail "automatic rollback did not restore v0.1.0 config bytes"
[[ "$(python3 -c 'import os,sys; print(os.path.realpath(sys.argv[1]))' "$HOME/.zshrc")" == \
    "$failure_recovery/old-release/home/dot_zshrc" ]] || \
    fail "automatic rollback did not restore the frozen v0.1.0 source"
zsh_backup="$(find "$HOME" -maxdepth 1 -name '.zshrc.bak.*' -print | LC_ALL=C sort | sed -n '1p')"
[[ -L "$zsh_backup" ]] || fail "config publication did not retain a pre-mutation zsh backup"
[[ "$(python3 -c 'import os,sys; print(os.path.realpath(sys.argv[1]))' "$zsh_backup")" == \
    "$old_checkout/home/dot_zshrc" ]] || fail "pre-mutation zsh backup does not retain v0.1.0 ownership"
if [[ "$FIXTURE_PLATFORM" == "Darwin" ]]; then
    [[ ! -e "$bin/darwin-rebuild" ]] || fail "nix-darwin state survived rollback"
    grep -F '#darwin-uninstaller' "$TEST_NIX_LOG" >/dev/null ||
        fail "rollback did not use the locked nix-darwin uninstaller"
else
    [[ ! -e "$XDG_STATE_HOME/home-manager" ]] || fail "Home Manager state survived rollback"
    grep -F "#home-manager" "$TEST_NIX_LOG" >/dev/null ||
        fail "rollback did not use the locked Home Manager app"
fi
[[ ! -e "$XDG_STATE_HOME/dotfiles/zsh-plugin-publisher.initialized" ]] ||
    fail "release migration executed a deferred chezmoi run script"
echo "ok  : later failure restores exact v0.1.0 config and removes the first Nix package activation"

attack_target="$HOME/.config/dotfiles-upgrade-attack"
ln -s "$new_checkout/home/dot_zshrc" "$attack_target"
cp "$failure_recovery/new-targets.txt" "$WORK/new-targets.original"
chmod u+w "$failure_recovery/new-targets.txt"
printf '%s\n' "$attack_target" >> "$failure_recovery/new-targets.txt"
chmod 400 "$failure_recovery/new-targets.txt"
refresh_payload_digest "$failure_recovery" new-targets.txt
if "$failure_recovery/upgrade-v0.1.0.sh" --rollback "$failure_recovery" \
    > "$WORK/altered-targets.out" 2>&1; then
    fail "coherently altered recovery target inventory unexpectedly passed"
fi
grep -F "target inventory differs" "$WORK/altered-targets.out" >/dev/null
[[ -L "$attack_target" ]] || fail "invalid recovery material mutated an unrelated target"
[[ "$(shasum -a 256 "$HOME/.zshrc" | awk '{print $1}')" == "$old_hash" ]] || \
    fail "invalid recovery material changed v0.1.0 config"
chmod u+w "$failure_recovery/new-targets.txt"
cp "$WORK/new-targets.original" "$failure_recovery/new-targets.txt"
chmod 400 "$failure_recovery/new-targets.txt"
refresh_payload_digest "$failure_recovery" new-targets.txt
rm "$attack_target"

cp "$failure_recovery/flake.lock" "$WORK/flake.lock.original"
chmod u+w "$failure_recovery/flake.lock"
printf '\n' >> "$failure_recovery/flake.lock"
chmod 400 "$failure_recovery/flake.lock"
refresh_payload_digest "$failure_recovery" flake.lock
if "$failure_recovery/upgrade-v0.1.0.sh" --rollback "$failure_recovery" \
    > "$WORK/altered-lock.out" 2>&1; then
    fail "coherently altered recovery lockfile unexpectedly passed"
fi
grep -F "flake.lock differs" "$WORK/altered-lock.out" >/dev/null
chmod u+w "$failure_recovery/flake.lock"
cp "$WORK/flake.lock.original" "$failure_recovery/flake.lock"
chmod 400 "$failure_recovery/flake.lock"
refresh_payload_digest "$failure_recovery" flake.lock

printf '%s\n' invalid-stage > "$failure_recovery/stage"
if "$failure_recovery/upgrade-v0.1.0.sh" --rollback "$failure_recovery" \
    > "$WORK/invalid-stage.out" 2>&1; then
    fail "invalid recovery stage unexpectedly passed"
fi
grep -F "recovery stage is invalid" "$WORK/invalid-stage.out" >/dev/null
printf '%s\n' rolled-back > "$failure_recovery/stage"
echo "ok  : altered target, lock, and stage recovery material fail before mutation"

set +e
TEST_SETUP_SIGNAL=TERM "$new_checkout/scripts/upgrade-v0.1.0.sh" --apply "$old_checkout" \
    > "$WORK/interruption.out" 2>&1
interruption_rc=$?
set -e
[[ "$interruption_rc" -eq 143 ]] || {
    cat "$WORK/interruption.out" >&2
    fail "post-publication TERM returned $interruption_rc instead of 143"
}
interruption_recovery="$(awk -F': ' '/^Recovery directory:/ { print $2 }' "$WORK/interruption.out")"
[[ -d "$interruption_recovery" && "$(< "$interruption_recovery/stage")" == "rolled-back" ]] || {
    cat "$WORK/interruption.out" >&2
    fail "post-publication TERM did not retain a completed rollback"
}
[[ "$(shasum -a 256 "$HOME/.zshrc" | awk '{print $1}')" == "$old_hash" ]] ||
    fail "post-publication TERM did not restore v0.1.0 config"
echo "ok  : post-publication TERM retains recovery and completes automatic rollback"

success_output="$("$new_checkout/scripts/upgrade-v0.1.0.sh" --apply "$old_checkout")"
success_recovery="$(awk -F': ' '/^Recovery directory:/ { print $2 }' <<<"$success_output")"
[[ -d "$success_recovery" && "$(< "$success_recovery/stage")" == "applied" ]] || \
    fail "successful migration did not retain applied recovery state"
[[ "$(python3 -c 'import os,sys; print(os.path.realpath(sys.argv[1]))' "$HOME/.zshrc")" == \
    "$success_recovery/new-release/home/dot_zshrc" ]] || \
    fail "successful migration did not publish frozen v0.3.0 config"
[[ ! -e "$XDG_STATE_HOME/dotfiles/zsh-plugin-publisher.initialized" ]] ||
    fail "successful release migration executed a deferred chezmoi run script"
[[ -d "$old_checkout" ]] || fail "successful migration removed the old checkout"
"$success_recovery/upgrade-v0.1.0.sh" --accept "$success_recovery" \
    > "$WORK/accept.out"
[[ "$(< "$success_recovery/stage")" == "accepted" ]] || \
    fail "explicit acceptance did not close the migration"
if "$success_recovery/upgrade-v0.1.0.sh" --rollback "$success_recovery" \
    > "$WORK/accepted-rollback.out" 2>&1; then
    fail "accepted migration still allowed automatic rollback"
fi
grep -F "automatic rollback authority has ended" "$WORK/accepted-rollback.out" >/dev/null
echo "ok  : success retains both checkouts until explicit verified acceptance"

echo "all exact v0.1.0 POSIX release-upgrade behaviors OK"
