package installer

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// The released helper installed upstream Nix 2.34.0 with profile edits
// disabled: a systemd daemon on Linux hosts with systemd, single-user Nix on
// other Linux hosts, and a daemon on its own APFS volume on macOS. Upstream has
// no uninstaller; these recipes follow the 2.34 manual for exactly the
// artifacts its 2.34.0 installer scripts create. Anything else is blocked.
// https://nix.dev/manual/nix/2.34/installation/uninstall.html
const legacyNixVersion = "nix (Nix) 2.34.0"

const (
	legacyNixDeterminate  = "determinate"
	legacyNixLinuxDaemon  = "linux-daemon"
	legacyNixLinuxSingle  = "linux-single"
	legacyNixDarwinDaemon = "darwin-daemon"
)

var legacyBuildUser = regexp.MustCompile(`^(_?)nixbld([1-9][0-9]?)$`)

// macOS creates these on a mounted APFS volume.
var legacyDarwinVolumeMetadata = []string{".fseventsd", ".Spotlight-V100", ".Trashes", ".TemporaryItems", ".DocumentRevisions-V100"}

// Released Home Manager packages and their replacement resources. Removing Nix
// removes these commands; unselected replacements are disclosed.
var legacyHomeManagerPackages = []struct{ Name, Replacement string }{
	{"ripgrep", "tool.rg"}, {"fd", "tool.fd"}, {"fzf", "tool.fzf"}, {"jq", "tool.jq"}, {"lazygit", "tool.lazygit"},
	{"Node.js 24", "tool.node"}, {"starship", "tool.starship"}, {"zoxide", "tool.zoxide"},
}

func legacySystemProgram(candidates ...string) (string, error) {
	for _, candidate := range candidates {
		info, err := os.Stat(candidate)
		if err == nil && info.Mode().IsRegular() {
			return candidate, nil
		}
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
	}
	return "", fmt.Errorf("required system program %s is unavailable", candidates[0])
}

// sys names a fixed native system path. Hermetic fixtures relocate the system
// root; production uses the host root unchanged. Values read from the system,
// such as link targets and account records, are host paths compared with sys.
func (d *legacyRetirementDriver) sys(path string) string {
	if d.systemRoot == "" {
		return path
	}
	return filepath.Join(d.systemRoot, path)
}

func (d *legacyRetirementDriver) program(candidates ...string) (string, error) {
	rooted := []string{}
	for _, candidate := range candidates {
		rooted = append(rooted, d.sys(candidate))
	}
	return legacySystemProgram(rooted...)
}

func (d *legacyRetirementDriver) stateRoot() string {
	return filepath.Dir(filepath.Dir(d.Normal.StatePath))
}

func (d *legacyRetirementDriver) dataRoot() string {
	if d.Folders.Data != "" {
		return d.Folders.Data
	}
	return filepath.Join(d.Folders.Home, ".local", "share")
}

func (d *legacyRetirementDriver) inspectNix(ctx context.Context, scope *legacyRetirementScope) error {
	if d.Platform.OS != "darwin" && d.Platform.OS != "linux" {
		return nil
	}
	entries, err := readPlainDirectory(d.sys("/nix"))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	darwinSystem := false
	if d.Platform.OS == "darwin" {
		// A dangling activation link still marks nix-darwin; without it, the
		// system profile can still identify the active generation.
		if _, err := os.Lstat(d.sys("/run/current-system")); err == nil {
			darwinSystem = true
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		} else if system, err := d.nixDarwinSystem(); err != nil {
			return err
		} else {
			darwinSystem = system != ""
		}
	}
	if len(entries) == 0 {
		// No store remains. Retire only leftover integrations that cannot run.
		if darwinSystem {
			scope.Blocked = "/run/current-system remains without a Nix store; inspect the earlier nix-darwin activation manually"
			return nil
		}
		if err := d.planNixLeftovers(ctx, scope, err == nil); err != nil || scope.Blocked != "" {
			return err
		}
		return d.planNixUserFiles(scope)
	}
	if dependents, err := d.legacyGitRuntimeDependents(d.sys("/nix")); err != nil {
		return err
	} else if len(dependents) != 0 {
		scope.Blocked = "Nix provides the only Git retirement can use to prove the released checkouts " + strings.Join(dependents, ", ") + "; retire the replaced legacy tools first or select Git in setup"
		return nil
	}
	if d.Platform.OS == "linux" {
		// A partly deleted root cannot prove its released layout; the saved
		// operation that started its deletion is the evidence instead.
		if recovered, err := d.planNixDeletionRecovery(ctx, scope); err != nil || recovered {
			if err != nil || scope.Blocked != "" {
				return err
			}
			return d.planNixUserFiles(scope)
		}
	}
	kind, reason, err := d.nixInstallKind(ctx, entries)
	if err != nil {
		return err
	}
	if reason != "" {
		scope.Blocked = reason
		return nil
	}
	consumers, err := d.nixConsumers(ctx, false, "")
	if err != nil {
		return err
	}
	if consumers != "" {
		scope.Blocked = consumers + "; remove or migrate those consumers, or keep Nix"
		return nil
	}
	if hints, err := d.nixProcessHints(ctx); err != nil {
		return err
	} else if hints != "" {
		scope.Notices = append(scope.Notices, "Processes still use Nix ("+hints+"). Close them and open a new login session before the irreversible store removal; it is rechecked with administrator rights immediately beforehand.")
	}
	// Order: login shell, system integrations, Home Manager links discovered
	// from the store, the runtime, then user links that vendor tools may need.
	steps := []func(context.Context, *legacyRetirementScope) error{d.planLoginShell}
	if darwinSystem {
		steps = append(steps, d.planNixDarwin)
	}
	steps = append(steps, d.planHomeManager)
	switch kind {
	case legacyNixDeterminate:
		steps = append(steps, d.planDeterminateRetirement)
	case legacyNixLinuxDaemon:
		steps = append(steps, func(ctx context.Context, scope *legacyRetirementScope) error {
			return d.planUpstreamLinux(ctx, scope, true)
		})
	case legacyNixLinuxSingle:
		steps = append(steps, func(ctx context.Context, scope *legacyRetirementScope) error {
			return d.planUpstreamLinux(ctx, scope, false)
		})
	case legacyNixDarwinDaemon:
		steps = append(steps, d.planUpstreamDarwin)
	}
	for _, plan := range steps {
		if err := plan(ctx, scope); err != nil || scope.Blocked != "" {
			return err
		}
	}
	if err := d.planNixUserFiles(scope); err != nil {
		return err
	}
	return d.discloseNixRemoval(scope)
}

