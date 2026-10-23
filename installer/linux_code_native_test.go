package installer

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"
)

// This explicitly opted-in fixture uses the actual native worker and official
// DEB. A disposable container proves package semantics, not the hosted GUI or
// AppArmor boundary; the public desktop fixture proves that separately.
func TestNativeLinuxCodeDEBLifecycle(t *testing.T) {
	hosted := os.Getenv("GITHUB_ACTIONS") == "true" && os.Getenv("RUNNER_ENVIRONMENT") == "github-hosted"
	_, containerMarker := os.Stat("/.dockerenv")
	container := os.Getenv("DOTFILES_TEST_DISPOSABLE_CONTAINER") == "1" && containerMarker == nil
	if runtime.GOOS != "linux" || os.Getenv("DOTFILES_TEST_NATIVE_CODE") != "1" || os.Getenv("DOTFILES_TEST_NATIVE_PACKAGES") != "1" || !hosted && !container {
		t.Skip("requires explicit disposable Linux Code package acceptance")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Minute)
	t.Cleanup(cancel)
	location, err := DiscoverLinuxAPT(ctx)
	if err != nil {
		t.Fatal(err)
	}
	run := func(input, program string, args ...string) []byte {
		t.Helper()
		if !location.root {
			args = append([]string{"-n", "DEBIAN_FRONTEND=noninteractive", program}, args...)
			program = location.Sudo
		}
		cmd := exec.CommandContext(ctx, program, args...)
		cmd.Env = append(os.Environ(), "DEBIAN_FRONTEND=noninteractive", "LC_ALL=C")
		cmd.Stdin = strings.NewReader(input)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("native Code fixture prerequisite: %s %v: %v\n%s", program, args, err, out)
		}
		return out
	}
	installed, err := linuxAPTInventory(ctx, location.query)
	if err != nil {
		t.Fatal(err)
	}
	if identity, err := linuxAPTIdentity("code", installed); err != nil || identity != "" || !installed["apt-utils:"+runtime.GOARCH].Healthy {
		t.Fatal("Code fixture requires absent Code and installed apt-utils", identity, err)
	}
	paths := []string{"/etc/apt/sources.list.d/vscode.sources", "/usr/share/keyrings/microsoft.gpg"}
	for _, path := range paths {
		if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
			t.Fatal("Code fixture will not replace existing native repository/key", path, err)
		}
	}
	const repo = "# personal repository fixture\nTypes: deb\nURIs: https://example.invalid/personal-code\nSuites: stable\nComponents: main\nEnabled: no\n"
	const key = "personal Microsoft key fixture\n"
	run(repo, "/usr/bin/tee", paths[0])
	run(key, "/usr/bin/tee", paths[1])
	run("code code/add-microsoft-repo boolean true\n", "/usr/bin/debconf-set-selections")
	t.Cleanup(func() {
		// Only fixture-created paths are removed, after checking their bytes.
		for i, want := range []string{repo, key} {
			data, err := os.ReadFile(paths[i])
			if err != nil || string(data) != want {
				t.Error("personal native sentinel changed; preserve it for inspection", paths[i], err)
				continue
			}
			run("", "/usr/bin/rm", "--", paths[i])
		}
	})
	assertPersonal := func() {
		t.Helper()
		for i, want := range []string{repo, key} {
			data, err := os.ReadFile(paths[i])
			if err != nil || string(data) != want {
				t.Fatal("Code changed a personal repository/key", paths[i], err)
			}
		}
		if out := run("GET code/add-microsoft-repo\n", "/usr/bin/debconf-communicate", "code"); strings.TrimSpace(string(out)) != "0 true" {
			t.Fatal("Code replaced the original personal debconf answer", string(out))
		}
	}
	root := filepath.Join(t.TempDir(), "private Code state ü with spaces")
	workerDirectory := filepath.Join(root, "worker")
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
	pin, err := defaultLinuxCodePin(runtime.GOARCH)
	if err != nil {
		t.Fatal(err)
	}
	d := &LinuxAPTDriver{Directory: filepath.Join(root, "apt"), WorkerDirectory: workerDirectory, Location: *location, Architecture: runtime.GOARCH,
		Code: &pin, Packages: map[string]string{"tool.vscode": "code"}, Checks: map[string][]string{"tool.vscode": {location.Dpkg, "--verify", "code"}}, Query: location.query, Run: session.run, approval: &linuxAPTApproval{}}
	if file := os.Getenv("DOTFILES_TEST_CODE_DEB"); file != "" {
		d.Client = &http.Client{Transport: windowsVendorTransport(func(request *http.Request) (*http.Response, error) {
			input, err := os.Open(file)
			if err != nil {
				return nil, err
			}
			return &http.Response{StatusCode: 200, Body: input, Header: http.Header{}, Request: request}, nil
		})}
	}
	pristine := filepath.Join(root, "reviewed-code.deb")
	r := Resource{ID: "tool.vscode", Name: "Code", Action: "vscode"}
	for _, action := range []string{"install", "repair", "update", "remove"} {
		if action == "update" {
			// Install the exact payload with deliberately newer package metadata,
			// through dpkg itself. This is a synthetic external-version fixture,
			// not a claim that an unpublished upstream release was downloaded.
			stage := filepath.Join(root, "external-version")
			privateRun := func(program string, args ...string) {
				t.Helper()
				if output, err := exec.CommandContext(ctx, program, args...).CombinedOutput(); err != nil {
					t.Fatalf("private package fixture: %v%s", err, nativeDiagnostic(output))
				}
			}
			privateRun("/usr/bin/dpkg-deb", "--raw-extract", pristine, stage)
			control := filepath.Join(stage, "DEBIAN", "control")
			data, err := os.ReadFile(control)
			if err != nil || strings.Count(string(data), "Version: "+pin.Version+"\n") != 1 {
				t.Fatal("unexpected pinned control", err)
			}
			const externalVersion = "1:1.141.0-1"
			data = []byte(strings.Replace(string(data), "Version: "+pin.Version+"\n", "Version: "+externalVersion+"\n", 1))
			if err := os.WriteFile(control, data, 0644); err != nil {
				t.Fatal(err)
			}
			external := filepath.Join(root, "external-code.deb")
			privateRun("/usr/bin/dpkg-deb", "--root-owner-group", "-Zgzip", "-z1", "--build", stage, external)
			op, err := digest("native external Code fixture")
			if err != nil {
				t.Fatal(err)
			}
			input, err := newLinuxCodeInput(pin, op)
			if err != nil {
				t.Fatal(err)
			}
			run("", "/usr/bin/env", "DEBCONF_DB_OVERRIDE=File{filename:"+filepath.Join(input.Directory, "answer.dat")+" accept_name:^code/add-microsoft-repo$ mode:0644 backup:no}", location.Dpkg, "--install", external)
			if err := input.cleanup(); err != nil {
				t.Fatal(err)
			}
			inventory, err := linuxAPTInventory(ctx, d.Query)
			if err != nil || inventory["code:"+runtime.GOARCH].Version != externalVersion {
				t.Fatal("external native upgrade missing", err)
			}
			detail, err := d.codeApplyDetail(ctx, externalVersion)
			if err != nil || !strings.Contains(detail, "downgrade") {
				t.Fatal("epoch-aware downgrade disclosure missing", detail, err)
			}
			t.Log("native external upgrade observed:", detail)
			assertPersonal()
			if err := os.RemoveAll(stage); err != nil {
				t.Fatal(err)
			}
			if err := os.Remove(external); err != nil {
				t.Fatal(err)
			}
		}

		state, err := d.ledger()
		if err != nil {
			t.Fatal(err)
		}
		inventory, err := linuxAPTInventory(ctx, d.Query)
		if err != nil {
			t.Fatal(err)
		}
		hash, err := d.inventoryDigest(state, inventory)
		if err != nil {
			t.Fatal(err)
		}
		d.approval.expected = ""
		if err := d.approveInventory(ctx, []string{hash}); err != nil {
			t.Fatal(err)
		}
		receipt := aptFixtureReceipt("native-code-" + action)
		// A controller restart must recover the successfully completed install,
		// using its original command/input, without rerunning maintainer scripts.
		if action == "install" {
			d.Run = func(ctx context.Context, command nativeCommand) ([]byte, error) {
				out, err := session.run(ctx, command)
				if err == nil && slices.Contains(command.Arguments, "install") {
					return out, errors.New("fixture lost completed Code install reply")
				}
				return out, err
			}
			if _, err := d.Apply(ctx, r, Operation{Action: action}, receipt); err == nil || !strings.Contains(err.Error(), "fixture lost completed") {
				t.Fatal("native install did not reach its deliberate lost reply", err)
			}
			if err := d.verifyCodePayload(pin); err != nil {
				t.Fatal("unknown outcome lost verified package", err)
			}
			if output, err := exec.CommandContext(ctx, "/bin/cp", d.codePayload(pin), pristine).CombinedOutput(); err != nil {
				t.Fatalf("save exact fixture input: %v%s", err, nativeDiagnostic(output))
			}
			d.Run = session.run
			current, err := linuxAPTInventory(ctx, d.Query)
			if err != nil {
				t.Fatal(err)
			}
			hash, err := d.inventoryDigest(state, current)
			if err != nil {
				t.Fatal(err)
			}
			d.approval.expected = ""
			if err := d.approveInventory(ctx, []string{hash}); err != nil {
				t.Fatal(err)
			}
		}
		got, err := d.Apply(ctx, r, Operation{Action: action}, receipt)
		if err != nil || got.Present != (action != "remove") || got.CompletedOperation != receipt.OperationID || action != "remove" && !got.Healthy {
			t.Fatal(action, got, err)
		}
		if _, err := os.Lstat(d.codePayload(pin)); !errors.Is(err, os.ErrNotExist) {
			t.Fatal("completed operation retained DEB cache", action, err)
		}
		if _, err := os.Stat("/usr/share/applications/com.microsoft.VSCode.desktop"); (action == "remove" && !errors.Is(err, os.ErrNotExist)) || (action != "remove" && err != nil) {
			t.Fatal("native package launcher lifecycle", action, err)
		}
		if action == "update" {
			inventory, err := linuxAPTInventory(ctx, d.Query)
			if err != nil || inventory["code:"+runtime.GOARCH].Version != pin.Version {
				t.Fatal("approved update did not reconcile reviewed pin", err)
			}
		}
		assertPersonal()
		t.Log("native pinned Code", action, "passed with repository/key/answer preserved")
	}
}
