#!/usr/bin/env bash
# shellcheck disable=SC1091,SC2030,SC2031,SC2329
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
INSTALL_DEPS_SOURCE_ONLY=1 source "$REPO_ROOT/install-deps.sh"
REAL_NODE="$(command -v node)"

fail() { echo "FAIL: $*" >&2; exit 1; }

TMP_ROOT="$REPO_ROOT/tests/.cache/pi-cli-test"
rm -rf "$TMP_ROOT"
mkdir -p "$TMP_ROOT"
trap 'rm -rf "$TMP_ROOT"' EXIT

payload_sri() {
    node -e 'const h=require("crypto").createHash("sha512"); process.stdin.on("data", d => h.update(d)); process.stdin.on("end", () => process.stdout.write("sha512-" + h.digest("base64")))'
}

VERIFIED_PAYLOAD='verified pi package tarball bytes'
TEST_SRI="$(printf '%s' "$VERIFIED_PAYLOAD" | payload_sri)"

write_npm_stub() {
    local dir="$1"
    cat > "$dir/npm" <<'EOF'
#!/usr/bin/env bash
set -u
printf '%s\n' "$*" >> "${PI_TEST_LOG:?}"
case "${1:-}" in
    pack)
        [[ "${PI_TEST_MODE:-success}" != "network-fail" ]] || exit 51
        [[ "${PI_TEST_MODE:-success}" != "interrupt" ]] || exit 130
        dest=""
        shift
        while [[ "$#" -gt 0 ]]; do
            if [[ "$1" == "--pack-destination" ]]; then
                shift
                dest="$1"
                break
            fi
            shift
        done
        [[ -n "$dest" ]] || exit 52
        filename='earendil-works-pi-coding-agent-0.80.10.tgz'
        if [[ "${PI_TEST_MODE:-success}" == "partial" ]]; then
            printf '%s' 'partial tarball' > "$dest/$filename"
        else
            printf '%s' "${PI_TEST_PAYLOAD:?}" > "$dest/$filename"
        fi
        reported="${PI_TEST_REPORTED_INTEGRITY:?}"
        printf '[{"filename":"%s","integrity":"%s"}]\n' "$filename" "$reported"
        ;;
    install)
        [[ "${PI_TEST_MODE:-success}" != "install-fail" ]] || exit 61
        tarball=''
        for arg in "$@"; do
            [[ "$arg" == *.tgz ]] && tarball="$arg"
        done
        [[ -s "$tarball" ]] || exit 62
        mkdir -p "$HOME/.local/bin"
        cat > "$HOME/.local/bin/pi" <<EOS
#!/usr/bin/env bash
printf '%s\n' "${PI_TEST_INSTALLED_VERSION:-0.80.10}"
EOS
        chmod +x "$HOME/.local/bin/pi"
        ;;
    prefix)
        [[ "$#" -eq 2 && "${2:-}" == "-g" ]] || exit 71
        printf '%s\n' "${PI_TEST_NPM_PREFIX:?}"
        ;;
    list)
        [[ "$*" == "list --global --prefix ${PI_TEST_NPM_PREFIX:?} --depth=0 @earendil-works/pi-coding-agent" ]] || exit 72
        [[ "${PI_TEST_NPM_OWNS_DUPLICATE:-0}" == "1" ]] || exit 1
        ;;
    *)
        printf 'unexpected npm args: %s\n' "$*" >&2
        exit 2
        ;;
esac
EOF
    chmod +x "$dir/npm"
    ln -s "$REAL_NODE" "$dir/node"
}

assert_no_pi_temp_dirs() {
    local root="$1"
    if find "$root" -maxdepth 1 -type d -name 'dotfiles-pi.*' -print | grep -q .; then
        fail "Pi temporary directories leaked under $root"
    fi
}

run_case() {
    local name="$1" mode="$2" reported="$3" installed_version="${4:-0.80.10}"
    local home="$TMP_ROOT/$name-home" bin="$TMP_ROOT/$name-bin" tmp="$TMP_ROOT/$name-tmp"
    mkdir -p "$home" "$bin" "$tmp"
    write_npm_stub "$bin"
    (
        HOME="$home"; export HOME
        TMPDIR="$tmp"; export TMPDIR
        PATH="$bin:/usr/bin:/bin"; export PATH
        PI_TEST_LOG="$TMP_ROOT/$name.log"; export PI_TEST_LOG
        PI_TEST_MODE="$mode"; export PI_TEST_MODE
        PI_TEST_PAYLOAD="$VERIFIED_PAYLOAD"; export PI_TEST_PAYLOAD
        PI_TEST_REPORTED_INTEGRITY="$reported"; export PI_TEST_REPORTED_INTEGRITY
        PI_TEST_INSTALLED_VERSION="$installed_version"; export PI_TEST_INSTALLED_VERSION
        PI_CLI_INTEGRITY="$TEST_SRI"
        YES_ALL=1
        DRY_RUN=0
        pi_cli_node_ready() { return 0; }
        pi_cli_version() {
            [[ -x "$HOME/.local/bin/pi" ]] || return 1
            "$HOME/.local/bin/pi" --version 2>/dev/null | awk 'NF { print $1; exit }'
        }
        install_pi_cli
    )
}

