package installer

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

// The embedded receipts are official nix-installer test fixtures, serialized
// by the vendor, reduced with jq to their first build account and with
// backticks spelled `; every other field is the vendor's. Unreduced
// fixtures are checked by TestLegacyDeterminateOfficialFixtures.

func legacyDeterminateDocument(t *testing.T, receipt string) map[string]any {
	t.Helper()
	var document map[string]any
	if err := json.Unmarshal([]byte(receipt), &document); err != nil {
		t.Fatal(err)
	}
	return document
}

func legacyDeterminateEncode(t *testing.T, document map[string]any) []byte {
	t.Helper()
	data, err := json.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// legacyDeterminateTop returns the action object of a top-level action.
func legacyDeterminateTop(document map[string]any, name string) map[string]any {
	for _, raw := range document["actions"].([]any) {
		action := raw.(map[string]any)["action"].(map[string]any)
		if action["action_name"] == name {
			return action
		}
	}
	panic("fixture lacks " + name)
}

func legacyDeterminateChild(action map[string]any, key string) map[string]any {
	return action[key].(map[string]any)["action"].(map[string]any)
}

func legacyDeterminateItem(action map[string]any, key string, index int) map[string]any {
	return action[key].([]any)[index].(map[string]any)["action"].(map[string]any)
}

func TestLegacyDeterminateReceiptModelsOfficialShapes(t *testing.T) {
	linux, err := parseLegacyDeterminateReceipt([]byte(legacyDeterminateLinuxReceipt))
	if err != nil {
		t.Fatal(err)
	}
	user := legacyDeterminateUser{Name: "nixbld1", Comment: "Nix build user 1", UID: 30001, GID: 30000, Reverted: true}
	if linux.Version != "3.23.3" || linux.Planner != "linux" || !linux.Determinate || linux.Init != "Systemd" || !linux.StartDaemon || !linux.Root || !linux.Nixd ||
		linux.Group != "nixbld" || linux.GroupID != 30000 || !linux.GroupRevert || !slices.Equal(linux.Users, []legacyDeterminateUser{user}) ||
		!slices.Equal(linux.Units, []string{"nix-daemon.socket", "determinate-nixd.socket", "nix-daemon.service"}) || len(linux.Prune) != 0 || linux.Volume != nil {
		t.Fatalf("%+v", linux)
	}
	for _, path := range []string{legacyDeterminateNixdPath, "/etc/nix/nix.conf", "/etc/nix/nix.custom.conf", "/etc/systemd/system/nix-daemon.service",
		"/etc/systemd/system/nix-daemon.socket", "/etc/systemd/system/determinate-nixd.socket", "/etc/tmpfiles.d/nix-daemon.conf"} {
		if !slices.Contains(linux.Removed, path) {
			t.Error("vendor removal not modeled:", path)
		}
	}
	edited := []string{}
	for _, insert := range linux.Inserts {
		edited = append(edited, insert.Path)
	}
	if !slices.Equal(edited, []string{"/etc/bashrc", "/etc/profile.d/nix.sh", "/etc/bash.bashrc", "/etc/zshrc", "/etc/zsh/zshrc", "/usr/share/fish/vendor_conf.d/nix.fish"}) {
		t.Error(edited)
	}

	mac, err := parseLegacyDeterminateReceipt([]byte(legacyDeterminateMacReceipt))
	if err != nil {
		t.Fatal(err)
	}
	if mac.Planner != "macos" || !mac.Determinate || mac.Init != "Launchd" || mac.Root || mac.Group != "nixbld" || mac.GroupID != 350 || mac.Users[0].Name != "_nixbld1" ||
		mac.Users[0].UID != 351 || *mac.Volume != (legacyDeterminateVolume{Disk: "disk4", Name: "Nix Store", Determinate: true, Deleted: true, Keychain: true}) ||
		!slices.Equal(mac.Services, []string{"systems.determinate.nix-store", "systems.determinate.nix-daemon"}) {
		t.Fatalf("%+v %+v", mac, mac.Volume)
	}
	for _, path := range []string{"/Library/LaunchDaemons/systems.determinate.nix-daemon.plist", "/Library/LaunchDaemons/systems.determinate.nix-store.plist",
		"/Library/LaunchDaemons/systems.determinate.nix-installer.nix-hook.plist", legacyDeterminateNixdPath} {
		if !slices.Contains(mac.Removed, path) {
			t.Error("vendor removal not modeled:", path)
		}
	}
	if !slices.ContainsFunc(mac.Inserts, func(insert legacyDeterminateInsert) bool {
		return insert.Path == "/etc/synthetic.conf" && insert.Buf == "nix\n"
	}) ||
		!slices.ContainsFunc(mac.Inserts, func(insert legacyDeterminateInsert) bool { return insert.Path == "/etc/zshenv" }) {
		t.Error("macOS boot and remote-building edits are not modeled", mac.Inserts)
	}

	// The upstream Nix distribution through the same installer, before
	// determinate-nixd was the default and before service_src became a UnitSrc.
	upstreamMac, err := parseLegacyDeterminateReceipt([]byte(legacyDeterminateMacUpstreamReceipt))
	if err != nil {
		t.Fatal(err)
	}
	if upstreamMac.Determinate || upstreamMac.Nixd || *upstreamMac.Volume != (legacyDeterminateVolume{Disk: "disk4", Name: "Nix Store", Deleted: true}) ||
		!slices.Equal(upstreamMac.Services, []string{"org.nixos.darwin-store", "org.nixos.nix-daemon"}) {
		t.Fatalf("%+v %+v", upstreamMac, upstreamMac.Volume)
	}
	upstreamLinux, err := parseLegacyDeterminateReceipt([]byte(legacyDeterminateLinuxUpstreamReceipt))
	if err != nil {
		t.Fatal(err)
	}
	if upstreamLinux.Version != "3.0.0" || upstreamLinux.Determinate || upstreamLinux.Nixd || !slices.Equal(upstreamLinux.Units, []string{"nix-daemon.socket", "nix-daemon.service"}) ||
		slices.Contains(upstreamLinux.Removed, legacyDeterminateNixdPath) {
		t.Fatalf("%+v", upstreamLinux)
	}
}

func TestLegacyDeterminateReceiptPathsUsePOSIXSemantics(t *testing.T) {
	for _, value := range []string{"/nix", "/etc/nix", "C:/nix", "nix", "/nix/../etc", "//nix", "/"} {
		raw, err := json.Marshal(map[string]string{"path": value})
		if err != nil {
			t.Fatal(err)
		}
		fields, err := legacyDeterminateObject(raw, "fixture")
		if err != nil {
			t.Fatal(err)
		}
		got, err := fields.path("path")
		valid := value == "/nix" || value == "/etc/nix"
		if (err == nil) != valid || valid && got != value {
			t.Fatal("receipt paths must use the target POSIX syntax on every host", value, got, err)
		}
	}
}

// The previous adapter walked only absolute strings. Every case below names
// no new path, or only a reviewed one, yet changes what the vendor deletes or
// edits: account and group names, the APFS volume name, launchd labels and
// systemd unit names are vendor arguments, and text removal edits files.
func TestLegacyDeterminateReceiptRefusesUnreviewedSemantics(t *testing.T) {
	linuxUsers := func(document map[string]any) map[string]any {
		return legacyDeterminateItem(legacyDeterminateTop(document, "create_users_and_group"), "create_users", 0)
	}
	linuxInit := func(document map[string]any) map[string]any {
		return legacyDeterminateChild(legacyDeterminateTop(document, "configure_determinate_nixd_init_service"), "configure_init_service")
	}
	shell := func(document map[string]any) map[string]any {
		return legacyDeterminateChild(legacyDeterminateTop(document, "configure_nix"), "configure_shell_profile")
	}
	cases := []struct {
		name, receipt, reason string
		mutate                func(map[string]any)
	}{
		{"account renamed to a personal account", legacyDeterminateLinuxReceipt, "name", func(d map[string]any) { linuxUsers(d)["name"] = "alice" }},
		{"account uid changed", legacyDeterminateLinuxReceipt, "identity", func(d map[string]any) { linuxUsers(d)["uid"] = 501 }},
		{"unreviewed account field", legacyDeterminateLinuxReceipt, "unreviewed field home", func(d map[string]any) { linuxUsers(d)["home"] = "/root" }},
		{"missing action name", legacyDeterminateLinuxReceipt, "action_name", func(d map[string]any) { delete(linuxUsers(d), "action_name") }},
		{"group renamed", legacyDeterminateLinuxReceipt, "name", func(d map[string]any) {
			legacyDeterminateChild(legacyDeterminateTop(d, "create_users_and_group"), "create_group")["name"] = "staff"
		}},
		{"systemd unit renamed", legacyDeterminateLinuxReceipt, "name", func(d map[string]any) {
			linuxInit(d)["socket_files"].([]any)[0].(map[string]any)["name"] = "ssh.socket"
		}},
		{"shell edit removes other text", legacyDeterminateLinuxReceipt, "unreviewed text", func(d map[string]any) {
			legacyDeterminateItem(shell(d), "create_or_insert_into_files", 3)["buf"] = "export PATH=\n"
		}},
		{"GitHub path edit outside a runner command file", legacyDeterminateLinuxReceipt, "unreviewed text", func(d map[string]any) {
			edit := legacyDeterminateItem(shell(d), "create_or_insert_into_files", 0)
			edit["path"], edit["buf"], edit["position"] = "/home/runner/.bashrc", legacyDeterminateGitHubBin, "End"
		}},
		{"configuration prune widened to /etc", legacyDeterminateLinuxReceipt, "outside the reviewed vendor directories", func(d map[string]any) {
			directory := legacyDeterminateChild(legacyDeterminateChild(legacyDeterminateTop(d, "configure_nix"), "place_nix_configuration"), "create_directory")
			directory["path"], directory["force_prune_on_revert"] = "/etc", true
		}},
		{"existing /nix mount", legacyDeterminateLinuxReceipt, "mount point", func(d map[string]any) { legacyDeterminateTop(d, "create_directory")["is_mountpoint"] = true }},
		{"mixed distributions", legacyDeterminateLinuxReceipt, "mixes", func(d map[string]any) {
			legacyDeterminateChild(legacyDeterminateTop(d, "provision_nix"), "fetch_nix")["distribution"] = "Nix"
		}},
		{"unknown action", legacyDeterminateLinuxReceipt, "not a reviewed top-level", func(d map[string]any) {
			d["actions"] = append(d["actions"].([]any), map[string]any{"action": map[string]any{"action_name": "run_command", "command": "id"}, "state": "Completed"})
		}},
		{"unknown state", legacyDeterminateLinuxReceipt, "state", func(d map[string]any) { d["actions"].([]any)[0].(map[string]any)["state"] = "Failed" }},
		{"newer vendor release", legacyDeterminateLinuxReceipt, "outside the reviewed vendor releases", func(d map[string]any) { d["version"] = "3.24.0" }},
		{"next major release", legacyDeterminateLinuxReceipt, "outside the reviewed vendor releases", func(d map[string]any) { d["version"] = "4.0.0" }},
		{"pre-3.0 release", legacyDeterminateLinuxReceipt, "outside the reviewed vendor releases", func(d map[string]any) { d["version"] = "0.38.1" }},
		{"prerelease", legacyDeterminateLinuxReceipt, "outside the reviewed vendor releases", func(d map[string]any) { d["version"] = "3.23.3-rc.1" }},
		{"unsupported planner", legacyDeterminateLinuxReceipt, "planner is not reviewed", func(d map[string]any) {
			d["planner"].(map[string]any)["planner"] = "steam-deck"
		}},
		{"volume deleted by another name", legacyDeterminateMacReceipt, "create_apfs_volume", func(d map[string]any) {
			legacyDeterminateChild(legacyDeterminateTop(d, "create_determinate_nix_volume"), "create_volume")["name"] = "Macintosh HD"
		}},
		{"keychain item on another disk", legacyDeterminateMacReceipt, "disk", func(d map[string]any) {
			legacyDeterminateChild(legacyDeterminateTop(d, "create_determinate_nix_volume"), "encrypt_volume")["disk"] = "disk0"
		}},
		{"launchd label replaced", legacyDeterminateMacReceipt, "service_name", func(d map[string]any) {
			legacyDeterminateChild(legacyDeterminateTop(d, "configure_determinate_nixd_init_service"), "configure_init_service")["service_name"] = "com.apple.metadata.mds"
		}},
		{"hook plist replaced", legacyDeterminateMacReceipt, "path", func(d map[string]any) {
			legacyDeterminateTop(d, "create_nix_hook_service")["path"] = "/Library/LaunchDaemons/com.apple.example.plist"
		}},
		{"EC2 instance store", legacyDeterminateMacReceipt, "EC2", func(d map[string]any) { d["planner"].(map[string]any)["use_ec2_instance_store"] = true }},
	}
	for _, item := range cases {
		t.Run(item.name, func(t *testing.T) {
			document := legacyDeterminateDocument(t, item.receipt)
			item.mutate(document)
			_, err := parseLegacyDeterminateReceipt(legacyDeterminateEncode(t, document))
			if err == nil || !strings.Contains(err.Error(), item.reason) {
				t.Fatalf("accepted or refused for another reason: %v", err)
			}
		})
	}
	// encoding/json keeps the last duplicate field; serde rejects duplicates.
	duplicate := strings.Replace(legacyDeterminateLinuxReceipt, `"name":"nixbld1"`, `"name":"nixbld1","name":"alice"`, 1)
	if _, err := parseLegacyDeterminateReceipt([]byte(duplicate)); err == nil || !strings.Contains(err.Error(), "repeats field") {
		t.Fatal("a duplicate field must not hide the vendor's value", err)
	}
	for _, document := range []string{"", "null", "[]", legacyDeterminateLinuxReceipt + "{}", strings.Repeat(" ", legacyDeterminateReceiptLimit+1)} {
		if _, err := parseLegacyDeterminateReceipt([]byte(document)); err == nil {
			t.Errorf("accepted %.40q", document)
		}
	}
}

// The vendor reverts an action only when it and its parents are Completed or
// Progress; a failed or cancelled run records the rest as Uncompleted.
func TestLegacyDeterminateReceiptFollowsVendorStates(t *testing.T) {
	document := legacyDeterminateDocument(t, legacyDeterminateLinuxReceipt)
	users := legacyDeterminateTop(document, "create_users_and_group")
	users["create_users"].([]any)[0].(map[string]any)["state"] = "Uncompleted"
	users["create_group"].(map[string]any)["state"] = "Skipped"
	for _, raw := range document["actions"].([]any) {
		if raw.(map[string]any)["action"].(map[string]any)["action_name"] == "configure_nix" {
			raw.(map[string]any)["state"] = "Uncompleted"
		}
	}
	r, err := parseLegacyDeterminateReceipt(legacyDeterminateEncode(t, document))
	if err != nil {
		t.Fatal(err)
	}
	if r.Users[0].Reverted || r.GroupRevert || slices.ContainsFunc(r.Inserts, func(insert legacyDeterminateInsert) bool { return insert.Reverted }) ||
		slices.Contains(r.Removed, "/etc/nix/nix.custom.conf") || !slices.Contains(r.Removed, legacyDeterminateNixdPath) {
		t.Fatalf("%+v", r)
	}
}

func TestLegacyDeterminateRootEntriesAreVendorOwned(t *testing.T) {
	for _, name := range []string{"store", "var", "receipt.json", "nix-installer", "temp-install-dir", "receipt.pre-repair.1760000000000.json"} {
		if !legacyDeterminateRootEntry("linux", name) {
			t.Error("vendor entry refused:", name)
		}
	}
	for _, name := range []string{".nix-installer-hook.err.log", ".fseventsd"} {
		if legacyDeterminateRootEntry("linux", name) || !legacyDeterminateRootEntry("darwin", name) {
			t.Error("macOS-only entry misclassified:", name)
		}
	}
	for _, name := range []string{"uninstall-phase1.json", ".original-receipt.1.json", "receipt.pre-repair.json", "personal", "receipt.json.bak"} {
		if legacyDeterminateRootEntry("darwin", name) {
			t.Error("unrecognized /nix data accepted:", name)
		}
	}
}

func TestLegacyDeterminateInsertRulesUseRealFiles(t *testing.T) {
	d, root := legacyStepFixture(t)
	path := filepath.Join(root, "synthetic.conf")
	if problem := d.determinateInsertProblem(context.Background(), path, "nix\n", ""); problem != "" {
		t.Fatal("an absent file needs no edit", problem)
	}
	for contents, refused := range map[string]bool{"nix\n": false, "run\tprivate/var/run\nnix\n": false, "data\tVolumes/unix\n": true, "personal\n": false} {
		writeConfigFixture(t, path, contents)
		if problem := d.determinateInsertProblem(context.Background(), path, "nix\n", ""); (problem != "") != refused {
			t.Errorf("%q: %q", contents, problem)
		}
	}
	link := filepath.Join(root, "zshrc")
	target := filepath.Join(root, "static", "zshrc")
	writeConfigFixture(t, target, legacyDeterminateShellHook)
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	if problem := d.determinateInsertProblem(context.Background(), link, legacyDeterminateShellHook, ""); !strings.Contains(problem, "is a link") {
		t.Fatal("the vendor would write through a link", problem)
	}
	if problem := d.determinateInsertProblem(context.Background(), link, legacyDeterminateShellHook, target); problem != "" {
		t.Fatal("the exact nix-darwin link is retired earlier in the same plan", problem)
	}
	if problem := d.determinateInsertProblem(context.Background(), root, legacyDeterminateShellHook, ""); !strings.Contains(problem, "not a regular file") {
		t.Fatal(problem)
	}
}

func TestLegacyDeterminateExecutableNeedsRootOwnedPlainFile(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Nix retirement requires POSIX file ownership")
	}
	root := t.TempDir()
	binary := filepath.Join(root, "nix-installer")
	writeConfigFixture(t, binary, "#!/bin/sh\n")
	if _, _, problem, err := legacyDeterminateExecutable(filepath.Join(root, "absent"), ""); err != nil || !strings.Contains(problem, "missing") {
		t.Fatal(problem, err)
	}
	link := filepath.Join(root, "link")
	if err := os.Symlink(binary, link); err != nil {
		t.Fatal(err)
	}
	if _, _, problem, err := legacyDeterminateExecutable(link, ""); err != nil || !strings.Contains(problem, "plain file") {
		t.Fatal("a link is not the vendor's installed copy", problem, err)
	}
	if os.Geteuid() != 0 {
		if _, _, problem, err := legacyDeterminateExecutable(binary, ""); err != nil || !strings.Contains(problem, "root-owned") {
			t.Fatal("a user-owned binary must not run as root", problem, err)
		}
		if _, _, problem, err := legacyDeterminateExecutable("/bin/sh", root); err != nil || !strings.Contains(problem, root) {
			t.Fatal("a user-owned parent can replace the binary", problem, err)
		}
	}
}

