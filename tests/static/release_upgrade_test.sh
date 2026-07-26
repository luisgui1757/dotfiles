#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
OLD_COMMIT="015617362830280bf85c7142e69d0681d376d453"
OLD_TAG_OBJECT="a3b4d6d7b6d289959cac68d76faec96219b3e310"
cd "$REPO_ROOT"

fail() {
    echo "FAIL: $*" >&2
    exit 1
}

[[ "$(git rev-parse 'v0.1.0^{commit}')" == "$OLD_COMMIT" ]] ||
    fail "v0.1.0 peeled commit drifted"
[[ "$(git rev-parse v0.1.0)" == "$OLD_TAG_OBJECT" ]] ||
    fail "v0.1.0 annotated tag object drifted"
git ls-tree -r --name-only "$OLD_COMMIT" home |
    cmp -s - tests/fixtures/v0.1.0-home-inventory.txt ||
    fail "exact v0.1.0 home source inventory drifted"

for script in \
    scripts/install-nix-prerequisite.sh \
    scripts/upgrade-v0.1.0.sh; do
    [[ -x "$script" ]] || fail "$script must be executable"
done
[[ -f scripts/upgrade-v0.1.0.ps1 ]] || fail "Windows release migrator is missing"

grep -F 'new_tag="v0.4.3"' scripts/upgrade-v0.1.0.sh >/dev/null ||
    fail "POSIX migration does not require v0.4.3"
grep -F "\$script:NewTag = 'v0.4.3'" scripts/upgrade-v0.1.0.ps1 >/dev/null ||
    fail "Windows migration does not require v0.4.3"
grep -F 'release_tag="v0.4.3"' scripts/install-nix-prerequisite.sh >/dev/null ||
    fail "Nix prerequisite installer does not require v0.4.3"
grep -F 'RELEASE_TAG="v0.4.3"' setup.sh >/dev/null ||
    fail "POSIX setup does not advertise v0.4.3"
grep -F "\$ReleaseTag     = 'v0.4.3'" setup.ps1 >/dev/null ||
    fail "Windows setup does not advertise v0.4.3"
for script in setup.sh scripts/upgrade-v0.1.0.sh; do
    grep -F 'v0.1.0-to-v0.4.3.' "$script" >/dev/null ||
        fail "$script does not use the v0.4.3 migration recovery namespace"
done
for script in setup.ps1 scripts/upgrade-v0.1.0.ps1; do
    grep -F 'v0.1.0-to-v0.4.3.' "$script" >/dev/null ||
        fail "$script does not use the v0.4.3 migration recovery namespace"
done
python3 - <<'PY'
import ast
import json
import pathlib
import re
import sys

shell = pathlib.Path("setup.sh").read_text(encoding="utf-8")
powershell = pathlib.Path("setup.ps1").read_text(encoding="utf-8")
manifest = json.loads(pathlib.Path("release/manifest.json").read_text(encoding="utf-8"))
ledger = pathlib.Path("docs/security/supply-chain.md").read_text(encoding="utf-8")
published = re.findall(r'^\| v0\.1\.0 to (v[0-9]+\.[0-9]+\.[0-9]+) release sources \|', ledger, re.MULTILINE)
expected = [tag for tag in published if tag != manifest["current"]["tag"]]
shell_match = re.search(r'^LEGACY_RELEASE_TAGS=\(([^\n]*)\)$', shell, re.MULTILINE)
ps_match = re.search(r'^\$LegacyReleaseTags = @\(([^\n]*)\)$', powershell, re.MULTILINE)
if not shell_match or not ps_match:
    print("FAIL: setup legacy release registries are missing", file=sys.stderr)
    sys.exit(1)
shell_tags = re.findall(r'"(v[0-9]+\.[0-9]+\.[0-9]+)"', shell_match.group(1))
ps_tags = [ast.literal_eval(part.strip()) for part in ps_match.group(1).split(",")]
if shell_tags != expected or ps_tags != expected:
    print(f"FAIL: setup legacy release registries drifted: shell={shell_tags}, powershell={ps_tags}", file=sys.stderr)
    sys.exit(1)
for text, label in ((shell, "setup.sh"), (powershell, "setup.ps1")):
    for snippet in ("for legacy_tag in", "unfinished $legacy_tag migration") if label.endswith(".sh") else ("foreach ($legacyTag in $LegacyReleaseTags)", "unfinished $legacyTag migration"):
        if snippet not in text:
            print(f"FAIL: {label} does not consume the complete legacy release registry", file=sys.stderr)
            sys.exit(1)
PY

for identity in "$OLD_COMMIT" "$OLD_TAG_OBJECT"; do
    grep -F "$identity" scripts/upgrade-v0.1.0.sh >/dev/null ||
        fail "POSIX migration lost historical identity $identity"
    grep -F "$identity" scripts/upgrade-v0.1.0.ps1 >/dev/null ||
        fail "Windows migration lost historical identity $identity"
