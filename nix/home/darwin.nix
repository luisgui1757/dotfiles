# Home Manager on Darwin -- PACKAGES ONLY. No home.file, no xdg.configFile, no
# programs.<tool> config-generating module: chezmoi owns every dotfile target
# (CLAUDE.md invariant 22, guarded by tests/static/nix_architecture_test.sh).
# This is the nix-owned CLI package set for a user who opts into `darwin-rebuild`.
{ pkgs
, username
, ...
}:
{
  home.username = username;
  # home.homeDirectory is derived by the nix-darwin Home Manager integration from
  # users.users.<username>.home (set in nix/darwin/configuration.nix), so it is
  # intentionally not set here to avoid a conflicting definition.
  home.stateVersion = "25.05";

  # Packages only. These are the CLI tools install-deps.sh otherwise provisions
  # via Homebrew; a user who adopts the Nix layer gets them from nixpkgs instead.
  home.packages = with pkgs; [
    ripgrep
    fd
    fzf
    jq
    lazygit
    starship
    zoxide
    tree-sitter
  ];
}