func TestLegacyDeterminateAccountShapes(t *testing.T) {
	linux := legacyDeterminateUser{Name: "nixbld1", Comment: "Nix build user 1", UID: 30001, GID: 30000}
	if !legacyDeterminateAccount("linux", "nixbld1:x:30001:30000:Nix build user 1:/var/empty:/sbin/nologin", linux) ||
		legacyDeterminateAccount("linux", "nixbld1:x:30001:30000:Nix build user 1:/home/nixbld1:/bin/bash", linux) ||
		legacyDeterminateAccount("linux", "nixbld1:x:1000:30000:Nix build user 1:/var/empty:/sbin/nologin", linux) {
		t.Fatal("Linux build accounts must match the vendor's useradd shape")
	}
	mac := legacyDeterminateUser{Name: "_nixbld1", Comment: "Nix build user 1", UID: 351, GID: 350}
	for record, ok := range map[string]bool{
		"_nixbld1:351:350:_nixbld1:/var/empty:/sbin/nologin:1":         true,
		"_nixbld1:351:350:Nix build user 1:/var/empty:/sbin/nologin:1": true, // adopted upstream account
		"_nixbld1:351:350:_nixbld1:/var/empty:/sbin/nologin:":          true, // the vendor tolerates a killed hide step
		"_nixbld1:351:20:_nixbld1:/var/empty:/sbin/nologin:1":          false,
		"_nixbld1:351:350:Jane:/Users/jane:/bin/zsh:0":                 false,
	} {
		if legacyDeterminateAccount("darwin", record, mac) != ok {
			t.Error(record)
		}
	}
}

func legacyDeterminateIntent(t *testing.T, d *legacyRetirementDriver, operation string, steps ...legacyRetirementStep) *legacyRetirementIntent {
	t.Helper()
	intent := legacyTestIntent(t, d, steps...)
	intent.Operation = strings.Repeat(operation, 64)
	if err := saveDocument(d.intentPath(intent.Operation), intent); err != nil {
		t.Fatal(err)
	}
	return intent
}

// The privileged command re-verifies both approved files, then execs the
// vendor with a minimal environment from /. Changed bytes never start it.
func TestLegacyDeterminateUninstallVerifiesApprovedBytesBeforeVendor(t *testing.T) {
	d, root := legacyStepFixture(t)
	ctx := context.Background()
	log := filepath.Join(root, "vendor.log")
	installer, receipt := filepath.Join(root, "nix-installer"), filepath.Join(root, "receipt.json")
	writeConfigFixture(t, installer, "#!/bin/sh\n{ pwd; printf '%s\\n' \"$@\"; echo \"telemetry=${DETSYS_IDS_TELEMETRY-} endpoint=${NIX_INSTALLER_DIAGNOSTIC_ENDPOINT+set:}${NIX_INSTALLER_DIAGNOSTIC_ENDPOINT-} home=${HOME-unset}\"; } > '"+log+"'\n")
	if err := os.Chmod(installer, 0o700); err != nil {
		t.Fatal(err)
	}
	writeConfigFixture(t, receipt, legacyDeterminateLinuxReceipt)
	installerSum, err := legacyFileSHA256(installer)
	if err != nil {
		t.Fatal(err)
	}
	command := legacyDeterminateUninstallCommand(installer, installerSum, receipt, legacyHash([]byte(legacyDeterminateLinuxReceipt)))
	parent := legacyDeterminateStagingParent(t, root)
	command.Arguments = append(command.Arguments, parent)
	step := legacyRetirementStep{Kind: "command", Label: "fixture vendor uninstall", Command: &command}
	if err := d.executeStep(ctx, legacyDeterminateIntent(t, d, "1", step), 0, false); err != nil {
		t.Fatal(err)
	}
	stages, err := os.ReadDir(parent)
	if err != nil || len(stages) != 1 {
		t.Fatal("private stage missing", stages, err)
	}
	want := "/\nuninstall\n--no-confirm\n" + filepath.Join(parent, stages[0].Name(), "receipt.json") + "\ntelemetry=disabled endpoint=set: home=unset\n"
	if data, err := os.ReadFile(log); err != nil || string(data) != want {
		t.Fatalf("vendor invocation %q, want %q: %v", data, want, err)
	}
	for _, change := range []struct {
		operation, code string
		change          func()
	}{
		{"2", "exit 123", func() { writeConfigFixture(t, receipt, legacyDeterminateLinuxReceipt+"\n") }},
		{"3", "exit 122", func() {
			writeConfigFixture(t, receipt, legacyDeterminateLinuxReceipt)
			if err := os.WriteFile(installer, []byte("#!/bin/sh\ntouch '"+log+"'\n"), 0o700); err != nil {
				t.Fatal(err)
			}
		}},
		{"4", "exit 120", func() {
			if err := os.Remove(receipt); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(filepath.Join(root, "elsewhere"), receipt); err != nil {
				t.Fatal(err)
			}
		}},
	} {
		if err := os.Remove(log); err != nil && !errors.Is(err, os.ErrNotExist) {
			t.Fatal(err)
		}
		change.change()
		if err := d.executeStep(ctx, legacyDeterminateIntent(t, d, change.operation, step), 0, false); err == nil || !strings.Contains(err.Error(), change.code) {
			t.Fatal("changed approved bytes reached the vendor or failed differently:", err)
		}
		if _, err := os.Lstat(log); !errors.Is(err, os.ErrNotExist) {
			t.Fatal("the vendor started after a refused verification", err)
		}
		if remaining, err := os.ReadDir(parent); err != nil || len(remaining) != 1 || remaining[0].Name() != stages[0].Name() {
			t.Fatal("a refused verification left private staging files", remaining, err)
		}
	}
}