# An already-current canonical installation is idempotent and never calls npm.
(
    HOME="$TMP_ROOT/current-home"; export HOME
    bin="$TMP_ROOT/current-bin"; mkdir -p "$bin" "$HOME/.local/bin"
    PATH="$bin:$HOME/.local/bin:/usr/bin:/bin"; export PATH
    cat > "$HOME/.local/bin/pi" <<'EOF'
#!/usr/bin/env bash
printf '%s\n' '0.80.10'
EOF
    chmod +x "$HOME/.local/bin/pi"
    cat > "$bin/npm" <<'EOF'
#!/usr/bin/env bash
exit 99
EOF
    chmod +x "$bin/npm"
    YES_ALL=1; DRY_RUN=0
    out="$(install_pi_cli 2>&1)"
    [[ "$out" == *"already installed (0.80.10)"* ]] || fail "current Pi must be idempotent; got: $out"
)

# A same-version foreign/global Pi is not the repo-owned installation. Setup
# must publish and prove its canonical ~/.local copy rather than accepting a
# coincidentally matching command elsewhere on PATH.
(
    HOME="$TMP_ROOT/foreign-current-home"; export HOME
    prefix="$TMP_ROOT/foreign-current-prefix"; bin="$prefix/bin"; tmp="$TMP_ROOT/foreign-current-tmp"
    mkdir -p "$bin" "$tmp"
    write_npm_stub "$bin"
    cat > "$bin/pi" <<'EOF'
#!/usr/bin/env bash
printf '%s\n' '0.80.10'
EOF
    chmod +x "$bin/pi"
    TMPDIR="$tmp"; export TMPDIR
    PATH="$bin:/usr/bin:/bin"; export PATH
    PI_TEST_LOG="$TMP_ROOT/foreign-current.log"; export PI_TEST_LOG
    PI_TEST_MODE=success; export PI_TEST_MODE
    PI_TEST_PAYLOAD="$VERIFIED_PAYLOAD"; export PI_TEST_PAYLOAD
    PI_TEST_REPORTED_INTEGRITY="$TEST_SRI"; export PI_TEST_REPORTED_INTEGRITY
    PI_TEST_INSTALLED_VERSION=0.80.10; export PI_TEST_INSTALLED_VERSION
    PI_TEST_NPM_PREFIX="$prefix"; export PI_TEST_NPM_PREFIX
    PI_TEST_NPM_OWNS_DUPLICATE=1; export PI_TEST_NPM_OWNS_DUPLICATE
    PI_CLI_INTEGRITY="$TEST_SRI"
    YES_ALL=1
    DRY_RUN=0
    pi_cli_node_ready() { return 0; }

    install_pi_cli > "$tmp/install.out" 2>&1
    foreign_out="$(cat "$tmp/install.out")"
    [[ "$foreign_out" == *"installed"*"0.80.10"* ]] \
        || fail "same-version foreign Pi was accepted as canonical: $foreign_out"
    [[ -x "$HOME/.local/bin/pi" ]] || fail "canonical Pi was not published"
    grep -q '^install ' "$PI_TEST_LOG" || fail "canonical Pi install did not run"
)

# Dry-run describes pack, byte verification, and local-tarball install.
(
    HOME="$TMP_ROOT/dry-home"; export HOME
    bin="$TMP_ROOT/dry-bin"; mkdir -p "$bin" "$HOME"
    PATH="$bin:/usr/bin:/bin"; export PATH
    write_npm_stub "$bin"
    PI_TEST_LOG="$TMP_ROOT/dry.log"; export PI_TEST_LOG
    YES_ALL=1; DRY_RUN=1
    pi_cli_node_ready() { return 0; }
    pi_cli_version() { return 1; }
    out="$(install_pi_cli 2>&1)"
    [[ "$out" == *"npm pack --ignore-scripts --json"* ]] || fail "dry-run missing npm pack; got: $out"
    [[ "$out" == *"tarball bytes"*"$PI_CLI_INTEGRITY"* ]] || fail "dry-run missing byte-bound SRI; got: $out"
    [[ "$out" == *"npm install -g --prefix"*"<verified-local-tarball>"*"<exact same-release Pi companions>"* ]] || fail "dry-run missing exact Pi install inputs; got: $out"
    [[ ! -e "$PI_TEST_LOG" ]] || fail "dry-run invoked npm"
)

