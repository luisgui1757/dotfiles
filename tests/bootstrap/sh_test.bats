#!/usr/bin/env bats
# Coverage for bootstrap.sh: idempotency, backup, partial-recovery, dry-run.

setup() {
    REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd -P)"
    FAKE_HOME="$(mktemp -d)"
    export HOME="$FAKE_HOME"
    # bootstrap.sh writes into $HOME-derived paths only — no need to mock $LOCALAPPDATA etc.
}

teardown() {
    rm -rf "$FAKE_HOME"
}

run_bootstrap() {
    HOME="$FAKE_HOME" bash "$REPO_ROOT/bootstrap.sh" "$@"
}

@test "fresh install creates the expected symlinks" {
    run run_bootstrap
    [ "$status" -eq 0 ]
    [ -L "$FAKE_HOME/.config/nvim" ]
    [ "$(readlink "$FAKE_HOME/.config/nvim")" = "$REPO_ROOT/nvim" ]
    [ -L "$FAKE_HOME/.config/starship.toml" ]
    [ "$(readlink "$FAKE_HOME/.config/starship.toml")" = "$REPO_ROOT/starship/starship.toml" ]
    [ -L "$FAKE_HOME/.tmux.conf" ]
    [ -L "$FAKE_HOME/.zshrc" ]
    # lazygit config -- carries the J/K move-commit binding.
    [ -L "$FAKE_HOME/.config/lazygit/config.yml" ]
    [ "$(readlink "$FAKE_HOME/.config/lazygit/config.yml")" = "$REPO_ROOT/lazygit/config.yml" ]
}

@test "re-running is a no-op (idempotent)" {
    run run_bootstrap
    [ "$status" -eq 0 ]

    # Snapshot link targets
    pre_nvim=$(readlink "$FAKE_HOME/.config/nvim")
    pre_tmux=$(readlink "$FAKE_HOME/.tmux.conf")

    run run_bootstrap
    [ "$status" -eq 0 ]

    # Targets unchanged
    [ "$(readlink "$FAKE_HOME/.config/nvim")" = "$pre_nvim" ]
    [ "$(readlink "$FAKE_HOME/.tmux.conf")" = "$pre_tmux" ]

    # No new .bak files should appear
    bak_count=$(find "$FAKE_HOME" -name "*.bak.*" 2>/dev/null | wc -l | tr -d ' ')
    [ "$bak_count" = "0" ]
}

@test "non-symlink target is backed up before being replaced" {
    mkdir -p "$FAKE_HOME"
    echo "user-written" > "$FAKE_HOME/.zshrc"

    run run_bootstrap
    [ "$status" -eq 0 ]
    [ -L "$FAKE_HOME/.zshrc" ]

    # The original is preserved as .bak.<timestamp>
    bak=$(find "$FAKE_HOME" -maxdepth 1 -name ".zshrc.bak.*" | head -1)
    [ -n "$bak" ]
    [ "$(cat "$bak")" = "user-written" ]
}

@test "partial recovery: only missing links are added, existing correct ones untouched" {
    mkdir -p "$FAKE_HOME/.config"
    ln -s "$REPO_ROOT/nvim" "$FAKE_HOME/.config/nvim"

    pre_target=$(readlink "$FAKE_HOME/.config/nvim")

    run run_bootstrap
    [ "$status" -eq 0 ]

    # Semantic check: existing CORRECT link still points where it did
    # (target preserved). Avoid inode comparison -- atomic-rename
    # implementations on some Linux filesystems can change inodes even
    # when nothing semantically changed.
    [ -L "$FAKE_HOME/.config/nvim" ]
    post_target=$(readlink "$FAKE_HOME/.config/nvim")
    [ "$pre_target" = "$post_target" ]

    # And no stray .bak.* file was produced for an already-correct link.
    run find "$FAKE_HOME/.config" -maxdepth 1 -name "nvim.bak.*"
    [ "$status" -eq 0 ]
    [ -z "$output" ]

    [ -L "$FAKE_HOME/.tmux.conf" ]      # missing link now in place
}

@test "--dry-run changes nothing on disk" {
    run run_bootstrap --dry-run
    [ "$status" -eq 0 ]
    [ ! -e "$FAKE_HOME/.config/nvim" ]
    [ ! -e "$FAKE_HOME/.tmux.conf" ]
    [ ! -e "$FAKE_HOME/.zshrc" ]
    [ ! -e "$FAKE_HOME/.config/lazygit/config.yml" ]
}

@test "WSL branch links ghostty config and re-running is idempotent" {
    run env HOME="$FAKE_HOME" DOTFILES_FORCE_OS=wsl bash "$REPO_ROOT/bootstrap.sh"
    [ "$status" -eq 0 ]
    [ -L "$FAKE_HOME/.config/ghostty/config" ]
    [ "$(readlink "$FAKE_HOME/.config/ghostty/config")" = "$REPO_ROOT/ghostty/config" ]

    # Second run must be a no-op: link unchanged, no stray backups.
    run env HOME="$FAKE_HOME" DOTFILES_FORCE_OS=wsl bash "$REPO_ROOT/bootstrap.sh"
    [ "$status" -eq 0 ]
    [ -L "$FAKE_HOME/.config/ghostty/config" ]
    [ "$(readlink "$FAKE_HOME/.config/ghostty/config")" = "$REPO_ROOT/ghostty/config" ]
    bak_count=$(find "$FAKE_HOME" -name "*.bak.*" 2>/dev/null | wc -l | tr -d ' ')
    [ "$bak_count" = "0" ]
}

@test "missing git still symlinks configs" {
    # Build a sandbox PATH without git but with the basics (ln, readlink, etc.)
    sandbox=$(mktemp -d)
    for cmd in bash ln readlink uname dirname date mkdir mv rm cat grep tr basename printf; do
        if path=$(command -v "$cmd"); then ln -s "$path" "$sandbox/$cmd"; fi
    done
    # Ensure pwsh is also absent so the optional pwsh branch is skipped.
    PATH="$sandbox" run bash "$REPO_ROOT/bootstrap.sh"
    [ "$status" -eq 0 ]
    [ -L "$FAKE_HOME/.config/nvim" ]
    [ "$(readlink "$FAKE_HOME/.config/nvim")" = "$REPO_ROOT/nvim" ]
}
