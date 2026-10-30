package installer

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// obsidianPortable is Linux Obsidian's official portable build.
const obsidianPortable = "tool.obsidian-portable"

// DesktopDriver publishes only the fixed application integrations below. Files,
// adoption, interruption and restoration use the ordinary configuration journal.
// Archives and generated inputs live below Directory, never in a user's config.
type DesktopDriver struct {
	Target     Context
	Folders    ConfigFolders
	Directory  string
	Archives   *ArchiveDriver
	PowerShell string
	Run        func(context.Context, nativeCommand) ([]byte, error)
}

type desktopRecipe struct {
	Package, Name, Folder, Destination string
	Source, Command, Icon              string
	Directory                          bool
	Contents                           string
}

func integrationConfiguration(directory string, target Context, folders ConfigFolders, r Resource, targets []ConfigTarget) (*ConfigDriver, Resource, error) {
	r.Action, r.Platforms, r.Requires, r.PlatformRequires, r.Bindings = "config", []string{target.OS}, nil, nil, nil
	c := &Catalog{Schema: 1, Resources: []Resource{r}}
	for i := range targets {
		targets[i].Resource, targets[i].Platforms = r.ID, []string{target.OS}
	}
	d := &ConfigDriver{Catalog: c, Manifest: ConfigManifest{Schema: 1, Targets: targets}, Target: target, Folders: folders, Repository: directory, Directory: directory}
	return d, r, d.Manifest.Validate(c)
}

func integrationRelative(directory, source string) (string, error) {
	rel, err := filepath.Rel(directory, source)
	if err != nil || !canonicalRelative(filepath.ToSlash(rel)) {
		return "", errors.New("integration input must be inside the private state directory")
	}
	return filepath.ToSlash(rel), nil
}