func (d *legacyRetirementDriver) discloseNixRemoval(scope *legacyRetirementScope) error {
	packages := slices.Clone(legacyHomeManagerPackages)
	if d.Platform.OS == "linux" {
		packages = append(packages, struct{ Name, Replacement string }{"clangd", "tool.clangd"})
	}
	for _, item := range packages {
		if err := d.disclosePrerequisite(scope, item.Replacement, item.Name+" from the Nix package layer is not selected in setup; it disappears with Nix."); err != nil {
			return err
		}
	}
	scope.Detail = append(scope.Detail, "The Nix store is deleted permanently; this cannot be undone from configuration backups.")
	return nil
}

func (d *legacyRetirementDriver) nixInstallKind(ctx context.Context, entries []os.DirEntry) (string, string, error) {
	names := []string{}
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	allowed := []string{"store", "var"}
	if slices.Contains(names, "receipt.json") {
		for _, name := range names {
			if legacyDeterminateRootEntry(d.Platform.OS, name) {
				allowed = append(allowed, name)
			}
		}
	}
	if d.Platform.OS == "darwin" {
		allowed = append(allowed, legacyDarwinVolumeMetadata...)
	}
	for _, name := range names {
		if !slices.Contains(allowed, name) {
			return "", "unrecognized data in the Nix installation: " + filepath.Join(d.sys("/nix"), name) + "; preserve and review it", nil
		}
	}
	if slices.Contains(names, "receipt.json") {
		return legacyNixDeterminate, "", nil
	}
	if d.Platform.OS == "darwin" {
		return legacyNixDarwinDaemon, "", nil
	}
	uid, _, _, err := legacyFileOwner(d.sys("/nix"))
	if err != nil {
		return "", "", err
	}
	group, err := d.groupRecord(ctx, "nixbld")
	if err != nil {
		return "", "", err
	}
	_, unitErr := os.Lstat(d.sys("/etc/systemd/system/nix-daemon.service"))
	if unitErr != nil && !errors.Is(unitErr, os.ErrNotExist) {
		return "", "", unitErr
	}
	switch {
	case uid == 0 && group != "absent" && unitErr == nil:
		return legacyNixLinuxDaemon, "", nil
	case strconv.FormatUint(uint64(uid), 10) == d.Environment.UID && group == "absent" && errors.Is(unitErr, os.ErrNotExist):
		return legacyNixLinuxSingle, "", nil
	}
	return "", "the Nix installation is neither the released systemd daemon nor the released single-user layout; inspect it natively", nil
}

// Leftovers after the store is gone are inert. They are the same integrations
// an approved deletion retires after the store, planned from what remains.
// Linux may keep an empty /nix.
func (d *legacyRetirementDriver) planNixLeftovers(ctx context.Context, scope *legacyRetirementScope, emptyRoot bool) error {
	before := len(scope.Steps)
	if err := d.planDeterminateLeftovers(ctx, scope); err != nil || scope.Blocked != "" {
		return err
	}
	vendorRemainder := len(scope.Steps) != before
	if vendorRemainder && d.Platform.OS == "darwin" {
		return nil // The receipt owns its own boot, service and account recipe.
	}
	if d.Platform.OS == "darwin" {
		return d.planDarwinNixIntegrations(ctx, scope, false)
	}
	if emptyRoot {
		scope.Steps = append(scope.Steps, legacyRetirementStep{Kind: "rmdir", Label: "Remove the empty /nix directory", Privileged: true, Path: d.sys("/nix"), Absent: []string{d.sys("/nix")}})
	}
	if vendorRemainder {
		return nil // Vendor units are regular files, unlike upstream's links.
	}
	return d.planLinuxNixIntegrations(ctx, scope)
}