// The vendor keeps reverting after an individual failure, so a nonzero exit
// can follow a complete uninstall. Recovery verifies the approved outcome
// before any new attempt and never repeats a vendor run whose outcome holds.
func TestLegacyDeterminateFailedVendorRunIsVerifiedNotRepeated(t *testing.T) {
	d, root := legacyStepFixture(t)
	ctx := context.Background()
	counter, store := filepath.Join(root, "runs"), filepath.Join(root, "store")
	installer, receipt := filepath.Join(root, "nix-installer"), filepath.Join(root, "receipt.json")
	writeConfigFixture(t, installer, "#!/bin/sh\nprintf x >> '"+counter+"'\nrm -rf -- '"+store+"' '"+receipt+"' '"+installer+"'\nexit 1\n")
	if err := os.Chmod(installer, 0o700); err != nil {
		t.Fatal(err)
	}
	writeConfigFixture(t, receipt, legacyDeterminateLinuxReceipt)
	writeConfigFixture(t, filepath.Join(store, "fixture-nix"), "store\n")
	before := []legacyRetirementFile{}
	for _, path := range []string{receipt, installer} {
		file, err := legacyFile(path)
		if err != nil {
			t.Fatal(err)
		}
		before = append(before, file)
	}
	installerSum, err := legacyFileSHA256(installer)
	if err != nil {
		t.Fatal(err)
	}
	command := legacyDeterminateUninstallCommand(installer, installerSum, receipt, legacyHash([]byte(legacyDeterminateLinuxReceipt)))
	command.Arguments = append(command.Arguments, legacyDeterminateStagingParent(t, root))
	intent := legacyDeterminateIntent(t, d, "5", legacyRetirementStep{Kind: "command", Label: "fixture vendor uninstall", Command: &command, Before: before,
		Absent: []string{installer, receipt, store}})
	if err := d.executeStep(ctx, intent, 0, false); err == nil || !strings.Contains(err.Error(), "exit 1") {
		t.Fatal("the vendor failure was not reported", err)
	}
	o, err := d.observeInterrupted(ctx, Observation{}, *intent)
	if err != nil || o.ResourceResume == nil || !strings.Contains(o.Pending, "failed") {
		t.Fatal("a known vendor failure needs explicit recovery approval", o, err)
	}
	if err := d.executeStep(ctx, intent, 0, true); err != nil {
		t.Fatal("the approved outcome holds and needs no new vendor run", err)
	}
	if data, err := os.ReadFile(counter); err != nil || string(data) != "x" {
		t.Fatal("the vendor uninstall was repeated", string(data), err)
	}
}

func TestLegacyDeterminateEvidenceIsExactAndPreserved(t *testing.T) {
	d, _ := legacyStepFixture(t)
	ctx := context.Background()
	evidence := legacyDeterminateEvidence{Schema: 1, Receipt: []byte(legacyDeterminateMacReceipt), Installer: strings.Repeat("a", 64),
		Accounts: []string{"_nixbld1:351:350:_nixbld1:/var/empty:/sbin/nologin:1"}, Group: "nixbld:350:",
		Volume: "VolumeName=Nix Store;VolumeUUID=11111111-2222-3333-4444-555555555555", VolumeUUID: "11111111-2222-3333-4444-555555555555"}
	step, err := d.determinateEvidenceStep(evidence)
	if err != nil {
		t.Fatal(err)
	}
	if err := d.validateStep(step); err != nil {
		t.Fatal(err)
	}
	for _, operation := range []string{"1", "2"} { // A repeated approval reuses the same exact file.
		if err := d.executeStep(ctx, legacyDeterminateIntent(t, d, operation, step), 0, false); err != nil {
			t.Fatal(err)
		}
	}
	path := step.After[0].Path
	encoded, err := json.Marshal(evidence)
	if err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(path); err != nil || !bytes.Equal(data, encoded) {
		t.Fatal("evidence bytes differ", err)
	}
	if info, err := os.Stat(path); err != nil || info.Mode().Perm() != 0o600 {
		t.Fatal("evidence must stay private", err)
	}
	loaded, err := d.determinateEvidence()
	if err != nil || len(loaded) != 1 || !bytes.Equal(loaded[0].Receipt, evidence.Receipt) || loaded[0].VolumeUUID != evidence.VolumeUUID {
		t.Fatal(loaded, err)
	}
	// A different file at the approved name is preserved, never replaced.
	writeConfigFixture(t, path, "personal\n")
	if err := d.executeStep(ctx, legacyDeterminateIntent(t, d, "3", step), 0, false); err == nil || !strings.Contains(err.Error(), "exit 91") {
		t.Fatal("changed evidence was replaced", err)
	}
	if data, err := os.ReadFile(path); err != nil || string(data) != "personal\n" {
		t.Fatal(string(data), err)
	}
	if _, err := d.determinateEvidence(); err == nil {
		t.Fatal("evidence whose bytes no longer match its name was trusted")
	}
}

func TestLegacyDeterminateLeftoversProtectNewOrChangedFiles(t *testing.T) {
	for _, kind := range []string{"older evidence without files", "changed file", "previously absent file"} {
		t.Run(kind, func(t *testing.T) {
			d, root := legacyStepFixture(t)
			d.systemRoot = filepath.Join(root, "system")
			d.Platform.Context = Context{OS: "linux", Arch: "amd64"}
			path := d.sys("/etc/systemd/system/determinate-nixd.socket")
			if kind == "changed file" {
				writeConfigFixture(t, path, "original vendor file\n")
			}
			file, err := legacyFile(path)
			if err != nil {
				t.Fatal(err)
			}
			evidence := legacyDeterminateEvidence{Schema: 1, Receipt: []byte(legacyDeterminateLinuxReceipt), Installer: strings.Repeat("a", 64)}
			if kind != "older evidence without files" {
				evidence.Files = []legacyRetirementFile{file}
			}
			step, err := d.determinateEvidenceStep(evidence)
			if err != nil {
				t.Fatal(err)
			}
			if err := d.executeStep(context.Background(), legacyDeterminateIntent(t, d, "1", step), 0, false); err != nil {
				t.Fatal(err)
			}
			writeConfigFixture(t, path, "personal replacement\n")
			var scope legacyRetirementScope
			if err := d.planNixLeftovers(context.Background(), &scope, false); err != nil || !strings.Contains(scope.Blocked, "not an unchanged file") {
				t.Fatal("new or changed service was not refused", scope, err)
			}
			if data, err := os.ReadFile(path); err != nil || string(data) != "personal replacement\n" {
				t.Fatal("personal file changed during preview", err)
			}
		})
	}
}

func TestLegacyDeterminateLeftoversUseTheirVendorRecipe(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("the Linux vendor unit recipe requires systemctl")
	}
	d, root := legacyStepFixture(t)
	d.systemRoot = filepath.Join(root, "system")
	d.Platform.Context = Context{OS: "linux", Arch: "amd64"}
	writeConfigFixture(t, d.sys("/usr/bin/systemctl"), "#!/bin/sh\nexit 99\n")
	path := d.sys("/etc/systemd/system/nix-daemon.service")
	writeConfigFixture(t, path, "recorded vendor unit\n")
	file, err := legacyFile(path)
	if err != nil {
		t.Fatal(err)
	}
	evidence := legacyDeterminateEvidence{Schema: 1, Receipt: []byte(legacyDeterminateLinuxReceipt), Installer: strings.Repeat("a", 64), Files: []legacyRetirementFile{file}}
	step, err := d.determinateEvidenceStep(evidence)
	if err != nil {
		t.Fatal(err)
	}
	if err := d.executeStep(context.Background(), legacyDeterminateIntent(t, d, "1", step), 0, false); err != nil {
		t.Fatal(err)
	}
	var scope legacyRetirementScope
	if err := d.planNixLeftovers(context.Background(), &scope, false); err != nil || scope.Blocked != "" {
		t.Fatal("a proved vendor unit was incorrectly treated as upstream integration", scope, err)
	}
	if !slices.ContainsFunc(scope.Steps, func(step legacyRetirementStep) bool {
		return step.Kind == "remove" && step.Path == path && slices.Contains(step.Before, file)
	}) {
		t.Fatal("the exact remaining vendor unit was not planned", scope)
	}
}

// legacyQueryDouble answers fixed system commands at the process boundary.
func legacyQueryDouble(t *testing.T, answers map[string]string) func(context.Context, nativeCommand) ([]byte, error) {
	return func(_ context.Context, command nativeCommand) ([]byte, error) {
		arguments := command.Arguments
		if command.Program == "/usr/bin/sudo" {
			arguments = arguments[3:] // -n -- /usr/bin/env
		}
		for i, argument := range arguments {
			if i > 0 && !strings.Contains(argument, "=") {
				key := strings.Join(arguments[i:], " ")
				if answer, ok := answers[key]; ok {
					return []byte(answer), nil
				}
				t.Fatalf("unexpected native query %q", key)
			}
		}
		t.Fatalf("invalid native query %v", command)
		return nil, nil
	}
}

func legacyPlistFixture(values map[string]string) string {
	text := `<?xml version="1.0" encoding="UTF-8"?><plist version="1.0"><dict>`
	for _, key := range slices.Sorted(maps.Keys(values)) {
		text += "<key>" + key + "</key><string>" + values[key] + "</string>"
	}
	return text + "</dict></plist>"
}

func TestLegacyDeterminateVolumeBindsNameDiskAndMount(t *testing.T) {
	const uuid, other = "11111111-2222-3333-4444-555555555555", "AAAAAAAA-2222-3333-4444-555555555555"
	mounted := map[string]string{"VolumeUUID": uuid, "MountPoint": "/nix", "VolumeName": "Nix Store", "FilesystemType": "apfs", "ParentWholeDisk": "disk3", "DeviceIdentifier": "disk3s7"}
	list := func(volumes ...string) string {
		text := `<?xml version="1.0" encoding="UTF-8"?><plist version="1.0"><dict><key>AllDisksAndPartitions</key><array><dict><key>APFSVolumes</key><array>`
		for _, volume := range volumes {
			name, device, _ := strings.Cut(volume, "@")
			text += "<dict><key>DeviceIdentifier</key><string>" + device + "</string><key>VolumeName</key><string>" + name + "</string></dict>"
		}
		return text + `</array></dict></array><key>VolumesFromDisks</key><array><string>Nix Store</string></array></dict></plist>`
	}
	volume := legacyDeterminateVolume{Disk: "disk3", Name: "Nix Store", Determinate: true, Deleted: true, Keychain: true}
	for _, item := range []struct {
		name, problem string
		mounted       map[string]string
		named         string
		listing       string
	}{
		{"exact", "", mounted, uuid, list("Macintosh HD@disk3s1", "Nix Store@disk3s7")},
		{"second volume with the vendor name", "2 volumes", mounted, uuid, list("Nix Store@disk3s7", "nix store@disk5s2")},
		{"name resolves elsewhere", "resolves to a different volume", mounted, other, list("Nix Store@disk3s7")},
		{"another container", "not the receipt's disk3", map[string]string{"VolumeUUID": uuid, "MountPoint": "/nix", "VolumeName": "Nix Store", "FilesystemType": "apfs", "ParentWholeDisk": "disk4", "DeviceIdentifier": "disk4s2"}, uuid, list("Nix Store@disk4s2")},
		{"not the mounted store", "is not the receipt's mounted APFS volume", map[string]string{"VolumeUUID": uuid, "MountPoint": "/Volumes/Nix Store", "VolumeName": "Nix Store", "FilesystemType": "apfs", "ParentWholeDisk": "disk3"}, uuid, list("Nix Store@disk3s7")},
	} {
		t.Run(item.name, func(t *testing.T) {
			named := maps.Clone(mounted)
			named["VolumeUUID"] = item.named
			d := &legacyRetirementDriver{Folders: ConfigFolders{Home: "/home/fixture"}, Environment: legacyRuntimeEnvironment{Username: "fixture"}}
			d.Query = legacyQueryDouble(t, map[string]string{
				"/usr/sbin/diskutil info -plist /nix":      legacyPlistFixture(item.mounted),
				"/usr/sbin/diskutil info -plist Nix Store": legacyPlistFixture(named),
				"/usr/sbin/diskutil list -plist":           item.listing,
			})
			identity, found, problem, err := d.determinateVolume(context.Background(), volume)
			if err != nil || !strings.Contains(problem, item.problem) || item.problem == "" && (problem != "" || found != uuid || !strings.Contains(identity, "Device=disk3s7")) {
				t.Fatalf("identity %q uuid %q problem %q err %v", identity, found, problem, err)
			}
		})
	}
}

