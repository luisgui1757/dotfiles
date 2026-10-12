package installer

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
)

// The released Linux installer made Linuxbrew's zsh the login shell:
// set_default_shell_zsh registered `command -v zsh`, <prefix>/bin/zsh, in
// /etc/shells and ran chsh to it. Removing the formula strands every login that
// reaches it. Before Homebrew removes zsh, this account's login shell changes to
// a healthy registered native shell: setup's APT zsh when it is selected,
// otherwise the system bash. Nothing is installed to make that possible.
// Another account that still reaches the formula keeps it installed; other
// accounts are never changed. A login stranded by an earlier removal is
// repaired the same way, from the native or released Linuxbrew prefix.

// legacyBrewZsh names one Linuxbrew prefix's zsh formula, both as configured
// and with existing links resolved.
type legacyBrewZsh struct {
	Prefix, Cellar    string
	prefixes, cellars []string
}

func newLegacyBrewZsh(prefix, cellar string) (legacyBrewZsh, error) {
	z := legacyBrewZsh{Prefix: prefix, Cellar: cellar, prefixes: []string{prefix}, cellars: []string{cellar}}
	for _, list := range []*[]string{&z.prefixes, &z.cellars} {
		resolved, err := filepath.EvalSymlinks((*list)[0])
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return z, err
		}
		*list = sortedUnique(append(*list, resolved))
	}
	return z, nil
}

// login is the released login shell: `command -v zsh` after brew shellenv.
func (z legacyBrewZsh) login() string { return filepath.Join(z.Prefix, "bin", "zsh") }

// owns reports whether a path belongs to the formula: its keg, its opt and
// linked-keg links, or a command it links into the prefix.
func (z legacyBrewZsh) owns(path string) bool {
	within := func(root string) bool { return path == root || strings.HasPrefix(path, root+"/") }
	for _, prefix := range z.prefixes {
		command := filepath.Join(prefix, "bin", "zsh")
		if path == command || strings.HasPrefix(path, command+"-") || within(filepath.Join(prefix, "opt", "zsh")) || within(filepath.Join(prefix, "var", "homebrew", "linked", "zsh")) {
			return true
		}
	}
	return slices.ContainsFunc(z.cellars, func(cellar string) bool { return within(filepath.Join(cellar, "zsh")) })
}

func legacyUnresolvable(err error) bool {
	return errors.Is(err, os.ErrNotExist) || errors.Is(err, syscall.ENOTDIR)
}

// legacyShellGone reports a login shell that can no longer start because its
// path resolves to nothing.
func legacyShellGone(shell string) (bool, error) {
	_, err := os.Stat(shell)
	if err == nil {
		return false, nil
	}
	if legacyUnresolvable(err) {
		return true, nil
	}
	return false, err
}

// reaches follows a login shell through each link, resolving parent
// directories, so a link into the formula counts even after the formula is
// gone. Only a path that cannot exist ends the walk; any other error is an
// inspection failure, never evidence that the shell is safe.
func (z legacyBrewZsh) reaches(shell string) (bool, error) {
	if !filepath.IsAbs(shell) {
		return false, nil
	}
	current := filepath.Clean(shell)
	for range 40 {
		if z.owns(current) {
			return true, nil
		}
		parent, err := filepath.EvalSymlinks(filepath.Dir(current))
		if legacyUnresolvable(err) {
			return false, nil
		}
		if err != nil {
			return false, fmt.Errorf("inspect login shell %s: %w", shell, err)
		}
		current = filepath.Join(parent, filepath.Base(current))
		if z.owns(current) {
			return true, nil
		}
		info, err := os.Lstat(current)
		if legacyUnresolvable(err) {
			return false, nil
		}
		if err != nil {
			return false, fmt.Errorf("inspect login shell %s: %w", shell, err)
		}
		if info.Mode()&os.ModeSymlink == 0 {
			return false, nil
		}
		target, err := os.Readlink(current)
		if err != nil {
			return false, fmt.Errorf("inspect login shell %s: %w", shell, err)
		}
		if !filepath.IsAbs(target) {
			target = filepath.Join(filepath.Dir(current), target)
		}
		current = filepath.Clean(target)
	}
	return false, fmt.Errorf("login shell %s has too many links to inspect", shell)
}