func (d *DesktopDriver) recipe(id string) (desktopRecipe, error) {
	var p desktopRecipe
	switch id {
	case "desktop.vscode":
		p = desktopRecipe{Package: "tool.vscode", Name: "Visual Studio Code"}
	case "integration.ghostty", "desktop.ghostty":
		p = desktopRecipe{Package: "tool.ghostty", Name: "Ghostty"}
	case "desktop.aerospace":
		p = desktopRecipe{Package: "tool.aerospace", Name: "AeroSpace"}
	case "desktop.windows-terminal":
		p = desktopRecipe{Package: "tool.windows-terminal", Name: "Windows Terminal"}
	case "desktop.rose-pine":
		p = desktopRecipe{Package: "vscode.rose-pine", Name: "Rose Pine"}
	case "desktop.openwhispr":
		p = desktopRecipe{Package: "tool.openwhispr", Name: "OpenWhispr"}
	case "desktop.obsidian":
		p = desktopRecipe{Package: "tool.obsidian", Name: "Obsidian"}
		if d.Target.OS == "linux" {
			p.Package = obsidianPortable
		}
	case "desktop.zen":
		p = desktopRecipe{Package: "tool.zen", Name: "Zen"}
	case "desktop.xremap":
		p = desktopRecipe{Package: "tool.xremap", Name: "Mac keyboard shortcuts"}
	default:
		return p, errors.New("unknown fixed desktop integration")
	}
	if d.Archives == nil {
		return p, errors.New("desktop integration requires the private archive provider")
	}
	required := func(name string) (string, error) { return d.Archives.RequiredFilePath(p.Package, name) }
	command := func(name string) (string, error) { return d.Archives.CommandPath(p.Package, name) }
	var err error
	if p.Package == "vscode.rose-pine" {
		entry, e := required("extension/package.json")
		if e != nil {
			return p, e
		}
		p.Source, p.Folder, p.Destination, p.Directory = filepath.Dir(entry), d.Folders.Home, ".vscode/extensions/dotfiles.rose-pine", true
		return p, nil
	}
	switch d.Target.OS {
	case "darwin":
		if p.Package == "tool.windows-terminal" {
			return p, errors.New("Windows Terminal requires Windows")
		}
		entry, e := required(p.Name + ".app/Contents/Info.plist")
		if e != nil {
			return p, e
		}
		p.Source, p.Folder, p.Destination, p.Directory = filepath.Dir(filepath.Dir(entry)), d.Folders.Home, "Applications/"+p.Name+".app", true
	case "linux":
		if p.Package == "tool.xremap" {
			return d.xremapAutostart(p)
		}
		switch p.Package {
		case "tool.ghostty":
			p.Command, err = command("ghostty")
			if err == nil {
				p.Icon, err = required("usr/share/icons/hicolor/256x256/apps/com.mitchellh.ghostty.png")
			}
		case "tool.openwhispr":
			p.Command, err = command("open-whispr")
			if err == nil {
				p.Icon, err = required("resources/src/assets/icon.png")
			}
		case obsidianPortable:
			p.Command, err = required("obsidian")
			if err == nil {
				p.Icon, err = required("resources/icon.png")
			}
		case "tool.zen":
			p.Command, err = command("zen")
			if err == nil {
				p.Icon, err = required("browser/chrome/icons/default/default128.png")
			}
		default:
			return p, errors.New("desktop application is unavailable on Linux")
		}
		if err != nil {
			return p, err
		}
		p.Folder = d.Folders.Data
		if p.Folder == "" {
			p.Folder = filepath.Join(d.Folders.Home, ".local", "share")
		}
		// Obsidian keeps the launcher file of its former Flatpak build, so an
		// update replaces that owned launcher in place.
		p.Destination = "applications/dotfiles-" + strings.TrimSuffix(strings.TrimPrefix(p.Package, "tool."), "-portable") + ".desktop"
		// GNOME groups windows under a launcher by its StartupWMClass when the
		// file name differs from the window class: Ghostty's GTK application ID,
		// Zen's remoting name and Obsidian's desktop name, as in their upstream
		// desktop entries. OpenWhispr sets its Electron desktop name to
		// open-whispr.desktop, which Electron uses for WM_CLASS.
		class := map[string]string{"tool.ghostty": "com.mitchellh.ghostty", "tool.zen": "zen", obsidianPortable: "md.obsidian.Obsidian", "tool.openwhispr": "open-whispr"}[p.Package]
		p.Contents, err = desktopEntry(p.Name, p.Command, p.Icon, class, p.Package == "tool.zen")
		if p.Package == obsidianPortable && err == nil {
			// Chromium's sandbox needs unprivileged user namespaces, which Ubuntu
			// restricts unless an AppArmor profile names the binary's path.
			line := "\nExec=" + desktopExecArgument(p.Command) + "\n"
			p.Contents = strings.Replace(p.Contents, line, strings.TrimSuffix(line, "\n")+" --no-sandbox %U\n", 1)
			p.Contents = strings.Replace(p.Contents, "Categories=Development;", "Categories=Office;\nMimeType=x-scheme-handler/obsidian;", 1)
		}
	case "windows":
		switch p.Package {
		case "tool.vscode":
			p.Command, err = required("Code.exe")
		case "tool.windows-terminal":
			p.Command, err = required("WindowsTerminal.exe")
		default:
			return p, errors.New("desktop application is unavailable on Windows")
		}
		if err != nil {
			return p, err
		}
		p.Icon, p.Folder, p.Destination = p.Command, d.Folders.Programs, "Dotfiles "+p.Name+".lnk"
	default:
		return p, errors.New("unsupported desktop platform")
	}
	return p, err
}

// xremap runs in the graphical session from an XDG autostart entry, so its
// window rules see that session. Its automatic desktop choice probes X display
// :0, which is not the session display on every system: name the backend from
// the session type. GNOME Wayland reports applications only through xremap's
// Shell extension; without it no rule matches and keys stay plain.
// --watch=config follows edits of the file and its replacement in the file's
// own directory. The configuration is a live link into the checkout, where an
// update (git) unlinks the file and creates a new one; the link's directory never
// changes. Resolving the link at login makes xremap watch the checkout file, so a
// pulled keymap reloads without a new login.
func (d *DesktopDriver) xremapAutostart(p desktopRecipe) (desktopRecipe, error) {
	command, err := d.Archives.CommandPath(p.Package, "xremap")
	if err != nil {
		return p, err
	}
	p.Folder = d.Folders.Config
	if p.Folder == "" {
		p.Folder = filepath.Join(d.Folders.Home, ".config")
	}
	config := filepath.Join(p.Folder, "xremap", "config.yml")
	if !filepath.IsAbs(command) || !filepath.IsAbs(config) || strings.ContainsAny(command+config, "\x00\r\n") {
		return p, errors.New("invalid xremap autostart path")
	}
	const script = `case "$XDG_SESSION_TYPE" in wayland) desktop=gnome ;; *) desktop=x11 ;; esac; config=$(readlink -f -- "$1") || config=$1; exec "$0" --desktop "$desktop" --watch=config,device --no-window-logging "$config"`
	exec := []string{}
	for _, argument := range []string{"/bin/sh", "-c", script, command, config} {
		exec = append(exec, desktopExecArgument(argument))
	}
	p.Command, p.Destination = command, "autostart/dotfiles-mac-keyboard.desktop"
	p.Contents = "[Desktop Entry]\nType=Application\nName=" + p.Name + " (dotfiles)\nExec=" + strings.Join(exec, " ") +
		"\nTerminal=false\nNoDisplay=true\nX-GNOME-Autostart-enabled=true\nX-GNOME-AutoRestart=true\n"
	return p, nil
}

