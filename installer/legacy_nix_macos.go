package installer

import (
	"bytes"
	"context"
	"encoding/xml"
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
	"time"
)

// macOS evidence is parsed as data. nix-darwin's own uninstaller is never run:
// it restores every /etc/*.before-nix-darwin and rewrites every /run/... login
// shell, which can overwrite personal edits and change other accounts.

var legacyVolumeUUID = regexp.MustCompile(`^[0-9A-F]{8}-[0-9A-F]{4}-[0-9A-F]{4}-[0-9A-F]{4}-[0-9A-F]{12}$`)

type legacyPlistNode struct {
	XMLName xml.Name
	Text    string            `xml:",chardata"`
	Nodes   []legacyPlistNode `xml:",any"`
}

func legacyPlist(data []byte) (map[string]legacyPlistNode, error) {
	var document struct {
		XMLName xml.Name        `xml:"plist"`
		Dict    legacyPlistNode `xml:"dict"`
	}
	if err := xml.Unmarshal(data, &document); err != nil {
		return nil, err
	}
	nodes := document.Dict.Nodes
	if len(nodes)%2 != 0 {
		return nil, errors.New("invalid property list dictionary")
	}
	values := map[string]legacyPlistNode{}
	for i := 0; i < len(nodes); i += 2 {
		key := strings.TrimSpace(nodes[i].Text)
		if nodes[i].XMLName.Local != "key" || key == "" {
			return nil, errors.New("invalid property list key")
		}
		if _, duplicate := values[key]; duplicate {
			return nil, errors.New("duplicate property list key")
		}
		values[key] = nodes[i+1]
	}
	return values, nil
}

func legacyPlistString(values map[string]legacyPlistNode, key string) string {
	if node, ok := values[key]; ok && node.XMLName.Local == "string" {
		return node.Text
	}
	return ""
}

func legacyPlistStrings(values map[string]legacyPlistNode, key string) []string {
	node, ok := values[key]
	if !ok || node.XMLName.Local != "array" {
		return nil
	}
	result := []string{}
	for _, child := range node.Nodes {
		if child.XMLName.Local != "string" {
			return nil
		}
		result = append(result, child.Text)
	}
	return result
}

// The released Nix installer wrote one UUID-based APFS entry for /nix.
func legacyNixFstabLine() (string, string, error) {
	return legacyNixFstabLineAt("/etc/fstab")
}

func legacyNixFstabLineAt(path string) (string, string, error) {
	data, _, err := readLegacyFile(path, 1<<20)
	if err != nil || data == nil {
		return "", "", err
	}
	return parseLegacyNixFstab(data)
}

func (d *legacyRetirementDriver) nixFstabLine(ctx context.Context) (string, string, error) {
	data, _, err := d.readSystemText(ctx, d.sys("/etc/fstab"))
	if err != nil {
		return "", "", err
	}
	return parseLegacyNixFstab(data)
}

func parseLegacyNixFstab(data []byte) (string, string, error) {
	uuid, line := "", ""
	for _, current := range strings.Split(string(data), "\n") {
		fields := strings.Fields(current)
		if len(fields) < 3 || strings.HasPrefix(fields[0], "#") || fields[1] != "/nix" {
			continue
		}
		value, ok := strings.CutPrefix(fields[0], "UUID=")
		if line != "" || !ok || !legacyVolumeUUID.MatchString(value) || fields[2] != "apfs" {
			return "", "", errors.New("/etc/fstab has an unrecognized or repeated /nix entry; review it manually")
		}
		uuid, line = value, current
	}
	return uuid, line, nil
}

// Volume identity is name, UUID and filesystem. The mount point is not part of
// it: after a reboot an unmounted Nix volume is still the same approved volume.
func (d *legacyRetirementDriver) volumeIdentity(ctx context.Context, uuid string) (string, error) {
	if !legacyVolumeUUID.MatchString(uuid) {
		return "", errors.New("invalid APFS volume UUID")
	}
	output, err := d.Query(ctx, d.systemCommand(false, d.sys("/usr/sbin/diskutil"), "info", "-plist", uuid))
	if code, ok := legacyExitCode(err); ok && code == 1 && (strings.Contains(err.Error(), "Could not find disk") || bytes.Contains(output, []byte("Could not find disk"))) {
		return "absent", nil
	}
	if err != nil {
		return "", err
	}
	values, err := legacyPlist(output)
	if err != nil {
		return "", err
	}
	return "VolumeName=" + legacyPlistString(values, "VolumeName") + ";VolumeUUID=" + legacyPlistString(values, "VolumeUUID") +
		";FilesystemType=" + legacyPlistString(values, "FilesystemType"), nil
}

