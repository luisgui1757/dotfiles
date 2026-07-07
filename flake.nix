{
  description = "luisgui1757/dotfiles Nix layer -- packages only. chezmoi owns every dotfile target on every OS (CLAUDE.md invariant 22). Native Windows is non-Nix.";

  # Inputs are pinned by the committed flake.lock. It is bumped only through
  # reviewed PRs (Renovate `nix` manager or an explicit `nix flake update`) --
  # never silently during setup/update.
  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
  };

  outputs =
    { self, nixpkgs, ... }:
    let
      # Nix applies to POSIX only. Native Windows stays on setup.ps1 + native
      # package managers + chezmoi -- there is deliberately no windows system.
      systems = [
        "aarch64-darwin"
        "x86_64-darwin"
        "aarch64-linux"
        "x86_64-linux"
      ];
      forAllSystems = f: nixpkgs.lib.genAttrs systems (system: f nixpkgs.legacyPackages.${system});
    in
    {
      # Convenience dev shell mirroring the repo's lint/test toolchain. This is a
      # PACKAGE set only -- it writes no config and owns no dotfiles. (The
      # packages-only / no-home.file boundary is enforced statically by
      # tests/static/nix_architecture_test.sh.)
      devShells = forAllSystems (pkgs: {
        default = pkgs.mkShellNoCC {
          packages = [
            pkgs.shellcheck
            pkgs.shfmt
            pkgs.yamllint
            pkgs.taplo
            pkgs.jq
            pkgs.chezmoi
            pkgs.nodejs
          ];
        };
      });

      # `nix flake check` builds these on the current system. Kept cheap and
      # hermetic (substituted from cache.nixos.org; no IFD, no build-time
      # network): prove the pinned nixpkgs resolves the repo's core CLI tools.
      checks = forAllSystems (pkgs: {
        toolchain = pkgs.runCommandNoCC "dotfiles-nix-toolchain" { } ''
          for bin in \
            ${pkgs.shellcheck}/bin/shellcheck \
            ${pkgs.taplo}/bin/taplo \
            ${pkgs.yamllint}/bin/yamllint \
            ${pkgs.jq}/bin/jq \
            ${pkgs.chezmoi}/bin/chezmoi; do
            test -x "$bin" || {
              echo "missing $bin"
              exit 1
            }
          done
          touch "$out"
        '';
      });

      formatter = forAllSystems (pkgs: pkgs.nixpkgs-fmt);
    };
}
