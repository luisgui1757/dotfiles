#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
TMP_ROOT="$REPO_ROOT/tests/.cache/setup-experimental-wsl-flag-test"
rm -rf "$TMP_ROOT"
mkdir -p "$TMP_ROOT"
trap 'rm -rf "$TMP_ROOT"' EXIT

cp "$REPO_ROOT/setup.sh" "$TMP_ROOT/setup.sh"
mkdir -p "$TMP_ROOT/home/.local/state/nix/profile/bin" "$TMP_ROOT/fakebin" \
    "$TMP_ROOT/pi" "$TMP_ROOT/scripts"
for theme_name in \
    rose-pine rose-pine-moon rose-pine-dawn
do
    cp "$REPO_ROOT/pi/$theme_name.json" "$TMP_ROOT/pi/$theme_name.json"
done
cp "$REPO_ROOT/scripts/configure-pi-theme.mjs" "$TMP_ROOT/scripts/configure-pi-theme.mjs"
cat > "$TMP_ROOT/install-deps.sh" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
printf '%s\n' "$@" > "$(cd "$(dirname "$0")" && pwd -P)/deps.args"
printf '%s\n' "${DOTFILES_EXPERIMENTAL_WSL_GUI:-}" > "$(cd "$(dirname "$0")" && pwd -P)/deps.env"
EOF
cat > "$TMP_ROOT/fakebin/chezmoi" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
printf '%s\n' "$*" >> "$SETUP_TEST_ROOT/chezmoi.args"
case " $* " in
    *" managed "*) exit 0 ;;
    *" apply "*)
        mkdir -p "$HOME/.pi/agent/themes"
        for theme_name in \
            rose-pine rose-pine-moon rose-pine-dawn
        do
            cp "$SETUP_TEST_ROOT/pi/$theme_name.json" "$HOME/.pi/agent/themes/$theme_name.json"
        done
        exit 0
        ;;
    *) exit 0 ;;
esac
EOF
cat > "$TMP_ROOT/fakebin/brew" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
if [[ "${1:-}" == "shellenv" ]]; then
    printf 'export PATH="%s/fakebin:$PATH"\n' "$SETUP_TEST_ROOT"
fi
EOF
cat > "$TMP_ROOT/fakebin/uname" <<'EOF'
#!/usr/bin/env bash
case "${1:-}" in
    -s) printf '%s\n' Linux ;;
    -m) printf '%s\n' x86_64 ;;
    *) command uname "$@" ;;
esac
EOF
cat > "$TMP_ROOT/fakebin/nix" <<'EOF'
#!/usr/bin/env bash
if [[ "${1:-}" == "eval" ]]; then
    printf '%s\n%s\n' "fake-home-manager-rev" "sha256-fake"
fi
EOF
cat > "$TMP_ROOT/fakebin/home-manager" <<'EOF'
#!/usr/bin/env bash
exit 0
EOF
cat > "$TMP_ROOT/fakebin/id" <<'EOF'
#!/usr/bin/env bash
case "${1:-}" in -u) echo 1000 ;; -un) echo tester ;; *) exit 1 ;; esac
EOF
cat > "$TMP_ROOT/fakebin/getent" <<EOF
#!/usr/bin/env bash
[[ "\${1:-}" == passwd && "\${2:-}" == tester ]] || exit 1
printf '%s\n' 'tester:x:1000:1000:Test User:$TMP_ROOT/home:/bin/zsh'
EOF
chmod +x "$TMP_ROOT/install-deps.sh" "$TMP_ROOT/fakebin/chezmoi" "$TMP_ROOT/fakebin/brew" \
    "$TMP_ROOT/fakebin/uname" "$TMP_ROOT/fakebin/nix" "$TMP_ROOT/fakebin/home-manager" \
    "$TMP_ROOT/fakebin/id" "$TMP_ROOT/fakebin/getent"
ln -s "$TMP_ROOT/fakebin/chezmoi" "$TMP_ROOT/home/.local/state/nix/profile/bin/chezmoi"
ln -s "$TMP_ROOT/fakebin/brew" "$TMP_ROOT/home/.local/state/nix/profile/bin/brew"
ln -s "$(command -v node)" "$TMP_ROOT/fakebin/node"

