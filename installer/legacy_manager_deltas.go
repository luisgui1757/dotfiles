package installer

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"maps"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

// Native-manager retirement deltas. The pre-overhaul main (ae9a6446) and the
// published v0.4.4 (05874e53) installed these choices through Homebrew on
// macOS and Linux, pinned .deb files on Ubuntu/Debian, and Scoop, winget,
// Chocolatey or pinned direct downloads on Windows, without receipts. The new
// installer replaces them with private payloads, or no longer supports the
// product. The exact released package identity plus a separate per-item
// approval is the only removal authority; matching command names never are.
// Every removal runs through the native manager that owns the package, after
// fresh identity, consumer and running-process gates, and records its outcome
// in the native worker journal. Manager metadata that runs scripts during
// uninstall (Scoop manifests, Chocolatey package scripts) must be provably the
// official repository's own bytes. Configuration, persisted data and sessions
// stay.
//
// Not deltas: Git and tmux on macOS are setup's reused Homebrew formulae, and
// macOS ships git, make, zsh, unzip, tar and python3 commands, so the released
// presence checks never requested those formulae. Git for Windows is shared
// developer infrastructure: Scoop updates its buckets with it and tools outside
// setup run it from PATH; setup's private PortableGit does not serve them.
// Packaged Windows Terminal (winget or Chocolatey MSIX) stays personal, like
// Store builds. Visual Studio Build Tools instances are reused by setup. The
// package managers themselves (Homebrew, Linuxbrew, Scoop, Chocolatey) and the
// devilspie2 distribution package can serve other software and stay; APT
// packages setup reuses are not deltas. chezmoi is retired by retire.chezmoi
// through these adapters.

const legacyDeltaPrefix = "retire.delta."

type legacyManagerDelta struct {
	Key, Name string
	// Replacement is the setup resource that now provides the tool; empty marks
	// a retired product without a replacement.
	Replacement  string
	Formula      string   // Homebrew core formula requested by install-deps.sh
	LinuxFormula string   // Linux-only formula when macOS uses a system command
	Cask         string   // Homebrew cask full token
	Bundle       string   // the cask application's bundle identifier
	Tap          string   // a tap added by the released nix-darwin configuration
	Scoop        string   // bucket/app
	Winget       string   // winget-source package identifier
	Choco        string   // Chocolatey package identifier
	Direct       []string // pinned Windows direct artifacts
	Processes    []string // Windows process names that must not be running
	Deb          string   // Linux dpkg package the released installer built from a pinned .deb
	LinuxDirect  []string // pinned Linux direct artifacts
}

// TestLegacyManagerDeltasMatchReleasedInstallers compares every identity and
// stated exclusion with both releases' install-deps.sh, install-deps.ps1 and
// nix-darwin configuration.
var legacyManagerDeltas = []legacyManagerDelta{
	{Key: "nvim", Name: "Neovim", Replacement: "tool.nvim", Formula: "neovim", Scoop: "main/neovim", Winget: "Neovim.Neovim", Choco: "neovim", Processes: []string{"nvim"}},
	{Key: "cmake", Name: "CMake", Replacement: "tool.cmake", Formula: "cmake", Scoop: "main/cmake", Winget: "Kitware.CMake", Choco: "cmake", Processes: []string{"cmake"}},
	{Key: "ripgrep", Name: "ripgrep", Replacement: "tool.rg", Formula: "ripgrep", Scoop: "main/ripgrep", Winget: "BurntSushi.ripgrep.MSVC", Choco: "ripgrep", Processes: []string{"rg"}},
	{Key: "fd", Name: "fd", Replacement: "tool.fd", Formula: "fd", Scoop: "main/fd", Winget: "sharkdp.fd", Choco: "fd", Processes: []string{"fd"}},
	{Key: "fzf", Name: "fzf", Replacement: "tool.fzf", Formula: "fzf", Scoop: "main/fzf", Winget: "junegunn.fzf", Choco: "fzf", Processes: []string{"fzf"}},
	{Key: "lsd", Name: "lsd", Replacement: "tool.lsd", Formula: "lsd", Scoop: "main/lsd", Winget: "lsd-rs.lsd", Choco: "lsd", Processes: []string{"lsd"}},
	{Key: "zoxide", Name: "zoxide", Replacement: "tool.zoxide", Formula: "zoxide", Scoop: "main/zoxide", Winget: "ajeetdsouza.zoxide", Choco: "zoxide", Processes: []string{"zoxide"}},
	{Key: "lazygit", Name: "LazyGit", Replacement: "tool.lazygit", Formula: "lazygit", Scoop: "main/lazygit", Winget: "JesseDuffield.lazygit", Choco: "lazygit", Processes: []string{"lazygit"}},
	{Key: "starship", Name: "Starship", Replacement: "tool.starship", Formula: "starship", Scoop: "main/starship", Winget: "Starship.Starship", Choco: "starship", Processes: []string{"starship"}},
	{Key: "node", Name: "Node.js", Replacement: "tool.node", Formula: "node", Scoop: "main/nodejs-lts", Winget: "OpenJS.NodeJS.LTS", Choco: "nodejs-lts", Processes: []string{"node"}},
	{Key: "python", Name: "Python", Replacement: "tool.python", LinuxFormula: "python@3.12", Scoop: "main/python", Winget: "Python.Python.3.12", Choco: "python", Processes: []string{"python", "pythonw"}},
	{Key: "tree-sitter", Name: "tree-sitter CLI", Replacement: "tool.tree-sitter", Formula: "tree-sitter-cli", Direct: []string{"tree-sitter"}, Processes: []string{"tree-sitter"}},
	{Key: "shellcheck", Name: "ShellCheck", Replacement: "tool.shellcheck", Formula: "shellcheck", Scoop: "main/shellcheck", Winget: "koalaman.shellcheck", Choco: "shellcheck", Processes: []string{"shellcheck"}},
	{Key: "jq", Name: "jq", Replacement: "tool.jq", Formula: "jq", Scoop: "main/jq", Winget: "jqlang.jq", Choco: "jq", Processes: []string{"jq"}},
	{Key: "gh", Name: "GitHub CLI", Replacement: "tool.gh", Formula: "gh", Scoop: "main/gh", Winget: "GitHub.cli", Choco: "gh", Processes: []string{"gh"}},
	{Key: "hyperfine", Name: "hyperfine", Replacement: "tool.hyperfine", Formula: "hyperfine", Scoop: "main/hyperfine", Winget: "sharkdp.hyperfine", Choco: "hyperfine", Processes: []string{"hyperfine"}},
	{Key: "taplo", Name: "Taplo", Replacement: "tool.taplo", Formula: "taplo", Scoop: "main/taplo", Processes: []string{"taplo"}},
	{Key: "yamllint", Name: "yamllint", Replacement: "tool.yamllint", Formula: "yamllint"},
	{Key: "editorconfig", Name: "editorconfig-checker", Replacement: "tool.editorconfig", Formula: "editorconfig-checker"},
	{Key: "herdr", Name: "Herdr", Replacement: "tool.herdr", Formula: "herdr", Direct: []string{"herdr"}, Processes: []string{"herdr"}},
	{Key: "fontconfig", Name: "fontconfig", Formula: "fontconfig"},
	{Key: "make", Name: "GNU Make", Replacement: "tool.make", LinuxFormula: "make", Scoop: "main/make", Winget: "GnuWin32.Make", Choco: "make", Processes: []string{"make"}},
	{Key: "git", Name: "Git", Replacement: "tool.git", LinuxFormula: "git"},
	{Key: "tmux", Name: "tmux", Replacement: "tool.tmux", LinuxFormula: "tmux"},
	{Key: "zsh", Name: "zsh", Replacement: "tool.zsh", LinuxFormula: "zsh"},
	{Key: "unzip", Name: "unzip", LinuxFormula: "unzip"},
	{Key: "powershell", Name: "PowerShell 7", Replacement: "tool.powershell", Scoop: "main/pwsh", Winget: "Microsoft.PowerShell", Choco: "powershell-core", Processes: []string{"pwsh"}},
	{Key: "psfzf", Name: "PSFzf module", Replacement: "powershell.psfzf", Direct: []string{"psfzf"}},
	{Key: "clipboard", Name: "win32yank", Replacement: "tool.clipboard", Scoop: "main/win32yank", Choco: "win32yank", Processes: []string{"win32yank"}},
	{Key: "zig", Name: "Zig", Scoop: "main/zig", Winget: "zig.zig", Choco: "zig", Processes: []string{"zig"}},
	{Key: "latex2text", Name: "pylatexenc converter", Replacement: "tool.latex2text", Direct: []string{"pylatexenc"}},
	{Key: "vscode", Name: "Visual Studio Code", Replacement: "tool.vscode", Cask: "visual-studio-code", Bundle: "com.microsoft.VSCode", Scoop: "extras/vscode", Winget: "Microsoft.VisualStudioCode", Choco: "vscode", Processes: []string{"Code"}},
	{Key: "ghostty", Name: "Ghostty", Replacement: "tool.ghostty", Cask: "ghostty", Bundle: "com.mitchellh.ghostty", Deb: "ghostty"},
	{Key: "wezterm", Name: "WezTerm", Cask: "wezterm", Bundle: "com.github.wez.wezterm", Scoop: "extras/wezterm", Winget: "wez.wezterm", Choco: "wezterm", Processes: []string{"wezterm", "wezterm-gui", "wezterm-mux-server"}, Deb: "wezterm"},
	{Key: "aerospace", Name: "AeroSpace", Replacement: "tool.aerospace", Cask: "nikitabobko/tap/aerospace", Bundle: "bobko.aerospace", Tap: "nikitabobko/tap"},
	{Key: "windows-terminal", Name: "unpackaged Windows Terminal", Replacement: "tool.windows-terminal", Scoop: "extras/windows-terminal", Direct: []string{"windows-terminal"}, Processes: []string{"WindowsTerminal"}},
	// `winget install psmux` resolves through the moniker of marlocarlo.psmux.
	{Key: "psmux", Name: "psmux", Scoop: "psmux/psmux", Winget: "marlocarlo.psmux", Choco: "psmux", Direct: []string{"psmux-plugins"}, Processes: []string{"psmux", "pmux", "tmux"}},
	{Key: "font", Name: "Hack Nerd Font", Replacement: "font.hack", Cask: "font-hack-nerd-font", Scoop: "nerd-fonts/Hack-NF", Direct: []string{"hack-font"}, LinuxDirect: []string{"hack-font"}},
	// The Ghostty maximize rule link and autostart entry. The devilspie2
	// distribution package may serve other rules and stays.
	{Key: "devilspie2", Name: "Ghostty maximize rule for devilspie2", LinuxDirect: []string{"devilspie2"}},
}

var legacyChezmoiPackages = legacyManagerDelta{Key: "chezmoi", Name: "chezmoi", Formula: "chezmoi", Scoop: "main/chezmoi", Winget: "twpayne.chezmoi", Choco: "chezmoi", Processes: []string{"chezmoi"}}

func (delta legacyManagerDelta) platforms() []string {
	platforms := []string{}
	if delta.Formula != "" || delta.Cask != "" || delta.Tap != "" {
		platforms = append(platforms, "darwin")
	}
	if delta.Formula != "" || delta.LinuxFormula != "" || delta.Deb != "" || len(delta.LinuxDirect) != 0 {
		platforms = append(platforms, "linux")
	}
	if delta.Scoop != "" || delta.Winget != "" || delta.Choco != "" || len(delta.Direct) != 0 {
		platforms = append(platforms, "windows")
	}
	return platforms
}

// legacyManagerDeltaResources are separately approvable retirement items, one
// per released tool, in the runtime phase after verified replacement setup.
func legacyManagerDeltaResources() []Resource {
	resources := []Resource{}
	for _, delta := range legacyManagerDeltas {
		scope := "user"
		if delta.Formula != "" || delta.LinuxFormula != "" || delta.Cask != "" || delta.Winget != "" || delta.Choco != "" || delta.Deb != "" {
			scope = "machine"
		}
		description := "Remove the earlier setup's " + delta.Name + ": packages through the manager that installed them, released files only while they match their pins;"
		if delta.Replacement == "" {
			description += " it is no longer part of setup."
		} else {
			description += " setup now provides it privately."
		}
		resources = append(resources, Resource{ID: legacyDeltaPrefix + delta.Key, Name: "Remove the earlier " + delta.Name,
			Description: description + " Personal configuration, persisted data and sessions stay.", Capability: true, Platforms: delta.platforms(),
			Requires: []string{legacyReplacements}, Action: "retire", Scope: scope, Retain: true})
	}
	return resources
}

func (d *legacyRetirementDriver) inspectManagerDelta(ctx context.Context, id string, scope *legacyRetirementScope) error {
	index := slices.IndexFunc(legacyManagerDeltas, func(delta legacyManagerDelta) bool { return legacyDeltaPrefix+delta.Key == id })
	if index < 0 || !slices.Contains(legacyManagerDeltas[index].platforms(), d.Platform.OS) {
		return fmt.Errorf("unknown legacy retirement resource %q", id)
	}
	delta := legacyManagerDeltas[index]
	if delta.Key == "font" {
		selected, err := d.replacementSelected("tool.font")
		if err != nil {
			return err
		}
		if selected {
			state, err := LoadState(d.Normal.StatePath, d.Normal.Home)
			if err != nil {
				return err
			}
			receipt := state.Receipts["tool.font"]
			if receipt.Ownership == "reused" && receipt.After.Provider == "native-font" {
				scope.Detail = append(scope.Detail, "The existing Hack Nerd Font family is reused by the selected setup and stays installed.")
				return nil
			}
		}
	}
	var err error
	switch d.Platform.OS {
	case "darwin":
		_, err = d.planBrewPackages(ctx, scope, delta)
	case "linux":
		if delta.LinuxFormula != "" {
			delta.Formula = delta.LinuxFormula
		}
		delta.Cask, delta.Tap, delta.Bundle = "", "", ""
		var installed bool
		if installed, err = d.planBrewPackages(ctx, scope, delta); err == nil {
			if err = d.planDebPackage(ctx, scope, delta); err == nil {
				err = d.planLinuxDirect(scope, delta)
			}
		}
		if err == nil && delta.Key == "zsh" && !installed {
			// A login can still name the formula after an earlier removal.
			return d.planBrewZshRepair(ctx, scope)
		}
	case "windows":
		if _, err = d.planWindowsPackages(ctx, scope, delta); err == nil {
			err = d.planWindowsDirect(ctx, scope, delta)
		}
	}
	if err != nil || len(scope.Steps) == 0 || scope.Blocked != "" {
		return err
	}
	if delta.Replacement == "" {
		scope.Detail = append(scope.Detail, delta.Name+" is no longer part of setup and has no replacement.")
		return nil
	}
	return d.disclosePrerequisite(scope, delta.Replacement, delta.Name+" is not selected in setup; its command disappears after retirement.")
}

// One preview inspects many items in a burst, and native inventories are slow.
// Inspection may reuse one read for a short interval. Every gate and check reads
// fresh evidence and replaces the entry, so dispatch never relies on it.
var legacyDeltaMemo = struct {
	sync.Mutex
	entries map[string]legacyMemoEntry
}{entries: map[string]legacyMemoEntry{}}

type legacyMemoEntry struct {
	at    time.Time
	value any
	err   error
}

const legacyDeltaMemoAge = 20 * time.Second

func legacyMemoized[T any](ctx context.Context, key string, fresh bool, load func() (T, error)) (T, error) {
	legacyDeltaMemo.Lock()
	entry, ok := legacyDeltaMemo.entries[key]
	legacyDeltaMemo.Unlock()
	if !fresh && ok && time.Since(entry.at) < legacyDeltaMemoAge {
		value, _ := entry.value.(T)
		return value, entry.err
	}
	value, err := load()
	if ctx.Err() == nil {
		legacyDeltaMemo.Lock()
		legacyDeltaMemo.entries[key] = legacyMemoEntry{at: time.Now(), value: value, err: err}
		legacyDeltaMemo.Unlock()
	}
	return value, err
}

// Homebrew.

func (d *legacyRetirementDriver) brewDeltaInventory(ctx context.Context, fresh bool) (map[string]nativePackage, error) {
	location := d.Platform.Homebrew
	if location == nil {
		return nil, errors.New("Homebrew is not installed")
	}
	return legacyMemoized(ctx, "homebrew\x00"+location.Program+"\x00"+location.Cellar, fresh, func() (map[string]nativePackage, error) {
		return brewInventory(ctx, location.query)
	})
}

// The identity binds every installed version, the requesting tap, whether
// Homebrew records the formula as requested, and its pin.
func legacyBrewIdentity(pkg nativePackage) string {
	if name, cask := strings.CutPrefix(pkg.Name, "cask/"); cask {
		return fmt.Sprintf("cask %s source=%s version=%s", name, pkg.Source, pkg.Version)
	}
	return fmt.Sprintf("formula %s source=%s versions=%s requested=%t pinned=%t", pkg.Name, pkg.Source, pkg.Version, !pkg.Automatic, pkg.Held)
}

// Installed runtime edges, including casks' formula dependencies, protect a
// formula. A tap is in use while any installed package comes from it.
func legacyBrewConsumers(inventory map[string]nativePackage, subject string, except []string) []string {
	consumers := []string{}
	tap, isTap := strings.CutPrefix(subject, "tap:")
	for key, pkg := range inventory {
		if key == subject || slices.Contains(except, key) {
			continue
		}
		if isTap && strings.HasPrefix(strings.ToLower(pkg.Source), strings.ToLower(tap)+"/") || !isTap && slices.Contains(pkg.Dependencies, subject) {
			consumers = append(consumers, key)
		}
	}
	slices.Sort(consumers)
	return consumers
}

func (d *legacyRetirementDriver) brewTaps(ctx context.Context) ([]string, error) {
	output, err := d.Platform.Homebrew.query(ctx, false, "brew", nil, "tap")
	if err != nil {
		return nil, err
	}
	return strings.Fields(string(output)), nil
}

func (d *legacyRetirementDriver) setupBrewFormula(name string) bool {
	if d.Normal.Catalog == nil {
		return false
	}
	return slices.ContainsFunc(d.Normal.Catalog.Resources, func(r Resource) bool {
		binding := r.Bindings[d.Platform.OS]
		return binding.Provider == "homebrew-formula" && binding.Package == name
	})
}