func (d *legacyRetirementDriver) launchdState(ctx context.Context, service string) (string, error) {
	_, err := d.Query(ctx, d.systemCommand(false, d.sys("/bin/launchctl"), "print", service))
	if err == nil {
		return "loaded", nil
	}
	if code, ok := legacyExitCode(err); ok && code == 113 {
		return "absent", nil
	}
	return "", err
}

// bootout can return while launchd is still unregistering the service. Only
// postconditions wait for that transition; approval/consumer gates remain
// immediate, and inspection failures are never treated as eventual success.
func (d *legacyRetirementDriver) waitLaunchdAbsent(ctx context.Context, service string) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		state, err := d.launchdState(ctx, service)
		if err != nil {
			return err
		}
		if state == "absent" {
			return nil
		}
		timer := time.NewTimer(100 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return fmt.Errorf("launchd service %s did not finish unloading: %w", service, ctx.Err())
		case <-timer.C:
		}
	}
}

func legacyDsclAttributes(output []byte) map[string]string {
	values := map[string]string{}
	key := ""
	for _, line := range strings.Split(string(output), "\n") {
		if strings.HasPrefix(line, " ") && key != "" {
			values[key] = strings.TrimSpace(values[key] + " " + strings.TrimSpace(line))
			continue
		}
		name, value, ok := strings.Cut(line, ":")
		if ok && (name == "dsAttrTypeNative" || name == "dsAttrTypeStandard") {
			attribute, rest, qualified := strings.Cut(value, ":")
			name, value, ok = name+":"+attribute, rest, qualified
		}
		if !ok || name == "" || strings.Contains(name, " ") {
			key = ""
			continue
		}
		key = name
		values[key] = strings.TrimSpace(value)
	}
	return values
}

func (d *legacyRetirementDriver) dsclList(ctx context.Context, kind, attribute string) (map[string]string, error) {
	output, err := d.Query(ctx, d.systemCommand(false, d.sys("/usr/bin/dscl"), ".", "-list", "/"+kind, attribute))
	if err != nil {
		return nil, err
	}
	values := map[string]string{}
	for _, line := range strings.Split(string(output), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 {
			values[fields[0]] = fields[1]
		}
	}
	return values, nil
}

func (d *legacyRetirementDriver) darwinAccountRecord(ctx context.Context, name string) (string, error) {
	accounts, err := d.dsclList(ctx, "Users", "UniqueID")
	if err != nil {
		return "", err
	}
	if _, found := accounts[name]; !found {
		return "absent", nil
	}
	output, err := d.Query(ctx, d.systemCommand(false, d.sys("/usr/bin/dscl"), ".", "-read", "/Users/"+name))
	if err != nil {
		return "", err
	}
	values := legacyDsclAttributes(output)
	hidden, ok := values["dsAttrTypeNative:IsHidden"]
	if !ok {
		// Older captures and abbreviated dscl output use the short name.
		hidden = values["IsHidden"]
	}
	return strings.Join([]string{name, values["UniqueID"], values["PrimaryGroupID"], values["RealName"], values["NFSHomeDirectory"], values["UserShell"], hidden}, ":"), nil
}

func (d *legacyRetirementDriver) darwinGroupRecord(ctx context.Context, name string) (string, error) {
	groups, err := d.dsclList(ctx, "Groups", "PrimaryGroupID")
	if err != nil {
		return "", err
	}
	gid, found := groups[name]
	if !found {
		return "absent", nil
	}
	output, err := d.Query(ctx, d.systemCommand(false, d.sys("/usr/bin/dscl"), ".", "-read", "/Groups/"+name))
	if err != nil {
		return "", err
	}
	others := []string{}
	for _, member := range strings.Fields(legacyDsclAttributes(output)["GroupMembership"]) {
		if !legacyBuildUser.MatchString(member) {
			others = append(others, member)
		}
	}
	return name + ":" + gid + ":" + strings.Join(sortedUnique(others), ","), nil
}

