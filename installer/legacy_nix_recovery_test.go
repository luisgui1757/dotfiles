package installer

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// These fixtures relocate the native system tree into a disposable directory.
// Files, links, rm, mv, flock and the edit/root-file scripts are real. Account,
// systemd, launchd, volume and keychain commands cross the process boundary
// into a recorded host database, persisted with the native worker's record
// format. Nothing outside the temporary directory is changed.

const legacyNixFixtureVolume = "11111111-2222-3333-4444-555555555555"

type legacyNixHost struct {
	t                 *testing.T
	d                 *legacyRetirementDriver
	base, root, home  string
	passwd            map[string]string
	groups            map[string]string
	darwinUsers       map[string]map[string]string
	darwinGroups      map[string]map[string]string
	launchd           map[string]bool
	volumes           map[string]string
	keychain          map[string]bool
	unitsLoaded, busy bool
	// die simulates a worker killed while the selected command runs.
	die func(nativeCommand) bool
}

func newLegacyNixHost(t *testing.T, goos string) *legacyNixHost {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("POSIX native retirement fixture")
	}
	if goos == "linux" && runtime.GOOS != "linux" {
		t.Skip("Linux recovery executes native util-linux flock and GNU rm; macOS has its own volume fixtures")
	}
	base, err := resolveConfigPath(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	h := &legacyNixHost{t: t, base: base, root: filepath.Join(base, "system"), home: filepath.Join(base, "home"), passwd: map[string]string{}, groups: map[string]string{},
		darwinUsers: map[string]map[string]string{}, darwinGroups: map[string]map[string]string{}, launchd: map[string]bool{}, volumes: map[string]string{}, keychain: map[string]bool{}}
	resources := []Resource{}
	for _, id := range []string{"tool.rg", "tool.fd", "tool.fzf", "tool.jq", "tool.lazygit", "tool.node", "tool.starship", "tool.zoxide", "tool.clangd"} {
		resources = append(resources, Resource{ID: id, Name: id, Capability: true, Action: "file"})
	}
	catalog := &Catalog{Schema: 1, Resources: resources}
	if err := catalog.Validate(); err != nil {
		t.Fatal(err)
	}
	state := filepath.Join(h.home, ".local", "state", "dotfiles")
	normalDriver := &fileDriver{dir: filepath.Join(base, "normal"), home: h.home, statePath: filepath.Join(state, "state.json")}
	if err := os.MkdirAll(normalDriver.dir, 0700); err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(context.Background(), catalog, linux, "source", h.home, normalDriver.statePath, approve(t, catalog, normalDriver, Request{Schema: 1, Mode: "apply", Selected: []string{}}), normalDriver); err != nil {
		t.Fatal(err)
	}
	arch := "amd64"
	if goos == "darwin" {
		arch = "arm64"
	}
	h.d = &legacyRetirementDriver{Directory: filepath.Join(state, "migration", "retirement"), Repository: filepath.Join(base, "repository"),
		Normal:   Controller{Catalog: catalog, Context: linux, Source: "source", Home: h.home, StatePath: normalDriver.statePath, Driver: normalDriver},
		Platform: NativePlatform{Context: Context{OS: goos, Arch: arch}}, Folders: ConfigFolders{Home: h.home},
		Environment: legacyRuntimeEnvironment{Executable: filepath.Join(base, "bin", "dotfiles"), Username: "fixture", UID: strconv.Itoa(os.Getuid()),
			Cache: filepath.Join(h.home, ".cache"), Root: true, Programs: map[string]string{}},
		WorkerDirectory: filepath.Join(state, "native", "worker"), systemRoot: h.root}
	h.d.Query, h.d.Run = h.query, h.run
	for _, program := range []string{"/usr/bin/ps", "/usr/bin/systemctl", "/usr/sbin/userdel", "/usr/sbin/groupdel"} {
		h.file(program, "#!/bin/sh\nexit 99\n")
	}
	if goos == "linux" {
		flock, err := exec.LookPath("flock")
		if err != nil {
			t.Fatal("the Linux contributor gate requires util-linux flock:", err)
		}
		h.link("/usr/bin/flock", flock)
	}
	h.file("/proc/self/mountinfo", "22 1 8:1 / / rw,relatime - ext4 /dev/root rw\n")
	return h
}

func (h *legacyNixHost) path(path string) string { return filepath.Join(h.root, path) }

func (h *legacyNixHost) file(path, content string) {
	h.t.Helper()
	writeConfigFixture(h.t, h.path(path), content)
}

func (h *legacyNixHost) link(path, target string) {
	h.t.Helper()
	if !filepath.IsAbs(path) || strings.HasPrefix(path, h.base) {
		h.t.Fatal("fixture links are named relative to the system root", path)
	}
	h.linkAt(h.path(path), target)
}

func (h *legacyNixHost) linkAt(path, target string) {
	h.t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		h.t.Fatal(err)
	}
	if err := os.Symlink(target, path); err != nil {
		h.t.Fatal(err)
	}
}

func legacyFakeArgv(command nativeCommand) []string {
	argv := append([]string{command.Program}, command.Arguments...)
	if argv[0] == "/usr/bin/sudo" {
		argv = argv[3:]
	}
	if argv[0] == "/usr/bin/env" {
		argv = argv[1:]
		for len(argv) > 0 && !strings.HasPrefix(argv[0], "/") {
			argv = argv[1:]
		}
	}
	return argv
}

func legacyFakeExit(code int, message string) error {
	return &nativeCommandError{Message: message, ExitCode: &code}
}

func (h *legacyNixHost) query(ctx context.Context, command nativeCommand) ([]byte, error) {
	argv := legacyFakeArgv(command)
	args := strings.Join(argv[1:], " ")
	switch filepath.Base(argv[0]) {
	case "getent":
		table := h.passwd
		if argv[1] == "group" {
			table = h.groups
		}
		if len(argv) == 2 {
			lines := []string{}
			for _, name := range slices.Sorted(maps.Keys(table)) {
				lines = append(lines, table[name])
			}
			return []byte(strings.Join(lines, "\n") + "\n"), nil
		}
		if line, ok := table[argv[2]]; ok {
			return []byte(line + "\n"), nil
		}
		return nil, legacyFakeExit(2, "getent: not found")
	case "ps":
		return nil, nil
	case "systemctl":
		switch {
		case strings.HasPrefix(args, "show --property=ActiveState --value -- "):
			return []byte("inactive\n"), nil
		case args == "show --property=MainPID --value nix-daemon.service":
			return []byte("0\n"), nil
		case strings.HasPrefix(args, "show --property=NeedDaemonReload --value "):
			need := "no"
			if h.unitsLoaded && !h.unitLinksPresent() {
				need = "yes"
			}
			return []byte(strings.Repeat(need+"\n", len(argv)-4)), nil
		}
	case "nix":
		if _, err := os.Stat(argv[0]); err != nil {
			return nil, legacyFakeExit(127, "nix: "+err.Error())
		}
		return []byte(legacyNixVersion + "\n"), nil
	case "dscl":
		return h.dscl(argv[2:])
	case "diskutil":
		if len(argv) == 4 && argv[1] == "info" && argv[2] == "-plist" {
			name, ok := h.volumes[argv[3]]
			if !ok {
				return nil, legacyFakeExit(1, "Could not find disk: "+argv[3])
			}
			return []byte(`<?xml version="1.0" encoding="UTF-8"?><plist version="1.0"><dict><key>FilesystemType</key><string>apfs</string><key>VolumeName</key><string>` +
				name + `</string><key>VolumeUUID</key><string>` + argv[3] + `</string></dict></plist>`), nil
		}
	case "launchctl":
		if len(argv) == 3 && argv[1] == "print" {
			if h.launchd[argv[2]] {
				return []byte("state = waiting\n"), nil
			}
			return nil, legacyFakeExit(113, "Could not find service")
		}
	case "security":
		if argv[1] == "find-generic-password" {
			if h.keychain[argv[3]] {
				return nil, nil
			}
			return nil, legacyFakeExit(44, "The specified item could not be found in the keychain.")
		}
	case "sh":
		switch {
		case slices.Contains(argv, legacyLinuxProcessScript):
			h.requireLeaseReleased(ctx)
			return nil, nil
		case slices.Contains(argv, legacyDarwinProcessScript):
			return nil, legacyFakeExit(1, "")
		}
		return queryLegacyRuntime(ctx, command)
	}
	h.t.Fatalf("unexpected native query %q", argv)
	return nil, nil
}

