#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
INSTALL_DEPS_SOURCE_ONLY=1 source "$REPO_ROOT/install-deps.sh"

fail() { echo "FAIL: $1"; exit 1; }

WORK="$REPO_ROOT/tests/.cache/homebrew-shellenv-test"
rm -rf "$WORK"
mkdir -p "$WORK/home/.linuxbrew/bin" "$WORK/home/.linuxbrew/sbin" \
    "$WORK/home/.linuxbrew/opt/make/libexec/gnubin"
trap 'rm -rf "$WORK"' EXIT

export HOME="$WORK/home"
prefix="$HOME/.linuxbrew"

cat > "$prefix/bin/brew" <<EOF
#!/usr/bin/env bash
if [[ "\${1:-}" == "shellenv" ]]; then
    printf 'export HOMEBREW_PREFIX="%s"\n' "$prefix"
    printf 'export PATH="%s/bin:\$PATH"\n' "$prefix"
elif [[ "\${1:-}" == "--prefix" && "\${2:-}" == "make" ]]; then
    printf '%s\n' "$prefix/opt/make"
elif [[ "\${1:-}" == "--prefix" ]]; then
    printf '%s\n' "$prefix"
elif [[ "\${1:-}" == "--repository" ]]; then
    printf '%s\n' "$prefix"
fi
EOF
chmod +x "$prefix/bin/brew"

uname() {
    case "${1:-}" in
        -s) printf '%s\n' "Linux" ;;
        *) command uname "$@" ;;
    esac
}

homebrew_bin() { printf '%s\n' "$prefix/bin/brew"; }

PATH="/usr/bin:/bin"
DRY_RUN=0

[[ "$(detect_pm)" == "brew" ]] || fail "detect_pm did not find Linuxbrew outside PATH"

enable_homebrew_for_current_shell
[[ "${HOMEBREW_PREFIX:-}" == "$prefix" ]] || fail "HOMEBREW_PREFIX was not exported"
[[ "$(command -v brew)" == "$prefix/bin/brew" ]] || fail "brew was not added to PATH"
case ":$PATH:" in
    *":$prefix/opt/make/libexec/gnubin:"*) ;;
    *) fail "Homebrew make gnubin was not added to the current PATH" ;;
esac

# Homebrew's documented idempotent state is successful shellenv with empty
# stdout when bin/sbin are already first. It must be accepted only because the
# selected brew is already the command this shell resolves.
cat > "$prefix/bin/brew" <<EOF
#!/usr/bin/env bash
if [[ "\${1:-}" == "shellenv" ]]; then
    exit 0
elif [[ "\${1:-}" == "--prefix" && "\${2:-}" == "make" ]]; then
    printf '%s\n' "$prefix/opt/make"
elif [[ "\${1:-}" == "--prefix" ]]; then
    printf '%s\n' "$prefix"
elif [[ "\${1:-}" == "--repository" ]]; then
    printf '%s\n' "$prefix"
fi
EOF
chmod +x "$prefix/bin/brew"
PATH="$prefix/bin:$prefix/sbin:/usr/bin:/bin"
enable_homebrew_for_current_shell
[[ "$(command -v brew)" == "$prefix/bin/brew" ]] \
    || fail "valid empty shellenv did not preserve the selected brew"

# Restore an emitting shellenv for the fresh-shell persistence assertions.
cat > "$prefix/bin/brew" <<EOF
#!/usr/bin/env bash
if [[ "\${1:-}" == "shellenv" ]]; then
    printf 'export HOMEBREW_PREFIX="%s"\n' "$prefix"
    printf 'export PATH="%s/bin:\$PATH"\n' "$prefix"
elif [[ "\${1:-}" == "--prefix" && "\${2:-}" == "make" ]]; then
    printf '%s\n' "$prefix/opt/make"
elif [[ "\${1:-}" == "--prefix" ]]; then
    printf '%s\n' "$prefix"
elif [[ "\${1:-}" == "--repository" ]]; then
    printf '%s\n' "$prefix"
fi
EOF
chmod +x "$prefix/bin/brew"

cat > "$HOME/.bashrc" <<EOF
# user content before managed block
# >>> dotfiles: Homebrew shellenv >>>
if [ -x "$prefix/bin/brew" ]; then
    eval "\$($prefix/bin/brew shellenv)"
fi
# <<< dotfiles: Homebrew shellenv <<<
# user content after managed block
EOF

persist_homebrew_shellenv
persist_homebrew_shellenv

for rc in "$HOME/.zshrc.local" "$HOME/.bashrc"; do
    [[ -f "$rc" ]] || fail "$rc was not written"
    count="$(grep -cF '# >>> dotfiles: Homebrew shellenv >>>' "$rc")"
    [[ "$count" == "1" ]] || fail "$rc marker count is $count"
done
grep -F 'dotfiles_make_gnubin' "$HOME/.bashrc" >/dev/null \
    || fail "legacy managed bashrc block was not upgraded with Homebrew make gnubin"
grep -F '# user content before managed block' "$HOME/.bashrc" >/dev/null \
    || fail "legacy managed bashrc replacement dropped content before the block"
grep -F '# user content after managed block' "$HOME/.bashrc" >/dev/null \
    || fail "legacy managed bashrc replacement dropped content after the block"