func TestLegacyDeterminateGroupConsumersAreOtherPrimaryAccounts(t *testing.T) {
	r, err := parseLegacyDeterminateReceipt([]byte(legacyDeterminateLinuxReceipt))
	if err != nil {
		t.Fatal(err)
	}
	d := &legacyRetirementDriver{Platform: NativePlatform{Context: Context{OS: "linux"}}, Folders: ConfigFolders{Home: "/home/fixture"}, Environment: legacyRuntimeEnvironment{Username: "fixture"}}
	accounts := "root:x:0:0:root:/root:/bin/bash\nnixbld1:x:30001:30000:Nix build user 1:/var/empty:/sbin/nologin\n"
	for passwd, consumer := range map[string]string{accounts: "", accounts + "builder:x:1001:30000::/home/builder:/bin/sh\n": "builder"} {
		d.Query = legacyQueryDouble(t, map[string]string{"/usr/bin/getent passwd": passwd})
		problem, err := d.determinateGroupConsumers(context.Background(), r)
		if err != nil || consumer == "" && problem != "" || consumer != "" && !strings.Contains(problem, consumer) {
			t.Fatal(problem, err)
		}
	}
}

// Only the init system's main process executing the provisioned binary is
// excluded from the consumer scan; any other process stays a consumer.
func TestLegacyDeterminateDaemonExclusionNeedsKernelExecutable(t *testing.T) {
	if _, err := legacySystemProgram("/usr/bin/systemctl", "/bin/systemctl"); err != nil {
		t.Skip("the Linux daemon proof needs a host systemctl path")
	}
	if _, err := legacySystemProgram("/usr/bin/readlink", "/bin/readlink"); err != nil {
		t.Skip("the Linux daemon proof needs a host readlink path")
	}
	systemctl, _ := legacySystemProgram("/usr/bin/systemctl", "/bin/systemctl")
	readlink, _ := legacySystemProgram("/usr/bin/readlink", "/bin/readlink")
	for _, item := range []struct{ pid, executable, want string }{
		{"4242\n", legacyDeterminateNixdPath + "\n", "4242"},
		{"4242\n", "/nix/store/fixture-nix/bin/nix\n", ""},
		{"4242\n", legacyDeterminateNixdPath + " (deleted)\n", ""},
		{"0\n", "", ""},
	} {
		d := &legacyRetirementDriver{Platform: NativePlatform{Context: Context{OS: "linux"}}, Folders: ConfigFolders{Home: "/home/fixture"},
			Environment: legacyRuntimeEnvironment{Username: "fixture", Root: true}}
		d.Query = legacyQueryDouble(t, map[string]string{
			systemctl + " show --property=MainPID --value nix-daemon.service": item.pid,
			readlink + " -- /proc/4242/exe":                                   item.executable,
		})
		if pid, err := d.determinateDaemonProcess(context.Background()); err != nil || pid != item.want {
			t.Fatal(item, pid, err)
		}
	}
	d := &legacyRetirementDriver{Platform: NativePlatform{Context: Context{OS: "darwin"}}, Folders: ConfigFolders{Home: "/Users/fixture"},
		Environment: legacyRuntimeEnvironment{Username: "fixture", Root: true}}
	for txt, want := range map[string]string{"p77\nftxt\nn" + legacyDeterminateNixdPath + "\nftxt\nn/usr/lib/dyld\n": "77", "p77\nftxt\nn/tmp/impostor\nftxt\nn" + legacyDeterminateNixdPath + "\n": ""} {
		d.Query = legacyQueryDouble(t, map[string]string{
			"/bin/launchctl print system/systems.determinate.nix-daemon": "\tprogram = " + legacyDeterminateNixdPath + "\n\tpid = 77\n",
			"/usr/sbin/lsof -n -a -p 77 -d txt -F fn":                    txt,
		})
		if pid, err := d.determinateDaemonProcess(context.Background()); err != nil || pid != want {
			t.Fatal(txt, pid, err)
		}
	}
}

// A leftover vendor block is removed exactly as the vendor would remove it,
// through the reviewed root edit step that keeps the original beside it.
func TestLegacyDeterminateBlockRemovalKeepsPersonalBytes(t *testing.T) {
	d, root := legacyStepFixture(t)
	path := filepath.Join(root, "zshrc")
	original := legacyDeterminateShellHook + "# personal\nalias ll='ls -l'\n"
	writeConfigFixture(t, path, original)
	var scope legacyRetirementScope
	if err := d.addDeterminateBlockRemoval(context.Background(), &scope, legacyDeterminateInsert{Path: path, Buf: legacyDeterminateShellHook, Reverted: true}); err != nil || scope.Blocked != "" || len(scope.Steps) != 1 {
		t.Fatal(scope, err)
	}
	if err := d.executeStep(context.Background(), legacyDeterminateIntent(t, d, "1", scope.Steps...), 0, false); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(path); err != nil || string(data) != "# personal\nalias ll='ls -l'\n" {
		t.Fatal("personal shell bytes changed", string(data), err)
	}
	if data, err := os.ReadFile(scope.Steps[0].Backup); err != nil || string(data) != original {
		t.Fatal("the original file was not kept", err)
	}
	scope = legacyRetirementScope{}
	if err := d.addDeterminateBlockRemoval(context.Background(), &scope, legacyDeterminateInsert{Path: path, Buf: legacyDeterminateShellHook, Reverted: true}); err != nil || len(scope.Steps) != 0 {
		t.Fatal("a removed block needs no further edit", scope, err)
	}
}

// DOTFILES_DETERMINATE_FIXTURES names a directory holding unreduced official
// fixtures, such as tests/fixtures of an official checkout or files exported
// from several tags. Linux and macOS receipts must validate; other planners
// must be refused.
func TestLegacyDeterminateOfficialFixtures(t *testing.T) {
	root := os.Getenv("DOTFILES_DETERMINATE_FIXTURES")
	if root == "" {
		t.Skip("set DOTFILES_DETERMINATE_FIXTURES to an official nix-installer fixture directory")
	}
	checked := 0
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() || filepath.Ext(path) != ".json" {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		r, parseErr := parseLegacyDeterminateReceipt(data)
		switch filepath.Base(path) {
		case "linux.json", "macos.json":
			if parseErr != nil || r.Planner+".json" != filepath.Base(path) || len(r.Users) != 32 {
				t.Errorf("%s: %v %+v", path, parseErr, r)
			}
		default:
			if parseErr == nil {
				t.Errorf("%s: an unreviewed planner was accepted", path)
			}
		}
		checked++
		return nil
	})
	if err != nil || checked == 0 {
		t.Fatal("no official fixtures were checked", err)
	}
}