// User state is moved aside last, after vendor and system steps that may
// still resolve it. Personal Nix configuration is disclosed and kept.
func (d *legacyRetirementDriver) planNixUserFiles(scope *legacyRetirementScope) error {
	home := d.Folders.Home
	for _, path := range []string{filepath.Join(home, ".nix-profile"), filepath.Join(home, ".nix-defexpr"), filepath.Join(home, ".nix-channels"),
		filepath.Join(d.stateRoot(), "nix"), filepath.Join(d.stateRoot(), "home-manager"), filepath.Join(d.dataRoot(), "nix"), filepath.Join(d.Environment.Cache, "nix")} {
		if err := d.addUserMove(scope, "Move retired Nix user state aside", path); err != nil {
			return err
		}
	}
	config := d.Folders.Config
	if config == "" {
		config = filepath.Join(home, ".config")
	}
	for _, path := range []string{filepath.Join(config, "nix"), filepath.Join(config, "home-manager")} {
		if _, err := os.Lstat(path); err == nil {
			scope.Detail = append(scope.Detail, "Personal configuration at "+path+" stays unchanged.")
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return nil
}

// Consumers are other accounts' profiles, roots and login shells, unrecognized
// garbage-collection roots, additional mounts and, with administrator rights,
// running processes. Subject "daemon" excludes only the Nix daemon itself
// before the step that stops it.
func (d *legacyRetirementDriver) nixConsumers(ctx context.Context, privileged bool, subject string) (string, error) {
	problems := []string{}
	for _, directory := range []string{d.sys("/nix/var/nix/profiles/per-user"), d.sys("/nix/var/nix/gcroots/per-user")} {
		entries, err := readPlainDirectory(directory)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return "", err
		}
		if len(entries) > 1024 {
			return "", errors.New("Nix account inventory exceeds its review bound")
		}
		for _, entry := range entries {
			if entry.Name() == d.Environment.Username {
				continue
			}
			path := filepath.Join(directory, entry.Name())
			children, err := readPlainDirectory(path)
			if err != nil {
				return "", err
			}
			if len(children) == 0 {
				continue
			}
			if entry.Name() == "root" && directory == d.sys("/nix/var/nix/profiles/per-user") {
				installed, err := d.nixRootDefaultProfile(path, children, false)
				if err != nil {
					return "", err
				}
				if !installed {
					installed, err = d.nixApprovedProfileRemainder(ctx, path, children)
					if err != nil {
						return "", err
					}
				}
				if installed {
					continue
				}
			}
			problems = append(problems, "account "+entry.Name()+" has Nix profiles or roots in "+directory)
		}
	}
	shells, err := d.nixAccountShells(ctx)
	if err != nil {
		return "", err
	}
	problems = append(problems, shells...)
	roots, err := d.nixOutsideRoots(ctx)
	if err != nil {
		return "", err
	}
	problems = append(problems, roots...)
	if d.Platform.OS == "linux" {
		mounts, err := os.ReadFile(d.sys("/proc/self/mountinfo"))
		if err != nil {
			return "", err
		}
		for _, line := range strings.Split(string(mounts), "\n") {
			fields := strings.Fields(line)
			if len(fields) < 5 {
				continue
			}
			point := legacyMountPath(fields[4])
			if point == d.sys("/nix") || strings.HasPrefix(point, d.sys("/nix")+"/") {
				problems = append(problems, "a filesystem is mounted at "+point+"; Nix removal never deletes mounted data")
			}
		}
	}
	if privileged {
		processes, err := d.nixProcesses(ctx, subject)
		if err != nil {
			return "", err
		}
		problems = append(problems, processes...)
	}
	return strings.Join(problems, "; "), nil
}

// accountShells reads every local account's login shell from the account
// database. Callers decide which shells their retirement would strand.
func (d *legacyRetirementDriver) accountShells(ctx context.Context) (map[string]string, error) {
	if d.Platform.OS == "darwin" {
		return d.dsclList(ctx, "Users", "UserShell")
	}
	output, err := d.Query(ctx, d.systemCommand(false, d.sys("/usr/bin/getent"), "passwd"))
	if err != nil {
		return nil, err
	}
	shells := map[string]string{}
	for _, line := range strings.Split(strings.TrimSuffix(string(output), "\n"), "\n") {
		fields := strings.Split(line, ":")
		if len(fields) != 7 || fields[0] == "" {
			return nil, errors.New("account inventory contains an invalid passwd record")
		}
		if _, duplicate := shells[fields[0]]; duplicate {
			return nil, fmt.Errorf("account inventory repeats %q", fields[0])
		}
		shells[fields[0]] = fields[6]
	}
	return shells, nil
}

// Another account whose login shell comes from Nix would be locked out by its
// removal. Retirement only changes this account's shell.
func (d *legacyRetirementDriver) nixAccountShells(ctx context.Context) ([]string, error) {
	shells, err := d.accountShells(ctx)
	if err != nil {
		return nil, err
	}
	problems := []string{}
	for _, name := range slices.Sorted(maps.Keys(shells)) {
		if name == d.Environment.Username {
			continue
		}
		nix, err := d.nixShell(shells[name])
		if err != nil {
			return nil, err
		}
		if nix {
			problems = append(problems, "account "+name+" uses the Nix login shell "+shells[name])
		}
	}
	return problems, nil
}

func legacyMountPath(escaped string) string {
	var result strings.Builder
	for i := 0; i < len(escaped); i++ {
		if escaped[i] == '\\' && i+3 < len(escaped) {
			if value, err := strconv.ParseUint(escaped[i+1:i+4], 8, 8); err == nil {
				result.WriteByte(byte(value))
				i += 3
				continue
			}
		}
		result.WriteByte(escaped[i])
	}
	return result.String()
}

// Known roots are this account's profiles, Home Manager generation links and
// nix-darwin's current system. Missing targets are stale and hold nothing.
func (d *legacyRetirementDriver) nixOutsideRoots(ctx context.Context) ([]string, error) {
	problems := []string{}
	root := d.sys("/nix/var/nix/gcroots")
	entries, err := readPlainDirectory(root)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		switch entry.Name() {
		case "auto", "per-user":
			continue
		case "profiles":
			if target, err := os.Readlink(filepath.Join(root, "profiles")); err == nil && target == d.sys("/nix/var/nix/profiles") {
				continue
			}
		case "current-system":
			if target, err := os.Readlink(filepath.Join(root, "current-system")); err == nil && target == d.sys("/run/current-system") && d.Platform.OS == "darwin" {
				continue
			}
		}
		problems = append(problems, "unrecognized Nix garbage-collection root "+filepath.Join(root, entry.Name()))
	}
	auto := filepath.Join(root, "auto")
	links, err := readPlainDirectory(auto)
	if errors.Is(err, os.ErrNotExist) {
		return problems, nil
	}
	if err != nil {
		return nil, err
	}
	if len(links) > 4096 {
		return nil, errors.New("Nix garbage-collection roots exceed their review bound")
	}
	// Nix registers its own per-user cache (for example the downloaded flake
	// registry) as an indirect root. That cache is retired user state, moved
	// aside with the rest; links anywhere else remain outside consumers.
	known := []string{filepath.Join(d.stateRoot(), "home-manager", "gcroots") + "/", filepath.Join(d.stateRoot(), "nix", "profiles") + "/",
		d.sys("/nix/var/nix/profiles") + "/", filepath.Join(d.Folders.Home, ".local", "state", "home-manager", "gcroots") + "/", filepath.Join(d.Folders.Home, ".local", "state", "nix", "profiles") + "/",
		filepath.Join(d.Environment.Cache, "nix") + "/"}
	for _, link := range links {
		path := filepath.Join(auto, link.Name())
		target, err := os.Readlink(path)
		if err != nil {
			problems = append(problems, "Nix root "+path+" is not a link")
			continue
		}
		if !filepath.IsAbs(target) {
			target = filepath.Join(auto, target)
		}
		if _, err := os.Lstat(target); errors.Is(err, os.ErrNotExist) {
			continue
		} else if errors.Is(err, os.ErrPermission) {
			// Determinate leaves stale roots for its removed scratch profile.
			// macOS puts that profile below root's private temporary directory.
			// Recheck as root without prompting or treating access denial as
			// absence. The same check runs again at the destructive-step gate.
			output, err := d.privilegedQuery(ctx, d.systemCommand(true, d.Environment.Executable, "internal-legacy-root-status", target))
			if err != nil {
				return nil, fmt.Errorf("read-only administrator inspection failed; refresh credentials with sudo -v before retrying migration: %w", err)
			}
			switch string(output) {
			case "absent\n":
				continue
			case "present\n":
			default:
				return nil, errors.New("privileged Nix root inspection returned an invalid status")
			}
		} else if err != nil {
			return nil, err
		}
		if target == d.sys("/run/current-system") || slices.ContainsFunc(known, func(prefix string) bool { return strings.HasPrefix(target, prefix) }) {
			continue
		}
		problems = append(problems, "an outside Nix consumer holds "+target)
	}
	return problems, nil
}