# Independently exercise the SRI verifier with known bytes.
known="$TMP_ROOT/known.tgz"
printf '%s' "$VERIFIED_PAYLOAD" > "$known"
verify_pi_cli_tarball_sri "$known" "$TEST_SRI" || fail "known tarball SRI was rejected"
printf 'x' >> "$known"
if verify_pi_cli_tarball_sri "$known" "$TEST_SRI"; then
    fail "modified tarball passed the pinned SRI"
fi

# Network/interruption failures clean partial temporary state.
for mode in network-fail interrupt; do
    set +e
    run_case "$mode" "$mode" "$TEST_SRI" >/dev/null 2>&1
    rc=$?
    set -e
    [[ "$rc" -ne 0 ]] || fail "$mode unexpectedly succeeded"
    assert_no_pi_temp_dirs "$TMP_ROOT/$mode-tmp"
done

# Registry metadata must agree with the reviewed SRI before installation.
set +e
metadata_out="$(run_case metadata-mismatch success sha512-AAAAAAAA 2>&1)"
metadata_rc=$?
set -e
[[ "$metadata_rc" -ne 0 ]] || fail "metadata/tarball disagreement unexpectedly succeeded"
[[ "$metadata_out" == *"metadata integrity mismatch"* ]] || fail "metadata mismatch diagnostic missing: $metadata_out"
! grep -q '^install ' "$TMP_ROOT/metadata-mismatch.log" || fail "install ran after metadata mismatch"
assert_no_pi_temp_dirs "$TMP_ROOT/metadata-mismatch-tmp"

# A partial tarball that claims the right metadata SRI still fails byte proof.
set +e
partial_out="$(run_case partial partial "$TEST_SRI" 2>&1)"
partial_rc=$?
set -e
[[ "$partial_rc" -ne 0 ]] || fail "partial tarball unexpectedly succeeded"
[[ "$partial_out" == *"tarball bytes do not match pinned SRI"* ]] || fail "partial tarball diagnostic missing: $partial_out"
! grep -q '^install ' "$TMP_ROOT/partial.log" || fail "install ran after partial tarball"
assert_no_pi_temp_dirs "$TMP_ROOT/partial-tmp"

# A verified local coding-agent tarball plus exact same-release Pi companions are
# the only install inputs; failures still clean temporary state.
set +e
install_fail_out="$(run_case install-fail install-fail "$TEST_SRI" 2>&1)"
install_fail_rc=$?
set -e
[[ "$install_fail_rc" -ne 0 ]] || fail "npm install failure unexpectedly succeeded"
[[ "$install_fail_out" == *"verified local tarball"* ]] || fail "local-tarball install failure diagnostic missing"
assert_no_pi_temp_dirs "$TMP_ROOT/install-fail-tmp"

# Success installs from the verified temporary tarball and repeated setup reuses
# the validated installed version without another pack/download.
run_case success success "$TEST_SRI" >/dev/null
grep -E '^install -g --prefix .*/dotfiles-pi\.[^/]*/earendil-works-pi-coding-agent-0\.80\.10\.tgz @earendil-works/pi-agent-core@0\.80\.10 @earendil-works/pi-ai@0\.80\.10 @earendil-works/pi-tui@0\.80\.10$' "$TMP_ROOT/success.log" >/dev/null \
    || fail "npm install did not receive the verified tarball and exact same-release companions: $(cat "$TMP_ROOT/success.log")"
assert_no_pi_temp_dirs "$TMP_ROOT/success-tmp"