// Official nix-installer v3.23.3 tests/fixtures/linux/linux.json.
const legacyDeterminateLinuxReceipt = `{"version":"3.23.3","actions":[{"action":{"action_name":"create_directory","path":"/nix","user":null,"group":null,"mode":493,"is_mountpoint":false,"force_prune_on_revert":true},"state":"Completed"},{"action":{"action_name":"provision_determinate_nixd","binary_location":"/usr/local/bin/determinate-nixd"},"state":"Completed"},{"action":{"action_name":"provision_nix","nix_store_gid":30000,"fetch_nix":{"action":{"action_name":"fetch_and_unpack_nix","distribution":"DeterminateNix","url_or_path":null,"dest":"/nix/temp-install-dir","proxy":null,"ssl_cert_file":null},"state":"Completed"},"create_nix_tree":{"action":{"action_name":"create_nix_tree","create_directories":[{"action":{"action_name":"create_directory","path":"/nix/var","user":null,"group":null,"mode":493,"is_mountpoint":false,"force_prune_on_revert":true},"state":"Completed"},{"action":{"action_name":"create_directory","path":"/nix/var/log","user":null,"group":null,"mode":493,"is_mountpoint":false,"force_prune_on_revert":true},"state":"Completed"},{"action":{"action_name":"create_directory","path":"/nix/var/log/nix","user":null,"group":null,"mode":493,"is_mountpoint":false,"force_prune_on_revert":true},"state":"Completed"},{"action":{"action_name":"create_directory","path":"/nix/var/log/nix/drvs","user":null,"group":null,"mode":493,"is_mountpoint":false,"force_prune_on_revert":true},"state":"Completed"},{"action":{"action_name":"create_directory","path":"/nix/var/nix","user":null,"group":null,"mode":493,"is_mountpoint":false,"force_prune_on_revert":true},"state":"Completed"},{"action":{"action_name":"create_directory","path":"/nix/var/nix/db","user":null,"group":null,"mode":493,"is_mountpoint":false,"force_prune_on_revert":true},"state":"Completed"},{"action":{"action_name":"create_directory","path":"/nix/var/nix/gcroots","user":null,"group":null,"mode":493,"is_mountpoint":false,"force_prune_on_revert":true},"state":"Completed"},{"action":{"action_name":"create_directory","path":"/nix/var/nix/gcroots/per-user","user":null,"group":null,"mode":493,"is_mountpoint":false,"force_prune_on_revert":true},"state":"Completed"},{"action":{"action_name":"create_directory","path":"/nix/var/nix/profiles","user":null,"group":null,"mode":493,"is_mountpoint":false,"force_prune_on_revert":true},"state":"Completed"},{"action":{"action_name":"create_directory","path":"/nix/var/nix/profiles/per-user","user":null,"group":null,"mode":493,"is_mountpoint":false,"force_prune_on_revert":true},"state":"Completed"},{"action":{"action_name":"create_directory","path":"/nix/var/nix/temproots","user":null,"group":null,"mode":493,"is_mountpoint":false,"force_prune_on_revert":true},"state":"Completed"},{"action":{"action_name":"create_directory","path":"/nix/var/nix/userpool","user":null,"group":null,"mode":493,"is_mountpoint":false,"force_prune_on_revert":true},"state":"Completed"},{"action":{"action_name":"create_directory","path":"/nix/var/nix/daemon-socket","user":null,"group":null,"mode":493,"is_mountpoint":false,"force_prune_on_revert":true},"state":"Completed"}]},"state":"Completed"},"move_unpacked_nix":{"action":{"action_name":"mount_unpacked_nix","unpacked_path":"/nix/temp-install-dir"},"state":"Completed"}},"state":"Completed"},{"action":{"action_name":"create_users_and_group","nix_build_group_name":"nixbld","nix_build_group_id":30000,"nix_build_user_count":1,"nix_build_user_prefix":"nixbld","nix_build_user_id_base":30000,"create_group":{"action":{"action_name":"create_group","name":"nixbld","gid":30000},"state":"Completed"},"create_users":[{"action":{"action_name":"create_user","name":"nixbld1","uid":30001,"groupname":"nixbld","gid":30000,"comment":"Nix build user 1"},"state":"Completed"}],"add_users_to_groups":[{"action":{"action_name":"add_user_to_group","name":"nixbld1","uid":30001,"groupname":"nixbld","gid":30000},"state":"Completed"}]},"state":"Completed"},{"action":{"action_name":"configure_nix","setup_default_profile":{"action":{"action_name":"setup_default_profile","unpacked_path":"/nix/temp-install-dir"},"state":"Completed"},"configure_shell_profile":{"action":{"action_name":"configure_shell_profile","locations":{"fish":{"confd_suffix":"conf.d/nix.fish","confd_prefixes":["/etc/fish","/usr/local/etc/fish","/opt/homebrew/etc/fish","/opt/local/etc/fish"],"vendor_confd_suffix":"vendor_conf.d/nix.fish","vendor_confd_prefixes":["/usr/share/fish/","/usr/local/share/fish/"]},"bash":["/etc/bashrc","/etc/profile.d/nix.sh","/etc/bash.bashrc"],"zsh":["/etc/zshrc","/etc/zsh/zshrc"]},"create_directories":[{"action":{"action_name":"create_directory","path":"/etc/zsh","user":null,"group":null,"mode":493,"is_mountpoint":false,"force_prune_on_revert":false},"state":"Completed"},{"action":{"action_name":"create_directory","path":"/usr/share/fish/vendor_conf.d","user":null,"group":null,"mode":493,"is_mountpoint":false,"force_prune_on_revert":false},"state":"Completed"}],"create_or_insert_into_files":[{"action":{"action_name":"create_or_insert_into_file","path":"/etc/bashrc","user":null,"group":null,"mode":420,"buf":"\n# Nix\nif [ -e '/nix/var/nix/profiles/default/etc/profile.d/nix-daemon.sh' ]; then\n    . '/nix/var/nix/profiles/default/etc/profile.d/nix-daemon.sh'\nfi\n# End Nix\n\n        \n","position":"Beginning"},"state":"Completed"},{"action":{"action_name":"create_or_insert_into_file","path":"/etc/profile.d/nix.sh","user":null,"group":null,"mode":420,"buf":"\n# Nix\nif [ -e '/nix/var/nix/profiles/default/etc/profile.d/nix-daemon.sh' ]; then\n    . '/nix/var/nix/profiles/default/etc/profile.d/nix-daemon.sh'\nfi\n# End Nix\n\n        \n","position":"Beginning"},"state":"Completed"},{"action":{"action_name":"create_or_insert_into_file","path":"/etc/bash.bashrc","user":null,"group":null,"mode":420,"buf":"\n# Nix\nif [ -e '/nix/var/nix/profiles/default/etc/profile.d/nix-daemon.sh' ]; then\n    . '/nix/var/nix/profiles/default/etc/profile.d/nix-daemon.sh'\nfi\n# End Nix\n\n        \n","position":"Beginning"},"state":"Completed"},{"action":{"action_name":"create_or_insert_into_file","path":"/etc/zshrc","user":null,"group":null,"mode":420,"buf":"\n# Nix\nif [ -e '/nix/var/nix/profiles/default/etc/profile.d/nix-daemon.sh' ]; then\n    . '/nix/var/nix/profiles/default/etc/profile.d/nix-daemon.sh'\nfi\n# End Nix\n\n        \n","position":"Beginning"},"state":"Completed"},{"action":{"action_name":"create_or_insert_into_file","path":"/etc/zsh/zshrc","user":null,"group":null,"mode":420,"buf":"\n# Nix\nif [ -e '/nix/var/nix/profiles/default/etc/profile.d/nix-daemon.sh' ]; then\n    . '/nix/var/nix/profiles/default/etc/profile.d/nix-daemon.sh'\nfi\n# End Nix\n\n        \n","position":"Beginning"},"state":"Completed"},{"action":{"action_name":"create_or_insert_into_file","path":"/usr/share/fish/vendor_conf.d/nix.fish","user":null,"group":null,"mode":420,"buf":"\n# Nix\nif test -e '/nix/var/nix/profiles/default/etc/profile.d/nix-daemon.fish'\n    . '/nix/var/nix/profiles/default/etc/profile.d/nix-daemon.fish'\nend\n# End Nix\n\n","position":"Beginning"},"state":"Completed"}]},"state":"Completed"},"place_nix_configuration":{"action":{"action_name":"place_nix_configuration","create_directory":{"action":{"action_name":"create_directory","path":"/etc/nix","user":null,"group":null,"mode":493,"is_mountpoint":false,"force_prune_on_revert":false},"state":"Completed"},"create_or_merge_standard_nix_config":null,"create_or_merge_custom_nix_config":{"action":{"action_name":"create_or_merge_nix_config","path":"/etc/nix/nix.custom.conf","pending_nix_config":{"settings":{}},"header":"# Written by https://github.com/DeterminateSystems/nix-installer.\n# The contents below are based on options specified at installation time.\n","footer":null},"state":"Completed"}},"state":"Completed"}},"state":"Completed"},{"action":{"action_name":"create_directory","path":"/etc/tmpfiles.d","user":null,"group":null,"mode":493,"is_mountpoint":false,"force_prune_on_revert":false},"state":"Completed"},{"action":{"action_name":"configure_determinate_nixd_init_service","init":"Systemd","configure_init_service":{"action":{"action_name":"configure_init_service","init":"Systemd","start_daemon":true,"service_src":{"Literal":"[Unit]\nDescription=Nix Daemon, with Determinate Nix superpowers.\nDocumentation=man:nix-daemon https://determinate.systems\nRequiresMountsFor=/nix/store\nRequiresMountsFor=/nix/var\nRequiresMountsFor=/nix/var/nix/db\nConditionPathIsReadWrite=/nix/var/nix/daemon-socket\n\n[Service]\nExecStart=@/usr/local/bin/determinate-nixd determinate-nixd daemon\nKillMode=process\nLimitNOFILE=1048576\nLimitSTACK=64M\nTasksMax=1048576\n\n[Install]\nWantedBy=multi-user.target\n"},"service_name":"nix-daemon.service","service_dest":"/etc/systemd/system/nix-daemon.service","socket_files":[{"name":"nix-daemon.socket","src":{"Literal":"[Unit]\nDescription=Determinate Nix Daemon Socket\nBefore=multi-user.target\nRequiresMountsFor=/nix/store\nRequiresMountsFor=/nix/var\nRequiresMountsFor=/nix/var/nix/db\nConditionPathIsReadWrite=/nix/var/nix/daemon-socket\n\n[Socket]\nFileDescriptorName=nix-daemon.socket\nListenStream=/nix/var/nix/daemon-socket/socket\n\n[Install]\nWantedBy=sockets.target\n"},"dest":"/etc/systemd/system/nix-daemon.socket"},{"name":"determinate-nixd.socket","src":{"Literal":"[Unit]\nDescription=Determinate Nixd Daemon Socket\nBefore=multi-user.target\nRequiresMountsFor=/nix/store\nRequiresMountsFor=/nix/var/determinate\n\n[Socket]\nFileDescriptorName=determinate-nixd.socket\nDirectoryMode=0755\nListenStream=/nix/var/determinate/determinate-nixd.socket\nService=nix-daemon.service\n\n[Install]\nWantedBy=sockets.target\n"},"dest":"/etc/systemd/system/determinate-nixd.socket"}]},"state":"Completed"}},"state":"Completed"},{"action":{"action_name":"cleanup","remove_scratch_dir":{"action":{"action_name":"remove_directory","path":"/nix/temp-install-dir"},"state":"Completed"},"original_home_dir":"/home/ubuntu"},"state":"Completed"}],"planner":{"planner":"linux","settings":{"determinate_nix":false,"prefer_upstream":false,"modify_profile":true,"nix_build_group_name":"nixbld","nix_build_group_id":30000,"nix_build_user_prefix":"nixbld","nix_build_user_count":1,"nix_build_user_id_base":30000,"nix_package_url":null,"proxy":null,"ssl_cert_file":null,"extra_conf":[],"force":false,"skip_nix_conf":false},"init":{"init":"Systemd","start_daemon":true}}}`

