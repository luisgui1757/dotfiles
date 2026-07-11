#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
cd "$REPO_ROOT"

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

expected="$tmp/expected.txt"
settings="$tmp/settings.txt"
safeguards_json="$tmp/safeguards-json.txt"
safeguards_function="$tmp/safeguards-function.txt"
ruleset="$tmp/ruleset.txt"

jq -r '.required[]' .github/check-identities.json > "$expected"

python3 - <<'PY'
import json
import pathlib
import re
import sys

metadata = json.loads(pathlib.Path(".github/check-identities.json").read_text(encoding="utf-8"))
if metadata.get("schema") != 2 or metadata.get("stage") != "stable-required-live-apply-pending":
    raise SystemExit("FAIL: required-check migration metadata has an unsupported stage")

test_workflow = pathlib.Path(".github/workflows/test.yml").read_text(encoding="utf-8")
test_jobs = test_workflow.split("\njobs:\n", 1)[1]
stable_jobs = set(re.findall(r"(?m)^  ([A-Za-z0-9_-]+):\s*$", test_jobs))
e2e = pathlib.Path(".github/workflows/e2e-install.yml").read_text(encoding="utf-8")
nix = pathlib.Path(".github/workflows/nix.yml").read_text(encoding="utf-8")

legacy = stable_jobs | set(re.findall(r"(?m)^\s+legacy_context:\s*(.+?)\s*$", e2e + "\n" + nix))
expected_legacy = set(metadata["legacyEmitted"])
if legacy != expected_legacy:
    print("FAIL: emitted legacy check identities differ from legacyEmitted", file=sys.stderr)
    print("missing:", sorted(expected_legacy - legacy), file=sys.stderr)
    print("extra:", sorted(legacy - expected_legacy), file=sys.stderr)
    raise SystemExit(1)

e2e_logical = set(re.findall(r"(?m)^\s+- logical_context:\s*(.+?)\s*$", e2e))
nix_logical_block = nix.split("\n  logical-proof:\n", 1)[1]
nix_logical = {
    f"nix flake check / {value}"
    for value in re.findall(r"(?m)^\s+- logical:\s*(.+?)\s*$", nix_logical_block)
}
required = stable_jobs | e2e_logical | nix_logical
expected_required = set(metadata["required"])
if required != expected_required:
    print("FAIL: emitted logical identities differ from required", file=sys.stderr)
    print("missing:", sorted(expected_required - required), file=sys.stderr)
    print("extra:", sorted(required - expected_required), file=sys.stderr)
    raise SystemExit(1)

replacements = {(item["legacy"], item["logical"]) for item in metadata["replacements"]}
if {old for old, _ in replacements} != expected_legacy - required:
    raise SystemExit("FAIL: replacement metadata does not cover every legacy-only identity")
if {new for _, new in replacements} != required - expected_legacy:
    raise SystemExit("FAIL: replacement metadata does not cover every logical-only identity")
if len(replacements) != len(metadata["replacements"]):
    raise SystemExit("FAIL: duplicate required-check replacement mapping")

for workflow in (e2e, nix):
    if "ci-logical-proof.sh emit" not in workflow or "ci-logical-proof.sh verify" not in workflow:
        raise SystemExit("FAIL: logical checks are not bound to exact proof artifacts")
    if "DOTFILES_SOURCE_HEAD_SHA: ${{ github.event.pull_request.head.sha || github.sha }}" not in workflow:
        raise SystemExit("FAIL: logical proof workflow does not distinguish PR source head from executed SHA")

print("OK: stable required identities are emitted while legacy producers remain available for the live transition")
PY

awk '
  /^[[:space:]]*contexts:[[:space:]]*$/ { in_contexts = 1; next }
  in_contexts && /^[[:space:]]*-[[:space:]]*/ {
    line = $0
    sub(/^[[:space:]]*-[[:space:]]*/, "", line)
    print line
    next
  }
  in_contexts && ! /^[[:space:]]*-[[:space:]]*/ { in_contexts = 0 }
' .github/settings.yml > "$settings"

awk '
  /"contexts": \[/ { in_contexts = 1; next }
  in_contexts && /^[[:space:]]*\]/ { in_contexts = 0; next }
  in_contexts {
    line = $0
    sub(/^[[:space:]]*"/, "", line)
    sub(/",?[[:space:]]*$/, "", line)
    if (line != "") print line
  }
' scripts/apply-repo-safeguards.sh > "$safeguards_json"

awk '
  /^required_check_contexts\(\) \{/ { in_fn = 1; next }
  in_fn && /^}/ { in_fn = 0; next }
  in_fn && /^[[:space:]]*cat <<'\''EOF'\''/ { in_heredoc = 1; next }
  in_fn && in_heredoc && /^EOF$/ { in_heredoc = 0; next }
  in_fn && in_heredoc { print }
' scripts/apply-repo-safeguards.sh > "$safeguards_function"

python3 - <<'PY' > "$ruleset"
import json

with open(".github/rulesets/main-integrity.json", encoding="utf-8") as fh:
    data = json.load(fh)

for rule in data["rules"]:
    if rule["type"] == "required_status_checks":
        for check in rule["parameters"]["required_status_checks"]:
            print(check["context"])
        break
PY

if ! diff -u "$expected" "$settings"; then
    echo "FAIL: .github/settings.yml required checks are out of sync with the stable target" >&2
    exit 1
fi
if ! diff -u "$expected" "$safeguards_json"; then
    echo "FAIL: safeguards JSON required checks are out of sync with the stable target" >&2
    exit 1
fi
if ! diff -u "$expected" "$safeguards_function"; then
    echo "FAIL: safeguards required_check_contexts is out of sync with the stable target" >&2
    exit 1
fi
if ! diff -u "$expected" "$ruleset"; then
    echo "FAIL: main-integrity ruleset required checks are out of sync with the stable target" >&2
    exit 1
fi

echo "OK"
