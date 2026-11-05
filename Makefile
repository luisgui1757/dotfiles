# Dotfiles test/lint entry point. Runs everything that can run on the
# current OS; sub-targets skip themselves with a clear message when the
# tool they depend on isn't installed.

.PHONY: ci test-installer test-bootstrap test-required test test-nvim test-shell test-starship test-tmux test-ghostty test-aerospace test-xcompose test-static validate-renovate lint release-check setup help

help:
	@echo "setup: launch the installer menu"
	@echo "ci / test-required: configuration, bootstrap, Go format/vet/race and Renovate gates"
	@echo "test: configuration/editor/shell checks; test-bootstrap: public launchers and release scripts"
	@echo "test-installer: Go installer behavior and recovery tests"
	@echo "release-check: validate the release manifest and its notes"

setup:
	@bash setup.sh

ci: test validate-renovate test-bootstrap test-installer
	@echo "=== ci summary: local pre-PR gate passed ==="

test-required: ci

test-bootstrap:
	@python3 -m unittest discover -s tests/bootstrap -p '*_test.py'

test-installer:
	@command -v go >/dev/null || { echo "FAIL: install the Go version declared in installer/go.mod to run the contributor gate" >&2; exit 1; }
	@cd installer && formatting=$$(gofmt -l .) && { test -z "$$formatting" || { printf 'Files requiring gofmt:\n%s\n' "$$formatting"; exit 1; }; }
	@cd installer && go vet ./...
	@cd installer && go test -race -timeout 30m ./...

test: test-static lint test-nvim test-shell test-starship test-tmux test-ghostty test-aerospace test-xcompose
	@echo
	@echo "=== test summary: see individual sub-target output above ==="

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

test-aerospace:
	@bash tests/aerospace/run_all.sh

test-xcompose:
	@python3 -m unittest discover -s tests/xcompose -p '*_test.py'

test-static:
	@bash tests/static/run_all.sh

validate-renovate:
	@bash scripts/validate-renovate.sh

lint:
	@bash tests/shell/lint.sh

release-check:
	@python3 scripts/release.py check