// linuxAccountRecords enumerates the account database in its own order. A name
// can repeat across services and every record is kept. A malformed record
// leaves the inventory incomplete, so it is an inspection failure.
func (d *legacyRetirementDriver) linuxAccountRecords(ctx context.Context) ([][]string, error) {
	output, err := d.Query(ctx, d.systemCommand(false, d.sys("/usr/bin/getent"), "passwd"))
	if err != nil {
		return nil, err
	}
	records := [][]string{}
	for _, line := range strings.Split(strings.TrimSuffix(string(output), "\n"), "\n") {
		fields := strings.Split(line, ":")
		if len(fields) != 7 || fields[0] == "" {
			return nil, errors.New("the account database returned an incomplete record")
		}
		records = append(records, fields)
	}
	return records, nil
}

// localAccount reads an account through NSS's files service alone: the local
// passwd record that chsh changes. A directory-service account has none.
func (d *legacyRetirementDriver) localAccount(ctx context.Context, name string) (string, error) {
	output, err := d.Query(ctx, d.systemCommand(false, d.sys("/usr/bin/getent"), "-s", "files", "passwd", name))
	if code, ok := legacyExitCode(err); ok && code == 2 {
		return "absent", nil
	}
	if err != nil {
		return "", err
	}
	return strings.TrimSuffix(string(output), "\n"), nil
}

// brewZshLogins lists the account records whose login shell reaches the formula.
func (d *legacyRetirementDriver) brewZshLogins(ctx context.Context, z legacyBrewZsh) ([][]string, error) {
	records, err := d.linuxAccountRecords(ctx)
	if err != nil {
		return nil, err
	}
	reached, logins := map[string]bool{}, [][]string{}
	for _, fields := range records {
		value, seen := reached[fields[6]]
		if !seen {
			if value, err = z.reaches(fields[6]); err != nil {
				return nil, err
			}
			reached[fields[6]] = value
		}
		if value {
			logins = append(logins, fields)
		}
	}
	return logins, nil
}

// brewZshLoginGate names every account whose login still reaches the zsh
// formula under a Linuxbrew prefix. Removal requires none.
func (d *legacyRetirementDriver) brewZshLoginGate(ctx context.Context, prefix string) (string, error) {
	cellar := filepath.Join(prefix, "Cellar")
	if location := d.Platform.Homebrew; location != nil && location.Prefix == prefix {
		cellar = location.Cellar
	}
	z, err := newLegacyBrewZsh(prefix, cellar)
	if err != nil {
		return "", err
	}
	logins, err := d.brewZshLogins(ctx, z)
	if err != nil {
		return "", err
	}
	names := []string{}
	for _, fields := range logins {
		names = append(names, fields[0])
	}
	return strings.Join(sortedUnique(names), ","), nil
}

// nativeLoginShell describes a replacement login shell: the executable it
// resolves to, its exact /etc/shells registration and the version it reports
// when started. A non-empty problem means it cannot serve as a login shell.
func (d *legacyRetirementDriver) nativeLoginShell(ctx context.Context, shell string) (string, string, error) {
	resolved, err := filepath.EvalSymlinks(shell)
	if legacyUnresolvable(err) {
		return "", "is not installed", nil
	}
	if err != nil {
		return "", "", err
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return "", "", err
	}
	if !info.Mode().IsRegular() || info.Mode()&0111 == 0 {
		return "", "is not an executable file", nil
	}
	listed, err := d.shellListed(shell)
	if err != nil {
		return "", "", err
	}
	if !listed {
		return "", "is not registered in /etc/shells", nil
	}
	identity, err := legacyPathIdentity(resolved)
	if err != nil {
		return "", "", err
	}
	output, err := d.Query(ctx, d.systemCommand(false, shell, "--version"))
	if ctxErr := ctx.Err(); ctxErr != nil {
		return "", "", ctxErr
	}
	if err != nil {
		return "", "did not start: " + truncateLegacyText(err.Error(), 256), nil
	}
	banner, _, _ := strings.Cut(string(output), "\n")
	expected := map[string]string{"zsh": "zsh ", "bash": "GNU bash, version "}[filepath.Base(shell)]
	if expected == "" || !strings.HasPrefix(banner, expected) || !legacyText(banner) {
		return "", "did not report itself as " + filepath.Base(shell), nil
	}
	return resolved + "=" + identity + " " + banner, "", nil
}