// The kernel process scan reports open descriptors under /nix, so no probe
// or deletion of this controller may still hold the root's lease when it runs.
func (h *legacyNixHost) requireLeaseReleased(ctx context.Context) {
	h.t.Helper()
	if _, err := os.Lstat(h.path("/nix")); errors.Is(err, os.ErrNotExist) {
		return
	}
	probe := nativeCommand{Program: "/bin/sh", Arguments: []string{"-c", `exec 9<"$1"; exec /usr/bin/flock -n -E 86 9`, "sh", h.path("/nix")}}
	if _, err := queryLegacyRuntime(ctx, probe); err != nil {
		h.t.Fatal("the consumer scan ran while the Nix root lease was held:", err)
	}
}

func (h *legacyNixHost) dscl(argv []string) ([]byte, error) {
	switch {
	case len(argv) == 3 && argv[0] == "-list" && argv[1] == "/Users":
		lines := []string{}
		for _, name := range slices.Sorted(maps.Keys(h.darwinUsers)) {
			if value, ok := h.darwinUsers[name][argv[2]]; ok {
				lines = append(lines, name+" "+value)
			}
		}
		return []byte(strings.Join(lines, "\n") + "\n"), nil
	case len(argv) == 3 && argv[0] == "-list" && argv[1] == "/Groups":
		lines := []string{}
		for _, name := range slices.Sorted(maps.Keys(h.darwinGroups)) {
			lines = append(lines, name+" "+h.darwinGroups[name]["PrimaryGroupID"])
		}
		return []byte(strings.Join(lines, "\n") + "\n"), nil
	case len(argv) >= 2 && argv[0] == "-read":
		table := h.darwinUsers
		name, ok := strings.CutPrefix(argv[1], "/Users/")
		if !ok {
			table, name = h.darwinGroups, strings.TrimPrefix(argv[1], "/Groups/")
		}
		record, ok := table[name]
		if !ok {
			return nil, legacyFakeExit(56, "eDSRecordNotFound")
		}
		lines := []string{}
		for _, key := range slices.Sorted(maps.Keys(record)) {
			if len(argv) == 3 && key != argv[2] {
				continue
			}
			lines = append(lines, key+": "+record[key])
		}
		return []byte(strings.Join(lines, "\n") + "\n"), nil
	}
	return nil, legacyFakeExit(64, "unexpected dscl query")
}

func (h *legacyNixHost) unitLinksPresent() bool {
	for _, path := range []string{"/etc/systemd/system/nix-daemon.service", "/etc/systemd/system/nix-daemon.socket", "/etc/systemd/system/sockets.target.wants/nix-daemon.socket"} {
		if _, err := os.Lstat(h.path(path)); err == nil {
			return true
		}
	}
	return false
}

// run follows the worker contract: persist intent, execute, persist the reply.
func (h *legacyNixHost) run(_ context.Context, command nativeCommand) ([]byte, error) {
	argv := legacyFakeArgv(command)
	if h.die != nil && h.die(command) {
		h.die = nil
		hash, err := digest(command)
		if err != nil {
			return nil, err
		}
		if err := saveDocument(filepath.Join(h.d.WorkerDirectory, "commands", command.Operation+".json"), nativeCommandRecord{Schema: 1, Command: hash}); err != nil {
			return nil, err
		}
		h.partialDeletion()
		return nil, errors.New("the native worker was killed before it replied")
	}
	effect := h.effect(argv)
	if effect == nil {
		reply, err := executeNativeCommand(h.d.WorkerDirectory, command)
		if err != nil {
			return nil, err
		}
		if !legacyReplySucceeded(reply) {
			return reply.Output, &nativeCommandError{Message: reply.Error, ExitCode: reply.ExitCode}
		}
		return reply.Output, nil
	}
	hash, err := digest(command)
	if err != nil {
		return nil, err
	}
	path := filepath.Join(h.d.WorkerDirectory, "commands", command.Operation+".json")
	if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
		h.t.Fatal("a recorded native command was dispatched again", argv, err)
	}
	if err := saveDocument(path, nativeCommandRecord{Schema: 1, Command: hash}); err != nil {
		return nil, err
	}
	code, message := effect()
	reply := nativeReply{ExitCode: &code}
	if code != 0 {
		reply.Error = message
	}
	if err := saveDocument(path, nativeCommandRecord{Schema: 1, Command: hash, Reply: &reply}); err != nil {
		return nil, err
	}
	if code != 0 {
		return nil, legacyFakeExit(code, message)
	}
	return nil, nil
}

func (h *legacyNixHost) effect(argv []string) func() (int, string) {
	switch filepath.Base(argv[0]) {
	case "chsh":
		return func() (int, string) {
			fields := strings.Split(h.passwd[argv[3]], ":")
			fields[6] = argv[2]
			h.passwd[argv[3]] = strings.Join(fields, ":")
			return 0, ""
		}
	case "userdel":
		return func() (int, string) { delete(h.passwd, argv[1]); return 0, "" }
	case "groupdel":
		return func() (int, string) { delete(h.groups, argv[1]); return 0, "" }
	case "systemctl":
		return func() (int, string) {
			if argv[1] == "stop" && !h.unitsLoaded {
				return 5, "Unit not loaded"
			}
			if argv[1] == "daemon-reload" {
				h.unitsLoaded = h.unitLinksPresent()
			}
			return 0, ""
		}
	case "dscl":
		return func() (int, string) {
			switch {
			case argv[2] == "-create":
				h.darwinUsers[strings.TrimPrefix(argv[3], "/Users/")][argv[4]] = argv[5]
			case strings.HasPrefix(argv[3], "/Users/"):
				delete(h.darwinUsers, strings.TrimPrefix(argv[3], "/Users/"))
			default:
				delete(h.darwinGroups, strings.TrimPrefix(argv[3], "/Groups/"))
			}
			return 0, ""
		}
	case "launchctl":
		return func() (int, string) { h.launchd[argv[2]] = false; return 0, "" }
	case "security":
		return func() (int, string) { delete(h.keychain, argv[3]); return 0, "" }
	case "diskutil":
		return func() (int, string) {
			if h.busy {
				return 1, "Volume on disk3s7 failed to unmount: dissented by PID 42"
			}
			delete(h.volumes, argv[3])
			entries, err := os.ReadDir(h.path("/nix"))
			if err != nil {
				return 1, err.Error()
			}
			for _, entry := range entries {
				if err := os.RemoveAll(filepath.Join(h.path("/nix"), entry.Name())); err != nil {
					return 1, err.Error()
				}
			}
			return 0, ""
		}
	}
	return nil
}