// planBrewPackages reports whether a released Homebrew package is installed.
// Only Homebrew's own uninstall runs; dependents keep a formula installed, and
// Homebrew's automatic cleanup, autoremove and dependency overrides stay off.
func (d *legacyRetirementDriver) planBrewPackages(ctx context.Context, scope *legacyRetirementScope, delta legacyManagerDelta) (bool, error) {
	if delta.Formula == "" && delta.Cask == "" && delta.Tap == "" {
		return false, nil // A Homebrew problem cannot affect a package Homebrew never had.
	}
	if d.Platform.HomebrewIssue != "" {
		return false, errors.New(d.Platform.HomebrewIssue)
	}
	location := d.Platform.Homebrew
	if location == nil {
		return false, nil
	}
	type candidate struct{ key, source, kind, bundle, keg string }
	candidates := []candidate{}
	if delta.Formula != "" {
		candidates = append(candidates, candidate{delta.Formula, delta.Formula, "formula", "", filepath.Join(location.Cellar, delta.Formula)})
	}
	if delta.Cask != "" {
		token := delta.Cask[strings.LastIndex(delta.Cask, "/")+1:]
		candidates = append(candidates, candidate{"cask/" + token, delta.Cask, "cask", delta.Bundle, filepath.Join(location.Prefix, "Caskroom", token)})
	}
	present := delta.Tap != ""
	for _, c := range candidates {
		if _, err := os.Lstat(c.keg); err == nil {
			present = true
		} else if !errors.Is(err, os.ErrNotExist) {
			return false, err
		}
	}
	if !present {
		return false, nil
	}
	inventory, err := d.brewDeltaInventory(ctx, false)
	if err != nil {
		return false, err
	}
	installed, removed := false, []string{}
	for _, c := range candidates {
		pkg, ok := inventory[c.key]
		if !ok {
			if _, err := os.Lstat(c.keg); err == nil {
				scope.Retained = append(scope.Retained, c.keg+" exists, but Homebrew does not list "+c.source+" as installed; it was preserved")
				installed = true
			}
			continue
		}
		label := "Homebrew " + c.kind + " " + c.source
		switch {
		case c.kind == "formula" && d.setupBrewFormula(c.key):
			scope.Detail = append(scope.Detail, label+" is reused by setup and stays.")
			continue
		case !strings.EqualFold(pkg.Source, c.source):
			scope.Detail = append(scope.Detail, "Homebrew "+c.kind+" "+pkg.Source+" is not the released "+c.source+"; it stays.")
			continue
		case pkg.Automatic:
			scope.Detail = append(scope.Detail, label+" is installed only as another package's dependency; it stays.")
			continue
		}
		installed = true
		if pkg.Held {
			scope.Retained = append(scope.Retained, label+" is pinned; unpin it to retire it, or keep it")
			continue
		}
		if consumers := legacyBrewConsumers(inventory, c.key, nil); len(consumers) != 0 {
			scope.Retained = append(scope.Retained, label+" is still required by "+strings.Join(consumers, ", ")+"; it was preserved")
			continue
		}
		dependents, err := d.legacyGitRuntimeDependents(c.keg)
		if err != nil {
			return installed, err
		}
		if len(dependents) != 0 {
			scope.Retained = append(scope.Retained, label+" provides the only Git retirement can use to prove the released checkouts "+strings.Join(dependents, ", ")+"; retire those first or select setup's Git, so it was preserved")
			continue
		}
		arguments := []string{"uninstall", "--cask", c.source}
		if c.kind == "formula" {
			// --force removes every installed version; dependents still block it.
			arguments = []string{"uninstall", "--formula", "--force", c.key}
		}
		command := d.brewCommand(location.Program, arguments...)
		step := legacyRetirementStep{Kind: "command", Label: "Uninstall " + label + " (" + pkg.Version + ")", Command: &command,
			Gates:  []legacyRetirementGate{{Kind: "brew-package", Subject: c.key, Expected: legacyBrewIdentity(pkg)}, {Kind: "brew-consumers", Subject: c.key}, {Kind: "query-git", Subject: c.keg}},
			Checks: []legacyRetirementGate{{Kind: "brew-package", Subject: c.key, Expected: "absent"}}}
		if c.bundle != "" {
			// Homebrew quits a running application by bundle identifier, which
			// would also close setup's copy and any sessions inside it.
			step.Gates = append(step.Gates, legacyRetirementGate{Kind: "mac-app", Subject: c.bundle, Expected: "not-running"})
			scope.Detail = append(scope.Detail, "Quit every "+delta.Name+" window, including setup's copy, before this removal; it is refused while one runs.")
		}
		if d.Platform.OS == "linux" && c.key == "zsh" {
			// The released Linux installer made this formula the login shell.
			planned, err := d.planBrewZshRemoval(ctx, scope, location, step)
			if err != nil {
				return installed, err
			}
			if planned {
				removed = append(removed, c.key)
			}
			continue
		}
		scope.Steps = append(scope.Steps, step)
		removed = append(removed, c.key)
	}
	if delta.Tap != "" {
		taps, err := d.brewTaps(ctx)
		if err != nil {
			return installed, err
		}
		if slices.ContainsFunc(taps, func(tap string) bool { return strings.EqualFold(tap, delta.Tap) }) {
			installed = true
			subject := "tap:" + delta.Tap
			if consumers := legacyBrewConsumers(inventory, subject, removed); len(consumers) != 0 {
				scope.Retained = append(scope.Retained, "Homebrew tap "+delta.Tap+" still provides "+strings.Join(consumers, ", ")+"; it was preserved")
			} else {
				command := d.brewCommand(location.Program, "untap", delta.Tap)
				scope.Steps = append(scope.Steps, legacyRetirementStep{Kind: "command", Label: "Remove the released Homebrew tap " + delta.Tap, Command: &command,
					Gates:  []legacyRetirementGate{{Kind: "brew-tap", Subject: delta.Tap, Expected: "present"}, {Kind: "brew-consumers", Subject: subject}},
					Checks: []legacyRetirementGate{{Kind: "brew-tap", Subject: delta.Tap, Expected: "absent"}}})
			}
		}
	}
	if len(scope.Steps) != 0 {
		scope.Detail = append(scope.Detail, "Dependencies left unused stay installed; Homebrew autoremove, cleanup and zap are never run.")
	}
	return installed, nil
}

// Debian packages the released Linux installer built from pinned upstream
// .deb files with `apt-get install ./file.deb`. The exact package version and
// the bytes of its main executable prove the pinned file; package name alone
// never does. Executable hashes were taken from the pinned .deb data members.
type legacyDebPin struct {
	Package, Architecture, Version, Executable, SHA256 string
	URL, Archive                                       string
}

var legacyDebPins = []legacyDebPin{
	{"wezterm", "amd64", "20240203-110809-5046fc22", "/usr/bin/wezterm-gui", "047f8dfea95b43108cfb85981ff3c4009c6746d921399327c0724cc3b3d327d1",
		"https://github.com/wezterm/wezterm/releases/download/20240203-110809-5046fc22/wezterm-20240203-110809-5046fc22.Ubuntu22.04.deb", "86358dab5794a4fb63f7c91dd68d4fdc3da58faad648a58fc77d2bd51c7b0686"},
	{"ghostty", "amd64", "1.3.1-0~ppa2", "/usr/bin/ghostty", "c42823821ede0d5aeb77db446bdbea7f1fbfcdbedd77efbde43dea5ec66a697b",
		"https://github.com/mkasberg/ghostty-ubuntu/releases/download/1.3.1-0-ppa2/ghostty_1.3.1-0.ppa2_amd64_24.04.deb", "478d440153ef544426418efc7d6d8901715359f452c46be29071901a94b8cd47"},
	{"ghostty", "arm64", "1.3.1-0~ppa2", "/usr/bin/ghostty", "ad9ffbdb58ca9d8681a180052d845a71f7542b198a74928760b5ecf40e85f274",
		"https://github.com/mkasberg/ghostty-ubuntu/releases/download/1.3.1-0-ppa2/ghostty_1.3.1-0.ppa2_arm64_24.04.deb", "91063815b6ce3d834d59714b4ad0310f744448b6716836d035b3d331d1923363"},
	{"ghostty", "amd64", "1.3.1-0~ppa2", "/usr/bin/ghostty", "4cc835a2094539512c4812c5ab619f026ad0bb669e0970e3a31c3c334b890682",
		"https://github.com/mkasberg/ghostty-ubuntu/releases/download/1.3.1-0-ppa2/ghostty_1.3.1-0.ppa2_amd64_25.10.deb", "793bde1c31163d8e1d12ea939c8b941f7908170e57bbf19b121434a0f6621c59"},
	{"ghostty", "arm64", "1.3.1-0~ppa2", "/usr/bin/ghostty", "3ec5810422f76e20731ec37a893cdd7a2752fb69d0c6b42eef2267ab72477a95",
		"https://github.com/mkasberg/ghostty-ubuntu/releases/download/1.3.1-0-ppa2/ghostty_1.3.1-0.ppa2_arm64_25.10.deb", "c6a4fd4fd786b4bdea42036650ef1724f535c4b636329f488f7ece36820d3d6b"},
	{"ghostty", "amd64", "1.3.1-0~ppa2", "/usr/bin/ghostty", "dd2a01832c20c6adae8bca4f812105b71ad86f9a64941a436710ccdb8e7ccc33",
		"https://github.com/mkasberg/ghostty-ubuntu/releases/download/1.3.1-0-ppa2/ghostty_1.3.1-0.ppa2_amd64_trixie.deb", "9fda8e418d7a7f58149ba3ba823a255d6b80f8bb5431b3bd7e912ff597715b2e"},
	{"ghostty", "arm64", "1.3.1-0~ppa2", "/usr/bin/ghostty", "8fb5e770c50fb08432e1cb9dc5d9f5d0c13d442d9b014c73016e0fed751c4f51",
		"https://github.com/mkasberg/ghostty-ubuntu/releases/download/1.3.1-0-ppa2/ghostty_1.3.1-0.ppa2_arm64_trixie.deb", "73f384e62c419d7a7809d686bf579fea5e23f52742b34f70c74d6adf0e72f8ab"},
}

type legacyDebState struct{ Package, Architecture, Version, Status string }

