# nix-darwin system configuration (macOS). PACKAGES / system policy only --
# chezmoi owns every dotfile target (CLAUDE.md invariant 22). This module wires
# declarative Homebrew for the GUI/vendor apps (WezTerm, AeroSpace casks) and the
# Herdr brew; the nix-owned CLI package set lives in Home Manager (nix/home/darwin.nix).
{ pkgs
, username
, userHome
, ...
}:
{
  # Determinate Systems owns the Nix daemon on the proving host, so nix-darwin
  # must NOT try to manage Nix itself (that would fight the Determinate daemon).
  nix.enable = false;

  system.stateVersion = 6;

  # Recent nix-darwin requires a concrete primary user for user-scoped
  # activation (Homebrew, defaults, Home Manager). Resolved from $USER at switch
  # time by the flake; pure eval / `nix flake check` gets a placeholder.
  system.primaryUser = username;

  # nixpkgs.hostPlatform is set by nix-darwin from the `system` argument the
  # flake passes to darwinSystem; setting it here from `pkgs` would be circular.

  # Tell nix-darwin the primary user's home so the Home Manager integration can
  # derive home.homeDirectory (we do NOT create/manage the account -- it already
  # exists on the real Mac; this only supplies the path metadata).
  users.users.${username}.home = userHome;

  # Declarative Homebrew. GUI / TCC-sensitive apps come from vendor channels
  # (casks / the trusted AeroSpace tap), never nixpkgs -- see the migration ruling.
  homebrew = {
    enable = true;

    onActivation = {
      # Never mutate the world implicitly: no `brew update`, no `brew upgrade`.
      autoUpdate = false;
      upgrade = false;
      # Homebrew is a mixed-ownership package plane: nix-darwin installs the
      # repo-declared subset, while install-deps and the user may own additional
      # formulae/casks. Never reject or remove those unrelated packages.
      cleanup = "none";
    };

    # Homebrew owns mutable tap clones as the target user. nix-homebrew pins the
    # Homebrew implementation but does not copy tap trees during root activation;
    # copied trees are root-owned and make ordinary brew update/install noisy.
    taps = [ "nikitabobko/tap" ];

    casks = [
      "wezterm"
      "aerospace"
    ];

    brews = [
      "herdr"
    ];
  };
}
