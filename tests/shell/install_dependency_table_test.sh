#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
INSTALL_DEPS_SOURCE_ONLY=1 source "$REPO_ROOT/install-deps.sh"

fail() {
    echo "FAIL: $*" >&2
    exit 1
}

TMP_ROOT="$(mktemp -d)"
trap 'rm -rf "$TMP_ROOT"' EXIT
mkdir -p "$TMP_ROOT/fakebin"

cat > "$TMP_ROOT/fakebin/present-tool" <<'EOF'
#!/usr/bin/env bash
if [[ "${1:-}" == "--version" ]]; then
    printf '%s\n' "present-tool 1.2.3"
    exit 0
fi
exit 0
EOF
chmod +x "$TMP_ROOT/fakebin/present-tool"

PATH="$TMP_ROOT/fakebin:/usr/bin:/bin"
PM=brew_missing
INSTALL_DEPS_SCAN_ITEMS=$'present-tool|command|present-tool\nmissing-tool|command|missing-tool'

pm_install() {
    fail "table scan attempted a package-manager install"
}

maybe_install_brew() {
    fail "table scan attempted a Homebrew bootstrap"
}

install() {
    fail "table scan attempted an install function"
}

output="$(print_install_dependency_table)"

printf '%s\n' "$output" | grep -Eq '^brew[[:space:]]+missing[[:space:]]+-[[:space:]]+install[[:space:]]*$' \
    || fail "pre-bootstrap package manager row missing or wrong"
printf '%s\n' "$output" | grep -Eq '^present-tool[[:space:]]+present[[:space:]]+present-tool 1\.2\.3[[:space:]]+skip[[:space:]]*$' \
    || fail "present tool row missing or wrong"
printf '%s\n' "$output" | grep -Eq '^missing-tool[[:space:]]+missing[[:space:]]+-[[:space:]]+install[[:space:]]*$' \
    || fail "missing tool row missing or wrong"
printf '%s\n' "$output" | grep -F '1 present, 2 missing' >/dev/null \
    || fail "summary count missing or wrong"

# The real dependency-table scan must pass the complete immutable identity to
# the hardened zsh predicate. The old three-argument call failed under `set -u`
# before any install work on a fresh Linux runner.
(
    checked=0
    zsh_plugin_ok() {
        [[ "$#" -eq 4 ]] || fail "zsh plugin scan passed $# identity fields; expected 4"
        case "${1##*/}" in
            fzf-tab)
                [[ "$2" == "https://github.com/Aloxaf/fzf-tab.git" && "$3" == "$FZF_TAB_COMMIT" && "$4" == "fzf-tab.plugin.zsh" ]] \
                    || fail "fzf-tab scan identity mismatch"
                ;;
            zsh-autosuggestions)
                [[ "$2" == "https://github.com/zsh-users/zsh-autosuggestions.git" && "$3" == "$ZSH_AUTOSUGGESTIONS_COMMIT" && "$4" == "zsh-autosuggestions.zsh" ]] \
                    || fail "zsh-autosuggestions scan identity mismatch"
                ;;
            *) fail "unexpected zsh plugin scan target: $1" ;;
        esac
        checked=$((checked + 1))
        return 0
    }

    install_scan_present "zsh plugins" zsh-plugins \
        || fail "complete zsh plugin identities were not accepted"
    [[ "$checked" -eq 2 ]] || fail "dependency scan did not verify both zsh plugins"
)

echo "OK"