// Preview is unprivileged: it can only hint at processes from their command
// lines. The irreversible step uses the exact kernel scan.
func (d *legacyRetirementDriver) nixProcessHints(ctx context.Context) (string, error) {
	ps, err := d.program("/usr/bin/ps", "/bin/ps")
	if err != nil {
		return "", err
	}
	format := "pid=,args="
	if d.Platform.OS == "darwin" {
		format = "pid=,comm="
	}
	output, err := d.Query(ctx, d.systemCommand(false, ps, "-A", "-o", format))
	if err != nil {
		return "", err
	}
	found := []string{}
	for _, line := range strings.Split(string(output), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 || strings.HasSuffix(fields[1], "/bin/nix-daemon") || fields[1] == "nix-daemon" {
			continue
		}
		text := strings.Join(fields[1:], " ")
		for _, marker := range []string{d.sys("/nix/store") + "/", "/.nix-profile/", d.sys("/nix/var/nix/profiles") + "/", d.sys("/run/current-system") + "/", d.sys("/etc/profiles/per-user") + "/"} {
			if strings.Contains(text, marker) {
				found = append(found, fields[0]+" "+truncateLegacyText(fields[1], 128))
				break
			}
		}
	}
	return truncateLegacyText(strings.Join(found, ", "), 1024), nil
}

func (d *legacyRetirementDriver) nixProcesses(ctx context.Context, subject string) ([]string, error) {
	problems := []string{}
	if d.Platform.OS == "darwin" {
		return d.darwinNixProcesses(ctx, subject)
	}

	excluded, executable := "", ""
	if subject == "determinate" {
		var err error
		if excluded, err = d.determinateDaemonProcess(ctx); err != nil {
			return nil, err
		}
	}
	if subject == "daemon" {
		systemctl, err := d.program("/usr/bin/systemctl", "/bin/systemctl")
		if err != nil {
			return nil, err
		}
		output, err := d.Query(ctx, d.systemCommand(false, systemctl, "show", "--property=MainPID", "--value", "nix-daemon.service"))
		if err != nil {
			return nil, err
		}
		excluded = strings.TrimSpace(string(output))
		pid, err := strconv.ParseUint(excluded, 10, 32)
		if err != nil {
			return nil, errors.New("the Nix service has an invalid process identity")
		}
		if pid != 0 {
			// Nix's daemon command is a link to its multicall nix executable.
			// Compare the kernel path with that resolved payload, not argv[0].
			executable, err = filepath.EvalSymlinks(d.sys("/nix/var/nix/profiles/default/bin/nix-daemon"))
			if err != nil || !strings.HasPrefix(executable, d.sys("/nix/store")+"/") {
				return nil, errors.Join(errors.New("the Nix daemon executable cannot be proved"), err)
			}
		}
	}
	if !legacyCanonicalPath(d.Environment.Executable) {
		return nil, errors.New("the installed process-inspection helper cannot be located")
	}
	output, err := d.privilegedQuery(ctx, d.systemCommand(true, d.Environment.Executable, "internal-legacy-linux-processes", d.sys("/proc"), d.sys("/nix")))
	if err != nil {
		return nil, err
	}
	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		pid, name, ok := strings.Cut(line, "\t")
		if line == "" {
			continue
		}
		if !ok {
			return nil, errors.New("the Nix process scan returned an invalid record")
		}
		if excluded != "" && excluded != "0" && pid == excluded && (subject == "determinate" || name == executable) {
			continue
		}
		problems = append(problems, "process "+pid+" uses "+truncateLegacyText(name, 256))
	}
	return problems, nil
}

func (d *legacyRetirementDriver) accountRecord(ctx context.Context, name string) (string, error) {
	if d.Platform.OS == "darwin" {
		return d.darwinAccountRecord(ctx, name)
	}
	output, err := d.Query(ctx, d.systemCommand(false, d.sys("/usr/bin/getent"), "passwd", name))
	if code, ok := legacyExitCode(err); ok && code == 2 {
		return "absent", nil
	}
	if err != nil {
		return "", err
	}
	return strings.TrimSuffix(string(output), "\n"), nil
}

