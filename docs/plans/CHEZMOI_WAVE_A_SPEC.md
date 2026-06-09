# Dotfiles → chezmoi — Wave A Pilot Implementation Spec (Codex-ready)

_Companion to `ROADMAP.md` → "Dotfiles → chezmoi Migration Program (2026-06-08)" (DC-1…DC-6)
and a sibling of `docs/plans/CONTAINERIZATION_WAVE_A_SPEC.md` (same house format). This spec
turns **Wave A** (the pilot + its parity gate) into something Codex 5.5 xhigh can execute
literally, grounded line-by-line in the real `luisgui1757/dotfiles` tree and in chezmoi's
documented semantics._

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
- **Scope discipline:** this is the **pilot**, not the full migration. Do exactly the slices in
  "Scope". The parity gate (DC-6) is **intersection-scoped** to the pilot paths — do not let it
  drift into a full-install diff.

## Where this runs — CRITICAL

**The work happens in the `luisgui1757/dotfiles` repo, NOT in Meridian.** This spec is *parked* in
the Meridian roadmap (owner request) so the two 2026-06-08 plans live together; it is not Meridian
product scope and should relocate to a dotfiles `ROADMAP.md`/issue when convenient.

- Operate on a checkout of `luisgui1757/dotfiles` on a feature branch (e.g. `chezmoi-pilot`).
- Build the chezmoi **source directory** as a new top-level dir **`home/`** in that repo. The
  existing `setup.sh` / `bootstrap.sh` / `install-deps.*` stay **untouched and working** for the
  whole pilot — the parity gate (DC-6) runs the relevant *slices* of the old path beside chezmoi and
  compares them, so they MUST coexist. Nothing in the old install path is deleted in Wave A (that is
  Wave C, after N green parity runs).

## Scope — Wave A pilot only

The pilot exercises the *mechanically hard* parts on a minimal slice so the owner can feel the
ergonomics before the 16–24-day Wave B (`ROADMAP.md:583-608`). Exactly these slices:

| Slice | What it proves | DC |
|---|---|---|
| **tmux single-source** (`tmux.conf` → `~/.tmux.conf` everywhere) + **Windows overlay** (`tmux.windows.conf`, ignored off-Windows) | trivial-config case + `.chezmoiignore` per-OS gating | DC-1 |
| **Path-divergent symlink: lazygit** (one source file → 3 different absolute paths per OS, live-edit on POSIX) | the single hardest config-layer mechanic | DC-1 |
| **One external: the two zsh plugins** as pinned `.chezmoiexternal` git-repos | the externals mechanism + its pin limits | DC-2 |
| **psmux install** (`run_once_after_`, the bucket-add→scoop→winget→choco chain) | imperative per-OS survivor with ordered fallback | DC-3 |
| **Windows Terminal `settings.json` merge** (`modify_` read-modify-write) | the JSON-merge mechanic; why a WT *fragment* can't replace it | DC-3 |
| **The parity gate** (old slices vs chezmoi, intersection-scoped, content-normalized + probes) | the keystone — nothing retires until it is green | DC-6 |
| **Hermetic per-OS template unit tests** (`execute-template` on an injected OS var) | host-independent branch coverage | DC-6 |

**Out of scope (Wave B, catalogued in Appendix A):** the other ~22 configs, the full
`PKG_TABLE`/`$Catalog` → `.chezmoidata` merge, the 5 non-plugin pinned binaries, the zsh login-shell
switch, devilspie2, the VSCode theme merge, the DC-4 secrets tier, the DC-5 distro matrix, and the
README/CLAUDE.md rewrite.

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
4. **The pilot external pins to upstream tags, AND a required `run_onchange_` asserts the exact
   commit** (DC-2). chezmoi `.chezmoiexternal` git-repo has no native exact-commit pin, and the
   pilot's whole point is fidelity, so the commit-equality assert (the old installer's
   `install-deps.sh:786-790` semantics) is promoted into the gate, not logged as a residual.
5. **Source dir = `home/` inside the dotfiles repo.** Old+new coexist; CI drives both.
6. **The automated CI parity gate is POSIX/Ubuntu-only for the pilot.** Windows-only deliverables
   (WT merge, psmux, copy-vs-symlink type) are verified **manually** in Wave A and gated in CI in
   Wave B (the existing `container-e2e.sh` is apt-only — `:90-101`).