# Single quotes are intentional: $HOME / $HOMEBREW_PREFIX must expand inside the
# inner `bash -c`, not in this outer shell.
# shellcheck disable=SC2016
env -i HOME="$HOME" PATH="/usr/bin:/bin" bash -c 'source "$HOME/.bashrc"; [[ "$HOMEBREW_PREFIX" == "$HOME/.linuxbrew" ]]'
# shellcheck disable=SC2016
env -i HOME="$HOME" PATH="/usr/bin:/bin" bash -c 'source "$HOME/.bashrc"; [[ ":$PATH:" == *":$HOME/.linuxbrew/opt/make/libexec/gnubin:"* ]]'

# nix-darwin intentionally exposes a /run/current-system/sw wrapper whose
# shellenv activates the architecture-native Homebrew entrypoint. Different
# executable paths are valid only when prefix + repository prove one install.
wrapper="$WORK/wrapper/bin/brew"
actual="$WORK/actual/bin/brew"
mkdir -p "$(dirname "$wrapper")" "$(dirname "$actual")"
cat > "$wrapper" <<EOF
#!/usr/bin/env bash
case "\${1:-}" in
    shellenv) printf 'export PATH="%s/bin:\$PATH"\n' "$(dirname "$(dirname "$actual")")" ;;
    --prefix|--repository) printf '%s\n' "$prefix" ;;
esac
EOF
cat > "$actual" <<EOF
#!/usr/bin/env bash
case "\${1:-}" in
    --prefix|--repository) printf '%s\n' "$prefix" ;;
esac
EOF
chmod +x "$wrapper" "$actual"
homebrew_bin() { printf '%s\n' "$wrapper"; }
PATH=/usr/bin:/bin
enable_homebrew_for_current_shell
[[ "$(command -v brew)" == "$actual" ]] \
    || fail "same-installation Homebrew wrapper did not activate its native entrypoint"
homebrew_bin() { printf '%s\n' "$prefix/bin/brew"; }

cat > "$prefix/bin/brew" <<'EOF'
#!/usr/bin/env bash
if [[ "${1:-}" == shellenv ]]; then
    echo 'export PATH="/untrusted-partial-shellenv:$PATH"'
    exit 9
elif [[ "${1:-}" == --prefix || "${1:-}" == --repository ]]; then
    printf '%s\n' '/untrusted-homebrew'
fi
EOF
chmod +x "$prefix/bin/brew"
PATH=/usr/bin:/bin
if enable_homebrew_for_current_shell >/dev/null 2>&1; then
    fail "failed brew shellenv was accepted"
fi
[[ ":$PATH:" != *":/untrusted-partial-shellenv:"* ]] \
    || fail "partial failed shellenv output was evaluated"

cat > "$prefix/bin/brew" <<'EOF'
#!/usr/bin/env bash
if [[ "${1:-}" == shellenv ]]; then
    printf '%s\n' 'export PATH="/partial-eval:$PATH"'
    printf '%s\n' 'export HOMEBREW_PREFIX="/partial-prefix"'
    printf '%s\n' 'false'
elif [[ "${1:-}" == --prefix || "${1:-}" == --repository ]]; then
    printf '%s\n' '/partial-prefix'
fi
EOF
chmod +x "$prefix/bin/brew"
PATH=/usr/bin:/bin
unset HOMEBREW_PREFIX
if enable_homebrew_for_current_shell >/dev/null 2>"$WORK/eval.err"; then
    fail "partially failing shellenv evaluation was accepted"
fi
[[ "$PATH" == /usr/bin:/bin ]] || fail "failed shellenv evaluation did not restore PATH"
[[ -z "${HOMEBREW_PREFIX+x}" ]] || fail "failed shellenv evaluation did not restore an unset Homebrew variable"
grep -F 'prior environment was restored' "$WORK/eval.err" >/dev/null \
    || fail "failed shellenv evaluation omitted recovery evidence"

cat > "$prefix/bin/brew" <<'EOF'
#!/usr/bin/env bash
if [[ "${1:-}" == shellenv ]]; then
    exit 0
elif [[ "${1:-}" == --prefix || "${1:-}" == --repository ]]; then
    printf '%s\n' '/selected-but-inactive'
fi
EOF
chmod +x "$prefix/bin/brew"
PATH=/usr/bin:/bin
if enable_homebrew_for_current_shell >/dev/null 2>&1; then
    fail "empty brew shellenv was accepted without an active selected brew"
fi

# macOS has no alternate package manager. A declined or failed bootstrap is an
# explicit precondition failure that reaches the consolidated summary instead
# of degrading to an unexplained `unknown` manager.
(
    PM=brew_missing
    DRY_RUN=0
    INSTALL_FAILURES_COUNT=0
    INSTALL_FAILURES_DETAIL=""
    maybe_install_brew() { return 1; }

    if bootstrap_package_manager; then
        fail "failed required macOS Homebrew bootstrap was accepted"
    fi
    record_install_failure "Homebrew bootstrap/activation" brew shellenv 1
    set +e
    summary="$(exit_if_install_failures 2>&1)"
    summary_rc=$?
    set -e
    [[ "$summary_rc" -ne 0 ]] || fail "failed Homebrew precondition summary exited zero"
    [[ "$summary" == *"Homebrew bootstrap/activation via brew (shellenv)"* ]] \
        || fail "failed Homebrew precondition was absent from the consolidated summary"
)

echo "OK"