// Official nix-installer v3.23.3 tests/fixtures/macos/macos.json.
const legacyDeterminateMacReceipt = `{"version":"3.23.3","actions":[{"action":{"action_name":"provision_determinate_nixd","binary_location":"/usr/local/bin/determinate-nixd"},"state":"Completed"},{"action":{"action_name":"create_determinate_nix_volume","disk":"disk4","name":"Nix Store","case_sensitive":false,"use_ec2_instance_store":false,"create_directory":{"action":{"action_name":"create_directory","path":"/etc/nix","user":null,"group":null,"mode":493,"is_mountpoint":false,"force_prune_on_revert":false},"state":"Completed"},"create_or_append_synthetic_conf":{"action":{"action_name":"create_or_insert_into_file","path":"/etc/synthetic.conf","user":null,"group":null,"mode":null,"buf":"nix\n","position":"End"},"state":"Completed"},"create_synthetic_objects":{"action":null,"state":"Completed"},"unmount_volume":{"action":{"action_name":"unmount_apfs_volume","disk":"disk4","name":"Nix Store"},"state":"Progress"},"create_volume":{"action":{"action_name":"create_apfs_volume","disk":"disk4","name":"Nix Store","case_sensitive":false},"state":"Completed"},"create_fstab_entry":{"action":{"action_name":"create_fstab_entry","apfs_volume_label":"Nix Store"},"state":"Completed"},"encrypt_volume":{"action":{"action_name":"encrypt_apfs_volume","distribution":"DeterminateNix","disk":"disk4","name":"Nix Store"},"state":"Completed"},"setup_volume_daemon":{"action":{"action_name":"create_determinate_volume_service","path":"/Library/LaunchDaemons/systems.determinate.nix-store.plist","mount_service_label":"systems.determinate.nix-store","needs_bootout":false,"use_ec2_instance_store":false},"state":"Completed"},"bootstrap_volume":{"action":{"action_name":"bootstrap_launchctl_service","service":"systems.determinate.nix-store","path":"/Library/LaunchDaemons/systems.determinate.nix-store.plist","is_present":false,"is_disabled":false},"state":"Completed"},"kickstart_launchctl_service":{"action":{"action_name":"kickstart_launchctl_service","domain":"system","service":"systems.determinate.nix-store"},"state":"Completed"},"enable_ownership":{"action":{"action_name":"enable_ownership","path":"/nix"},"state":"Completed"}},"state":"Completed"},{"action":{"action_name":"provision_nix","nix_store_gid":350,"fetch_nix":{"action":{"action_name":"fetch_and_unpack_nix","distribution":"DeterminateNix","url_or_path":null,"dest":"/nix/temp-install-dir","proxy":null,"ssl_cert_file":null},"state":"Completed"},"create_nix_tree":{"action":{"action_name":"create_nix_tree","create_directories":[{"action":{"action_name":"create_directory","path":"/nix/var","user":null,"group":null,"mode":493,"is_mountpoint":false,"force_prune_on_revert":true},"state":"Completed"},{"action":{"action_name":"create_directory","path":"/nix/var/log","user":null,"group":null,"mode":493,"is_mountpoint":false,"force_prune_on_revert":true},"state":"Completed"},{"action":{"action_name":"create_directory","path":"/nix/var/log/nix","user":null,"group":null,"mode":493,"is_mountpoint":false,"force_prune_on_revert":true},"state":"Completed"},{"action":{"action_name":"create_directory","path":"/nix/var/log/nix/drvs","user":null,"group":null,"mode":493,"is_mountpoint":false,"force_prune_on_revert":true},"state":"Completed"},{"action":{"action_name":"create_directory","path":"/nix/var/nix","user":null,"group":null,"mode":493,"is_mountpoint":false,"force_prune_on_revert":true},"state":"Completed"},{"action":{"action_name":"create_directory","path":"/nix/var/nix/db","user":null,"group":null,"mode":493,"is_mountpoint":false,"force_prune_on_revert":true},"state":"Completed"},{"action":{"action_name":"create_directory","path":"/nix/var/nix/gcroots","user":null,"group":null,"mode":493,"is_mountpoint":false,"force_prune_on_revert":true},"state":"Completed"},{"action":{"action_name":"create_directory","path":"/nix/var/nix/gcroots/per-user","user":null,"group":null,"mode":493,"is_mountpoint":false,"force_prune_on_revert":true},"state":"Completed"},{"action":{"action_name":"create_directory","path":"/nix/var/nix/profiles","user":null,"group":null,"mode":493,"is_mountpoint":false,"force_prune_on_revert":true},"state":"Completed"},{"action":{"action_name":"create_directory","path":"/nix/var/nix/profiles/per-user","user":null,"group":null,"mode":493,"is_mountpoint":false,"force_prune_on_revert":true},"state":"Completed"},{"action":{"action_name":"create_directory","path":"/nix/var/nix/temproots","user":null,"group":null,"mode":493,"is_mountpoint":false,"force_prune_on_revert":true},"state":"Completed"},{"action":{"action_name":"create_directory","path":"/nix/var/nix/userpool","user":null,"group":null,"mode":493,"is_mountpoint":false,"force_prune_on_revert":true},"state":"Completed"},{"action":{"action_name":"create_directory","path":"/nix/var/nix/daemon-socket","user":null,"group":null,"mode":493,"is_mountpoint":false,"force_prune_on_revert":true},"state":"Completed"}]},"state":"Completed"},"move_unpacked_nix":{"action":{"action_name":"mount_unpacked_nix","unpacked_path":"/nix/temp-install-dir"},"state":"Completed"}},"state":"Completed"},{"action":{"action_name":"create_users_and_group","nix_build_group_name":"nixbld","nix_build_group_id":350,"nix_build_user_count":1,"nix_build_user_prefix":"_nixbld","nix_build_user_id_base":350,"create_group":{"action":{"action_name":"create_group","name":"nixbld","gid":350},"state":"Completed"},"create_users":[{"action":{"action_name":"create_user","name":"_nixbld1","uid":351,"groupname":"nixbld","gid":350,"comment":"Nix build user 1"},"state":"Completed"}],"add_users_to_groups":[{"action":{"action_name":"add_user_to_group","name":"_nixbld1","uid":351,"groupname":"nixbld","gid":350},"state":"Completed"}]},"state":"Completed"},{"action":{"action_name":"set_tmutil_exclusions","set_tmutil_exclusions":[{"action":{"action_name":"set_tmutil_exclusion","path":"/nix/store"},"state":"Completed"},{"action":{"action_name":"set_tmutil_exclusion","path":"/nix/var"},"state":"Completed"}]},"state":"Completed"},{"action":{"action_name":"configure_nix","setup_default_profile":{"action":{"action_name":"setup_default_profile","unpacked_path":"/nix/temp-install-dir"},"state":"Completed"},"configure_shell_profile":{"action":{"action_name":"configure_shell_profile","locations":{"fish":{"confd_suffix":"conf.d/nix.fish","confd_prefixes":["/etc/fish","/usr/local/etc/fish","/opt/homebrew/etc/fish","/opt/local/etc/fish"],"vendor_confd_suffix":"vendor_conf.d/nix.fish","vendor_confd_prefixes":["/usr/share/fish/","/usr/local/share/fish/"]},"bash":["/etc/bashrc","/etc/profile.d/nix.sh","/etc/bash.bashrc"],"zsh":["/etc/zshrc","/etc/zsh/zshrc"]},"create_directories":[{"action":{"action_name":"create_directory","path":"/etc/profile.d","user":null,"group":null,"mode":493,"is_mountpoint":false,"force_prune_on_revert":false},"state":"Completed"},{"action":{"action_name":"create_directory","path":"/etc/zsh","user":null,"group":null,"mode":493,"is_mountpoint":false,"force_prune_on_revert":false},"state":"Completed"}],"create_or_insert_into_files":[{"action":{"action_name":"create_or_insert_into_file","path":"/etc/bashrc","user":null,"group":null,"mode":420,"buf":"\n# Nix\nif [ -e '/nix/var/nix/profiles/default/etc/profile.d/nix-daemon.sh' ]; then\n    . '/nix/var/nix/profiles/default/etc/profile.d/nix-daemon.sh'\nfi\n# End Nix\n\n        \n","position":"Beginning"},"state":"Completed"},{"action":{"action_name":"create_or_insert_into_file","path":"/etc/profile.d/nix.sh","user":null,"group":null,"mode":420,"buf":"\n# Nix\nif [ -e '/nix/var/nix/profiles/default/etc/profile.d/nix-daemon.sh' ]; then\n    . '/nix/var/nix/profiles/default/etc/profile.d/nix-daemon.sh'\nfi\n# End Nix\n\n        \n","position":"Beginning"},"state":"Completed"},{"action":{"action_name":"create_or_insert_into_file","path":"/etc/bash.bashrc","user":null,"group":null,"mode":420,"buf":"\n# Nix\nif [ -e '/nix/var/nix/profiles/default/etc/profile.d/nix-daemon.sh' ]; then\n    . '/nix/var/nix/profiles/default/etc/profile.d/nix-daemon.sh'\nfi\n# End Nix\n\n        \n","position":"Beginning"},"state":"Completed"},{"action":{"action_name":"create_or_insert_into_file","path":"/etc/zshrc","user":null,"group":null,"mode":420,"buf":"\n# Nix\nif [ -e '/nix/var/nix/profiles/default/etc/profile.d/nix-daemon.sh' ]; then\n    . '/nix/var/nix/profiles/default/etc/profile.d/nix-daemon.sh'\nfi\n# End Nix\n\n        \n","position":"Beginning"},"state":"Completed"},{"action":{"action_name":"create_or_insert_into_file","path":"/etc/zsh/zshrc","user":null,"group":null,"mode":420,"buf":"\n# Nix\nif [ -e '/nix/var/nix/profiles/default/etc/profile.d/nix-daemon.sh' ]; then\n    . '/nix/var/nix/profiles/default/etc/profile.d/nix-daemon.sh'\nfi\n# End Nix\n\n        \n","position":"Beginning"},"state":"Completed"}]},"state":"Completed"},"place_nix_configuration":{"action":{"action_name":"place_nix_configuration","create_directory":{"action":{"action_name":"create_directory","path":"/etc/nix","user":null,"group":null,"mode":493,"is_mountpoint":false,"force_prune_on_revert":false},"state":"Completed"},"create_or_merge_standard_nix_config":null,"create_or_merge_custom_nix_config":{"action":{"action_name":"create_or_merge_nix_config","path":"/etc/nix/nix.custom.conf","pending_nix_config":{"settings":{}},"header":"# Written by https://github.com/DeterminateSystems/nix-installer.\n# The contents below are based on options specified at installation time.\n","footer":null},"state":"Completed"}},"state":"Completed"}},"state":"Completed"},{"action":{"action_name":"configure_remote_building","create_or_insert_into_file":{"action":{"action_name":"create_or_insert_into_file","path":"/etc/zshenv","user":null,"group":null,"mode":420,"buf":"\n# Set up Nix only on SSH connections\n# See: https://github.com/DeterminateSystems/nix-installer/pull/714\nif [ -e '/nix/var/nix/profiles/default/etc/profile.d/nix-daemon.sh' ] && [ -n \"${SSH_CONNECTION:-}\" ] && [ \"${SHLVL:-0}\" -eq 1 ]; then\n    . '/nix/var/nix/profiles/default/etc/profile.d/nix-daemon.sh'\nfi\n# End Nix\n","position":"Beginning"},"state":"Completed"}},"state":"Completed"},{"action":{"action_name":"create_nix_hook_service","path":"/Library/LaunchDaemons/systems.determinate.nix-installer.nix-hook.plist","service_label":"systems.determinate.nix-installer.nix-hook","needs_bootout":false},"state":"Completed"},{"action":{"action_name":"configure_determinate_nixd_init_service","init":"Launchd","configure_init_service":{"action":{"action_name":"configure_init_service","init":"Launchd","start_daemon":true,"service_src":{"Literal":"<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<!DOCTYPE plist PUBLIC \"-//Apple//DTD PLIST 1.0//EN\" \"http://www.apple.com/DTDs/PropertyList-1.0.dtd\">\n<plist version=\"1.0\">\n<dict>\n\t<key>Label</key>\n\t<string>systems.determinate.nix-daemon</string>\n\t<key>ProgramArguments</key>\n\t<array>\n\t\t<string>/usr/local/bin/determinate-nixd</string>\n\t\t<string>daemon</string>\n\t</array>\n\t<key>RunAtLoad</key>\n\t<false/>\n\t<key>Sockets</key>\n\t<dict>\n\t\t<key>determinate-nixd.socket</key>\n\t\t<dict>\n\t\t\t<key>SockFamily</key>\n\t\t\t<string>Unix</string>\n\t\t\t<key>SockPassive</key>\n\t\t\t<true/>\n\t\t\t<key>SockPathName</key>\n\t\t\t<string>/var/run/determinate-nixd.socket</string>\n\t\t</dict>\n\t\t<key>nix-daemon.socket</key>\n\t\t<dict>\n\t\t\t<key>SockFamily</key>\n\t\t\t<string>Unix</string>\n\t\t\t<key>SockPassive</key>\n\t\t\t<true/>\n\t\t\t<key>SockPathName</key>\n\t\t\t<string>/var/run/nix-daemon.socket</string>\n\t\t</dict>\n\t</dict>\n\t<key>StandardErrorPath</key>\n\t<string>/var/log/determinate-nix-daemon.log</string>\n\t<key>StandardOutPath</key>\n\t<string>/var/log/determinate-nix-daemon.log</string>\n\t<key>SoftResourceLimits</key>\n\t<dict>\n\t\t<key>NumberOfFiles</key>\n\t\t<integer>1048576</integer>\n\t\t<key>NumberOfProcesses</key>\n\t\t<integer>1048576</integer>\n\t\t<key>Stack</key>\n\t\t<integer>67108864</integer>\n\t</dict>\n\t<key>HardResourceLimits</key>\n\t<dict>\n\t\t<key>NumberOfFiles</key>\n\t\t<integer>1048576</integer>\n\t\t<key>NumberOfProcesses</key>\n\t\t<integer>1048576</integer>\n\t\t<key>Stack</key>\n\t\t<integer>67108864</integer>\n\t</dict>\n</dict>\n</plist>"},"service_name":"systems.determinate.nix-daemon","service_dest":"/Library/LaunchDaemons/systems.determinate.nix-daemon.plist","socket_files":[{"name":"nix-daemon.socket","src":{"Literal":"[Unit]\nDescription=Determinate Nix Daemon Socket\nBefore=multi-user.target\nRequiresMountsFor=/nix/store\nRequiresMountsFor=/nix/var\nRequiresMountsFor=/nix/var/nix/db\nConditionPathIsReadWrite=/nix/var/nix/daemon-socket\n\n[Socket]\nFileDescriptorName=nix-daemon.socket\nListenStream=/nix/var/nix/daemon-socket/socket\n\n[Install]\nWantedBy=sockets.target\n"},"dest":"/etc/systemd/system/nix-daemon.socket"},{"name":"determinate-nixd.socket","src":{"Literal":"[Unit]\nDescription=Determinate Nixd Daemon Socket\nBefore=multi-user.target\nRequiresMountsFor=/nix/store\nRequiresMountsFor=/nix/var/determinate\n\n[Socket]\nFileDescriptorName=determinate-nixd.socket\nDirectoryMode=0755\nListenStream=/nix/var/determinate/determinate-nixd.socket\nService=nix-daemon.service\n\n[Install]\nWantedBy=sockets.target\n"},"dest":"/etc/systemd/system/determinate-nixd.socket"}]},"state":"Completed"}},"state":"Completed"},{"action":{"action_name":"remove_directory","path":"/nix/temp-install-dir"},"state":"Completed"}],"planner":{"planner":"macos","settings":{"determinate_nix":false,"prefer_upstream":false,"modify_profile":true,"nix_build_group_name":"nixbld","nix_build_group_id":350,"nix_build_user_prefix":"_nixbld","nix_build_user_count":1,"nix_build_user_id_base":350,"nix_package_url":null,"proxy":null,"ssl_cert_file":null,"extra_conf":[],"force":false,"skip_nix_conf":false},"encrypt":null,"case_sensitive":false,"volume_label":"Nix Store","root_disk":"disk4","use_ec2_instance_store":false}}`

