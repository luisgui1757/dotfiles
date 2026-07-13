#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
DOTFILES_LINUX_OWNER_LIFECYCLE_SOURCE_ONLY=1 source "$REPO_ROOT/tests/linux_owner_lifecycle.sh"

dockerfile="$REPO_ROOT/tests/greenfield/linux-owner-lifecycle.Dockerfile"
base_image_count="$(grep -Ec '^FROM ' "$dockerfile")"
[[ "$base_image_count" -eq 2 ]] ||
    fail "Linux lifecycle Dockerfile must declare exactly two base images"
while IFS= read -r base_image; do
    [[ "$base_image" =~ ^[^@]+@sha256:[0-9a-f]{64}$ ]] ||
        fail "Linux lifecycle base image is not digest-pinned: $base_image"
done < <(sed -n 's/^FROM \([^ ]*\)\( AS [^ ]*\)\{0,1\}$/\1/p' "$dockerfile")
echo "ok  : Linux lifecycle container bases are immutable digest pins"

docker_driver="$REPO_ROOT/tests/greenfield/docker-linux-owner-lifecycle.sh"
container_driver="$REPO_ROOT/tests/ci/linux-owner-lifecycle-container.sh"
grep -F "git -C \"\$REPO_ROOT\" bundle create \"\$bundle\" HEAD" "$docker_driver" >/dev/null ||
    fail "Linux lifecycle driver must export the exact committed HEAD as a Git bundle"
grep -F "[[ \"\$actual_head\" == \"\$expected_head\" ]]" "$container_driver" >/dev/null ||
    fail "Linux lifecycle container must verify the bundled checkout commit"
if grep -F -- "--volume \"\$REPO_ROOT:/repo:ro\"" "$docker_driver" >/dev/null; then
    fail "Linux lifecycle must not copy Docker Desktop's bind-mounted .git directory"
fi
echo "ok  : Linux lifecycle transports and verifies an exact-HEAD Git bundle"

WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT
fake_bin="$WORK/bin"
STATE_DIR="$WORK/state"
mkdir -p "$fake_bin" "$STATE_DIR"

cat > "$fake_bin/dpkg-query" <<'DPKG'
#!/usr/bin/env bash
printf '%s\n' ${DOTFILES_TEST_PACKAGE_ROWS:?}
DPKG
chmod +x "$fake_bin/dpkg-query"
PATH="$fake_bin:/usr/bin:/bin"
export PATH

detect_package_backend
[[ "$PACKAGE_BACKEND" == "dpkg" ]] || fail "lifecycle did not select the available dpkg inventory"

DOTFILES_TEST_PACKAGE_ROWS='zlib bash coreutils'
export DOTFILES_TEST_PACKAGE_ROWS
write_package_inventory "$STATE_DIR/packages.before"
DOTFILES_TEST_PACKAGE_ROWS='curl zlib bash coreutils'
export DOTFILES_TEST_PACKAGE_ROWS
assert_no_removed_packages >/dev/null
echo "ok  : Linux lifecycle accepts additive native package installation"

DOTFILES_TEST_PACKAGE_ROWS='zlib bash'
export DOTFILES_TEST_PACKAGE_ROWS
if (assert_no_removed_packages >/dev/null 2>&1); then
    fail "lifecycle accepted removal of a pre-existing native package"
fi
echo "ok  : Linux lifecycle rejects removal of a pre-existing native package"

echo "all Linux owner lifecycle package-boundary behaviors OK"