// macOS build accounts must have exactly the 2.34.0 installer shape:
// _nixbldN, UID 350+N, group 350, "Nix build user N", /var/empty,
// /sbin/nologin and hidden. No other account may use the build group.
func (d *legacyRetirementDriver) planDarwinBuildAccounts(ctx context.Context, scope *legacyRetirementScope) error {
	accounts, err := d.dsclList(ctx, "Users", "PrimaryGroupID")
	if err != nil {
		return err
	}
	users, uids := []string{}, []string{}
	records := map[string]string{}
	for _, name := range slices.Sorted(maps.Keys(accounts)) {
		match := legacyBuildUser.FindStringSubmatch(name)
		if match == nil || match[1] != "_" {
			if accounts[name] == "350" {
				scope.Blocked = "account " + name + " uses the Nix build group as its primary group; it is another consumer"
				return nil
			}
			continue
		}
		index, _ := strconv.Atoi(match[2])
		record, err := d.darwinAccountRecord(ctx, name)
		if err != nil {
			return err
		}
		expected := fmt.Sprintf("%s:%d:350:Nix build user %d:/var/empty:/sbin/nologin:1", name, 350+index, index)
		if index > 32 || record != expected {
			scope.Blocked = "account " + name + " does not have the exact upstream Nix build-account shape; inspect it manually"
			return nil
		}
		users, uids, records[name] = append(users, name), append(uids, strconv.Itoa(350+index)), record
	}
	group, err := d.darwinGroupRecord(ctx, "nixbld")
	if err != nil {
		return err
	}
	if group != "absent" && group != "nixbld:350:" {
		scope.Blocked = "the nixbld group is not the upstream build group or has other members (" + group + ")"
		return nil
	}
	if running, err := d.accountProcesses(ctx, strings.Join(uids, ",")); err != nil {
		return err
	} else if running != "" {
		scope.Blocked = "Nix build accounts still run processes (UIDs " + running + "); stop Nix builds first"
		return nil
	}
	for _, name := range users {
		command := d.systemCommand(true, d.sys("/usr/bin/dscl"), ".", "-delete", "/Users/"+name)
		scope.Steps = append(scope.Steps, legacyRetirementStep{Kind: "command", Label: "Delete Nix build account " + name, Privileged: true, Command: &command,
			Gates:  []legacyRetirementGate{{Kind: "account", Subject: name, Expected: records[name]}, {Kind: "account-processes", Subject: strings.Join(uids, ",")}},
			Checks: []legacyRetirementGate{{Kind: "account", Subject: name, Expected: "absent"}}})
	}
	if group != "absent" {
		command := d.systemCommand(true, d.sys("/usr/bin/dscl"), ".", "-delete", "/Groups/nixbld")
		scope.Steps = append(scope.Steps, legacyRetirementStep{Kind: "command", Label: "Delete the Nix build group nixbld", Privileged: true, Command: &command,
			Gates:  []legacyRetirementGate{{Kind: "group", Subject: "nixbld", Expected: "nixbld:350:"}},
			Checks: []legacyRetirementGate{{Kind: "group", Subject: "nixbld", Expected: "absent"}}})
	}
	return nil
}

// A launchd plist is retired only when its bytes are the exact installed copy
// of the approved source and its Label matches the file name.
func (d *legacyRetirementDriver) planLaunchdService(ctx context.Context, scope *legacyRetirementScope, source, installed, domain string, privileged bool) error {
	resolved, err := filepath.EvalSymlinks(source)
	if err != nil {
		return err
	}
	expected, _, err := readLegacyFile(resolved, 1<<20)
	if err != nil {
		return err
	}
	actual, _, err := readLegacyFile(installed, 1<<20)
	if err != nil {
		return err
	}
	if actual == nil {
		return nil
	}
	values, err := legacyPlist(actual)
	label := strings.TrimSuffix(filepath.Base(installed), ".plist")
	if err != nil || !bytes.Equal(actual, expected) || legacyPlistString(values, "Label") != label {
		scope.Blocked = installed + " differs from its recorded Nix launch service; inspect it manually"
		return nil
	}
	service := domain + "/" + label
	state, err := d.launchdState(ctx, service)
	if err != nil {
		return err
	}
	if state == "loaded" {
		command := d.systemCommand(privileged, d.sys("/bin/launchctl"), "bootout", service)
		scope.Steps = append(scope.Steps, legacyRetirementStep{Kind: "command", Label: "Stop launch service " + service, Privileged: privileged, Command: &command,
			Gates:  []legacyRetirementGate{{Kind: "launchd", Subject: service, Expected: "loaded"}},
			Checks: []legacyRetirementGate{{Kind: "launchd", Subject: service, Expected: "absent"}}})
	}
	if privileged {
		return addLegacyRemoval(scope, "Remove the retired launch service", installed)
	}
	return addLegacyDiscard(scope, "Remove the retired launch agent", installed)
}