// targetAccount is this account's NSS record, the one its login uses.
func (d *legacyRetirementDriver) targetAccount(ctx context.Context) ([]string, error) {
	record, err := d.accountRecord(ctx, d.Environment.Username)
	if err != nil {
		return nil, err
	}
	fields := strings.Split(record, ":")
	if len(fields) != 7 || fields[0] != d.Environment.Username {
		return nil, errors.New("account lookup did not identify exactly the target user")
	}
	return fields, nil
}

// planLoginSwitch changes this account's login shell from the released
// Linuxbrew zsh to a healthy registered native shell. The account records, the
// original shell and the replacement are rechecked immediately before chsh.
// A non-empty reason explains why no change is safe; nothing was planned.
func (d *legacyRetirementDriver) planLoginSwitch(ctx context.Context, scope *legacyRetirementScope, z legacyBrewZsh, fields []string, when string) (string, error) {
	name, record := fields[0], strings.Join(fields, ":")
	shell := "this account's login shell " + fields[6]
	if fields[6] != z.login() {
		return shell + " is not the earlier setup's " + z.login() + " but reaches Linuxbrew's zsh; change it with chsh", nil
	}
	local, err := d.localAccount(ctx, name)
	if err != nil {
		return "", err
	}
	if local != record {
		return shell + " is not in the local account file that chsh changes; ask your directory administrator to change it", nil
	}
	replacement, source := d.sys("/bin/bash"), "zsh is not selected in setup, so the system bash"
	selected, err := d.replacementSelected("tool.zsh")
	if err != nil {
		return "", err
	}
	if selected {
		replacement, source = d.sys("/usr/bin/zsh"), "setup's zsh"
	}
	identity, problem, err := d.nativeLoginShell(ctx, replacement)
	if err != nil {
		return "", err
	}
	if problem != "" {
		return shell + " cannot change to " + replacement + ": it " + problem, nil
	}
	chsh, err := d.program("/usr/bin/chsh")
	if err != nil {
		return "", err
	}
	command := d.systemCommand(true, chsh, "-s", replacement, name)
	changed := strings.Join(append(fields[:6:6], replacement), ":")
	scope.Steps = append(scope.Steps, legacyRetirementStep{Kind: "command", Label: "Change this account's login shell from " + fields[6] + " to " + replacement, Privileged: true, Command: &command,
		Gates: []legacyRetirementGate{{Kind: "account", Subject: name, Expected: record}, {Kind: "local-account", Subject: name, Expected: record},
			{Kind: "native-login-shell", Subject: replacement, Expected: identity}},
		Checks: []legacyRetirementGate{{Kind: "account", Subject: name, Expected: changed}, {Kind: "local-account", Subject: name, Expected: changed}}})
	scope.Detail = append(scope.Detail, "Your login shell is "+fields[6]+"; "+source+" ("+replacement+") becomes your login shell "+when+". Open a new login session afterwards. Other accounts are never changed.")
	return "", nil
}

// planBrewZshRemoval adds the formula's uninstall after this account's login
// switch, or keeps the formula while any login would be stranded.
func (d *legacyRetirementDriver) planBrewZshRemoval(ctx context.Context, scope *legacyRetirementScope, location *HomebrewLocation, uninstall legacyRetirementStep) (bool, error) {
	z, err := newLegacyBrewZsh(location.Prefix, location.Cellar)
	if err != nil {
		return false, err
	}
	account, err := d.targetAccount(ctx)
	if err != nil {
		return false, err
	}
	logins, err := d.brewZshLogins(ctx, z)
	if err != nil {
		return false, err
	}
	others, own := []string{}, false
	for _, fields := range logins {
		if fields[0] == account[0] && fields[6] == account[6] {
			own = true
		} else {
			others = append(others, "account "+fields[0]+" ("+fields[6]+")")
		}
	}
	if len(others) != 0 {
		scope.Retained = append(scope.Retained, "Homebrew formula zsh is the login shell of "+strings.Join(sortedUnique(others), ", ")+"; other accounts are never changed, so it was preserved")
		return false, nil
	}
	if own {
		reason, err := d.planLoginSwitch(ctx, scope, z, account, "before Homebrew removes zsh")
		if err != nil {
			return false, err
		}
		if reason != "" {
			scope.Retained = append(scope.Retained, reason+"; Homebrew formula zsh was preserved")
			return false, nil
		}
	}
	// Every login is rechecked immediately before Homebrew runs.
	uninstall.Gates = append(uninstall.Gates, legacyRetirementGate{Kind: "brew-login-shells", Subject: z.Prefix})
	scope.Steps = append(scope.Steps, uninstall)
	return true, d.planBrewZshRegistration(ctx, scope, z)
}

