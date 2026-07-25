#!/usr/bin/env bash
# Regressions for the POSIX npm contract:
# - ensure_npm installs npm on apt (Debian/Ubuntu split it from nodejs);
# - configure_npm_user_prefix keeps mutable global packages out of immutable
#   runtime prefixes such as /nix/store without replacing private npm config.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
INSTALL_DEPS_SOURCE_ONLY=1 source "$REPO_ROOT/install-deps.sh"

fail() { echo "FAIL: $*" >&2; exit 1; }

TMP_ROOT="$(mktemp -d)"
trap 'rm -rf "$TMP_ROOT"' EXIT
INSTALL_LOG="$TMP_ROOT/install.log"
unset NPM_CONFIG_PREFIX npm_config_prefix NPM_CONFIG_USERCONFIG npm_config_userconfig
NPM_BIN="$(builtin command -v npm)" || fail "real npm command is required"

# node is always present (its package installed); npm presence is controlled by
# NPM_PRESENT so we can simulate a Debian nodejs without npm.
command() {
    if [[ "${1:-}" == "-v" ]]; then
        case "${2:-}" in
            node) return 0 ;;
            npm) [[ "${NPM_PRESENT:-1}" -eq 1 ]] && return 0 || return 1 ;;
            *) builtin command "$@" ;;
        esac
    fi
    builtin command "$@"
}
native_linux_pm() { printf '%s\n' "${NATIVE_PM:-apt}"; }
native_linux_pm_install() { printf '%s\n' "$*" >> "$INSTALL_LOG"; return 0; }

PM=apt
YES_ALL=1
DRY_RUN=0

# Case 1: npm already present -> no install attempt.
: > "$INSTALL_LOG"
NPM_PRESENT=1 ensure_npm >/dev/null
[[ -s "$INSTALL_LOG" ]] && fail "installed npm even though already present"

# Case 2: npm missing on apt -> installs the npm package.
: > "$INSTALL_LOG"
NPM_PRESENT=0 NATIVE_PM=apt ensure_npm >/dev/null
grep -q "apt npm" "$INSTALL_LOG" \
    || fail "did not install npm on apt (log: $(cat "$INSTALL_LOG"))"

# Case 3: dry-run on apt -> previews, mutates nothing.
: > "$INSTALL_LOG"
out="$(NPM_PRESENT=0 NATIVE_PM=apt DRY_RUN=1 ensure_npm)"
[[ -s "$INSTALL_LOG" ]] && fail "dry-run still installed npm"
[[ "$out" == *"would:"* ]] || fail "dry-run did not print a would line"

# Case 4: a real npm user config is amended, not replaced. This starts from the
# same effective failure shape as Nix: npm's default global prefix follows its
# runtime until the user-level prefix is persisted.
npm_home="$TMP_ROOT/npm-home"
mkdir -p "$npm_home"
cat > "$npm_home/.npmrc" <<'EOF'
registry=https://registry.npmjs.org/
//registry.npmjs.org/:_authToken=fake-token-for-regression-test
EOF
HOME="$npm_home" NPM_PRESENT=1 DRY_RUN=0 configure_npm_user_prefix >/dev/null
[[ "$(HOME="$npm_home" "$NPM_BIN" prefix --global)" == "$npm_home/.local" ]] \
    || fail "effective npm global prefix was not repaired"
grep -Fxq "registry=https://registry.npmjs.org/" "$npm_home/.npmrc" \
    || fail "npm registry config was replaced"
grep -Fxq "//registry.npmjs.org/:_authToken=fake-token-for-regression-test" "$npm_home/.npmrc" \
    || fail "npm auth config was replaced"
grep -Fxq "prefix=$npm_home/.local" "$npm_home/.npmrc" \
    || fail "npm user prefix was not persisted"

# Case 5: an ordinary global install, including a package self-updater's generic
# `npm install --global`, now publishes beneath ~/.local without a special flag.
probe_package="$TMP_ROOT/npm-prefix-probe"
mkdir -p "$probe_package"
cat > "$probe_package/package.json" <<'EOF'
{
  "name": "npm-prefix-regression-probe",
  "version": "1.0.0",
  "bin": {
    "npm-prefix-regression-probe": "cli.js"
  }
}
EOF
cat > "$probe_package/cli.js" <<'EOF'
#!/usr/bin/env node
process.stdout.write("ok\n");
EOF
chmod +x "$probe_package/cli.js"
HOME="$npm_home" "$NPM_BIN" install --global --ignore-scripts "$probe_package" >/dev/null
[[ -x "$npm_home/.local/bin/npm-prefix-regression-probe" ]] \
    || fail "generic npm global install did not publish beneath ~/.local"

# Case 6: reconciliation is byte-idempotent once the desired config is present.
before="$(cksum "$npm_home/.npmrc")"
HOME="$npm_home" NPM_PRESENT=1 DRY_RUN=0 configure_npm_user_prefix >/dev/null
after="$(cksum "$npm_home/.npmrc")"
[[ "$after" == "$before" ]] || fail "npm prefix reconciliation was not idempotent"

# Case 7: dry-run previews both actions and leaves the user config untouched.
before="$(cksum "$npm_home/.npmrc")"
out="$(HOME="$npm_home" NPM_PRESENT=1 DRY_RUN=1 configure_npm_user_prefix)"
after="$(cksum "$npm_home/.npmrc")"
[[ "$after" == "$before" ]] || fail "dry-run changed npm user config"
[[ "$out" == *"npm config set prefix"* ]] || fail "dry-run omitted config preview"
[[ "$out" == *"verify npm global prefix"* ]] || fail "dry-run omitted verification preview"

# Case 8: a failed config write is a hard failure for the reconciliation step.
npm() {
    [[ "${1:-} ${2:-}" == "config set" ]] && return 73
    fail "unexpected npm invocation in write-failure case: $*"
}
if HOME="$npm_home" DRY_RUN=0 configure_npm_user_prefix >/dev/null 2>&1; then
    fail "accepted a failed npm user-config write"
fi
unset -f npm

# Case 9: an environment or config override that defeats the persisted value is
# detected by the independent effective-prefix readback.
npm() {
    case "${1:-} ${2:-}" in
        "config set") return 0 ;;
        "prefix --global") printf '%s\n' "/nix/store/read-only-node"; return 0 ;;
        *) fail "unexpected npm invocation in readback case: $*" ;;
    esac
}
if HOME="$npm_home" DRY_RUN=0 configure_npm_user_prefix >/dev/null 2>&1; then
    fail "accepted an effective npm prefix outside ~/.local"
fi
unset -f npm

# Case 10: the real language-tooling flow reconciles npm after availability and
# before any repo-pinned npm global package is installed.
ensure_line="$(grep -n '^run_install_step npm .* ensure_npm$' "$REPO_ROOT/install-deps.sh" | cut -d: -f1)"
prefix_line="$(grep -n '^run_install_step "npm global prefix" .* configure_npm_user_prefix$' "$REPO_ROOT/install-deps.sh" | cut -d: -f1)"
pi_line="$(grep -n '^run_install_step pi .* install_pi_cli$' "$REPO_ROOT/install-deps.sh" | cut -d: -f1)"
[[ -n "$ensure_line" && -n "$prefix_line" && -n "$pi_line" ]] \
    || fail "npm prefix reconciliation is not wired into language tooling"
(( ensure_line < prefix_line && prefix_line < pi_line )) \
    || fail "npm prefix reconciliation runs in the wrong order"

echo "OK"