---

## Step 0 — Prerequisites & the sandbox contract (do this first)

**Nothing in this spec runs without chezmoi installed and a real sandbox.** `chezmoi --destination`
alone does NOT sandbox — chezmoi still reads its config/state from the real `~/.config/chezmoi` and
`~/.local/share/chezmoi`, and `.chezmoi.toml.tmpl` resolves `.chezmoi.homeDir`/`.chezmoi.os` from the
real environment. The only correct sandbox is to **set `HOME`** for the chezmoi invocation (which
relocates config, state, and the template's home), ideally inside a container.

```sh
# --- Step 0a: install a pinned chezmoi (min v2.52.0 — supports .chezmoiexternal clone.args,
#     modify_ scripts, the `template` action, execute-template --init). ---
CHEZMOI_VERSION=v2.52.0          # pin; bump deliberately
sh -c "$(curl -fsLS get.chezmoi.io)" -- -b "$HOME/.local/bin" "$CHEZMOI_VERSION"
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
    windows-terminal/settings.fragment.jsonc   # DC-3 Step 2: the WT fragment
  dot_tmux.conf                          # Step 3
  dot_tmux.windows.conf                  # Step 3 (Windows-gated)
  dot_config/lazygit/symlink_config.yml.tmpl                 # Step 4 (Linux/WSL)
  Library/Application Support/lazygit/symlink_config.yml.tmpl # Step 4 (macOS)
  AppData/Local/lazygit/config.yml.tmpl                      # Step 4 (Windows copy)
  .chezmoiscripts/run_once_after_10-install-psmux.ps1.tmpl   # DC-3 Step 1
  AppData/Local/Packages/Microsoft.WindowsTerminal_8wekyb3d8bbwe/LocalState/modify_settings.json.ps1.tmpl  # DC-3 Step 2
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
in the pilot** (the real wsl distinction, needed only for ghostty/devilspie2, arrives in Wave B with
its own funnel logic):

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
  vs Linux `~/.config/ghostty/config`, `bootstrap.sh:211-219`; no Windows target) — Wave B.

### DC-1 Acceptance (maps `ROADMAP.md:407-423`)

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

### Step 2 — Required commit-assert `home/.chezmoiscripts/run_after_20-verify-zsh-plugin-pins.sh.tmpl`

chezmoi pins by ref, not by asserting a commit (Verified mechanics §4). The old installer **fails**
on commit mismatch (`install-deps.sh:786-790`). To keep that fidelity (resolved decision #4), assert
the exact pinned commits after the externals are fetched. `run_after_` runs after files/externals
(Verified mechanics §8); the embedded hash forces a re-run when the pins change.

```sh
# home/.chezmoiscripts/run_after_20-verify-zsh-plugin-pins.sh.tmpl
{{- if ne .targetOS "windows" -}}
#!/usr/bin/env bash
# {{ "install-deps.sh:26-29 pins" }} — re-run trigger; bump when the pins below change.
set -euo pipefail
root="${XDG_DATA_HOME:-$HOME/.local/share}/dotfiles/zsh-plugins"
# Bash 3.2-safe (stock macOS has no associative arrays — the repo keeps install-deps.sh:4 compatible).
for pair in \
  "zsh-autocomplete a76f26ae25528e76ee53df98ad38fbacdf89fd2e" \
  "zsh-autosuggestions e52ee8ca55bcc56a17c828767a3f98f22a68d4eb"; do   # install-deps.sh:27,29
  name="${pair%% *}"; want="${pair##* }"
  got="$(git -C "$root/$name" rev-parse HEAD)"
  [ "$got" = "$want" ] || { echo "FAIL: $name HEAD $got != pinned $want" >&2; exit 1; }
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

---

## DC-3 — Run-scripts: psmux install + the Windows Terminal merge

Maps `ROADMAP.md:455-487`. Two imperative survivors (Windows-only; verified manually in the pilot —
resolved decision #6).

### Step 1 — psmux: `home/.chezmoiscripts/run_once_after_10-install-psmux.ps1.tmpl`

psmux needs an ordered bucket-add **before** install, so it can't be a flat package entry
(`install-deps.ps1:359-402`). Ported from `install-deps.ps1:376-399` (the Ask gate `:368` and DryRun
`:372-375` are intentionally dropped — chezmoi scripts are non-interactive, matching the old "no-TTY
→ --all" behavior, `setup.sh:69-73`). **Each PM call is guarded by `Get-Command` so a missing
manager falls through instead of throwing** (`$ErrorActionPreference='Stop'` would otherwise abort).
The psmux bucket-add is hardened via `Add-ScoopBucketSafe` (idempotent + non-interactive, mirroring
`install-deps.ps1`) because the chezmoi run-script is non-interactive and `Stop`-strict — an
un-hardened clone credential prompt would hang `chez apply` with no answerable console.

```powershell
# home/.chezmoiscripts/run_once_after_10-install-psmux.ps1.tmpl
{{- if eq .targetOS "windows" -}}
$ErrorActionPreference = 'Stop'
function Test-Tool($n) { [bool](Get-Command $n -ErrorAction SilentlyContinue) }

# Idempotent, non-interactive scoop bucket add. NEVER throws (Stop-safe);
# GIT_TERMINAL_PROMPT/GCM_INTERACTIVE off so a clone auth challenge fails fast
# instead of hanging chez apply, and we verify the bucket populated
# (ScoopInstaller/Scoop#5482). Mirrors install-deps.ps1 Add-ScoopBucketSafe.
function Add-ScoopBucketSafe($Name, $Url) {
  $root = if ($env:SCOOP) { $env:SCOOP } else { Join-Path $env:USERPROFILE 'scoop' }
  $dir  = Join-Path (Join-Path $root 'buckets') $Name
  $ok   = { (Test-Path -LiteralPath $dir) -and (@(Get-ChildItem -LiteralPath $dir -Force -ErrorAction SilentlyContinue).Count -gt 0) }
  if (& $ok) { return $true }
  $op = $env:GIT_TERMINAL_PROMPT; $og = $env:GCM_INTERACTIVE
  $env:GIT_TERMINAL_PROMPT = '0'; $env:GCM_INTERACTIVE = '0'
  try {
    foreach ($i in 1..2) {
      if ([string]::IsNullOrEmpty($Url)) { scoop bucket add $Name 2>&1 | Out-Null }
      else { scoop bucket add $Name $Url 2>&1 | Out-Null }
      if (& $ok) { return $true }
      if (Test-Path -LiteralPath $dir) { Remove-Item -LiteralPath $dir -Recurse -Force -ErrorAction SilentlyContinue }
      scoop bucket rm $Name 2>&1 | Out-Null
    }
    Write-Warning "scoop bucket add $Name did not populate a usable bucket; recover with 'scoop bucket rm $Name' then re-run"
    return $false
  } finally { $env:GIT_TERMINAL_PROMPT = $op; $env:GCM_INTERACTIVE = $og }
}

if (Test-Tool 'psmux') { Write-Host 'ok        psmux already installed'; return }

# 1) scoop (custom bucket first) — install-deps.ps1:376-384
if (Test-Tool 'scoop') {
  if (Add-ScoopBucketSafe 'psmux' 'https://github.com/psmux/scoop-psmux') {
    scoop install psmux
    if ($LASTEXITCODE -eq 0 -and (Test-Tool 'psmux')) { Write-Host 'installed psmux via scoop'; return }
  }
  Write-Warning 'scoop install of psmux failed; trying winget...'
}
# 2) winget — install-deps.ps1:386-390
if (Test-Tool 'winget') {
  winget install psmux --accept-source-agreements --accept-package-agreements --silent
  if ($LASTEXITCODE -eq 0 -and (Test-Tool 'psmux')) { Write-Host 'installed psmux via winget'; return }
  Write-Warning 'winget install of psmux failed; trying choco...'
}
# 3) choco — install-deps.ps1:394-398
if (Test-Tool 'choco') {
  choco install psmux -y
  if ($LASTEXITCODE -eq 0 -and (Test-Tool 'psmux')) { Write-Host 'installed psmux via choco'; return }
}
Write-Warning 'psmux install failed via scoop/winget/choco.'
exit 1   # required survivor: fail so run_once_ records NO success and retries next apply
{{- end -}}
```

### Step 2 — Windows Terminal: `modify_settings.json.ps1.tmpl` (the merge)

The WT `settings.json` is **app-owned** (the user/app writes profiles, `defaultProfile`, …); we
merge our keys without clobbering theirs — the `modify_` contract (Verified mechanics §9). A Fragment
Extension **cannot** do this (it can't set the 7 globals + `profiles.defaults` + 15 keybindings —
`settings.fragment.jsonc:24-109`, merge logic `bootstrap.ps1:307-466`). Stable Store-WT path is static
(`Microsoft.WindowsTerminal_8wekyb3d8bbwe` — `bootstrap.ps1:296`).

> **Wave A scopes to the stable Store WT only.** The old code also handles WT **Preview**
> (`Microsoft.WindowsTerminalPreview_8wekyb3d8bbwe`) and discovers the path via `%LOCALAPPDATA%`
> (`bootstrap.ps1:295-299`); Preview and a redirected `%LOCALAPPDATA%` are **Wave B**. Because the
> managed target is a fixed `~/AppData/Local/...` path, the manual Windows parity harness (DC-6 Step 4)
> must run in a throwaway Windows profile with `USERPROFILE`/`LOCALAPPDATA`/`APPDATA` pointed at the
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

- [ ] On Windows, `chez apply` installs psmux (scoop→winget→choco fallback) and merges the WT
      fragment into a live `settings.json` **without dropping** pre-existing profiles/keys.
- [ ] The merged `settings.json` contains exactly the managed key set — 7 globals, 15 keybindings,
      `profiles.defaults`, and the named rose-pine scheme+theme — and **not** a stray `$schema`
      (compare the merged KEY SET, not a raw diff against the fragment).
- [ ] Re-running `chez apply` is a no-op (stable modify output; psmux content-hash unchanged).
- [ ] On POSIX both are skipped (`.chezmoiignore`); `chez verify` clean.

---

## DC-6 — The parity gate (the keystone) + hermetic template tests

Maps `ROADMAP.md:540-582`. **Nothing in the old install path is deleted until this is green for N
runs (DC-6 decision-gate item).** The gate is **intersection-scoped to the pilot's leaf target
paths** — it is NOT a raw union of two full installs (a full `setup.sh --all` would produce dozens of
deferred-config paths and devilspie2/font/package side effects that the pilot legitimately does not
manage, false-FAILing the gate). It runs the **pilot slices only** of the old path and compares
**only the pilot allow-list**.

### Step 1 — Run the pilot slices of the OLD path (no full install, no out-of-scope footprint)

In the Ubuntu container, into `$HOME_OLD=$(mktemp -d)`:

```sh
HOME_OLD="$(mktemp -d)"
# tmux + lazygit symlinks ONLY (bootstrap.sh is the symlink layer; it installs no packages,
# no devilspie2, no fonts). Intersection-scoping (Step 3) ignores any non-pilot links it makes.
env HOME="$HOME_OLD" "$REPO_ROOT/bootstrap.sh"
# zsh plugins ONLY — source install-deps.sh and call the single function (avoids the full installer).
# YES_ALL=1 is REQUIRED: install_zsh_plugins gates on ask() (install-deps.sh:809), which auto-accepts
# only when YES_ALL=1 or DRY_RUN=1 (install-deps.sh:199-207); without it a non-interactive gate skips
# the plugins and P3/P4 then false-FAIL.
env HOME="$HOME_OLD" INSTALL_DEPS_SOURCE_ONLY=1 YES_ALL=1 bash -c \
  'source "'"$REPO_ROOT"'/install-deps.sh"; install_zsh_plugins'   # install-deps.sh:737-826
```

> This deliberately does NOT run `install-deps.sh --all` / `setup.sh` — so no fonts, login-shell,
> devilspie2, packages, or other Wave-B footprint is produced. The gate therefore needs **no giant
> ignore-list**; it only ever looks at the pilot allow-list (Step 3).

### Step 2 — Apply chezmoi into a separate sandbox

```sh
HOME_NEW="$(mktemp -d)"
env HOME="$HOME_NEW" chezmoi --source "$SRC" init
env HOME="$HOME_NEW" chezmoi --source "$SRC" apply
```

### Step 3 — The intersection-scoped comparison (the gate)

The **pilot allow-list** (the only paths the gate inspects):

```
P1  ~/.tmux.conf
P2  ~/.config/lazygit/config.yml                 # Linux target
P3  ~/.local/share/dotfiles/zsh-plugins/zsh-autocomplete       (git checkout)
P4  ~/.local/share/dotfiles/zsh-plugins/zsh-autosuggestions    (git checkout)
```

For each allow-list path, assert (and **nothing else** — no recursive `find`, no directory rows, no
raw `find -type d` bookkeeping, which the review showed false-FAILs on intermediate dirs):

```
for p in {P1,P2}:                                  # the two managed config files
    assert exists(HOME_OLD/p) and exists(HOME_NEW/p)
    # TYPE per chezmoi model, asserted SEPARATELY (do NOT compare raw link targets:
    #   old -> repo file, new -> sourceDir file; they legitimately differ):
    assert is_symlink(HOME_OLD/p) and is_symlink(HOME_NEW/p)        # POSIX/Ubuntu arm
    # BYTE parity via DEREFERENCED content (safe: type already asserted):
    assert sha(deref(HOME_OLD/p)) == sha(deref(HOME_NEW/p))
    # PERMISSION parity of the RESOLVED content (NOT the link node, which carries no info):
    assert mode(deref(HOME_OLD/p)) == mode(deref(HOME_NEW/p))

for d in {P3,P4}:                                  # the two plugin checkouts
    assert git_head(HOME_OLD/d) == git_head(HOME_NEW/d)            # exact-commit equality (DC-2)
    # (no file-by-file diff of the checkout — the commit hash IS the content identity)

# Single-source assertion (DC-1 Step 4 obligation):
assert sha(REPO_ROOT/lazygit/config.yml) == sha(SRC/.chezmoitemplates/lazygit/config.yml)
```

Portable helpers (the gate runs in the Ubuntu container — GNU coreutils — but these keep a manual
macOS run working):

```sh
# Guard on command availability, NOT on pipe exit codes (a piped `|| ` keys off cut's status, not the hasher's).
sha()  { if command -v sha256sum >/dev/null 2>&1; then sha256sum "$1" | cut -d' ' -f1; else shasum -a 256 "$1" | cut -d' ' -f1; fi; }
mode() { if stat -c '%a' "$1" >/dev/null 2>&1; then stat -c '%a' "$1"; else stat -f '%Lp' "$1"; fi; }   # GNU vs BSD/macOS
deref(){ if readlink -f "$1" >/dev/null 2>&1; then readlink -f "$1"; else python3 -c 'import os,sys;print(os.path.realpath(sys.argv[1]))' "$1"; fi; }
```

The gate **FAILS** on any allow-list path that is missing on one side, differs in dereferenced
content/permissions, or (plugins) differs in HEAD commit. It cannot be fooled by deferred configs
(not in the allow-list) or by directory bookkeeping (excluded).

### Step 4 — Non-file probes (POSIX arm) — what `chez verify` cannot see

`verify` checks target state only (Verified mechanics §12). The pilot's only POSIX side effect is the
plugin checkouts, already covered by the HEAD-commit assert in Step 3. **The Windows-only side
effects (psmux install, WT merge) do NOT run on the Ubuntu arm** — they are verified **manually** for
the pilot (resolved decision #6) and gated in Wave B:

- **psmux (Windows, manual):** after `chez apply` on a Windows sandbox, `Get-Command psmux` resolves;
  `psmux` launches reading `~/.tmux.conf`.
- **WT merge (Windows, manual) — value-level, not key-presence:** in a throwaway Windows profile with
  `USERPROFILE`/`LOCALAPPDATA`/`APPDATA` pointed at the sandbox (so the old `%LOCALAPPDATA%` path and the
  new `~/AppData/Local` path are the same file — see DC-3 Step 2), seed an identical baseline
  `settings.json` (e.g. WT's default + one user profile) into two sandboxes; in one run the OLD
  `bootstrap.ps1 -MergeWindowsTerminal`, in the other `chez apply`; then **deep-compare the two
  resulting JSONs** (normalize: sort keys/arrays canonically) and assert structural equality of the
  managed subset (7 globals + all 15 keybindings + `profiles.defaults` + named scheme/theme) AND that
  the user's seeded profile/`defaultProfile` survived in both. (Key *presence* alone is insufficient —
  a merge that dropped 12 keybindings or clobbered `defaultProfile` would still have `.actions`.)

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

### Step 6 — CI wiring (reuse, don't rewrite)

- The existing container E2E hard-fails on any non-apt PM (`container-e2e.sh:90-101`), so the pilot
  parity gate runs on the **existing `apt`/Ubuntu** entry only; distro-matrix widening is Wave B/DC-5
  (each distro needs a matrix entry **and** a root-prep arm **and** a `required_checks_test.sh`
  update — `e2e-install.yml:43`, `required_checks_test.sh:27`). Do not attempt it in Wave A.
- **Add (don't rename) a CI job.** Put the gate in a new `test.yml` job named `chezmoi-parity`. Its
  required-status context is the **bare job name `chezmoi-parity`** (`required_checks_test.sh:16-48`
  extracts `test.yml` job contexts as the bare key; only `e2e-install.yml` matrix jobs get
  `<group> / <id>` prefixes). Mirror the exact string `chezmoi-parity` in all three sync files in the
  SAME commit: `.github/settings.yml`, `scripts/apply-repo-safeguards.sh`, `.github/rulesets/main-integrity.json`
  — else `required_checks_test.sh` (and branch protection) breaks.
- Leave `tests/static/invariants_test.sh:98` (the `.\test.ps1` Windows-entrypoint invariant) intact —
  the pilot adds, not replaces.

### DC-6 Acceptance (maps `ROADMAP.md:580-582`)

- [ ] Step-5 template tests pass on a single host for all three OS values (tmux + lazygit branches).
- [ ] A second `chez apply` is a no-op; `chez verify` + `chez doctor` clean.
- [ ] The Step-3 intersection gate passes (dereferenced content + resolved-permission parity for P1/P2;
      exact-commit equality for P3/P4; the single-source equality assert) on the Ubuntu arm for **N
      consecutive runs** (N fixed in the decision gate) before any old script is considered for
      retirement (retirement = Wave C).
- [ ] The Windows-only checks (psmux, WT value-level deep-compare) pass on **one manual Windows run**.

---

## Pilot decision gate — evaluate before committing to Wave B

After the pilot is green, the owner judges (the point of Wave A — `ROADMAP.md:583-588`):

1. **Ergonomics:** does the copy-vs-symlink split + the `.chezmoitemplates`-anchored path-divergent
   symlink feel maintainable, or is it more ceremony than `bootstrap.sh link()`?
2. **Go-template verbosity:** readable at the scale of 28 configs?
3. **Parity-gate cost:** the gate is a **migration oracle**, not a test — worth building out for the
   full set, or is the two-script status quo fine?
4. **Pin fidelity:** is the tag-pin + commit-assert combo acceptable, or do you want exact-commit
   externals natively (a stronger chezmoi feature/version)?
5. **Green bar (the quantitative gate):** retirement (Wave C) is authorized only after **N = 10
   consecutive green Ubuntu parity runs + 1 manual Windows pass + 1 manual macOS pass**, where
   "green" = {dereferenced content parity, type-per-model, externals exact-commit equality, WT
   structural deep-compare, single-source equality}. Until that bar is met, the old scripts stay.

Only if 1–4 clear and the bar in 5 is achievable does Wave B begin.

---

## Acceptance checklist (consolidated)

- [ ] **Step 0** pinned chezmoi installed (`chezmoi --version` ≥ v2.52.0); sandbox via `HOME=$SANDBOX`
      (not `--destination` alone).
- [ ] **DC-1** tmux single-source on all OSes; tmux.windows.conf Windows-only; lazygit at the correct
      per-OS path; POSIX symlinks (live-edit) / Windows copies; verify clean; re-apply no-op.
- [ ] **DC-2** both zsh plugins cloned to the unchanged path; the commit-assert passes; skipped on
      Windows.
- [ ] **DC-3** psmux installed via the guarded scoop→winget→choco chain; WT `settings.json` merged
      (managed subset only, no `$schema`) without clobbering user keys; both no-op on re-apply; both
      skipped on POSIX.
- [ ] **DC-6** template tests pass on one host; the intersection parity gate is green for N Ubuntu
      runs + 1 manual Windows + 1 manual macOS (the decision-gate bar); surviving config-level tests
      still pass.
- [ ] **No old script deleted** (Wave A is additive; retirement is Wave C).
- [ ] Docs: a short `home/README.md`; the dotfiles `ROADMAP.md`/issue updated (this plan relocated out
      of Meridian per its Open-items note).

---

## Appendix A — Wave B forward-refs (deferred, grounded so the shape is known)

| Deferred item | Grounding | chezmoi disposition |
|---|---|---|
| Other ~22 configs (zshrc/zshenv, starship, nvim 18 files + `lazy-lock.json`, powershell profile, ghostty) | `bootstrap.sh:202-249`, `bootstrap.ps1:275-289` | `dot_*`/`dot_config/*`; nvim verbatim; ghostty = lazygit pattern |
| Full `PKG_TABLE` (7 cols) + `$Catalog` (3 cols) → `.chezmoidata` | `install-deps.sh:851-895`, `install-deps.ps1:114-132` | one `.chezmoidata.yaml`; Unix picks one column, Windows iterates the `$order` fallback chain (`:179`) |
| 5 binary/font installers (nvim-linux `v0.12.2`, lazygit-linux `v0.62.2`, Hack Nerd Font `v3.4.0`, ghostty-ubuntu `1.3.1-0-ppa2`, win32yank) | `install-deps.sh:20-37,539-1219` | `.chezmoiexternal` **+ run-script** (fc-cache/`/opt`/apt/`chmod` side effects) |
| zsh login-shell switch (chsh / `/etc/shells` / domain `~/.bashrc` exec-zsh) | `install-deps.sh:418-443,398-416` | `run_once_` preserving `is_local_account` + `maybe_sudo` branching |
| devilspie2 daemon + autostart | `install-deps.sh:1297-1334` | managed `.lua` + `.desktop`; pkg-install + daemon-start in a Linux/X11-gated `run_once_` |
| VSCode `workbench.colorTheme` merge | `install-deps.ps1:313-357` | `modify_` (beware the JSONC `catch` no-op `:331-333`) |
| DC-4 secrets/private tier (`~/.zshrc.local`, `NOTES_VAULT`, brew shellenv) | `install-deps.sh:126-163` | `.chezmoi.toml.tmpl` prompts + `age`/password-manager (owner opt-in) |
| DC-5 distro matrix (dnf/pacman/apk/zypper) + a Windows CI parity arm | `container-e2e.sh:90-101` (apt-only) | matrix entry + root-prep arm + required-checks sync, per distro; a Windows runner for the WT/psmux gate |
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
- **`.chezmoitemplates/` is read via the `template` action**, not `include` (DC-3 Step 2 / DC-1 Step 4).
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
- **The parity gate is intersection-scoped** to the pilot allow-list, runs only the pilot slices of
  the old path, excludes directory rows, compares dereferenced content/perms (never symlink-node
  mode), and is Ubuntu-only with Windows verified manually (DC-6).

---

_Provenance: 2026-06-09 chezmoi Wave A pilot spec (owner-requested), branch
`claude/chezmoi-wave-a-spec` (stacked on PR #158). Grounded by a 5-agent read-through of
`luisgui1757/dotfiles` @ `96d85ee` (configs, unix + windows installers, WT fragment, tests/CI) and a
web-verified chezmoi-mechanics pass against chezmoi.io. **Adversarially reviewed by a 4-lens red-team
(2026-06-09):** grounding came back clean; correctness/executability/parity findings folded in — the
parity gate was reworked from a raw union diff into an intersection-scoped gate (pilot slices only,
dereferenced content/perms, exact-commit externals, value-level WT deep-compare, Ubuntu-only +
manual Windows), the `[interpreters]` block removed, the WT fragment switched from `include` to the
`template` action, the sandbox switched to `HOME`-based, and a Step 0 chezmoi-install prerequisite
added. **Then reviewed by Codex 5.5 (xhigh), 2026-06-09** (read-only, against the same grounded
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
cited in §8). Scope = Wave A pilot only; Wave B is catalogued (Appendix A) but not specced. Companion
to `ROADMAP.md` DC-1…DC-6 and `docs/plans/CONTAINERIZATION_WAVE_A_SPEC.md`._