// planBrewZshRepair handles a formula that is already gone: this account's
// stranded login changes to a native shell, other stranded logins are
// disclosed, and the released registration goes once no login names it.
func (d *legacyRetirementDriver) planBrewZshRepair(ctx context.Context, scope *legacyRetirementScope) error {
	account, err := d.targetAccount(ctx)
	if err != nil {
		return err
	}
	locations := [][2]string{}
	if location := d.Platform.Homebrew; location != nil {
		locations = append(locations, [2]string{location.Prefix, location.Cellar})
	} else {
		// The prefixes the released installer could use, after Homebrew itself left.
		for _, prefix := range []string{d.sys("/home/linuxbrew/.linuxbrew"), filepath.Join(d.Folders.Home, ".linuxbrew")} {
			locations = append(locations, [2]string{prefix, filepath.Join(prefix, "Cellar")})
		}
	}
	for _, location := range locations {
		z, err := newLegacyBrewZsh(location[0], location[1])
		if err != nil {
			return err
		}
		if _, err := os.Lstat(filepath.Join(z.Cellar, "zsh")); err == nil {
			continue // An installed formula still serves its logins.
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
		reaches, err := z.reaches(account[6])
		if err != nil {
			return err
		}
		stranded := false
		if reaches {
			if stranded, err = legacyShellGone(account[6]); err != nil {
				return err
			}
		}
		registered, err := d.shellListed(z.login())
		if err != nil {
			return err
		}
		if !stranded && !registered {
			continue
		}
		logins, err := d.brewZshLogins(ctx, z)
		if err != nil {
			return err
		}
		inUse := reaches && !stranded
		for _, fields := range logins {
			if fields[0] == account[0] && fields[6] == account[6] {
				continue
			}
			inUse = true
			if gone, err := legacyShellGone(fields[6]); err != nil {
				return err
			} else if gone {
				scope.Retained = append(scope.Retained, "account "+fields[0]+"'s login shell "+fields[6]+" no longer exists since Linuxbrew's zsh was removed; other accounts are never changed")
			}
		}
		if stranded {
			reason, err := d.planLoginSwitch(ctx, scope, z, account, "in place of the removed formula")
			if err != nil {
				return err
			}
			if reason != "" {
				scope.Retained = append(scope.Retained, "Linuxbrew's zsh was removed, but "+reason)
				inUse = true
			}
		}
		if inUse {
			continue
		}
		gone, err := legacyShellGone(z.login())
		if err != nil {
			return err
		}
		if gone {
			if err := d.planBrewZshRegistration(ctx, scope, z); err != nil {
				return err
			}
		}
	}
	return nil
}

// planBrewZshRegistration removes the released /etc/shells line once no login
// can use it, keeping every other line and the original beside the file.
func (d *legacyRetirementDriver) planBrewZshRegistration(ctx context.Context, scope *legacyRetirementScope, z legacyBrewZsh) error {
	path := d.sys("/etc/shells")
	data, _, err := readLegacyFile(path, maxProfileBytes)
	if err != nil || data == nil {
		return err
	}
	count := 0
	for _, line := range strings.Split(string(data), "\n") {
		if line == z.login() {
			count++
		}
	}
	switch {
	case count == 0:
		return nil
	case count > 1:
		scope.Retained = append(scope.Retained, fmt.Sprintf("%s lists the retired %s %d times; review it manually, so it was preserved", path, z.login(), count))
		return nil
	}
	before := len(scope.Steps)
	if err := d.addLineRemoval(ctx, scope, "Unregister the retired Linuxbrew zsh", path, z.login()); err != nil || len(scope.Steps) == before {
		return err
	}
	step := &scope.Steps[len(scope.Steps)-1]
	step.Gates = append(step.Gates, legacyRetirementGate{Kind: "brew-login-shells", Subject: z.Prefix})
	return nil
}
