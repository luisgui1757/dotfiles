#!/usr/bin/env bash
# Cross-cutting invariants. These catch regressions that aren't bound to a
# single file's spec — bugs that could re-appear if someone copy-pastes from
# an old commit or AI-generates similar code.
set -euo pipefail
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
cd "$REPO_ROOT"

fail=0
check_absent() {
    local desc="$1" pattern="$2"; shift 2
    if grep -rE --binary-files=without-match "$pattern" "$@" >/dev/null 2>&1; then
        echo "FAIL: $desc — pattern '$pattern' still appears:"
        grep -rnE --binary-files=without-match "$pattern" "$@" | head -5
        fail=1
    else
        echo "ok  : $desc"
    fi
}

check_absent "NODE_TLS_REJECT_UNAUTHORIZED gone (security)" \
    "NODE_TLS_REJECT_UNAUTHORIZED" \
    --exclude-dir=.git --exclude-dir=.claude --exclude-dir=home \
    --exclude-dir=tests \
    --exclude="CLAUDE.md" --exclude="README.md" \
    .

check_absent "vim.loop replaced by vim.uv (deprecation)" \
    "vim\\.loop\\." \
    --exclude-dir=.git --exclude-dir=tests \
    nvim/

check_absent "client.supports_method dot-call replaced (0.11)" \
    "client\\.supports_method" \
    --exclude-dir=.git --exclude-dir=tests \
    nvim/

check_absent "vim.lsp.set_log_level replaced by vim.lsp.log.set_level (0.11)" \
    "vim\\.lsp\\.set_log_level" \
    --exclude-dir=.git --exclude-dir=tests \
    nvim/

check_absent "starship literal '(style)' typo gone" \
    "\\]\\(style\\)" \
    --exclude-dir=.git --exclude-dir=tests \
    starship/

check_absent "bare-Esc kill-whole-line bindkey gone (Meta prefix shadow)" \
    "bindkey[[:space:]]+'\\\\e'[[:space:]]+kill-whole-line" \
    --exclude-dir=.git --exclude-dir=tests \
    shells/

# psmux freeze guard. The native-clipboard probes use `if-shell`, which spawns a
# shell at config-LOAD time; under psmux/ConPTY on Windows that shell never
# returns and hangs the whole config load. Those probes live ONLY in the
# POSIX-only tmux.posix.conf overlay (sourced via `source-file -q`, absent on
# Windows). The cross-platform tmux.conf -- and its byte-identical chezmoi mirror
# -- must contain NO command-position `if-shell`. tmux.posix.conf legitimately
# does, so it is deliberately NOT in this check's file set.
check_absent "no load-time if-shell in cross-platform tmux.conf (psmux freeze guard)" \
    "^[[:space:]]*if-shell" \
    tmux/tmux.conf home/dot_tmux.conf