done

for flag in --skip-native-deps --skip-config-scripts --skip-nvim --skip-agents; do
    grep -F -- "$flag" scripts/upgrade-v0.1.0.sh >/dev/null ||
        fail "POSIX migration is missing its reversible setup boundary: $flag"
done
grep -F -- '--skip-native-deps)' setup.sh >/dev/null ||
    fail "setup does not expose the release migration native-dependency boundary"
grep -F -- '--skip-config-scripts)' setup.sh >/dev/null ||
    fail "setup does not expose the release migration chezmoi-script boundary"
grep -F "'-All', '-SkipDeps'" scripts/upgrade-v0.1.0.ps1 >/dev/null ||
    fail "Windows migration can mutate packages inside its config recovery boundary"
for flag in '-SkipNvim' '-SkipAgents' '-SkipConfigScripts'; do
    grep -F "'$flag'" scripts/upgrade-v0.1.0.ps1 >/dev/null ||
        fail "Windows migration is missing its reversible setup boundary: $flag"
done
grep -F "[switch]\$SkipConfigScripts" setup.ps1 >/dev/null ||
    fail "Windows setup does not expose the files/symlinks-only migration boundary"
grep -F "'-SkipLegacyKnownFolderMigration'" scripts/upgrade-v0.1.0.ps1 >/dev/null ||
    fail "Windows migration can move v0.1 known-folder targets before acceptance"
for frozen_contract in \
    'Save-FrozenReleaseState -Recovery' \
    "Join-Path \$frozen.NewSource 'setup.ps1'" \
    "\$old = \$state.OldSource" \
    "\$new = \$state.NewSource"; do
    grep -F "$frozen_contract" scripts/upgrade-v0.1.0.ps1 >/dev/null ||
        fail "Windows migration lost its frozen-source boundary: $frozen_contract"
done
if grep -F "Join-Path \$preflight.NewCheckout 'setup.ps1'" scripts/upgrade-v0.1.0.ps1 >/dev/null; then
    fail "Windows migration still publishes from the mutable release checkout"
fi

for hash in \
    47cb78c9fdc7b630dbbb9a89869c8e8bcd8c9eb17be036fba18585120693a4c1 \
    5676b0887f1274e62edd175b6611af49aa8170c69c16877aa9bc6cebceb19855 \
    cfddd4008b57a71464a16d5232cba79b1c76ae9dc81bbf71b4972b0118bc29c5; do
    grep -F "$hash" scripts/install-nix-prerequisite.sh >/dev/null ||
        fail "reviewed Nix release hash is missing: $hash"
    grep -F "$hash" docs/security/supply-chain.md >/dev/null ||
        fail "reviewed Nix release hash is missing from the supply-chain ledger: $hash"
done
python3 - <<'PY'
import pathlib
import re
import sys

row = next(
    (
        line
        for line in pathlib.Path("docs/security/supply-chain.md").read_text(encoding="utf-8").splitlines()
        if line.startswith("| v0.4.3 Nix prerequisite |")
    ),
    "",
)
hashes = re.findall(r"`([0-9a-f]+)`", row)
if len(hashes) != 9 or any(len(value) != 64 for value in hashes):
    print("FAIL: v0.4.3 Nix supply-chain ledger must contain exactly nine 64-hex SHA-256 values", file=sys.stderr)
    sys.exit(1)
PY
for hash in \
    832c033bac08eac43e2749427cb3e85d12f11d34685f44153bf044c6d32fafd0 \
    de0074c29f938cac623e0734e359021a5a6b595b8969908ca7c4ef3598b88332 \
    328dc650e29350b3d87f48b4b46e564458a5f2e413abb598c271fca3191f35d1 \
    02ed7d08aea2c191cfefda3f7e21aa17a10cc9384debe494f7a4c1357b65bff1 \
    d287e7cc727ccfa49e1a4756636c8292bda00c0d0743e79035ceddc7a42a45ae \
    54c0a6e1678c4c26a28d5bf638b8654ee12b2173ba0be521be24346d4de14eff; do
    grep -F "$hash" scripts/install-nix-prerequisite.sh >/dev/null ||
        fail "reviewed Nix daemon script hash is missing from the helper: $hash"
    grep -F "$hash" docs/security/supply-chain.md >/dev/null ||
        fail "reviewed Nix daemon script hash is missing from the supply-chain ledger: $hash"
done
grep -F 'nix_version="2.34.0"' scripts/install-nix-prerequisite.sh >/dev/null ||
    fail "Nix prerequisite version drifted"
