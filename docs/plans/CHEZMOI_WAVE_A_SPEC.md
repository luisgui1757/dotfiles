# Dotfiles → chezmoi — Wave A Pilot Implementation Spec (Codex-ready)

_Companion to `ROADMAP.md` → "Dotfiles → chezmoi Migration Program (2026-06-08)" (DC-1…DC-6)
and a sibling of `docs/plans/CONTAINERIZATION_WAVE_A_SPEC.md` (same house format). This spec
turns **Wave A** (the pilot + its parity gate) into something Codex 5.5 xhigh can execute
literally, grounded line-by-line in the real `luisgui1757/dotfiles` tree and in chezmoi's
documented semantics._

2026-06-09 current-branch status: this document is now partly historical. The
branch grew from the original Wave A pilot into a full config-layer migration:
`home/` covers nvim, Starship, zshenv/zshrc, Ghostty, lazygit, tmux,
`tmux.windows.conf`, Windows Terminal merge, the Windows PowerShell profile, and
the two pinned zsh plugin externals. Use
`docs/MIGRATION_STATUS.md` as the current owner-facing status. The original
pilot steps remain useful implementation history, but any "pilot only" wording
below is superseded by the manifest-driven full migrated config set described
in DC-6 status notes.

---

## How to use this spec (Codex 5.5 xhigh)

- Execute **top to bottom**, starting at **Step 0 (Prerequisites)**. Each `### Step` is
  self-contained: it names the exact file to create/edit, gives a paste-ready block, and ends with
  how to prove it.
- **Do not invent chezmoi behavior.** Every mechanism used here is pinned in **"Verified chezmoi
  mechanics"** below with a chezmoi.io citation. If a step seems to need a feature not listed there,
  STOP and surface it — do not guess.
- **Do not fabricate pins/SHAs/paths.** Every version pin, sha256, and target path is cited to a
  `file:line` in the dotfiles repo (grounded @ HEAD `96d85ee`). If you cannot re-verify a citation
  in the working tree, STOP and report the drift.
- Where a value must be obtained at runtime (a base digest, a commit SHA), the step says so with the
  command to get it. Never write a placeholder as if it were real.
- **Scope discipline:** this was written as a pilot spec. On the current branch,
  the config layer has expanded beyond the original seed slices; keep the
  current scope grounded in `docs/MIGRATION_STATUS.md` and the DC-6 manifest,
  not in stale pilot allow-list assumptions.

## Where this runs — CRITICAL

**The work happens in the `luisgui1757/dotfiles` repo, NOT in Meridian.** This spec is *parked* in
the Meridian roadmap (owner request) so the two 2026-06-08 plans live together; it is not Meridian
product scope and should relocate to a dotfiles `ROADMAP.md`/issue when convenient.

- Operate on a checkout of `luisgui1757/dotfiles` on a feature branch (e.g. `chezmoi-pilot`).
- Historical instruction: build the chezmoi **source directory** as a new top-level dir **`home/`** in
  that repo. On the current branch, `home/` already exists. The existing `setup.sh` / `bootstrap.sh` /
  `install-deps.*` stay **untouched and working** until Wave C — the parity gate (DC-6) runs the
  relevant config slices of the old path beside chezmoi and compares them, so they MUST coexist.
  Nothing in the old install path is deleted in Wave A (that is Wave C, after N green parity runs).

## Original Scope — Wave A seed pilot

The pilot exercises the *mechanically hard* parts on a minimal slice so the owner can feel the
ergonomics before the 16–24-day Wave B (`ROADMAP.md:583-608`). Exactly these slices:

Current status note: the table below is the original seed scope. The checked-in
branch has expanded beyond it into the full config-layer manifest described in
DC-6 and `docs/MIGRATION_STATUS.md`.

| Slice | What it proves | DC |
|---|---|---|
| **tmux single-source** (`tmux.conf` → `~/.tmux.conf` everywhere) + **Windows overlay** (`tmux.windows.conf`, ignored off-Windows) | trivial-config case + `.chezmoiignore` per-OS gating | DC-1 |
| **Path-divergent symlink: lazygit** (one source file → 3 different absolute paths per OS, live-edit on POSIX) | the single hardest config-layer mechanic | DC-1 |
| **One external: the two zsh plugins** as pinned `.chezmoiexternal` git-repos | the externals mechanism + its pin limits | DC-2 |
| **psmux install** | REMOVED from chezmoi scope; stays in `install-deps.ps1` provisioning via `Install-Psmux` and `Add-ScoopBucketSafe` | DC-3 |
| **Windows Terminal `settings.json` merge** (`modify_` read-modify-write) | the JSON-merge mechanic; why a WT *fragment* can't replace it | DC-3 |
| **The parity gate** (old slices vs chezmoi, intersection-scoped, content-normalized + probes) | the keystone — nothing retires until it is green | DC-6 |
| **Hermetic per-OS template unit tests** (`execute-template` on an injected OS var) | host-independent branch coverage | DC-6 |

**Original out of scope (Wave B, catalogued in Appendix A):** the full
`PKG_TABLE`/`$Catalog` -> `.chezmoidata` merge, the 5 non-plugin pinned
binaries/fonts, the zsh login-shell switch, devilspie2, the VSCode theme merge,
the DC-4 secrets tier, and the DC-5 distro matrix remain provisioning/secrets
scope. The current branch has since pulled the rest of the dotfiles config
layer into the DC-6 manifest and added the README/CLAUDE status rewrite.

## Verification status

- **Grounded** against `/tmp/dotfiles-ground` (clone of `luisgui1757/dotfiles`, HEAD `96d85ee`) by a
  5-agent read-through: every `file:line`, pin, sha256, and key count was read from source.
- **chezmoi mechanics web-verified** against chezmoi.io (2026-06). Notable verified facts now baked
  in: default `.ps1` interpreter is `pwsh -NoLogo -File` (so **no custom interpreter block is
  needed**); `.chezmoitemplates/` files are referenced via the **`template` action** (NOT `include`);
  there is **no `.Data` namespace** (`.chezmoidata` vars are `{{ .x }}` at root).
- **Adversarially reviewed** by a 4-lens red-team (chezmoi-correctness, grounding-accuracy,
  Codex-executability, parity-gate soundness). All blockers/majors are folded in (see Provenance);
  the parity gate was substantially reworked to be intersection-scoped and to avoid false-FAILs on a
  correct migration.

## Resolved decisions (do not re-decide)

1. **Per-OS `mode`, copy-default, symlink-on-POSIX via `.chezmoi.toml.tmpl`.** Generated per machine
   (Verified mechanics §3). Windows = `mode = "file"` (kills the Developer-Mode/`CreateSymbolicLink`
   requirement — the genuine Windows win). POSIX = `mode = "symlink"` (live-edit). Path-divergent
   files use explicit `symlink_` entries regardless (global symlink mode is all-or-nothing).
2. **Old scripts are NOT deleted in Wave A.** They run beside chezmoi for the parity gate.
3. **The WT merge stays a full read-modify-write merge** (a `modify_` script), NOT a Fragment
   Extension (it can't set globals/`profiles.defaults`/15 keybindings — grounded
   `settings.fragment.jsonc:24-109`, merge logic `bootstrap.ps1:307-466`).
4. **The zsh external pins to upstream tags, AND a required `run_onchange_after_` asserts the exact
   commit** (DC-2). chezmoi `.chezmoiexternal` git-repo has no native exact-commit pin, and the
   migration's whole point is fidelity, so the commit-equality assert (the old installer's
   `install-deps.sh:786-790` semantics) is promoted into the gate, not logged as a residual. It uses
   `run_onchange_` so pin changes re-assert while ordinary apply/verify remains clean.
