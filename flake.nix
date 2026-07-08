{
  description = "luisgui1757/dotfiles Nix layer -- packages only. chezmoi owns every dotfile target on every OS (CLAUDE.md invariant 22). Native Windows is non-Nix.";

  # Inputs are pinned by the committed flake.lock. It is bumped only through
  # reviewed PRs (Renovate `nix` manager or an explicit `nix flake update`) --
  # never silently during setup/update.
  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";

    nix-darwin = {
      url = "github:nix-darwin/nix-darwin/master";
      inputs.nixpkgs.follows = "nixpkgs";
    };
    home-manager = {
      url = "github:nix-community/home-manager";
      inputs.nixpkgs.follows = "nixpkgs";
    };
    nix-homebrew.url = "github:zhaofengli/nix-homebrew";

    # Homebrew taps pinned as (non-flake) inputs so nix-homebrew can run with
    # mutableTaps = false (reproducible, no implicit tap mutation). These are the
    # source repos for the declarative casks/brews (WezTerm, AeroSpace, Herdr).
    homebrew-core = {
      url = "github:homebrew/homebrew-core";
      flake = false;
    };
    homebrew-cask = {
      url = "github:homebrew/homebrew-cask";
      flake = false;
    };
    homebrew-nikitabobko = {
      url = "github:nikitabobko/homebrew-tap";
      flake = false;
    };
  };

  outputs =
    inputs@{ self
    , nixpkgs
    , nix-darwin
    , home-manager
    , nix-homebrew
    , ...
    }:
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

      # The proving host's user is resolved impurely at switch time. sudo-based
      # nix-darwin activation leaves USER=root, so prefer SUDO_USER when present,
      # then USER, and keep runner only for pure eval / CI.
      resolvedUser =
        let
          sudoUser = builtins.getEnv "SUDO_USER";
          user = builtins.getEnv "USER";
        in
        if sudoUser != "" then sudoUser else if user != "" then user else "runner";

      mkDarwin =
        { username
        , system ? "aarch64-darwin"
        ,
        }:
        nix-darwin.lib.darwinSystem {
          inherit system;
          specialArgs = { inherit inputs username; };
          modules = [
            ./nix/darwin/configuration.nix
            nix-homebrew.darwinModules.nix-homebrew
            home-manager.darwinModules.home-manager
            {
              nix-homebrew = {
                enable = true;
                user = username;
                autoMigrate = true;
                mutableTaps = false;
                trust.taps = [
                  # Homebrew 5 refuses personal-tap casks unless the tap is
                  # explicitly trusted. This is the documented AeroSpace trust
                  # decision, and the tap itself is pinned as a flake input.
                  "nikitabobko/tap"
                ];
                taps = {
                  "homebrew/homebrew-core" = inputs.homebrew-core;
                  "homebrew/homebrew-cask" = inputs.homebrew-cask;
                  "nikitabobko/homebrew-tap" = inputs.homebrew-nikitabobko;
                };
              };

              home-manager = {
                useGlobalPkgs = true;
                useUserPackages = true;
                extraSpecialArgs = { inherit username; };
                users.${username} = import ./nix/home/darwin.nix;
              };
            }
          ];
        };

      # Standalone Home Manager for native Linux / WSL userland (packages only).
      # Activated on the real host by setup.sh's default POSIX package phase via
      # `home-manager switch --flake .#<arch>-linux` -- no root, WSL split-host
      # preserved (writes only to the Linux ~/.nix-profile, never /mnt/c).
      mkHome =
        system:
        home-manager.lib.homeManagerConfiguration {
          pkgs = nixpkgs.legacyPackages.${system};
          extraSpecialArgs = { username = resolvedUser; };
          modules = [ ./nix/home/linux.nix ];
        };
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
        toolchain = pkgs.runCommand "dotfiles-nix-toolchain" { } ''
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

      # macOS host configuration: nix-darwin + declarative Homebrew + Home Manager
      # (packages only). Activated on the real Mac by setup.sh's default POSIX
      # package phase via `darwin-rebuild switch --flake .#dotfiles`.
      darwinConfigurations."dotfiles" = mkDarwin { username = resolvedUser; };

      # Native Linux / WSL userland package sets (Home Manager, packages only).
      # Keyed by arch so setup.sh can `home-manager switch --flake .#$(uname -m)-linux`.
      homeConfigurations = {
        "x86_64-linux" = mkHome "x86_64-linux";
        "aarch64-linux" = mkHome "aarch64-linux";
      };
    };
}