grep -F -- '--yes --no-channel-add --no-modify-profile' scripts/install-nix-prerequisite.sh >/dev/null ||
    fail "Nix prerequisite installer still allows the unused mutable channel bootstrap"

e2e_workflow=".github/workflows/e2e-install.yml"
grep -F './scripts/install-nix-prerequisite.sh --install --allow-unreleased' "$e2e_workflow" >/dev/null ||
    fail "hosted POSIX bootstrap does not exercise the exact unreleased source head"
grep -F 'umask 077' "$e2e_workflow" >/dev/null ||
    fail "hosted POSIX bootstrap does not model a restrictive managed-host umask"
if grep -F 'git checkout --detach refs/tags/v0.4.3' "$e2e_workflow" >/dev/null; then
    fail "hosted POSIX bootstrap still replaces the reviewed source head with v0.4.3"
fi
for proof in \
    'Verified local Nix daemon profile-ownership patch:' \
    'Leaving shell profiles unchanged (--no-modify-profile)' \
    'Setting up shell profiles:' \
    'channels.nixos.org/nixpkgs-unstable'; do
    grep -F "$proof" "$e2e_workflow" >/dev/null ||
        fail "hosted POSIX bootstrap does not assert installer ownership proof: $proof"
done

grep -Eq '^[[:space:]]*ensure_nix_prerequisite[[:space:]]*$' setup.sh ||
    fail "POSIX setup does not own verified Nix prerequisite installation"
grep -Eq '^[[:space:]]*maybe_complete_v0_1_upgrade[[:space:]]*$' setup.sh ||
    fail "POSIX setup does not own v0.1.0 migration orchestration"
grep -F 'DOTFILES_RELEASE_MIGRATION_ACTIVE=1' setup.sh >/dev/null ||
    fail "POSIX setup does not guard nested migration setup from recursion"
grep -F 'DOTFILES_RELEASE_MIGRATION_ACTIVE=1' scripts/upgrade-v0.1.0.sh >/dev/null ||
    fail "POSIX migrator does not guard its frozen setup from recursive migration"
grep -F "DOTFILES_RELEASE_MIGRATION_ACTIVE = '1'" scripts/upgrade-v0.1.0.ps1 >/dev/null ||
    fail "Windows migrator does not guard its frozen setup from recursive migration"
grep -F 'Invoke-SetupV01Migration -Identity' setup.ps1 >/dev/null ||
    fail "Windows setup does not own v0.1.0 migration orchestration"
grep -F "[Alias('Upgrade')] [switch]\$Update" setup.ps1 >/dev/null ||
    fail "Windows setup does not retain Upgrade as an Update alias"
grep -F -- '--update|--upgrade)' setup.sh >/dev/null ||
    fail "POSIX setup does not retain upgrade as an update alias"
grep -F 'setup_universal_entrypoint_test.sh' < <(find tests/shell -maxdepth 1 -type f -print) >/dev/null ||
    fail "universal setup orchestration regression is missing"

grep -F 'tests/migration/v0_1_upgrade_test.sh' Makefile >/dev/null ||
    fail "exact historical upgrade test is not in the local gate"
grep -F "'scripts/upgrade-v0.1.0.ps1'" test.ps1 >/dev/null ||
    fail "Windows migration script is outside analyzer coverage"
[[ "$(grep -c 'Exact v0.1.0 release migration' .github/workflows/test.yml)" -eq 2 ]] ||
    fail "exact historical migration must run on hosted Linux and Apple Silicon"

for document in README.md docs/UPGRADING.md docs/releases/v*.md; do
    [[ -f "$document" ]] || fail "release documentation is missing: $document"
done
if grep -F 'git -C ~/dotfiles pull' README.md >/dev/null; then
    fail "README still publishes the unsafe in-place v0.1.0 command"
fi
if grep -R -E '<post-Nix-release-tag>|<next-release>|git (pull|checkout).*main' \
    README.md docs/UPGRADING.md docs/releases/v*.md >/dev/null; then
    fail "release documentation contains a moving branch or release placeholder"
fi
# Literal Markdown assertion.
# shellcheck disable=SC2016
grep -F '`v0.1.0` is already chezmoi-based' README.md >/dev/null ||
    fail "README still misclassifies v0.1.0"
grep -F './setup.sh --all' docs/UPGRADING.md >/dev/null ||
    fail "upgrade guide does not make POSIX setup the normal migration entry point"
grep -F '.\setup.ps1 -All' docs/UPGRADING.md >/dev/null ||
    fail "upgrade guide does not make Windows setup the normal migration entry point"
grep -F 'setup bootstraps Nix' README.md >/dev/null ||
    fail "README does not state that POSIX setup owns Nix bootstrap"

echo "release upgrade identities, rollback boundaries, evidence, and documentation OK"
