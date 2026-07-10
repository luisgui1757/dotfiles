# Home Manager standalone on Linux / WSL userland -- PACKAGES ONLY (see
# nix/home/common.nix). chezmoi owns every dotfile target (CLAUDE.md invariant
# 22). On WSL this writes ONLY to the Linux home (~/.nix-profile) -- never to a
# Windows-host path under /mnt/c: the split-host model is preserved (Windows
# apps/fonts/terminal stay a Windows-host responsibility).
{ config
, homeDirectory
, username
, ...
}:
{
  imports = [ ./common.nix ];

  home.homeDirectory = homeDirectory;

  # The managed zsh sources Home Manager's canonical hm-session-vars.sh. Make
  # that file carry the active profile bin path itself so a clean login shell
  # does not depend on caller/system PATH injection. This owns session state,
  # not a dotfile; chezmoi remains the sole ~/.zshrc owner.
  home.sessionPath = [ "${config.home.profileDirectory}/bin" ];

  # Standalone Home Manager manages its own `home-manager` CLI. This is the ONE
  # allowed programs.* module (it installs the HM tool, it does not render any
  # chezmoi-owned config).
  programs.home-manager.enable = true;
}