// dpkgPackages reads dpkg's own database through dpkg-query. Exit status 1 is
// dpkg-query's documented "no package matched"; every other failure stays one.
func (d *legacyRetirementDriver) dpkgPackages(ctx context.Context, name string) ([]legacyDebState, error) {
	output, err := d.Query(ctx, d.systemCommand(false, "/usr/bin/dpkg-query", "--show", "--showformat=${binary:Package}\t${Architecture}\t${Version}\t${db:Status-Abbrev}\n", "--", name))
	if code, known := legacyExitCode(err); known && code == 1 && len(bytes.TrimSpace(output)) == 0 {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	states := []legacyDebState{}
	for _, line := range strings.Split(strings.TrimSuffix(string(output), "\n"), "\n") {
		fields := strings.Split(line, "\t")
		if len(fields) != 4 || !legacyText(fields[0]) || !legacyText(fields[2]) || len(fields[3]) < 2 {
			return nil, errors.New("dpkg-query returned an unexpected package record")
		}
		// Only installed payloads count; "rc" keeps nothing but conffiles.
		if fields[3][1] != 'n' && fields[3][1] != 'c' {
			states = append(states, legacyDebState{fields[0], fields[1], fields[2], strings.TrimSpace(fields[3])})
		}
	}
	return states, nil
}

func (d *legacyRetirementDriver) debIdentity(ctx context.Context, name string) (string, *legacyDebPin, error) {
	states, err := d.dpkgPackages(ctx, name)
	if err != nil || len(states) == 0 {
		return "absent", nil, err
	}
	if len(states) != 1 {
		return fmt.Sprintf("%d installed architectures", len(states)), nil, nil
	}
	state := states[0]
	identity := fmt.Sprintf("package=%s architecture=%s version=%s status=%s", state.Package, state.Architecture, state.Version, state.Status)
	index := slices.IndexFunc(legacyDebPins, func(pin legacyDebPin) bool {
		return pin.Package == name && pin.Architecture == state.Architecture && pin.Version == state.Version
	})
	if index < 0 {
		return identity, nil, nil
	}
	executable := legacyDebPins[index].Executable
	if info, err := os.Lstat(executable); err != nil || !info.Mode().IsRegular() {
		return identity + " executable=absent", nil, nil
	}
	actual, err := legacyFileSHA256(executable)
	if err != nil {
		return "", nil, err
	}
	identity += " executable=" + actual
	// Each distribution build of the same version has its own pinned bytes.
	for i, pin := range legacyDebPins {
		if pin.Package == name && pin.Architecture == state.Architecture && pin.Version == state.Version && pin.SHA256 == actual {
			return identity, &legacyDebPins[i], nil
		}
	}
	return identity, nil, nil
}

// Installed packages whose Depends or Pre-Depends name the package directly.
// dpkg --remove also refuses removal while a dependency is unsatisfied.
func (d *legacyRetirementDriver) debConsumers(ctx context.Context, name string) ([]string, error) {
	output, err := d.Query(ctx, d.systemCommand(false, "/usr/bin/dpkg-query", "--show", "--showformat=${binary:Package}\t${db:Status-Abbrev}\t${Depends}, ${Pre-Depends}\n"))
	if err != nil {
		return nil, err
	}
	consumers := []string{}
	for _, line := range strings.Split(strings.TrimSuffix(string(output), "\n"), "\n") {
		fields := strings.SplitN(line, "\t", 3)
		if len(fields) != 3 || len(fields[1]) < 2 {
			return nil, errors.New("dpkg-query returned an unexpected dependency record")
		}
		if fields[1][1] == 'n' || fields[1][1] == 'c' || strings.Split(fields[0], ":")[0] == name {
			continue
		}
		for _, alternatives := range strings.Split(fields[2], ",") {
			for _, alternative := range strings.Split(alternatives, "|") {
				dependency := strings.Fields(strings.SplitN(alternative, "(", 2)[0])
				if len(dependency) == 1 && strings.Split(dependency[0], ":")[0] == name {
					consumers = append(consumers, fields[0])
				}
			}
		}
	}
	return sortedUnique(consumers), nil
}

func (d *legacyRetirementDriver) planDebPackage(ctx context.Context, scope *legacyRetirementScope, delta legacyManagerDelta) error {
	if delta.Deb == "" {
		return nil
	}
	if _, err := os.Lstat("/usr/bin/dpkg-query"); errors.Is(err, os.ErrNotExist) {
		return nil
	} else if err != nil {
		return err
	}
	identity, pin, err := d.debIdentity(ctx, delta.Deb)
	if err != nil || identity == "absent" {
		return err
	}
	if pin == nil {
		scope.Retained = append(scope.Retained, "the installed "+delta.Deb+" package ("+identity+") is not the released pinned "+delta.Name+" .deb; it was preserved")
		return nil
	}
	consumers, err := d.debConsumers(ctx, delta.Deb)
	if err != nil {
		return err
	}
	if len(consumers) != 0 {
		scope.Retained = append(scope.Retained, "the released "+delta.Deb+" package is still required by "+strings.Join(consumers, ", ")+"; it was preserved")
		return nil
	}
	// dpkg --remove keeps conffiles, runs the package's own prerm/postrm and
	// refuses to break another package's dependency.
	command := nativeCommand{Program: "/usr/bin/env", Arguments: []string{"-i", "PATH=/usr/sbin:/usr/bin:/sbin:/bin", "LC_ALL=C", "DEBIAN_FRONTEND=noninteractive", "/usr/bin/dpkg", "--remove", delta.Deb}}
	scope.Steps = append(scope.Steps, legacyRetirementStep{Kind: "command", Label: "Remove the released " + delta.Name + " package " + pin.Version + " (" + pin.Architecture + ") with dpkg; its configuration files stay", Privileged: true, Command: &command,
		Gates:  []legacyRetirementGate{{Kind: "dpkg-package", Subject: delta.Deb, Expected: identity}, {Kind: "dpkg-consumers", Subject: delta.Deb}},
		Checks: []legacyRetirementGate{{Kind: "dpkg-package", Subject: delta.Deb, Expected: "absent"}}})
	scope.Detail = append(scope.Detail, "Packages APT installed for "+delta.Name+"'s dependencies stay; APT autoremove and purge are never run. A running "+delta.Name+" keeps running until it exits.")
	return nil
}

// Released Linux direct artifacts other than marker-proved binaries.
func (d *legacyRetirementDriver) planLinuxDirect(scope *legacyRetirementScope, delta legacyManagerDelta) error {
	for _, recipe := range delta.LinuxDirect {
		var err error
		switch recipe {
		case "hack-font":
			err = d.planLinuxHackFont(scope)
		case "devilspie2":
			err = d.planDevilspie2Rule(scope)
		default:
			err = fmt.Errorf("unknown released Linux artifact %q", recipe)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// `unzip -o Hack.zip -d ${XDG_DATA_HOME:-~/.local/share}/fonts/HackNerdFont`.
// fontconfig notices the removal through the directory's changed time.
func (d *legacyRetirementDriver) planLinuxHackFont(scope *legacyRetirementScope) error {
	data := d.Folders.Data
	if data == "" {
		data = filepath.Join(d.Folders.Home, ".local", "share")
	}
	root := filepath.Join(data, "fonts", "HackNerdFont")
	entries, err := inspectTree(root, maxConfigBytes, maxConfigEntries)
	if err != nil {
		scope.Retained = append(scope.Retained, legacyReason(root+" could not be recorded; it was preserved", err))
		return nil
	}
	if len(entries) == 0 {
		return nil
	}
	release, difference := legacyMatchingTree(entries, legacyHackFontReleases)
	if release == nil {
		scope.Retained = append(scope.Retained, root+" is not exactly a released Hack Nerd Font archive ("+difference+"); it was preserved")
		return nil
	}
	if err := addLegacyDiscard(scope, "Remove the released Hack Nerd Font "+release.Version+" files", root); err != nil {
		return err
	}
	scope.Detail = append(scope.Detail, "Setup's own Dotfiles-HackNerdFont faces stay; fontconfig rescans the changed font directory.")
	return nil
}

const legacyDevilspie2Autostart = "[Desktop Entry]\nType=Application\nName=devilspie2\nExec=devilspie2\nX-GNOME-Autostart-enabled=true\n"

// The released installer linked <checkout>/linux/devilspie2/ghostty-maximize.lua
// into ${XDG_CONFIG_HOME:-~/.config}/devilspie2 and wrote a fixed autostart
// entry. Only that link and those exact bytes go; other rules stay.
func (d *legacyRetirementDriver) planDevilspie2Rule(scope *legacyRetirementScope) error {
	config := d.Folders.Config
	if config == "" {
		config = filepath.Join(d.Folders.Home, ".config")
	}
	rules := filepath.Join(config, "devilspie2")
	link := filepath.Join(rules, "ghostty-maximize.lua")
	before := len(scope.Steps)
	target, err := os.Readlink(link)
	switch {
	case errors.Is(err, os.ErrNotExist):
	case err != nil:
		scope.Retained = append(scope.Retained, link+" is not the released rule link; it was preserved")
	default:
		checkout := filepath.Dir(filepath.Dir(filepath.Dir(target)))
		origin, originErr := legacyGitOrigin(checkout)
		if !filepath.IsAbs(target) || filepath.ToSlash(strings.TrimPrefix(target, checkout)) != "/linux/devilspie2/ghostty-maximize.lua" ||
			checkout != d.Repository && (originErr != nil || !legacyDotfilesOrigin(origin)) {
			scope.Retained = append(scope.Retained, legacyReason(link+" points to "+target+", not a dotfiles checkout's released rule; it was preserved", originErr))
		} else if err := addLegacyDiscard(scope, "Remove the released Ghostty maximize rule link", link); err != nil {
			return err
		}
	}
	autostart := filepath.Join(config, "autostart", "devilspie2.desktop")
	data, _, err := readLegacyFile(autostart, 4096)
	switch {
	case err != nil:
		scope.Retained = append(scope.Retained, legacyReason(autostart+" could not be read as the released autostart entry; it was preserved", err))
	case data != nil && string(data) != legacyDevilspie2Autostart:
		scope.Retained = append(scope.Retained, autostart+" differs from the released autostart entry; it was preserved")
	case data != nil:
		if err := addLegacyDiscard(scope, "Remove the released devilspie2 autostart entry", autostart); err != nil {
			return err
		}
	}
	if len(scope.Steps) == before {
		return nil
	}
	if info, err := os.Lstat(rules); err == nil && info.IsDir() {
		scope.Steps = append(scope.Steps, legacyRetirementStep{Kind: "rmdir", Label: "Remove the empty released directory " + rules, Path: rules})
	}
	scope.Detail = append(scope.Detail, "The devilspie2 package stays: it is a distribution package that can serve other rules; a running devilspie2 keeps the rule until it exits.")
	return nil
}

// Homebrew's own running-application test, with static JXA source. The bundle
// identifier is data in argv; an application LaunchServices cannot resolve is
// not running.
const legacyMacApplicationScript = `'use strict';
ObjC.import('AppKit')
function run(argv) {
	const applications = $.NSWorkspace.sharedWorkspace.runningApplications
	for (let i = 0; i < applications.count; i++) {
		if (ObjC.unwrap(applications.objectAtIndex(i).bundleIdentifier) === argv[0]) {
			return 'running'
		}
	}
	return 'not-running'
}`

func (d *legacyRetirementDriver) macApplicationState(ctx context.Context, bundle string) (string, error) {
	output, err := d.Query(ctx, d.systemCommand(false, "/usr/bin/osascript", "-l", "JavaScript", "-e", legacyMacApplicationScript, bundle))
	if err != nil {
		return "", err
	}
	state := strings.TrimSpace(string(output))
	if state != "running" && state != "not-running" {
		return "", errors.New("macOS did not return a complete application inventory")
	}
	return state, nil
}

// Windows package managers.

func (d *legacyRetirementDriver) planWindowsPackages(ctx context.Context, scope *legacyRetirementScope, delta legacyManagerDelta) (bool, error) {
	installed, before := false, len(scope.Steps)
	for _, plan := range []func(context.Context, *legacyRetirementScope, legacyManagerDelta) (bool, error){d.planScoopApp, d.planWingetPackage, d.planChocoPackage} {
		found, err := plan(ctx, scope, delta)
		if err != nil {
			return installed, err
		}
		installed = installed || found
	}
	if len(scope.Steps) != before && len(delta.Processes) != 0 {
		scope.Detail = append(scope.Detail, "Close running "+strings.Join(delta.Processes, ", ")+" processes, including setup's copies, before retirement; removal is refused while one runs.")
	}
	return installed, nil
}

// Fixed scripts read their single input from the environment, because queries
// carry no process input. Built-in modules are bound by windowsVendorArguments.
func (d *legacyRetirementDriver) windowsScript(script string, input any) (nativeCommand, error) {
	data, err := json.Marshal(input)
	if err != nil {
		return nativeCommand{}, err
	}
	return nativeCommand{Program: d.Platform.WindowsPowerShell, Arguments: append([]string{"-ExecutionPolicy", "Bypass"}, windowsVendorArguments(script)...),
		Environment: []string{"DOTFILES_RETIREMENT_INPUT=" + string(data)}}, nil
}

type legacyProcessQuery struct {
	Directories []string `json:"directories"`
	Names       []string `json:"names"`
}

func legacyProcessGate(directories, names []string) (legacyRetirementGate, error) {
	data, err := json.Marshal(legacyProcessQuery{Directories: append([]string{}, directories...), Names: append([]string{}, names...)})
	return legacyRetirementGate{Kind: "windows-processes", Subject: string(data)}, err
}

// The same test Scoop applies before uninstalling: a process running from a
// targeted directory blocks removal instead of being closed. Process names
// stand in for a location only where it is unknown (winget and Chocolatey
// gates name no directory). With a known directory, a same-named program
// elsewhere is not this installation: Adobe's bundled node.exe once blocked
// Scoop's Node.js, and setup's own pwsh would block Scoop's PowerShell 7.
// Image paths come from QueryFullProcessImageNameW with the limited-query
// right, which Windows grants for elevated and protected processes too. A
// process whose image still cannot be read blocks only when its image name is
// a requested name or an executable inside a targeted directory; unrelated
// protected system processes cannot be ours. A process that exited while
// being inspected (ERROR_INVALID_PARAMETER) is gone.
const legacyWindowsProcessScript = `
$ErrorActionPreference = 'Stop'
$request = $env:DOTFILES_RETIREMENT_INPUT | ConvertFrom-Json
Add-Type -TypeDefinition @'
using System;
using System.Collections.Generic;
using System.IO;
using System.Runtime.InteropServices;
using System.Text;
public static class DotfilesProcess {
	[DllImport("kernel32.dll", SetLastError=true)] static extern IntPtr OpenProcess(uint access, bool inherit, int id);
	[DllImport("kernel32.dll", SetLastError=true, CharSet=CharSet.Unicode)] static extern bool QueryFullProcessImageNameW(IntPtr process, uint flags, StringBuilder name, ref uint size);
	[DllImport("kernel32.dll")] static extern bool CloseHandle(IntPtr handle);
	public static string Image(int id, out bool exited) {
		exited = false;
		IntPtr process = OpenProcess(0x1000, false, id);
		if (process == IntPtr.Zero) { exited = Marshal.GetLastWin32Error() == 87; return null; }
		try {
			StringBuilder name = new StringBuilder(32768); uint size = 32768;
			return QueryFullProcessImageNameW(process, 0, name, ref size) ? name.ToString(0, (int)size) : null;
		} finally { CloseHandle(process); }
	}
	// Reparse points are not followed: a Scoop current junction names a
	// version directory that is enumerated beside it.
	public static void Executables(string directory, HashSet<string> names, ref int budget) {
		foreach (string entry in Directory.GetFileSystemEntries(directory)) {
			if (--budget < 0) { throw new Exception("Too many files to inspect for running executables"); }
			FileAttributes attributes = File.GetAttributes(entry);
			if ((attributes & FileAttributes.ReparsePoint) != 0) { continue; }
			if ((attributes & FileAttributes.Directory) != 0) { Executables(entry, names, ref budget); }
			else if (entry.EndsWith(".exe", StringComparison.OrdinalIgnoreCase)) { names.Add(Path.GetFileNameWithoutExtension(entry)); }
		}
	}
}
'@
$names = [Collections.Generic.HashSet[string]]::new([StringComparer]::OrdinalIgnoreCase)
foreach ($name in @($request.names)) { [void]$names.Add([string]$name) }
$budget = 200000
$directories = @(@($request.directories) | Where-Object { $_ })
foreach ($directory in $directories) {
	if (Test-Path -LiteralPath $directory -PathType Container) { [DotfilesProcess]::Executables([string]$directory, $names, [ref]$budget) }
}
$found = [Collections.Generic.List[string]]::new()
foreach ($process in @(Get-Process)) {
	$exited = $false
	$path = [DotfilesProcess]::Image($process.Id, [ref]$exited)
	if ($exited) { continue }
	$match = $directories.Count -eq 0 -and @($request.names) -contains $process.ProcessName
	if ($path) {
		foreach ($directory in $directories) {
			if ($path.StartsWith(([string]$directory).TrimEnd('\') + '\', [StringComparison]::OrdinalIgnoreCase)) { $match = $true }
		}
		if ($match) { $found.Add(([string]$process.Id) + ' ' + $path) }
	} elseif ($names.Contains($process.ProcessName)) {
		$found.Add(([string]$process.Id) + ' ' + $process.ProcessName + ' (its image path cannot be inspected)')
	}
}
[Console]::Out.Write((@($found | Sort-Object) -join '; '))
`

func (d *legacyRetirementDriver) windowsProcesses(ctx context.Context, subject string) (string, error) {
	var request legacyProcessQuery
	if err := json.Unmarshal([]byte(subject), &request); err != nil {
		return "", err
	}
	command, err := d.windowsScript(legacyWindowsProcessScript, request)
	if err != nil {
		return "", err
	}
	output, err := d.Query(ctx, command)
	return strings.TrimSpace(string(output)), err
}

// Scoop, as located by the released installer: $env:SCOOP or ~\scoop.

func (d *legacyRetirementDriver) scoopRoot() string {
	if root := os.Getenv("SCOOP"); filepath.IsAbs(root) {
		return filepath.Clean(root)
	}
	return filepath.Join(d.Folders.Home, "scoop")
}

type legacyScoopRecord struct {
	Bucket, Version, Identity string
	Depends                   []string
	Manifest                  []byte
}

// The official repositories behind the buckets the released installer used,
// as listed in Scoop's buckets.json plus the released psmux bucket URL.
var legacyScoopBuckets = map[string]string{
	"main":       "https://github.com/ScoopInstaller/Main",
	"extras":     "https://github.com/ScoopInstaller/Extras",
	"nerd-fonts": "https://github.com/matthewjberger/scoop-nerd-fonts",
	"psmux":      "https://github.com/psmux/scoop-psmux",
}

func legacyStringList(raw json.RawMessage) ([]string, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	var single string
	if json.Unmarshal(raw, &single) == nil {
		return []string{single}, nil
	}
	var values []string
	return values, json.Unmarshal(raw, &values)
}

func legacyText(value string) bool {
	return value != "" && len(value) <= 256 && utf8.ValidString(value) && !strings.ContainsFunc(value, func(r rune) bool { return r < ' ' || r == 0x7f })
}

// Scoop reads scoop-install.json and scoop-manifest.json, falling back to the
// earlier install.json and manifest.json names (lib/manifest.ps1 install_info
// and installed_manifest). The same files are read here.
func readLegacyScoopFile(current, name, legacy string) ([]byte, configSnapshot, error) {
	data, snapshot, err := readLegacyFile(filepath.Join(current, name), 1<<20)
	if err != nil || data != nil {
		return data, snapshot, err
	}
	data, snapshot, err = readLegacyFile(filepath.Join(current, legacy), 1<<20)
	if err == nil && data == nil {
		err = fmt.Errorf("%s has neither %s nor %s", current, name, legacy)
	}
	return data, snapshot, err
}

// The current junction's install record names the bucket; the installed
// manifest names the version and dependencies. Both bind the approval.
func readLegacyScoopApp(directory string) (legacyScoopRecord, bool, error) {
	var record legacyScoopRecord
	if _, err := os.Lstat(directory); errors.Is(err, os.ErrNotExist) {
		return record, false, nil
	} else if err != nil {
		return record, true, err
	}
	current := filepath.Join(directory, "current")
	install, installSnapshot, err := readLegacyScoopFile(current, "scoop-install.json", "install.json")
	if err != nil {
		return record, true, err
	}
	manifest, manifestSnapshot, err := readLegacyScoopFile(current, "scoop-manifest.json", "manifest.json")
	if err != nil {
		return record, true, err
	}
	var source struct {
		Bucket string `json:"bucket"`
	}
	var definition struct {
		Version string          `json:"version"`
		Depends json.RawMessage `json:"depends"`
	}
	if err := errors.Join(json.Unmarshal(install, &source), json.Unmarshal(bytes.TrimPrefix(manifest, []byte{0xef, 0xbb, 0xbf}), &definition)); err != nil {
		return record, true, err
	}
	record.Depends, err = legacyStringList(definition.Depends)
	if err != nil || !legacyText(source.Bucket) || !legacyText(definition.Version) {
		return record, true, errors.Join(errors.New("the install record has no bucket or version"), err)
	}
	record.Bucket, record.Version, record.Manifest = source.Bucket, definition.Version, manifest
	record.Identity = fmt.Sprintf("bucket=%s version=%s install=%s manifest=%s", record.Bucket, record.Version, installSnapshot.Hash, manifestSnapshot.Hash)
	return record, true, nil
}

// Scoop's uninstall runs hook and uninstaller scripts and removes environment
// settings named by the installed manifest. Those strings are accepted only
// as the official bucket's own: its Git checkout's origin is the released
// repository and its history recorded exactly the bytes Scoop copied when
// installing (save_installed_manifest copies the bucket file).
func (d *legacyRetirementDriver) scoopManifestProvenance(ctx context.Context, root, bucket, app string, manifest []byte) (string, error) {
	official := legacyScoopBuckets[bucket]
	checkout := filepath.Join(root, "buckets", bucket)
	origin, err := legacyGitOrigin(checkout)
	if err != nil {
		return legacyReason("Scoop bucket "+bucket+" at "+checkout+" is not a Git checkout whose history can prove the installed manifest; run `scoop update` once so Scoop converts it", err), nil
	}
	if official == "" || !legacyRepositoryURL(origin, official) {
		return "Scoop bucket " + bucket + " comes from " + truncateLegacyText(origin, 256) + ", not the released " + official, nil
	}
	directory, relative := checkout, ""
	if info, err := os.Lstat(filepath.Join(checkout, "bucket")); err == nil && info.IsDir() {
		directory, relative = filepath.Join(checkout, "bucket"), "bucket/"
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	entries, err := readPlainDirectory(directory)
	if err != nil {
		return "", err
	}
	index := slices.IndexFunc(entries, func(entry os.DirEntry) bool { return strings.EqualFold(entry.Name(), app+".json") })
	if index < 0 {
		return "Scoop bucket " + bucket + " no longer has a manifest for " + app + ", so the installed one cannot be proved", nil
	}
	proved, err := d.legacyBlobInHistory(ctx, checkout, relative+entries[index].Name(), manifest)
	if err != nil {
		return legacyReason("the history of Scoop bucket "+bucket+" could not be searched for the installed "+app+" manifest", err), nil
	}
	if !proved {
		return "the installed " + app + " manifest is not one recorded by the " + bucket + " bucket's available history; it may be locally modified, or the bucket clone may be shallow", nil
	}
	return "", nil
}

// Consumers need only the dependencies an installed manifest declares. An app
// installed from a URL or a local manifest records no bucket but still names them.
func readLegacyScoopDepends(directory string) ([]string, error) {
	manifest, _, err := readLegacyScoopFile(filepath.Join(directory, "current"), "scoop-manifest.json", "manifest.json")
	if err != nil {
		return nil, err
	}
	var definition struct {
		Depends json.RawMessage `json:"depends"`
	}
	if err := json.Unmarshal(bytes.TrimPrefix(manifest, []byte{0xef, 0xbb, 0xbf}), &definition); err != nil {
		return nil, err
	}
	return legacyStringList(definition.Depends)
}

// Scoop records no reverse edges; installed manifests' depends name them.
func legacyScoopConsumers(root, app string) ([]string, error) {
	entries, err := readPlainDirectory(filepath.Join(root, "apps"))
	if err != nil {
		return nil, err
	}
	if len(entries) > maxConfigEntries {
		return nil, errors.New("too many Scoop apps to inspect")
	}
	consumers := []string{}
	for _, entry := range entries {
		if strings.EqualFold(entry.Name(), app) || strings.EqualFold(entry.Name(), "scoop") {
			continue
		}
		depends, err := readLegacyScoopDepends(filepath.Join(root, "apps", entry.Name()))
		if err != nil {
			return nil, fmt.Errorf("inspect Scoop app %s: %w", entry.Name(), err)
		}
		if slices.ContainsFunc(depends, func(dependency string) bool {
			return strings.EqualFold(dependency[strings.LastIndex(dependency, "/")+1:], app)
		}) {
			consumers = append(consumers, entry.Name())
		}
	}
	return consumers, nil
}

// Scoop runs with this engine's built-in modules and the caller's preferences.
// Without --purge, its persisted data stays; Scoop refuses running apps itself.
const legacyScoopUninstallScript = `
$request = $env:DOTFILES_RETIREMENT_INPUT | ConvertFrom-Json
& ([string]$request.scoop) uninstall ([string]$request.app)
exit $LASTEXITCODE
`

func (d *legacyRetirementDriver) planScoopApp(ctx context.Context, scope *legacyRetirementScope, delta legacyManagerDelta) (bool, error) {
	if delta.Scoop == "" {
		return false, nil
	}
	bucket, app, _ := strings.Cut(delta.Scoop, "/")
	root := d.scoopRoot()
	directory := filepath.Join(root, "apps", app)
	record, present, err := readLegacyScoopApp(directory)
	if !present {
		return false, err
	}
	if err != nil {
		scope.Retained = append(scope.Retained, legacyReason("Scoop app "+directory+" has no readable install record; it was preserved", err))
		return true, nil
	}
	if !strings.EqualFold(record.Bucket, bucket) {
		scope.Detail = append(scope.Detail, "Scoop "+app+" comes from bucket "+record.Bucket+", not the released "+bucket+" bucket; it stays.")
		return false, nil
	}
	label := "Scoop " + delta.Scoop + " " + record.Version
	// Retained text names the app folder, so a kept leftover is easy to find.
	kept := label + " at " + directory
	script := filepath.Join(root, "apps", "scoop", "current", "bin", "scoop.ps1")
	if info, err := os.Lstat(script); err != nil || !info.Mode().IsRegular() || d.Platform.WindowsPowerShell == "" {
		scope.Retained = append(scope.Retained, legacyReason(kept+" needs Scoop and Windows PowerShell to uninstall; it was preserved", err))
		return true, nil
	}
	refusal, err := d.scoopManifestProvenance(ctx, root, bucket, app, record.Manifest)
	if err != nil {
		return true, err
	}
	if refusal != "" {
		scope.Retained = append(scope.Retained, kept+": "+refusal+"; it was preserved")
		return true, nil
	}
	consumers, err := legacyScoopConsumers(root, app)
	if err != nil {
		scope.Retained = append(scope.Retained, legacyReason(kept+": installed Scoop apps could not be checked for a dependency on it; it was preserved", err))
		return true, nil
	}
	if len(consumers) != 0 {
		scope.Retained = append(scope.Retained, kept+" is still required by Scoop apps "+strings.Join(consumers, ", ")+"; it was preserved")
		return true, nil
	}
	processes, err := legacyProcessGate([]string{directory}, delta.Processes)
	if err != nil {
		return true, err
	}
	command, err := d.windowsScript(legacyScoopUninstallScript, struct {
		Scoop string `json:"scoop"`
		App   string `json:"app"`
	}{script, app})
	if err != nil {
		return true, err
	}
	scope.Steps = append(scope.Steps, legacyRetirementStep{Kind: "command", Label: "Uninstall " + label + " (its persisted data stays)", Command: &command,
		Gates:  []legacyRetirementGate{processes, {Kind: "scoop-app", Subject: directory, Expected: record.Identity}, {Kind: "scoop-consumers", Subject: directory}},
		Absent: []string{directory}})
	return true, nil
}

func (d *legacyRetirementDriver) scoopGate(subject string, consumers bool) (string, error) {
	record, present, err := readLegacyScoopApp(subject)
	if err != nil || !present {
		return "absent", err
	}
	if !consumers {
		return record.Identity, nil
	}
	found, err := legacyScoopConsumers(filepath.Dir(filepath.Dir(subject)), filepath.Base(subject))
	return strings.Join(found, ","), err
}

// winget, through its documented export format. Only packages correlated to
// the winget source qualify; winget records no reverse dependencies.

type legacyWingetPackage struct{ ID, Version, Scope string }

var legacyWingetID = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._+-]{0,127}$`)

func (d *legacyRetirementDriver) wingetProgram() string {
	if !filepath.IsAbs(d.Folders.LocalAppData) {
		return ""
	}
	program := filepath.Join(d.Folders.LocalAppData, "Microsoft", "WindowsApps", "winget.exe")
	if _, err := os.Lstat(program); err != nil {
		return ""
	}
	return program
}

func parseLegacyWingetExport(data []byte) (map[string][]legacyWingetPackage, error) {
	var document struct {
		Sources []struct {
			SourceDetails struct {
				Name string `json:"Name"`
			} `json:"SourceDetails"`
			Packages []struct {
				PackageIdentifier string `json:"PackageIdentifier"`
				Version           string `json:"Version"`
				Scope             string `json:"Scope"`
			} `json:"Packages"`
		} `json:"Sources"`
	}
	if err := json.Unmarshal(bytes.TrimPrefix(data, []byte{0xef, 0xbb, 0xbf}), &document); err != nil {
		return nil, err
	}
	if document.Sources == nil {
		return nil, errors.New("winget export omitted its sources")
	}
	inventory := map[string][]legacyWingetPackage{}
	for _, source := range document.Sources {
		if source.SourceDetails.Name != "winget" {
			continue
		}
		for _, pkg := range source.Packages {
			if !legacyWingetID.MatchString(pkg.PackageIdentifier) || pkg.Version != "" && !legacyText(pkg.Version) || pkg.Scope != "" && pkg.Scope != "user" && pkg.Scope != "machine" {
				return nil, errors.New("winget export contains an invalid package identity")
			}
			key := strings.ToLower(pkg.PackageIdentifier)
			inventory[key] = append(inventory[key], legacyWingetPackage{pkg.PackageIdentifier, pkg.Version, pkg.Scope})
		}
	}
	return inventory, nil
}

func (d *legacyRetirementDriver) wingetInventory(ctx context.Context, fresh bool) (map[string][]legacyWingetPackage, error) {
	program := d.wingetProgram()
	if program == "" {
		return nil, errors.New("winget is unavailable")
	}
	return legacyMemoized(ctx, "winget\x00"+program, fresh, func() (inventory map[string][]legacyWingetPackage, result error) {
		directory, err := os.MkdirTemp("", "dotfiles-winget-")
		if err != nil {
			return nil, err
		}
		defer func() { result = errors.Join(result, os.RemoveAll(directory)) }()
		file := filepath.Join(directory, "packages.json")
		if _, err := d.Query(ctx, nativeCommand{Program: program, Arguments: []string{"export", "--output", file, "--source", "winget", "--include-versions",
			"--accept-source-agreements", "--disable-interactivity", "--nowarn"}}); err != nil {
			return nil, err
		}
		data, _, err := readLegacyFile(file, 8<<20)
		if err == nil && data == nil {
			err = errors.New("winget export wrote no inventory")
		}
		if err != nil {
			return nil, err
		}
		return parseLegacyWingetExport(data)
	})
}

func legacyWingetIdentity(pkg legacyWingetPackage) string {
	return fmt.Sprintf("id=%s version=%s scope=%s", pkg.ID, pkg.Version, pkg.Scope)
}

func (d *legacyRetirementDriver) planWingetPackage(ctx context.Context, scope *legacyRetirementScope, delta legacyManagerDelta) (bool, error) {
	program := d.wingetProgram()
	if delta.Winget == "" || program == "" {
		return false, nil
	}
	inventory, err := d.wingetInventory(ctx, false)
	if err != nil {
		scope.Retained = append(scope.Retained, legacyReason("winget's installed inventory could not be read, so "+delta.Winget+" was not checked", err))
		return true, nil
	}
	installed := inventory[strings.ToLower(delta.Winget)]
	switch {
	case len(installed) == 0:
		return false, nil
	case len(installed) > 1:
		scope.Retained = append(scope.Retained, fmt.Sprintf("winget reports %d installations of %s; uninstall the intended one manually", len(installed), delta.Winget))
		return true, nil
	case d.Platform.WindowsPowerShell == "":
		scope.Retained = append(scope.Retained, "winget package "+delta.Winget+" cannot be checked for running processes without Windows PowerShell; it was preserved")
		return true, nil
	}
	pkg := installed[0]
	processes, err := legacyProcessGate(nil, delta.Processes)
	if err != nil {
		return true, err
	}
	arguments := []string{"uninstall", "--id", pkg.ID, "--exact", "--source", "winget", "--silent", "--disable-interactivity", "--accept-source-agreements"}
	if pkg.Scope != "" {
		arguments = append(arguments, "--scope", pkg.Scope)
	}
	command := nativeCommand{Program: program, Arguments: arguments}
	scope.Steps = append(scope.Steps, legacyRetirementStep{Kind: "command", Label: "Uninstall winget package " + pkg.ID + " " + pkg.Version, Command: &command,
		Gates:  []legacyRetirementGate{processes, {Kind: "winget-package", Subject: pkg.ID, Expected: legacyWingetIdentity(pkg)}},
		Checks: []legacyRetirementGate{{Kind: "winget-package", Subject: pkg.ID, Expected: "absent"}}})
	detail := "winget records no dependents of " + pkg.ID + "; programs outside setup that run it are not detected."
	if pkg.Scope != "user" {
		detail += " Windows may ask for administrator approval."
	}
	scope.Detail = append(scope.Detail, detail)
	return true, nil
}

func (d *legacyRetirementDriver) wingetGate(ctx context.Context, id string) (string, error) {
	inventory, err := d.wingetInventory(ctx, true)
	if err != nil {
		return "", err
	}
	installed := inventory[strings.ToLower(id)]
	switch len(installed) {
	case 0:
		return "absent", nil
	case 1:
		return legacyWingetIdentity(installed[0]), nil
	}
	return fmt.Sprintf("%d installations", len(installed)), nil
}

// Chocolatey keeps each package's nuspec in lib\<id>. Its declared
// dependencies are the reverse edges that protect a package.

var (
	legacyChocoID      = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,99}$`)
	legacyChocoVersion = regexp.MustCompile(`^[0-9A-Za-z][0-9A-Za-z.+-]{0,63}$`)
)

type legacyChocoPackage struct {
	ID, Version, Library, Nuspec, Identity string
	Dependencies                           []string
}

func chocolateyRoot() string {
	if root := os.Getenv("ChocolateyInstall"); filepath.IsAbs(root) {
		return filepath.Clean(root)
	}
	if data := os.Getenv("ProgramData"); filepath.IsAbs(data) {
		return filepath.Join(filepath.Clean(data), "chocolatey")
	}
	return ""
}

func readLegacyChocoPackage(library string) (legacyChocoPackage, bool, error) {
	pkg := legacyChocoPackage{Library: library, Nuspec: filepath.Join(library, filepath.Base(library)+".nuspec")}
	if _, err := os.Lstat(library); errors.Is(err, os.ErrNotExist) {
		return pkg, false, nil
	} else if err != nil {
		return pkg, true, err
	}
	data, snapshot, err := readLegacyFile(pkg.Nuspec, 1<<20)
	// Chocolatey's official bootstrap copies only chocolatey.nupkg into
	// lib/chocolatey. The manager itself stays installed, but its dependencies
	// must still participate in the inventory's outside-consumer protection.
	if err == nil && data == nil && strings.EqualFold(filepath.Base(library), "chocolatey") {
		pkg.Nuspec = filepath.Join(library, filepath.Base(library)+".nupkg")
		data, snapshot, err = readLegacyFile(pkg.Nuspec, 32<<20)
		if err == nil && data != nil {
			data, err = legacyChocoBootstrapNuspec(data)
		}
	}
	if err == nil && data == nil {
		err = errors.New("the package has no nuspec")
	}
	if err != nil {
		return pkg, true, err
	}
	id, version, dependencies, err := parseLegacyNuspec(data)
	if err != nil {
		return pkg, true, err
	}
	if !strings.EqualFold(id, filepath.Base(library)) {
		return pkg, true, errors.New("the nuspec does not identify this package")
	}
	pkg.ID, pkg.Version, pkg.Dependencies = id, version, dependencies
	pkg.Identity = fmt.Sprintf("id=%s version=%s nuspec=%s", pkg.ID, pkg.Version, snapshot.Hash)
	return pkg, true, nil
}

func legacyChocoBootstrapNuspec(data []byte) ([]byte, error) {
	archive, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, err
	}
	if len(archive.File) > 65536 {
		return nil, errors.New("the Chocolatey bootstrap package has too many entries")
	}
	var nuspec *zip.File
	for _, file := range archive.File {
		if strings.EqualFold(file.Name, "chocolatey.nuspec") {
			if nuspec != nil {
				return nil, errors.New("the Chocolatey bootstrap package has duplicate metadata")
			}
			nuspec = file
		}
	}
	if nuspec == nil || nuspec.UncompressedSize64 > 1<<20 {
		return nil, errors.New("the Chocolatey bootstrap package has missing or oversized metadata")
	}
	file, err := nuspec.Open()
	if err != nil {
		return nil, err
	}
	data, readErr := io.ReadAll(io.LimitReader(file, 1<<20+1))
	if err := errors.Join(readErr, file.Close()); err != nil {
		return nil, err
	}
	if len(data) > 1<<20 {
		return nil, errors.New("the Chocolatey bootstrap metadata exceeds its read bound")
	}
	return data, nil
}

func parseLegacyNuspec(data []byte) (string, string, []string, error) {
	type dependency struct {
		ID string `xml:"id,attr"`
	}
	var document struct {
		Metadata struct {
			ID           string `xml:"id"`
			Version      string `xml:"version"`
			Dependencies struct {
				Dependency []dependency `xml:"dependency"`
				Group      []struct {
					Dependency []dependency `xml:"dependency"`
				} `xml:"group"`
			} `xml:"dependencies"`
		} `xml:"metadata"`
	}
	if err := xml.Unmarshal(data, &document); err != nil {
		return "", "", nil, err
	}
	metadata := document.Metadata
	if !legacyChocoID.MatchString(metadata.ID) || !legacyChocoVersion.MatchString(metadata.Version) {
		return "", "", nil, errors.New("the nuspec has no valid package identity")
	}
	dependencies := metadata.Dependencies.Dependency
	for _, group := range metadata.Dependencies.Group {
		dependencies = append(dependencies, group.Dependency...)
	}
	ids := []string{}
	for _, item := range dependencies {
		if !legacyChocoID.MatchString(item.ID) {
			return "", "", nil, errors.New("the nuspec declares an invalid dependency")
		}
		ids = append(ids, strings.ToLower(item.ID))
	}
	return metadata.ID, metadata.Version, sortedUnique(ids), nil
}

// The released installer ran `choco install <id> -y` with Chocolatey's
// configured sources. The community repository is the only accepted one.
var legacyChocoSources = []string{"https://community.chocolatey.org/api/v2", "https://chocolatey.org/api/v2"}

func legacyChocoEnabledSources(root string) ([]string, error) {
	data, _, err := readLegacyFile(filepath.Join(root, "config", "chocolatey.config"), 4<<20)
	if err == nil && data == nil {
		err = errors.New("chocolatey.config is missing")
	}
	if err != nil {
		return nil, err
	}
	var document struct {
		Sources struct {
			Source []struct {
				Value    string `xml:"value,attr"`
				Disabled string `xml:"disabled,attr"`
			} `xml:"source"`
		} `xml:"sources"`
	}
	if err := xml.Unmarshal(data, &document); err != nil {
		return nil, err
	}
	sources := []string{}
	for _, source := range document.Sources.Source {
		if !strings.EqualFold(strings.TrimSpace(source.Disabled), "true") {
			sources = append(sources, source.Value)
		}
	}
	return sources, nil
}

func legacyChocoScript(name string) bool {
	extension := strings.ToLower(filepath.Ext(name))
	return extension == ".ps1" || extension == ".psm1" || extension == ".psd1"
}

// Chocolatey runs package scripts from lib\<id> and loads every installed
// extension module. Each must still equal its copy inside the package's own
// retained .nupkg, whose nuspec names the same package.
func legacyChocoScriptsMatch(root string, pkg legacyChocoPackage) (string, error) {
	archive := filepath.Join(pkg.Library, filepath.Base(pkg.Library)+".nupkg")
	reader, err := zip.OpenReader(archive)
	if err != nil {
		return legacyReason("Chocolatey "+pkg.ID+" has no readable retained package "+archive, err), nil
	}
	defer reader.Close()
	if len(reader.File) > 65536 {
		return "Chocolatey " + pkg.ID + "'s package has too many entries to verify", nil
	}
	scripts, nuspec := map[string]*zip.File{}, (*zip.File)(nil)
	for _, file := range reader.File {
		name, err := url.PathUnescape(file.Name)
		if err != nil {
			return "Chocolatey " + pkg.ID + "'s package has an invalid entry name", nil
		}
		name = strings.ToLower(strings.ReplaceAll(name, `\`, "/"))
		if legacyChocoScript(name) {
			scripts[name] = file
		} else if !strings.Contains(name, "/") && strings.HasSuffix(name, ".nuspec") {
			nuspec = file
		}
	}
	read := func(file *zip.File) ([]byte, error) {
		if file.UncompressedSize64 > 16<<20 {
			return nil, errors.New("package entry exceeds its size bound")
		}
		opened, err := file.Open()
		if err != nil {
			return nil, err
		}
		data, readErr := io.ReadAll(io.LimitReader(opened, 16<<20+1))
		return data, errors.Join(readErr, opened.Close())
	}
	if nuspec == nil {
		return "Chocolatey " + pkg.ID + "'s package has no nuspec", nil
	}
	data, err := read(nuspec)
	if err != nil {
		return "", err
	}
	if id, version, _, err := parseLegacyNuspec(data); err != nil || !strings.EqualFold(id, pkg.ID) || version != pkg.Version {
		return legacyReason("Chocolatey "+pkg.ID+"'s retained package is not version "+pkg.Version, err), nil
	}
	trees := map[string]string{pkg.Library: ""}
	if name, extension := strings.CutSuffix(strings.ToLower(pkg.ID), ".extension"); extension {
		trees[filepath.Join(root, "extensions", name)] = "extensions/"
	}
	budget := 100000
	for tree, prefix := range trees {
		err := filepath.WalkDir(tree, func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if budget--; budget < 0 {
				return errors.New("too many files to verify")
			}
			if entry.IsDir() || !legacyChocoScript(path) {
				return nil
			}
			relative, err := filepath.Rel(tree, path)
			if err != nil {
				return err
			}
			want := scripts[strings.ToLower(prefix+filepath.ToSlash(relative))]
			if want == nil || !entry.Type().IsRegular() {
				return fmt.Errorf("%s is not a script of the retained package", path)
			}
			expected, err := read(want)
			if err != nil {
				return err
			}
			actual, _, err := readLegacyFile(path, 16<<20)
			if err != nil {
				return err
			}
			if !bytes.Equal(actual, expected) {
				return fmt.Errorf("%s differs from the retained package", path)
			}
			return nil
		})
		if errors.Is(err, os.ErrNotExist) && prefix != "" {
			continue
		}
		if err != nil {
			return legacyReason("Chocolatey "+pkg.ID+"'s installed scripts cannot be proved to be the package's own", err), nil
		}
	}
	return "", nil
}

// legacyChocoProvenance accepts package scripts only from the community
// repository, unmodified since installation, including installed extensions.
func legacyChocoProvenance(root string, inventory map[string]legacyChocoPackage, candidates []legacyChocoPackage) (string, error) {
	sources, err := legacyChocoEnabledSources(root)
	if err != nil {
		return legacyReason("Chocolatey's source configuration could not be read, so the package's origin cannot be proved", err), nil
	}
	if len(sources) == 0 {
		return "Chocolatey has no enabled source, so the package's origin cannot be proved", nil
	}
	for _, source := range sources {
		if !legacyRepositoryURL(source, legacyChocoSources...) {
			return "Chocolatey also installs from " + truncateLegacyText(source, 256) + ", so the package's origin cannot be proved", nil
		}
	}
	checked := slices.Clone(candidates)
	for _, key := range slices.Sorted(maps.Keys(inventory)) {
		if strings.HasSuffix(key, ".extension") {
			checked = append(checked, inventory[key])
		}
	}
	for _, pkg := range checked {
		if refusal, err := legacyChocoScriptsMatch(root, pkg); err != nil || refusal != "" {
			return refusal, err
		}
	}
	return "", nil
}

func legacyChocoInventory(root string) (map[string]legacyChocoPackage, error) {
	library := filepath.Join(root, "lib")
	entries, err := readPlainDirectory(library)
	if errors.Is(err, os.ErrNotExist) {
		return map[string]legacyChocoPackage{}, nil
	}
	if err != nil {
		return nil, err
	}
	if len(entries) > maxConfigEntries {
		return nil, errors.New("too many Chocolatey packages to inspect")
	}
	inventory := map[string]legacyChocoPackage{}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		pkg, _, err := readLegacyChocoPackage(filepath.Join(library, entry.Name()))
		if err != nil {
			return nil, fmt.Errorf("inspect Chocolatey package %s: %w", entry.Name(), err)
		}
		inventory[strings.ToLower(pkg.ID)] = pkg
	}
	return inventory, nil
}

func legacyChocoConsumers(inventory map[string]legacyChocoPackage, id string, except []string) []string {
	consumers := []string{}
	for key, pkg := range inventory {
		if key != strings.ToLower(id) && !slices.Contains(except, key) && slices.Contains(pkg.Dependencies, strings.ToLower(id)) {
			consumers = append(consumers, pkg.ID)
		}
	}
	slices.Sort(consumers)
	return consumers
}

// Chocolatey requires an administrator. The approved nuspec bytes are checked
// again immediately before Windows asks for consent, exactly as the reviewed
// vendor provider elevates. Its own uninstaller runs without --force or
// dependency removal; a restart request is a success once the package is gone.
const legacyChocoUninstallScript = `
$ErrorActionPreference = 'Stop'
$request = $env:DOTFILES_RETIREMENT_INPUT | ConvertFrom-Json
if ((Get-FileHash -LiteralPath $request.nuspec -Algorithm SHA256).Hash.ToLowerInvariant() -cne $request.sha256) {
	[Console]::Error.WriteLine('The Chocolatey package changed after approval')
	exit 72
}
$start = [Diagnostics.ProcessStartInfo]::new()
$start.FileName = $request.program
$start.Arguments = 'uninstall ' + $request.package + ' --version=' + $request.version + ' --yes --no-progress'
$start.UseShellExecute = $true
$identity = [Security.Principal.WindowsIdentity]::GetCurrent()
try { $administrator = ([Security.Principal.WindowsPrincipal]::new($identity)).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator) }
finally { $identity.Dispose() }
if (-not $administrator) { $start.Verb = 'runas' }
try {
	$process = [Diagnostics.Process]::Start($start)
	try { $process.WaitForExit(); $code = [int]$process.ExitCode }
	finally { $process.Dispose() }
} catch {
	$failure = $_.Exception
	while ($failure -and -not ($failure -is [ComponentModel.Win32Exception])) { $failure = $failure.InnerException }
	if (-not $failure -or $failure.NativeErrorCode -ne 1223) { throw }
	exit 1223
}
if (Test-Path -LiteralPath $request.library) {
	if ($code -eq 0) { exit 74 }
	exit $code
}
if ($code -eq 1641 -or $code -eq 3010) { 'Chocolatey removed the package; Windows reports that a restart is required.' }
elseif ($code -ne 0) { exit $code }
exit 0
`

func (d *legacyRetirementDriver) planChocoPackage(_ context.Context, scope *legacyRetirementScope, delta legacyManagerDelta) (bool, error) {
	root := chocolateyRoot()
	if delta.Choco == "" || root == "" {
		return false, nil
	}
	inventory, err := legacyChocoInventory(root)
	if err != nil {
		scope.Retained = append(scope.Retained, legacyReason("Chocolatey's package library could not be read, so "+delta.Choco+" was not checked", err))
		return true, nil
	}
	pkg, ok := inventory[strings.ToLower(delta.Choco)]
	if !ok {
		return false, nil
	}
	program := filepath.Join(root, "choco.exe")
	if info, err := os.Lstat(program); err != nil || !info.Mode().IsRegular() || d.Platform.WindowsPowerShell == "" {
		scope.Retained = append(scope.Retained, legacyReason("Chocolatey "+pkg.ID+" needs choco.exe and Windows PowerShell to uninstall; it was preserved", err))
		return true, nil
	}
	// Chocolatey's virtual packages install the software through their
	// <id>.install or <id>.portable package, and Python's through a version
	// series (python -> python3 -> python313). Those exact companions retire
	// with the package when nothing outside the chain needs them; other
	// dependencies stay, because Chocolatey cannot say who requested them.
	chain, companions, others := []string{strings.ToLower(pkg.ID)}, []legacyChocoPackage{}, []string{}
	for next := 0; next < len(chain); next++ {
		parent := chain[next]
		for _, dependency := range inventory[parent].Dependencies {
			companion, present := inventory[dependency]
			series := strings.TrimPrefix(dependency, parent)
			switch {
			case !present || slices.Contains(chain, dependency):
			case series == ".install" || series == ".portable" || series != dependency && series != "" && strings.Trim(series, "0123456789") == "":
				chain = append(chain, dependency)
				companions = append(companions, companion)
			default:
				others = append(others, companion.ID)
			}
		}
	}
	if consumers := legacyChocoConsumers(inventory, pkg.ID, nil); len(consumers) != 0 {
		scope.Retained = append(scope.Retained, "Chocolatey "+pkg.ID+" is still required by "+strings.Join(consumers, ", ")+"; it was preserved")
		return true, nil
	}
	refusal, err := legacyChocoProvenance(root, inventory, append([]legacyChocoPackage{pkg}, companions...))
	if err != nil {
		return true, err
	}
	if refusal != "" {
		scope.Retained = append(scope.Retained, refusal+"; Chocolatey "+pkg.ID+" was preserved")
		return true, nil
	}
	// The software would stay behind a removed virtual package, so a chain
	// member another package needs keeps the whole chain.
	for _, candidate := range companions {
		if consumers := legacyChocoConsumers(inventory, candidate.ID, chain); len(consumers) != 0 {
			scope.Retained = append(scope.Retained, "Chocolatey "+candidate.ID+" is still required by "+strings.Join(consumers, ", ")+"; "+pkg.ID+" and its companions were preserved")
			return true, nil
		}
	}
	for _, candidate := range append([]legacyChocoPackage{pkg}, companions...) {
		hash, err := legacyFileSHA256(candidate.Nuspec)
		if err != nil {
			return true, err
		}
		processes, err := legacyProcessGate(nil, delta.Processes)
		if err != nil {
			return true, err
		}
		command, err := d.windowsScript(legacyChocoUninstallScript, struct {
			Program string `json:"program"`
			Package string `json:"package"`
			Version string `json:"version"`
			Nuspec  string `json:"nuspec"`
			SHA256  string `json:"sha256"`
			Library string `json:"library"`
		}{program, candidate.ID, candidate.Version, candidate.Nuspec, hash, candidate.Library})
		if err != nil {
			return true, err
		}
		scope.Steps = append(scope.Steps, legacyRetirementStep{Kind: "command", Label: "Uninstall Chocolatey package " + candidate.ID + " " + candidate.Version + " (Windows asks for administrator approval)", Command: &command,
			Gates:  []legacyRetirementGate{processes, {Kind: "choco-package", Subject: candidate.Library, Expected: candidate.Identity}, {Kind: "choco-consumers", Subject: candidate.Library}},
			Absent: []string{candidate.Library}, Checks: []legacyRetirementGate{{Kind: "choco-package", Subject: candidate.Library, Expected: "absent"}}})
	}
	if len(others) != 0 {
		scope.Detail = append(scope.Detail, "Chocolatey dependencies "+strings.Join(sortedUnique(others), ", ")+" stay installed; dependency removal is never requested.")
	}
	return true, nil
}

func (d *legacyRetirementDriver) chocoGate(subject string, consumers bool) (string, error) {
	pkg, present, err := readLegacyChocoPackage(subject)
	if err != nil || !present {
		return "absent", err
	}
	if !consumers {
		return pkg.Identity, nil
	}
	inventory, err := legacyChocoInventory(filepath.Dir(filepath.Dir(subject)))
	if err != nil {
		return "", err
	}
	return strings.Join(legacyChocoConsumers(inventory, pkg.ID, nil), ","), nil
}

// legacyReleaseTree is one pinned upstream archive and the exact members the
// released installers copied from it. TestLegacyManagerDeltasMatchReleasedInstallers
// compares the archive pins with the release sources; the opt-in
// TestLegacyReleaseTreesMatchPinnedArchives recomputes the member hashes.
type legacyReleaseTree struct {
	Version, URL, SHA256 string
	Commits              []string
	// Prefix is the archive directory whose contents were copied.
	Prefix string
	Files  map[string]string
}

const (
	legacyPsmuxPluginsCommit = "0f46ccca5a9b748fd03851db00b85fd784f42791"
	legacyPylatexencVersion  = "2.11"
	legacySetuptoolsVersion  = "84.0.0"
)

// install-deps.ps1 copied the downloaded preview executable itself.
var legacyHerdrWindowsReleases = []legacyReleaseTree{{Version: "preview-2026-07-21-0f10e1453a7f", Commits: []string{legacyReleaseCommit, legacyStableCommit},
	URL:    "https://github.com/ogulcancelik/herdr/releases/download/preview-2026-07-21-0f10e1453a7f/herdr-windows-x86_64.exe",
	SHA256: "75c85763db0ca5fd13b485d0728cc3e9ea1152964a4e976e1d49f2e86b01a92b",
	Files:  map[string]string{"herdr-windows-x86_64.exe": "75c85763db0ca5fd13b485d0728cc3e9ea1152964a4e976e1d49f2e86b01a92b"}}}

var legacyTreeSitterWindowsReleases = []legacyReleaseTree{
	{Version: "v0.27.0", Commits: []string{legacyReleaseCommit}, URL: "https://github.com/tree-sitter/tree-sitter/releases/download/v0.27.0/tree-sitter-cli-windows-x64.zip",
		SHA256: "46188d31c1f3847307b03e92f3a3f60606eb04147609e26cdd50a07f7d0b35da", Files: map[string]string{"tree-sitter.exe": "9fbc4f285c876b1a38c7e9d5223a51fb7842255285cdd7db3ffb0ba3934f2662"}},
	{Version: "v0.26.11", Commits: []string{legacyStableCommit}, URL: "https://github.com/tree-sitter/tree-sitter/releases/download/v0.26.11/tree-sitter-cli-windows-x64.zip",
		SHA256: "d40e158839062803a5182ecefca76f809b89464146625d303ef779ed8ceb0f73", Files: map[string]string{"tree-sitter.exe": "98c646b69c647d1cee690b9d8f829b833bcb50b520628e0942aced21e1728882"}},
}

// The terminal-<version> tree install-deps.ps1 copied into LocalAppData\Programs.
var legacyWindowsTerminalReleases = []legacyReleaseTree{
	{Version: "1.25.2733.0", Commits: []string{legacyReleaseCommit}, Prefix: "terminal-1.25.2733.0/",
		URL:    "https://github.com/microsoft/terminal/releases/download/v1.25.2733.0/Microsoft.WindowsTerminal_1.25.2733.0_x64.zip",
		SHA256: "bf3ef2012f6c44d8340a4c58125acc9498d19b580f9890dc043cdf831852e796", Files: map[string]string{
			"CascadiaCode.ttf":                                                  "54eae6e26961e980eb3e1ec4ee30f6d3ba7a932a2028832baceb91cc65844a65",
			"CascadiaCodeItalic.ttf":                                            "48e64f8d82acd86956df5f33076966eacf4d9c3ea620c9d88231fa94da117f2c",
			"CascadiaMono.ttf":                                                  "08026f617eea315caca59898784e999f6cdb2d0c34d2040deeb042bcdc359fa2",
			"CascadiaMonoItalic.ttf":                                            "d9d23dfb03c6844448a24cf0337a657ac99965262874d2e6fb5b82ac0ac08c4d",
			"Images/terminal_contrast-black.ico":                                "29864669d81e7d4e50a8ecc16e992b339658454f5234e1949c512a07edbbabf4",
			"Images/terminal_contrast-white.ico":                                "0b452ba3cc2ef2e330422aaa55a78fe59b5454279bb35778f2633d770f592f3b",
			"Microsoft.Terminal.Control.dll":                                    "e27cb1d3d592934826152e0a2b3c2fb0b933c80d8dbd105ef2c869fc99e5d46b",
			"Microsoft.Terminal.Settings.Editor.dll":                            "9f3a3c93e7f7a4e20c3774519ca270257427cf1b740b0e788fd1d508535c82d2",
			"Microsoft.Terminal.Settings.Model.dll":                             "24e4ead02e25556542e748b4d6ef83307bf4917333f77ad3ecb84363eef7dae1",
			"Microsoft.Terminal.UI.Markdown.dll":                                "2fdb54a3f47c405353f746faa771daf016ef06f2375a2aa8c170c5e56535d135",
			"Microsoft.Terminal.UI.dll":                                         "9f69388a3699b1e04927e0195dc18b1c8d6ea85f42718df97a0faff12ef04eb9",
			"Microsoft.UI.Xaml.dll":                                             "8b92de46c8950ced663e970a3352d3930ff3c978f3c73770935b5316cf6ca5ad",
			"Microsoft.UI.Xaml/Assets/NoiseAsset_256x256_PNG.png":               "4f2aa94a2e345a32dae689176b86c644bef87081ff9daef8c77417731862031b",
			"NOTICE.html":                                                       "9691413dfd4979a89d3a02853a7153fe5f40df937cc817dd2715621ec8ea04ef",
			"OpenConsole.exe":                                                   "3d66b23d0a71bb8eed2b77edc8b9df9bf54ce6c8fb74c863a30e760997f80586",
			"OpenConsoleProxy.dll":                                              "c83f033fa96c4f16eeed838df4379b61b7330d2334db02f21da8bb69ca98b30a",
			"ProfileGeneratorIcons/AzureCloudShell.png":                         "db182b52129d6cd66fa60e9ec74e690369eb053699a58ef5707da9c4399d60fe",
			"ProfileGeneratorIcons/PowerShell.png":                              "69da2be6d04562817eb7e379c394bd738371882b6db30a1a0c304a7379d67443",
			"ProfileGeneratorIcons/SSH.png":                                     "f61db9bcc372516558e00cfebc4ff9c310675cd4c9ec6bfe2dc97b86120db376",
			"ProfileGeneratorIcons/VisualStudio.png":                            "1076e1c09147816ee95747fc3420c695eb1ebbabd30a27248590b7d2f374c791",
			"ProfileGeneratorIcons/WSL.png":                                     "8e52bdaff8d8307d1e0585cbb17282a46d69af8846ebea37e576b4c9c019d577",
			"ProfileIcons/pwsh-preview.scale-100.png":                           "592862382a46c236c50899ec68ced77630dac228c2c588955c7b89d38f7c46a7",
			"ProfileIcons/pwsh-preview.scale-125.png":                           "7425cb7ca6c49012a7f04ef8f2f9e8a45ac11af4f4445a87ecf73848ba022628",
			"ProfileIcons/pwsh-preview.scale-150.png":                           "f73a78e761d53185ac9f5d693d8e23505f7f21131b9c562b3dfc68a33e0299af",
			"ProfileIcons/pwsh-preview.scale-200.png":                           "47d2c2fb504cb7abe79184ba5857ea78f8b2ddae5667a53a7472c421d121b9f6",
			"ProfileIcons/pwsh.scale-100.png":                                   "1e870e4eeda1440e5c8eeb673806466beaf156eb26dfa41539480d23433fe5e2",
			"ProfileIcons/pwsh.scale-125.png":                                   "6fcaa38c91cbc961dc69b29d98e24323048965dbc53f3570e0bd683fd4bb6161",
			"ProfileIcons/pwsh.scale-150.png":                                   "f1f79191a7a9a2b3cf32a7dd3ec23810494faadde0fcefc29e3a9b00b155dc03",
			"ProfileIcons/pwsh.scale-200.png":                                   "69da2be6d04562817eb7e379c394bd738371882b6db30a1a0c304a7379d67443",
			"ProfileIcons/vs-cmd.scale-100.png":                                 "82145026660f5550437bbf73c32a006244f8845b531d4bbec2d4903860938452",
			"ProfileIcons/vs-cmd.scale-150.png":                                 "00bfd2ce8690914feb08b9fc079bfd52378f5687bfc89b6c276977e08766e8eb",
			"ProfileIcons/vs-cmd.scale-200.png":                                 "95315646a7ff0b3e387c0cb3572fe2c9ba46410b90d50f69beca8fa406621937",
			"ProfileIcons/vs-powershell.scale-100.png":                          "5b070331f25f4bc00ecfc41455087ca42514766f09df44ab212da5790f5e5b60",
			"ProfileIcons/vs-powershell.scale-150.png":                          "95a5fd04df60b722df61d2dbf3ad109713010fc8b38ad3ea1c0b40ad9629dff5",
			"ProfileIcons/vs-powershell.scale-200.png":                          "54f619cbfdc7c2297d893e8cc6c0a8c045db1daeabc8fa1fa43439f15b45a59e",
			"ProfileIcons/vs-pwsh.scale-100.png":                                "b32bf11e4f4fe8f82ddc2b65c028c30ffe9abe79faf6981565aae4561ba46d45",
			"ProfileIcons/vs-pwsh.scale-150.png":                                "b4b87122c42468d12c774e3ca0e5f9dc3c3e4d632aa60b0b9be0b8588ca6b38f",
			"ProfileIcons/vs-pwsh.scale-200.png":                                "2a8ae61e7aeba63978d75cb7b8d76fb4f7abfd069dd093638812a8ec46154e81",
			"ProfileIcons/{0caa0dad-35be-5f56-a8ff-afceeeaa6101}.scale-100.png": "b774f1b51667fa6ee4eccf9568dadd5dea40e79ef847e163e80b279d55338511",
			"ProfileIcons/{0caa0dad-35be-5f56-a8ff-afceeeaa6101}.scale-200.png": "0d575b3d46310599369bcb41ebedd4fd8e3d76e6887bdf8ade9e2fb7f06667d0",
			"ProfileIcons/{574e775e-4f2a-5b96-ac1e-a2962a402336}.scale-100.png": "8f81d3436aa7a675ed6e17f73bcafac807d070a910fa3286ab0ce4ae437b223d",
			"ProfileIcons/{574e775e-4f2a-5b96-ac1e-a2962a402336}.scale-200.png": "43f262ca440644b97e1d76411238bd9c87c7137e19dde948a39920389907b522",
			"ProfileIcons/{61c54bbd-c2c6-5271-96e7-009a87ff44bf}.scale-100.png": "225f69c01321d74637be8d3a041469ff141a7b50e37d529d6b185543030d86ee",
			"ProfileIcons/{61c54bbd-c2c6-5271-96e7-009a87ff44bf}.scale-200.png": "ce9b7c608daac3e788b589c28dfd0dc3477bee6b46535dec2e5c9a4372470045",
			"ProfileIcons/{9acb9455-ca41-5af7-950f-6bca1bc9722f}.scale-100.png": "92e13dc9a7989969667889390d771b364147fc94b259e01b8df64b4ed6d6a2f5",
			"ProfileIcons/{9acb9455-ca41-5af7-950f-6bca1bc9722f}.scale-200.png": "8e52bdaff8d8307d1e0585cbb17282a46d69af8846ebea37e576b4c9c019d577",
			"ProfileIcons/{b453ae62-4e3d-5e58-b989-0a998ec441b8}.scale-100.png": "8180f10e3746de383720b1ba708f385962d66696f6892c51dda4aa7cc0c8de02",
			"ProfileIcons/{b453ae62-4e3d-5e58-b989-0a998ec441b8}.scale-200.png": "db182b52129d6cd66fa60e9ec74e690369eb053699a58ef5707da9c4399d60fe",
			"TerminalApp.dll":                                                   "92de39f7be239997f6e15c6bed5f2c52452cce7238e79d920e98af5f97a4091d",
			"TerminalConnection.dll":                                            "d3e4f2ad8c3b15328754b29ae58caed0a86c4d0e700ddf384c78d04a58894156",
			"TerminalThemeHelpers.dll":                                          "8ba64350a3846631f6050c5a239c785d09caf46cb84958b2439e9f05e383d73c",
			"WindowsTerminal.exe":                                               "1cbe52914f7437a4f064327383f1b4c45fe2eb8e92d4f8dae0ad7efcb13a20a1",
			"WindowsTerminalShellExt.dll":                                       "b624b7efe4efd7b43bbed1a590334fc921ceceba83b379b8cac66505814e7aae",
			"defaults.json":                                                     "5abe4fefaf0d0d1cb160924ba55b26677e9c5c41eec4b0ad3d2c57a0e3ccf504",
			"elevate-shim.exe":                                                  "c57b198e7331041ecfaf06fad4feeb97b32ec97641393e1766b44f9e284e19a4",
			"resources.pri":                                                     "e4ed7c2d9c3641ad08e9d5071786b4cba6f06df2ce390e119d5b0e11d384c6b3",
			"wt.exe":                                                            "9599bd3d79c86e8c2000d4098f9099fc7c1cbeb8aceaa93e6b3c8ae9951acc27",
		}},
	{Version: "1.24.11911.0", Commits: []string{legacyStableCommit}, Prefix: "terminal-1.24.11911.0/",
		URL:    "https://github.com/microsoft/terminal/releases/download/v1.24.11911.0/Microsoft.WindowsTerminal_1.24.11911.0_x64.zip",
		SHA256: "7691efeb71c8dd0b95536c84e366fa4cf809a42c534912f9cefa1056534383bd", Files: map[string]string{
			"CascadiaCode.ttf":                                                  "54eae6e26961e980eb3e1ec4ee30f6d3ba7a932a2028832baceb91cc65844a65",
			"CascadiaCodeItalic.ttf":                                            "48e64f8d82acd86956df5f33076966eacf4d9c3ea620c9d88231fa94da117f2c",
			"CascadiaMono.ttf":                                                  "08026f617eea315caca59898784e999f6cdb2d0c34d2040deeb042bcdc359fa2",
			"CascadiaMonoItalic.ttf":                                            "d9d23dfb03c6844448a24cf0337a657ac99965262874d2e6fb5b82ac0ac08c4d",
			"Images/terminal_contrast-black.ico":                                "29864669d81e7d4e50a8ecc16e992b339658454f5234e1949c512a07edbbabf4",
			"Images/terminal_contrast-white.ico":                                "0b452ba3cc2ef2e330422aaa55a78fe59b5454279bb35778f2633d770f592f3b",
			"Microsoft.Terminal.Control.dll":                                    "4276b8245f17f0350c3b5fa9fc339ff379af9da81393a65b41136481ed09b286",
			"Microsoft.Terminal.Settings.Editor.dll":                            "70914eea3d8a0fa02e6864d5a89d8b6ceeca5e00165448356751c472cf24b1d3",
			"Microsoft.Terminal.Settings.Model.dll":                             "f3a8c1d6f3c0c53ffb790cd0547d6739ee1e43c14eb42f658059bc59b1158ef5",
			"Microsoft.Terminal.UI.Markdown.dll":                                "b645735812fa04197a1755221ef76c7211603210c63b0255392aefa88b1f807f",
			"Microsoft.Terminal.UI.dll":                                         "19dcf753ef15065ee511688273ba14039ebb5d025b5bac3ede50e8c443b750e3",
			"Microsoft.UI.Xaml.dll":                                             "8b92de46c8950ced663e970a3352d3930ff3c978f3c73770935b5316cf6ca5ad",
			"Microsoft.UI.Xaml/Assets/NoiseAsset_256x256_PNG.png":               "4f2aa94a2e345a32dae689176b86c644bef87081ff9daef8c77417731862031b",
			"NOTICE.html":                                                       "9691413dfd4979a89d3a02853a7153fe5f40df937cc817dd2715621ec8ea04ef",
			"OpenConsole.exe":                                                   "b7fd936c2668b87b9ecf7b3366dc6568afc1c6f981874cba3e955a1c35cf8160",
			"OpenConsoleProxy.dll":                                              "422d3c5d84b19defed77286e02f3bd7476d42533eb0838aef5bc79f58d42eefd",
			"ProfileGeneratorIcons/AzureCloudShell.png":                         "db182b52129d6cd66fa60e9ec74e690369eb053699a58ef5707da9c4399d60fe",
			"ProfileGeneratorIcons/PowerShell.png":                              "69da2be6d04562817eb7e379c394bd738371882b6db30a1a0c304a7379d67443",
			"ProfileGeneratorIcons/SSH.png":                                     "f61db9bcc372516558e00cfebc4ff9c310675cd4c9ec6bfe2dc97b86120db376",
			"ProfileGeneratorIcons/VisualStudio.png":                            "1076e1c09147816ee95747fc3420c695eb1ebbabd30a27248590b7d2f374c791",
			"ProfileGeneratorIcons/WSL.png":                                     "8e52bdaff8d8307d1e0585cbb17282a46d69af8846ebea37e576b4c9c019d577",
			"ProfileIcons/pwsh-preview.scale-100.png":                           "592862382a46c236c50899ec68ced77630dac228c2c588955c7b89d38f7c46a7",
			"ProfileIcons/pwsh-preview.scale-125.png":                           "7425cb7ca6c49012a7f04ef8f2f9e8a45ac11af4f4445a87ecf73848ba022628",
			"ProfileIcons/pwsh-preview.scale-150.png":                           "f73a78e761d53185ac9f5d693d8e23505f7f21131b9c562b3dfc68a33e0299af",
			"ProfileIcons/pwsh-preview.scale-200.png":                           "47d2c2fb504cb7abe79184ba5857ea78f8b2ddae5667a53a7472c421d121b9f6",
			"ProfileIcons/pwsh.scale-100.png":                                   "1e870e4eeda1440e5c8eeb673806466beaf156eb26dfa41539480d23433fe5e2",
			"ProfileIcons/pwsh.scale-125.png":                                   "6fcaa38c91cbc961dc69b29d98e24323048965dbc53f3570e0bd683fd4bb6161",
			"ProfileIcons/pwsh.scale-150.png":                                   "f1f79191a7a9a2b3cf32a7dd3ec23810494faadde0fcefc29e3a9b00b155dc03",
			"ProfileIcons/pwsh.scale-200.png":                                   "69da2be6d04562817eb7e379c394bd738371882b6db30a1a0c304a7379d67443",
			"ProfileIcons/vs-cmd.scale-100.png":                                 "82145026660f5550437bbf73c32a006244f8845b531d4bbec2d4903860938452",
			"ProfileIcons/vs-cmd.scale-150.png":                                 "00bfd2ce8690914feb08b9fc079bfd52378f5687bfc89b6c276977e08766e8eb",
			"ProfileIcons/vs-cmd.scale-200.png":                                 "95315646a7ff0b3e387c0cb3572fe2c9ba46410b90d50f69beca8fa406621937",
			"ProfileIcons/vs-powershell.scale-100.png":                          "5b070331f25f4bc00ecfc41455087ca42514766f09df44ab212da5790f5e5b60",
			"ProfileIcons/vs-powershell.scale-150.png":                          "95a5fd04df60b722df61d2dbf3ad109713010fc8b38ad3ea1c0b40ad9629dff5",
			"ProfileIcons/vs-powershell.scale-200.png":                          "54f619cbfdc7c2297d893e8cc6c0a8c045db1daeabc8fa1fa43439f15b45a59e",
			"ProfileIcons/vs-pwsh.scale-100.png":                                "b32bf11e4f4fe8f82ddc2b65c028c30ffe9abe79faf6981565aae4561ba46d45",
			"ProfileIcons/vs-pwsh.scale-150.png":                                "b4b87122c42468d12c774e3ca0e5f9dc3c3e4d632aa60b0b9be0b8588ca6b38f",
			"ProfileIcons/vs-pwsh.scale-200.png":                                "2a8ae61e7aeba63978d75cb7b8d76fb4f7abfd069dd093638812a8ec46154e81",
			"ProfileIcons/{0caa0dad-35be-5f56-a8ff-afceeeaa6101}.scale-100.png": "b774f1b51667fa6ee4eccf9568dadd5dea40e79ef847e163e80b279d55338511",
			"ProfileIcons/{0caa0dad-35be-5f56-a8ff-afceeeaa6101}.scale-200.png": "0d575b3d46310599369bcb41ebedd4fd8e3d76e6887bdf8ade9e2fb7f06667d0",
			"ProfileIcons/{574e775e-4f2a-5b96-ac1e-a2962a402336}.scale-100.png": "8f81d3436aa7a675ed6e17f73bcafac807d070a910fa3286ab0ce4ae437b223d",
			"ProfileIcons/{574e775e-4f2a-5b96-ac1e-a2962a402336}.scale-200.png": "43f262ca440644b97e1d76411238bd9c87c7137e19dde948a39920389907b522",
			"ProfileIcons/{61c54bbd-c2c6-5271-96e7-009a87ff44bf}.scale-100.png": "225f69c01321d74637be8d3a041469ff141a7b50e37d529d6b185543030d86ee",
			"ProfileIcons/{61c54bbd-c2c6-5271-96e7-009a87ff44bf}.scale-200.png": "ce9b7c608daac3e788b589c28dfd0dc3477bee6b46535dec2e5c9a4372470045",
			"ProfileIcons/{9acb9455-ca41-5af7-950f-6bca1bc9722f}.scale-100.png": "92e13dc9a7989969667889390d771b364147fc94b259e01b8df64b4ed6d6a2f5",
			"ProfileIcons/{9acb9455-ca41-5af7-950f-6bca1bc9722f}.scale-200.png": "8e52bdaff8d8307d1e0585cbb17282a46d69af8846ebea37e576b4c9c019d577",
			"ProfileIcons/{b453ae62-4e3d-5e58-b989-0a998ec441b8}.scale-100.png": "8180f10e3746de383720b1ba708f385962d66696f6892c51dda4aa7cc0c8de02",
			"ProfileIcons/{b453ae62-4e3d-5e58-b989-0a998ec441b8}.scale-200.png": "db182b52129d6cd66fa60e9ec74e690369eb053699a58ef5707da9c4399d60fe",
			"TerminalApp.dll":                                                   "819c870debd3f2cc9450b057958788227d22204a73ad94756e86d903c4c5595e",
			"TerminalConnection.dll":                                            "5fc79585c2a263de4cc0f149626587715c1c75dbe388b8bf00324f53fd9a4e33",
			"TerminalThemeHelpers.dll":                                          "f8d058b3b8a7c38143f8e3b04c5e7186643d80dab578f17d042c1f92507f4bc6",
			"WindowsTerminal.exe":                                               "c2159565e068cee5a0018d89560f4d8d40364e91380dfe27a6ffb2b8b8c8bf89",
			"WindowsTerminalShellExt.dll":                                       "873b776f044bde5d51735b679208fb6a639446e374ab1b6c63b44791a6d31f07",
			"defaults.json":                                                     "39d3d978ebd3f3429e7eaf84b1e6020cdbf09bedf4e139751fdf537180475d0a",
			"elevate-shim.exe":                                                  "3108677b7aa4d6b6092226f87a086c1053c39f17aa59eb08f162773a6ecf8f99",
			"resources.pri":                                                     "7937557f926747c3c77aed7b5635478052b92a9eb555d2386bf46349036c6023",
			"wt.exe":                                                            "28649d867b398fa9e968c69d42d8cf7dea22b34cf427a255d7983be0f361d899",
		}},
}

// Hack.zip. Windows copied its twelve faces into the per-user font folder and
// registered each as "<name> (TrueType)"; Linux unzipped every member into
// ${XDG_DATA_HOME:-~/.local/share}/fonts/HackNerdFont.
var legacyHackFontReleases = []legacyReleaseTree{
	{Version: "v3.5.1", Commits: []string{legacyReleaseCommit}, URL: "https://github.com/ryanoasis/nerd-fonts/releases/download/v3.5.1/Hack.zip",
		SHA256: "fa24da7de7cefe7766614d27762570b20453c852fc1d5b657111666df9a5e449", Files: map[string]string{
			"HackNerdFont-Bold.ttf":            "add70efb723227c08400c4d0f0068cf9f1783e22f6702b811bafbf3f5920902c",
			"HackNerdFont-BoldItalic.ttf":      "81f1b2b159e934f29c355814ce4f0e7bcbc3926a46a4c46da89f7e0bb7873ad9",
			"HackNerdFont-Italic.ttf":          "c54feb21d2296e112aea1867284343aef67173046b05e4446d700e1acc7aec9e",
			"HackNerdFont-Regular.ttf":         "8cba545f0ab36d8f313a448676df9988d6c679a014ffed863e52305844e7b113",
			"HackNerdFontMono-Bold.ttf":        "7c2c76c26ff80fb546798b204ae519335573417c969eb3142d202298722d3832",
			"HackNerdFontMono-BoldItalic.ttf":  "ad9e8d4504c0ffaab07ff44bb98cc0cfd67655df9cda97f5c9fc15e77b301497",
			"HackNerdFontMono-Italic.ttf":      "7b45fbee724e4a58bca6e23d176006d6334cdc848d0ec53ec38daef252900fa8",
			"HackNerdFontMono-Regular.ttf":     "e5aeb58577a8acf2b42083e4a07ce48bd53116fdd868636403cadb5b547351be",
			"HackNerdFontPropo-Bold.ttf":       "2d2381c1ca2fb0a1066a9412cdc10df0f1f25140d084554214c33dc96f8295fe",
			"HackNerdFontPropo-BoldItalic.ttf": "c0d27b05e31d73777741b056c45290365be9843fe4a8a44107951a650c121308",
			"HackNerdFontPropo-Italic.ttf":     "b9aa1b4b3c1630f04c95c3a3cff838220f50820c497ce85611cf00ce44425469",
			"HackNerdFontPropo-Regular.ttf":    "81c4fe14ac6d7866f84d881eb5b0edf726eb21f07dd9228d3ac4895cd371ff46",
			"LICENSE.md":                       "1f61bb7c790c59b4b0ecdf304628b94e42ae4c8020094a8c3da73381ab212623",
			"README.md":                        "3071e1a8fadca84ed6ee14580fac50c04a623120c9e85fae1e40e758f487c6b5",
		}},
	{Version: "v3.4.0", Commits: []string{legacyStableCommit}, URL: "https://github.com/ryanoasis/nerd-fonts/releases/download/v3.4.0/Hack.zip",
		SHA256: "8ca33a60c791392d872b80d26c42f2bfa914a480f9eb2d7516d9f84373c36897", Files: map[string]string{
			"HackNerdFont-Bold.ttf":            "7fb835cbd3273d509868dcd4e03eab3dc98679ac0bdffd52ef23411244396082",
			"HackNerdFont-BoldItalic.ttf":      "1a6fdc79f82beb3ca3a0bc2088568a24b094b55f0f93cbf9731bf8e1ede85d6b",
			"HackNerdFont-Italic.ttf":          "b65edfcbdce25a6f359b6ad129f51fc0b7c59e4893676fc3b75a4f49e3a12b18",
			"HackNerdFont-Regular.ttf":         "7e6b5d86baee613984b10cef14c8d6aee86c976a3d1cbd87abffd424d6ec4c64",
			"HackNerdFontMono-Bold.ttf":        "711084fdea9f9eb4e5dbca372a19e6a5af996fc88bfce55918eeef560f0f6722",
			"HackNerdFontMono-BoldItalic.ttf":  "82fca6ff9e87bc65b6abb1bbde1e3884fa9418bfb14f15b9db2d3274b87bf44e",
			"HackNerdFontMono-Italic.ttf":      "86c6e1b14e2cb02ac8041269c53dac3673c70ff58375d1aafe0ecff8087f8126",
			"HackNerdFontMono-Regular.ttf":     "03e60d3c1a9f8bef4e1f78836f80aacb9ec005260a6b094f5bfc10043bb115ab",
			"HackNerdFontPropo-Bold.ttf":       "0db6462b7d70d43ea5f7909cb2a21362cba0d6229852dabfae143524c71d0c8e",
			"HackNerdFontPropo-BoldItalic.ttf": "a494c1f56478b50ccaa779f9412d16dfb3d56b294f8b46073bb7f6706090a3e5",
			"HackNerdFontPropo-Italic.ttf":     "c76ae22703e44c53ec73d250e3d742e59206f643ebef955edbf1d2de89737b7d",
			"HackNerdFontPropo-Regular.ttf":    "407aaea4972909bcb2c818a294d1b8856155e12c7a69307dade072195833fb46",
			"LICENSE.md":                       "1f61bb7c790c59b4b0ecdf304628b94e42ae4c8020094a8c3da73381ab212623",
			"README.md":                        "59c2d30c9bf4e362a76b5b0b1a4bd51cd5be259abcdcae29033b0c12895d927a",
		}},
}

// legacyReleaseMember reports whether name has these exact bytes in any release.
func legacyReleaseMember(releases []legacyReleaseTree, name, hash string) bool {
	return slices.ContainsFunc(releases, func(release legacyReleaseTree) bool { return release.Files[name] == hash && hash != "" })
}

func legacyReleaseHashes(releases []legacyReleaseTree, name string) []string {
	hashes := []string{}
	for _, release := range releases {
		if hash := release.Files[name]; hash != "" {
			hashes = append(hashes, hash)
		}
	}
	return hashes
}

// psmux/psmux-plugins at the released commit; Git for Windows may have checked
// them out with CRLF, so the LF hashes are compared after that conversion.
var legacyPsmuxResurrectFiles = map[string]string{
	"README.md":           "96d7d8f8c1afaa561c2f9bf69b3d829b58bc019868cd0b1c6f8caac7df97d46a",
	"plugin.conf":         "643d777959099a019a82e4326b24aef11e4392026f8726b50b157d05e6f9ca1e",
	"psmux-resurrect.ps1": "ebc744bcf5e87c281490320d7d16605558e9aa6d0110b01ade77e6b28949f304",
	"scripts/restore.ps1": "488b86c6f6ced0cdeea5d40ad1a57015447ad03beb160262364e2490898e292c",
	"scripts/save.ps1":    "fba8f558c6265db95596faf439db6ff0cf1ec9549ba087014a88401d553214a6",
}

func (d *legacyRetirementDriver) planWindowsDirect(ctx context.Context, scope *legacyRetirementScope, delta legacyManagerDelta) error {
	local := d.Folders.LocalAppData
	for _, recipe := range delta.Direct {
		var err error
		switch {
		case !filepath.IsAbs(local):
			err = errors.New("legacy Windows retirement requires the actual LocalAppData folder")
		case recipe == "herdr":
			err = d.planWindowsExecutable(ctx, scope, filepath.Join(local, "Programs", "Herdr", "bin", "herdr.exe"), legacyReleaseHashes(legacyHerdrWindowsReleases, "herdr-windows-x86_64.exe"), filepath.Join(local, "Programs", "Herdr"))
		case recipe == "tree-sitter":
			// Its parent is setup's own state directory, which always stays.
			err = d.planWindowsExecutable(ctx, scope, filepath.Join(local, "dotfiles", "bin", "tree-sitter.exe"), legacyReleaseHashes(legacyTreeSitterWindowsReleases, "tree-sitter.exe"), "")
		case recipe == "windows-terminal":
			err = d.planWindowsTerminalPortable(ctx, scope, filepath.Join(local, "Programs", "WindowsTerminal"))
		case recipe == "psmux-plugins":
			err = d.planPsmuxPlugins(scope)
		case recipe == "hack-font":
			err = d.planWindowsHackFonts(scope)
		case recipe == "pylatexenc":
			root := filepath.Join(local, "dotfiles", "python-tools", "pylatexenc")
			var removing bool
			removing, err = d.planPylatexencVenv(scope, root)
			if err == nil && (removing || legacyAbsent(root)) {
				err = d.planUserPathEntry(ctx, scope, filepath.Join(root, "Scripts"))
			}
		case recipe == "psfzf":
			err = d.planPSFzfModules(scope)
		default:
			err = fmt.Errorf("unknown released Windows artifact %q", recipe)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func legacyAbsent(path string) bool {
	_, err := os.Lstat(path)
	return errors.Is(err, os.ErrNotExist)
}

// Remove one released executable whose bytes still match its pin. Its directory
// and released parent go only when nothing else remains; then the User PATH
// entry the released installer added for it goes too.
func (d *legacyRetirementDriver) planWindowsExecutable(ctx context.Context, scope *legacyRetirementScope, path string, hashes []string, parent string) error {
	directory := filepath.Dir(path)
	removing := false
	info, err := os.Lstat(path)
	switch {
	case err == nil:
		if !info.Mode().IsRegular() {
			scope.Retained = append(scope.Retained, path+" is not a regular file; it was preserved")
			return nil
		}
		actual, err := legacyFileSHA256(path)
		if err != nil {
			return err
		}
		if !slices.Contains(hashes, actual) {
			scope.Retained = append(scope.Retained, path+" differs from every released pinned binary; it was preserved")
			return nil
		}
		if err := d.addGatedDiscard(scope, "Remove the released executable", path, []string{directory}); err != nil {
			return err
		}
		removing = len(scope.Steps) != 0 && scope.Steps[len(scope.Steps)-1].Path == path
	case !errors.Is(err, os.ErrNotExist):
		return err
	}
	entries, err := os.ReadDir(directory)
	if errors.Is(err, os.ErrNotExist) {
		return d.planUserPathEntry(ctx, scope, directory)
	}
	if err != nil {
		return err
	}
	if len(entries) != 0 && !(removing && len(entries) == 1) {
		return nil // Other files keep the directory and its PATH entry.
	}
	for _, empty := range []string{directory, parent} {
		if empty != "" {
			scope.Steps = append(scope.Steps, legacyRetirementStep{Kind: "rmdir", Label: "Remove the empty released directory " + empty, Path: empty})
		}
	}
	return d.planUserPathEntry(ctx, scope, directory)
}

// Discard a recorded tree after confirming no process runs from it; a locked
// executable would otherwise leave a partially deleted installation.
func (d *legacyRetirementDriver) addGatedDiscard(scope *legacyRetirementScope, label, path string, directories []string) error {
	before := len(scope.Steps)
	if err := addLegacyDiscard(scope, label, path); err != nil || len(scope.Steps) == before {
		return err
	}
	gate, err := legacyProcessGate(directories, nil)
	if err != nil {
		return err
	}
	if d.Platform.WindowsPowerShell == "" {
		scope.Steps = scope.Steps[:before]
		scope.Retained = append(scope.Retained, path+" cannot be checked for running processes without Windows PowerShell; it was preserved")
		return nil
	}
	scope.Steps[before].Gates = append(scope.Steps[before].Gates, gate)
	return nil
}

// The exact released portable tree; any other file means it is not ours.
func (d *legacyRetirementDriver) planWindowsTerminalPortable(ctx context.Context, scope *legacyRetirementScope, root string) error {
	entries, err := inspectTree(root, maxConfigBytes, maxConfigEntries)
	if err != nil {
		scope.Retained = append(scope.Retained, legacyReason(root+" could not be recorded; it was preserved", err))
		return nil
	}
	if len(entries) == 0 {
		return d.planUserPathEntry(ctx, scope, root)
	}
	release, difference := legacyMatchingTree(entries, legacyWindowsTerminalReleases)
	if release == nil {
		scope.Retained = append(scope.Retained, root+" is not a complete released portable Windows Terminal ("+difference+"); it was preserved")
		return nil
	}
	if err := d.addGatedDiscard(scope, "Remove the released portable Windows Terminal "+release.Version+" (settings in LocalAppData stay)", root, []string{root}); err != nil {
		return err
	}
	if len(scope.Steps) != 0 && scope.Steps[len(scope.Steps)-1].Path == root {
		return d.planUserPathEntry(ctx, scope, root)
	}
	return nil
}

// legacyMatchingTree returns the release whose members are exactly this
// recorded tree, or a description of the first difference.
func legacyMatchingTree(entries []configEntry, releases []legacyReleaseTree) (*legacyReleaseTree, string) {
	difference := "it is empty"
	for i := range releases {
		release := &releases[i]
		directories := map[string]bool{".": true}
		for name := range release.Files {
			for parent := path.Dir(name); parent != "."; parent = path.Dir(parent) {
				directories[parent] = true
			}
		}
		files, mismatch := 0, ""
		for _, entry := range entries {
			relative := filepath.ToSlash(entry.Path)
			switch {
			case entry.Kind == "directory" && directories[relative]:
			case entry.Kind == "file" && release.Files[relative] == entry.Content:
				files++
			default:
				mismatch = relative + " is not part of release " + release.Version
			}
			if mismatch != "" {
				break
			}
		}
		if mismatch == "" && files == len(release.Files) {
			return release, ""
		}
		if mismatch == "" {
			mismatch = fmt.Sprintf("%d of release %s's %d files are present", files, release.Version, len(release.Files))
		}
		if i == 0 {
			difference = mismatch
		}
	}
	return nil, difference
}

// Saved sessions live in ~\.psmux\resurrect and stay with the folder.
func (d *legacyRetirementDriver) planPsmuxPlugins(scope *legacyRetirementScope) error {
	plugins := filepath.Join(d.Folders.Home, ".psmux", "plugins")
	root := filepath.Join(plugins, "psmux-resurrect")
	entries, err := inspectTree(root, maxConfigBytes, maxConfigEntries)
	if err != nil || len(entries) == 0 {
		if err != nil {
			scope.Retained = append(scope.Retained, legacyReason(root+" could not be recorded; it was preserved", err))
		}
		return nil
	}
	files := 0
	for _, entry := range entries {
		relative := filepath.ToSlash(entry.Path)
		if entry.Kind == "directory" && (relative == "." || relative == "scripts") {
			continue
		}
		matched := false
		if entry.Kind == "file" {
			data, _, err := readLegacyFile(filepath.Join(root, entry.Path), 1<<20)
			if err != nil {
				return err
			}
			if relative == ".pinned-commit" {
				matched = string(data) == legacyPsmuxPluginsCommit
			} else if want := legacyPsmuxResurrectFiles[relative]; want != "" {
				matched = legacyHash(data) == want || bytes.Contains(data, []byte("\r\n")) && legacyHash(bytes.ReplaceAll(data, []byte("\r\n"), []byte("\n"))) == want
			}
		}
		if !matched {
			scope.Retained = append(scope.Retained, root+" contains "+relative+", which differs from the released psmux-resurrect "+legacyPsmuxPluginsCommit[:12]+"; it was preserved")
			return nil
		}
		files++
	}
	if files != len(legacyPsmuxResurrectFiles)+1 {
		scope.Retained = append(scope.Retained, root+" is not the complete released psmux-resurrect checkout; it was preserved")
		return nil
	}
	if err := addLegacyDiscard(scope, "Remove the released psmux-resurrect plugin", root); err != nil {
		return err
	}
	for _, directory := range []string{plugins, filepath.Dir(plugins)} {
		scope.Steps = append(scope.Steps, legacyRetirementStep{Kind: "rmdir", Label: "Remove the empty released directory " + directory, Path: directory})
	}
	scope.Detail = append(scope.Detail, "Saved psmux sessions and personal psmux configuration stay.")
	return nil
}

// Registered by name "<face> (TrueType)" with the full file path as data.
const legacyWindowsFontRemoval = `
$key = [Microsoft.Win32.Registry]::CurrentUser.CreateSubKey($fontKey)
try {
	foreach ($face in @($inputData.faces)) {
		$registration = Get-Registration $face.name
		if ($registration -ne '' -and $registration -cne $face.path) { throw ('A different font now uses the registration ' + $face.name) }
		$present = Test-Path -LiteralPath $face.path -PathType Leaf
		if ($present -and (Get-FileHash -LiteralPath $face.path -Algorithm SHA256).Hash.ToLowerInvariant() -cne $face.hash) { throw ('Font file changed after approval: ' + $face.path) }
		if ($present) { [void][DotfilesFont]::RemoveFontResourceExW($face.path, 0, [IntPtr]::Zero) }
		if ($registration -ne '') { $key.DeleteValue($face.name, $false) }
		if ($present) { Remove-Item -LiteralPath $face.path -Force }
	}
} finally { $key.Dispose() }
[IntPtr]$result = [IntPtr]::Zero
[void][DotfilesFont]::SendMessageTimeoutW([IntPtr]0xffff, 0x001d, [IntPtr]::Zero, [IntPtr]::Zero, 2, 2000, [ref]$result)
`

// Fonts owned by Scoop's Hack-NF are retired by Scoop instead. Each face is
// its own step, so a font still held open stops only that face's removal and
// a retry continues from it.
func (d *legacyRetirementDriver) planWindowsHackFonts(scope *legacyRetirementScope) error {
	if !legacyAbsent(filepath.Join(d.scoopRoot(), "apps", "Hack-NF")) {
		return nil
	}
	folder := filepath.Join(d.Folders.LocalAppData, "Microsoft", "Windows", "Fonts")
	type face struct {
		Name string `json:"name"`
		Path string `json:"path"`
		Hash string `json:"hash"`
	}
	names := []string{}
	for name := range legacyHackFontReleases[0].Files {
		if strings.HasSuffix(name, ".ttf") {
			names = append(names, name)
		}
	}
	slices.Sort(names)
	steps := []legacyRetirementStep{}
	for _, name := range names {
		path := filepath.Join(folder, name)
		file, err := legacyFile(path)
		if err != nil {
			return err
		}
		if file.Snapshot.Kind == "absent" {
			continue
		}
		actual, err := legacyFileSHA256(path)
		if err != nil || file.Snapshot.Kind != "file" || !legacyReleaseMember(legacyHackFontReleases, name, actual) {
			scope.Retained = append(scope.Retained, legacyReason(path+" differs from the released Hack Nerd Font faces; it was preserved", err))
			continue
		}
		input, err := json.Marshal(struct {
			Faces []face `json:"faces"`
		}{[]face{{strings.TrimSuffix(name, ".ttf") + " (TrueType)", path, actual}}})
		if err != nil {
			return err
		}
		command := nativeCommand{Program: d.Platform.WindowsPowerShell, Arguments: windowsVendorArguments(windowsFontPrelude + legacyWindowsFontRemoval), Input: input}
		steps = append(steps, legacyRetirementStep{Kind: "command", Label: "Unregister and remove the released font " + path, Command: &command,
			Before: []legacyRetirementFile{file}, Absent: []string{path}})
	}
	if len(steps) == 0 {
		return nil
	}
	if d.Platform.WindowsPowerShell == "" {
		scope.Retained = append(scope.Retained, "the released Hack Nerd Font files in "+folder+" need Windows PowerShell to unregister; they were preserved")
		return nil
	}
	scope.Steps = append(scope.Steps, steps...)
	scope.Detail = append(scope.Detail, "Close applications using Hack Nerd Font if a face cannot be removed; setup's own Dotfiles-prefixed faces stay registered.")
	return nil
}

// The released converter venv contains exactly pip plus the pinned setuptools
// and pylatexenc distributions. Each top-level entry is recorded separately so
// the bounded deletion never exceeds its entry limit.
func (d *legacyRetirementDriver) planPylatexencVenv(scope *legacyRetirementScope, root string) (bool, error) {
	entries, err := readPlainDirectory(root)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		scope.Retained = append(scope.Retained, legacyReason(root+" is not a plain directory; it was preserved", err))
		return false, nil
	}
	site := filepath.Join(root, "Lib", "site-packages")
	if d.Platform.OS != "windows" {
		matches, err := filepath.Glob(filepath.Join(root, "lib", "python3.*", "site-packages"))
		if err != nil || len(matches) != 1 {
			scope.Retained = append(scope.Retained, root+" does not have one Python site-packages directory; it was preserved")
			return false, err
		}
		site = matches[0]
	}
	packages, err := readPlainDirectory(site)
	if err != nil {
		scope.Retained = append(scope.Retained, legacyReason(root+" has no readable site-packages; it was preserved", err))
		return false, nil
	}
	distributions := []string{}
	for _, entry := range packages {
		if strings.HasSuffix(entry.Name(), ".dist-info") {
			name := strings.TrimSuffix(entry.Name(), ".dist-info")
			if strings.HasPrefix(name, "pip-") {
				name = "pip"
			}
			distributions = append(distributions, name)
		}
	}
	slices.Sort(distributions)
	released := slices.Equal(distributions, []string{"pip", "pylatexenc-" + legacyPylatexencVersion, "setuptools-" + legacySetuptoolsVersion}) ||
		slices.Equal(distributions, []string{"pip", "pylatexenc-" + legacyPylatexencVersion, "setuptools-83.0.0"}) // v0.4.4
	if info, err := os.Lstat(filepath.Join(root, "pyvenv.cfg")); err != nil || !info.Mode().IsRegular() || !released {
		scope.Retained = append(scope.Retained, root+" is not the released pylatexenc "+legacyPylatexencVersion+" environment; it was preserved")
		return false, nil
	}
	steps, retained := len(scope.Steps), len(scope.Retained)
	paths := []string{}
	for _, entry := range packages {
		paths = append(paths, filepath.Join(site, entry.Name()))
	}
	for _, entry := range entries {
		if path := filepath.Join(root, entry.Name()); !strings.HasPrefix(site, path+string(filepath.Separator)) {
			paths = append(paths, path)
		}
	}
	for _, path := range paths {
		if err := addLegacyDiscard(scope, "Remove the released converter environment entry", path); err != nil {
			return false, err
		}
	}
	if len(scope.Retained) != retained {
		// Retire the environment entirely or not at all.
		scope.Steps = scope.Steps[:steps]
		return false, nil
	}
	for directory := site; ; directory = filepath.Dir(directory) {
		scope.Steps = append(scope.Steps, legacyRetirementStep{Kind: "rmdir", Label: "Remove the empty released directory " + directory, Path: directory})
		if directory == root {
			break
		}
	}
	scope.Steps = append(scope.Steps, legacyRetirementStep{Kind: "rmdir", Label: "Remove the empty released directory " + filepath.Dir(root), Path: filepath.Dir(root)})
	return true, nil
}

var (
	legacyPSGetName       = regexp.MustCompile(`<S N="Name">PSFzf</S>`)
	legacyPSGetRepository = regexp.MustCompile(`<S N="Repository">PSGallery</S>`)
)

// Install-Module -Scope CurrentUser placed PSFzf beside personal modules. The
// whole module folder moves aside intact; a renamed folder is not discovered.
func (d *legacyRetirementDriver) planPSFzfModules(scope *legacyRetirementScope) error {
	if !filepath.IsAbs(d.Folders.Documents) {
		return nil
	}
	for _, edition := range []string{"WindowsPowerShell", "PowerShell"} {
		directory := filepath.Join(d.Folders.Documents, edition, "Modules", "PSFzf")
		versions, err := readPlainDirectory(directory)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		gallery := err == nil && len(versions) != 0
		for _, version := range versions {
			data, _, readErr := readLegacyFile(filepath.Join(directory, version.Name(), "PSGetModuleInfo.xml"), 1<<20)
			gallery = gallery && readErr == nil && legacyPSGetName.Match(data) && legacyPSGetRepository.Match(data)
		}
		if !gallery {
			scope.Retained = append(scope.Retained, legacyReason(directory+" is not only the PowerShell Gallery PSFzf module; it was preserved", err))
			continue
		}
		if err := d.addUserMove(scope, "Move the released PowerShell Gallery PSFzf module aside", directory); err != nil {
			return err
		}
	}
	return nil
}

// User PATH entries are read raw from HKCU\Environment. Removal keeps every
// other byte and the value's registry type; new processes see the change.
const legacyUserPathRead = `
$ErrorActionPreference = 'Stop'
$request = $env:DOTFILES_RETIREMENT_INPUT | ConvertFrom-Json
$entry = ([string]$request.entry).TrimEnd('\')
$kind = 'Absent'
$value = ''
$key = [Microsoft.Win32.Registry]::CurrentUser.OpenSubKey('Environment')
if ($key) {
	try {
		if (@($key.GetValueNames()) -contains 'Path') {
			$kind = [string]$key.GetValueKind('Path')
			$value = [string]$key.GetValue('Path', '', [Microsoft.Win32.RegistryValueOptions]::DoNotExpandEnvironmentNames)
		}
	} finally { $key.Dispose() }
}
$parts = @($value.Split(';'))
$matches = @($parts | Where-Object { $_.TrimEnd('\') -ieq $entry }).Count
$sha = [Security.Cryptography.SHA256]::Create()
try { $hash = [BitConverter]::ToString($sha.ComputeHash([Text.Encoding]::UTF8.GetBytes($kind + [char]10 + $value))).Replace('-', '').ToLowerInvariant() }
finally { $sha.Dispose() }
`

const legacyUserPathQuery = legacyUserPathRead + `
[Console]::Out.Write(('{0} {1}' -f $matches, $hash))
`

const legacyUserPathEdit = legacyUserPathRead + `
if ($hash -cne $request.hash) { [Console]::Error.WriteLine('The User PATH changed after approval'); exit 72 }
if ($kind -ne 'String' -and $kind -ne 'ExpandString') { exit 75 }
$kept = @($parts | Where-Object { $_.TrimEnd('\') -ine $entry })
if ($kept.Count -eq $parts.Count) { exit 73 }
$key = [Microsoft.Win32.Registry]::CurrentUser.OpenSubKey('Environment', $true)
try { $key.SetValue('Path', ($kept -join ';'), [Microsoft.Win32.RegistryValueKind]$kind) } finally { $key.Dispose() }
`

// Historical user-path gates and scripts above bind the entire value. New
// removals bind only their exact matching entries, so independently approved
// resources can remove their own entries in the same transaction.
const legacyUserPathEntryRead = legacyUserPathRead + `
$owned = @($parts | Where-Object { $_.TrimEnd('\') -ieq $entry })
$sha = [Security.Cryptography.SHA256]::Create()
try { $hash = [BitConverter]::ToString($sha.ComputeHash([Text.Encoding]::Unicode.GetBytes('user-path-entry;' + ($owned -join ';')))).Replace('-', '').ToLowerInvariant() }
finally { $sha.Dispose() }
`

const legacyUserPathEntryQuery = legacyUserPathEntryRead + `
[Console]::Out.Write(('{0} {1}' -f $matches, $hash))
`

const legacyUserPathEntryEdit = legacyUserPathEntryRead + `
if ($hash -cne $request.hash) { [Console]::Error.WriteLine('The approved User PATH entry changed'); exit 72 }
if ($kind -ne 'String' -and $kind -ne 'ExpandString') { exit 75 }
$kept = @($parts | Where-Object { $_.TrimEnd('\') -ine $entry })
if ($kept.Count -eq $parts.Count) { exit 73 }
$key = [Microsoft.Win32.Registry]::CurrentUser.OpenSubKey('Environment', $true)
try {
	$currentKind = [string]$key.GetValueKind('Path')
	$currentValue = [string]$key.GetValue('Path', '', [Microsoft.Win32.RegistryValueOptions]::DoNotExpandEnvironmentNames)
	if ($currentKind -cne $kind -or -not [string]::Equals($currentValue, $value, [StringComparison]::Ordinal)) { [Console]::Error.WriteLine('The User PATH changed during removal'); exit 72 }
	$key.SetValue('Path', ($kept -join ';'), [Microsoft.Win32.RegistryValueKind]$kind)
} finally { $key.Dispose() }
`

func (d *legacyRetirementDriver) userPathEntry(ctx context.Context, entry string) (string, error) {
	return d.queryUserPathEntry(ctx, entry, legacyUserPathQuery)
}

func (d *legacyRetirementDriver) queryUserPathEntry(ctx context.Context, entry, script string) (string, error) {
	command, err := d.windowsScript(script, struct {
		Entry string `json:"entry"`
	}{entry})
	if err != nil {
		return "", err
	}
	output, err := d.Query(ctx, command)
	if err != nil {
		return "", err
	}
	count, hash, ok := strings.Cut(strings.TrimSpace(string(output)), " ")
	matches, convErr := strconv.Atoi(count)
	if !ok || convErr != nil || matches < 0 || !operationID.MatchString(hash) {
		return "", errors.New("invalid User PATH inspection result")
	}
	if matches == 0 {
		return "absent", nil
	}
	return fmt.Sprintf("%d:%s", matches, hash), nil
}

// Only an entry for a directory this retirement removes, or that is already
// gone, is removed; an entry still serving files stays.
func (d *legacyRetirementDriver) planUserPathEntry(ctx context.Context, scope *legacyRetirementScope, directory string) error {
	if d.Platform.WindowsPowerShell == "" {
		scope.Retained = append(scope.Retained, "the User PATH entry "+directory+" cannot be inspected without Windows PowerShell")
		return nil
	}
	value, err := d.queryUserPathEntry(ctx, directory, legacyUserPathEntryQuery)
	if err != nil || value == "absent" {
		return err
	}
	_, hash, _ := strings.Cut(value, ":")
	command, err := d.windowsScript(legacyUserPathEntryEdit, struct {
		Entry string `json:"entry"`
		Hash  string `json:"hash"`
	}{directory, hash})
	if err != nil {
		return err
	}
	scope.Steps = append(scope.Steps, legacyRetirementStep{Kind: "command", Label: "Remove the released entry " + directory + " from the User PATH (other entries stay)", Command: &command,
		Gates:  []legacyRetirementGate{{Kind: "path-absent", Subject: directory, Expected: "absent"}, {Kind: "user-path-entry", Subject: directory, Expected: value}},
		Checks: []legacyRetirementGate{{Kind: "user-path-entry", Subject: directory, Expected: "absent"}}})
	return nil
}

// chezmoi source recognition. Configuration is parsed as data: no template,
// hook or chezmoi command runs. The search mirrors chezmoi v2.73.0: the first
// XDG configuration directory with a config file wins, and the source is
// sourceDir, else the first existing XDG data directory's chezmoi folder.

var legacyChezmoiConfigNames = []string{"chezmoi.json", "chezmoi.jsonc", "chezmoi.toml", "chezmoi.yaml"}

func (d *legacyRetirementDriver) chezmoiBaseDirectory(variable, fallback string) string {
	if d.Platform.OS == "windows" {
		if value := os.Getenv(variable); filepath.IsAbs(value) {
			return filepath.Clean(value)
		}
		return filepath.Join(d.Folders.Home, fallback)
	}
	observed := d.Folders.Config
	if variable == "XDG_DATA_HOME" {
		observed = d.Folders.Data
	}
	if observed != "" {
		return observed
	}
	return filepath.Join(d.Folders.Home, fallback)
}

func legacyXDGDirectories(variable string, defaults ...string) []string {
	values := filepath.SplitList(os.Getenv(variable))
	if len(values) == 0 {
		values = defaults
	}
	directories := []string{}
	for _, value := range values {
		if filepath.IsAbs(value) {
			directories = append(directories, filepath.Clean(value))
		}
	}
	return directories
}

func (d *legacyRetirementDriver) chezmoiConfigFile(userConfig string) (string, error) {
	directories := []string{userConfig}
	if d.Platform.OS != "windows" {
		for _, directory := range legacyXDGDirectories("XDG_CONFIG_DIRS", "/etc/xdg") {
			directories = append(directories, filepath.Join(directory, "chezmoi"))
		}
	}
	for _, directory := range directories {
		entries, err := os.ReadDir(directory)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return "", err
		}
		found := []string{}
		for _, entry := range entries {
			if slices.Contains(legacyChezmoiConfigNames, entry.Name()) {
				found = append(found, filepath.Join(directory, entry.Name()))
			}
		}
		if len(found) > 1 {
			return "", fmt.Errorf("chezmoi refuses multiple configuration files: %s", strings.Join(found, ", "))
		}
		if len(found) == 1 {
			return found[0], nil
		}
	}
	return "", nil
}

func (d *legacyRetirementDriver) chezmoiSourceDirectory(userConfig string) (string, string, error) {
	file, err := d.chezmoiConfigFile(userConfig)
	if err != nil || file == "" {
		if err == nil {
			source, origin := d.chezmoiDefaultSource()
			return source, origin, nil
		}
		return "", "", err
	}
	data, _, err := readLegacyFile(file, 1<<20)
	if err != nil {
		return "", "", fmt.Errorf("%s cannot be read as plain data: %w", file, err)
	}
	value, found, err := legacyChezmoiSourceSetting(file, data)
	if err != nil {
		return "", "", fmt.Errorf("%s: %w", file, err)
	}
	if !found {
		source, origin := d.chezmoiDefaultSource()
		return source, origin, nil
	}
	clean := filepath.Clean(value)
	switch {
	case clean == "~":
		clean = d.Folders.Home
	case strings.HasPrefix(clean, "~"+string(filepath.Separator)) || strings.HasPrefix(clean, "~/"):
		clean = filepath.Join(d.Folders.Home, clean[2:])
	case !filepath.IsAbs(clean):
		return "", "", fmt.Errorf("%s sets a relative sourceDir %q that depends on chezmoi's working directory", file, value)
	}
	return clean, "the sourceDir in " + file, nil
}

func (d *legacyRetirementDriver) chezmoiDefaultSource() (string, string) {
	home := d.chezmoiBaseDirectory("XDG_DATA_HOME", filepath.Join(".local", "share"))
	directories := []string{home}
	if d.Platform.OS != "windows" {
		directories = append(directories, legacyXDGDirectories("XDG_DATA_DIRS", "/usr/local/share", "/usr/share")...)
	}
	for _, directory := range directories {
		if _, err := os.Stat(filepath.Join(directory, "chezmoi")); err == nil {
			return filepath.Join(directory, "chezmoi"), "chezmoi's default source"
		}
	}
	return filepath.Join(home, "chezmoi"), "chezmoi's default source"
}

// legacyChezmoiSourceSetting returns the top-level sourceDir setting. chezmoi
// matches configuration keys case-insensitively. Anything that cannot be read
// unambiguously as data is refused rather than guessed.
func legacyChezmoiSourceSetting(name string, data []byte) (string, bool, error) {
	if !utf8.Valid(data) || bytes.IndexByte(data, 0) >= 0 {
		return "", false, errors.New("the configuration is not UTF-8 text")
	}
	text := strings.TrimPrefix(string(data), "\ufeff")
	switch filepath.Ext(name) {
	case ".json", ".jsonc":
		clean, _, err := jsoncMask([]byte(text))
		if err != nil {
			return "", false, err
		}
		properties, err := jsonProperties(clean)
		if err != nil {
			return "", false, err
		}
		value, found := "", false
		for _, property := range properties {
			if !strings.EqualFold(property.name, "sourceDir") {
				continue
			}
			if found || json.Unmarshal(clean[property.value:property.end], &value) != nil {
				return "", false, errors.New("sourceDir is repeated or is not a string")
			}
			found = true
		}
		return value, found, nil
	case ".toml":
		return legacyTOMLSourceDir(text)
	case ".yaml":
		return legacyYAMLSourceDir(text)
	}
	return "", false, errors.New("unknown chezmoi configuration format")
}

func legacySourceDirKey(key string) bool { return strings.EqualFold(key, "sourcedir") }

// Only top-level keys before the first table can configure chezmoi itself.
func legacyTOMLSourceDir(text string) (string, bool, error) {
	mentions := strings.Contains(strings.ToLower(text), "sourcedir")
	if mentions && (strings.Contains(text, `"""`) || strings.Contains(text, `'''`)) {
		return "", false, errors.New("multi-line strings make the sourceDir setting ambiguous")
	}
	value, found := "", false
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(strings.TrimSuffix(line, "\r"))
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") {
			break
		}
		if !strings.Contains(strings.ToLower(line), "sourcedir") {
			continue
		}
		key, rest, ok := strings.Cut(line, "=")
		key = strings.TrimSpace(key)
		if unquoted, err := strconv.Unquote(key); err == nil {
			key = unquoted
		} else if len(key) > 1 && key[0] == '\'' && key[len(key)-1] == '\'' {
			key = key[1 : len(key)-1]
		}
		if !ok || !legacySourceDirKey(key) {
			if strings.Contains(strings.ToLower(key), "sourcedir") {
				return "", false, errors.New("sourceDir uses an unsupported TOML key form")
			}
			continue
		}
		parsed, err := legacyTOMLString(strings.TrimSpace(rest))
		if err != nil || found {
			return "", false, errors.Join(errors.New("sourceDir is repeated or is not a single-line TOML string"), err)
		}
		value, found = parsed, true
	}
	return value, found, nil
}

func legacyTOMLString(text string) (string, error) {
	if text == "" || text[0] != '"' && text[0] != '\'' {
		return "", errors.New("not a string")
	}
	quote := text[0]
	var value strings.Builder
	i := 1
	for ; i < len(text) && text[i] != quote; i++ {
		if quote == '\'' || text[i] != '\\' {
			value.WriteByte(text[i])
			continue
		}
		if i+1 == len(text) {
			return "", errors.New("unterminated escape")
		}
		i++
		switch text[i] {
		case '\\', '"':
			value.WriteByte(text[i])
		case 'u', 'U':
			width := 4
			if text[i] == 'U' {
				width = 8
			}
			if i+width >= len(text) {
				return "", errors.New("short unicode escape")
			}
			code, err := strconv.ParseUint(text[i+1:i+1+width], 16, 32)
			if err != nil || code > utf8.MaxRune || !utf8.ValidRune(rune(code)) {
				return "", errors.New("invalid unicode escape")
			}
			value.WriteRune(rune(code))
			i += width
		default:
			return "", errors.New("unsupported escape in a path")
		}
	}
	if i == len(text) {
		return "", errors.New("unterminated string")
	}
	if rest := strings.TrimSpace(text[i+1:]); rest != "" && !strings.HasPrefix(rest, "#") {
		return "", errors.New("unexpected content after the string")
	}
	return value.String(), nil
}

// Only unindented keys of a single-document mapping configure chezmoi.
func legacyYAMLSourceDir(text string) (string, bool, error) {
	if !strings.Contains(strings.ToLower(text), "sourcedir") {
		return "", false, nil
	}
	value, found := "", false
	for number, line := range strings.Split(text, "\n") {
		line = strings.TrimSuffix(line, "\r")
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") || number == 0 && trimmed == "---" {
			continue
		}
		if line[0] == ' ' || line[0] == '\t' {
			continue // Nested mappings are template data or other settings.
		}
		if strings.HasPrefix(trimmed, "---") || strings.HasPrefix(trimmed, "...") || strings.ContainsAny(line[:1], "{[-?&*!|>%@`") {
			return "", false, errors.New("YAML documents, flow collections and anchors make sourceDir ambiguous")
		}
		key, rest, ok := strings.Cut(line, ":")
		key = strings.TrimSpace(key)
		if unquoted, err := strconv.Unquote(key); err == nil {
			key = unquoted
		} else if len(key) > 1 && key[0] == '\'' && key[len(key)-1] == '\'' {
			key = key[1 : len(key)-1]
		}
		if !ok || !legacySourceDirKey(key) {
			continue
		}
		parsed, err := legacyYAMLScalar(strings.TrimSpace(rest))
		if err != nil || found {
			return "", false, errors.Join(errors.New("sourceDir is repeated or is not a single-line YAML string"), err)
		}
		value, found = parsed, true
	}
	return value, found, nil
}

func legacyYAMLScalar(text string) (string, error) {
	switch {
	case text == "" || text == "~" || strings.ContainsAny(text[:1], "{[&*!|>%@`#"):
		return "", errors.New("not a plain string")
	case text[0] == '"':
		end := strings.LastIndexByte(text, '"')
		if end <= 0 {
			return "", errors.New("unterminated string")
		}
		if rest := strings.TrimSpace(text[end+1:]); rest != "" && !strings.HasPrefix(rest, "#") {
			return "", errors.New("unexpected content after the string")
		}
		return strconv.Unquote(text[:end+1])
	case text[0] == '\'':
		end := strings.LastIndexByte(text, '\'')
		if end <= 0 {
			return "", errors.New("unterminated string")
		}
		if rest := strings.TrimSpace(text[end+1:]); rest != "" && !strings.HasPrefix(rest, "#") {
			return "", errors.New("unexpected content after the string")
		}
		return strings.ReplaceAll(text[1:end], "''", "'"), nil
	}
	if comment := strings.Index(text, " #"); comment >= 0 {
		text = strings.TrimSpace(text[:comment])
	}
	if strings.Contains(text, ": ") {
		return "", errors.New("ambiguous plain scalar")
	}
	return text, nil
}

func legacyDotfilesOrigin(url string) bool {
	url = strings.TrimSuffix(strings.TrimSuffix(strings.TrimSpace(url), "/"), ".git")
	return slices.ContainsFunc([]string{"https://github.com/luisgui1757/dotfiles", "git@github.com:luisgui1757/dotfiles", "ssh://git@github.com/luisgui1757/dotfiles"},
		func(known string) bool { return strings.EqualFold(url, known) })
}

func (d *legacyRetirementDriver) samePath(a, b string) bool {
	if d.Platform.OS == "windows" {
		return strings.EqualFold(a, b)
	}
	return a == b
}

// Git inspects without hooks, optional locks or personal configuration.
func (d *legacyRetirementDriver) gitInspect(ctx context.Context, directory string, arguments ...string) (string, error) {
	if d.Platform.OS != "windows" {
		output, err := d.git(ctx, directory, arguments...)
		return strings.TrimSpace(string(output)), err
	}
	program := d.Environment.Programs["git"]
	if program == "" {
		return "", errors.New("git is required to identify the checkout")
	}
	values := append([]string{"-c", "core.fsmonitor=false", "-c", "core.untrackedCache=false", "-c", "credential.helper=", "-C", directory}, arguments...)
	output, err := d.Query(ctx, nativeCommand{Program: program, Arguments: values,
		Environment: []string{"GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null", "GIT_OPTIONAL_LOCKS=0", "GIT_TERMINAL_PROMPT=0", "LC_ALL=C"}})
	return strings.TrimSpace(string(output)), err
}

// A dotfiles checkout is the top-level of a Git worktree whose own
// configuration names the dotfiles repository as origin, used as source either
// through its released .chezmoiroot or directly through its home directory.
// Its metadata is read as data, so recognition outlives any retired Git.
func dotfilesCheckout(source string) (string, error) {
	top := source
	if _, err := os.Lstat(filepath.Join(source, ".git")); errors.Is(err, os.ErrNotExist) && filepath.Base(source) == "home" {
		top = filepath.Dir(source)
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	origin, err := legacyGitOrigin(top)
	if err != nil {
		return "", fmt.Errorf("it is neither a readable Git checkout nor its home directory: %w", err)
	}
	if !legacyDotfilesOrigin(origin) {
		return "", fmt.Errorf("its origin %q is not the dotfiles repository", truncateLegacyText(origin, 256))
	}
	if top != source {
		return top, nil
	}
	// Git for Windows may check the released LF file out with CRLF.
	data, _, err := readLegacyFile(filepath.Join(top, ".chezmoiroot"), 4096)
	if err == nil && (string(data) == "home\n" || string(data) == "home\r\n") {
		return top, nil
	}
	return "", errors.Join(errors.New("its top-level has no released .chezmoiroot"), err)
}

// inspectChezmoiSource blocks retirement while chezmoi has an unknown source.
// A recognized dotfiles source stays in place; only chezmoi's own
// configuration, state and cache move aside.
func (d *legacyRetirementDriver) inspectChezmoiSource(_ context.Context, scope *legacyRetirementScope, config string) error {
	source, origin, err := d.chezmoiSourceDirectory(config)
	if err != nil {
		scope.Blocked = legacyReason("chezmoi's source cannot be determined from its configuration as data; review that chezmoi user before retirement", err)
		return nil
	}
	info, err := os.Stat(source)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil || !info.IsDir() {
		scope.Blocked = legacyReason("chezmoi source "+source+" ("+origin+") cannot be inspected; chezmoi was kept", err)
		return nil
	}
	if entries, err := os.ReadDir(source); err != nil || len(entries) == 0 {
		return err
	}
	resolved, err := resolveConfigPath(source)
	if err != nil {
		return err
	}
	description := "this dotfiles checkout"
	if !d.samePath(resolved, d.Repository) && !d.samePath(resolved, filepath.Join(d.Repository, "home")) {
		top, err := dotfilesCheckout(resolved)
		if err != nil {
			scope.Blocked = legacyReason("another chezmoi source exists at "+source+" ("+origin+"); chezmoi is still in use and was kept", err)
			return nil
		}
		description = "the dotfiles checkout " + top
	}
	for _, moved := range []string{config, filepath.Join(d.Environment.Cache, "chezmoi")} {
		if relative, err := filepath.Rel(moved, resolved); err == nil && (relative == "." || filepath.IsLocal(relative)) {
			scope.Blocked = "chezmoi's source " + source + " lies inside " + moved + ", which retirement would move; review it manually"
			return nil
		}
	}
	scope.Detail = append(scope.Detail, "chezmoi's source "+source+" is "+description+"; it stays in place.")
	return nil
}

// Gate values for the delta vocabulary. Every value is a fresh observation.

var legacyDeltaGateKinds = []string{"brew-package", "brew-consumers", "brew-tap", "mac-app", "scoop-app", "scoop-consumers", "winget-package", "choco-package", "choco-consumers",
	"windows-processes", "user-path", "user-path-entry", "path-absent", "tree", "query-git", "dpkg-package", "dpkg-consumers", "local-account", "native-login-shell", "brew-login-shells"}

// State gates name the approved original of what a step changes.
var legacyDeltaStateGates = []string{"brew-package", "brew-tap", "scoop-app", "winget-package", "choco-package", "user-path", "user-path-entry", "tree", "dpkg-package", "local-account"}

func (d *legacyRetirementDriver) deltaGateValue(ctx context.Context, gate legacyRetirementGate) (string, error) {
	switch gate.Kind {
	case "brew-package", "brew-consumers":
		inventory, err := d.brewDeltaInventory(ctx, true)
		if err != nil {
			return "", err
		}
		if gate.Kind == "brew-consumers" {
			return strings.Join(legacyBrewConsumers(inventory, gate.Subject, nil), ","), nil
		}
		if pkg, ok := inventory[gate.Subject]; ok {
			return legacyBrewIdentity(pkg), nil
		}
		return "absent", nil
	case "brew-tap":
		if d.Platform.Homebrew == nil {
			return "absent", nil
		}
		taps, err := d.brewTaps(ctx)
		if err != nil || !slices.ContainsFunc(taps, func(tap string) bool { return strings.EqualFold(tap, gate.Subject) }) {
			return "absent", err
		}
		return "present", nil
	case "mac-app":
		return d.macApplicationState(ctx, gate.Subject)
	case "scoop-app", "scoop-consumers":
		return d.scoopGate(gate.Subject, gate.Kind == "scoop-consumers")
	case "winget-package":
		return d.wingetGate(ctx, gate.Subject)
	case "choco-package", "choco-consumers":
		return d.chocoGate(gate.Subject, gate.Kind == "choco-consumers")
	case "windows-processes":
		return d.windowsProcesses(ctx, gate.Subject)
	case "user-path":
		return d.userPathEntry(ctx, gate.Subject)
	case "user-path-entry":
		return d.queryUserPathEntry(ctx, gate.Subject, legacyUserPathEntryQuery)
	case "path-absent":
		if legacyAbsent(gate.Subject) {
			return "absent", nil
		}
		return "present", nil
	case "tree":
		snapshot, err := snapshotTree(gate.Subject, maxConfigBytes, maxConfigEntries)
		if err != nil || snapshot.Kind == "absent" {
			return "absent", err
		}
		return snapshot.Hash, nil
	case "query-git":
		return d.queryGitGate(gate.Subject)
	case "dpkg-package":
		identity, _, err := d.debIdentity(ctx, gate.Subject)
		return identity, err
	case "dpkg-consumers":
		consumers, err := d.debConsumers(ctx, gate.Subject)
		return strings.Join(consumers, ","), err
	case "local-account":
		return d.localAccount(ctx, gate.Subject)
	case "native-login-shell":
		identity, problem, err := d.nativeLoginShell(ctx, gate.Subject)
		if problem != "" {
			return problem, err
		}
		return identity, err
	case "brew-login-shells":
		return d.brewZshLoginGate(ctx, gate.Subject)
	}
	return "", fmt.Errorf("unknown legacy gate %q", gate.Kind)
}