// nix-darwin is retired from its active generation only: launch services it
// copied, /etc links into /etc/static and the backups those links displaced.
// Each step stays rediscoverable: a removed link still restores its backup,
// an already removed /etc/static is not a foreign change, and the generation
// link that identifies nix-darwin is removed last.
func (d *legacyRetirementDriver) planNixDarwin(ctx context.Context, scope *legacyRetirementScope) error {
	system, err := os.Readlink(d.sys("/run/current-system"))
	if err != nil || !strings.HasPrefix(system, d.sys("/nix/store")+"/") || !strings.Contains(filepath.Base(system), "darwin-system") {
		scope.Blocked = "/run/current-system is not a nix-darwin generation in the Nix store"
		return nil
	}
	etc, err := filepath.EvalSymlinks(filepath.Join(system, "etc"))
	if err != nil {
		return err
	}
	static, err := os.Readlink(d.sys("/etc/static"))
	if err != nil && !errors.Is(err, os.ErrNotExist) || err == nil && static != etc {
		scope.Blocked = legacyReason("/etc/static does not belong to the active nix-darwin generation; inspect it manually", err)
		return nil
	}
	scope.Native = strings.TrimSpace(scope.Native + " nix-darwin " + filepath.Base(system))
	gates := []legacyRetirementGate{{Kind: "replacements"}}
	first := len(scope.Steps)
	for _, item := range []struct{ source, installed, domain string }{
		{filepath.Join(system, "Library", "LaunchDaemons"), d.sys("/Library/LaunchDaemons"), "system"},
		{filepath.Join(system, "user", "Library", "LaunchAgents"), filepath.Join(d.Folders.Home, "Library", "LaunchAgents"), "gui/" + d.Environment.UID},
	} {
		entries, err := os.ReadDir(item.source)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return err
		}
		for _, entry := range entries {
			if err := d.planLaunchdService(ctx, scope, filepath.Join(item.source, entry.Name()), filepath.Join(item.installed, entry.Name()), item.domain, item.domain == "system"); err != nil || scope.Blocked != "" {
				return err
			}
		}
	}
	if entries, err := os.ReadDir(filepath.Join(system, "Library", "LaunchAgents")); err == nil && len(entries) != 0 {
		scope.Blocked = "nix-darwin system-wide launch agents are not part of the released configuration; inspect them manually"
		return nil
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	err = filepath.WalkDir(etc, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.Type()&fs.ModeSymlink == 0 {
			return err
		}
		relative, err := filepath.Rel(etc, path)
		if err != nil {
			return err
		}
		target := filepath.Join(d.sys("/etc"), relative)
		link, err := os.Readlink(target)
		if errors.Is(err, os.ErrNotExist) {
			// The link is already removed; finish restoring what it displaced.
			return d.addDarwinRestore(scope, target)
		}
		if err != nil || link != filepath.Join(d.sys("/etc/static"), relative) {
			scope.Detail = append(scope.Detail, target+" no longer points to nix-darwin and stays unchanged.")
			return nil
		}
		if err := addLegacyRemoval(scope, "Remove the nix-darwin /etc link", target); err != nil {
			return err
		}
		return d.addDarwinRestore(scope, target)
	})
	if err != nil {
		return err
	}
	if err := addLegacyRemoval(scope, "Remove the nix-darwin activation link", d.sys("/etc/static")); err != nil {
		return err
	}
	apps := d.sys("/Applications/Nix Apps")
	if entries, err := readPlainDirectory(apps); err == nil {
		if len(entries) == 0 {
			scope.Steps = append(scope.Steps, legacyRetirementStep{Kind: "rmdir", Label: "Remove the empty " + apps + " folder", Privileged: true, Path: apps, Absent: []string{apps}})
		} else {
			scope.Retained = append(scope.Retained, apps+" contains copied applications that may depend on Nix; review and remove them manually")
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := addLegacyRemoval(scope, "Remove the nix-darwin activation link", d.sys("/run/current-system")); err != nil {
		return err
	}
	if first < len(scope.Steps) {
		scope.Steps[first].Gates = append(scope.Steps[first].Gates, gates...)
	}
	scope.Detail = append(scope.Detail, "Only this account's login shell is changed; other accounts and unrelated /etc files are not touched. Do not run darwin-rebuild after this step.")
	return nil
}

func (d *legacyRetirementDriver) addDarwinRestore(scope *legacyRetirementScope, target string) error {
	backup := target + ".before-nix-darwin"
	identity, err := legacyPathIdentity(backup)
	if err != nil || identity == "absent" {
		return err
	}
	if info, err := os.Lstat(backup); err != nil || !info.Mode().IsRegular() {
		scope.Detail = append(scope.Detail, backup+" is not a regular file and stays for manual review.")
		return err
	}
	scope.Steps = append(scope.Steps, legacyRetirementStep{Kind: "move", Label: "Restore the file nix-darwin displaced: " + backup + " -> " + target, Privileged: true,
		Path: backup, Backup: target, Identity: identity, Restores: true})
	return nil
}

// Nix 2.34.0's create-darwin-volume.sh mounts the volume with one of exactly
// two commands; the encrypted one reads its password from the keychain.
func legacyNixStoreMount(arguments []string, uuid string) bool {
	plain := []string{"/usr/sbin/diskutil", "mount", "-mountPoint", "/nix", uuid}
	encrypted := []string{"/bin/sh", "-c", "/usr/bin/security find-generic-password -s '" + uuid + "' -w | /usr/sbin/diskutil apfs unlockVolume '" + uuid + "' -mountpoint '/nix' -stdinpassphrase"}
	return legacyVolumeUUID.MatchString(uuid) && (slices.Equal(arguments, plain) || slices.Equal(arguments, encrypted))
}

var legacyNixStoreMountUUID = regexp.MustCompile(`^/usr/bin/security find-generic-password -s '([0-9A-F-]{36})' -w `)

// darwinStoreService reads the mount service. It returns its volume UUID when
// the plist is exactly the installer's, or a reason when it is something else.
func (d *legacyRetirementDriver) darwinStoreService(uuid string) (bool, string, string, error) {
	data, _, err := readLegacyFile(d.sys("/Library/LaunchDaemons/org.nixos.darwin-store.plist"), 1<<20)
	if err != nil || data == nil {
		return false, "", "", err
	}
	values, err := legacyPlist(data)
	arguments := legacyPlistStrings(values, "ProgramArguments")
	if uuid == "" && len(arguments) == 5 {
		uuid = arguments[4]
	} else if uuid == "" && len(arguments) == 3 {
		if match := legacyNixStoreMountUUID.FindStringSubmatch(arguments[2]); match != nil {
			uuid = match[1]
		}
	}
	if err != nil || legacyPlistString(values, "Label") != "org.nixos.darwin-store" || !legacyNixStoreMount(arguments, uuid) {
		return true, "", legacyReason("the Nix Store mount service does not mount the recorded volume; inspect it manually", err), nil
	}
	return true, uuid, "", nil
}

// The volume deletion follows only the steps that stop consumers; the mount
// service, boot-time entries and build accounts that identify the volume stay
// until it is gone. If the volume is busy, a restart remounts it unchanged.
func (d *legacyRetirementDriver) planUpstreamDarwin(ctx context.Context, scope *legacyRetirementScope) error {
	version, err := d.Query(ctx, d.systemCommand(false, d.sys("/nix/var/nix/profiles/default/bin/nix"), "--version"))
	if err != nil || strings.TrimSpace(string(version)) != legacyNixVersion {
		scope.Blocked = legacyReason("the installed Nix is not the released upstream 2.34.0 layout; inspect it natively before removal", err)
		return nil
	}
	uuid, _, err := d.nixFstabLine(ctx)
	if err != nil || uuid == "" {
		scope.Blocked = legacyReason("the Nix Store volume has no single recognized /etc/fstab entry; inspect it manually", err)
		return nil
	}
	volume, err := d.volumeIdentity(ctx, uuid)
	if err != nil {
		return err
	}
	expectedVolume := "VolumeName=Nix Store;VolumeUUID=" + uuid + ";FilesystemType=apfs"
	if volume != expectedVolume {
		scope.Blocked = "the /nix volume is not the recorded Nix Store APFS volume (" + volume + ")"
		return nil
	}
	present, _, reason, err := d.darwinStoreService(uuid)
	if err != nil || !present || reason != "" {
		scope.Blocked = legacyReason("the Nix Store mount service is missing or unreadable", err)
		if reason != "" {
			scope.Blocked = reason
		}
		return nil
	}
	scope.Native = strings.TrimSpace("upstream Nix 2.34.0 daemon on APFS volume " + uuid + "; " + scope.Native)
	if err := d.discloseSystemShellHooks(scope); err != nil {
		return err
	}
	first := len(scope.Steps)
	if err := d.planLaunchdService(ctx, scope, d.sys("/nix/var/nix/profiles/default/Library/LaunchDaemons/org.nixos.nix-daemon.plist"), d.sys("/Library/LaunchDaemons/org.nixos.nix-daemon.plist"), "system", true); err != nil || scope.Blocked != "" {
		return err
	}
	ready, err := d.readyGates(ctx, "daemon")
	if err != nil {
		return err
	}
	ready = append(ready, legacyRetirementGate{Kind: "volume", Subject: uuid, Expected: expectedVolume})
	if first < len(scope.Steps) {
		scope.Steps[first].Gates = append(scope.Steps[first].Gates, ready...)
	}
	command := d.systemCommand(true, d.sys("/usr/sbin/diskutil"), "apfs", "deleteVolume", uuid)
	removalGates := slices.Clone(ready)
	for i := range removalGates {
		if removalGates[i].Kind == "nix-consumers" {
			removalGates[i].Subject = ""
		}
	}
	scope.Steps = append(scope.Steps, legacyRetirementStep{Kind: "command", Label: "Permanently delete the Nix Store APFS volume " + uuid, Privileged: true, Irreversible: true, Command: &command,
		Gates:  removalGates,
		Checks: []legacyRetirementGate{{Kind: "volume", Subject: uuid, Expected: "absent"}}})
	if err := d.planDarwinNixIntegrations(ctx, scope, true); err != nil || scope.Blocked != "" {
		return err
	}
	// Volume identity binds dev:ino of /nix, which a restart can change.
	scope.Detail = append(scope.Detail, "If the volume is busy, restart the Mac, then abandon this operation and review a fresh retirement preview; an empty /nix folder disappears after the next restart.")
	return nil
}

// planDarwinNixIntegrations retires what the upstream daemon leaves once its
// volume is deleted: the volume password, the mount service, the boot-time
// fstab and synthetic.conf entries, root's Nix files, /etc/nix and the exact
// build accounts. In an approved deletion they follow deleteVolume; afterwards
// they are planned from what remains. The fstab entry outlives the password
// and mount service it identifies, and the build accounts go last because
// they remain the evidence that /etc/nix and root's files belong to Nix.
func (d *legacyRetirementDriver) planDarwinNixIntegrations(ctx context.Context, scope *legacyRetirementScope, afterDeletion bool) error {
	uuid, line, err := d.nixFstabLine(ctx)
	if err != nil {
		scope.Blocked = err.Error()
		return nil
	}
	service, uuid, reason, err := d.darwinStoreService(uuid)
	if err != nil {
		return err
	}
	if reason != "" {
		scope.Blocked = reason
		return nil
	}
	if uuid != "" && !afterDeletion {
		volume, err := d.volumeIdentity(ctx, uuid)
		if err != nil {
			return err
		}
		if volume != "absent" {
			scope.Blocked = "the Nix Store volume " + uuid + " exists but is not mounted at /nix; mount it or restart, then review retirement again"
			return nil
		}
	}
	if uuid != "" {
		// security exits 44 when the item is absent; only a present item is removed.
		keychain := d.sys("/Library/Keychains/System.keychain")
		_, err := d.Query(ctx, d.systemCommand(false, d.sys("/usr/bin/security"), "find-generic-password", "-s", uuid, "-a", "Nix Store", keychain))
		if code, ok := legacyExitCode(err); err != nil && (!ok || code != 44) {
			return err
		}
		if err == nil {
			command := d.systemCommand(true, d.sys("/usr/bin/security"), "delete-generic-password", "-s", uuid, "-a", "Nix Store", keychain)
			scope.Steps = append(scope.Steps, legacyRetirementStep{Kind: "command", Label: "Delete the retired Nix Store volume password from the System keychain", Privileged: true, Command: &command})
		}
	}
	if service {
		if state, err := d.launchdState(ctx, "system/org.nixos.darwin-store"); err != nil {
			return err
		} else if state == "loaded" {
			command := d.systemCommand(true, d.sys("/bin/launchctl"), "bootout", "system/org.nixos.darwin-store")
			scope.Steps = append(scope.Steps, legacyRetirementStep{Kind: "command", Label: "Stop the Nix Store mount service", Privileged: true, Command: &command,
				Gates:  []legacyRetirementGate{{Kind: "launchd", Subject: "system/org.nixos.darwin-store", Expected: "loaded"}},
				Checks: []legacyRetirementGate{{Kind: "launchd", Subject: "system/org.nixos.darwin-store", Expected: "absent"}}})
		}
		if err := addLegacyRemoval(scope, "Remove the Nix Store mount service", d.sys("/Library/LaunchDaemons/org.nixos.darwin-store.plist")); err != nil {
			return err
		}
	}
	if line != "" {
		if err := d.addLineRemoval(ctx, scope, "Stop mounting the Nix Store at boot", d.sys("/etc/fstab"), line); err != nil || scope.Blocked != "" {
			return err
		}
	}
	data, _, err := d.readSystemText(ctx, d.sys("/etc/synthetic.conf"))
	if err != nil {
		return err
	}
	synthetic := slices.Contains(strings.Split(string(data), "\n"), "nix")
	if synthetic {
		if err := d.addLineRemoval(ctx, scope, "Stop creating the /nix mount point", d.sys("/etc/synthetic.conf"), "nix"); err != nil || scope.Blocked != "" {
			return err
		}
	}
	accounts := legacyRetirementScope{}
	if present, err := d.nixBuildAccountsPresent(ctx); err != nil {
		return err
	} else if present {
		if err := d.planDarwinBuildAccounts(ctx, &accounts); err != nil {
			return err
		}
	}
	if accounts.Blocked != "" {
		scope.Blocked = accounts.Blocked
		return nil
	}
	// A lone synthetic.conf entry is removed but proves no daemon installation.
	if afterDeletion || uuid != "" || service || len(accounts.Steps) != 0 {
		if err := d.addRootFiles(scope, d.sys("/var/root")); err != nil {
			return err
		}
		if err := d.addSystemMove(scope, "Preserve the retired system Nix configuration", d.sys("/etc/nix")); err != nil {
			return err
		}
	}
	scope.Steps = append(scope.Steps, accounts.Steps...)
	return nil
}

// nix-homebrew replaced the Homebrew repository with store links and removed
// its Git checkout; installed kegs, casks and mutable taps stayed in place.
func (d *legacyRetirementDriver) inspectNixHomebrew(ctx context.Context, scope *legacyRetirementScope) error {
	location := d.Platform.Homebrew
	if d.Platform.OS != "darwin" || location == nil || LegacyNixHomebrewIssue(*location) == "" {
		return nil
	}
	prefix := location.Prefix
	brew, library := filepath.Join(prefix, "bin", "brew"), filepath.Join(prefix, "Library", "Homebrew")
	marker, repository := filepath.Join(prefix, ".managed_by_nix_darwin"), filepath.Join(prefix, "Library", ".homebrew-is-managed-by-nix")
	if location.Program != brew {
		scope.Blocked = "the active brew is not " + brew + "; inspect the Homebrew installation manually"
		return nil
	}
	if _, err := os.Lstat(filepath.Join(prefix, ".git")); err == nil {
		scope.Blocked = "a Homebrew repository already exists beside nix-homebrew markers; inspect it manually"
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	for _, link := range []string{brew, library} {
		if target, err := os.Readlink(link); err != nil || !strings.HasPrefix(target, "/nix/store/") {
			scope.Blocked = link + " is not a nix-homebrew store link; inspect it manually"
			return nil
		}
	}
	if info, err := os.Lstat(marker); err != nil || !info.Mode().IsRegular() {
		scope.Blocked = "the nix-homebrew ownership marker is missing; inspect the prefix manually"
		return nil
	}
	if info, err := os.Lstat(filepath.Join(repository, ".git", "HEAD")); err != nil || !info.Mode().IsRegular() {
		scope.Blocked = "the nix-homebrew placeholder repository is missing; inspect the prefix manually"
		return nil
	}
	// Repository conversion needs Apple's Git and native toolchain. Provisioning
	// missing Command Line Tools belongs to the reviewed Apple CLT provider.
	if _, err := d.Query(ctx, d.systemCommand(false, "/usr/bin/xcode-select", "-p")); err != nil {
		scope.Blocked = "Apple Command Line Tools are required before Homebrew conversion; install them (xcode-select --install), then review again"
		return nil
	}
	version, err := location.query(ctx, false, "brew", nil, "--version")
	if err != nil || !strings.HasPrefix(string(version), "Homebrew ") {
		return errors.Join(errors.New("the Nix-managed Homebrew does not run"), err)
	}
	inventory, err := d.brewPreservationIdentity(ctx)
	if err != nil {
		return err
	}
	scope.Native = strings.TrimSpace(strings.SplitN(string(version), "\n", 2)[0]) + " from nix-homebrew"
	for _, path := range []string{brew, library, marker} {
		if err := d.addUserMove(scope, "Move the nix-homebrew link or marker aside", path); err != nil {
			return err
		}
	}
	uid, _, _, err := legacyFileOwner(repository)
	if err != nil {
		return err
	}
	if uid == 0 {
		err = d.addSystemMove(scope, "Move the root-owned nix-homebrew placeholder repository aside", repository)
	} else {
		err = d.addUserMove(scope, "Move the nix-homebrew placeholder repository aside", repository)
	}
	if err != nil {
		return err
	}
	return d.planHomebrewRepository(scope, prefix, inventory)
}

func (d *legacyRetirementDriver) homebrewHealth(ctx context.Context) (string, error) {
	location := d.Platform.Homebrew
	if location == nil {
		return "Homebrew is not installed", nil
	}
	if issue := LegacyNixHomebrewIssue(*location); issue != "" {
		return issue, nil
	}
	if info, err := os.Lstat(location.Program); err != nil || !info.Mode().IsRegular() {
		return "brew is not a regular file in its prefix", nil
	}
	if info, err := os.Lstat(filepath.Join(location.Prefix, ".git")); err != nil || !info.IsDir() {
		return "Homebrew has no repository", nil
	}
	output, err := location.query(ctx, false, "brew", nil, "--version")
	if err != nil || !strings.HasPrefix(string(output), "Homebrew ") {
		return "brew --version failed", nil
	}
	for option, expected := range map[string]string{"--prefix": location.Prefix, "--cellar": location.Cellar} {
		data, err := location.query(ctx, false, "brew", nil, option)
		if err != nil || strings.TrimSuffix(string(data), "\n") != expected {
			return "brew " + option + " differs from the discovered location", nil
		}
	}
	origin, err := d.git(ctx, location.Prefix, "config", "--local", "--get", "remote.origin.url")
	if source := strings.TrimSpace(string(origin)); err != nil || source != "https://github.com/Homebrew/brew" && source != "https://github.com/Homebrew/brew.git" {
		return "the Homebrew repository is not the official source", nil
	}
	return "healthy", nil
}

func (d *legacyRetirementDriver) brewInventoryIdentity(ctx context.Context) (string, error) {
	if d.Platform.Homebrew == nil {
		return "", errors.New("Homebrew is not installed")
	}
	inventory, err := brewInventory(ctx, d.Platform.Homebrew.query)
	if err != nil {
		return "", err
	}
	return digest(inventory)
}

func (d *legacyRetirementDriver) brewFormula(ctx context.Context, name string) (string, error) {
	if d.Platform.Homebrew == nil {
		return "absent", nil
	}
	output, err := d.Platform.Homebrew.query(ctx, false, "brew", nil, "list", "--formula", "--versions", name)
	if code, ok := legacyExitCode(err); ok && code == 1 && len(bytes.TrimSpace(output)) == 0 {
		return "absent", nil
	}
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

// lsof's +f selects the whole mounted Nix filesystem. Keep warnings enabled
// and merge them into its parsed output so incomplete inspection cannot become
// an empty consumer list. See the upstream lsof manpage, +f and +w options.
const legacyDarwinProcessScript = `exec /usr/sbin/lsof -n +w -F pfn +f -- "$1" 2>&1`

func (d *legacyRetirementDriver) darwinNixProcesses(ctx context.Context, subject string) ([]string, error) {
	excluded, executable := "", ""
	if subject == "determinate" {
		var err error
		if excluded, err = d.determinateDaemonProcess(ctx); err != nil {
			return nil, err
		}
	}
	if subject == "daemon" {
		service := "system/org.nixos.nix-daemon"
		state, err := d.launchdState(ctx, service)
		if err != nil {
			return nil, err
		}
		if state == "loaded" {
			output, err := d.Query(ctx, d.systemCommand(false, d.sys("/bin/launchctl"), "print", service))
			if err != nil {
				return nil, err
			}
			for _, line := range strings.Split(string(output), "\n") {
				if value, ok := strings.CutPrefix(strings.TrimSpace(line), "pid = "); ok {
					pid, err := strconv.ParseUint(value, 10, 32)
					if err != nil || pid == 0 || excluded != "" {
						return nil, errors.New("the Nix launchd service has an invalid process identity")
					}
					excluded = value
				}
			}
			if excluded != "" {
				executable, err = filepath.EvalSymlinks(d.sys("/nix/var/nix/profiles/default/bin/nix-daemon"))
				if err != nil || !strings.HasPrefix(executable, d.sys("/nix/store")+"/") {
					return nil, errors.Join(errors.New("the Nix daemon executable cannot be proved"), err)
				}
			}
		}
	}
	output, err := d.privilegedQuery(ctx, d.systemCommand(true, "/bin/sh", "-c", legacyDarwinProcessScript, "sh", d.sys("/nix")))
	if code, ok := legacyExitCode(err); ok && code == 1 && len(bytes.TrimSpace(output)) == 0 {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return legacyDarwinProcessConsumers(output, excluded, executable, subject == "determinate")
}

func legacyDarwinProcessConsumers(output []byte, excluded, executable string, provedVendorDaemon bool) ([]string, error) {
	names := map[string][]string{}
	daemon := map[string]bool{}
	pid, descriptor := "", ""
	for _, line := range strings.Split(strings.TrimSuffix(string(output), "\n"), "\n") {
		switch {
		case strings.HasPrefix(line, "p"):
			value, err := strconv.ParseUint(line[1:], 10, 32)
			if err != nil || value == 0 {
				return nil, errors.New("lsof returned an invalid process identity")
			}
			pid, descriptor = line[1:], ""
			if _, ok := names[pid]; !ok {
				names[pid] = nil
			}
		case strings.HasPrefix(line, "f") && len(line) > 1 && pid != "":
			descriptor = line[1:]
		case strings.HasPrefix(line, "n/nix/") && pid != "" && descriptor != "":
			names[pid] = append(names[pid], line[1:])
			if pid == excluded && descriptor == "txt" && line[1:] == executable {
				daemon[pid] = true
			}
		case line == "n/nix" && pid != "" && descriptor != "":
			names[pid] = append(names[pid], "/nix")
		default:
			return nil, errors.New("lsof reported an inspection warning or unrecognized file record; Nix removal is refused")
		}
	}
	problems := []string{}
	for _, pid := range slices.Sorted(maps.Keys(names)) {
		if len(names[pid]) == 0 {
			return nil, errors.New("lsof returned a process without inspected files")
		}
		if !daemon[pid] && !(provedVendorDaemon && excluded != "" && pid == excluded) {
			problems = append(problems, "process "+pid+" uses "+truncateLegacyText(names[pid][0], 256))
		}
	}
	return problems, nil
}