output="$(HOME="$TMP_ROOT/home" SETUP_TEST_ROOT="$TMP_ROOT" PATH="$TMP_ROOT/fakebin:/usr/bin:/bin" bash "$TMP_ROOT/setup.sh" --all --skip-nvim --skip-agents --experimental-wsl-gui 2>&1)"
[[ "$output" == *"setup.sh: done"* ]]
grep -Fx -- "--experimental-wsl-gui" "$TMP_ROOT/deps.args" >/dev/null
grep -Fx -- "1" "$TMP_ROOT/deps.env" >/dev/null
grep -F -- "--override-data {\"experimentalWslGui\":true} managed" "$TMP_ROOT/chezmoi.args" >/dev/null
grep -F -- "--override-data {\"experimentalWslGui\":true} --no-tty --force apply" "$TMP_ROOT/chezmoi.args" >/dev/null
python3 - "$TMP_ROOT/home/.pi/agent/settings.json" <<'PY'
import json
import pathlib
import sys

assert json.loads(pathlib.Path(sys.argv[1]).read_text())["theme"] == "rose-pine"
PY

python3 - "$TMP_ROOT/home/.pi/agent/settings.json" <<'PY'
import json
import pathlib
import sys

path = pathlib.Path(sys.argv[1])
settings = json.loads(path.read_text())
settings["theme"] = "rose-pine-moon"
path.write_text(json.dumps(settings) + "\n")
PY

for retired_theme in \
    rose-pine-fable rose-pine-moon-fable rose-pine-dawn-fable
do
    ln -s \
        "$TMP_ROOT/home/dot_pi/agent/themes/$retired_theme.json" \
        "$TMP_ROOT/home/.pi/agent/themes/$retired_theme.json"
done
output="$(HOME="$TMP_ROOT/home" SETUP_TEST_ROOT="$TMP_ROOT" PATH="$TMP_ROOT/fakebin:/usr/bin:/bin" bash "$TMP_ROOT/setup.sh" --all --skip-nvim --skip-agents --experimental-wsl-gui 2>&1)"
[[ "$output" == *"setup.sh: done"* ]]
python3 - "$TMP_ROOT/home/.pi/agent/settings.json" <<'PY'
import json
import pathlib
import sys

assert json.loads(pathlib.Path(sys.argv[1]).read_text())["theme"] == "rose-pine-moon"
PY
for retired_theme in \
    rose-pine-fable rose-pine-moon-fable rose-pine-dawn-fable
do
    [[ ! -e "$TMP_ROOT/home/.pi/agent/themes/$retired_theme.json" ]]
    [[ ! -L "$TMP_ROOT/home/.pi/agent/themes/$retired_theme.json" ]]
done

printf '%s\n' '{"name":"rose-pine-fable","userOwned":true}' \
    > "$TMP_ROOT/home/.pi/agent/themes/rose-pine-fable.json"
sed 's/"name": "rose-pine-moon"/"name": "rose-pine-moon-fable"/' \
    "$TMP_ROOT/pi/rose-pine-moon.json" \
    > "$TMP_ROOT/home/.pi/agent/themes/rose-pine-moon-fable.json"
sed 's/"name": "rose-pine-dawn"/"name": "rose-pine-dawn-fable"/' \
    "$TMP_ROOT/pi/rose-pine-dawn.json" | sed 's/$/\r/' \
    > "$TMP_ROOT/home/.pi/agent/themes/rose-pine-dawn-fable.json"
output="$(HOME="$TMP_ROOT/home" SETUP_TEST_ROOT="$TMP_ROOT" PATH="$TMP_ROOT/fakebin:/usr/bin:/bin" bash "$TMP_ROOT/setup.sh" --all --skip-nvim --skip-agents --experimental-wsl-gui 2>&1)"
[[ "$output" == *"setup.sh: done"* ]]
grep -F '"userOwned":true' "$TMP_ROOT/home/.pi/agent/themes/rose-pine-fable.json" >/dev/null
[[ ! -e "$TMP_ROOT/home/.pi/agent/themes/rose-pine-moon-fable.json" ]]
[[ ! -e "$TMP_ROOT/home/.pi/agent/themes/rose-pine-dawn-fable.json" ]]

echo "OK"
