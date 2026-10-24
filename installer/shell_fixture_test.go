package installer

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// The shell/Pester runners call this test-only exporter. It uses the production
// composer and isolated process/module fixtures; it never installs a package or
// changes the caller's profiles. No equivalent mode exists in the public CLI.
func TestWriteActiveShellFixtures(t *testing.T) {
	root := os.Getenv("DOTFILES_WRITE_SHELL_FIXTURES")
	if root == "" {
		t.Skip("shell/Pester runner supplies a fresh temporary export directory")
	}
	root, err := resolveConfigPath(root)
	if err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 0 {
		t.Fatal("shell fixtures require an existing empty directory", err)
	}
	variants := []string{"windows"}
	if runtime.GOOS != "windows" {
		variants = append(variants, "zsh", "posix")
	}
	for _, variant := range variants {
		home := filepath.Join(root, variant)
		folders := ConfigFolders{Home: home, Config: filepath.Join(home, "config"), Documents: filepath.Join(home, "documents"), Zsh: home}
		d := &ProfileDriver{Directory: filepath.Join(home, "state"), Targets: map[string][]ProfileTarget{}, Windows: variant == "windows"}
		a := &ArchiveDriver{Directory: filepath.Join(home, "packages"), Pins: map[string]ArchivePin{}}
		add := func(id, file, command, content string) {
			pin := ArchivePin{Version: "fixture", URL: "https://example.invalid/fixture.zip", SHA256: strings.Repeat("1", 64), Format: "zip", RequiredFiles: []string{file}}
			if command != "" {
				pin.Commands = map[string]string{command: file}
				pin.BinDirs = []string{"bin"}
			}
			a.Pins[id] = pin
			path := filepath.Join(a.currentLink(id), filepath.FromSlash(file))
			writeConfigFixture(t, path, content)
			if err := os.Chmod(path, 0700); err != nil {
				t.Fatal(err)
			}
		}
		target := Context{OS: "linux", Arch: "amd64"}
		selected := []string{"config.zsh", "integration.lsd"}
		if variant == "windows" {
			target.OS = "windows"
			selected = []string{"config.powershell", "integration.starship", "integration.fzf", "integration.lsd", "integration.zoxide"}
			add("tool.starship", "bin/starship.ps1", "starship", "$global:LASTEXITCODE = 0\n'function global:prompt { return \"fixture> \" }'\n")
			add("tool.zoxide", "bin/zoxide.ps1", "zoxide", "$global:LASTEXITCODE = 0\n'function global:z { }'\n")
			add("tool.fzf", "bin/fzf.ps1", "fzf", "# External picker fixture; no terminal picker is invoked.\n")
			add("powershell.psfzf", "PSFzf.psd1", "", "@{ RootModule = 'PSFzf.psm1'; ModuleVersion = '0.0.1'; FunctionsToExport = @('Set-PsFzfOption') }\n")
			writeConfigFixture(t, filepath.Join(a.currentLink("powershell.psfzf"), "PSFzf.psm1"), `function Set-PsFzfOption {
	[CmdletBinding()]
	param([string]$PSReadlineChordProvider, [string]$PSReadlineChordReverseHistory, [string]$PSReadlineChordSetLocation)
	$global:DotfilesFixtureFzf = @((Get-PSReadLineOption).EditMode.ToString(), $PSReadlineChordProvider, $PSReadlineChordReverseHistory, $PSReadlineChordSetLocation)
}
`)
		} else {
			add("plugin.fzf-tab", "fzf-tab.plugin.zsh", "", "fzf-tab-complete() { zle menu-select }\nzle -N fzf-tab-complete\n")
			add("plugin.zsh-autosuggestions", "zsh-autosuggestions.zsh", "", "_zsh_autosuggest_start() { :; }\n")
			if variant == "posix" {
				// The handoff stays inert in these runners: they start zsh, or bash
				// only for syntax checks, so both shells parse its generated code.
				selected = append(selected, "integration.starship", "integration.fzf", "integration.zoxide", "integration.zsh-handoff")
				add("tool.starship", "bin/starship", "starship", "#!/bin/sh\nprintf '%s\\n' 'export DOTFILES_FIXTURE_STARSHIP=loaded'\n")
				add("tool.zoxide", "bin/zoxide", "zoxide", "#!/bin/sh\nprintf '%s\\n' 'z() { :; }'\n")
				add("tool.fzf", "bin/fzf", "fzf", `#!/bin/sh
cat <<'ZSH'
fzf-completion() { zle menu-select }
zle -N fzf-completion
bindkey '^I' fzf-completion
fzf-history-widget() { zle history-incremental-search-backward }
zle -N fzf-history-widget
bindkey -M viins '^R' fzf-history-widget
bindkey -M vicmd '^R' fzf-history-widget
ZSH
`)
			}
		}
		if err := configureShellProfiles(d, target, folders, a, filepath.Join(home, "starship.toml"), selected); err != nil {
			t.Fatal(variant, err)
		}
		catalog := &Catalog{Schema: 1, Resources: []Resource{
			{ID: "fixture", Name: "Fixture", Capability: true, Requires: []string{"integration.shells"}},
			{ID: "integration.shells", Name: "Shell profiles", Action: "profile", Scope: "user"},
		}}
		if err := catalog.Validate(); err != nil {
			t.Fatal(err)
		}
		controller := Controller{Catalog: catalog, Context: target, Source: "shell-fixture", Home: home, StatePath: filepath.Join(d.Directory, "state.json"), Driver: d}
		dispatchApproved(t, controller, Request{Schema: 1, Mode: "apply", Selected: []string{"fixture"}})
	}
}