func desktopExecArgument(argument string) string {
	arg := strings.NewReplacer("\\", "\\\\", "\"", "\\\"", "`", "\\`", "$", "\\$", "%", "%%").Replace(argument)
	return strings.ReplaceAll("\""+arg+"\"", "\\", "\\\\")
}

// Desktop Entry specification: first quote the Exec argument, then escape the
// value for the key/value file. Percent is a field code even inside quotes.
// GLib checks that the program exists before it expands %%, so GIO cannot start
// a program whose path contains a literal percent (hosted run, 2026-10-22).
func desktopEntry(name, command, icon, class string, browser bool) (string, error) {
	if !filepath.IsAbs(command) || !filepath.IsAbs(icon) || strings.ContainsAny(command+icon+name+class, "\x00\r\n") || strings.ContainsRune(command, '=') {
		return "", errors.New("invalid desktop entry path")
	}
	value := func(s string) string { return strings.ReplaceAll(s, "\\", "\\\\") }
	exec, categories, extra := desktopExecArgument(command), "Development;", ""
	if class != "" {
		extra = "StartupWMClass=" + class + "\n"
	}
	if browser {
		exec += " %u"
		categories = "Network;WebBrowser;"
		extra += "MimeType=text/html;x-scheme-handler/http;x-scheme-handler/https;\n"
	}
	return "[Desktop Entry]\nType=Application\nName=" + name + "\nExec=" + exec + "\nIcon=" + value(icon) + "\nTerminal=false\nCategories=" + categories + "\n" + extra, nil
}

func (d *DesktopDriver) configuration(r Resource, receipt Receipt) (*ConfigDriver, Resource, desktopRecipe, string, error) {
	p, err := d.recipe(r.ID)
	if err != nil {
		return nil, r, p, "", err
	}
	pin, err := d.Archives.validate(p.Package)
	if err != nil {
		return nil, r, p, "", err
	}
	desired, err := digest(struct {
		Recipe desktopRecipe
		Pin    ArchivePin
	}{p, pin})
	if err != nil {
		return nil, r, p, "", err
	}
	if p.Source == "" {
		ext := ".desktop"
		if d.Target.OS == "windows" {
			ext = ".lnk"
		}
		p.Source = filepath.Join(d.Directory, "desktop", "inputs", desired+ext)
	}
	source, err := integrationRelative(d.Directory, p.Source)
	if err != nil {
		return nil, r, p, "", err
	}
	mode := ""
	if !p.Directory {
		mode = "copy"
	}
	if p.Directory && d.Target.OS == "windows" {
		mode = "junction"
	}
	folders := d.Folders
	folders.Home = p.Folder
	c, cr, err := integrationConfiguration(d.Directory, d.Target, folders, r, []ConfigTarget{{Source: source, Folder: "home", Path: p.Destination, Directory: p.Directory, Mode: mode}})
	if err == nil && d.Target.OS == "windows" && !p.Directory {
		if err = c.validate(cr, receipt); err != nil {
			return nil, r, p, "", err
		}
		j, journalErr := c.readJournal(cr, receipt)
		if journalErr == nil {
			// COM shortcut bytes need not be deterministic. The existing journal,
			// including legacy recipe-only inputs, owns the exact prepared source.
			if len(j.Entries) != 1 || !d.shortcutInput(j.Entries[0].State.Target.Source) {
				return nil, r, p, "", errors.New("saved shortcut input is outside private preparation storage")
			}
			p.Source = j.Entries[0].State.Target.Source
			c.Manifest.Targets[0].Source, err = integrationRelative(d.Directory, p.Source)
		} else if !errors.Is(journalErr, errConfigJournalAbsent) {
			err = journalErr
		}
	}
	return c, cr, p, desired, err
}