// Build-account members are filtered from group records because userdel
// removes them; any other member keeps the group in use.
func (d *legacyRetirementDriver) groupRecord(ctx context.Context, name string) (string, error) {
	if d.Platform.OS == "darwin" {
		return d.darwinGroupRecord(ctx, name)
	}
	output, err := d.Query(ctx, d.systemCommand(false, d.sys("/usr/bin/getent"), "group", name))
	if code, ok := legacyExitCode(err); ok && code == 2 {
		return "absent", nil
	}
	if err != nil {
		return "", err
	}
	fields := strings.Split(strings.TrimSuffix(string(output), "\n"), ":")
	if len(fields) != 4 || fields[0] != name {
		return "", fmt.Errorf("group lookup did not identify exactly %s", name)
	}
	others := []string{}
	for _, member := range strings.Split(fields[3], ",") {
		if member != "" && !legacyBuildUser.MatchString(member) {
			others = append(others, member)
		}
	}
	return name + ":" + fields[2] + ":" + strings.Join(sortedUnique(others), ","), nil
}

func (d *legacyRetirementDriver) accountProcesses(ctx context.Context, uids string) (string, error) {
	ps, err := d.program("/usr/bin/ps", "/bin/ps")
	if err != nil {
		return "", err
	}
	output, err := d.Query(ctx, d.systemCommand(false, ps, "-A", "-o", "uid="))
	if err != nil {
		return "", err
	}
	wanted := strings.Split(uids, ",")
	found := []string{}
	for _, field := range strings.Fields(string(output)) {
		if slices.Contains(wanted, field) {
			found = append(found, field)
		}
	}
	return strings.Join(sortedUnique(found), ","), nil
}

// Linux build accounts must have exactly the 2.34.0 installer shape:
// nixbldN, UID 30000+N, GID 30000, "Nix build user N", /var/empty,
// /sbin/nologin. Every other account must not use the build group.
func (d *legacyRetirementDriver) planLinuxBuildAccounts(ctx context.Context, scope *legacyRetirementScope) error {
	output, err := d.Query(ctx, d.systemCommand(false, d.sys("/usr/bin/getent"), "passwd"))
	if err != nil {
		return err
	}
	users, uids := []string{}, []string{}
	records := map[string]string{}
	for _, line := range strings.Split(strings.TrimSuffix(string(output), "\n"), "\n") {
		fields := strings.Split(line, ":")
		if len(fields) != 7 {
			continue
		}
		match := legacyBuildUser.FindStringSubmatch(fields[0])
		if match == nil || match[1] != "" {
			if fields[3] == "30000" {
				scope.Blocked = "account " + fields[0] + " uses the Nix build group as its primary group; it is another consumer"
				return nil
			}
			continue
		}
		index, _ := strconv.Atoi(match[2])
		expected := fmt.Sprintf("%s:x:%d:30000:Nix build user %d:/var/empty:/sbin/nologin", fields[0], 30000+index, index)
		if index > 32 || line != expected {
			scope.Blocked = "account " + fields[0] + " does not have the exact upstream Nix build-account shape; inspect it manually"
			return nil
		}
		users, uids, records[fields[0]] = append(users, fields[0]), append(uids, strconv.Itoa(30000+index)), line
	}
	group, err := d.groupRecord(ctx, "nixbld")
	if err != nil {
		return err
	}
	if group != "absent" && group != "nixbld:30000:" {
		scope.Blocked = "the nixbld group is not the upstream build group or has other members (" + group + ")"
		return nil
	}
	if running, err := d.accountProcesses(ctx, strings.Join(uids, ",")); err != nil {
		return err
	} else if running != "" {
		scope.Blocked = "Nix build accounts still run processes (UIDs " + running + "); stop Nix builds first"
		return nil
	}
	userdel, err := d.program("/usr/sbin/userdel")
	if err != nil {
		return err
	}
	for _, name := range users {
		command := d.systemCommand(true, userdel, name)
		scope.Steps = append(scope.Steps, legacyRetirementStep{Kind: "command", Label: "Delete Nix build account " + name, Privileged: true, Command: &command,
			Gates:  []legacyRetirementGate{{Kind: "account", Subject: name, Expected: records[name]}, {Kind: "account-processes", Subject: strings.Join(uids, ",")}},
			Checks: []legacyRetirementGate{{Kind: "account", Subject: name, Expected: "absent"}}})
	}
	if group != "absent" {
		groupdel, err := d.program("/usr/sbin/groupdel")
		if err != nil {
			return err
		}
		command := d.systemCommand(true, groupdel, "nixbld")
		scope.Steps = append(scope.Steps, legacyRetirementStep{Kind: "command", Label: "Delete the Nix build group nixbld", Privileged: true, Command: &command,
			Gates:  []legacyRetirementGate{{Kind: "group", Subject: "nixbld", Expected: "nixbld:30000:"}},
			Checks: []legacyRetirementGate{{Kind: "group", Subject: "nixbld", Expected: "absent"}}})
	}
	return nil
}

// Leftover build accounts are retired only while Nix's own build group or
// build accounts remain. Another account's numeric group is not Nix evidence.
func (d *legacyRetirementDriver) nixBuildAccountsPresent(ctx context.Context) (bool, error) {
	group, err := d.groupRecord(ctx, "nixbld")
	if err != nil {
		return false, err
	}
	if group != "absent" {
		return true, nil
	}
	names := []string{}
	if d.Platform.OS == "darwin" {
		accounts, err := d.dsclList(ctx, "Users", "UniqueID")
		if err != nil {
			return false, err
		}
		names = slices.Collect(maps.Keys(accounts))
	} else {
		output, err := d.Query(ctx, d.systemCommand(false, d.sys("/usr/bin/getent"), "passwd"))
		if err != nil {
			return false, err
		}
		for _, line := range strings.Split(strings.TrimSuffix(string(output), "\n"), "\n") {
			name, _, _ := strings.Cut(line, ":")
			names = append(names, name)
		}
	}
	return slices.ContainsFunc(names, legacyBuildUser.MatchString), nil
}

