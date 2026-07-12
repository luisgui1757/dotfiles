# Dotfiles test/lint entry point. Runs everything that can run on the
# current OS; sub-targets skip themselves with a clear message when the
# tool they depend on isn't installed.

.PHONY: ci test-required test test-migration test-nvim test-shell test-starship test-tmux test-ghostty test-wezterm test-aerospace test-nix test-static validate-renovate lint setup setup-dryrun install dryrun deps deps-dryrun chezmoi chezmoi-diff help

REPO := $(shell pwd)

help:
	@echo "Targets:"
	@echo "  setup           — ONE-SHOT: deps + config + plugins + LSP (recommended)"
	@echo "  setup-dryrun    — preview every step of setup without acting"
	@echo "  ci              — full local pre-PR gate: test + Renovate + migration"
	@echo "  test-required   — alias for ci"
	@echo "  test            — run all test sub-targets (skips what's not installed)"
	@echo "  test-migration  — chezmoi template/parity/round-trip/oracle checks"
	@echo "  test-nvim       — plenary busted suite under nvim --headless"
	@echo "  test-shell      — shellcheck + zsh smoke + Esc-binding regression"
	@echo "  test-starship   — render snapshot + perf budget (80ms local / 150ms CI mean)"
	@echo "  test-tmux       — load + option assertions"
	@echo "  test-ghostty    — +validate-config + scheme grep (mac only)"
	@echo "  test-wezterm    — Lua smoke (stubbed require) + no-multiplexer-autolaunch"
	@echo "  test-aerospace  — reserved-chord + start-at-login config guards (mac WM)"
	@echo "  test-nix        — nix-darwin config eval (skips w/o nix) + setup --nix-darwin"
	@echo "  test-static     — json/toml/yaml lint, editorconfig, invariants"
	@echo "  validate-renovate — schema-check renovate.json under Node 24"
	@echo "  lint            — shellcheck everything"
	@echo
	@echo "Maintainer phase targets:"
	@echo "  deps            — phase 1 only: dependency install"
	@echo "  deps-dryrun     — preview phase 1"
	@echo "  install         — phase 2 only: apply configs with chezmoi"
	@echo "  dryrun          — preview phase 2 with chezmoi diff"
	@echo
	@echo "chezmoi (config layer):"
	@echo "  chezmoi         — apply the config layer with chezmoi (config only, no deps)"
	@echo "  chezmoi-diff    — preview what chezmoi would change (dry run)"

setup:
	@bash setup.sh

setup-dryrun:
	@bash setup.sh --dry-run

deps:
	@bash install-deps.sh

deps-dryrun:
	@bash install-deps.sh --dry-run

install:
	@chezmoi --source $(REPO)/home apply

dryrun:
	@chezmoi --source $(REPO)/home diff

# Config-only re-apply via chezmoi (the migrated config layer; no provisioning).
# Source tree is home/; .chezmoiroot lets remote `chezmoi init --apply` find it too.
chezmoi:
	@chezmoi --source $(REPO)/home apply

chezmoi-diff:
	@chezmoi --source $(REPO)/home diff

ci: test validate-renovate test-migration
	@echo
	@echo "=== ci summary: local pre-PR gate passed ==="

test-required: ci

test: test-static lint test-nvim test-shell test-starship test-tmux test-ghostty test-wezterm test-aerospace test-nix
	@echo
	@echo "=== test summary: see individual sub-target output above ==="

test-migration:
	@PATH="$$HOME/.local/bin:$$PATH" bash tests/migration/v0_1_upgrade_test.sh
	@PATH="$$HOME/.local/bin:$$PATH" bash tests/migration/template_test.sh
	@PATH="$$HOME/.local/bin:$$PATH" bash tests/migration/parity_gate.sh
	@PATH="$$HOME/.local/bin:$$PATH" bash tests/migration/greenfield_roundtrip.sh
	@PATH="$$HOME/.local/bin:$$PATH" bash tests/migration/uninstall_safety_test.sh
	@PATH="$$HOME/.local/bin:$$PATH" bash tests/migration/uninstall_backup_order_test.sh
	@PATH="$$HOME/.local/bin:$$PATH" bash tests/migration/windows_render_test.sh
	@PATH="$$HOME/.local/bin:$$PATH" bash tests/migration/oracle_test.sh

test-nvim:
	@bash tests/nvim/run.sh

test-shell:
	@bash tests/shell/run_all.sh

test-starship:
	@bash tests/starship/run_all.sh

test-tmux:
	@bash tests/tmux/run_all.sh

test-ghostty:
	@bash tests/ghostty/run_all.sh

test-wezterm:
	@bash tests/wezterm/run_all.sh

test-aerospace:
	@bash tests/aerospace/run_all.sh

test-nix:
	@bash tests/nix/run_all.sh

test-static:
	@bash tests/static/run_all.sh

validate-renovate:
	@bash scripts/validate-renovate.sh

lint:
	@bash tests/shell/lint.sh