func (d *DesktopDriver) shortcutInput(path string) bool {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return false
	}
	rel, err := filepath.Rel(filepath.Join(d.Directory, "desktop", "inputs"), path)
	if err != nil {
		return false
	}
	parts := strings.Split(filepath.ToSlash(rel), "/")
	return len(parts) == 1 && strings.HasSuffix(parts[0], ".lnk") && operationID.MatchString(strings.TrimSuffix(parts[0], ".lnk")) ||
		len(parts) == 2 && operationID.MatchString(parts[0]) && parts[1] == "shortcut.lnk"
}

func (d *DesktopDriver) Observe(ctx context.Context, r Resource, receipt Receipt) (Observation, error) {
	c, cr, _, desired, err := d.configuration(r, receipt)
	if err != nil {
		return Observation{}, err
	}
	o, err := c.Observe(ctx, cr, receipt)
	o.Desired = desired
	// Inputs are prepared only after approval/dependency installation.
	if !o.Unknown {
		o.ApplyBlocked = ""
	}
	return o, err
}

func (d *DesktopDriver) Apply(ctx context.Context, r Resource, op Operation, receipt Receipt) (Observation, error) {
	c, cr, p, desired, err := d.configuration(r, receipt)
	if err != nil {
		return Observation{}, err
	}
	current, err := d.Observe(ctx, r, receipt)
	if err != nil || !sameArtifact(current, op.Observed) || op.Observed.Desired != desired {
		return current, errors.Join(errors.New("desktop integration changed after approval"), err)
	}
	archive, err := d.Archives.Observe(ctx, Resource{ID: p.Package}, Receipt{})
	if err != nil || !archive.Healthy {
		return current, errors.Join(errors.New("desktop integration requires its verified private application"), err)
	}
	if !p.Directory {
		if d.Target.OS == "windows" {
			if _, err := c.readJournal(cr, receipt); !errors.Is(err, errConfigJournalAbsent) {
				return current, errors.Join(errors.New("configuration operation already exists; explicitly resume it"), err)
			}
			p.Source, err = d.prepareShortcut(ctx, p, desired, receipt.OperationID)
			if err == nil {
				c.Manifest.Targets[0].Source, err = integrationRelative(d.Directory, p.Source)
			}
		} else {
			err = writeIntegrationInput(p.Source, []byte(p.Contents))
		}
		if err != nil {
			return current, err
		}
	}
	_, actual, err := inspectConfiguration(c.Catalog, c.Manifest, c.Target, c.Folders, c.Repository, cr.ID)
	if err != nil {
		return current, err
	}
	op.Observed.Desired = actual.Desired
	if _, err = c.Apply(ctx, cr, op, receipt); err != nil {
		return current, err
	}
	return d.Observe(ctx, r, receipt)
}