func (d *legacyRetirementDriver) loginShell(ctx context.Context) (string, error) {
	if d.Platform.OS == "darwin" {
		output, err := d.Query(ctx, d.systemCommand(false, d.sys("/usr/bin/dscl"), ".", "-read", "/Users/"+d.Environment.Username, "UserShell"))
		if err != nil {
			return "", err
		}
		value, ok := strings.CutPrefix(strings.TrimSpace(string(output)), "UserShell: ")
		if !ok || strings.ContainsAny(value, "\n") {
			return "", errors.New("macOS did not identify the account's login shell")
		}
		return value, nil
	}
	record, err := d.accountRecord(ctx, d.Environment.Username)
	if err != nil {
		return "", err
	}
	fields := strings.Split(record, ":")
	if len(fields) != 7 || fields[0] != d.Environment.Username {
		return "", errors.New("account lookup did not identify exactly the target user")
	}
	return fields[6], nil
}

// A shell is from Nix when it names a Nix profile path or resolves into the
// store. Only absence means it resolves nowhere; other lookup errors are
// inspection failures, never evidence that the shell is safe to keep.
func (d *legacyRetirementDriver) nixShell(shell string) (bool, error) {
	for _, prefix := range []string{d.sys("/nix") + "/", d.sys("/run/current-system") + "/", d.sys("/etc/profiles/per-user") + "/", filepath.Join(d.Folders.Home, ".nix-profile") + "/", filepath.Join(d.stateRoot(), "nix", "profiles") + "/"} {
		if strings.HasPrefix(shell, prefix) {
			return true, nil
		}
	}
	if !filepath.IsAbs(shell) {
		return false, nil
	}
	resolved, err := filepath.EvalSymlinks(shell)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("inspect login shell %s: %w", shell, err)
	}
	return strings.HasPrefix(resolved, d.sys("/nix")+"/"), nil
}

func (d *legacyRetirementDriver) shellListed(shell string) (bool, error) {
	data, _, err := readLegacyFile(d.sys("/etc/shells"), 1<<20)
	if err != nil {
		return false, err
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) == shell {
			return true, nil
		}
	}
	return false, nil
}

// Change only this account's shell, and only away from a Nix-provided one.
func (d *legacyRetirementDriver) planLoginShell(ctx context.Context, scope *legacyRetirementScope) error {
	shell, err := d.loginShell(ctx)
	if err != nil {
		return err
	}
	nix, err := d.nixShell(shell)
	if err != nil {
		return err
	}
	if nix {
		replacement := d.sys("/bin/bash")
		if filepath.Base(shell) == "zsh" {
			replacement = d.sys("/bin/zsh")
			if d.Platform.OS == "linux" {
				replacement = d.sys("/usr/bin/zsh")
			}
		}
		info, err := os.Stat(replacement)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		listed, listErr := d.shellListed(replacement)
		if listErr != nil {
			return listErr
		}
		if err != nil || !info.Mode().IsRegular() || !listed {
			scope.Blocked = "the login shell " + shell + " comes from Nix and its native replacement " + replacement + " is not installed and registered in /etc/shells; select it in setup first"
			return nil
		}
		var command nativeCommand
		if d.Platform.OS == "darwin" {
			command = d.systemCommand(true, d.sys("/usr/bin/dscl"), ".", "-create", "/Users/"+d.Environment.Username, "UserShell", replacement)
		} else {
			passwd, _, err := readLegacyFile(d.sys("/etc/passwd"), 16<<20)
			if err != nil {
				return err
			}
			if !bytes.HasPrefix(passwd, []byte(d.Environment.Username+":")) && !bytes.Contains(passwd, []byte("\n"+d.Environment.Username+":")) {
				scope.Blocked = "the Nix login shell belongs to a directory-service account that chsh cannot change; update it with your directory administrator"
				return nil
			}
			command = d.systemCommand(true, d.sys("/usr/bin/chsh"), "-s", replacement, d.Environment.Username)
		}
		scope.Steps = append(scope.Steps, legacyRetirementStep{Kind: "command", Label: "Change the login shell from " + shell + " to " + replacement, Privileged: true, Command: &command,
			Gates:  []legacyRetirementGate{{Kind: "login-shell", Expected: shell}},
			Checks: []legacyRetirementGate{{Kind: "login-shell", Expected: replacement}}})
	}
	// The released installer could register a Nix shell in /etc/shells.
	if info, err := os.Lstat(d.sys("/etc/shells")); err == nil && info.Mode().IsRegular() {
		data, _, err := readLegacyFile(d.sys("/etc/shells"), 1<<20)
		if err != nil {
			return err
		}
		remove := []string{}
		for _, line := range strings.Split(string(data), "\n") {
			if !strings.HasPrefix(line, "/") {
				continue
			}
			nix, err := d.nixShell(line)
			if err != nil {
				return err
			}
			if nix {
				remove = append(remove, line)
			}
		}
		return d.addLineRemoval(ctx, scope, "Unregister retired Nix shells", d.sys("/etc/shells"), sortedUnique(remove)...)
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

// Home Manager's own ownership rule: a home path is managed only when it is a
// link to the same relative path inside the active generation's home-files.
func (d *legacyRetirementDriver) planHomeManager(_ context.Context, scope *legacyRetirementScope) error {
	generation := ""
	for _, candidate := range []string{filepath.Join(d.stateRoot(), "home-manager", "gcroots", "current-home"), filepath.Join(d.stateRoot(), "nix", "profiles", "home-manager"),
		filepath.Join(d.sys("/nix/var/nix/profiles/per-user"), d.Environment.Username, "home-manager")} {
		resolved, err := filepath.EvalSymlinks(candidate)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return err
		}
		if !strings.HasPrefix(resolved, d.sys("/nix/store")+"/") {
			scope.Blocked = "the Home Manager generation at " + candidate + " is outside the Nix store"
			return nil
		}
		generation = resolved
		break
	}
	if generation == "" {
		return nil
	}
	files, err := filepath.EvalSymlinks(filepath.Join(generation, "home-files"))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if !strings.HasPrefix(files, d.sys("/nix/store")+"/") {
		scope.Blocked = "Home Manager home-files are outside the Nix store"
		return nil
	}
	scope.Native = strings.TrimSpace(scope.Native + " Home Manager " + filepath.Base(generation))
	count := 0
	return filepath.WalkDir(files, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if count++; count > maxConfigEntries {
			return errors.New("Home Manager file inventory exceeds its review bound")
		}
		if entry.IsDir() {
			return nil
		}
		relative, err := filepath.Rel(files, path)
		if err != nil {
			return err
		}
		target := filepath.Join(d.Folders.Home, relative)
		info, err := os.Lstat(target)
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink == 0 {
			scope.Detail = append(scope.Detail, target+" replaced its Home Manager link and stays.")
			return nil
		}
		link, err := os.Readlink(target)
		if err != nil {
			return err
		}
		if link != path {
			scope.Detail = append(scope.Detail, target+" no longer points to Home Manager and stays.")
			return nil
		}
		scope.Steps = append(scope.Steps, legacyRetirementStep{Kind: "discard", Label: "Remove Home Manager link " + target, Path: target,
			Entries: []configEntry{{Path: ".", Kind: "link", Content: link}}, Absent: []string{target}})
		return nil
	})
}

