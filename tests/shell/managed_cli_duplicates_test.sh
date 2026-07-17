#!/usr/bin/env bash
# shellcheck disable=SC1091,SC2329
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
INSTALL_DEPS_SOURCE_ONLY=1 source "$REPO_ROOT/install-deps.sh"
PM=unknown

fail() { echo "FAIL: $*" >&2; exit 1; }

TMP_ROOT="$REPO_ROOT/tests/.cache/managed-cli-duplicates-test"
rm -rf "$TMP_ROOT"
mkdir -p "$TMP_ROOT"
trap 'rm -rf "$TMP_ROOT"' EXIT

selected_bin="$TMP_ROOT/selected/bin"
duplicate_bin="$TMP_ROOT/duplicate/bin"
alias_bin="$TMP_ROOT/alias/bin"
mkdir -p "$selected_bin" "$duplicate_bin" "$alias_bin"

cat > "$selected_bin/rg" <<'EOF'
#!/usr/bin/env bash
printf '%s\n' selected-executed >> "${MANAGED_CLI_EXEC_LOG:?}"
EOF
cat > "$duplicate_bin/rg" <<'EOF'
#!/usr/bin/env bash
printf '%s\n' duplicate-executed >> "${MANAGED_CLI_EXEC_LOG:?}"
EOF
chmod +x "$selected_bin/rg" "$duplicate_bin/rg"
ln -s "$selected_bin/rg" "$alias_bin/rg"

# The selected command is the runtime authority. A physically distinct command
# later on PATH is reported without executing either command; a second symlink
# to the selected executable is not a second installation.
(
    PATH="$selected_bin:$alias_bin:$duplicate_bin:/usr/bin:/bin"; export PATH
    MANAGED_CLI_EXEC_LOG="$TMP_ROOT/executed.log"; export MANAGED_CLI_EXEC_LOG
    INSTALL_DEPS_AUDIT_ITEMS='rg|rg'; export INSTALL_DEPS_AUDIT_ITEMS

    out="$(audit_managed_cli_installations 2>&1)"
    [[ "$out" == *"multiple managed rg commands are on PATH"* ]] \
        || fail "generic duplicate warning missing: $out"
    [[ "$out" == *"selected: $selected_bin/rg"* ]] \
        || fail "selected command missing: $out"
    [[ "$out" == *"duplicate: $duplicate_bin/rg"* ]] \
        || fail "distinct duplicate missing: $out"
    [[ "$out" != *"$alias_bin/rg"* ]] \
        || fail "same physical executable was reported twice: $out"
    [[ ! -e "$MANAGED_CLI_EXEC_LOG" ]] \
        || fail "the audit executed a managed command"
)

# Base-OS command directories are fallbacks, not removable competing
# installations. They must not produce a permanent warning behind a managed
# command that intentionally wins PATH.
managed_cli_system_fallback_path /usr/bin/rg \
    || fail "/usr/bin was not recognized as a system fallback"
managed_cli_system_fallback_path /bin/sh \
    || fail "/bin was not recognized as a system fallback"
if managed_cli_system_fallback_path "$duplicate_bin/rg"; then
    fail "a user-managed path was mistaken for a system fallback"
fi

# The default audit is derived from the install pre-flight inventory, so a new
# managed command cannot silently miss duplicate detection. Explicit commands
# installed outside that table remain listed beside it.
audit_items="$(managed_cli_audit_items)"
while IFS='|' read -r tool kind _version_bin; do
    [[ "$kind" == "command" ]] || continue
    printf '%s\n' "$audit_items" | awk -F'|' -v expected="$tool" '$1 == expected { found = 1 } END { exit !found }' \
        || fail "$tool is present in the install inventory but absent from the duplicate audit"
done < <(install_dependency_scan_items)
for tool in zoxide npm latex2text wezterm aerospace herdr devilspie2; do
    printf '%s\n' "$audit_items" | awk -F'|' -v expected="$tool" '$1 == expected { found = 1 } END { exit !found }' \
        || fail "$tool is absent from the explicit duplicate-audit inventory"
done

# A global npm duplicate gets an exact same-user cleanup command only after the
# sibling npm proves both its prefix and the package receipt. The competing Pi
# binary itself must remain unexecuted.
(
    home="$TMP_ROOT/npm-home"
    prefix="$TMP_ROOT/npm-prefix"
    mkdir -p "$home/.local/bin" "$prefix/bin" "$prefix/lib/node_modules/@earendil-works/pi-coding-agent"
    cat > "$home/.local/bin/pi" <<'EOF'
#!/usr/bin/env bash
exit 99
EOF
    cat > "$prefix/bin/npm" <<EOF
#!/usr/bin/env bash
case "\${1:-}" in
    prefix) printf '%s\n' '$prefix' ;;
    list) [[ "\$*" == 'list --global --prefix $prefix --depth=0 @earendil-works/pi-coding-agent' ]] ;;
    *) exit 91 ;;
esac
EOF
    chmod +x "$home/.local/bin/pi" "$prefix/bin/npm"
    cat > "$prefix/lib/node_modules/@earendil-works/pi-coding-agent/cli.js" <<'EOF'
#!/usr/bin/env bash
printf '%s\n' duplicate-pi-executed >> "${MANAGED_CLI_EXEC_LOG:?}"
EOF
    chmod +x "$prefix/lib/node_modules/@earendil-works/pi-coding-agent/cli.js"
    ln -s "$prefix/lib/node_modules/@earendil-works/pi-coding-agent/cli.js" \
        "$prefix/lib/node_modules/@earendil-works/pi-coding-agent/pi-target"
    ln -s "$prefix/lib/node_modules/@earendil-works/pi-coding-agent/pi-target" "$prefix/bin/pi"

    HOME="$home"; export HOME
    PATH="$home/.local/bin:$prefix/bin:/usr/bin:/bin"; export PATH
    MANAGED_CLI_EXEC_LOG="$TMP_ROOT/npm-executed.log"; export MANAGED_CLI_EXEC_LOG

    out="$(audit_managed_cli_command pi pi "$home/.local/bin/pi" 2>&1)"
    [[ "$out" == *"owner=npm package=@earendil-works/pi-coding-agent"* ]] \
        || fail "npm ownership proof missing: $out"
    [[ "$out" == *"$prefix/bin/npm uninstall --global --prefix $prefix @earendil-works/pi-coding-agent"* ]] \
        || fail "proven npm cleanup missing: $out"
    [[ "$out" != *"sudo npm"* ]] || fail "npm cleanup incorrectly recommends sudo"
    [[ ! -e "$MANAGED_CLI_EXEC_LOG" ]] || fail "duplicate Pi was executed"
)

echo "OK"