func writeIntegrationInput(path string, data []byte) error {
	bound, err := bindConfigDestination(path)
	if err != nil || bound != path {
		return errors.Join(errors.New("private integration input parent was redirected"), err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if errors.Is(err, os.ErrExist) {
		current, e := os.ReadFile(path)
		if e != nil || string(current) != string(data) {
			return errors.Join(errors.New("private integration input differs from the fixed recipe"), e)
		}
		return nil
	}
	if err != nil {
		return err
	}
	_, err = f.Write(data)
	return errors.Join(err, f.Close())
}

func (d *DesktopDriver) Remove(ctx context.Context, r Resource, receipt Receipt) (Observation, error) {
	c, cr, _, _, err := d.configuration(r, receipt)
	if err != nil {
		return Observation{}, err
	}
	if _, err = c.Remove(ctx, cr, receipt); err != nil {
		return Observation{}, err
	}
	return d.Observe(ctx, r, receipt)
}
func (d *DesktopDriver) ResumeResource(ctx context.Context, r Resource, op Operation, receipt Receipt) (Observation, error) {
	c, cr, _, _, err := d.configuration(r, receipt)
	if err != nil {
		return Observation{}, err
	}
	current, err := d.Observe(ctx, r, receipt)
	if err != nil || current.Desired != op.Observed.Desired {
		return current, errors.Join(errors.New("desktop recipe changed during saved operation"), err)
	}
	// The configuration journal continues binding the actual prepared input bytes.
	if _, err = c.ResumeResource(ctx, cr, op, receipt); err != nil {
		return current, err
	}
	return d.Observe(ctx, r, receipt)
}
func (d *DesktopDriver) ObserveRestore(ctx context.Context, id string, receipt Receipt) (Observation, error) {
	return (&ConfigDriver{Directory: d.Directory}).ObserveRestore(ctx, id, receipt)
}
func (d *DesktopDriver) RestoreResource(ctx context.Context, id string, o Observation, receipt Receipt) error {
	return (&ConfigDriver{Directory: d.Directory}).RestoreResource(ctx, id, o, receipt)
}
func (d *DesktopDriver) FinishTransaction(ctx context.Context, plan Plan, receipts map[string]Receipt) (map[string][]string, error) {
	return (&ConfigDriver{Directory: d.Directory}).FinishTransaction(ctx, plan, receipts)
}

func (d *DesktopDriver) prepareShortcut(ctx context.Context, p desktopRecipe, identity, operation string) (string, error) {
	if d.Run == nil || !filepath.IsAbs(d.PowerShell) || !operationID.MatchString(operation) {
		return "", errors.New("shortcut preparation requires a native session and saved operation")
	}
	// Each attempt has isolated files and native evidence. A lost/failed reply may
	// leave its private directory intact, but cannot poison a later preparation.
	payload, err := digest(struct{ Kind, Recipe, Operation, Attempt string }{"desktop-shortcut", identity, operation, rand.Text()})
	if err != nil {
		return "", err
	}
	directory := filepath.Join(d.Directory, "desktop", "inputs", payload)
	bound, err := bindConfigDestination(directory)
	if err != nil || bound != directory {
		return "", errors.Join(errors.New("shortcut input parent was redirected"), err)
	}
	if err := os.MkdirAll(filepath.Dir(directory), 0700); err != nil {
		return "", err
	}
	if err := os.Mkdir(directory, 0700); err != nil {
		return "", err
	}
	stage, source := filepath.Join(directory, "staged.lnk"), filepath.Join(directory, "shortcut.lnk")
	script := `$ErrorActionPreference='Stop'
$ProgressPreference='SilentlyContinue'
$destination=` + desktopPSQuote(stage) + `
$target=` + desktopPSQuote(p.Command) + `
$icon=` + desktopPSQuote(p.Icon) + `
if (Test-Path -LiteralPath $destination) { throw 'Private shortcut input already exists without this preparation result; preserve it for inspection' }
$link=(New-Object -ComObject WScript.Shell).CreateShortcut($destination)
$link.TargetPath=$target; $link.Arguments=''; $link.WorkingDirectory=[IO.Path]::GetDirectoryName($target); $link.IconLocation=$icon+',0'; $link.Save()
(Get-FileHash -LiteralPath $destination -Algorithm SHA256).Hash.ToLowerInvariant()`
	output, err := d.Run(ctx, nativeCommand{Operation: payload, Program: d.PowerShell, Arguments: windowsVendorArguments(script)})
	if err != nil {
		return "", fmt.Errorf("prepare private Start menu shortcut: %w: %s", err, nativeErrorDetail(err, output))
	}
	data, err := readDocument(stage)
	if err != nil {
		return "", err
	}
	if fmt.Sprintf("%x", sha256.Sum256(data)) != strings.TrimSpace(string(output)) {
		return "", fmt.Errorf("shortcut bytes differ from native preparation result (native output: %d bytes, PowerShell CLIXML: %t)", len(output), strings.Contains(string(output), "#< CLIXML"))
	}
	// Copy the verified bytes into a flushed exclusive file before atomic
	// publication; never publish a COM-owned partial or follow its replacement.
	ready := filepath.Join(directory, "ready.lnk")
	file, err := os.OpenFile(ready, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return "", err
	}
	_, writeErr := file.Write(data)
	if err := errors.Join(writeErr, file.Sync(), file.Close()); err != nil {
		return "", err
	}
	if err := moveConfigExclusive(ready, source); err != nil {
		return "", err
	}
	return source, nil
}

func desktopPSQuote(value string) string { return "'" + strings.ReplaceAll(value, "'", "''") + "'" }