// Hooks the released helper never wrote come from another installation. They
// are guarded by the Nix profile path, so they are disclosed, never edited.
func (d *legacyRetirementDriver) discloseSystemShellHooks(scope *legacyRetirementScope) error {
	for _, name := range []string{"/etc/profile", "/etc/bash.bashrc", "/etc/bashrc", "/etc/zsh/zshrc", "/etc/zshrc", "/etc/zshenv", "/etc/zsh/zshenv", "/etc/profile.d/nix.sh"} {
		path := d.sys(name)
		info, err := os.Lstat(path)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			path += ".before-nix-darwin"
			info, err = os.Lstat(path)
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			if err != nil {
				return err
			}
			if !info.Mode().IsRegular() {
				continue
			}
		}
		data, _, err := readLegacyFile(path, 1<<20)
		if err != nil {
			return err
		}
		if bytes.Contains(data, []byte("/nix/var/nix/profiles/default/etc/profile.d/nix")) {
			scope.Retained = append(scope.Retained, path+" contains a Nix shell hook from another installer; remove that guarded block manually if you no longer want it")
		}
	}
	return nil
}

func (d *legacyRetirementDriver) readyGates(ctx context.Context, consumers string) ([]legacyRetirementGate, error) {
	store, err := d.nixStoreIdentity(ctx)
	if err != nil {
		return nil, err
	}
	return []legacyRetirementGate{{Kind: "replacements"}, {Kind: "nix-homebrew"}, {Kind: "query-git", Subject: d.sys("/nix")}, {Kind: "nix-store", Expected: store}, {Kind: "nix-consumers", Subject: consumers}}, nil
}

// The irreversible deletion follows only the steps that stop consumers. Every
// step that removes classification evidence comes after it, so each completed
// prefix stays rediscoverable by a fresh inspection.
func (d *legacyRetirementDriver) planUpstreamLinux(ctx context.Context, scope *legacyRetirementScope, daemon bool) error {
	nix := filepath.Join(d.Folders.Home, ".nix-profile", "bin", "nix")
	if daemon {
		nix = d.sys("/nix/var/nix/profiles/default/bin/nix")
	}
	version, err := d.Query(ctx, d.systemCommand(false, nix, "--version"))
	if err != nil || strings.TrimSpace(string(version)) != legacyNixVersion {
		scope.Blocked = legacyReason("the installed Nix is not the released upstream 2.34.0 layout; inspect it natively before removal", err)
		return nil
	}
	scope.Native = strings.TrimSpace("upstream Nix 2.34.0 " + map[bool]string{true: "multi-user daemon", false: "single-user"}[daemon] + "; " + scope.Native)
	if err := d.discloseSystemShellHooks(scope); err != nil {
		return err
	}
	if daemon {
		links, reason, err := d.linuxNixUnitLinks()
		if err != nil || reason != "" {
			scope.Blocked = reason
			return err
		}
		if !slices.Contains(links, d.sys("/etc/systemd/system/nix-daemon.service")) {
			scope.Blocked = "the Nix daemon unit is missing; inspect the partial installation manually"
			return nil
		}
		gates, err := d.readyGates(ctx, "daemon")
		if err != nil {
			return err
		}
		if err := d.planLinuxDaemonStop(scope, gates); err != nil {
			return err
		}
		if err := d.planResolvedNixActivationLinks(scope, links); err != nil {
			return err
		}
	}
	approval, err := d.nixStoreIdentity(ctx)
	if err != nil {
		return err
	}
	step, err := d.nixDeletionStep("Permanently delete the Nix store and runtime at /nix", approval)
	if err != nil {
		return err
	}
	scope.Steps = append(scope.Steps, step)
	if !daemon {
		return nil
	}
	return d.planLinuxNixIntegrations(ctx, scope)
}

// The 2.34.0 systemd installer links these paths to the default profile.
// systemctl enable may point the unit's wants link at either location.
func (d *legacyRetirementDriver) nixUnitTargets(path string) []string {
	profile := d.sys("/nix/var/nix/profiles/default/lib")
	switch path {
	case d.sys("/etc/systemd/system/nix-daemon.service"):
		return []string{profile + "/systemd/system/nix-daemon.service"}
	case d.sys("/etc/systemd/system/nix-daemon.socket"):
		return []string{profile + "/systemd/system/nix-daemon.socket"}
	case d.sys("/etc/systemd/system/sockets.target.wants/nix-daemon.socket"):
		return []string{d.sys("/etc/systemd/system/nix-daemon.socket"), profile + "/systemd/system/nix-daemon.socket"}
	case d.sys("/etc/systemd/system/multi-user.target.wants/nix-daemon.service"):
		return []string{d.sys("/etc/systemd/system/nix-daemon.service"), profile + "/systemd/system/nix-daemon.service"}
	case d.sys("/etc/tmpfiles.d/nix-daemon.conf"):
		return []string{profile + "/tmpfiles.d/nix-daemon.conf"}
	}
	return nil
}

