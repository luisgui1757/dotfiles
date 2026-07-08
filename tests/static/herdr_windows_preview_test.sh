#!/usr/bin/env bash
# Herdr native-Windows guard. Stable Herdr releases still ship macOS/Linux
# assets only; the Windows build is preview beta. Windows may therefore install
# only the repo-pinned preview .exe with adjacent SHA-256 verification, never the
# upstream herdr.dev remote-eval installer or an unpinned package-manager guess.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
INSTALL_PS1="$REPO_ROOT/install-deps.ps1"

fail=0

herdr_dev_hits="$(grep -rniE 'herdr\.dev/install' "$REPO_ROOT" \
    --include='*.sh' --include='*.ps1' --include='*.psm1' --include='*.cmd' --include='*.bat' \
    --exclude='herdr_windows_preview_test.sh' \
    --exclude-dir=.git --exclude-dir=.cache 2>/dev/null |
    grep -vE '^[^:]+:[0-9]+:[[:space:]]*#' || true)"
if [[ -n "$herdr_dev_hits" ]]; then
    echo "FAIL: repo code references the herdr.dev remote-eval installer:"
    printf '%s\n' "$herdr_dev_hits" | sed 's/^/  /'
    fail=1
else
    echo "ok  : no herdr.dev remote-eval installer in repo code"
fi

if ! grep -Eq "\\\$HerdrWindowsPreviewVersion = 'preview-[0-9]{4}-[0-9]{2}-[0-9]{2}-[0-9a-f]{12}'" "$INSTALL_PS1"; then
    echo "FAIL: install-deps.ps1 must pin a concrete Herdr Windows preview tag"
    fail=1
else
    echo "ok  : Herdr Windows preview tag is pinned"
fi

if ! grep -Eq "\\\$HerdrWindowsX64Sha256 = '[0-9a-f]{64}'" "$INSTALL_PS1"; then
    echo "FAIL: install-deps.ps1 must carry the Herdr Windows preview SHA-256"
    fail=1
else
    echo "ok  : Herdr Windows preview SHA-256 is pinned"
fi

for snippet in \
    'herdr-windows-x86_64.exe' \
    "Invoke-WebRequest -Uri \$assetUrl -OutFile \$download -UseBasicParsing -ErrorAction Stop" \
    "Test-FileSha256 -Path \$download -Expected \$HerdrWindowsX64Sha256" \
    "Copy-Item -LiteralPath \$download -Destination (Join-Path \$installRoot 'herdr.exe') -Force" \
    'Install-HerdrWindowsPreview'
do
    if ! grep -Fq "$snippet" "$INSTALL_PS1"; then
        echo "FAIL: install-deps.ps1 missing Herdr Windows direct-artifact snippet: $snippet"
        fail=1
    fi
done

if grep -Eq "^[[:space:]]*herdr[[:space:]]*=[[:space:]]*@\\{" "$INSTALL_PS1"; then
    echo "FAIL: Herdr Windows must not be a Scoop/winget/choco catalog row"
    fail=1
else
    echo "ok  : Herdr Windows is not package-manager catalog-owned"
fi

if grep -Eq 'Install-One[[:space:]]+herdr\b' "$INSTALL_PS1"; then
    echo "FAIL: Herdr Windows must not install via Install-One/package managers"
    fail=1
else
    echo "ok  : Herdr Windows installs through the pinned direct-artifact function"
fi

[[ "$fail" -eq 0 ]] || exit 1
echo "all Herdr Windows preview invariants OK"