# An older globally installed Pi must not shadow the verified user-local copy.
# This reproduces a macOS field failure where ~/.local/bin was already present
# later on PATH, so setup installed 0.80.10 but verified Homebrew's stale 0.80.3.
(
    HOME="$TMP_ROOT/shadow-home"; export HOME
    prefix="$TMP_ROOT/shadow-prefix"; bin="$prefix/bin"; tmp="$TMP_ROOT/shadow-tmp"
    mkdir -p "$HOME/.local/bin" "$bin" "$tmp"
    write_npm_stub "$bin"
    cat > "$bin/pi" <<'EOF'
#!/usr/bin/env bash
printf '%s\n' executed >> "${PI_TEST_DUPLICATE_EXEC_LOG:?}"
printf '%s\n' '0.80.3'
EOF
    chmod +x "$bin/pi"
    TMPDIR="$tmp"; export TMPDIR
    PATH="$bin:$HOME/.local/bin:/usr/bin:/bin"; export PATH
    PI_TEST_LOG="$TMP_ROOT/shadow.log"; export PI_TEST_LOG
    PI_TEST_MODE=success; export PI_TEST_MODE
    PI_TEST_PAYLOAD="$VERIFIED_PAYLOAD"; export PI_TEST_PAYLOAD
    PI_TEST_REPORTED_INTEGRITY="$TEST_SRI"; export PI_TEST_REPORTED_INTEGRITY
    PI_TEST_INSTALLED_VERSION=0.80.10; export PI_TEST_INSTALLED_VERSION
    PI_TEST_NPM_PREFIX="$prefix"; export PI_TEST_NPM_PREFIX
    PI_TEST_NPM_OWNS_DUPLICATE=1; export PI_TEST_NPM_OWNS_DUPLICATE
    PI_TEST_DUPLICATE_EXEC_LOG="$TMP_ROOT/shadow-executed.log"; export PI_TEST_DUPLICATE_EXEC_LOG
    PI_CLI_INTEGRITY="$TEST_SRI"
    YES_ALL=1
    DRY_RUN=0
    pi_cli_node_ready() { return 0; }

    shadow_rc=0
    install_pi_cli > "$tmp/install.out" 2>&1 || shadow_rc=$?
    shadow_out="$(cat "$tmp/install.out")"
    [[ "$shadow_rc" -eq 0 ]] || fail "stale global Pi shadowed the verified install: $shadow_out"
    [[ "$(command -v pi)" == "$HOME/.local/bin/pi" ]] \
        || fail "user-local Pi is not first after install: $(command -v pi)"
    [[ "$(pi --version)" == "0.80.10" ]] || fail "wrong Pi wins after install"
    local_count="$(printf '%s\n' "$PATH" | tr ':' '\n' | grep -Fxc "$HOME/.local/bin")"
    [[ "$local_count" == "1" ]] || fail "$HOME/.local/bin appears $local_count times after install"
    [[ "$shadow_out" == *"WARN: multiple managed pi commands are on PATH"* ]] \
        || fail "duplicate Pi warning missing: $shadow_out"
    [[ "$shadow_out" == *"$bin/pi"* ]] || fail "duplicate Pi path missing from warning: $shadow_out"
    [[ "$shadow_out" == *"$bin/npm uninstall --global --prefix $prefix @earendil-works/pi-coding-agent"* ]] \
        || fail "proven npm cleanup command missing: $shadow_out"
    [[ "$shadow_out" != *"sudo npm uninstall"* ]] || fail "duplicate cleanup incorrectly recommends sudo"
    [[ ! -e "$PI_TEST_DUPLICATE_EXEC_LOG" ]] || fail "duplicate Pi was executed during detection"
)

# Unknown duplicate owners still get a path-specific warning, but setup does
# not invent an uninstall command or execute the foreign command to classify it.
(
    HOME="$TMP_ROOT/unknown-owner-home"; export HOME
    bin="$TMP_ROOT/unknown-owner-bin"
    mkdir -p "$HOME/.local/bin" "$bin"
    cat > "$HOME/.local/bin/pi" <<'EOF'
#!/usr/bin/env bash
printf '%s\n' '0.80.10'
EOF
    cat > "$bin/pi" <<'EOF'
#!/usr/bin/env bash
printf '%s\n' executed >> "${PI_TEST_DUPLICATE_EXEC_LOG:?}"
EOF
    chmod +x "$HOME/.local/bin/pi" "$bin/pi"
    PATH="$HOME/.local/bin:$bin:/usr/bin:/bin"; export PATH
    PI_TEST_DUPLICATE_EXEC_LOG="$TMP_ROOT/unknown-owner-executed.log"; export PI_TEST_DUPLICATE_EXEC_LOG

    unknown_out="$(pi_cli_warn_duplicate_installations 2>&1)"
    [[ "$unknown_out" == *"$bin/pi"* ]] || fail "unknown duplicate path missing: $unknown_out"
    [[ "$unknown_out" == *"original package manager"* ]] || fail "unknown-owner guidance missing: $unknown_out"
    [[ "$unknown_out" != *"cleanup (same user"* ]] || fail "unproved cleanup command was emitted"
    [[ ! -e "$PI_TEST_DUPLICATE_EXEC_LOG" ]] || fail "unknown duplicate Pi was executed"
)

(
    HOME="$TMP_ROOT/success-home"; export HOME
    bin="$TMP_ROOT/success-bin"; PATH="$bin:$HOME/.local/bin:/usr/bin:/bin"; export PATH
    PI_TEST_LOG="$TMP_ROOT/success.log"; export PI_TEST_LOG
    before="$(wc -l < "$PI_TEST_LOG" | tr -d ' ')"
    YES_ALL=1; DRY_RUN=0
    install_pi_cli >/dev/null
    after="$(wc -l < "$PI_TEST_LOG" | tr -d ' ')"
    [[ "$after" == "$before" ]] || fail "repeated Pi setup repacked an already-current install"
)

echo "OK"
