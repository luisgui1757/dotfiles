package installer

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"
)

// This is a real provider/worker lifecycle, not a simulator. The temporary
// repository contains only uniquely named data packages and is removed during
// cleanup. Neither maintainer scripts nor unrelated package upgrades are needed.
func TestNativeLinuxAPTDriverLifecycle(t *testing.T) {
	if runtime.GOOS != "linux" || os.Getenv("DOTFILES_TEST_NATIVE_PACKAGES") != "1" || os.Getenv("GITHUB_ACTIONS") != "true" {
		t.Skip("requires an explicitly opted-in disposable GitHub Linux runner")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	defer cancel()
	location, err := DiscoverLinuxAPT(ctx)
	if err != nil {
		t.Fatal(err)
	}
	run := func(privileged bool, program string, args ...string) ([]byte, error) {
		if privileged && !location.root {
			args = append([]string{"-n", "LC_ALL=C", "DEBIAN_FRONTEND=noninteractive", program}, args...)
			program = location.Sudo
		}
		command := exec.CommandContext(ctx, program, args...)
		command.Env = append(os.Environ(), "LC_ALL=C", "DEBIAN_FRONTEND=noninteractive")
		return command.CombinedOutput()
	}
	require := func(privileged bool, program string, args ...string) []byte {
		t.Helper()
		output, err := run(privileged, program, args...)
		if err != nil {
			t.Fatalf("%s %v: %v\n%s", program, args, err, output)
		}
		return output
	}
	baseline := require(false, location.DpkgQuery, "-W", "-f=${Package}:${Architecture}\t${Version}\t${db:Status-Abbrev}\n")
	root, err := os.MkdirTemp("/var/tmp", "dotfiles-apt-driver-")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(root, 0755); err != nil {
		t.Fatal(err)
	}
	prefix := "dotfiles-driver-" + strings.ToLower(rand.Text()[:10])
	names := []string{prefix + "-first", prefix + "-second", prefix + "-outside", prefix + "-shared", prefix + "-unrelated"}
	conffile := filepath.Join(root, "retained.conf")
	source := filepath.Join("/etc/apt/sources.list.d", prefix+".list")
	t.Cleanup(func() {
		cleanup, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()
		commands := [][]string{append([]string{location.Dpkg, "--remove"}, names...), {"/usr/bin/rm", "-f", "--", source}, {"/usr/bin/rm", "-rf", "--", root}}
		entries, err := os.ReadDir("/var/lib/apt/lists")
		if err != nil {
			t.Error(err)
		} else {
			for _, entry := range entries {
				if strings.Contains(entry.Name(), filepath.Base(root)) && !entry.IsDir() {
					commands = append(commands, []string{"/usr/bin/rm", "-f", "--", filepath.Join("/var/lib/apt/lists", entry.Name())})
				}
			}
		}
		for _, command := range commands {
			program, args := command[0], command[1:]
			if !location.root {
				args = append([]string{"-n", program}, args...)
				program = location.Sudo
			}
			if output, err := exec.CommandContext(cleanup, program, args...).CombinedOutput(); err != nil {
				t.Errorf("APT fixture cleanup: %v\n%s", err, output)
			}
		}
	})
	repository := filepath.Join(root, "repository")
	if err := os.Mkdir(repository, 0755); err != nil {
		t.Fatal(err)
	}
	writeRepository := func(version string) {
		t.Helper()
		var index bytes.Buffer
		for _, name := range names {
			packageVersion := "1.0"
			if name == names[0] {
				packageVersion = version
			}
			control := fmt.Sprintf("Package: %s\nVersion: %s\nArchitecture: all\nMaintainer: Dotfiles fixture <test@example.invalid>\nDescription: Disposable native provider data\n", name, packageVersion)
			if slices.Contains(names[:3], name) {
				control += "Depends: " + names[3] + "\n"
			}
			build := filepath.Join(root, "build", name)
			for _, path := range []string{filepath.Join(build, "DEBIAN"), filepath.Join(build, "usr/share", name)} {
				if err := os.MkdirAll(path, 0755); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.WriteFile(filepath.Join(build, "DEBIAN/control"), []byte(control), 0644); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(build, "usr/share", name, "fixture"), []byte(packageVersion), 0644); err != nil {
				t.Fatal(err)
			}
			if name == names[0] {
				payloadConfig := filepath.Join(build, strings.TrimPrefix(conffile, "/"))
				if err := os.MkdirAll(filepath.Dir(payloadConfig), 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(payloadConfig, []byte("fixture default\n"), 0644); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(build, "DEBIAN/conffiles"), []byte(conffile+"\n"), 0644); err != nil {
					t.Fatal(err)
				}
			}
			archive := filepath.Join(repository, name+"-"+packageVersion+".deb")
			require(false, "dpkg-deb", "--root-owner-group", "--build", build, archive)
			data, err := os.ReadFile(archive)
			if err != nil {
				t.Fatal(err)
			}
			fmt.Fprintf(&index, "%sFilename: ./%s\nSize: %d\nSHA256: %x\n\n", control, filepath.Base(archive), len(data), sha256.Sum256(data))
		}
		if err := os.WriteFile(filepath.Join(repository, "Packages"), index.Bytes(), 0644); err != nil {
			t.Fatal(err)
		}
	}
	writeRepository("1.0")
	localSource := filepath.Join(root, "source.list")
	if err := os.WriteFile(localSource, []byte("deb [trusted=yes] file:"+repository+" ./\n"), 0644); err != nil {
		t.Fatal(err)
	}
	require(true, "/usr/bin/install", "-m", "644", localSource, source)
	require(true, location.APTGet, "-o", "APT::Update::Error-Mode=any", "update")
	require(true, location.APTGet, "--yes", "--no-remove", "--no-install-recommends", "install", names[4])
	require(true, location.APTMark, "auto", names[4])
	// Remove only this fixture's cached repository index. The provider must
	// refresh it itself before any absent fixture root becomes discoverable.
	listEntries, err := os.ReadDir("/var/lib/apt/lists")
	if err != nil {
		t.Fatal(err)
	}
	fixtureLists := []string{}
	for _, entry := range listEntries {
		if strings.Contains(entry.Name(), filepath.Base(root)) && !entry.IsDir() {
			fixtureLists = append(fixtureLists, filepath.Join("/var/lib/apt/lists", entry.Name()))
		}
	}
	if len(fixtureLists) == 0 {
		t.Fatal("fixture APT cache was not found for the fresh-index check")
	}
	require(true, "/usr/bin/rm", append([]string{"--"}, fixtureLists...)...)
	if output, err := run(false, "apt-cache", "show", names[0]); err == nil {
		t.Fatalf("fresh root remained discoverable after its scoped index removal:\n%s", output)
	}
	workerDirectory := filepath.Join(root, "state", "worker")
	session := &nativeSession{Directory: workerDirectory, Start: func(context.Context) (*nativeWorkerClient, error) { return workerFixture(workerDirectory) }}
	guard, err := session.acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := guard(); err != nil {
			t.Error(err)
		}
	})
	d := &LinuxAPTDriver{Directory: filepath.Join(root, "state", "apt"), WorkerDirectory: workerDirectory, Location: *location,
		Packages: map[string]string{"tool.first": names[0], "tool.second": names[1]}, Query: location.query, Run: session.run, approval: &linuxAPTApproval{}}
	for _, name := range []string{"first", "second"} {
		if got, err := d.Observe(ctx, aptFixtureResource(name), Receipt{}); err != nil || got.Present {
			t.Fatal("native fixture did not establish fresh package absence", got, err)
		}
	}
	approve := func() {
		t.Helper()
		state, err := d.ledger()
		if err != nil {
			t.Fatal(err)
		}
		installed, err := linuxAPTInventory(ctx, d.Query)
		if err != nil {
			t.Fatal(err)
		}
		hash, err := d.inventoryDigest(state, installed)
		if err != nil {
			t.Fatal(err)
		}
		d.approval.expected = ""
		if err := d.approveInventory(ctx, []string{hash}); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"first", "second"} {
		approve()
		r, receipt := aptFixtureResource(name), aptFixtureReceipt("native-install-"+name)
		if name == "first" {
			d.Run = func(ctx context.Context, command nativeCommand) ([]byte, error) {
				output, err := session.run(ctx, command)
				if err == nil && slices.Contains(command.Arguments, "install") {
					return output, errors.New("fixture lost completed APT reply")
				}
				return output, err
			}
			if _, err := d.Apply(ctx, r, Operation{Action: "install"}, receipt); err == nil || !strings.Contains(err.Error(), "lost completed") {
				t.Fatal("real native reply loss was not exercised", err)
			}
			d.Run = session.run
			if err := guard(); err != nil {
				t.Fatal(err)
			}
			guard, err = session.acquire(ctx)
			if err != nil {
				t.Fatal(err)
			}
			approve()
			if got, err := d.ResumeResource(ctx, r, Operation{Action: "install"}, receipt); err != nil || got.CompletedOperation != receipt.OperationID || !got.Healthy {
				t.Fatal("real APT restart recovery failed", got, err)
			}
		} else if got, err := d.Apply(ctx, r, Operation{Action: "install"}, receipt); err != nil || !got.Healthy {
			t.Fatal("real APT installation failed", got, err)
		}
	}
	writeRepository("2.0")
	approve()
	if got, err := d.Apply(ctx, aptFixtureResource("first"), Operation{Action: "update"}, aptFixtureReceipt("native-update")); err != nil || !got.Healthy {
		t.Fatal("real APT update failed", got, err)
	}
	if got := string(require(false, location.DpkgQuery, "-W", "-f=${Version}", names[0])); got != "2.0" {
		t.Fatal("APT did not change the actual installed version", got)
	}
	require(true, "/usr/bin/rm", "--", filepath.Join("/usr/share", names[0], "fixture"))
	approve()
	if _, err := d.Apply(ctx, aptFixtureResource("first"), Operation{Action: "repair"}, aptFixtureReceipt("native-repair")); err != nil {
		t.Fatal("real APT reinstall failed", err)
	}
	if data, err := os.ReadFile(filepath.Join("/usr/share", names[0], "fixture")); err != nil || string(data) != "2.0" {
		t.Fatal("APT reinstall did not restore missing data", string(data), err)
	}
	require(true, location.APTGet, "--yes", "--no-remove", "--no-install-recommends", "install", names[2])
	personalConfig := []byte("fixture personal configuration\n")
	personalSource := filepath.Join(root, "personal.conf")
	if err := os.WriteFile(personalSource, personalConfig, 0644); err != nil {
		t.Fatal(err)
	}
	require(true, "/usr/bin/install", "-m", "644", personalSource, conffile)
	firstRemoval := aptFixtureReceipt("native-remove-first")
	for _, name := range []string{"first", "second"} {
		approve()
		receipt := aptFixtureReceipt("native-remove-" + name)
		got, err := d.Remove(ctx, aptFixtureResource(name), receipt)
		if err != nil || got.Present || !slices.Contains(got.Preserved, "apt:"+names[3]+":all") {
			t.Fatal("real APT removal lost outside-consumer protection", got, err)
		}
	}
	for _, name := range names[2:] {
		if got := string(require(false, location.DpkgQuery, "-W", "-f=${db:Status-Abbrev}", name)); got != "ii " {
			t.Fatal("APT provider changed protected package", name, got)
		}
	}
	assertResidual := func() {
		t.Helper()
		if status := string(require(false, location.DpkgQuery, "-W", "-f=${db:Status-Status}", names[0])); status != "config-files" {
			t.Fatal("normal removal did not retain the dpkg conffile state", status)
		}
		if data, err := os.ReadFile(conffile); err != nil || !bytes.Equal(data, personalConfig) {
			t.Fatal("normal removal changed personal conffile bytes", string(data), err)
		}
		t.Log("native removal retained config-files state and personal conffile bytes")
	}
	assertResidual()
	approve()
	reinstall := aptFixtureReceipt("native-install-after-remove")
	if got, err := d.Apply(ctx, aptFixtureResource("first"), Operation{Action: "install"}, reinstall); err != nil || !got.Healthy || got.CompletedOperation != reinstall.OperationID {
		t.Fatal("real APT installation from residual configuration failed", got, err)
	}
	intent, err := d.intent(reinstall.OperationID)
	if err != nil {
		t.Fatal(err)
	}
	log, err := os.ReadFile(filepath.Join(d.Directory, "logs", intent.Commands[len(intent.Commands)-1].Command.Operation+".dpkg"))
	if err != nil || !strings.Contains(string(log), " install "+names[0]+":all 2.0 2.0\n") {
		t.Fatal("native reinstall did not exercise remembered old-version evidence", string(log), err)
	}
	t.Logf("native reinstall operation dpkg evidence:\n%s", log)
	state, err := d.ledger()
	if err != nil || state.Roots["tool.first"].CreatedBy != reinstall.OperationID {
		t.Fatal("native reinstall lost operation-owned root", state, err)
	}
	approve()
	if got, err := d.Remove(ctx, aptFixtureResource("first"), aptFixtureReceipt("native-remove-reinstalled")); err != nil || got.Present || !slices.Contains(got.Preserved, "apt:"+names[3]+":all") {
		t.Fatal("reinstalled root could not be removed safely", got, err)
	}
	assertResidual()
	require(true, location.Dpkg, "--remove", names[2], names[3], names[4])
	if got, err := d.Observe(ctx, aptFixtureResource("first"), firstRemoval); err != nil || len(got.Preserved) != 0 {
		t.Fatal("real APT old disclosure reported absent package", got, err)
	}
	after := require(false, location.DpkgQuery, "-W", "-f=${Package}:${Architecture}\t${Version}\t${db:Status-Abbrev}\n")
	withoutFixtures := func(data []byte) string {
		result := []string{}
		for _, line := range strings.Split(string(data), "\n") {
			if !strings.HasPrefix(line, prefix) {
				result = append(result, line)
			}
		}
		return strings.Join(result, "\n")
	}
	if withoutFixtures(baseline) != withoutFixtures(after) {
		t.Fatal("APT provider lifecycle changed an unrelated native package")
	}
}
