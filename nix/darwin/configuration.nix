# nix-darwin system configuration (macOS). PACKAGES / system policy only --
# chezmoi owns every dotfile target (CLAUDE.md invariant 22). This module wires
# declarative Homebrew for the GUI/vendor apps (WezTerm, AeroSpace casks) and the
# Herdr brew; the nix-owned CLI package set lives in Home Manager (nix/home/darwin.nix).
{ config
, pkgs
, username
, userHome
, ...
}:
let
  hostedCiHomebrewCleanup = builtins.getEnv "DOTFILES_NIX_DARWIN_HOSTED_CI" == "1";
in
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
  # (casks / a pinned tap), never nixpkgs -- see the migration ruling.
  homebrew = {
    enable = true;

    onActivation = {
      # Never mutate the world implicitly: no `brew update`, no `brew upgrade`.
      autoUpdate = false;
      upgrade = false;
      # Report drift (uninstall-what-is-not-declared) WITHOUT destroying it.
      # GitHub's hosted macOS images ship a large preinstalled Brew surface, so
      # setup.sh passes DOTFILES_NIX_DARWIN_HOSTED_CI=1 only for that disposable
      # activation path. Real hosts keep the strict drift check.
      cleanup = if hostedCiHomebrewCleanup then "none" else "check";
    };

    # With nix-homebrew mutableTaps=false, the declared taps are the pinned flake
    # inputs; mirror them here so the homebrew module does not try to re-tap.
    taps = builtins.attrNames config.nix-homebrew.taps;

    casks = [
      "wezterm"
      "aerospace"
    ];

    brews = [
      "herdr"
    ];
  };
}