// Official nix-installer v3.14.0 tests/fixtures/macos/macos.json.
const legacyDeterminateMacUpstreamReceipt = `{"version":"3.14.0","actions":[{"action":{"action_name":"create_nix_volume","disk":"disk4","name":"Nix Store","case_sensitive":false,"encrypt":false,"create_or_append_synthetic_conf":{"action":{"action_name":"create_or_insert_into_file","path":"/etc/synthetic.conf","user":null,"group":null,"mode":null,"buf":"nix\n","position":"End"},"state":"Completed"},"create_synthetic_objects":{"action":null,"state":"Completed"},"unmount_volume":{"action":{"action_name":"unmount_apfs_volume","disk":"disk4","name":"Nix Store"},"state":"Progress"},"create_volume":{"action":{"action_name":"create_apfs_volume","disk":"disk4","name":"Nix Store","case_sensitive":false},"state":"Completed"},"create_fstab_entry":{"action":{"action_name":"create_fstab_entry","apfs_volume_label":"Nix Store"},"state":"Completed"},"encrypt_volume":null,"setup_volume_daemon":{"action":{"action_name":"create_volume_service","path":"/Library/LaunchDaemons/org.nixos.darwin-store.plist","apfs_volume_label":"Nix Store","mount_service_label":"org.nixos.darwin-store","mount_point":"/nix","encrypt":false,"needs_bootout":false},"state":"Completed"},"bootstrap_volume":{"action":{"action_name":"bootstrap_launchctl_service","service":"org.nixos.darwin-store","path":"/Library/LaunchDaemons/org.nixos.darwin-store.plist","is_present":false,"is_disabled":false},"state":"Completed"},"kickstart_launchctl_service":{"action":{"action_name":"kickstart_launchctl_service","domain":"system","service":"org.nixos.darwin-store"},"state":"Completed"},"enable_ownership":{"action":{"action_name":"enable_ownership","path":"/nix"},"state":"Completed"}},"state":"Completed"},{"action":{"action_name":"provision_nix","nix_store_gid":350,"fetch_nix":{"action":{"action_name":"fetch_and_unpack_nix","url_or_path":null,"dest":"/nix/temp-install-dir","proxy":null,"ssl_cert_file":null,"distribution":"Nix"},"state":"Completed"},"create_nix_tree":{"action":{"action_name":"create_nix_tree","create_directories":[{"action":{"action_name":"create_directory","path":"/nix/var","user":null,"group":null,"mode":493,"is_mountpoint":false,"force_prune_on_revert":true},"state":"Completed"},{"action":{"action_name":"create_directory","path":"/nix/var/log","user":null,"group":null,"mode":493,"is_mountpoint":false,"force_prune_on_revert":true},"state":"Completed"},{"action":{"action_name":"create_directory","path":"/nix/var/log/nix","user":null,"group":null,"mode":493,"is_mountpoint":false,"force_prune_on_revert":true},"state":"Completed"},{"action":{"action_name":"create_directory","path":"/nix/var/log/nix/drvs","user":null,"group":null,"mode":493,"is_mountpoint":false,"force_prune_on_revert":true},"state":"Completed"},{"action":{"action_name":"create_directory","path":"/nix/var/nix","user":null,"group":null,"mode":493,"is_mountpoint":false,"force_prune_on_revert":true},"state":"Completed"},{"action":{"action_name":"create_directory","path":"/nix/var/nix/db","user":null,"group":null,"mode":493,"is_mountpoint":false,"force_prune_on_revert":true},"state":"Completed"},{"action":{"action_name":"create_directory","path":"/nix/var/nix/gcroots","user":null,"group":null,"mode":493,"is_mountpoint":false,"force_prune_on_revert":true},"state":"Completed"},{"action":{"action_name":"create_directory","path":"/nix/var/nix/gcroots/per-user","user":null,"group":null,"mode":493,"is_mountpoint":false,"force_prune_on_revert":true},"state":"Completed"},{"action":{"action_name":"create_directory","path":"/nix/var/nix/profiles","user":null,"group":null,"mode":493,"is_mountpoint":false,"force_prune_on_revert":true},"state":"Completed"},{"action":{"action_name":"create_directory","path":"/nix/var/nix/profiles/per-user","user":null,"group":null,"mode":493,"is_mountpoint":false,"force_prune_on_revert":true},"state":"Completed"},{"action":{"action_name":"create_directory","path":"/nix/var/nix/temproots","user":null,"group":null,"mode":493,"is_mountpoint":false,"force_prune_on_revert":true},"state":"Completed"},{"action":{"action_name":"create_directory","path":"/nix/var/nix/userpool","user":null,"group":null,"mode":493,"is_mountpoint":false,"force_prune_on_revert":true},"state":"Completed"},{"action":{"action_name":"create_directory","path":"/nix/var/nix/daemon-socket","user":null,"group":null,"mode":493,"is_mountpoint":false,"force_prune_on_revert":true},"state":"Completed"}]},"state":"Completed"},"move_unpacked_nix":{"action":{"action_name":"mount_unpacked_nix","unpacked_path":"/nix/temp-install-dir"},"state":"Completed"}},"state":"Completed"},{"action":{"action_name":"create_users_and_group","nix_build_group_name":"nixbld","nix_build_group_id":350,"nix_build_user_count":1,"nix_build_user_prefix":"_nixbld","nix_build_user_id_base":350,"create_group":{"action":{"action_name":"create_group","name":"nixbld","gid":350},"state":"Completed"},"create_users":[{"action":{"action_name":"create_user","name":"_nixbld1","uid":351,"groupname":"nixbld","gid":350,"comment":"Nix build user 1"},"state":"Completed"}],"add_users_to_groups":[{"action":{"action_name":"add_user_to_group","name":"_nixbld1","uid":351,"groupname":"nixbld","gid":350},"state":"Completed"}]},"state":"Completed"},{"action":{"action_name":"set_tmutil_exclusions","set_tmutil_exclusions":[{"action":{"action_name":"set_tmutil_exclusion","path":"/nix/store"},"state":"Completed"},{"action":{"action_name":"set_tmutil_exclusion","path":"/nix/var"},"state":"Completed"}]},"state":"Completed"},{"action":{"action_name":"configure_nix","setup_default_profile":{"action":{"action_name":"setup_default_profile","unpacked_path":"/nix/temp-install-dir"},"state":"Completed"},"configure_shell_profile":{"action":{"action_name":"configure_shell_profile","locations":{"fish":{"confd_suffix":"conf.d/nix.fish","confd_prefixes":["/etc/fish","/usr/local/etc/fish","/opt/homebrew/etc/fish","/opt/local/etc/fish"],"vendor_confd_suffix":"vendor_conf.d/nix.fish","vendor_confd_prefixes":["/usr/share/fish/","/usr/local/share/fish/"]},"bash":["/etc/bashrc","/etc/profile.d/nix.sh","/etc/bash.bashrc"],"zsh":["/etc/zshrc","/etc/zsh/zshrc"]},"create_directories":[{"action":{"action_name":"create_directory","path":"/etc/profile.d","user":null,"group":null,"mode":493,"is_mountpoint":false,"force_prune_on_revert":false},"state":"Completed"},{"action":{"action_name":"create_directory","path":"/etc/zsh","user":null,"group":null,"mode":493,"is_mountpoint":false,"force_prune_on_revert":false},"state":"Completed"}],"create_or_insert_into_files":[{"action":{"action_name":"create_or_insert_into_file","path":"/etc/bashrc","user":null,"group":null,"mode":420,"buf":"\n# Nix\nif [ -e '/nix/var/nix/profiles/default/etc/profile.d/nix-daemon.sh' ]; then\n    . '/nix/var/nix/profiles/default/etc/profile.d/nix-daemon.sh'\nfi\n# End Nix\n\n        \n","position":"Beginning"},"state":"Completed"},{"action":{"action_name":"create_or_insert_into_file","path":"/etc/profile.d/nix.sh","user":null,"group":null,"mode":420,"buf":"\n# Nix\nif [ -e '/nix/var/nix/profiles/default/etc/profile.d/nix-daemon.sh' ]; then\n    . '/nix/var/nix/profiles/default/etc/profile.d/nix-daemon.sh'\nfi\n# End Nix\n\n        \n","position":"Beginning"},"state":"Completed"},{"action":{"action_name":"create_or_insert_into_file","path":"/etc/bash.bashrc","user":null,"group":null,"mode":420,"buf":"\n# Nix\nif [ -e '/nix/var/nix/profiles/default/etc/profile.d/nix-daemon.sh' ]; then\n    . '/nix/var/nix/profiles/default/etc/profile.d/nix-daemon.sh'\nfi\n# End Nix\n\n        \n","position":"Beginning"},"state":"Completed"},{"action":{"action_name":"create_or_insert_into_file","path":"/etc/zshrc","user":null,"group":null,"mode":420,"buf":"\n# Nix\nif [ -e '/nix/var/nix/profiles/default/etc/profile.d/nix-daemon.sh' ]; then\n    . '/nix/var/nix/profiles/default/etc/profile.d/nix-daemon.sh'\nfi\n# End Nix\n\n        \n","position":"Beginning"},"state":"Completed"},{"action":{"action_name":"create_or_insert_into_file","path":"/etc/zsh/zshrc","user":null,"group":null,"mode":420,"buf":"\n# Nix\nif [ -e '/nix/var/nix/profiles/default/etc/profile.d/nix-daemon.sh' ]; then\n    . '/nix/var/nix/profiles/default/etc/profile.d/nix-daemon.sh'\nfi\n# End Nix\n\n        \n","position":"Beginning"},"state":"Completed"}]},"state":"Completed"},"place_nix_configuration":{"action":{"action_name":"place_nix_configuration","create_directory":{"action":{"action_name":"create_directory","path":"/etc/nix","user":null,"group":null,"mode":493,"is_mountpoint":false,"force_prune_on_revert":false},"state":"Completed"},"create_or_merge_standard_nix_config":{"action":{"action_name":"create_or_merge_nix_config","path":"/etc/nix/nix.conf","pending_nix_config":{"settings":{"extra-experimental-features":"nix-command flakes","always-allow-substitutes":"true","extra-trusted-substituters":"https://cache.flakehub.com","extra-trusted-public-keys":"cache.flakehub.com-3:hJuILl5sVK4iKm86JzgdXW12Y2Hwd5G07qKtHTOcDCM= cache.flakehub.com-4:Asi8qIv291s0aYLyH6IOnr5Kf6+OF14WVjkE6t3xMio= cache.flakehub.com-5:zB96CRlL7tiPtzA9/WKyPkp3A2vqxqgdgyTVNGShPDU= cache.flakehub.com-6:W4EGFwAGgBj3he7c5fNh9NkOXw0PUVaxygCVKeuvaqU= cache.flakehub.com-7:mvxJ2DZVHn/kRxlIaxYNMuDG1OvMckZu32um1TadOR8= cache.flakehub.com-8:moO+OVS0mnTjBTcOUh2kYLQEd59ExzyoW1QgQ8XAARQ= cache.flakehub.com-9:wChaSeTI6TeCuV/Sg2513ZIM9i0qJaYsF+lZCXg0J6o= cache.flakehub.com-10:2GqeNlIp6AKp4EF2MVbE1kBOp9iBSyo0UPR9KoR0o1Y=","bash-prompt-prefix":"(nix:$name)\\040","max-jobs":"auto","extra-nix-path":"nixpkgs=flake:nixpkgs","upgrade-nix-store-path-url":"https://install.determinate.systems/nix-upgrade/stable/universal"}},"header":"# Generated by https://github.com/DeterminateSystems/nix-installer.\n# See \u0060/nix/nix-installer --version\u0060 for the version details.\n","footer":"!include nix.custom.conf"},"state":"Completed"},"create_or_merge_custom_nix_config":{"action":{"action_name":"create_or_merge_nix_config","path":"/etc/nix/nix.custom.conf","pending_nix_config":{"settings":{}},"header":"# Written by https://github.com/DeterminateSystems/nix-installer.\n# The contents below are based on options specified at installation time.\n","footer":null},"state":"Completed"}},"state":"Completed"}},"state":"Completed"},{"action":{"action_name":"configure_remote_building","create_or_insert_into_file":{"action":{"action_name":"create_or_insert_into_file","path":"/etc/zshenv","user":null,"group":null,"mode":420,"buf":"\n# Set up Nix only on SSH connections\n# See: https://github.com/DeterminateSystems/nix-installer/pull/714\nif [ -e '/nix/var/nix/profiles/default/etc/profile.d/nix-daemon.sh' ] && [ -n \"${SSH_CONNECTION:-}\" ] && [ \"${SHLVL:-0}\" -eq 1 ]; then\n    . '/nix/var/nix/profiles/default/etc/profile.d/nix-daemon.sh'\nfi\n# End Nix\n","position":"Beginning"},"state":"Completed"}},"state":"Completed"},{"action":{"action_name":"create_nix_hook_service","path":"/Library/LaunchDaemons/systems.determinate.nix-installer.nix-hook.plist","service_label":"systems.determinate.nix-installer.nix-hook","needs_bootout":false},"state":"Completed"},{"action":{"action_name":"create_upstream_init_service","configure_init_service":{"action":{"action_name":"configure_init_service","init":"Launchd","start_daemon":true,"service_src":{"Path":"/nix/var/nix/profiles/default/Library/LaunchDaemons/org.nixos.nix-daemon.plist"},"service_name":"org.nixos.nix-daemon","service_dest":"/Library/LaunchDaemons/org.nixos.nix-daemon.plist","socket_files":[{"name":"nix-daemon.socket","src":{"Path":"/nix/var/nix/profiles/default/lib/systemd/system/nix-daemon.socket"},"dest":"/etc/systemd/system/nix-daemon.socket"}]},"state":"Completed"}},"state":"Completed"},{"action":{"action_name":"remove_directory","path":"/nix/temp-install-dir"},"state":"Completed"}],"planner":{"planner":"macos","settings":{"determinate_nix":true,"prefer_upstream":false,"modify_profile":true,"nix_build_group_name":"nixbld","nix_build_group_id":350,"nix_build_user_prefix":"_nixbld","nix_build_user_count":1,"nix_build_user_id_base":350,"nix_package_url":null,"proxy":null,"ssl_cert_file":null,"extra_conf":[],"force":false,"skip_nix_conf":false},"encrypt":null,"case_sensitive":false,"volume_label":"Nix Store","root_disk":"disk4","use_ec2_instance_store":false}}`

