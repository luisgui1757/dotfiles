#!/usr/bin/env bash
# Exercise the real helper against private mounted directories, never the
# runner's original /usr/share. Mounts disappear with the private namespace.
set -euo pipefail
if [[ "${GITHUB_ACTIONS:-}" != true || "${RUNNER_ENVIRONMENT:-}" != github-hosted || "$(uname -s)" != Linux ]]; then
    echo 'FAIL: native host preparation tests require disposable GitHub-hosted Linux' >&2
    exit 1
fi
repo=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)
fixture=$(mktemp -d)
trap 'sudo -n rm -rf -- "$fixture"' EXIT

sudo -n unshare --mount --propagation private bash -s -- "$repo/scripts/prepare-disposable-linux-host.sh" "$fixture" <<'FIXTURE'
set -euo pipefail
helper=$1
fixture=$2
export GITHUB_ACTIONS=true RUNNER_ENVIRONMENT=github-hosted
mkdir -p "$fixture/share/zsh/vendor-completions" "$fixture/redirect"
chmod 0777 "$fixture/share" "$fixture/share/zsh" "$fixture/share/zsh/vendor-completions" "$fixture/redirect"
mount --bind "$fixture/share" /usr/share

unchanged() {
    for path in /usr/share /usr/share/zsh; do
        test "$(stat -c %a "$path")" = 777
    done
}
refused() {
    if bash "$helper" "$@"; then
        echo 'FAIL: unsafe fixture unexpectedly allowed host preparation' >&2
        exit 1
    fi
    unchanged
}

chown 65534:65534 /usr/share/zsh/vendor-completions
refused shell
test "$(stat -c %u:%g:%a /usr/share/zsh/vendor-completions)" = 65534:65534:777
echo 'PASS: foreign ownership refused before any chmod'
chown 0:0 /usr/share/zsh/vendor-completions

rmdir /usr/share/zsh/vendor-completions
ln -s "$fixture/redirect" /usr/share/zsh/vendor-completions
refused shell
test "$(stat -c %a "$fixture/redirect")" = 777
echo 'PASS: redirected completion directory and destination preserved'
rm /usr/share/zsh/vendor-completions
mkdir -m 0777 /usr/share/zsh/vendor-completions

mv /usr/share/zsh "$fixture/original-zsh"
ln -s "$fixture/original-zsh" /usr/share/zsh
if bash "$helper" shell; then exit 1; fi
test "$(stat -c %a /usr/share)" = 777
test "$(stat -c %a "$fixture/original-zsh")" = 777
echo 'PASS: redirected ancestor refused before any chmod'
rm /usr/share/zsh
mv "$fixture/original-zsh" /usr/share/zsh

GITHUB_ACTIONS=false refused shell
RUNNER_ENVIRONMENT=self-hosted refused shell
refused arbitrary
refused desktop extra
echo 'PASS: host and fixed-recipe guards reject unsupported authority'

bash "$helper" desktop
test "$(stat -c %u:%g:%a /usr/share)" = 0:0:755
test "$(stat -c %a /usr/share/zsh)" = 777
test "$(stat -c %a /usr/share/zsh/vendor-completions)" = 777
echo 'PASS: desktop recipe changes only its reviewed parent'
FIXTURE