5. **Source dir = `home/` inside the dotfiles repo.** Old+new coexist; CI drives both.
6. **The automated CI parity gate now has Ubuntu, macOS, and Windows arms for the migration.** Ubuntu
   remains the networked POSIX baseline for externals; macOS runs the same template/parity/oracle
   scripts on `macos-26`; Windows runs a sandboxed copy-mode + WT merge parity e2e
   (`tests/migration/windows_apply_test.ps1`) with a full apply. The psmux install was removed from
   the chezmoi scope and remains in `install-deps.ps1` per the full-migration owner decision.

---

## Step 0 — Prerequisites & the sandbox contract (do this first)

**Nothing in this spec runs without chezmoi installed and a real sandbox.** `chezmoi --destination`
alone does NOT sandbox — chezmoi still reads its config/state from the real `~/.config/chezmoi` and
`~/.local/share/chezmoi`, and `.chezmoi.toml.tmpl` resolves `.chezmoi.homeDir`/`.chezmoi.os` from the
real environment. The only correct sandbox is to **set `HOME`** for the chezmoi invocation (which
relocates config, state, and the template's home), ideally inside a container.

- **Windows parity-harness prerequisite (DC-3):** `pwsh` must already be on `PATH` (installed by the
  legacy `install-deps.ps1` `$Catalog` `pwsh`/scoop row and modern-shell `Install-One pwsh` call)
  **before** `chezmoi apply` processes the Windows `.ps1.tmpl` modify script, because the default
  `.ps1` interpreter is `pwsh -NoLogo -File`. On a Windows-PowerShell-5.1-only host, install pwsh first or override with
  the FULL form `[interpreters.ps1] command = "powershell" args = ["-NoLogo", "-File"]` (the `args`
  are required — without them chezmoi invokes `powershell <scriptpath>` with the path as a positional
  command token, which does NOT reproduce `pwsh -NoLogo -File`; see §9). No new chezmoi package step lands in Wave A; the
  `$Catalog` to `.chezmoidata` pwsh row stays Wave B.

```sh
# --- Step 0a: install a pinned chezmoi (min v2.52.0 — supports .chezmoiexternal clone.args,
#     modify_ scripts, the `template` action, execute-template --init). ---
CHEZMOI_VERSION=v2.52.0          # pin; bump deliberately
# NOTE: pin the version with `-t <tag>`. A bare trailing arg is treated by the
# get.chezmoi.io installer as a chezmoi command to run after install (it would
# fail with: chezmoi: unknown command "v2.52.0").
sh -c "$(curl -fsLS get.chezmoi.io)" -- -b "$HOME/.local/bin" -t "$CHEZMOI_VERSION"
chezmoi --version                # verify before proceeding

# --- Step 0b: the repo root and the sandbox HOME (define ONCE; used everywhere below). ---
REPO_ROOT="$(git -C . rev-parse --show-toplevel)"   # the dotfiles checkout; run from inside it
SRC="$REPO_ROOT/home"                                # the chezmoi source dir (absolute)
SANDBOX="$(mktemp -d)"                                # a throwaway HOME; never the real one

# --- Step 0c: the canonical sandboxed-apply invocation (use this form throughout). ---
chez() { env HOME="$SANDBOX" "$(command -v chezmoi)" --source "$SRC" "$@"; }
# `init` renders .chezmoi.toml.tmpl into $SANDBOX/.config/chezmoi/chezmoi.toml (no clone — source
# already exists). `apply` materializes the dotfiles under $SANDBOX. On the real host you instead
# run chezmoi against the real $HOME, deliberately, once the pilot is accepted.
```

> On Windows, run the equivalent in a sandbox profile or container; the pilot's Windows
> deliverables (WT, psmux) are verified manually (resolved decision #6).

---

## Verified chezmoi mechanics (the factual backbone) — READ FIRST

Every claim checked against chezmoi.io. The spec uses ONLY these.

1. **Attribute prefixes** (`/reference/source-state-attributes/`, `/reference/target-types/`):
   `dot_`→leading dot; `symlink_`→target is a symlink (body = link target, §2); `private_`→go-rwx;
   `executable_`→+x; `exact_`→**dir mirror, DELETES unmanaged entries** (footgun — never on a dir an
   app writes to); `.tmpl`→body run through the template engine.
2. **`symlink_NAME(.tmpl)`** (`/reference/target-types/`): the file **body** (post-template, trailing
   `\n` stripped) is the **link target**; the **path where the symlink is created is fixed by the
   source entry's name/location**. ⇒ one entry = one fixed path. For "same content, different path
   per OS": one real file + one `symlink_` entry per OS path, each gated by a templated
   `.chezmoiignore`, all bodies → `{{ .chezmoi.sourceDir }}/<path>`.
3. **`mode = "file" | "symlink"`** (`/reference/configuration-file/variables/`,
   `/user-guide/machines/general/`): single config value; **the config can be a `.chezmoi.toml.tmpl`
   generated per-machine**, so per-OS mode IS achievable. `symlink` mode symlinks **every** eligible
   regular non-template file into the source dir — all-or-nothing, not per-file.
4. **`.chezmoiexternal.toml(.tmpl)`** (`/reference/special-files/chezmoiexternal-format/`): types
   `file`/`archive`/`archive-file`/`git-repo`; optional `checksum.sha256`; `refreshPeriod` (default
   `0`=never). **Download + verify ONLY** — no installers/`fc-cache`/root/apt. git-repo accepts
   `clone.args`/`pull.args`. (It pins by ref, not by asserting a commit SHA — see DC-2.)
5. **`.chezmoidata.{yaml,toml,json}`** (`/reference/special-files/chezmoidata-format/`): static (NOT
   templatable); vars merge to the **ROOT** of the template dict — `{{ .x }}`, **never** `{{ .Data.x }}`
   (no `.Data` namespace). Config `[data]` overrides.
6. **`.chezmoiignore`** (`/reference/special-files/chezmoiignore/`): **always a template**; patterns
   are **target-relative** (no `dot_` prefixes), `doublestar.Match`, `!` re-includes.
7. **`.chezmoi.toml.tmpl` + prompts** (`/reference/commands/init/`): `promptString`/`promptBool`
   always prompt; `promptStringOnce`/`promptBoolOnce` prompt only if absent (pass `.` + dotted path).
   Init-only.
8. **`run_once_`/`run_onchange_`/`run_before_`/`run_after_` (+ `.tmpl`)** (`/reference/target-types/`):
   order = ASCII filename within phase; hash = of the **rendered** contents; `run_once_` records its
   hash **only on success** (failed scripts retry). Force a re-run with `{{ include "file" | sha256sum }}`
   in a comment. **Empty-script idiom (load-bearing for OS-gating):** a script whose rendered contents
   are only whitespace is **NOT executed** (`/user-guide/use-scripts-to-perform-actions/` — "useful for
   disabling scripts dynamically"). So a `{{ if eq .targetOS "windows" }}…{{ end }}` guard that renders
   empty off-Windows disables the script per-OS with **no `.chezmoiignore` needed** and **no interpreter
   (e.g. pwsh) invoked on the wrong OS**. Prove it: `chezmoi --source <fixture-os=linux> execute-template`
   of each `.ps1.tmpl` renders empty/whitespace.
9. **`modify_` scripts** (`/reference/target-types/`): a filter — **current target on stdin, desired
   on stdout**. For an interpreted modify script, name it `modify_<target>.<ext>.tmpl`: chezmoi strips
   `.tmpl`, then the next extension (`.ps1`) **selects the interpreter and is stripped from the target
   name** (target = `<target>`). **Default `.ps1` interpreter = `pwsh -NoLogo -File <scriptpath>`**
   (`/reference/configuration-file/interpreters/`) — no custom `[interpreters]` block needed (pwsh
   must be installed; for Windows-PowerShell-only hosts override with
   `[interpreters.ps1] command="powershell" args=["-NoLogo","-File"]`).
10. **`.chezmoitemplates/`** (`/user-guide/templating/`): files there are parsed as templates and
    referenced via the **`template` action** `{{ template "<relpath>" . }}` (pass `.` for data); plain
    JSON with no template syntax is emitted verbatim. (`include` reads a path relative to the source
    dir and is NOT the way to read a `.chezmoitemplates/` entry — use `template`.)
11. **`execute-template`** (`/reference/commands/execute-template/`): `--init`/`--promptString k=v`
    simulate init prompts but **cannot spoof `.chezmoi.os`/`.arch`**. Hermetic per-OS tests render
    against a **fixture sourceDir whose `.chezmoidata.yaml` sets an injected var** (e.g. `targetOS`);
    production templates funnel `.chezmoi.os` into that var in ONE place.
12. **`managed [--format json]` / `verify` / `doctor`**: `managed` = owned entries (`--include=dirs`
    etc.); `verify` = exit 0/1 on whether on-disk **target state** matches source — **does NOT check
    installer side effects** (the DC-6 reason for non-file probes); `doctor` = env/tooling health.

---

## DC-1 — Source tree, per-OS mode, tmux single-source, path-divergent symlink

Maps `ROADMAP.md:389-423`.

### Step 1 — Source tree skeleton (run from `$REPO_ROOT`)

Create every source subdir explicitly (chezmoi does not create source dirs for you). Note the
literal space in `Application Support` is the real macOS dir name — **do not escape or rename it**.

```sh
cd "$REPO_ROOT"
mkdir -p \
  "$SRC/.chezmoitemplates/lazygit" \
  "$SRC/.chezmoitemplates/windows-terminal" \
  "$SRC/dot_config/lazygit" \
  "$SRC/Library/Application Support/lazygit" \
  "$SRC/AppData/Local/lazygit" \
  "$SRC/AppData/Local/Packages/Microsoft.WindowsTerminal_8wekyb3d8bbwe/LocalState" \
  "$SRC/.chezmoiscripts"
```

Resulting layout (every path below is created/filled by a step in this DC):

```
home/                                   # = $SRC (chezmoi sourceDir)
  .chezmoi.toml.tmpl                     # Step 2
  .chezmoiignore                         # Steps 3,4 (templated)
  .chezmoiexternal.toml.tmpl             # DC-2
  .chezmoitemplates/
    lazygit/config.yml                   # Step 4: the ONE real lazygit file
    windows-terminal/settings.fragment.jsonc   # DC-3 Step 1: the WT fragment
  dot_tmux.conf                          # Step 3
  dot_tmux.windows.conf                  # Step 3 (Windows-gated)
  dot_config/lazygit/symlink_config.yml.tmpl                 # Step 4 (Linux/WSL)
  Library/Application Support/lazygit/symlink_config.yml.tmpl # Step 4 (macOS)
  AppData/Local/lazygit/config.yml.tmpl                      # Step 4 (Windows copy)
  AppData/Local/Packages/Microsoft.WindowsTerminal_8wekyb3d8bbwe/LocalState/modify_settings.json.ps1.tmpl  # DC-3 Step 1
```

Smoke-test the sandbox plumbing once the files exist: `chez init && chez apply --dry-run --verbose`
(uses the `chez()` function from Step 0c; CWD is irrelevant because `$SRC` is absolute).

### Step 2 — `home/.chezmoi.toml.tmpl` (per-OS mode + the OS funnel var + a seed prompt)

`.targetOS` is the **single funnel point** that lets `execute-template` unit tests inject an OS
(Verified mechanics §11). No `[interpreters]` block — the default `pwsh -NoLogo -File` is correct
(Verified mechanics §9). **No interactive prompt in the pilot** — DC-4's per-machine/secrets prompts
are Wave B, so non-interactive `chez init` / CI `chezmoi init` never blocks. **`mode` MUST appear
before any `[table]` header**, or TOML parses it as `data.mode` (NOT chezmoi's top-level `mode`) and
POSIX silently stays in `file` mode — which would break *every* symlink and the parity gate.

```toml
# home/.chezmoi.toml.tmpl
{{- $os := .chezmoi.os -}}
{{- /* top-level `mode` FIRST — before [data] — or it becomes data.mode and is ignored. */ -}}
{{- if eq $os "windows" }}
mode = "file"      # no symlink privilege / Developer-Mode needed
{{- else }}
mode = "symlink"   # live-edit
{{- end }}

[data]
    targetOS = {{ $os | quote }}     # the ONE place production templates read the OS; tests override it
```

> Verify after rendering: `chez init && grep -E '^mode' "$SANDBOX/.config/chezmoi/chezmoi.toml"`
> must show a **top-level** `mode = "symlink"` on POSIX (not nested under `[data]`).

### Step 3 — tmux: single source + Windows overlay

`tmux.conf` → `~/.tmux.conf` on **every** OS (`bootstrap.sh:204`, `bootstrap.ps1:277`) — the trivial
case. `tmux.windows.conf` is linked **only on Windows** (`bootstrap.ps1:281`; absent in
`bootstrap.sh` — `tmux.conf:132` sources it with `source-file -q`, a silent no-op on POSIX). Gate it
to Windows for exact parity (no stray POSIX file).

```sh
cp "$REPO_ROOT/tmux/tmux.conf"         "$SRC/dot_tmux.conf"            # not a template
cp "$REPO_ROOT/tmux/tmux.windows.conf" "$SRC/dot_tmux.windows.conf"
```

`home/.chezmoiignore` (templated; target-relative — Verified mechanics §6):

```
{{- if ne .targetOS "windows" }}
.tmux.windows.conf
{{- end }}
```

- Under POSIX `mode=symlink`, `dot_tmux.conf` → symlink `~/.tmux.conf` into the source dir
  (live-edit). On Windows it is copied. Verify (host-bound): on the running OS, `chez managed | grep
  tmux` shows `.tmux.conf` everywhere and `.tmux.windows.conf` only on Windows. **Cross-OS branch
  proof comes from the Step-5 `execute-template` tests, not `managed`** (which reflects the host's
  real `.chezmoi.os`).

### Step 4 — lazygit: the path-divergent symlink (the hard case, worked end-to-end)

Per-OS target paths (grounded): macOS `~/Library/Application Support/lazygit/config.yml`
(`bootstrap.sh:215-216`); Linux/WSL `~/.config/lazygit/config.yml` (`bootstrap.sh:220,229`); Windows
`%LOCALAPPDATA%\lazygit\config.yml` = `~/AppData/Local/lazygit/config.yml` (`bootstrap.ps1:289`). One
30-line source file, byte-identical across OSes; only the path differs. Per Verified mechanics §2 one
`symlink_` entry can't serve three paths, so: **one real file in `.chezmoitemplates/`, one `symlink_`
entry per POSIX path (live-edit), a copied template on Windows.**

```sh
cp "$REPO_ROOT/lazygit/config.yml" "$SRC/.chezmoitemplates/lazygit/config.yml"
```

> **Single-source obligation (pilot-only):** while the old scripts coexist, this
> `.chezmoitemplates/lazygit/config.yml` must stay **byte-identical** to the canonical
> `lazygit/config.yml` (the old scripts still symlink the canonical). The DC-6 gate asserts this
> equality. In Wave C the canonical moves into chezmoi and there is genuinely one copy.

`home/dot_config/lazygit/symlink_config.yml.tmpl` (Linux/WSL — body = link target = the real file in
the source clone, giving live-edit):

```
{{ .chezmoi.sourceDir }}/.chezmoitemplates/lazygit/config.yml
```

`home/Library/Application Support/lazygit/symlink_config.yml.tmpl` (macOS — same body):

```
{{ .chezmoi.sourceDir }}/.chezmoitemplates/lazygit/config.yml
```

`home/AppData/Local/lazygit/config.yml.tmpl` (Windows — real **copied** file; body emits the shared
fragment via the `template` action, so still single-source — Verified mechanics §10):

```
{{ template "lazygit/config.yml" . }}
```

Gate each entry to its OS in `home/.chezmoiignore` (append; target-relative paths). **WSL is reported
as `os=linux`** by chezmoi, so the `linux` clause covers it — there is **no `wsl` value of `.targetOS`
in the pilot**. WSL-specific Ghostty opt-in behavior and devilspie2 still need Wave-B funnel logic.

```
{{- if ne .targetOS "darwin" }}
Library/Application Support/lazygit/config.yml
{{- end }}
{{- if ne .targetOS "linux" }}
.config/lazygit/config.yml
{{- end }}
{{- if ne .targetOS "windows" }}
AppData/Local/lazygit/config.yml
{{- end }}
```

- **Live-edit check (POSIX):** `readlink ~/.config/lazygit/config.yml` →
  `…/home/.chezmoitemplates/lazygit/config.yml`; editing through it edits the source clone → `git diff`
  shows it; `chez apply` is a no-op (the symlink body is unchanged).
- **ghostty is the identical pattern** (macOS `~/Library/Application Support/com.mitchellh.ghostty/config`
  vs Linux `~/.config/ghostty/config`, grounded in `bootstrap.sh`; no Windows target) and is now part
  of the Wave A manifest.

### DC-1 Acceptance (maps `ROADMAP.md:407-423`)

Status note (2026-06-09): DC-1 source files landed and were expanded beyond the
seed scope. The manifest now covers nvim as a directory symlink, Starship,
zshenv/zshrc, Ghostty, lazygit, tmux, `tmux.windows.conf`, the Windows
PowerShell profile copy, and the Windows Terminal merge. Windows copy-mode and
WT merge semantics are covered by `tests/migration/windows_apply_test.ps1`;
psmux installation is deliberately not a chezmoi responsibility.

- [ ] `chez apply` (sandbox) lands `~/.tmux.conf` on macOS/Linux/Windows; `~/.tmux.windows.conf` only
      on Windows.
- [ ] lazygit config lands at the correct per-OS absolute path; bodies byte-identical (one source).
- [ ] On POSIX the lazygit + tmux configs are **symlinks** (live-edit); on Windows real files (no
      `CreateSymbolicLink`/Developer-Mode).
- [ ] `chez verify` clean; second `chez apply` is a no-op.

---

## DC-2 — One external: the zsh plugins as pinned `.chezmoiexternal` git-repos + commit-assert

Maps `ROADMAP.md:425-453`. The two zsh plugins are the cleanest pure-external fit (git-repos, no
post-install side effect — `install-deps.sh:737-826`). The 5 side-effecting binary/font installers
are Wave B (Appendix A) precisely because externals can't run their `fc-cache`/`/opt`/apt steps
(Verified mechanics §4).

### Step 1 — `home/.chezmoiexternal.toml.tmpl`

Pins from `install-deps.sh:26-29`; URLs `:816,:822`; install path = `zsh_plugin_root`
(`install-deps.sh:725-727`).

```toml
# home/.chezmoiexternal.toml.tmpl  — POSIX only (zshrc sources these)
{{- if ne .targetOS "windows" }}
[".local/share/dotfiles/zsh-plugins/zsh-autocomplete"]
    type = "git-repo"
    url = "https://github.com/marlonrichert/zsh-autocomplete.git"
    refreshPeriod = "0"
    [".local/share/dotfiles/zsh-plugins/zsh-autocomplete".clone]
        args = ["--depth", "1", "--branch", "25.03.19"]   # install-deps.sh:26

[".local/share/dotfiles/zsh-plugins/zsh-autosuggestions"]
    type = "git-repo"
    url = "https://github.com/zsh-users/zsh-autosuggestions.git"
    refreshPeriod = "0"
    [".local/share/dotfiles/zsh-plugins/zsh-autosuggestions".clone]
        args = ["--depth", "1", "--branch", "v0.7.1"]     # install-deps.sh:28
{{- end }}
```

### Step 2 — Required commit-assert `home/.chezmoiscripts/run_onchange_after_20-verify-zsh-plugin-pins.sh.tmpl`

chezmoi pins by ref, not by asserting a commit (Verified mechanics §4). The old installer **fails**
on commit mismatch (`install-deps.sh:786-790`). To keep that fidelity (resolved decision #4), assert
the exact pinned commits after the externals are fetched. Use `run_onchange_after_` (it re-fires when
the embedded pins change), NOT a bare `run_after_`: a `run_after_` re-runs on EVERY apply, which makes
`chezmoi verify` perpetually report a pending change — verify-clean is the more valuable invariant, and
the parity gate asserts it. The externals are `refreshPeriod = "0"` (never auto-refetched), so a
checkout cannot drift on its own; the pin is asserted on first apply and on every pin change. (Manual
`git checkout` tampering inside a plugin dir without a pin change is out of scope — `chezmoi verify`
does not inspect external commit HEADs either way.) This keeps re-apply a true no-op AND verify clean.
The oracle test (`tests/migration/oracle_test.sh`) still proves the assert FIRES by rendering and
running it against a corrupted checkout.

```sh
# home/.chezmoiscripts/run_onchange_after_20-verify-zsh-plugin-pins.sh.tmpl
{{- if ne .targetOS "windows" -}}
#!/usr/bin/env bash
set -euo pipefail

# Externals use this fixed managed root; an XDG-aware root is Wave B.
root="$HOME/.local/share/dotfiles/zsh-plugins"

# Bash 3.2-safe: no associative arrays, no mapfile, no namerefs.
for pair in \
    "zsh-autocomplete a76f26ae25528e76ee53df98ad38fbacdf89fd2e" \
    "zsh-autosuggestions e52ee8ca55bcc56a17c828767a3f98f22a68d4eb"; do
    name="${pair%% *}"
    want="${pair##* }"
    got="$(git -C "$root/$name" rev-parse HEAD 2>/dev/null || true)"
    if [[ "$got" != "$want" ]]; then
        echo "FAIL: $name HEAD ${got:-unknown} != pinned $want" >&2
        exit 1
    fi
done
{{- end -}}
```

> If a future chezmoi cannot clone the tag to the exact pinned commit (e.g. upstream re-tagged), this
> script FAILS the apply — the correct, parity-preserving behavior. (The tags are date-/semver-
> versioned and immutable today, so this passes now.)

### DC-2 Acceptance (maps `ROADMAP.md:451-453`)

- [ ] `chez apply` clones both plugins to `~/.local/share/dotfiles/zsh-plugins/<name>`; the
      commit-assert script passes (`git rev-parse HEAD` equals the pinned commit on each).
- [ ] The `shells/zshrc` loader lines (`zshrc:68`) still resolve (path unchanged).
- [ ] On Windows both the external block and the assert script are skipped; `chez verify` clean.

2026-06-09 implementation note: the two source files exist in `home/`, render empty for Windows via
`execute-template`, render parseable POSIX TOML/script bodies with the current `install-deps.sh` pins,
and pass offline parse validation. Live external clone plus commit-assert execution is intentionally
deferred to networked validation.

---

## DC-3 — Windows Terminal merge; psmux stays provisioning

Maps `ROADMAP.md:455-487`. The full-migration owner decision is
`chezmoi=dotfiles, install-deps=provisioning`, so psmux install was removed
from the chezmoi scope and stays in `install-deps.ps1` (`Install-Psmux` with
the hardened `Add-ScoopBucketSafe` path). Chezmoi keeps the psmux-readable
config (`.tmux.conf` plus `.tmux.windows.conf`) and the Windows Terminal
`modify_` merge, which are config-layer work.

### Step 1 — Windows Terminal: `modify_settings.json.ps1.tmpl` (the merge)

The WT `settings.json` is **app-owned** (the user/app writes profiles, `defaultProfile`, …); we
merge our keys without clobbering theirs — the `modify_` contract (Verified mechanics §9). A Fragment
Extension **cannot** do this (it can't set the 7 globals + `profiles.defaults` + 15 keybindings —
`settings.fragment.jsonc:24-109`, merge logic `bootstrap.ps1:307-466`). Stable Store-WT path is static
(`Microsoft.WindowsTerminal_8wekyb3d8bbwe` — `bootstrap.ps1:296`).

The legacy `bootstrap.ps1` merge is now also default-on, so the chezmoi
`modify_` default-on behavior is intentional parity rather than a new
divergence.

> **Wave A scopes to the stable Store WT only.** The old code also handles WT **Preview**
> (`Microsoft.WindowsTerminalPreview_8wekyb3d8bbwe`) and discovers the path via `%LOCALAPPDATA%`
> (`bootstrap.ps1:295-299`); Preview and a redirected `%LOCALAPPDATA%` are **Wave B**. Because the
> managed target is a fixed `~/AppData/Local/...` path, the Windows parity harness (DC-6 Step 4)
> runs in a throwaway Windows profile with `USERPROFILE`/`LOCALAPPDATA`/`APPDATA` pointed at the
> sandbox, or the old (`%LOCALAPPDATA%`) and new (`~/AppData/Local`) sides write different files.

The fragment lives in `.chezmoitemplates/` and is embedded via the **`template` action** (Verified
mechanics §10) so it stays single-source:

```sh
cp "$REPO_ROOT/windows-terminal/settings.fragment.jsonc" \
   "$SRC/.chezmoitemplates/windows-terminal/settings.fragment.jsonc"
```

```powershell
# home/AppData/Local/Packages/Microsoft.WindowsTerminal_8wekyb3d8bbwe/LocalState/modify_settings.json.ps1.tmpl
{{- if ne .targetOS "windows" -}}
{{- /* off-Windows: also .chezmoiignore'd; emit nothing */ -}}
{{- else -}}
$ErrorActionPreference = 'Stop'
$current = [Console]::In.ReadToEnd()           # modify_ contract: current settings.json on stdin
$fragmentJson = @'
{{ template "windows-terminal/settings.fragment.jsonc" . }}
'@
function Strip-Jsonc([string]$s) { ($s -split "`n" | Where-Object { $_ -notmatch '^\s*//' }) -join "`n" }
$cur  = if ([string]::IsNullOrWhiteSpace($current)) { [pscustomobject]@{} } else { (Strip-Jsonc $current) | ConvertFrom-Json }
$frag = (Strip-Jsonc $fragmentJson) | ConvertFrom-Json

# --- Port ALL SIX helper functions VERBATIM from bootstrap.ps1:329-447 (they are nested there;
#     paste them at top level here). They are INTERDEPENDENT — Get-ArrayValue is required by the
#     two Merge-* helpers, so do NOT omit it:
#       Set-OrAdd-Property           bootstrap.ps1:329-336
#       Get-ArrayValue               bootstrap.ps1:337-343   (REQUIRED by the two Merge-* below)
#       Merge-ObjectArrayByProperty  bootstrap.ps1:344-375
#       Get-WTActionKeySet           bootstrap.ps1:376-392
#       Test-WTActionKeyOverlap      bootstrap.ps1:393-403
#       Merge-WTActions              bootstrap.ps1:404-447
# (Paste the six function definitions here, unedited.)

# --- Then run the EXACT merge sequence from bootstrap.ps1:448-463 against $cur / $frag (operate on
#     $cur, merge from $frag). Merge ONLY the managed subset; do NOT propagate the fragment's
#     top-level $schema (bootstrap.ps1 does not — parity):
foreach ($k in 'copyFormatting','copyOnSelect','firstWindowPreference','initialRows','theme','useAcrylicInTabRow','windowingBehavior') {
  if ($null -ne $frag.$k) { Set-OrAdd-Property $cur $k $frag.$k }                 # 7 globals  (:448-454)
}
if ($null -eq $cur.profiles.defaults) {
  $cur.profiles | Add-Member -NotePropertyName defaults -NotePropertyValue $frag.profiles.defaults -Force
} else { $cur.profiles.defaults = $frag.profiles.defaults }                       # profiles.defaults (:456-460)
Set-OrAdd-Property $cur "actions" @(Merge-WTActions $cur.actions $frag.actions)                       # 15 keybinds (:461)
Set-OrAdd-Property $cur "schemes" @(Merge-ObjectArrayByProperty $cur.schemes $frag.schemes "name")    # (:462)
Set-OrAdd-Property $cur "themes"  @(Merge-ObjectArrayByProperty $cur.themes  $frag.themes  "name")    # (:463)
$cur | ConvertTo-Json -Depth 100               # stdout becomes the new settings.json (modify_ contract)
{{- end -}}
```

> **Codex verification (modify_ target name):** after building this, run (on Windows/sandbox)
> `chez managed | grep settings.json` and confirm the managed target is `…/LocalState/settings.json`
> (the `.ps1` selects the pwsh interpreter and is stripped, the `.tmpl` is stripped first — Verified
> mechanics §9). If your chezmoi version yields `settings.json.ps1`, switch to the
> `chezmoi:modify-template` directive form (template, `.chezmoi.stdin`) — but the external pwsh form
> above is preferred because it reuses the proven `bootstrap.ps1` merge functions.

### DC-3 Acceptance (maps `ROADMAP.md:485-487`)

Implementation status in this checkout (2026-06-09): DC-3 source files have been built and POSIX
render-gating is validated from macOS with fixture `.chezmoidata.yaml` values. Current Windows CI runs a
full apply for copy-mode and WT merge parity. psmux install is deliberately outside chezmoi scope.

- [ ] On Windows, `chez apply` merges the WT fragment into a live `settings.json`
      **without dropping** pre-existing profiles/keys.
- [ ] The merged `settings.json` contains exactly the managed key set — 7 globals, 15 keybindings,
      `profiles.defaults`, and the named rose-pine scheme+theme — and **not** a stray `$schema`
      (compare the merged KEY SET, not a raw diff against the fragment).
- [ ] Re-running `chez apply` is a no-op (stable modify output).
- [ ] On POSIX both are skipped (`.chezmoiignore`); `chez verify` clean.

---

## DC-6 — The parity gate (the keystone) + hermetic template tests

Maps `ROADMAP.md:540-582`. **Nothing in the old install path is deleted until this is green for N
runs (DC-6 decision-gate item).** The gate is manifest-scoped to the full migrated config set — it is
NOT a raw union of two full installs (a full `setup.sh --all` would produce package, font, login-shell,
daemon, and other side effects that the pilot legitimately does not manage, false-FAILing the gate).
It runs only the relevant config slices of the old path and compares only the manifest rows.

### Step 1 — Run the pilot slices of the OLD path (no full install, no out-of-scope footprint)

In the Ubuntu container, into `$HOME_OLD=$(mktemp -d)`:

```sh
HOME_OLD="$(mktemp -d)"
# tmux + lazygit symlinks ONLY (bootstrap.sh is the symlink layer; it installs no packages,
# no devilspie2, no fonts). Intersection-scoping (Step 3) ignores any non-pilot links it makes.
env HOME="$HOME_OLD" "$REPO_ROOT/bootstrap.sh"
# zsh plugins ONLY — source install-deps.sh and call the single function (avoids the full installer).
# YES_ALL=1 is REQUIRED: install_zsh_plugins gates on ask() (install-deps.sh:199-207), which
# auto-accepts only when YES_ALL=1 or DRY_RUN=1; without it a non-interactive gate skips the plugins
# and P3/P4 false-FAIL. CRITICAL: set YES_ALL=1 *AFTER* the source, not as an env var — install-deps.sh
# initializes YES_ALL=0 at the top (install-deps.sh:17) at source time, which clobbers an env-passed
# value. (Verified by running parity_gate.sh: the env form skips the plugins.)
env HOME="$HOME_OLD" INSTALL_DEPS_SOURCE_ONLY=1 bash -c \
  'source "'"$REPO_ROOT"'/install-deps.sh"; YES_ALL=1; install_zsh_plugins'   # install-deps.sh zsh slice
```

> This deliberately does NOT run `install-deps.sh --all` / `setup.sh` — so no fonts, login-shell,
> devilspie2, packages, or other Wave-B footprint is produced. The gate therefore needs **no giant
> ignore-list**; it only ever looks at the manifest rows (Step 3).

### Step 2 — Apply chezmoi into a separate sandbox

```sh
HOME_NEW="$(mktemp -d)"
env HOME="$HOME_NEW" chezmoi --source "$SRC" init
env HOME="$HOME_NEW" chezmoi --source "$SRC" apply
```

### Step 3 — The manifest-scoped comparison (the gate)

The gate is now manifest-driven, not a two-path allow-list. `tests/migration/parity_gate.sh` defines
one row manifest with: label, type, darwin target, linux target, canonical repo source, and the
`home/` source copy when one exists. The manifest covers every config migrated in Wave A:

```
tmux.conf, tmux.windows.conf, lazygit config, nvim, starship, zshenv, zshrc,
ghostty config, powershell profile
```

Rows with empty POSIX targets are Windows-only for applied-state parity but still participate in the
single-source copy assertion. The host OS selects only the darwin/linux target column for OLD-vs-NEW
applied parity.

For each active manifest row on the host OS:

```
for config-file rows:
    assert exists(HOME_OLD/p) and exists(HOME_NEW/p)
    assert both sides are symlinks on POSIX
    assert sha(deref(HOME_OLD/p)) == sha(deref(HOME_NEW/p))
    assert mode(deref(HOME_OLD/p)) == mode(deref(HOME_NEW/p))

for the nvim row:
    assert both sides are directory symlinks
    assert both dereference to realpath(REPO_ROOT/nvim)
    assert diff -r of the dereferenced trees is empty

for every non-nvim row with a home source copy:
    assert sha(REPO_ROOT/source) == sha(REPO_ROOT/home/source-copy)

for P3/P4:
    assert git_head(HOME_OLD/plugin) == git_head(HOME_NEW/plugin)
```

Portable helpers (the gate runs in the Ubuntu container — GNU coreutils — but these keep a manual
macOS run working):

```sh
# Guard on command availability, NOT on pipe exit codes (a piped `|| ` keys off cut's status, not the hasher's).
sha()  { if command -v sha256sum >/dev/null 2>&1; then sha256sum "$1" | cut -d' ' -f1; else shasum -a 256 "$1" | cut -d' ' -f1; fi; }
mode() { if stat -c '%a' "$1" >/dev/null 2>&1; then stat -c '%a' "$1"; else stat -f '%Lp' "$1"; fi; }   # GNU vs BSD/macOS
deref(){ if readlink -f "$1" >/dev/null 2>&1; then readlink -f "$1"; else python3 -c 'import os,sys;print(os.path.realpath(sys.argv[1]))' "$1"; fi; }
```

The gate **FAILS** on any active manifest path that is missing on one side, has the wrong type,
differs in dereferenced content/permissions, resolves `nvim` anywhere other than repo `nvim/`, or
(plugins) differs in HEAD commit. After apply it also asserts wrong-OS directories are absent (for
example, no `~/AppData`, no POSIX-wrong Ghostty/lazygit path, and no `~/Documents` on darwin/linux).

### Step 4 — Non-file probes (POSIX arm) — what `chez verify` cannot see

`verify` checks target state only (Verified mechanics §12). The pilot's only POSIX side effect is the
plugin checkouts, already covered by the HEAD-commit assert in Step 3. **The Windows-only checks do
not run on the Ubuntu arm.** The Windows CI arm now covers full apply, copy-mode
content, and WT merge parity. psmux installation is not a migration probe
because it remains `install-deps.ps1` provisioning:

- **WT merge (Windows CI) — value-level, not key-presence:** in a throwaway Windows profile with
  `USERPROFILE`/`LOCALAPPDATA`/`APPDATA` pointed at the sandbox (so the old `%LOCALAPPDATA%` path and the
  new `~/AppData/Local` path are the same file — see DC-3 Step 1), seed an identical baseline
  `settings.json` (e.g. WT's default + one user profile) into two sandboxes; in one run the OLD
  `bootstrap.ps1` with no switch (`-MergeWindowsTerminal` is accepted as a back-compat alias), in the
  other `chez apply`; then **deep-compare the two resulting JSONs** (normalize: sort keys/arrays
  canonically) and assert structural equality of the managed subset (7 globals + all 15 keybindings +
  `profiles.defaults` + named scheme/theme), assert the exact managed action-key set from
  `windows-terminal/settings.fragment.jsonc`, and assert that the user's seeded
  profile/`defaultProfile` survived in both. (Key *presence* or count alone is insufficient — a merge
  that dropped keybindings or clobbered `defaultProfile` could still have `.actions`.) Implemented in
  `tests/migration/windows_apply_test.ps1`, which first tries the real `bootstrap.ps1` path, reports
  whether it fell back, and falls back to the scoped legacy WT merge only if the runner cannot complete
  the symlink phase.

- **Windows config content parity (CI):** the Windows arm applies without excluding scripts, then checks
  `.tmux.conf`, `.tmux.windows.conf`, lazygit, Starship, and the PowerShell profile as real copy-mode
  files whose SHA-256 matches the canonical repo source. `AppData/Local/nvim` is intentionally a
  directory symlink to repo `nvim/`, and the test verifies both the link target and `init.lua` content.

### Step 5 — Hermetic per-OS template unit tests (runnable on ANY host)

`execute-template` can't spoof `.chezmoi.os` (Verified mechanics §11), and production templates read
`.targetOS`, so tests render against a **fixture sourceDir whose `.chezmoidata.yaml` sets `targetOS`**:

```sh
# tests/migration/template_test.sh — runs on any host, no real apply, no prompts.
fixture="$(mktemp -d)"
for os in darwin linux windows; do
  printf 'targetOS: %s\n' "$os" > "$fixture/.chezmoidata.yaml"
  out="$(chezmoi --source "$fixture" execute-template \
        '{{ if ne .targetOS "windows" }}KEEP{{ else }}DROP{{ end }}')"
  case "$os" in
    windows) [ "$out" = "DROP" ] || { echo "FAIL tmux.windows gate ($os)"; exit 1; } ;;
    *)       [ "$out" = "KEEP" ] || { echo "FAIL tmux.windows gate ($os)"; exit 1; } ;;
  esac
  # Likewise assert exactly ONE lazygit path is active per OS by rendering the .chezmoiignore body.
done
```

These are **added alongside** the installer-level `pkg_for`/OS-branch shell tests (`tests/shell/*`),
which stay green during Wave A (old scripts untouched) and **retire only in Wave C** — not now (Wave
A must not delete coverage while the old path is still part of the parity gate). The **config-level**
tests survive unchanged — `tests/nvim/spec/*`,
`tests/starship/*` (incl. the 150 ms CI perf budget `perf_test.sh:18-19`), `tests/tmux/*`,
`tests/ghostty/*` test the *applied* config.

### Step 5b — Negative oracle tests (prove the guards can fail)

`tests/migration/oracle_test.sh` runs two POSIX-only destructive checks in separate sandbox homes:

- corrupt `zsh-autocomplete` to a different real commit, re-render the
  `run_onchange_after_20-verify-zsh-plugin-pins.sh.tmpl` commit assert, and require the
  `FAIL: zsh-autocomplete HEAD ...` line plus a nonzero exit;
- replace the managed `~/.tmux.conf` symlink with divergent regular-file content and require
  `chezmoi verify` to exit nonzero.

This stays in the existing `chezmoi-parity` job. It needs network because each sandbox performs a
real `chezmoi apply` and the commit-corruption case deepens the cloned zsh-autocomplete checkout.

### Step 6 — CI wiring (reuse, don't rewrite)

- The existing container E2E hard-fails on any non-apt PM (`container-e2e.sh:90-101`), so the pilot
  parity gate runs on the **existing `apt`/Ubuntu** entry only; distro-matrix widening is Wave B/DC-5
  (each distro needs a matrix entry **and** a root-prep arm **and** a `required_checks_test.sh`
  update — `e2e-install.yml:43`, `required_checks_test.sh:27`). Do not attempt it in Wave A.
- **Add (don't rename) CI jobs.** The original gate is `test.yml` job `chezmoi-parity`; the macOS arm
  is `chezmoi-parity-macos`; the Windows arm is `chezmoi-parity-windows`. Their required-status
  contexts are the **bare job names** (`required_checks_test.sh:16-48` extracts `test.yml` job
  contexts as bare keys; only `e2e-install.yml` matrix jobs get `<group> / <id>` prefixes). Mirror the
  exact strings in all three sync files in the SAME commit: `.github/settings.yml`,
  `scripts/apply-repo-safeguards.sh`, `.github/rulesets/main-integrity.json` — else
  `required_checks_test.sh` (and branch protection) breaks.
- Leave `tests/static/invariants_test.sh:98` (the `.\test.ps1` Windows-entrypoint invariant) intact —
  the pilot adds, not replaces.

### DC-6 Acceptance (maps `ROADMAP.md:580-582`)

- [ ] Step-5 template tests pass on a single host for all three OS values (tmux + lazygit branches).
- [ ] A second `chez apply` is a no-op; `chez verify` + `chez doctor` clean.
- [ ] Negative oracle tests prove the zsh exact-commit assert and `chez verify` drift guard fail when
      deliberately corrupted.
- [ ] The Step-3 intersection gate passes (dereferenced content + resolved-permission parity for P1/P2;
      exact-commit equality for P3/P4; the single-source equality assert) on the Ubuntu arm for **N
      consecutive runs** (N fixed in the decision gate) before any old script is considered for
      retirement (retirement = Wave C).
- [ ] The Windows CI arm passes full apply, copy-mode content checks, and WT value-level deep-compare.

Status note (2026-06-09): DC-6 harness and CI wiring landed in this checkout:
`tests/migration/template_test.sh`, `tests/migration/parity_gate.sh`,
`tests/migration/windows_render_test.sh`, `tests/migration/oracle_test.sh`, the
`chezmoi-parity` job, and required-check sync. `parity_gate.sh` is manifest-driven for the full
migrated config set, asserts second-apply idempotency, `chezmoi verify`, no `error` rows from
`chezmoi doctor`, single-source byte equality for every copied config, and wrong-OS applied-state
absence. `windows_apply_test.ps1` checks Windows copy-mode content, nvim dir-symlink content, exact WT
managed action keys, and reports whether the Part-2 legacy-bootstrap fallback was used. Static repo
walkers exclude `home/` managed copies; those copies are validated by the parity gate, while canonical
top-level configs remain linted. The live networked `parity_gate.sh` and `oracle_test.sh` runs remain
separate because P3/P4 clone external zsh-plugin repos.

2026-06-09 CI expansion note: `test.yml` now also has `chezmoi-parity-macos`
(`macos-26`, running template/parity/oracle scripts) and `chezmoi-parity-windows`
(`windows-2025`, running `tests/migration/windows_apply_test.ps1`). The Windows
script seeds a throwaway profile, applies chezmoi without excluding scripts, asserts
Windows copy-mode and WT merge preservation, then deep-compares the managed WT
subset against the legacy merge path. The required-check contexts were added to
`.github/settings.yml`, `scripts/apply-repo-safeguards.sh`, and
`.github/rulesets/main-integrity.json` in the same change.

---

## Pilot decision gate — evaluate before committing to Wave B

After the pilot is green, the owner judges (the point of Wave A — `ROADMAP.md:583-588`):

1. **Ergonomics:** does the copy-vs-symlink split + the `.chezmoitemplates`-anchored path-divergent
   symlink feel maintainable, or is it more ceremony than `bootstrap.sh link()`?
2. **Go-template verbosity:** readable at the scale of 28 configs?
3. **Parity-gate cost:** the gate is a **migration oracle**, not a test — is the manifest-driven full
   migrated config set worth maintaining, or is the two-script status quo fine?
4. **Pin fidelity:** is the tag-pin + commit-assert combo acceptable, or do you want exact-commit
   externals natively (a stronger chezmoi feature/version)?
5. **Green bar (the quantitative gate):** retirement (Wave C) is authorized only after **N = 10
   consecutive green Ubuntu parity runs + green macOS/Windows CI parity arms**,
   where "green" = {dereferenced content parity, type-per-model, externals exact-commit
   equality, WT structural deep-compare, single-source equality}. Until that bar is met, the old
   scripts stay.

Only if 1–4 clear and the bar in 5 is achievable does Wave B begin.

---

## Acceptance checklist (consolidated)

- [ ] **Step 0** pinned chezmoi installed (`chezmoi --version` ≥ v2.52.0); sandbox via `HOME=$SANDBOX`
      (not `--destination` alone).
- [ ] **DC-1** tmux single-source on all OSes; tmux.windows.conf Windows-only; lazygit at the correct
      per-OS path; POSIX symlinks (live-edit) / Windows copies; verify clean; re-apply no-op.
- [ ] **DC-2** both zsh plugins cloned to the unchanged path; the commit-assert passes; skipped on
      Windows.
- [ ] **DC-3** psmux install remains in `install-deps.ps1`; WT `settings.json` merged
      (managed subset only, no `$schema`) without clobbering user keys; re-apply is stable and the
      merge is skipped on POSIX.
- [ ] **DC-6** template tests pass on one host; the manifest-driven parity gate is green for N Ubuntu
      runs + CI macOS + CI Windows WT/copy-mode/content coverage (the decision-gate bar); surviving
      config-level tests still pass.
- [ ] **No old script deleted** (Wave A is additive; retirement is Wave C).
- [ ] Docs: `docs/MIGRATION_STATUS.md`; the dotfiles `ROADMAP.md`/issue updated
      if Wave C retirement planning changes.

---

## Appendix A — Wave B forward-refs (deferred, grounded so the shape is known)

| Deferred item | Grounding | chezmoi disposition |
|---|---|---|
| Future config additions beyond the current config-layer set | `bootstrap.sh`, `bootstrap.ps1`, future app configs | The current branch covers tmux, lazygit, nvim dir-symlink, Starship, zshenv, zshrc, Ghostty, Windows tmux overlay, Windows Terminal merge, PowerShell profile, and zsh plugin externals. Future rows should add manifest entries and matching single-source assertions in the same change. |
| Full `PKG_TABLE` (7 cols) + `$Catalog` (3 cols) → `.chezmoidata` | `install-deps.sh:851-895`, `install-deps.ps1:114-132` | one `.chezmoidata.yaml`; Unix picks one column, Windows iterates the `$order` fallback chain (`:179`) |
| 5 binary/font installers (nvim-linux `v0.12.2`, lazygit-linux `v0.62.2`, Hack Nerd Font `v3.4.0`, ghostty-ubuntu `1.3.1-0-ppa2`, win32yank) | `install-deps.sh:20-37,539-1219` | `.chezmoiexternal` **+ run-script** (fc-cache/`/opt`/apt/`chmod` side effects) |
| zsh login-shell switch (chsh / `/etc/shells` / domain `~/.bashrc` exec-zsh) | `install-deps.sh:418-443,398-416` | `run_once_` preserving `is_local_account` + `maybe_sudo` branching |
| devilspie2 daemon + autostart | `install-deps.sh:1297-1334` | managed `.lua` + `.desktop`; pkg-install + daemon-start in a Linux/X11-gated `run_once_` |
| VSCode `workbench.colorTheme` merge | `install-deps.ps1:313-357` | `modify_` (beware the JSONC `catch` no-op `:331-333`) |
| DC-4 secrets/private tier (`~/.zshrc.local`, `NOTES_VAULT`, brew shellenv) | `install-deps.sh:126-163` | `.chezmoi.toml.tmpl` prompts + `age`/password-manager (owner opt-in) |
| DC-5 distro matrix (dnf/pacman/apk/zypper) + deeper Windows CI parity | `container-e2e.sh:90-101` (apt-only) | matrix entry + root-prep arm + required-checks sync, per distro; Windows runner already covers copy-mode + WT merge while psmux install stays in `install-deps.ps1` |
| Edge cases to re-encode in run-scripts | `install-deps.sh:200-220,481-484`; `setup.sh:69-73` | `maybe_sudo`, domain accounts, `--best-effort`, no-TTY→all, dry-run |

## Appendix B — Pi CLI (and any TypeScript-based CLI tool) as a future add-on

The owner plans to add **Pi CLI** (`@mariozechner/pi-coding-agent`, `earendil-works/pi`) config +
TypeScript extensions. **chezmoi supports this fully — it is content-agnostic; TypeScript is a
non-issue** (jiti runs `.ts` extensions with no build step). It is another `dot_pi/agent/…` subtree
under the same DC-1/DC-3/DC-4 patterns. The only ways to break something — none a chezmoi limitation:

1. **Track config, IGNORE runtime state.** `~/.pi/agent/sessions/` is auto-written every run → put it
   in `.chezmoiignore` (else perpetual churn). Track `extensions/`, `skills/`, `themes/`, config;
   ignore `sessions/` + any cache/log.
2. **Never `exact_` `~/.pi/agent/`** (Verified mechanics §1 footgun) — it would delete sessions.
   Manage the specific subdirs, not the parent.
3. **Keep the LLM API key out of the PUBLIC dotfiles repo** — DC-4 secrets path (`~/.zshrc.local`
   escape hatch / `age` / 1Password template).
4. **Don't mark a `.ts`/JSON file `.tmpl`** unless you want templating — plain files are copied
   byte-for-byte, so TS with literal `{{`/`}}` is safe (escape only if you template one).
5. **If an extension has npm deps**, track its `package.json`+lockfile and add a `run_onchange_`
   `npm install` — **and embed `{{ include "package-lock.json" | sha256sum }}` in a comment** so it
   re-fires when the lockfile changes (Verified mechanics §8); otherwise it only re-runs on edits to
   the script itself. Dependency-free extensions need nothing.

Net: Pi CLI = one `dot_*` subtree + one `.chezmoiignore` rule + (maybe) one run-script + the secrets
rule. It does **not** threaten the chezmoi approach. (Sources: npm `@mariozechner/pi-coding-agent`;
`earendil-works/pi` `coding-agent/docs/extensions.md`.)

## Appendix C — corrections folded in (so they aren't re-litigated)

- **Sandbox via `HOME`, not `--destination`** — `--destination` leaves config/state/templates reading
  the real home (Step 0).
- **`.ps1` default interpreter is `pwsh -NoLogo -File`** — no custom `[interpreters]` block (a block
  ending in `-Command` would make scripts no-op).
- **`.chezmoitemplates/` is read via the `template` action**, not `include` (DC-3 Step 1 / DC-1 Step 4).
- **No `.Data` namespace** — `.chezmoidata`/`[data]` vars are `{{ .x }}` at root.
- **`symlink_*.tmpl` body = link target, not the managed path** — path-divergent files need one
  `symlink_` entry per OS path + a templated `.chezmoiignore` (DC-1 Step 4).
- **Per-OS `mode` is achievable** via `.chezmoi.toml.tmpl` — ergonomic choice, not a capability limit.
- **`.chezmoiexternal` downloads+verifies only** — and pins by ref, so the exact-commit assert is a
  required run-script (DC-2 Step 2), not optional.
- **The WT merge stays a value-level merge**, and the parity check is a structural deep-compare of the
  managed subset, NOT key-presence (DC-6 Step 4). `$schema` is not propagated.
- **`execute-template` can't spoof `.chezmoi.os`** — tests render against a fixture `.chezmoidata`
  (DC-6 Step 5).
- **The parity gate is manifest-scoped** to the full migrated config set, runs only the pilot slices of
  the old path, excludes directory rows, compares type + dereferenced content/perms (never
  symlink-node mode) + nvim realpath/tree parity + copied-source byte equality, and has separate
  Ubuntu, macOS, and Windows CI arms; psmux install stays in `install-deps.ps1`.

---

_Provenance: 2026-06-09 chezmoi Wave A pilot spec (owner-requested), branch
`claude/chezmoi-wave-a-spec` (stacked on PR #158). Grounded by a 5-agent read-through of
`luisgui1757/dotfiles` @ `96d85ee` (configs, unix + windows installers, WT fragment, tests/CI) and a
web-verified chezmoi-mechanics pass against chezmoi.io. **Adversarially reviewed by a 4-lens red-team
(2026-06-09):** grounding came back clean; correctness/executability/parity findings folded in — the
parity gate was reworked from a raw union diff into an intersection-scoped gate (old-path config slices only,
dereferenced content/perms, exact-commit externals, value-level WT deep-compare; now expanded from
Ubuntu-only/manual Windows to Ubuntu + macOS + Windows CI arms), the `[interpreters]` block removed,
the WT fragment switched from `include` to the `template` action, the sandbox switched to
`HOME`-based, and a Step 0 chezmoi-install prerequisite added. **Then reviewed by Codex 5.5 (xhigh),
2026-06-09** (read-only, against the same grounded
clone + chezmoi.io): it confirmed the prior fixes held (lazygit design, `modify_` target-name, WT
grounding, zsh pins all "tried to break but could not") and found 9 NET-NEW issues, all folded in —
a TOML bug (`mode` written under `[data]` parsed as `data.mode`, silently disabling POSIX symlinks),
a non-interactive-`init` hang (`promptStringOnce` removed from the pilot), the WT merge not being
paste-ready (now lists all six interdependent helpers incl. `Get-ArrayValue` + inlines the merge,
dropping the invented entrypoint), the old-path zsh slice needing `YES_ALL=1`, a `declare -A` that
breaks Bash 3.2/macOS, psmux not failing loud, WT scoped explicitly to stable + the manual harness's
`USERPROFILE`/`LOCALAPPDATA`/`APPDATA`, a "replace tests" contradiction (now "added alongside; retire
in Wave C"), and `sha()`/macOS-acceptance hardening. Codex's claim that empty-rendered OS-gated
scripts still run was **partially refuted** (chezmoi documents skipping whitespace-only scripts — now
cited in §8). Original scope was Wave A pilot only; the current branch has
expanded that to the full config layer while keeping provisioning and secrets in
Wave B / install-deps. Companion to `ROADMAP.md` DC-1…DC-6 and
`docs/plans/CONTAINERIZATION_WAVE_A_SPEC.md`._
