package installer

import (
	"embed"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

//go:embed shells/*
var shellSources embed.FS

// Shell ownership is fixed: one resource attaches the loader to personal
// profiles. The graph selects fragments in the combined private initialization,
// never through the continued existence of edited files or installed commands.
func configureShellProfiles(d *ProfileDriver, target Context, folders ConfigFolders, archives *ArchiveDriver, starshipConfig string, selected []string, nativeDirectories ...string) error {
	root := filepath.Join(d.Directory, "shells")
	features := map[string]string{}
	selectedFeature := func(id string) bool { return slices.Contains(selected, id) }
	placeholder := func(id string) string { return "# @dotfiles-feature:" + id + "@" }
	source := func(name string) (string, error) {
		data, err := shellSources.ReadFile("shells/" + name)
		return string(data), err
	}
	add := func(id, script string) { features[id] = script }
	binDirs := []string{filepath.Join(folders.Home, ".local", "bin")}
	binDirs = append(binDirs, nativeDirectories...)
	ids := make([]string, 0, len(archives.Pins))
	for id := range archives.Pins {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	for _, id := range ids {
		dirs, err := archives.BinaryDirectories(id)
		if err != nil {
			return err
		}
		binDirs = append(binDirs, dirs...)
	}
	if target.OS == "windows" {
		if !filepath.IsAbs(folders.Documents) {
			return errors.New("shell integration needs the actual Documents known folder")
		}
		quoted := make([]string, len(binDirs))
		for i, directory := range binDirs {
			if strings.Contains(directory, ";") {
				return errors.New("shell command directory cannot contain the PATH separator")
			}
			quoted[i] = powershellLiteral(directory)
		}
		interactive, err := source("interactive.ps1")
		if err != nil {
			return err
		}
		cache, err := source("cache.ps1")
		if err != nil {
			return err
		}
		agentEnvironment := ""
		if selectedFeature("tool.claude") {
			agentEnvironment = "$env:DISABLE_UPDATES = '1'\n"
		}
		if selectedFeature("tool.copilot") {
			agentEnvironment += "$env:COPILOT_AUTO_UPDATE = 'false'\n"
		}
		loader := agentEnvironment + "$dotfilesPaths = @(" + strings.Join(quoted, ", ") + ")\n" + `foreach ($directory in $dotfilesPaths) {
	if ((Test-Path -LiteralPath $directory -PathType Container) -and
		$directory -notin ($env:PATH -split [regex]::Escape([IO.Path]::PathSeparator))) {
		$env:PATH = $directory + [IO.Path]::PathSeparator + $env:PATH
	}
}
` + interactive + "\nif ($global:DotfilesShellReady) { return }\n" + cache
		for _, id := range []string{"config.powershell", "integration.starship", "integration.fzf", "integration.lsd", "integration.zoxide"} {
			if selectedFeature(id) {
				loader += placeholder(id) + "\n"
			}
		}
		loader += "$global:DotfilesShellReady = $true\n"
		loaderPath := filepath.Join(root, "init.ps1")
		d.Targets["integration.shells"] = []ProfileTarget{{loaderPath, loader}}
		for _, directory := range []string{"PowerShell", "WindowsPowerShell"} {
			d.Targets["integration.shells"] = append(d.Targets["integration.shells"], ProfileTarget{filepath.Join(folders.Documents, directory, "profile.ps1"), "if (Test-Path -LiteralPath " + powershellLiteral(loaderPath) + " -PathType Leaf) { . " + powershellLiteral(loaderPath) + " }"})
		}
		for id, file := range map[string]string{"config.powershell": "powershell-core.ps1", "integration.lsd": "lsd.ps1", "integration.fzf": "fzf.ps1"} {
			script, err := source(file)
			if err != nil {
				return err
			}
			if id == "integration.fzf" {
				module, err := archives.RequiredFilePath("powershell.psfzf", "PSFzf.psd1")
				if err != nil {
					return err
				}
				script = "$script:DotfilesPSFzfPath = " + powershellLiteral(module) + "\n" + script
			}
			add(id, script)
		}
	} else {
		zshRoot := folders.Zsh
		if zshRoot == "" {
			zshRoot = folders.Home
		}
		if !filepath.IsAbs(zshRoot) {
			return errors.New("ZDOTDIR must be absolute")
		}
		// Login shells can reorder an inherited PATH (notably macOS path_helper).
		// Restore managed priority while preserving other entries, including empty
		// components, without accumulating duplicates across nested shells.
		environment := `_dotfiles_prepend_path() {
	_dotfiles_rest=${PATH-}
	PATH=$1
	[ -n "$_dotfiles_rest" ] || return 0
	while :; do
		case $_dotfiles_rest in
			*:*) _dotfiles_entry=${_dotfiles_rest%%:*}; _dotfiles_rest=${_dotfiles_rest#*:}; _dotfiles_last=0 ;;
			*) _dotfiles_entry=$_dotfiles_rest; _dotfiles_last=1 ;;
		esac
		if [ "$_dotfiles_entry" != "$1" ]; then PATH=$PATH:$_dotfiles_entry; fi
		[ "$_dotfiles_last" = 0 ] || break
	done
}
`
		for _, directory := range binDirs {
			if strings.Contains(directory, ":") {
				return errors.New("shell command directory cannot contain the PATH separator")
			}
			environment += "_dotfiles_directory=" + shellLiteral(directory) + "\n" + `if [ -d "$_dotfiles_directory" ]; then
	_dotfiles_prepend_path "$_dotfiles_directory"
fi
`
		}
		environment += "export PATH\nunset -f _dotfiles_prepend_path\nunset _dotfiles_directory _dotfiles_rest _dotfiles_entry _dotfiles_last\n"
		if selectedFeature("tool.claude") {
			environment += "export DISABLE_UPDATES=1\n"
		}
		// Copilot ignores its autoUpdate setting; only this variable stops it
		// replacing or re-executing over the pinned binary.
		if selectedFeature("tool.copilot") {
			environment += "export COPILOT_AUTO_UPDATE=false\n"
		}
		init := "case $- in *i*) ;; *) return 0 ;; esac\n[ \"${__DOTFILES_SHELL_READY-}\" != 1 ] || return 0\n"
		// Hand off before any other initialization, so bash does no work that
		// zsh repeats. The zsh is the one setup observes on each OS.
		zsh := "/usr/bin/zsh"
		if target.OS == "darwin" {
			zsh = "/bin/zsh"
		}
		if selectedFeature("integration.zsh-handoff") {
			init += zshHandoff(zsh)
		}
		if selectedFeature("config.zsh") || selectedFeature("integration.lsd") {
			palette, err := source("ls-colors.sh")
			if err != nil {
				return err
			}
			init += palette + "\n"
		}
		if selectedFeature("config.zsh") {
			init += "if [ -n \"${ZSH_VERSION-}\" ]; then\n" + placeholder("config.zsh") + "\nfi\n"
		}
		for _, id := range []string{"integration.starship", "integration.fzf", "integration.lsd", "integration.zoxide"} {
			if selectedFeature(id) {
				init += placeholder(id) + "\n"
			}
		}
		if selectedFeature("config.zsh") {
			local := shellLiteral(filepath.Join(folders.Home, ".zshrc.local"))
			init += "if [ -n \"${ZSH_VERSION-}\" ] && [ -r " + local + " ]; then . " + local + "; fi\n"
		}
		if selectedFeature("integration.zsh-handoff") {
			init += zshHandoffGhostty
		}
		init += "__DOTFILES_SHELL_READY=1\n"
		envPath, initPath := filepath.Join(root, "env.sh"), filepath.Join(root, "init.sh")
		attachEnv := "if [ -r " + shellLiteral(envPath) + " ]; then . " + shellLiteral(envPath) + "; fi\n"
		attachInit := attachEnv + "if [ -r " + shellLiteral(initPath) + " ]; then . " + shellLiteral(initPath) + "; fi\n"
		login, err := bashLoginPath(folders.Home)
		if err != nil {
			return err
		}
		localLogin := filepath.Join(folders.Home, ".zprofile.local")
		d.Targets["integration.shells"] = []ProfileTarget{
			{envPath, environment}, {initPath, init},
			{filepath.Join(zshRoot, ".zshenv"), ":"},
			{filepath.Join(zshRoot, ".zprofile"), attachEnv + "if [ -r " + shellLiteral(localLogin) + " ]; then . " + shellLiteral(localLogin) + "; fi"},
			{filepath.Join(zshRoot, ".zshrc"), attachInit},
			{filepath.Join(folders.Home, ".bashrc"), attachInit}, {login, attachInit},
		}
		if selectedFeature("config.zsh") {
			d.Targets["integration.shells"][2].Script = "skip_global_compinit=1"
		}

		core, err := source("zsh-core.zsh")
		if err != nil {
			return err
		}
		fzfTab, err := archives.RequiredFilePath("plugin.fzf-tab", "fzf-tab.plugin.zsh")
		if err != nil {
			return err
		}
		autosuggestions, err := archives.RequiredFilePath("plugin.zsh-autosuggestions", "zsh-autosuggestions.zsh")
		if err != nil {
			return err
		}
		add("config.zsh", "__DOTFILES_FZF_TAB="+shellLiteral(fzfTab)+"\n__DOTFILES_AUTOSUGGESTIONS="+shellLiteral(autosuggestions)+"\n"+core)
		add("integration.lsd", "if command -v lsd >/dev/null 2>&1; then\n    alias ls='lsd' l='lsd -l' la='lsd -a' lla='lsd -la' lt='lsd --tree'\nfi")
		if _, available := archives.Pins["tool.fzf"]; available {
			fzf, err := archives.CommandPath("tool.fzf", "fzf")
			if err != nil {
				return err
			}
			add("integration.fzf", "if [ -n \"${ZSH_VERSION-}\" ]; then\n    source <("+shellLiteral(fzf)+" --zsh)\n    if whence -w fzf-tab-complete >/dev/null 2>&1; then\n        bindkey '^I' fzf-tab-complete\n        bindkey -M vicmd '^I' fzf-tab-complete\n    fi\nelif [ -n \"${BASH_VERSION-}\" ]; then\n    source <("+shellLiteral(fzf)+" --bash)\nfi")
		}
	}
	for _, id := range []string{"starship", "zoxide"} {
		pin, ok := archives.Pins["tool."+id]
		if !ok {
			continue
		}
		command, err := archives.CommandPath("tool."+id, id)
		if err != nil {
			return err
		}
		prefix := ""
		if target.OS == "windows" {
			if id == "starship" {
				prefix = "$env:STARSHIP_CONFIG = " + powershellLiteral(starshipConfig) + "\n"
			}
			arguments := "@('init', 'powershell')"
			if id == "starship" {
				arguments = "@('init', 'powershell', '--print-full-init')"
			}
			cache := filepath.Join(d.Directory, "shell-cache", id+"-"+pin.SHA256+".ps1")
			add("integration."+id, prefix+". (Get-DotfilesInitScript -Command "+powershellLiteral(command)+" -Arguments "+arguments+" -CachePath "+powershellLiteral(cache)+")")
		} else {
			if id == "starship" {
				prefix = "export STARSHIP_CONFIG=" + shellLiteral(starshipConfig) + "\n"
			}
			add("integration."+id, prefix+"if [ -n \"${ZSH_VERSION-}\" ]; then\n    eval \"$("+shellLiteral(command)+" init zsh)\"\nelif [ -n \"${BASH_VERSION-}\" ]; then\n    eval \"$("+shellLiteral(command)+" init bash)\"\nfi")
		}
	}
	for i := range d.Targets["integration.shells"] {
		entry := &d.Targets["integration.shells"][i]
		for _, id := range []string{"config.zsh", "config.powershell", "integration.starship", "integration.fzf", "integration.lsd", "integration.zoxide"} {
			if strings.Contains(entry.Script, placeholder(id)) {
				script, ok := features[id]
				if !ok {
					return errors.New("selected shell feature has no native recipe: " + id)
				}
				entry.Script = strings.ReplaceAll(entry.Script, placeholder(id), script)
			}
		}
	}
	for id, targets := range d.Targets {
		for i := range targets {
			bound, err := bindConfigDestination(targets[i].Path)
			if err != nil {
				return err
			}
			targets[i].Path = bound
		}
		d.Targets[id] = targets
	}
	return nil
}

// A directory account's login shell can be bash with no way to change it
// (owner report 2026-10-23). Interactive terminal bash hands over to zsh with
// SHELL set, so Herdr, tmux and Neovim terminals, which start $SHELL, get zsh
// too. Scripts, bash -c, an explicit bash inside zsh (the exported marker) and an
// account whose shell is already zsh stay as they are. A failed exec leaves the
// interactive bash running with its original SHELL: bash 3.2 exits on a failed
// exec while reading startup files, so execfail is set for the exec only.
func zshHandoff(zsh string) string {
	z := shellLiteral(zsh)
	return `if [ -n "${BASH_VERSION-}" ] && [ -z "${DOTFILES_ZSH_HANDOFF-}" ] && [ -z "${BASH_EXECUTION_STRING-}" ] && [ -t 0 ] && [ -t 1 ] && [ -x ` + z + ` ]; then
	case ${SHELL-} in
	*/zsh) ;;
	*)
		__dotfiles_had_shell=${SHELL+1}
		__dotfiles_shell=${SHELL-}
		if shopt -q execfail; then __dotfiles_execfail=; else __dotfiles_execfail=1; shopt -s execfail; fi
		export DOTFILES_ZSH_HANDOFF=1 SHELL=` + z + `
		if shopt -q login_shell; then exec ` + z + ` -l; else exec ` + z + `; fi
		if [ -n "$__dotfiles_execfail" ]; then shopt -u execfail; fi
		if [ -n "$__dotfiles_had_shell" ]; then SHELL=$__dotfiles_shell; else unset SHELL; fi
		unset DOTFILES_ZSH_HANDOFF __dotfiles_had_shell __dotfiles_shell __dotfiles_execfail
		;;
	esac
fi
`
}

// Ghostty injects its integration only into the shell it starts, here bash.
// Its zsh script documents sourcing after `exec zsh` and guards against a second
// load. Only the zsh that received the handoff loads it, as on a zsh login shell;
// the marker then stays set so a later bash is not handed over again.
const zshHandoffGhostty = `if [ -n "${ZSH_VERSION-}" ] && [ "${DOTFILES_ZSH_HANDOFF-}" = 1 ]; then
	export DOTFILES_ZSH_HANDOFF=2
	if [ -n "${GHOSTTY_RESOURCES_DIR-}" ] && [ -r "$GHOSTTY_RESOURCES_DIR/shell-integration/zsh/ghostty-integration" ]; then
		. "$GHOSTTY_RESOURCES_DIR/shell-integration/zsh/ghostty-integration"
	fi
fi
`

func bashLoginPath(home string) (string, error) {
	for _, name := range []string{".bash_profile", ".bash_login", ".profile"} {
		candidate := filepath.Join(home, name)
		if _, err := os.Lstat(candidate); err == nil {
			return candidate, nil
		} else if !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
	}
	return filepath.Join(home, ".bash_profile"), nil
}

// Bash chooses the first existing login profile, but a newly created personal
// profile cannot relocate our already-owned block. The schema-1 baseline holds
// the chosen path; its complete receipt identity still authorizes every target.
func retainBashLoginTarget(d *ProfileDriver, home string, receipt Receipt) error {
	if receipt.Ownership == "" || receipt.Ownership == "reused" {
		return nil
	}
	const id = "integration.shells"
	if receipt.Before.Provider != "profile-block" || !validProfileRecovery(receipt.Recovery) {
		return errors.New("shell profile receipt has invalid baseline identity")
	}
	data, err := readDocument(d.baselinePath(receipt.Recovery))
	if errors.Is(err, os.ErrNotExist) && receipt.Ownership == "uncertain" {
		// A crash can precede the first baseline write. Existing recovery may
		// proceed only when the original approved targets and blocks still match.
		observed, inspectErr := d.inspect(id)
		if inspectErr == nil && sameArtifact(receipt.Before, observed) {
			return nil
		}
	}
	if err != nil {
		return d.journalError(id, receipt, err)
	}
	var baseline profileBaseline
	if err := Decode(data, &baseline); err != nil {
		return d.journalError(id, receipt, err)
	}
	allowed := []string{}
	for _, name := range []string{".bash_profile", ".bash_login", ".profile"} {
		path, err := bindConfigDestination(filepath.Join(home, name))
		if err != nil {
			return err
		}
		allowed = append(allowed, path)
	}
	targets := d.Targets[id]
	index := -1
	for i, target := range targets {
		if slices.Contains(allowed, target.Path) {
			if index != -1 {
				return errors.New("shell profile recipe has multiple Bash login targets")
			}
			index = i
		}
	}
	if index < 0 || len(baseline.Entries) != len(targets) || !slices.Contains(allowed, baseline.Entries[index].Path) {
		return d.journalError(id, receipt, errors.New("shell baseline has an invalid Bash login target"))
	}
	targets[index].Path = baseline.Entries[index].Path
	_, err = d.readBaseline(id, receipt)
	if err != nil {
		return d.journalError(id, receipt, err)
	}
	return nil
}