// linuxNixUnitLinks returns the exact installer links that remain, or a
// refusal for any other file at their paths.
func (d *legacyRetirementDriver) linuxNixUnitLinks() ([]string, string, error) {
	links := []string{}
	for _, path := range []string{d.sys("/etc/systemd/system/nix-daemon.service"), d.sys("/etc/systemd/system/nix-daemon.socket"),
		d.sys("/etc/systemd/system/sockets.target.wants/nix-daemon.socket"),
		d.sys("/etc/systemd/system/multi-user.target.wants/nix-daemon.service"), d.sys("/etc/tmpfiles.d/nix-daemon.conf")} {
		info, err := os.Lstat(path)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, "", err
		}
		target := ""
		if info.Mode()&os.ModeSymlink != 0 {
			if target, err = os.Readlink(path); err != nil {
				return nil, "", err
			}
		}
		accepted := slices.Contains(d.nixUnitTargets(path), target)
		if !accepted && slices.Contains([]string{"sockets.target.wants", "multi-user.target.wants"}, filepath.Base(filepath.Dir(path))) {
			accepted, err = d.nixActivationTarget(filepath.Base(path), target)
			if err != nil {
				return nil, "", err
			}
		}
		if !accepted {
			return nil, "unrecognized Nix systemd integration at " + path + "; " + d.nixUnitLinkAdvice(path, target), nil
		}
		links = append(links, path)
	}
	return links, "", nil
}

func (d *legacyRetirementDriver) planLinuxDaemonStop(scope *legacyRetirementScope, gates []legacyRetirementGate) error {
	stop, err := d.systemdStopStep("Stop the Nix daemon", []string{"nix-daemon.socket", "nix-daemon.service"})
	if err != nil {
		return err
	}
	stop.Gates = gates
	scope.Steps = append(scope.Steps, stop)
	return nil
}

// The daemon's system integrations are inert once its store is gone, so they
// are retired after it: the exact unit links (not systemctl disable, which can
// fail on a dangling linked unit), the proxy override, a systemd reload,
// root's Nix files and /etc/nix. The exact build accounts go last; until then
// they remain the evidence that the other integrations belong to this daemon.
func (d *legacyRetirementDriver) planLinuxNixIntegrations(ctx context.Context, scope *legacyRetirementScope) error {
	links, reason, err := d.linuxNixUnitLinks()
	if err != nil || reason != "" {
		scope.Blocked = reason
		return err
	}
	override := d.sys("/etc/systemd/system/nix-daemon.service.d")
	overrideIdentity, err := legacyPathIdentity(override)
	if err != nil {
		return err
	}
	accounts := legacyRetirementScope{}
	if present, err := d.nixBuildAccountsPresent(ctx); err != nil {
		return err
	} else if present {
		if err := d.planLinuxBuildAccounts(ctx, &accounts); err != nil {
			return err
		}
	}
	if accounts.Blocked != "" {
		scope.Blocked = accounts.Blocked
		return nil
	}
	if len(links) == 0 && overrideIdentity == "absent" && len(accounts.Steps) == 0 {
		return nil
	}
	systemctl, err := d.program("/usr/bin/systemctl", "/bin/systemctl")
	if err != nil {
		return err
	}
	reload := len(links) != 0 || overrideIdentity != "absent"
	if !reload {
		// An interrupted retirement can remove the files before reloading.
		output, err := d.Query(ctx, d.systemCommand(false, systemctl, "show", "--property=NeedDaemonReload", "--value", "nix-daemon.service", "nix-daemon.socket"))
		if err != nil {
			return err
		}
		reload = slices.Contains(strings.Fields(string(output)), "yes")
	}
	for _, path := range links {
		if slices.ContainsFunc(scope.Steps, func(step legacyRetirementStep) bool { return step.Kind == "remove" && step.Path == path }) {
			continue // This store-resolved activation link is removed first.
		}
		if err := addLegacyRemoval(scope, "Remove the Nix systemd link", path); err != nil {
			return err
		}
	}
	if err := d.addSystemMove(scope, "Preserve the Nix daemon proxy override", override); err != nil {
		return err
	}
	if reload {
		command := d.systemCommand(true, systemctl, "daemon-reload")
		scope.Steps = append(scope.Steps, legacyRetirementStep{Kind: "command", Label: "Reload the systemd unit inventory", Privileged: true, Command: &command})
	}
	root, err := d.accountRecord(ctx, "root")
	if err != nil {
		return err
	}
	fields := strings.Split(root, ":")
	if len(fields) != 7 || !legacyCanonicalPath(fields[5]) {
		return errors.New("root's home directory could not be identified")
	}
	if err := d.addRootFiles(scope, fields[5]); err != nil {
		return err
	}
	if err := d.addSystemMove(scope, "Preserve the retired system Nix configuration", d.sys("/etc/nix")); err != nil {
		return err
	}
	scope.Steps = append(scope.Steps, accounts.Steps...)
	return nil
}

// Root's home is unreadable to the controller, so each fixed name moves only
// if present. Backups are named from this installation's /etc/nix, which
// stays in place until these moves are done.
func (d *legacyRetirementDriver) addRootFiles(scope *legacyRetirementScope, home string) error {
	installation, err := legacyPathIdentity(d.sys("/etc/nix"))
	if err != nil {
		return err
	}
	names := []string{".nix-profile", ".nix-defexpr", ".nix-channels", ".cache/nix", ".local/state/nix", ".local/share/nix"}
	for _, name := range names {
		source := filepath.Join(home, name)
		backup, err := legacySiblingBackup(source, installation)
		if err != nil {
			return err
		}
		command := d.systemCommand(true, "/bin/sh", "-c", legacyRootFilesScript, "sh", source, backup)
		scope.Steps = append(scope.Steps, legacyRetirementStep{Kind: "command", Label: "Preserve root's Nix file if present: " + source + " -> " + backup, Privileged: true, Backup: backup, Command: &command})
	}
	return nil
}