// Official nix-installer v3.0.0 tests/fixtures/linux/linux.json.
const legacyDeterminateLinuxUpstreamReceipt = `{"version":"3.0.0","actions":[{"action":{"action_name":"create_directory","path":"/nix","user":null,"group":null,"mode":493,"is_mountpoint":false,"force_prune_on_revert":true},"state":"Completed"},{"action":{"action_name":"provision_nix","nix_store_gid":30000,"fetch_nix":{"action":{"action_name":"fetch_and_unpack_nix","url_or_path":null,"dest":"/nix/temp-install-dir","proxy":null,"ssl_cert_file":null,"distribution":"Nix"},"state":"Completed"},"create_nix_tree":{"action":{"action_name":"create_nix_tree","create_directories":[{"action":{"action_name":"create_directory","path":"/nix/var","user":null,"group":null,"mode":493,"is_mountpoint":false,"force_prune_on_revert":true},"state":"Completed"},{"action":{"action_name":"create_directory","path":"/nix/var/log","user":null,"group":null,"mode":493,"is_mountpoint":false,"force_prune_on_revert":true},"state":"Completed"},{"action":{"action_name":"create_directory","path":"/nix/var/log/nix","user":null,"group":null,"mode":493,"is_mountpoint":false,"force_prune_on_revert":true},"state":"Completed"},{"action":{"action_name":"create_directory","path":"/nix/var/log/nix/drvs","user":null,"group":null,"mode":493,"is_mountpoint":false,"force_prune_on_revert":true},"state":"Completed"},{"action":{"action_name":"create_directory","path":"/nix/var/nix","user":null,"group":null,"mode":493,"is_mountpoint":false,"force_prune_on_revert":true},"state":"Completed"},{"action":{"action_name":"create_directory","path":"/nix/var/nix/db","user":null,"group":null,"mode":493,"is_mountpoint":false,"force_prune_on_revert":true},"state":"Completed"},{"action":{"action_name":"create_directory","path":"/nix/var/nix/gcroots","user":null,"group":null,"mode":493,"is_mountpoint":false,"force_prune_on_revert":true},"state":"Completed"},{"action":{"action_name":"create_directory","path":"/nix/var/nix/gcroots/per-user","user":null,"group":null,"mode":493,"is_mountpoint":false,"force_prune_on_revert":true},"state":"Completed"},{"action":{"action_name":"create_directory","path":"/nix/var/nix/profiles","user":null,"group":null,"mode":493,"is_mountpoint":false,"force_prune_on_revert":true},"state":"Completed"},{"action":{"action_name":"create_directory","path":"/nix/var/nix/profiles/per-user","user":null,"group":null,"mode":493,"is_mountpoint":false,"force_prune_on_revert":true},"state":"Completed"},{"action":{"action_name":"create_directory","path":"/nix/var/nix/temproots","user":null,"group":null,"mode":493,"is_mountpoint":false,"force_prune_on_revert":true},"state":"Completed"},{"action":{"action_name":"create_directory","path":"/nix/var/nix/userpool","user":null,"group":null,"mode":493,"is_mountpoint":false,"force_prune_on_revert":true},"state":"Completed"},{"action":{"action_name":"create_directory","path":"/nix/var/nix/daemon-socket","user":null,"group":null,"mode":493,"is_mountpoint":false,"force_prune_on_revert":true},"state":"Completed"}]},"state":"Completed"},"move_unpacked_nix":{"action":{"action_name":"mount_unpacked_nix","unpacked_path":"/nix/temp-install-dir"},"state":"Completed"}},"state":"Completed"},{"action":{"action_name":"create_users_and_group","nix_build_group_name":"nixbld","nix_build_group_id":30000,"nix_build_user_count":1,"nix_build_user_prefix":"nixbld","nix_build_user_id_base":30000,"create_group":{"action":{"action_name":"create_group","name":"nixbld","gid":30000},"state":"Completed"},"create_users":[{"action":{"action_name":"create_user","name":"nixbld1","uid":30001,"groupname":"nixbld","gid":30000,"comment":"Nix build user 1"},"state":"Completed"}],"add_users_to_groups":[{"action":{"action_name":"add_user_to_group","name":"nixbld1","uid":30001,"groupname":"nixbld","gid":30000},"state":"Completed"}]},"state":"Completed"},{"action":{"action_name":"configure_nix","setup_default_profile":{"action":{"action_name":"setup_default_profile","unpacked_path":"/nix/temp-install-dir"},"state":"Completed"},"configure_shell_profile":{"action":{"action_name":"configure_shell_profile","locations":{"fish":{"confd_suffix":"conf.d/nix.fish","confd_prefixes":["/etc/fish","/usr/local/etc/fish","/opt/homebrew/etc/fish","/opt/local/etc/fish"],"vendor_confd_suffix":"vendor_conf.d/nix.fish","vendor_confd_prefixes":["/usr/share/fish/","/usr/local/share/fish/"]},"bash":["/etc/bashrc","/etc/profile.d/nix.sh","/etc/bash.bashrc"],"zsh":["/etc/zshrc","/etc/zsh/zshrc"]},"create_directories":[{"action":{"action_name":"create_directory","path":"/etc/zsh","user":null,"group":null,"mode":493,"is_mountpoint":false,"force_prune_on_revert":false},"state":"Completed"},{"action":{"action_name":"create_directory","path":"/usr/share/fish/vendor_conf.d","user":null,"group":null,"mode":493,"is_mountpoint":false,"force_prune_on_revert":false},"state":"Completed"}],"create_or_insert_into_files":[{"action":{"action_name":"create_or_insert_into_file","path":"/etc/bashrc","user":null,"group":null,"mode":420,"buf":"\n# Nix\nif [ -e '/nix/var/nix/profiles/default/etc/profile.d/nix-daemon.sh' ]; then\n    . '/nix/var/nix/profiles/default/etc/profile.d/nix-daemon.sh'\nfi\n# End Nix\n\n        \n","position":"Beginning"},"state":"Completed"},{"action":{"action_name":"create_or_insert_into_file","path":"/etc/profile.d/nix.sh","user":null,"group":null,"mode":420,"buf":"\n# Nix\nif [ -e '/nix/var/nix/profiles/default/etc/profile.d/nix-daemon.sh' ]; then\n    . '/nix/var/nix/profiles/default/etc/profile.d/nix-daemon.sh'\nfi\n# End Nix\n\n        \n","position":"Beginning"},"state":"Completed"},{"action":{"action_name":"create_or_insert_into_file","path":"/etc/bash.bashrc","user":null,"group":null,"mode":420,"buf":"\n# Nix\nif [ -e '/nix/var/nix/profiles/default/etc/profile.d/nix-daemon.sh' ]; then\n    . '/nix/var/nix/profiles/default/etc/profile.d/nix-daemon.sh'\nfi\n# End Nix\n\n        \n","position":"Beginning"},"state":"Completed"},{"action":{"action_name":"create_or_insert_into_file","path":"/etc/zshrc","user":null,"group":null,"mode":420,"buf":"\n# Nix\nif [ -e '/nix/var/nix/profiles/default/etc/profile.d/nix-daemon.sh' ]; then\n    . '/nix/var/nix/profiles/default/etc/profile.d/nix-daemon.sh'\nfi\n# End Nix\n\n        \n","position":"Beginning"},"state":"Completed"},{"action":{"action_name":"create_or_insert_into_file","path":"/etc/zsh/zshrc","user":null,"group":null,"mode":420,"buf":"\n# Nix\nif [ -e '/nix/var/nix/profiles/default/etc/profile.d/nix-daemon.sh' ]; then\n    . '/nix/var/nix/profiles/default/etc/profile.d/nix-daemon.sh'\nfi\n# End Nix\n\n        \n","position":"Beginning"},"state":"Completed"},{"action":{"action_name":"create_or_insert_into_file","path":"/usr/share/fish/vendor_conf.d/nix.fish","user":null,"group":null,"mode":420,"buf":"\n# Nix\nif test -e '/nix/var/nix/profiles/default/etc/profile.d/nix-daemon.fish'\n    . '/nix/var/nix/profiles/default/etc/profile.d/nix-daemon.fish'\nend\n# End Nix\n\n","position":"Beginning"},"state":"Completed"}]},"state":"Completed"},"place_nix_configuration":{"action":{"action_name":"place_nix_configuration","create_directory":{"action":{"action_name":"create_directory","path":"/etc/nix","user":null,"group":null,"mode":493,"is_mountpoint":false,"force_prune_on_revert":false},"state":"Completed"},"create_or_merge_standard_nix_config":{"action":{"action_name":"create_or_merge_nix_config","path":"/etc/nix/nix.conf","pending_nix_config":{"settings":{"extra-experimental-features":"nix-command flakes","auto-optimise-store":"true","always-allow-substitutes":"true","extra-trusted-substituters":"https://cache.flakehub.com","extra-trusted-public-keys":"cache.flakehub.com-3:hJuILl5sVK4iKm86JzgdXW12Y2Hwd5G07qKtHTOcDCM= cache.flakehub.com-4:Asi8qIv291s0aYLyH6IOnr5Kf6+OF14WVjkE6t3xMio= cache.flakehub.com-5:zB96CRlL7tiPtzA9/WKyPkp3A2vqxqgdgyTVNGShPDU= cache.flakehub.com-6:W4EGFwAGgBj3he7c5fNh9NkOXw0PUVaxygCVKeuvaqU= cache.flakehub.com-7:mvxJ2DZVHn/kRxlIaxYNMuDG1OvMckZu32um1TadOR8= cache.flakehub.com-8:moO+OVS0mnTjBTcOUh2kYLQEd59ExzyoW1QgQ8XAARQ= cache.flakehub.com-9:wChaSeTI6TeCuV/Sg2513ZIM9i0qJaYsF+lZCXg0J6o= cache.flakehub.com-10:2GqeNlIp6AKp4EF2MVbE1kBOp9iBSyo0UPR9KoR0o1Y=","bash-prompt-prefix":"(nix:$name)\\040","max-jobs":"auto","extra-nix-path":"nixpkgs=flake:nixpkgs","upgrade-nix-store-path-url":"https://install.determinate.systems/nix-upgrade/stable/universal"}},"header":"# Generated by https://github.com/DeterminateSystems/nix-installer.\n# See \u0060/nix/nix-installer --version\u0060 for the version details.\n","footer":"!include nix.custom.conf"},"state":"Completed"},"create_or_merge_custom_nix_config":{"action":{"action_name":"create_or_merge_nix_config","path":"/etc/nix/nix.custom.conf","pending_nix_config":{"settings":{}},"header":"# Written by https://github.com/DeterminateSystems/nix-installer.\n# The contents below are based on options specified at installation time.\n","footer":null},"state":"Completed"}},"state":"Completed"}},"state":"Completed"},{"action":{"action_name":"create_directory","path":"/etc/tmpfiles.d","user":null,"group":null,"mode":493,"is_mountpoint":false,"force_prune_on_revert":false},"state":"Completed"},{"action":{"action_name":"create_upstream_init_service","configure_init_service":{"action":{"action_name":"configure_init_service","init":"Systemd","start_daemon":true,"service_src":"/nix/var/nix/profiles/default/lib/systemd/system/nix-daemon.service","service_name":null,"service_dest":"/etc/systemd/system/nix-daemon.service","socket_files":[{"name":"nix-daemon.socket","src":{"Path":"/nix/var/nix/profiles/default/lib/systemd/system/nix-daemon.socket"},"dest":"/etc/systemd/system/nix-daemon.socket"}]},"state":"Completed"}},"state":"Completed"},{"action":{"action_name":"remove_directory","path":"/nix/temp-install-dir"},"state":"Completed"}],"planner":{"planner":"linux","settings":{"determinate_nix":false,"modify_profile":true,"nix_build_group_name":"nixbld","nix_build_group_id":30000,"nix_build_user_prefix":"nixbld","nix_build_user_count":1,"nix_build_user_id_base":30000,"nix_package_url":null,"proxy":null,"ssl_cert_file":null,"extra_conf":[],"force":false,"skip_nix_conf":false},"init":{"init":"Systemd","start_daemon":true}}}`
