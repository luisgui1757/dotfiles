# Home Manager on Darwin -- PACKAGES ONLY (see nix/home/common.nix). The nix-owned
# CLI package set for a user who opts into `darwin-rebuild`. chezmoi owns every
# dotfile target (CLAUDE.md invariant 22).
{ ... }:
{
  imports = [ ./common.nix ];

  # home.homeDirectory is derived by the nix-darwin Home Manager integration from
  # users.users.<username>.home (set in nix/darwin/configuration.nix), so it is
  # intentionally not set here to avoid a conflicting definition.
}
