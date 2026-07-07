#!/usr/bin/env bash
# Herdr native-Windows guard. Herdr's stable channel is macOS/Linux only; its
# native Windows build is preview-only beta and installable only through a banned
# `irm | iex` remote-eval. So install-deps.ps1 must NOT install Herdr, and the
# herdr.dev remote-eval installer must not appear in any repo code path.
# (Comments documenting the omission are allowed.)
set -euo pipefail
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"

fail=0

# 1) install-deps.ps1 must have no NON-COMMENT Herdr reference (no catalog entry,
#    no Install-One herdr, no brew/scoop/winget/choco herdr).
ps1_hits="$(grep -iE 'herdr' "$REPO_ROOT/install-deps.ps1" | grep -vE '^[[:space:]]*#' || true)"
if [[ -n "$ps1_hits" ]]; then
    echo "FAIL: install-deps.ps1 has a non-comment Herdr reference (Windows Herdr is preview-beta, must not install):"
    printf '%s\n' "$ps1_hits" | sed 's/^/  /'
    fail=1
else
    echo "ok  : install-deps.ps1 installs no Herdr (native Windows blocked)"
fi

# 2) The herdr.dev remote-eval installer (install.sh / install.ps1) must not be
#    referenced by any repo shell/PowerShell code.
if grep -rniE 'herdr\.dev/install' "$REPO_ROOT" \
    --include='*.sh' --include='*.ps1' \
    --exclude-dir=.git --exclude-dir=.cache >/dev/null 2>&1; then
    echo "FAIL: repo code references the herdr.dev remote-eval installer:"
    grep -rniE 'herdr\.dev/install' "$REPO_ROOT" --include='*.sh' --include='*.ps1' \
        --exclude-dir=.git --exclude-dir=.cache | sed 's/^/  /'
    fail=1
else
    echo "ok  : no herdr.dev remote-eval installer in repo code"
fi

[[ "$fail" -eq 0 ]] || exit 1
echo "all herdr Windows-block invariants OK"
