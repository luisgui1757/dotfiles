#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

fixture="$WORK/repo"
mkdir -p "$fixture/scripts" "$fixture/.github"
cp "$REPO_ROOT/scripts/apply-repo-safeguards.sh" "$fixture/scripts/"
cp "$REPO_ROOT/.github/check-identities.json" "$fixture/.github/"
cp "$REPO_ROOT/.github/settings.yml" "$fixture/.github/"
cp -R "$REPO_ROOT/.github/rulesets" "$fixture/.github/"
(
    cd "$fixture"
    git init -q
    git config user.name test
    git config user.email test@example.invalid
    git add .
    git commit -qm fixture
)
fixture_head="$(git -C "$fixture" rev-parse HEAD)"

mkdir -p "$WORK/bin"
cat > "$WORK/bin/gh" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
if [[ "${1:-}" == "auth" && "${2:-}" == "status" ]]; then
    exit 0
fi
if [[ "${1:-}" == "api" && "${2:-}" == "repos/owner/repo/commits/main" ]]; then
    printf '%s\n' "$TEST_LIVE_SHA"
    exit 0
fi
if [[ "${1:-}" == "api" && "${2:-}" == "--paginate" && "${3:-}" == repos/owner/repo/commits/*/check-runs* ]]; then
    printf '%s\n' "$TEST_CONTEXTS"
    exit 0
fi
printf '%s\n' "$*" >> "$TEST_MUTATION_LOG"
echo "unexpected gh invocation: $*" >&2
exit 91
EOF
chmod +x "$WORK/bin/gh"

required_contexts="$(jq -r '.required[]' "$fixture/.github/check-identities.json")"
mutation_log="$WORK/mutations.log"

run_preflight() {
    TEST_LIVE_SHA="$1" \
    TEST_CONTEXTS="$2" \
    TEST_MUTATION_LOG="$mutation_log" \
    PATH="$WORK/bin:$PATH" \
        "$fixture/scripts/apply-repo-safeguards.sh" --preflight-only owner/repo
}

output="$(run_preflight "$fixture_head" "$required_contexts")"
grep -F "Safeguard preflight passed for live main; no repository state changed." <<<"$output" >/dev/null
[[ ! -s "$mutation_log" ]] || {
    echo "FAIL: preflight-only invoked a mutating GitHub API" >&2
    exit 1
}
echo "ok  : exact live main with all stable checks passes without mutation"

missing_contexts="$(printf '%s\n' "$required_contexts" | sed '$d')"
set +e
output="$(run_preflight "$fixture_head" "$missing_contexts" 2>&1)"
rc=$?
set -e
[[ "$rc" -ne 0 ]] || {
    echo "FAIL: missing stable check passed preflight" >&2
    exit 1
}
grep -F "setup.ps1 / windows" <<<"$output" >/dev/null
echo "ok  : missing stable check fails before mutation"

set +e
output="$(run_preflight 0000000000000000000000000000000000000000 "$required_contexts" 2>&1)"
rc=$?
set -e
[[ "$rc" -ne 0 ]] || {
    echo "FAIL: non-main checkout passed preflight" >&2
    exit 1
}
grep -F "is not live main" <<<"$output" >/dev/null
echo "ok  : checkout not at live main fails before mutation"

printf '\n# local unreviewed mutation\n' >> "$fixture/.github/settings.yml"
set +e
output="$(run_preflight "$fixture_head" "$required_contexts" 2>&1)"
rc=$?
set -e
[[ "$rc" -ne 0 ]] || {
    echo "FAIL: dirty safeguard sources passed preflight" >&2
    exit 1
}
grep -F "safeguard sources differ from the exact live main commit" <<<"$output" >/dev/null
echo "ok  : dirty safeguard source fails before mutation"

[[ ! -s "$mutation_log" ]] || {
    echo "FAIL: a failed preflight invoked a mutating GitHub API" >&2
    exit 1
}
echo "all repository safeguard preflight behaviors OK"
