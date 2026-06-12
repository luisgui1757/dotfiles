#!/usr/bin/env bash
# Static parse-check for every .ps1 file in the repo. Catches syntax errors
# (like comma-as-line-continuation that PowerShell 5.1 rejects) without
# needing to actually execute the scripts. Skips gracefully on machines
# without pwsh.
set -euo pipefail
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"

if ! command -v pwsh >/dev/null 2>&1; then
    echo "skipped: pwsh not installed (brew install powershell)"
    exit 0
fi

fail=0
check_file() {
    local f="$1"
    pwsh -NoProfile -Command "
\$tokens = \$null
\$errors = \$null
[System.Management.Automation.Language.Parser]::ParseFile('$f', [ref]\$tokens, [ref]\$errors) | Out-Null
if (\$errors.Count -gt 0) {
    foreach (\$e in \$errors) { Write-Error \$e.Message }
    exit 1
}
exit 0
"
}

while IFS= read -r f; do
    if out=$(check_file "$f" 2>&1); then
        echo "ok  : $f"
    else
        echo "FAIL: $f"
        printf '%s\n' "${out//$'\n'/$'\n  '}"
        fail=1
    fi
done < <(
    find "$REPO_ROOT" -type f -name "*.ps1" -not -path "*/.git/*" -not -path "*/tests/.cache/*" -not -path "$REPO_ROOT/home/*"
    find "$REPO_ROOT/home/.chezmoitemplates" -type f -name "*.ps1" 2>/dev/null
)

[[ $fail -eq 0 ]] && echo "all ps1 files parse" || exit 1
