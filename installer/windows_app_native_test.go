package installer

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestNativeWindowsT3CodeAppLifecycle(t *testing.T) {
	if runtime.GOOS != "windows" || runtime.GOARCH != "amd64" || os.Getenv("GITHUB_ACTIONS") != "true" || os.Getenv("RUNNER_ENVIRONMENT") != "github-hosted" || os.Getenv("DOTFILES_NATIVE_WINDOWS_APP") != "1" {
		t.Skip("requires explicit opt-in on a disposable GitHub-hosted Windows amd64 runner")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	folders, err := DiscoverConfigFolders()
	if err != nil {
		t.Fatal(err)
	}
	root, err := resolveConfigPath(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	pins, err := DefaultWindowsAppPins()
	if err != nil {
		t.Fatal(err)
	}
	powershell := filepath.Join(os.Getenv("SystemRoot"), "System32", "WindowsPowerShell", "v1.0", "powershell.exe")
	session := &nativeSession{Directory: filepath.Join(root, "worker"), Start: func(context.Context) (*nativeWorkerClient, error) {
		return workerFixture(filepath.Join(root, "worker"))
	}}
	driver, err := configureWindowsApps(NativePlatform{Context: Context{OS: "windows", Arch: "amd64"}, WindowsPowerShell: powershell}, folders, session)
	if err != nil {
		t.Fatal(err)
	}
	resource := Resource{ID: "app.t3-code-windows", Action: "package", Scope: "user"}
	pin := pins[resource.ID]
	location := filepath.Join(folders.LocalAppData, "Programs", pin.Folder)
	observed, err := driver.Observe(ctx, resource, Receipt{})
	if err != nil || observed.Unknown || observed.Present {
		t.Fatalf("fresh lifecycle needs an absent T3 Code: %+v: %v", observed, err)
	}
	unlock, err := Lock(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := unlock(); err != nil {
			t.Error(err)
		}
	})
	native := &NativeDriver{StatePath: filepath.Join(root, "state.json"), session: session}
	release, err := native.AcquireMutation(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := release(); err != nil {
			t.Error(err)
		}
	})
	witness := filepath.Join(folders.Home, ".t3", "dotfiles-native-witness.txt")
	writeConfigFixture(t, witness, "personal T3 data survives the app lifecycle\n")
	kept, err := snapshotConfig(witness)
	if err != nil {
		t.Fatal(err)
	}

	receipt := Receipt{OperationID: strings.Repeat("a", 64), Status: "in-progress", Ownership: "uncertain"}
	after, err := driver.Apply(ctx, resource, Operation{Action: "install"}, receipt)
	if err != nil || !after.Healthy || after.Scope != "user" || !strings.EqualFold(after.Identity, location) || after.CompletedOperation != receipt.OperationID {
		t.Fatalf("native install has not proved the signed per-user app: %+v: %v", after, err)
	}
	application := exec.Command(filepath.Join(location, pin.Executable))
	if err := application.Start(); err != nil {
		t.Fatal(err)
	}
	time.Sleep(20 * time.Second)
	running, err := driver.Observe(ctx, resource, Receipt{})
	if err != nil || !running.Healthy {
		t.Fatalf("running app lost its identity: %+v: %v", running, err)
	}
	stop := exec.CommandContext(ctx, powershell, "-NoLogo", "-NoProfile", "-NonInteractive", "-Command",
		"$prefix = $env:T3_LOCATION.TrimEnd('\\') + '\\'; $found = @(Get-CimInstance -ClassName Win32_Process | Where-Object { $_.ExecutablePath -and $_.ExecutablePath.StartsWith($prefix, [StringComparison]::OrdinalIgnoreCase) }); if (-not $found.Count) { throw 'T3 Code exited during its first 20 seconds' }; $found | ForEach-Object { Stop-Process -Id $_.ProcessId -Force -ErrorAction SilentlyContinue }; Start-Sleep -Seconds 5")
	stop.Env = append(os.Environ(), "T3_LOCATION="+location)
	if output, err := stop.CombinedOutput(); err != nil {
		t.Fatalf("T3 Code did not stay running: %s: %v", output, err)
	}
	_ = application.Wait()

	receipt = Receipt{Ownership: "created", After: after, Status: "in-progress", OperationID: strings.Repeat("b", 64)}
	after, err = driver.Apply(ctx, resource, Operation{Action: "repair"}, receipt)
	if err != nil || !after.Healthy || after.CompletedOperation != receipt.OperationID {
		t.Fatalf("native repair has not proved readiness: %+v: %v", after, err)
	}
	receipt = Receipt{Ownership: "created", After: after, Status: "in-progress", OperationID: strings.Repeat("c", 64)}
	after, err = driver.Remove(ctx, resource, receipt)
	if err != nil || after.Present || after.CompletedOperation != receipt.OperationID {
		t.Fatalf("native removal has not proved absence: %+v: %v", after, err)
	}
	if now, err := snapshotConfig(witness); err != nil || now != kept {
		t.Fatal("removal changed personal T3 data", err)
	}

	installer := filepath.Join(driver.Directory, "payloads", pin.SHA256, "installer.exe")
	if output, err := exec.CommandContext(ctx, installer, "/S").CombinedOutput(); err != nil {
		t.Fatalf("manual vendor install: %s: %v", output, err)
	}
	t.Cleanup(func() {
		uninstall := exec.Command(powershell, "-NoLogo", "-NoProfile", "-NonInteractive", "-Command",
			"& (Join-Path $env:T3_LOCATION $env:T3_UNINSTALLER) /currentuser /S; $deadline = [DateTime]::UtcNow.AddMinutes(10); while ((Test-Path -LiteralPath (Join-Path $env:T3_LOCATION $env:T3_UNINSTALLER)) -and [DateTime]::UtcNow -lt $deadline) { Start-Sleep -Seconds 2 }")
		uninstall.Env = append(os.Environ(), "T3_LOCATION="+location, "T3_UNINSTALLER="+pin.Uninstaller)
		if output, err := uninstall.CombinedOutput(); err != nil {
			t.Errorf("clean up the manual install: %s: %v", output, err)
		}
	})
	observed, err = driver.Observe(ctx, resource, Receipt{})
	if err != nil || !observed.Present || !observed.Healthy {
		t.Fatalf("manual vendor install is not reusable: %+v: %v", observed, err)
	}
	reused := Receipt{Ownership: "reused", Before: observed, After: observed, Status: "in-progress", OperationID: strings.Repeat("d", 64)}
	if _, err := driver.Remove(ctx, resource, reused); err == nil {
		t.Fatal("removal claimed a manual installation")
	}
	if observed, err = driver.Observe(ctx, resource, Receipt{}); err != nil || !observed.Healthy {
		t.Fatalf("refused removal changed the manual installation: %+v: %v", observed, err)
	}
}