// partialDeletion leaves what a killed rm leaves: some store paths and the
// Nix binary are gone, others and the runtime state remain.
func (h *legacyNixHost) partialDeletion() {
	h.t.Helper()
	for _, path := range []string{"/nix/store/aaa-nix-2.34.0", "/nix/store/bbb-bash-5.2"} {
		if err := os.RemoveAll(h.path(path)); err != nil {
			h.t.Fatal(err)
		}
	}
	if err := os.Remove(h.path("/nix/store/ccc-home-manager-generation/activate")); err != nil {
		h.t.Fatal(err)
	}
}

// installLinuxUser writes the released single-user layout with Home Manager.
func (h *legacyNixHost) installLinuxUser() {
	uid, gid := strconv.Itoa(os.Getuid()), strconv.Itoa(os.Getgid())
	store := h.path("/nix/store")
	h.passwd["root"] = "root:x:0:0:root:" + h.path("/root") + ":/bin/bash"
	h.passwd["fixture"] = "fixture:x:" + uid + ":" + gid + "::" + h.home + ":" + store + "/bbb-bash-5.2/bin/bash"
	h.installLinuxUserRoot()
	state := filepath.Join(h.home, ".local", "state")
	h.linkAt(filepath.Join(state, "nix", "profiles", "profile-1-link"), store+"/fff-user-environment")
	h.linkAt(filepath.Join(state, "nix", "profiles", "profile"), "profile-1-link")
	h.linkAt(filepath.Join(h.home, ".nix-profile"), filepath.Join(state, "nix", "profiles", "profile"))
	h.linkAt(filepath.Join(state, "home-manager", "gcroots", "current-home"), store+"/ccc-home-manager-generation")
	h.linkAt(filepath.Join(h.home, ".config", "fixture", "config"), store+"/eee-home-manager-files/.config/fixture/config")
	writeConfigFixture(h.t, filepath.Join(h.home, ".nix-defexpr", "channels"), "personal\n")
	writeConfigFixture(h.t, filepath.Join(h.home, ".cache", "nix", "fetcher-cache-v1.sqlite"), "cache\n")
	h.file("/bin/bash", "system bash\n")
	h.file("/etc/shells", "/bin/sh\n"+h.path("/bin/bash")+"\n"+store+"/bbb-bash-5.2/bin/bash\n")
	h.file("/etc/passwd", h.passwd["root"]+"\n"+h.passwd["fixture"]+"\n")
}

// installLinuxUserRoot writes the single-user Nix root with Home Manager.
func (h *legacyNixHost) installLinuxUserRoot() {
	store := h.path("/nix/store")
	h.file("/nix/store/aaa-nix-2.34.0/bin/nix", "nix\n")
	h.file("/nix/store/bbb-bash-5.2/bin/bash", "bash\n")
	h.file("/nix/store/ccc-home-manager-generation/activate", "#!"+store+"/bbb-bash-5.2/bin/bash\n")
	h.link("/nix/store/ccc-home-manager-generation/home-files", store+"/eee-home-manager-files")
	h.file("/nix/store/eee-home-manager-files/.config/fixture/config", "managed\n")
	h.link("/nix/store/fff-user-environment/bin/nix", store+"/aaa-nix-2.34.0/bin/nix")
	h.file("/nix/var/nix/db/db.sqlite", "db\n")
	if err := os.MkdirAll(h.path("/nix/var/nix/profiles/per-user"), 0755); err != nil {
		h.t.Fatal(err)
	}
	h.link("/nix/var/nix/gcroots/auto/fixture", filepath.Join(h.home, ".local", "state", "home-manager", "gcroots", "current-home"))
}

// installLinuxDaemonIntegrations writes the inert system integrations of the
// released daemon layout that outlive its store.
func (h *legacyNixHost) installLinuxDaemonIntegrations() {
	profile := h.path("/nix/var/nix/profiles/default/lib")
	h.link("/etc/systemd/system/nix-daemon.service", profile+"/systemd/system/nix-daemon.service")
	h.link("/etc/systemd/system/nix-daemon.socket", profile+"/systemd/system/nix-daemon.socket")
	h.link("/etc/systemd/system/sockets.target.wants/nix-daemon.socket", h.path("/etc/systemd/system/nix-daemon.socket"))
	h.link("/etc/systemd/system/multi-user.target.wants/nix-daemon.service", h.path("/etc/systemd/system/nix-daemon.service"))
	h.link("/etc/tmpfiles.d/nix-daemon.conf", profile+"/tmpfiles.d/nix-daemon.conf")
	h.file("/etc/systemd/system/nix-daemon.service.d/override.conf", "[Service]\nEnvironment=\"https_proxy=http://proxy.invalid\"\n")
	h.file("/etc/nix/nix.conf", "build-users-group = nixbld\n")
	h.link("/root/.nix-profile", h.path("/nix/var/nix/profiles/per-user/root/profile"))
	h.file("/root/.cache/nix/fetcher-cache-v1.sqlite", "root cache\n")
	h.passwd["root"] = "root:x:0:0:root:" + h.path("/root") + ":/bin/bash"
	for index := 1; index <= 2; index++ {
		name := fmt.Sprintf("nixbld%d", index)
		h.passwd[name] = fmt.Sprintf("%s:x:%d:30000:Nix build user %d:/var/empty:/sbin/nologin", name, 30000+index, index)
	}
	h.groups["nixbld"] = "nixbld:x:30000:nixbld1,nixbld2"
	h.unitsLoaded = true
}

const legacyNixDaemonPlist = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
	<dict>
		<key>Label</key>
		<string>org.nixos.nix-daemon</string>
		<key>KeepAlive</key>
		<true/>
		<key>RunAtLoad</key>
		<true/>
		<key>ProgramArguments</key>
		<array>
			<string>/bin/sh</string>
			<string>-c</string>
			<string>/bin/wait4path /nix/var/nix/profiles/default/bin/nix-daemon &amp;&amp; exec /nix/var/nix/profiles/default/bin/nix-daemon</string>
		</array>
		<key>StandardErrorPath</key>
		<string>/var/log/nix-daemon.log</string>
		<key>StandardOutPath</key>
		<string>/dev/null</string>
		<key>SoftResourceLimits</key>
		<dict>
			<key>NumberOfFiles</key>
			<integer>1048576</integer>
		</dict>
	</dict>
</plist>
`

// legacyNixStorePlist reproduces create-darwin-volume.sh generate_mount_daemon
// from Nix 2.34.0 for both of its mount commands.
func legacyNixStorePlist(encrypted bool) string {
	arguments := []string{"/usr/sbin/diskutil", "mount", "-mountPoint", "/nix", legacyNixFixtureVolume}
	if encrypted {
		arguments = []string{"/bin/sh", "-c", "/usr/bin/security find-generic-password -s '" + legacyNixFixtureVolume + "' -w | /usr/sbin/diskutil apfs unlockVolume '" +
			legacyNixFixtureVolume + "' -mountpoint '/nix' -stdinpassphrase"}
	}
	lines := ""
	for _, argument := range arguments {
		lines += "    <string>" + argument + "</string>\n"
	}
	return `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple Computer//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>RunAtLoad</key>
	<true/>
	<key>Label</key>
	<string>org.nixos.darwin-store</string>
	<key>ProgramArguments</key>
	<array>
