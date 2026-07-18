# Shared Home Manager base -- PACKAGES ONLY. Imported by both the darwin config
# (nix/home/darwin.nix) and the standalone-Linux config (nix/home/linux.nix).
# No home.file, no xdg.configFile, no programs.<tool> config-generating module:
# chezmoi owns every dotfile target (CLAUDE.md invariant 22, guarded by
# tests/static/nix_architecture_test.sh).
{ pkgs
, username
, ...
}:
{
  home.username = username;
  home.stateVersion = "25.05";

  # The nix-owned CLI package set. These are tools install-deps.sh otherwise
  # provisions via the native package manager / Homebrew in legacy or direct
  # install-deps paths; public POSIX setup gets them from nixpkgs. Vendor-channel apps
  # (WezTerm/AeroSpace/Herdr) come from Homebrew, not here.
  #
  # DELIBERATELY EXCLUDED (deferred, ABI-coupled to the still-native Neovim):
  # neovim and the tree-sitter CLI. nvim-treesitter `main` compiles parsers whose
  # ABI must match nvim's built-in libtree-sitter, and the repo pins the
  # tree-sitter CLI (v0.26.11) precisely to keep that build reproducible. A nix
  # neovim / tree-sitter shadowing the pinned native ones would risk an
  # E5113-class parser/ABI mismatch. Moving nvim into the same Nix closure as its
  # parser toolchain is a follow-up; see ROADMAP + CLAUDE.md invariant 19.
  home.packages = with pkgs; [
    ripgrep
    fd
    fzf
    jq
    lazygit
    nodejs_24
    starship
    zoxide
  ];
}
