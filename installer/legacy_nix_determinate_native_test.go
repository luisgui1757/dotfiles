package installer

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

// Run after the upstream retirement fixture on the same disposable runner.
// The empty synthetic /nix mountpoint can remain on macOS until reboot.
func TestNativeLegacyDeterminateRetirement(t *testing.T) {
	nativeLegacyDeterminateRetirement(t, false)
}

func TestNativeLegacyDeterminateUpstreamRetirement(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("the Linux upstream-flavoured vendor unit layout")
	}
	nativeLegacyDeterminateRetirement(t, true)
}

func nativeLegacyDeterminateRetirement(t *testing.T, upstream bool) {
	if os.Getenv("DOTFILES_TEST_NATIVE_PACKAGES") != "1" || os.Getenv("DOTFILES_TEST_LEGACY_DETERMINATE") != "1" || runtime.GOOS == "windows" {
		t.Skip("set DOTFILES_TEST_NATIVE_PACKAGES=1 and DOTFILES_TEST_LEGACY_DETERMINATE=1 on a disposable POSIX runner")
	}
	if os.Geteuid() == 0 {
		t.Fatal("the fixture requires a non-root account with sudo")
	}
	if entries, err := os.ReadDir("/nix"); err == nil {
		if runtime.GOOS != "darwin" || len(entries) != 0 {
			t.Fatal("the fixture requires Nix absent", entries)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
	for _, path := range []string{"/etc/nix", legacyDeterminateNixdPath, "/run/current-system"} {
		if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
			t.Fatal("pre-existing Nix evidence prevents fixture installation", path, err)
		}
	}
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	home, err = resolveConfigPath(home)
	if err != nil {
		t.Fatal(err)
	}
	installed := newLegacyInstalled(t, home)
	folders, err := DiscoverConfigFolders()
	if err != nil {
		t.Fatal(err)
	}
	state, err := DiscoverStateDirectory(Context{OS: runtime.GOOS, Arch: runtime.GOARCH}, folders)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(filepath.Join(state, "state.json")); errors.Is(err, os.ErrNotExist) {
		installed.setup([]string{})
	} else if err != nil {
		t.Fatal(err)
	}
	if check := installed.run(Request{Schema: 1, Mode: "check"}, "setup", ""); check.Status != "ready" {
		t.Fatal(check)
	}

	pins := map[string]struct{ asset, sha string }{
		"linux/amd64": {"nix-installer-x86_64-linux", "d3c174b95b948ab1744a9a3507599d030bf31a39033305e8769b51c3134daeff"},
		"linux/arm64": {"nix-installer-aarch64-linux", "6b05c29b805e45e761ccf3fc8ca9ff7f03472f0ae2c98ce38ebb1bdb3b568694"},
	}
	mac := legacyDeterminateMacReleases["3.23.2"]
	pins["darwin/arm64"] = struct{ asset, sha string }{mac.Asset, mac.SHA256}
	pin, ok := pins[runtime.GOOS+"/"+runtime.GOARCH]
	if !ok {
		t.Fatal("unsupported fixture target")
	}
	data := legacyPinnedDownload(t, "https://github.com/DeterminateSystems/nix-installer/releases/download/v3.23.2/"+pin.asset, pin.sha)
	binary := filepath.Join(t.TempDir(), "nix-installer")
	if err := os.WriteFile(binary, data, 0700); err != nil {
		t.Fatal(err)
	}
	arguments := []string{"install", "--no-confirm"}
	rootHome := "/var/root"
	if runtime.GOOS == "linux" {
		rootHome = "/root"
		arguments = []string{"install", "linux", "--no-confirm"}
		if _, err := os.Stat("/run/systemd/system"); errors.Is(err, os.ErrNotExist) {
			arguments = append(arguments, "--init", "none")
		} else if err != nil {
			t.Fatal(err)
		}
		if upstream {
			arguments = append(arguments, "--prefer-upstream-nix")
		}
	}
	command := []string{"-n", "/usr/bin/env", "-i", "PATH=/usr/bin:/bin:/usr/sbin:/sbin", "HOME=" + rootHome,
		"DETSYS_IDS_TELEMETRY=disabled", "NIX_INSTALLER_DIAGNOSTIC_ENDPOINT=", binary}
	legacyRun(t, "", nil, "/usr/bin/sudo", append(command, arguments...)...)
	receiptData, _, err := readLegacyFile(legacyDeterminateReceiptPath, legacyDeterminateReceiptLimit)
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := parseLegacyDeterminateReceipt(receiptData)
	if err != nil {
		t.Fatal("the real installed receipt was refused", err)
	}
	if receipt.Version != "3.23.2" {
		t.Fatal(receipt.Version)
	}
	if receipt.Determinate == upstream {
		t.Fatal("the installed receipt does not match the requested distribution")
	}
	if legacyFileHash(t, legacyDeterminateInstallerPath) != pin.sha {
		t.Fatal("installed vendor binary changed")
	}
	if runtime.GOOS == "darwin" {
		uid, gid, mode, err := legacyFileOwner(legacyDeterminateInstallerPath)
		t.Logf("installed vendor binary: uid=%d gid=%d mode=%o", uid, gid, mode)
		if err != nil || int(uid) != os.Getuid() || mode != 0755 {
			t.Fatal("the ordinary Mac installation did not preserve its source owner", err)
		}
		if err := os.WriteFile(legacyDeterminateInstallerPath, append(append([]byte(nil), data...), '\n'), 0755); err != nil {
			t.Fatal(err)
		}
		preview := installed.run(Request{Schema: 1, Mode: "apply", Selected: []string{"retire.nix"}}, "migrate", "runtime")
		if op := operation(t, preview.Plan, "retire.nix"); op.Action != "pending" || !strings.Contains(op.Reason, "reviewed release digest") {
			t.Fatal("a modified user-owned vendor binary was accepted", op)
		}
		if err := os.WriteFile(legacyDeterminateInstallerPath, data, 0755); err != nil {
			t.Fatal(err)
		}
	}

	// A real outside profile must block the public preview before removal.
	other := "/nix/var/nix/profiles/per-user/dotfiles-fixture-other"
	if _, err := os.Lstat(other); !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
	legacyRun(t, "", nil, "/usr/bin/sudo", "-n", "/bin/mkdir", other)
	legacyRun(t, "", nil, "/usr/bin/sudo", "-n", "/bin/ln", "-s", "/nix/var/nix/profiles/default", filepath.Join(other, "profile"))
	preview := installed.run(Request{Schema: 1, Mode: "apply", Selected: []string{"retire.nix"}}, "migrate", "runtime")
	if op := operation(t, preview.Plan, "retire.nix"); op.Action != "pending" || !strings.Contains(op.Reason, "account dotfiles-fixture-other") {
		t.Fatal("outside profile did not prevent vendor retirement", preview)
	}
	legacyRun(t, "", nil, "/usr/bin/sudo", "-n", "/bin/rm", "--", filepath.Join(other, "profile"))
	legacyRun(t, "", nil, "/usr/bin/sudo", "-n", "/bin/rmdir", "--", other)
	if receipt.Init == "Systemd" && !upstream {
		// A partial vendor revert can remove a receipt-listed socket and
		// reload systemd before the next approved migration attempt. A crashed
		// service also remains failed after stop, rather than becoming inactive.
		unit := receipt.Units[0]
		if !strings.HasSuffix(unit, ".socket") {
			t.Fatal("the reviewed receipt's first unit is not a socket", unit)
		}
		legacyRun(t, "", nil, "/usr/bin/sudo", "-n", "/usr/bin/systemctl", "stop", unit)
		legacyRun(t, "", nil, "/usr/bin/sudo", "-n", "/bin/rm", "--", filepath.Join("/etc/systemd/system", unit))
		// Kill this fixture's entire service cgroup: KillMode=process would
		// otherwise orphan its real nix/crashpad children. The production
		// kernel scan correctly refuses deletion while those consumers live.
		legacyRun(t, "", nil, "/usr/bin/sudo", "-n", "/usr/bin/systemctl", "kill", "--signal=KILL", "--kill-whom=all", "nix-daemon.service")
		deadline := time.Now().Add(20 * time.Second)
		for {
			state := strings.TrimSpace(legacyRun(t, "", nil, "/usr/bin/systemctl", "show", "--property=ActiveState", "--value", "nix-daemon.service"))
			if state == "failed" {
				break
			}
			if time.Now().After(deadline) {
				t.Fatal("the killed daemon did not enter failed state", state)
			}
			time.Sleep(100 * time.Millisecond)
		}
		legacyRun(t, "", nil, "/usr/bin/sudo", "-n", "/bin/rm", "--", "/etc/systemd/system/nix-daemon.service")
		legacyRun(t, "", nil, "/usr/bin/sudo", "-n", "/usr/bin/systemctl", "daemon-reload")
		if state := legacyRun(t, "", nil, "/usr/bin/systemctl", "show", "--property=LoadState", "--value", unit); strings.TrimSpace(state) != "not-found" {
			t.Fatal("partial-revert fixture did not unload its removed socket", state)
		}
		if state := legacyRun(t, "", nil, "/usr/bin/systemctl", "show", "--property=ActiveState", "--value", "nix-daemon.service"); strings.TrimSpace(state) != "failed" {
			t.Fatal("partial-revert fixture did not retain the daemon's failed state", state)
		}
		for _, path := range legacyDeterminateUnitLinks(receipt) {
			if _, err := os.Readlink(path); err != nil {
				t.Fatal("the partial revert must retain the real activation links", path, err)
			}
		}
	}
	if receipt.Init == "Systemd" && upstream {
		path := "/etc/systemd/system/multi-user.target.wants/nix-daemon.service"
		target, err := os.Readlink(path)
		resolved, resolveErr := filepath.EvalSymlinks("/etc/systemd/system/nix-daemon.service")
		if err != nil || resolveErr != nil || target != resolved || !strings.HasPrefix(target, "/nix/store/") {
			t.Fatal("the vendor did not produce the reviewed store-resolved service link", target, resolved, err, resolveErr)
		}
	}

	legacyRun(t, "", nil, "/usr/bin/sudo", "-n", "/bin/sh", "-c", `printf '%s\n' "$1" >> /etc/nix/nix.custom.conf`, "sh", "# dotfiles-determinate-personal-witness "+t.Name())
	personal := legacyFileHash(t, "/etc/nix/nix.custom.conf")
	// A process outside the daemon's service still uses the store through its
	// working directory. Stopping the receipt-owned service must not grant
	// permission to remove this process's data or run the vendor uninstaller.
	consumer := exec.Command("/bin/sleep", "300")
	consumer.Dir = "/nix/store"
	if err := consumer.Start(); err != nil {
		t.Fatal(err)
	}
	stopConsumer := func() {
		if consumer.ProcessState != nil {
			return
		}
		if err := consumer.Process.Kill(); err != nil {
			t.Error(err)
		}
		var exit *exec.ExitError
		if err := consumer.Wait(); err != nil && !errors.As(err, &exit) {
			t.Error(err)
		}
	}
	t.Cleanup(stopConsumer)
	receiptHash := legacyFileHash(t, legacyDeterminateReceiptPath)
	request := Request{Schema: 1, Mode: "apply", Selected: []string{"retire.nix"}}
	preview = installed.run(request, "migrate", "runtime")
	if preview.Status != "preview" {
		t.Fatal(preview)
	}
	request.ExpectedPlan = preview.Plan.ID
	blocked, _, diagnostic := installed.command(request, "migrate", "runtime")
	if err := blocked.Run(); err == nil || !strings.Contains(diagnostic.data.String(), "nix-consumers") || !strings.Contains(diagnostic.data.String(), "process "+strconv.Itoa(consumer.Process.Pid)+" uses ") {
		t.Fatalf("outside process did not block the vendor uninstall: %v%s", err, nativeDiagnostic(diagnostic.data.Bytes()))
	}
	if receipt.Init == "Systemd" {
		for _, unit := range receipt.Units {
			if state := strings.TrimSpace(legacyRun(t, "", nil, "/usr/bin/systemctl", "show", "--property=ActiveState", "--value", unit)); state != "inactive" && state != "failed" {
				t.Fatal("the receipt-owned service remains active", unit, state)
			}
		}
	}
	if legacyFileHash(t, legacyDeterminateReceiptPath) != receiptHash || legacyFileHash(t, legacyDeterminateInstallerPath) != pin.sha {
		t.Fatal("the vendor ran despite an active outside consumer")
	}
	stopConsumer()
	result := installed.finishRuntime()
	if result.Status != "ready" {
		t.Fatal("vendor retirement did not finish", result)
	}
	for _, entry := range []string{"migrate", "setup"} {
		phase := ""
		if entry == "migrate" {
			phase = "runtime"
		}
		if check := installed.run(Request{Schema: 1, Mode: "check"}, entry, phase); check.Status != "ready" {
			t.Fatal(check)
		}
	}
	requireLegacyAbsent(t, legacyDeterminateInstallerPath, legacyDeterminateReceiptPath, legacyDeterminateNixdPath, "/nix/store")
	for _, path := range legacyDeterminateUnitLinks(receipt) {
		requireLegacyAbsent(t, path)
	}
	d := legacyNativeDriver(t)
	for _, account := range receipt.Users {
		if record, err := d.accountRecord(context.Background(), account.Name); err != nil || record != "absent" {
			t.Fatal("vendor account remains", record, err)
		}
	}
	if record, err := d.groupRecord(context.Background(), receipt.Group); err != nil || record != "absent" {
		t.Fatal("vendor group remains", record, err)
	}
	backups, err := filepath.Glob("/etc/.dotfiles-retired-*-nix/nix.custom.conf")
	if err != nil {
		t.Fatal(err)
	}
	kept := false
	for _, backup := range backups {
		kept = kept || legacyFileHash(t, backup) == personal
	}
	if !kept {
		t.Fatal("personal system Nix configuration was not preserved")
	}
	if runtime.GOOS == "linux" {
		legacyAssertLinuxLogin(t)
	}
	// Completed vendor evidence must never authorize a later personal file at
	// one of its old service paths. Exercise the public preview after removal.
	path := "/etc/systemd/system/determinate-nixd.socket"
	if upstream {
		path = "/etc/systemd/system/nix-daemon.service"
	}
	if runtime.GOOS == "darwin" {
		path = "/Library/LaunchDaemons/systems.determinate.nix-daemon.plist"
	}
	requireLegacyAbsent(t, path)
	legacyRun(t, "", nil, "/usr/bin/sudo", "-n", "/bin/sh", "-c", `printf '%s\n' '# personal service after Nix removal' > "$1"`, "sh", path)
	serviceHash := legacyFileHash(t, path)
	request = Request{Schema: 1, Mode: "apply", Selected: []string{"retire.nix"}}
	preview = installed.run(request, "migrate", "runtime")
	// An upstream no-init receipt never owned this service, so the generic
	// integration guard refuses it instead of the saved vendor-file guard. Both
	// must name the protected path and preserve it through an attempted apply.
	if op := operation(t, preview.Plan, "retire.nix"); preview.Status != "preview" || op.Action != "pending" || !strings.Contains(op.Reason, path) || !strings.Contains(op.Reason, "preserve") {
		t.Fatal("a new personal service was not explicitly protected", preview)
	}
	if legacyFileHash(t, path) != serviceHash {
		t.Fatal("preview changed the new personal service")
	}
	request.ExpectedPlan = preview.Plan.ID
	if result := installed.run(request, "migrate", "runtime"); result.Status != "needs-action" || !strings.Contains(result.Message, path) {
		t.Fatal("application did not refuse the new personal service", result)
	}
	if legacyFileHash(t, path) != serviceHash {
		t.Fatal("application changed the new personal service")
	}
	legacyRun(t, "", nil, "/usr/bin/sudo", "-n", "/bin/rm", "--", path)
	// The refused apply leaves a recoverable transaction. Explicitly resume
	// it after removing this fixture's witness before the next distribution.
	if result := installed.approve(Request{Schema: 1, Mode: "apply", Retry: true}, "migrate", "runtime"); result.Status != "ready" {
		t.Fatal("personal-service refusal did not recover after fixture cleanup", result)
	}
	// The vendor can recreate configuration (for example sentry-endpoint)
	// after migration backs it up. The fixture began with /etc/nix absent;
	// preserve this run's remainder separately so the next run starts clean.
	// Production retention and the preceding preservation assertions stand.
	if info, err := os.Lstat("/etc/nix"); err == nil {
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			t.Fatal("the fixture's remaining Nix configuration is not a plain directory")
		}
		kept := strings.TrimSpace(legacyRun(t, "", nil, "/usr/bin/sudo", "-n", "/usr/bin/mktemp", "-d", "/etc/.dotfiles-fixture-nix.XXXXXXXX"))
		if filepath.Dir(kept) != "/etc" || !strings.HasPrefix(filepath.Base(kept), ".dotfiles-fixture-nix.") {
			t.Fatal("unexpected fixture preservation directory", kept)
		}
		legacyRun(t, "", nil, "/usr/bin/sudo", "-n", "/bin/mv", "/etc/nix", filepath.Join(kept, "nix"))
		t.Log("preserved fixture-generated configuration at", kept)
	} else if !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
}
