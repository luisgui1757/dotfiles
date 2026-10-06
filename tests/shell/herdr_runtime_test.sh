#!/usr/bin/env bash
# Exercise the runtime gate through an external Herdr command fixture.
set -euo pipefail
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
mkdir -p "$tmp/bin"
config="$tmp/managed config.toml"
printf 'theme = "rose-pine"\n' > "$config"
export HERDR_TEST_CONFIG="$config" HERDR_TEST_CALLS="$tmp/calls"
export HERDR_TEST_PROBE_EXIT=0 HERDR_TEST_CONFIG_EXIT=0
cat > "$tmp/bin/herdr" <<'SH'
#!/usr/bin/env bash
set -euo pipefail
if [[ "$*" == --version ]]; then
    printf '%s\n' "$HERDR_TEST_IDENTITY"
    exit "$HERDR_TEST_PROBE_EXIT"
fi
[[ $# == 2 && "$1" == config && "$2" == check ]]
[[ "$HERDR_CONFIG_PATH" == "$HERDR_TEST_CONFIG" ]]
printf 'config check\n' >> "$HERDR_TEST_CALLS"
exit "$HERDR_TEST_CONFIG_EXIT"
SH
chmod +x "$tmp/bin/herdr"
export PATH="$tmp/bin:$PATH"
fail() { printf 'FAIL: %s\n' "$*" >&2; exit 1; }
check="$REPO_ROOT/scripts/check-herdr-runtime.sh"

for version in 0.7.5 0.7.10 0.9.3 0.9.10 0.9.999999999999999999999999; do
    export HERDR_TEST_IDENTITY="herdr $version"
    rm -f "$HERDR_TEST_CALLS"
    bash "$check" "$config" > "$tmp/output" 2>&1 || fail "rejected $version"
    [[ "$(cat "$HERDR_TEST_CALLS")" == 'config check' ]] || fail "skipped config check"
done

for identity in 'herdr 0.7.4' 'herdr 0.9.2' 'herdr 0.8.9' 'herdr 0.10.0' \
    'herdr 1.0.0' 'herdr 0.9.03' 'herdr 0.9.3-rc1' 'herdr 0.9.3+build' \
    'herdr 0.9.' 'herdr 0.9.-1' 'herdr 0.9.3 extra' 'herdr 00.9.3' '' \
    $'herdr 0.9.3\nherdr 0.7.5'; do
    export HERDR_TEST_IDENTITY="$identity"
    rm -f "$HERDR_TEST_CALLS"
    if bash "$check" "$config" > "$tmp/output" 2>&1; then
        fail "accepted unreviewed identity: $identity"
    fi
    [[ ! -e "$HERDR_TEST_CALLS" ]] || fail "ran config check after rejecting identity"
done

export HERDR_TEST_IDENTITY='herdr 0.9.3' HERDR_TEST_PROBE_EXIT=7
if bash "$check" "$config" > "$tmp/output" 2>&1; then
    fail "ignored identity command failure"
fi
[[ ! -e "$HERDR_TEST_CALLS" ]] || fail "checked config after failed identity command"
export HERDR_TEST_PROBE_EXIT=0 HERDR_TEST_CONFIG_EXIT=9
if bash "$check" "$config" > "$tmp/output" 2>&1; then
    fail "ignored config rejection"
fi
[[ -s "$HERDR_TEST_CALLS" ]] || fail "did not exercise real config command boundary"
export HERDR_TEST_CONFIG_EXIT=0
export HERDR_TEST_CONFIG="$tmp/missing.toml"
rm -f "$HERDR_TEST_CALLS"
if bash "$check" "$tmp/missing.toml" > "$tmp/output" 2>&1; then
    fail "accepted missing config"
fi
[[ ! -e "$HERDR_TEST_CALLS" ]] || fail "passed a missing config to Herdr"
export HERDR_TEST_CONFIG="$config"
rm -f "$HERDR_TEST_CALLS"
if bash "$check" > "$tmp/output" 2>&1; then
    fail "accepted zero arguments"
fi
[[ ! -e "$HERDR_TEST_CALLS" ]] || fail "called Herdr with zero script arguments"
if bash "$check" "$config" "$config" > "$tmp/output" 2>&1; then
    fail "accepted two arguments"
fi
[[ ! -e "$HERDR_TEST_CALLS" ]] || fail "called Herdr with two script arguments"
bash_bin="$(command -v bash)"
mkdir "$tmp/empty-path"
if PATH="$tmp/empty-path" "$bash_bin" "$check" "$config" > "$tmp/output" 2>&1; then
    fail "accepted a missing Herdr executable"
fi
for doc in README.md docs/security/supply-chain.md; do
    for bound in '0.7.x >= 0.7.5' '0.9.x >= 0.9.3'; do
        grep -F "$bound" "$REPO_ROOT/$doc" >/dev/null || fail "$doc lost the tested bound $bound"
    done
done
echo 'OK: Herdr release bounds, strict identity and config failures'