` + lines + `  </array>
</dict>
</plist>
`
}

// installDarwin writes the released macOS layout: upstream daemon on its APFS
// volume, nix-darwin with a displaced /etc file, and hidden build accounts.
func (h *legacyNixHost) installDarwin(encrypted bool) {
	store := h.path("/nix/store")
	h.darwinUsers["fixture"] = map[string]string{"UniqueID": "501", "PrimaryGroupID": "20", "UserShell": store + "/bbb-zsh-5.9/bin/zsh"}
	h.darwinUsers["root"] = map[string]string{"UniqueID": "0", "PrimaryGroupID": "0", "UserShell": "/bin/sh"}
	for index := 1; index <= 2; index++ {
		h.darwinUsers[fmt.Sprintf("_nixbld%d", index)] = map[string]string{"UniqueID": strconv.Itoa(350 + index), "PrimaryGroupID": "350", "RealName": fmt.Sprintf("Nix build user %d", index),
			"NFSHomeDirectory": "/var/empty", "UserShell": "/sbin/nologin", "IsHidden": "1"}
	}
	h.darwinGroups["nixbld"] = map[string]string{"PrimaryGroupID": "350", "GroupMembership": "_nixbld1 _nixbld2"}
	h.file("/nix/store/aaa-nix-2.34.0/bin/nix", "nix\n")
	h.file("/nix/store/aaa-nix-2.34.0/Library/LaunchDaemons/org.nixos.nix-daemon.plist", legacyNixDaemonPlist)
	h.file("/nix/store/bbb-zsh-5.9/bin/zsh", "zsh\n")
	h.link("/nix/var/nix/profiles/default-1-link", store+"/aaa-nix-2.34.0")
	h.link("/nix/var/nix/profiles/default", "default-1-link")
	if err := os.MkdirAll(h.path("/nix/var/nix/profiles/per-user/fixture"), 0755); err != nil {
		h.t.Fatal(err)
	}
	h.file("/nix/store/iii-zshrc", "# nix-darwin zshrc\n")
	h.link("/nix/store/hhh-etc/zshrc", store+"/iii-zshrc")
	h.file("/nix/store/ggg-darwin-system-26.05/Library/LaunchDaemons/org.nixos.activate-system.plist", activateSystemPlist)
	h.link("/nix/store/ggg-darwin-system-26.05/etc", store+"/hhh-etc")
	h.link("/run/current-system", store+"/ggg-darwin-system-26.05")
	h.link("/etc/static", store+"/hhh-etc")
	h.link("/etc/zshrc", h.path("/etc/static/zshrc"))
	h.file("/etc/zshrc.before-nix-darwin", "# original system zshrc\n")
	h.file("/Library/LaunchDaemons/org.nixos.activate-system.plist", activateSystemPlist)
	h.file("/Library/LaunchDaemons/org.nixos.nix-daemon.plist", legacyNixDaemonPlist)
	h.file("/Library/LaunchDaemons/org.nixos.darwin-store.plist", legacyNixStorePlist(encrypted))
	h.launchd["system/org.nixos.activate-system"] = true
	h.launchd["system/org.nixos.nix-daemon"] = true
	h.launchd["system/org.nixos.darwin-store"] = true
	h.volumes[legacyNixFixtureVolume] = "Nix Store"
	h.keychain[legacyNixFixtureVolume] = encrypted
	h.file("/etc/fstab", "# personal\nUUID="+legacyNixFixtureVolume+" /nix apfs rw,noauto,nobrowse,nosuid,noatime,owners\n")
	h.file("/etc/synthetic.conf", "nix\n")
	h.file("/etc/nix/nix.conf", "build-users-group = nixbld\n")
	h.file("/var/root/.cache/nix/fetcher-cache-v1.sqlite", "root cache\n")
	h.file("/bin/zsh", "system zsh\n")
	h.file("/etc/shells", "/bin/sh\n"+h.path("/bin/zsh")+"\n")
	h.linkAt(filepath.Join(h.home, ".nix-profile"), h.path("/nix/var/nix/profiles/per-user/fixture/profile"))
}

const activateSystemPlist = `<?xml version="1.0" encoding="UTF-8"?>
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>org.nixos.activate-system</string>
</dict>
</plist>
`

// unmountVolume models a restart after which the volume was not mounted.
func (h *legacyNixHost) unmountVolume() func() {
	h.t.Helper()
	held := filepath.Join(h.base, "unmounted-volume")
	if err := os.Rename(h.path("/nix"), held); err != nil {
		h.t.Fatal(err)
	}
	if err := os.Mkdir(h.path("/nix"), 0755); err != nil {
		h.t.Fatal(err)
	}
	return func() {
		h.t.Helper()
		if err := os.Remove(h.path("/nix")); err != nil {
			h.t.Fatal(err)
		}
		if err := os.Rename(held, h.path("/nix")); err != nil {
			h.t.Fatal(err)
		}
	}
}

func (h *legacyNixHost) inspect() legacyRetirementScope {
	h.t.Helper()
	scope, err := h.d.inspect(context.Background(), "retire.nix")
	if err != nil {
		h.t.Fatal("retirement inspection failed:", err)
	}
	return scope
}

// approved saves an operation journal for an inspected scope, as Apply does.
func (h *legacyNixHost) approved(scope legacyRetirementScope) *legacyRetirementIntent {
	h.t.Helper()
	if scope.Blocked != "" || len(scope.Steps) == 0 {
		h.t.Fatalf("expected an approvable retirement, found %q with %d steps", scope.Blocked, len(scope.Steps))
	}
	desired, err := digest(scope)
	if err != nil {
		h.t.Fatal(err)
	}
	operation, err := digest(struct{ Fixture, Scope string }{h.t.Name(), desired})
	if err != nil {
		h.t.Fatal(err)
	}
	intent := &legacyRetirementIntent{Schema: 1, Resource: "retire.nix", Operation: operation, Scope: scope, Attempts: make([]int, len(scope.Steps)),
		Before: Observation{Provider: "legacy-retirement", Identity: "retire.nix", Scope: "machine", Desired: desired}}
	if err := saveDocument(h.d.intentPath(operation), intent); err != nil {
		h.t.Fatal(err)
	}
	return intent
}

func (h *legacyNixHost) execute(intent *legacyRetirementIntent, recovery bool) error {
	h.t.Helper()
	if err := h.d.executeStep(context.Background(), intent, intent.Next, recovery); err != nil {
		return err
	}
	intent.Next++
	return saveDocument(h.d.intentPath(intent.Operation), intent)
}

func legacyStepLabels(steps []legacyRetirementStep) []string {
	labels := []string{}
	for _, step := range steps {
		labels = append(labels, step.Label)
	}
	return labels
}

// Steps whose effect the unprivileged controller cannot observe, or which are
// repeated harmlessly while the store remains, are replanned. Everything else
// must disappear exactly when it is done.
func legacyReplannedStep(label string) bool {
	return strings.HasPrefix(label, "Preserve root's Nix file if present") || label == "Stop the Nix daemon"
}

// requireRemaining proves prefix closure: after any completed prefix, a fresh
// inspection plans exactly the unfinished approved steps, in order.
func (h *legacyNixHost) requireRemaining(want []legacyRetirementStep) {
	h.t.Helper()
	scope := h.inspect()
	if scope.Blocked != "" {
		h.t.Fatalf("a completed prefix destroyed its own classification evidence: %s\nremaining approved steps: %q", scope.Blocked, legacyStepLabels(want))
	}
	got, expected := legacyStepLabels(scope.Steps), legacyStepLabels(want)
	next := 0
	for _, label := range got {
		if next < len(expected) && label == expected[next] {
			next++
			continue
		}
		if !legacyReplannedStep(label) {
			h.t.Fatalf("fresh inspection planned %q outside the remaining approved steps\ngot:  %q\nwant: %q", label, got, expected)
		}
	}
	for ; next < len(expected); next++ {
		if !legacyReplannedStep(expected[next]) {
			h.t.Fatalf("fresh inspection lost remaining step %q\ngot:  %q\nwant: %q", expected[next], got, expected)
		}
	}
}

// runPrefixes executes an approved scope one step at a time and reinspects
// after every completed prefix.
func (h *legacyNixHost) runPrefixes(intent *legacyRetirementIntent, stop func(legacyRetirementStep) bool) {
	h.t.Helper()
	for intent.Next < len(intent.Scope.Steps) {
		step := intent.Scope.Steps[intent.Next]
		if stop != nil && stop(step) {
			return
		}
		if err := h.execute(intent, false); err != nil {
			h.t.Fatalf("%s: %v", step.Label, err)
		}
		h.requireRemaining(intent.Scope.Steps[intent.Next:])
	}
}

func legacyIrreversible(step legacyRetirementStep) bool { return step.Irreversible }

func requireLegacyNixStoreIntact(t *testing.T, h *legacyNixHost) {
	t.Helper()
	for _, path := range []string{"/nix/store/aaa-nix-2.34.0/bin/nix", "/nix/store/bbb-bash-5.2/bin/bash", "/nix/var/nix/db/db.sqlite"} {
		if _, err := os.Lstat(h.path(path)); err != nil {
			t.Fatal("the store changed while another deletion held its lease:", path, err)
		}
	}
}

func TestLegacyNixSingleUserEveryPrefixIsRediscoverable(t *testing.T) {
	h := newLegacyNixHost(t, "linux")
	h.installLinuxUser()
	first := h.approved(h.inspect())
	h.runPrefixes(first, legacyIrreversible)
	deletion := first.Scope.Steps[first.Next]
	command, err := h.d.stepCommand(*first, first.Next)
	if err != nil {
		t.Fatal(err)
	}
	// The worker is killed while rm runs: part of the store is gone and the
	// native result is unknown. It must not be resumed or replayed blindly.
	h.die = func(dispatched nativeCommand) bool { return dispatched.Operation == command.Operation }
	if err := h.execute(first, false); err == nil {
		t.Fatal("a killed worker reported success")
	}
	if o, err := h.d.observeInterrupted(context.Background(), Observation{}, *first); err != nil || o.ResourceResume != nil || !strings.Contains(o.Pending, "unknown") {
		t.Fatal("an unknown deletion outcome received recovery approval", o, err)
	}
	// A fresh, separately approved scope finishes only the approved members.
	recovery := h.inspect()
	if recovery.Blocked != "" || len(recovery.Steps) == 0 || !recovery.Steps[0].Irreversible || !strings.Contains(strings.Join(recovery.Detail, "\n"), first.Operation) {
		t.Fatalf("the partially deleted store has no truthful recovery scope: blocked=%q steps=%q detail=%q", recovery.Blocked, legacyStepLabels(recovery.Steps), recovery.Detail)
	}
	if want := first.Scope.Steps[first.Next+1:]; !slices.Equal(legacyStepLabels(recovery.Steps[1:]), legacyStepLabels(want)) {
		t.Fatalf("recovery lost the remaining approved steps after %q\ngot:  %q\nwant: %q", deletion.Label, legacyStepLabels(recovery.Steps[1:]), legacyStepLabels(want))
	}
	second := h.approved(recovery)
	h.runPrefixes(second, nil)
	if scope := h.inspect(); scope.Blocked != "" || len(scope.Steps) != 0 {
		t.Fatal("retirement did not converge", scope.Blocked, legacyStepLabels(scope.Steps))
	}
	if _, err := os.Lstat(h.path("/nix")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("the store remains", err)
	}
	if data, err := os.ReadFile(h.path("/etc/shells")); err != nil || strings.Contains(string(data), "/nix/store/") || !strings.Contains(string(data), h.path("/bin/bash")) {
		t.Fatal("the retired Nix shell is still registered or the system shell was lost", string(data), err)
	}
}

func TestLegacyNixFailedDeletionResumesTheApprovedSubset(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores the permission failure this fixture relies on")
	}
	h := newLegacyNixHost(t, "linux")
	h.installLinuxUser()
	// More than one membership chunk; a later chunk carries approval evidence.
	for index := 0; index < 4200; index++ {
		h.file(fmt.Sprintf("/nix/store/%05d-fixture", index), "")
	}
	h.file("/nix/store/000-locked/keep", "kept by a permission failure\n")
	intent := h.approved(h.inspect())
	h.runPrefixes(intent, legacyIrreversible)
	if err := os.Chmod(h.path("/nix/store/000-locked"), 0555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(h.path("/nix/store/000-locked"), 0755) })
	if err := h.execute(intent, false); err == nil {
		t.Fatal("the fixture deletion did not fail partway")
	}
	if _, err := os.Lstat(h.path("/nix/store/aaa-nix-2.34.0")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("the fixture deletion did not remove part of the store", err)
	}
	if err := os.Chmod(h.path("/nix/store/000-locked"), 0755); err != nil {
		t.Fatal(err)
	}
	// The known failure is resumed explicitly; only remaining approved members
	// exist, so one new attempt finishes the same deletion.
	if err := h.execute(intent, true); err != nil {
		t.Fatal("the approved remaining subset could not be finished:", err)
	}
	if _, err := os.Lstat(h.path("/nix")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("the store remains", err)
	}
	h.runPrefixes(intent, nil)
}

func TestLegacyNixInterruptedDeletionRefusesUnapprovedMember(t *testing.T) {
	h := newLegacyNixHost(t, "linux")
	h.installLinuxUser()
	intent := h.approved(h.inspect())
	h.runPrefixes(intent, legacyIrreversible)
	command, err := h.d.stepCommand(*intent, intent.Next)
	if err != nil {
		t.Fatal(err)
	}
	h.die = func(dispatched nativeCommand) bool { return dispatched.Operation == command.Operation }
	if err := h.execute(intent, false); err == nil {
		t.Fatal("a killed worker reported success")
	}
	h.file("/nix/store/zzz-installed-after-approval/bin/tool", "new\n")
	scope := h.inspect()
	if !strings.Contains(scope.Blocked, "zzz-installed-after-approval") || len(scope.Steps) != 0 {
		t.Fatal("a store member outside the approved deletion was not named and refused", scope.Blocked, legacyStepLabels(scope.Steps))
	}
}

func TestLegacyNixDeletionWaitsForAnEarlierDeletionLease(t *testing.T) {
	h := newLegacyNixHost(t, "linux")
	h.installLinuxUser()
	intent := h.approved(h.inspect())
	h.runPrefixes(intent, legacyIrreversible)
	// An earlier deletion survives its killed worker and still holds the lease
	// on the Nix root. No new deletion may run until it exits.
	holder := exec.Command("/bin/sh", "-c", `exec 9<"$1"; /usr/bin/flock 9 || exit 1; echo locked; read _ || exit 0`, "sh", h.path("/nix"))
	input, err := holder.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	output, err := holder.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := holder.Start(); err != nil {
		t.Fatal(err)
	}
	released := false
	t.Cleanup(func() {
		if !released {
			_ = input.Close()
			_ = holder.Wait()
		}
	})
	if line, err := bufio.NewReader(output).ReadString('\n'); err != nil || line != "locked\n" {
		t.Fatal("lease holder did not start", line, err)
	}
	err = h.execute(intent, false)
	requireLegacyNixStoreIntact(t, h)
	if err == nil || !strings.Contains(err.Error(), "still running") {
		t.Fatal("a deletion ran while an earlier one held the Nix root lease", err)
	}
	released = true
	if err := input.Close(); err != nil {
		t.Fatal(err)
	}
	if err := holder.Wait(); err != nil {
		t.Fatal(err)
	}
	if err := h.execute(intent, false); err != nil {
		t.Fatal("the deletion did not proceed after the earlier lease ended:", err)
	}
}

func TestLegacyNixShellLookupErrorsAreNotDiscarded(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root bypasses the directory permission this fixture denies")
	}
	h := newLegacyNixHost(t, "linux")
	h.installLinuxUser()
	locked := h.path("/opt/locked")
	h.link("/opt/locked/bin/zsh", h.path("/nix/store/bbb-bash-5.2/bin/bash"))
	fields := strings.Split(h.passwd["fixture"], ":")
	fields[6] = locked + "/bin/zsh"
	h.passwd["fixture"] = strings.Join(fields, ":")
	if err := os.Chmod(locked, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0700) })
	if scope, err := h.d.inspect(context.Background(), "retire.nix"); err == nil {
		t.Fatal("an uninspectable login shell was treated as not coming from Nix", scope.Blocked, legacyStepLabels(scope.Steps))
	}
}

func TestLegacyNixConsumerGateAcceptsRootDefaultProfileOnly(t *testing.T) {
	h := newLegacyNixHost(t, "linux")
	h.installLinuxUser()
	// The 2.34.0 multi-user installer runs nix-env -i three times as root.
	legacyRootProfileFixture(t, h)
	perUser := "/nix/var/nix/profiles/per-user/root"
	gate := legacyRetirementGate{Kind: "nix-consumers"}
	if value, err := h.d.gateValue(context.Background(), gate); err != nil || value != "" {
		t.Fatalf("root's installer default profile was treated as another consumer: %q %v", value, err)
	}
	h.link(perUser+"/channels-1-link", h.path("/nix/store/r4-channels"))
	if value, err := h.d.gateValue(context.Background(), gate); err != nil || !strings.Contains(value, "root") {
		t.Fatalf("root's additional Nix profile was not a consumer: %q %v", value, err)
	}
}

func TestLegacyNixConsumerGateRefusesAnotherAccountsNixShell(t *testing.T) {
	h := newLegacyNixHost(t, "linux")
	h.installLinuxUser()
	h.passwd["other"] = "other:x:1001:1001::/home/other:" + h.path("/nix/store/bbb-bash-5.2/bin/bash")
	value, err := h.d.gateValue(context.Background(), legacyRetirementGate{Kind: "nix-consumers"})
	if err != nil || !strings.Contains(value, "account other") {
		t.Fatalf("another account's Nix login shell was not a consumer: %q %v", value, err)
	}
}

func TestLegacyNixDaemonIntegrationsOutliveTheStore(t *testing.T) {
	h := newLegacyNixHost(t, "linux")
	h.installLinuxDaemonIntegrations()
	h.linkAt(filepath.Join(h.home, ".nix-profile"), h.path("/nix/var/nix/profiles/per-user/fixture/profile"))
	scope := h.inspect()
	labels := strings.Join(legacyStepLabels(scope.Steps), "\n")
	for _, want := range []string{"/etc/systemd/system/nix-daemon.service", "/etc/tmpfiles.d/nix-daemon.conf", "nix-daemon.service.d", "Reload the systemd unit inventory", "/etc/nix",
		"Delete Nix build account nixbld2", "Delete the Nix build group nixbld", "Preserve root's Nix file if present: " + h.path("/root/.nix-profile")} {
		if !strings.Contains(labels, want) {
			t.Fatalf("an inert daemon integration that outlived the store was not retired: %q\n%s", want, labels)
		}
	}
	h.runPrefixes(h.approved(scope), nil)
	if after := h.inspect(); len(after.Steps) != 0 || after.Blocked != "" {
		t.Fatal("daemon leftovers did not converge", after.Blocked, legacyStepLabels(after.Steps))
	}
	if _, ok := h.groups["nixbld"]; ok || h.unitsLoaded {
		t.Fatal("the build group or stale systemd units remain")
	}
	if backups, err := filepath.Glob(h.path("/root/.dotfiles-retired-*-.nix-profile")); err != nil || len(backups) != 1 {
		t.Fatal("root's Nix profile link was not preserved", backups, err)
	}
}

func TestLegacyNixDarwinBuildAccountsRecognizeNativeAttributeNames(t *testing.T) {
	h := newLegacyNixHost(t, "darwin")
	h.installDarwin(false)
	for _, name := range []string{"_nixbld1", "_nixbld2"} {
		attributes := h.darwinUsers[name]
		delete(attributes, "IsHidden")
		attributes["dsAttrTypeNative:IsHidden"] = "1"
		// A native attribute with the same short name must not replace the
		// standard identity used to authorize account deletion.
		attributes["dsAttrTypeNative:UniqueID"] = "999"
	}
	var scope legacyRetirementScope
	if err := h.d.planDarwinBuildAccounts(context.Background(), &scope); err != nil || scope.Blocked != "" || len(scope.Steps) != 3 {
		t.Fatal("native dscl attribute names hid the recognized build accounts", scope.Blocked, len(scope.Steps), err)
	}
}

func TestLegacyNixDarwinEveryPrefixIsRediscoverable(t *testing.T) {
	h := newLegacyNixHost(t, "darwin")
	h.installDarwin(false)
	first := h.approved(h.inspect())
	h.runPrefixes(first, legacyIrreversible)
	// A busy volume refuses deletion. After a restart the boot-time mount
	// integrations must still identify the volume; nothing may report done.
	h.busy = true
	if err := h.execute(first, false); err == nil {
		t.Fatal("busy volume deletion reported success")
	}
	remount := h.unmountVolume()
	if scope := h.inspect(); !strings.Contains(scope.Blocked, legacyNixFixtureVolume) {
		t.Fatal("an unmounted existing Nix Store volume was not reported", scope.Blocked, legacyStepLabels(scope.Steps))
	}
	remount()
	h.busy = false
	fresh := h.inspect()
	if fresh.Blocked != "" || len(fresh.Steps) == 0 || !fresh.Steps[0].Irreversible {
		t.Fatal("the remounted volume has no fresh deletion scope", fresh.Blocked, legacyStepLabels(fresh.Steps))
	}
	if want := first.Scope.Steps[first.Next:]; !slices.Equal(legacyStepLabels(fresh.Steps), legacyStepLabels(want)) {
		t.Fatalf("the fresh scope differs from the remaining approved steps\ngot:  %q\nwant: %q", legacyStepLabels(fresh.Steps), legacyStepLabels(want))
	}
	h.runPrefixes(h.approved(fresh), nil)
	if scope := h.inspect(); scope.Blocked != "" || len(scope.Steps) != 0 {
		t.Fatal("retirement did not converge", scope.Blocked, legacyStepLabels(scope.Steps))
	}
	if len(h.volumes) != 0 || h.launchd["system/org.nixos.darwin-store"] || h.darwinGroups["nixbld"] != nil {
		t.Fatal("the volume, its mount service or the build group remain", h.volumes, h.launchd, h.darwinGroups)
	}
	for _, path := range []string{"/Library/LaunchDaemons/org.nixos.darwin-store.plist", "/etc/synthetic.conf", "/etc/nix", "/run/current-system", "/etc/static"} {
		if _, err := os.Lstat(h.path(path)); !errors.Is(err, os.ErrNotExist) {
			t.Error("retired macOS integration remains:", path, err)
		}
	}
	if data, err := os.ReadFile(h.path("/etc/fstab")); err != nil || string(data) != "# personal\n" {
		t.Fatal("personal fstab content changed or the Nix entry remains", string(data), err)
	}
	if data, err := os.ReadFile(h.path("/etc/zshrc")); err != nil || string(data) != "# original system zshrc\n" {
		t.Fatal("the file nix-darwin displaced was not restored", string(data), err)
	}
}

func TestLegacyNixDarwinRecognizesEncryptedStoreVolume(t *testing.T) {
	h := newLegacyNixHost(t, "darwin")
	h.installDarwin(true)
	scope := h.inspect()
	if scope.Blocked != "" {
		t.Fatal("the upstream encrypted Nix Store mount service was not recognized:", scope.Blocked)
	}
	labels := legacyStepLabels(scope.Steps)
	deletion := slices.IndexFunc(scope.Steps, legacyIrreversible)
	keychain := slices.IndexFunc(labels, func(label string) bool { return strings.Contains(label, "System keychain") })
	if deletion < 0 || keychain < deletion {
		t.Fatal("the volume password must be deleted after its volume", labels)
	}
	h.runPrefixes(h.approved(scope), nil)
	if h.keychain[legacyNixFixtureVolume] || len(h.volumes) != 0 {
		t.Fatal("the encrypted volume or its password remains")
	}
}

// startLegacyNixLease models an earlier deletion that outlived its killed
// worker: it holds the Nix root lease until the returned release runs.
func startLegacyNixLease(t *testing.T, root string) func() {
	t.Helper()
	holder := exec.Command("/bin/sh", "-c", `exec 9<"$1"; /usr/bin/flock 9 || exit 1; echo locked; read _ || exit 0`, "sh", root)
	input, err := holder.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	output, err := holder.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := holder.Start(); err != nil {
		t.Fatal(err)
	}
	released := false
	release := func() {
		t.Helper()
		if released {
			return
		}
		released = true
		if err := errors.Join(input.Close(), holder.Wait()); err != nil {
			t.Fatal("lease holder failed:", err)
		}
	}
	t.Cleanup(release)
	if line, err := bufio.NewReader(output).ReadString('\n'); err != nil || line != "locked\n" {
		t.Fatal("lease holder did not start", line, err)
	}
	return release
}

func TestLegacyNixDeletionCommandHoldsItsOwnLease(t *testing.T) {
	h := newLegacyNixHost(t, "linux")
	h.installLinuxUser()
	approval, err := h.d.nixStoreIdentity(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	step, err := h.d.nixDeletionStep("fixture deletion", approval)
	if err != nil {
		t.Fatal(err)
	}
	release := startLegacyNixLease(t, h.path("/nix"))
	// Even if a second command were dispatched past every gate, it cannot run
	// beside the earlier deletion: the lease is taken before rm starts.
	command := *step.Command
	command.Operation = strings.Repeat("c", 64)
	reply, err := executeNativeCommand(h.d.WorkerDirectory, command)
	if err != nil || reply.ExitCode == nil || *reply.ExitCode != 86 {
		t.Fatal("a second deletion was not refused by the held lease", reply, err)
	}
	requireLegacyNixStoreIntact(t, h)
	release()
	command.Operation = strings.Repeat("d", 64)
	if reply, err := executeNativeCommand(h.d.WorkerDirectory, command); err != nil || !legacyReplySucceeded(reply) {
		t.Fatal("the deletion did not run after the lease was released", reply, err)
	}
	if _, err := os.Lstat(h.path("/nix")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("the leased deletion did not remove the root", err)
	}
}

// interruptedLegacyNixDeletion saves an operation whose store deletion was
// dispatched and lost with its worker, leaving a partial store.
func interruptedLegacyNixDeletion(t *testing.T, h *legacyNixHost) *legacyRetirementIntent {
	t.Helper()
	approval, err := h.d.nixStoreIdentity(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	step, err := h.d.nixDeletionStep("Permanently delete the Nix store and runtime at /nix", approval)
	if err != nil {
		t.Fatal(err)
	}
	scope := legacyRetirementScope{Steps: []legacyRetirementStep{step}}
	if scope.ReplacementState, err = h.d.replacementState(); err != nil {
		t.Fatal(err)
	}
	intent := h.approved(scope)
	command, err := h.d.stepCommand(*intent, 0)
	if err != nil {
		t.Fatal(err)
	}
	h.die = func(dispatched nativeCommand) bool { return dispatched.Operation == command.Operation }
	if err := h.execute(intent, false); err == nil {
		t.Fatal("a killed worker reported success")
	}
	return intent
}

func (h *legacyNixHost) installLinuxStore() {
	store := h.path("/nix/store")
	uid, gid := strconv.Itoa(os.Getuid()), strconv.Itoa(os.Getgid())
	h.passwd["fixture"] = "fixture:x:" + uid + ":" + gid + "::" + h.home + ":/bin/sh"
	h.file("/nix/store/aaa-nix-2.34.0/bin/nix", "nix\n")
	h.file("/nix/store/bbb-bash-5.2/bin/bash", "bash\n")
	h.file("/nix/store/ccc-home-manager-generation/activate", "#!"+store+"/bbb-bash-5.2/bin/bash\n")
	h.file("/nix/var/nix/db/db.sqlite", "db\n")
	h.link("/nix/var/nix/profiles/default-1-link", store+"/aaa-nix-2.34.0")
	h.link("/nix/var/nix/profiles/default", "default-1-link")
}

func TestLegacyNixDaemonDeletionRecoveryFinishesItsIntegrations(t *testing.T) {
	h := newLegacyNixHost(t, "linux")
	h.installLinuxStore()
	h.installLinuxDaemonIntegrations()
	first := interruptedLegacyNixDeletion(t, h)
	scope := h.inspect()
	labels := legacyStepLabels(scope.Steps)
	if scope.Blocked != "" || len(labels) < 3 || labels[0] != "Stop the Nix daemon" || !scope.Steps[1].Irreversible || !strings.Contains(strings.Join(scope.Detail, "\n"), first.Operation) {
		t.Fatalf("the interrupted daemon deletion has no truthful recovery: %q %q", scope.Blocked, labels)
	}
	for _, want := range []string{"Remove the Nix systemd link: " + h.path("/etc/systemd/system/nix-daemon.service"), "Reload the systemd unit inventory", "Delete the Nix build group nixbld"} {
		if !slices.Contains(labels, want) {
			t.Fatalf("recovery lost %q: %q", want, labels)
		}
	}
	h.runPrefixes(h.approved(scope), nil)
	if after := h.inspect(); after.Blocked != "" || len(after.Steps) != 0 {
		t.Fatal("daemon recovery did not converge", after.Blocked, legacyStepLabels(after.Steps))
	}
}

func TestLegacyNixDeletionRecoveryAfterSystemdReload(t *testing.T) {
	h := newLegacyNixHost(t, "linux")
	h.installLinuxStore()
	h.installLinuxDaemonIntegrations()
	interruptedLegacyNixDeletion(t, h)
	// A reboot or daemon-reload drops units whose store-backed files were
	// already deleted. Their exact integration links remain for retirement.
	h.unitsLoaded = false
	scope := h.inspect()
	// A newly inspected suffix may omit the now-redundant daemon-reload.
	// Finish the approved recovery and verify its actual final state.
	intent := h.approved(scope)
	for intent.Next < len(intent.Scope.Steps) {
		if err := h.execute(intent, false); err != nil {
			t.Fatal("unloaded units blocked recovery", err)
		}
	}
	if after := h.inspect(); after.Blocked != "" || len(after.Steps) != 0 {
		t.Fatal("unloaded units stranded interrupted deletion", after.Blocked, legacyStepLabels(after.Steps))
	}
}

func TestLegacyNixMembershipEvidenceMustMatchItsApproval(t *testing.T) {
	h := newLegacyNixHost(t, "linux")
	h.installLinuxStore()
	first := interruptedLegacyNixDeletion(t, h)
	approval := ""
	for _, gate := range first.Scope.Steps[0].Gates {
		if gate.Kind == "nix-store" {
			approval = gate.Expected
		}
	}
	if !operationID.MatchString(approval) {
		t.Fatal("the deletion lacks its approved store identity")
	}
	path := h.d.nixMembershipPath(approval, 0)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal("the approved membership was not persisted before dispatch:", err)
	}
	// A rewritten record would widen what the deletion may finish.
	if err := os.WriteFile(path, []byte(strings.Replace(string(data), "store/bbb-bash-5.2=", "store/zzz-widened=", 1)), 0600); err != nil {
		t.Fatal(err)
	}
	scope := h.inspect()
	if !strings.Contains(scope.Blocked, "differs from its approval") || len(scope.Steps) != 0 {
		t.Fatal("altered membership evidence was trusted", scope.Blocked, legacyStepLabels(scope.Steps))
	}
}

func TestLegacyNixReinstalledRootIsClassifiedFresh(t *testing.T) {
	h := newLegacyNixHost(t, "linux")
	h.installLinuxUser()
	interruptedLegacyNixDeletion(t, h)
	// The user finished removal manually and installed Nix again. Keep the old
	// root allocated until the new one exists so its inode cannot be reused.
	old := filepath.Join(h.base, "removed-nix")
	if err := os.Rename(h.path("/nix"), old); err != nil {
		t.Fatal(err)
	}
	h.installLinuxUserRoot()
	if err := os.RemoveAll(old); err != nil {
		t.Fatal(err)
	}
	scope := h.inspect()
	if scope.Blocked != "" || !slices.Contains(legacyStepLabels(scope.Steps), "Permanently delete the Nix store and runtime at /nix") {
		t.Fatal("an unrelated earlier deletion blocked a new installation's retirement", scope.Blocked, legacyStepLabels(scope.Steps))
	}
}

func TestLegacyNixUnreadableJournalAuthorizesNothing(t *testing.T) {
	h := newLegacyNixHost(t, "linux")
	h.installLinuxStore()
	intent := interruptedLegacyNixDeletion(t, h)
	intent.Scope.Detail = []string{"widened after approval"}
	if err := saveDocument(h.d.intentPath(intent.Operation), intent); err != nil {
		t.Fatal(err)
	}
	scope := h.inspect()
	if len(scope.Steps) != 0 || scope.Blocked == "" || !strings.Contains(strings.Join(scope.Detail, "\n"), "unreadable and authorizes nothing") {
		t.Fatal("a changed journal authorized recovery or was not disclosed", scope.Blocked, legacyStepLabels(scope.Steps), scope.Detail)
	}
}

func TestLegacyNixLeftoversNeedBuildAccountEvidence(t *testing.T) {
	for _, goos := range []string{"linux", "darwin"} {
		t.Run(goos, func(t *testing.T) {
			h := newLegacyNixHost(t, goos)
			// A host that never had Nix, with an unrelated account that happens
			// to use the upstream build group ID, has nothing to retire.
			h.passwd["builder"] = "builder:x:1001:30000::/home/builder:/bin/sh"
			h.darwinUsers["builder"] = map[string]string{"UniqueID": "1001", "PrimaryGroupID": "350", "UserShell": "/bin/sh"}
			h.darwinUsers["fixture"] = map[string]string{"UniqueID": "501", "PrimaryGroupID": "20", "UserShell": "/bin/zsh"}
			h.file("/etc/nix/nix.conf", "# not from this release\n")
			if scope := h.inspect(); scope.Blocked != "" || len(scope.Steps) != 0 {
				t.Fatal("a host without Nix was given retirement work", scope.Blocked, legacyStepLabels(scope.Steps))
			}
		})
	}
}

func TestLegacyNixInterruptedDeletionRefusesNewRootMember(t *testing.T) {
	h := newLegacyNixHost(t, "linux")
	h.installLinuxStore()
	interruptedLegacyNixDeletion(t, h)
	recovery := h.approved(h.inspect())
	// Personal data placed beside the store after approval is not Nix.
	h.file("/nix/personal/notes.txt", "keep\n")
	if scope := h.inspect(); !strings.Contains(scope.Blocked, "personal") || len(scope.Steps) != 0 {
		t.Fatal("a new /nix member was not named and refused in preview", scope.Blocked, legacyStepLabels(scope.Steps))
	}
	if err := h.execute(recovery, false); err == nil || !strings.Contains(err.Error(), "changed after approval") {
		t.Fatal("the approved finish step ran over a new /nix member", err)
	}
	if data, err := os.ReadFile(h.path("/nix/personal/notes.txt")); err != nil || string(data) != "keep\n" {
		t.Fatal("a new /nix member was deleted", err)
	}
}

func TestLegacyNixRecoveryWhenTheEarlierDeletionCompletes(t *testing.T) {
	h := newLegacyNixHost(t, "linux")
	h.installLinuxStore()
	h.installLinuxDaemonIntegrations()
	interruptedLegacyNixDeletion(t, h)
	// The earlier rm finishes between inspection's directory read and its
	// membership read; the root is gone, so only inert leftovers remain.
	if err := os.RemoveAll(h.path("/nix")); err != nil {
		t.Fatal(err)
	}
	scope := legacyRetirementScope{}
	recovered, err := h.d.planNixDeletionRecovery(context.Background(), &scope)
	if err != nil || scope.Blocked != "" || slices.ContainsFunc(scope.Steps, legacyIrreversible) {
		t.Fatal("a completed earlier deletion was not handled as absence", recovered, err, scope.Blocked, legacyStepLabels(scope.Steps))
	}
	if !slices.Contains(legacyStepLabels(scope.Steps), "Delete the Nix build group nixbld") {
		t.Fatal("the completed deletion's inert integrations were not planned", legacyStepLabels(scope.Steps))
	}
}
