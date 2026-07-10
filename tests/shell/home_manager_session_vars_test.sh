#!/usr/bin/env bash
# A fresh Linux/WSL zsh must source Home Manager's canonical session variables
# without CI-side PATH injection, while no-Nix hosts and repeated sourcing stay
# harmless and idempotent.
set -euo pipefail
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
command -v zsh >/dev/null 2>&1 || { echo "SKIP: zsh not installed"; exit 0; }
WORK="$REPO_ROOT/tests/.cache/home-manager-session-vars-test"
rm -rf "$WORK"
HOME_DIR="$WORK/Custom Home"
STATE_DIR="$WORK/Custom State"
mkdir -p "$HOME_DIR" "$STATE_DIR/nix/profiles/profile/etc/profile.d" \
    "$HOME_DIR/.nix-profile/etc/profile.d" "$HOME_DIR/nix-bin" "$WORK/zdot"
trap 'rm -rf "$WORK"' EXIT
ln -s "$REPO_ROOT/shells/zshrc" "$WORK/zdot/.zshrc"

cat > "$STATE_DIR/nix/profiles/profile/etc/profile.d/hm-session-vars.sh" <<EOF
export __HM_SESS_VARS_SOURCED=1
export DOTFILES_HM_SOURCE=canonical
export PATH="$HOME_DIR/nix-bin:\$PATH"
DOTFILES_HM_SOURCE_COUNT=\$((\${DOTFILES_HM_SOURCE_COUNT:-0} + 1))
export DOTFILES_HM_SOURCE_COUNT
EOF
cat > "$HOME_DIR/.nix-profile/etc/profile.d/hm-session-vars.sh" <<'EOF'
export DOTFILES_HM_SOURCE=legacy
EOF
cat > "$HOME_DIR/nix-bin/hm-owned-tool" <<'EOF'
#!/usr/bin/env sh
exit 0
EOF
chmod +x "$HOME_DIR/nix-bin/hm-owned-tool"

run_zsh() {
    env -i HOME="$HOME_DIR" XDG_STATE_HOME="$STATE_DIR" ZDOTDIR="$WORK/zdot" \
        PATH=/usr/bin:/bin TERM=dumb zsh "$@"
}

for mode in '-i' '-l -i'; do
    # shellcheck disable=SC2086
    output="$(run_zsh $mode -c "printf '%s|%s|%s\\n' \"\$DOTFILES_HM_SOURCE\" \"\$DOTFILES_HM_SOURCE_COUNT\" \"\$(command -v hm-owned-tool)\"" 2>/dev/null)"
    [[ "$output" == "canonical|1|$HOME_DIR/nix-bin/hm-owned-tool" ]] || {
        echo "FAIL: fresh zsh mode [$mode] did not consume canonical HM state: $output"
        exit 1
    }
done

repeat="$(run_zsh -f -c "source \"\$1\" >/dev/null 2>&1; source \"\$1\" >/dev/null 2>&1; printf '%s|%s' \"\$DOTFILES_HM_SOURCE\" \"\$DOTFILES_HM_SOURCE_COUNT\"" zsh "$REPO_ROOT/shells/zshrc")"
[[ "$repeat" == 'canonical|1' ]] || { echo "FAIL: repeated zshrc sourcing re-sourced HM state: $repeat"; exit 1; }

rm -f "$STATE_DIR/nix/profiles/profile/etc/profile.d/hm-session-vars.sh"
legacy="$(run_zsh -f -c "source \"\$1\" >/dev/null 2>&1; printf %s \"\$DOTFILES_HM_SOURCE\"" zsh "$REPO_ROOT/shells/zshrc")"
[[ "$legacy" == legacy ]] || { echo "FAIL: legacy HM session-vars fallback was not sourced"; exit 1; }

rm -f "$HOME_DIR/.nix-profile/etc/profile.d/hm-session-vars.sh"
no_nix="$(run_zsh -f -c "source \"\$1\" >/dev/null 2>&1 && ! command -v hm-owned-tool >/dev/null 2>&1 && printf safe" zsh "$REPO_ROOT/shells/zshrc")"
[[ "$no_nix" == safe ]] || { echo "FAIL: missing HM profiles were not harmless"; exit 1; }

# The pinned Home Manager source documents this third canonical location for
# profiles integrated with a system configuration. Use the effective account
# identity, never an unvalidated ambient USER path component.
grep -F "/etc/profiles/per-user/\$_dotfiles_hm_user/etc/profile.d/hm-session-vars.sh" \
    "$REPO_ROOT/shells/zshrc" >/dev/null \
    || { echo "FAIL: system-integrated Home Manager profile fallback is missing"; exit 1; }
grep -F "_dotfiles_hm_user=\"\$(id -un 2>/dev/null)\"" "$REPO_ROOT/shells/zshrc" >/dev/null \
    || { echo "FAIL: Home Manager system profile is not keyed by effective account identity"; exit 1; }

# The hosted native-Linux proof must execute the login zsh that setup actually
# selected from the account record. Ubuntu starts without /usr/bin/zsh and the
# public installer legitimately selects Linuxbrew zsh, so hardcoding a path
# would fail before testing Home Manager state.
workflow="$REPO_ROOT/.github/workflows/e2e-install.yml"
grep -F "fresh_zsh_user=\"\$(id -un)\"" "$workflow" >/dev/null \
    || { echo "FAIL: native-Linux proof does not resolve the effective account"; exit 1; }
grep -F "getent passwd \"\$fresh_zsh_user\"" "$workflow" >/dev/null \
    || { echo "FAIL: native-Linux proof does not resolve the account-record shell"; exit 1; }
grep -F "\"\$fresh_zsh_login_shell\" -l -i -c" "$workflow" >/dev/null \
    || { echo "FAIL: native-Linux proof does not execute the resolved login zsh"; exit 1; }
if grep -F '/usr/bin/zsh -l -i' "$workflow" >/dev/null; then
    echo "FAIL: native-Linux proof hardcodes a zsh path instead of the account record"
    exit 1
fi
grep -F "cat \"\$fresh_zsh_stderr\" >&2" "$workflow" >/dev/null \
    || { echo "FAIL: native-Linux proof discards login-shell diagnostics"; exit 1; }

cmp -s "$REPO_ROOT/shells/zshrc" "$REPO_ROOT/home/dot_zshrc" || {
    echo "FAIL: chezmoi zshrc twin drifted"
    exit 1
}
echo "OK"
