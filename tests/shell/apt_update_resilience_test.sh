#!/usr/bin/env bash
# Regression guard: a flaky `apt-get update` must NOT prevent `apt-get install`.
#
# The apt arms of pm_install / native_linux_pm_install / pm_update previously
# ran `apt-get update -qq && apt-get install ...`, so a single failing update
# (unreachable third-party PPA, expired repo key, transient mirror outage --
# all common) short-circuited the && and the install was NEVER attempted, even
# when the package is already indexed in the local apt cache. The fix decouples
# them: update is best-effort (warn on failure), install always runs. Every apt
# call also carries DEBIAN_FRONTEND=noninteractive through sudo so dependency
# packages such as tzdata cannot block an unattended --all run.
# shellcheck disable=SC1091,SC2034
set -uo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
INSTALL_DEPS_SOURCE_ONLY=1 source "$REPO_ROOT/install-deps.sh"

fail() { echo "FAIL: $*" >&2; exit 1; }

TMP_ROOT="$(mktemp -d)"
trap 'rm -rf "$TMP_ROOT"' EXIT
CMD_LOG="$TMP_ROOT/commands.log"
: > "$CMD_LOG"

DRY_RUN=0

# Simulate a host where `apt-get update` ALWAYS fails (exit 100, apt's real
# failure code) but `apt-get install` would succeed from the existing cache.
maybe_sudo() {
    printf '%s\n' "$*" >> "$CMD_LOG"
    case "$*" in
        *"apt-get update"*) return 100 ;;
        *) return 0 ;;
    esac
}

# --- pm_install: install proceeds despite the failed update ------------------
PM=apt
err="$(pm_install ripgrep curl 2>&1 >/dev/null)"; rc=$?
grep -Fq 'apt-get update -qq' "$CMD_LOG" || fail "pm_install: update was not attempted first"
grep -Fq 'apt-get install -y ripgrep curl' "$CMD_LOG" || fail "pm_install: install was SKIPPED after a failing update (the && short-circuit bug)"
grep -Fq 'env DEBIAN_FRONTEND=noninteractive apt-get update -qq' "$CMD_LOG" || fail "pm_install: apt update did not carry the noninteractive debconf boundary through sudo"
grep -Fq 'env DEBIAN_FRONTEND=noninteractive apt-get install -y ripgrep curl' "$CMD_LOG" || fail "pm_install: apt install did not carry the noninteractive debconf boundary through sudo"
[[ "$rc" -eq 0 ]] || fail "pm_install: returned $rc; should reflect the (successful) install, not the failed update"
printf '%s\n' "$err" | grep -Fq 'WARN: apt-get update failed' || fail "pm_install: no WARN emitted on update failure"

# --- native_linux_pm_install: same decoupling, same return semantics ---------
: > "$CMD_LOG"
native_linux_pm_install apt ripgrep >/dev/null 2>&1; rc=$?
grep -Fq 'apt-get install -y ripgrep' "$CMD_LOG" || fail "native_linux_pm_install: install SKIPPED after failing update"
grep -Fq 'env DEBIAN_FRONTEND=noninteractive apt-get install -y ripgrep' "$CMD_LOG" || fail "native_linux_pm_install: apt install was not noninteractive"
[[ "$rc" -eq 0 ]] || fail "native_linux_pm_install: returned $rc; should reflect the install, not the failed update"

# --- pm_update: scoped upgrade still proceeds (tool, pkg) --------------------
: > "$CMD_LOG"
PM=apt
pm_update ripgrep ripgrep >/dev/null 2>&1; rc=$?
grep -Fq 'apt-get install -y --only-upgrade ripgrep' "$CMD_LOG" || fail "pm_update: upgrade SKIPPED after failing update"
grep -Fq 'env DEBIAN_FRONTEND=noninteractive apt-get install -y --only-upgrade ripgrep' "$CMD_LOG" || fail "pm_update: scoped apt upgrade was not noninteractive"
[[ "$rc" -eq 0 ]] || fail "pm_update: returned $rc; should reflect the upgrade, not the failed update"

echo "OK"