# Lazy-load discipline: only rose-pine should be lazy=false
nonlazy=$(grep -lE "lazy[[:space:]]*=[[:space:]]*false" nvim/lua/plugins/*.lua 2>/dev/null | grep -v rose-pine.lua || true)
if [[ -n "$nonlazy" ]]; then
    echo "FAIL: only rose-pine should be lazy=false. Offenders:"
    echo "$nonlazy"
    fail=1
else
    echo "ok  : lazy-load discipline (only rose-pine is lazy=false)"
fi

# Rose-pine must keep priority = 1000 so it loads before any plugin renders
# (without it, you get a brief flash of default colorscheme on startup).
if ! grep -E "priority[[:space:]]*=[[:space:]]*1000" nvim/lua/plugins/rose-pine.lua >/dev/null; then
    echo "FAIL: rose-pine.lua must keep priority = 1000"
    fail=1
else
    echo "ok  : rose-pine.lua has priority = 1000"
fi

# Lazy-lock must not contain references to removed plugins.
for stale in mason-lspconfig.nvim none-ls.nvim CopilotChat.nvim copilot.vim nui.nvim; do
    if grep -q "\"$stale\"" nvim/lazy-lock.json 2>/dev/null; then
        echo "FAIL: lazy-lock.json still references removed plugin '$stale'"
        fail=1
    fi
done

if ! grep -q 'lazy-lock\.json' nvim/init.lua \
    || ! grep -q 'locked_plugin_commit("lazy.nvim")' nvim/init.lua \
    || ! grep -q '"checkout", "--detach", lazy_commit' nvim/init.lua; then
    echo "FAIL: lazy.nvim bootstrap must pin itself to nvim/lazy-lock.json"
    fail=1
else
    echo "ok  : lazy.nvim bootstrap is lockfile-pinned"
fi

if lazy_sync_hits=$(grep -rnF '+Lazy! sync' \
    setup.sh setup.ps1 .github/workflows/e2e-install.yml \
    tests/greenfield/validate.sh tests/greenfield/validate.ps1 \
    tests/nvim/spec/startup_spec.lua); then
    echo "FAIL: setup and validation paths must use Lazy! restore, not Lazy! sync"
    echo "$lazy_sync_hits"
    fail=1
else
    echo "ok  : setup and validation paths restore lazy-lock.json instead of syncing upstream"
fi

if ! grep -q 'lazy-lock\.json' tests/nvim/minimal_init.lua \
    || ! grep -q 'locked_plugin_commit("plenary.nvim")' tests/nvim/minimal_init.lua \
    || ! grep -q '"checkout", "--detach", plenary_commit' tests/nvim/minimal_init.lua; then
    echo "FAIL: plenary test harness must pin itself to nvim/lazy-lock.json"
    fail=1
else
    echo "ok  : plenary test harness is lockfile-pinned"
fi

if ! grep -Fq -- "-path './tests/.cache'" tests/static/editorconfig_check.sh \
    || ! grep -Fq -- "! -name '.DS_Store'" tests/static/editorconfig_check.sh \
    || ! grep -Fq "editorconfig-checker \"\$file\"" tests/static/editorconfig_check.sh; then
    echo "FAIL: editorconfig_check.sh must feed editorconfig-checker a pruned file list excluding generated tests/.cache and OS metadata content"
    fail=1
elif find . \
    \( -path './.git' -o -path './.claude' -o -path './tests/.cache' -o -path './home' \) -prune -o \
    -type f \
    ! -name '.DS_Store' \
    ! -path './nvim/lazy-lock.json' \
    -print |
    grep -Fqx './tests/.cache/plenary.nvim/README.md'; then
    echo "FAIL: editorconfig_check.sh file-list pruning still includes generated tests/.cache content"
    fail=1
else
    echo "ok  : editorconfig_check.sh excludes generated tests/.cache and OS metadata content"
fi

if grep -Eq '^[[:space:]]*end_of_line[[:space:]]*=[[:space:]]*crlf' .editorconfig; then
    echo "FAIL: .editorconfig must not request CRLF; .gitattributes intentionally keeps repo text LF-only"
    fail=1
elif awk '$0 !~ /^[[:space:]]*#/ && $0 ~ /eol=crlf/ { found = 1 } END { exit(found ? 0 : 1) }' .gitattributes; then
    echo "FAIL: .gitattributes must not add CRLF overrides; repo text stays LF-only across platforms"
    fail=1
elif ! git check-attr eol text -- tests/nvim/fixtures/sample.bat tests/.cache/attr-probe.cmd |
    grep -Fqx 'tests/nvim/fixtures/sample.bat: eol: lf' ||
    ! git check-attr eol text -- tests/nvim/fixtures/sample.bat tests/.cache/attr-probe.cmd |
        grep -Fqx 'tests/nvim/fixtures/sample.bat: text: set' ||
    ! git check-attr eol text -- tests/nvim/fixtures/sample.bat tests/.cache/attr-probe.cmd |
        grep -Fqx 'tests/.cache/attr-probe.cmd: eol: lf' ||
    ! git check-attr eol text -- tests/nvim/fixtures/sample.bat tests/.cache/attr-probe.cmd |
        grep -Fqx 'tests/.cache/attr-probe.cmd: text: set'; then
    echo "FAIL: .gitattributes must explicitly enforce LF text checkout for .bat and .cmd files"
    fail=1
else
    echo "ok  : repo text line endings stay LF-only across EditorConfig and Git attributes"
fi

if ! grep -Fq '.\test.ps1' .github/workflows/test.yml; then
    echo "FAIL: Windows CI must use the repo-local test.ps1 entry point"
    fail=1
else
    echo "ok  : Windows CI uses test.ps1"
fi

if [[ ! -f AGENTS.md ]] || ! grep -q 'CLAUDE.md' AGENTS.md; then
    echo "FAIL: AGENTS.md must stay a thin pointer to CLAUDE.md"
    fail=1
elif [[ "$(wc -l < AGENTS.md | tr -d ' ')" -gt 25 ]]; then
    echo "FAIL: AGENTS.md must stay thin; keep the canonical guide in CLAUDE.md"
    fail=1
else
    echo "ok  : AGENTS.md points to the canonical CLAUDE.md guide"
fi

if grep -q 'sequential[[:space:]]*=[[:space:]]*true' tests/nvim/run.ps1 .github/workflows/test.yml 2>/dev/null; then
    echo "FAIL: Windows nvim test path must not use Plenary sequential mode"
    fail=1
else
    echo "ok  : Windows nvim test path avoids Plenary sequential mode"
fi

if grep -q 'PlenaryBustedDirectory' tests/nvim/run.ps1 2>/dev/null \
    || ! grep -q "plenary.busted" tests/nvim/run.ps1 2>/dev/null; then
    echo "FAIL: Windows nvim test path must run specs directly through plenary.busted"
    fail=1
else
    echo "ok  : Windows nvim test path uses direct plenary.busted specs"
fi

markdown_renderers=$(grep -nE "headlines\\.nvim|markview\\.nvim" nvim/lua/plugins/*.lua 2>/dev/null || true)
if [[ -n "$markdown_renderers" ]]; then
    echo "FAIL: markdown rendering must stay owned by render-markdown.nvim:"
    echo "$markdown_renderers"
    fail=1
else
    echo "ok  : markdown rendering excludes headlines.nvim and markview.nvim"
fi

# .ps1 files must be pure ASCII. Windows PowerShell 5.1 reads files
# without a BOM as ANSI / CP-1252; UTF-8 multi-byte chars (em-dash,
# arrows) get mis-tokenized and cause "Missing closing ')'" parse
# errors. Save-as-UTF-8-BOM would also work, but ASCII is simpler.
ps1_files=()
while IFS= read -r f; do ps1_files+=("$f"); done < <(
    find . -type f -name '*.ps1' -not -path './.git/*' -not -path './tests/.cache/*' -not -path './home/*'
    find ./home/.chezmoitemplates -type f -name '*.ps1' 2>/dev/null
)
find_non_ascii_ps1() {
    [[ "$#" -gt 0 ]] || return 0
    LC_ALL=C awk '/[\200-\377]/{print FILENAME; nextfile}' "$@" 2>/dev/null
}

ps1_non_ascii=""
if ! ps1_non_ascii="$(find_non_ascii_ps1 "${ps1_files[@]}")"; then
    echo "FAIL: .ps1 ASCII scan errored"
    fail=1
elif [[ -n "$ps1_non_ascii" ]]; then
    echo "FAIL: non-ASCII chars in .ps1 file(s) (PS 5.1 will mis-parse):"
    # shellcheck disable=SC2001  # sed is clearer than ${//} for line-prefixing
    echo "$ps1_non_ascii" | sed 's/^/  /'
    fail=1
else
    echo "ok  : all .ps1 files are pure ASCII (PS 5.1 safe)"
fi

ps1_ascii_regression_dir="$REPO_ROOT/tests/.cache/ps1-ascii-invariant"
rm -rf "$ps1_ascii_regression_dir"
mkdir -p "$ps1_ascii_regression_dir"
trap 'rm -rf "$ps1_ascii_regression_dir"' EXIT
ps1_ascii_regression_file="$ps1_ascii_regression_dir/high-bit.ps1"
printf 'Write-Host ok\n# high byte: \303\251\n' > "$ps1_ascii_regression_file"
ps1_ascii_regression_hit=""
if ! ps1_ascii_regression_hit="$(find_non_ascii_ps1 "$ps1_ascii_regression_file")"; then
    echo "FAIL: .ps1 ASCII regression scan errored"
    fail=1
elif [[ "$ps1_ascii_regression_hit" != "$ps1_ascii_regression_file" ]]; then
    echo "FAIL: .ps1 ASCII check missed a high-bit byte in tests/.cache"
    fail=1
else
    echo "ok  : .ps1 ASCII check catches high-bit bytes"
fi
rm -rf "$ps1_ascii_regression_dir"

# PS 5.1 has been observed mis-parsing comments that contain a lone
# apostrophe (e.g. "5.1's") — it sometimes treats the apostrophe as
# the start of a string literal that runs past the comment terminator,
# producing "Missing closing ')' in expression" errors that point at
# the wrong line. Easier to ban stray apostrophes in .ps1 comments than
# to debug the next occurrence.
ps1_comment_apos=$(grep -nE "^\s*#.*'" "${ps1_files[@]}" 2>/dev/null || true)
if [[ -n "$ps1_comment_apos" ]]; then
    echo "FAIL: apostrophe in a .ps1 comment (PS 5.1 may mis-tokenize):"
    # shellcheck disable=SC2001  # sed is clearer than ${//} for line-prefixing
    echo "$ps1_comment_apos" | sed 's/^/  /'
    fail=1
else
    echo "ok  : no apostrophes in .ps1 comments"
fi

# $Home is a READ-ONLY automatic variable. Assigning to it -- including declaring
# a param named $Home -- PARSES fine but CRASHES at runtime ("Cannot overwrite
# variable Home because it is read-only or constant"). It bit validate.ps1
# (a `[string]$Home` param) and only surfaced on a real Windows run. Use
# $HomeOverride / $userHome / $env:USERPROFILE instead. This flags an assignment
# (`$Home =`) or a typed param (`[string]$Home`); reading $Home is fine.
# ($matches is a DIFFERENT trap -- it is writable but gets clobbered by -match,
# a correctness issue, not a crash -- so it is not banned here.)
# shellcheck disable=SC2016  # literal $home in the grep regex, not a shell expansion
ps1_home_assign=$(grep -nEi '\$home[[:space:]]*=|\]\$home\b' "${ps1_files[@]}" 2>/dev/null || true)
if [[ -n "$ps1_home_assign" ]]; then
    echo "FAIL: assignment to read-only \$Home crashes PowerShell at runtime:"
    # shellcheck disable=SC2001  # sed is clearer than ${//} for line-prefixing
    echo "$ps1_home_assign" | sed 's/^/  /'
    fail=1
else
    echo "ok  : no assignment to read-only \$Home in .ps1"
fi

# Lua indentation: spaces only. stylua.toml at repo root declares Spaces+2;
# .editorconfig [*.lua] declares space+2. If a .lua file starts a line with
# a TAB, either stylua was bypassed (someone wrote without format-on-save)
# or one of the two configs got reverted -- fix the cause, do not retab
# manually. Scope: every .lua under nvim/, tests/nvim/, linux/. Portable
# `grep -E "^<TAB>"` via printf so this passes on BSD grep (macOS) too.
tab_pat="$(printf '^\t')"
tab_indented_lua=$(find nvim tests/nvim linux -name '*.lua' -type f \
    -exec grep -lE "$tab_pat" {} + 2>/dev/null || true)
if [[ -n "$tab_indented_lua" ]]; then
    echo "FAIL: .lua files have tab indentation (should be spaces):"
    # shellcheck disable=SC2001
    echo "$tab_indented_lua" | sed 's/^/  /'
    fail=1
else
    echo "ok  : no tab-indented .lua"
fi

# Dead-code guards
for dead in nvim/lua/plugins.lua nvim/lua/plugins/ai.lua nvim/lua/plugins/avante.lua nvim/lua/plugins/none-ls.lua; do
    if [[ -e "$dead" ]]; then
        echo "FAIL: $dead should be deleted"
        fail=1
    fi
done
[[ "$fail" -eq 0 ]] && echo "ok  : dead-code files gone"

[[ "$fail" -eq 0 ]] || exit 1
echo "all invariants OK"
