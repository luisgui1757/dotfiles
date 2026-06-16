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
    # Pass the path through an environment variable, NOT interpolated into the
    # command string. The old form embedded $f inside a single-quoted PowerShell
    # literal, so a repo path containing a single quote broke the parser harness
    # itself. An env var needs no quoting and is robust to any character.
    # shellcheck disable=SC2016  # the $ are PowerShell vars; single quotes are deliberate
    PS1_PARSE_FILE="$f" pwsh -NoProfile -Command '
$path = $env:PS1_PARSE_FILE
$tokens = $null
$errors = $null
[System.Management.Automation.Language.Parser]::ParseFile($path, [ref]$tokens, [ref]$errors) | Out-Null
if ($errors.Count -gt 0) {
    foreach ($e in $errors) { Write-Error $e.Message }
    exit 1
}
exit 0
'
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
