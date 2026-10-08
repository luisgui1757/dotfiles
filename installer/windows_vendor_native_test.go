package installer

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestWindowsVendorPowerShellSyntax(t *testing.T) {
	program := os.Getenv("DOTFILES_TEST_POWERSHELL")
	if program == "" {
		var err error
		program, err = exec.LookPath("pwsh")
		if err != nil {
			t.Skip("PowerShell parser is not available")
		}
	}
	for _, script := range []string{windowsVendorObserveScript, windowsVendorInstallScript} {
		// The Windows CreateProcess limit includes the executable and encoded script.
		if len(strings.Join(windowsVendorArguments(script), " "))+len(program)+4 >= 32767 {
			t.Fatal("vendor PowerShell command exceeds the Windows command-line limit")
		}
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		command := exec.CommandContext(ctx, program, "-NoLogo", "-NoProfile", "-NonInteractive", "-Command", `$source=[Console]::In.ReadToEnd(); $tokens=$null; $problems=$null; $null=[Management.Automation.Language.Parser]::ParseInput($source,[ref]$tokens,[ref]$problems); if($problems.Count) { $problems | Format-List | Out-String | Write-Output; exit 1 }`)
		command.Stdin = strings.NewReader(script)
		output, err := command.CombinedOutput()
		cancel()
		if err != nil {
			t.Fatalf("PowerShell syntax: %s: %v", output, err)
		}
	}
}

func TestWindowsVendorWorkflowPassesCompleteNativeTestArguments(t *testing.T) {
	program := os.Getenv("DOTFILES_TEST_POWERSHELL")
	if program == "" {
		var err error
		program, err = exec.LookPath("pwsh")
		if err != nil {
			t.Skip("PowerShell is not available")
		}
	}
	workflow, err := os.ReadFile(filepath.Join("..", ".github", "workflows", "installer-engine.yml"))
	if err != nil {
		t.Fatal(err)
	}
	_, tail, ok := strings.Cut(string(workflow), "$env:WINDOWS_TEST_IMAGE ")
	if !ok {
		t.Fatal("native container test invocation is missing")
	}
	arguments, _, _ := strings.Cut(tail, "\n")
	arguments = strings.ReplaceAll(arguments, "${{ matrix.test }}", "TestWindowsVendorWorkflowArgumentHelper")
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	command := exec.CommandContext(ctx, program, "-NoLogo", "-NoProfile", "-NonInteractive", "-Command",
		"& "+powershellLiteral(executable)+" "+arguments+"; exit $LASTEXITCODE")
	command.Env = append(os.Environ(), "DOTFILES_TEST_ARGUMENT_HELPER=1")
	output, err := command.CombinedOutput()
	if err != nil || !strings.Contains(string(output), "complete native arguments arrived") {
		t.Fatalf("workflow arguments did not reach a native executable intact: %s: %v", output, err)
	}
}

func TestWindowsVendorWorkflowArgumentHelper(t *testing.T) {
	if os.Getenv("DOTFILES_TEST_ARGUMENT_HELPER") != "1" {
		t.Skip("native argument subprocess only")
	}
	t.Log("complete native arguments arrived")
}

func TestWindowsRuntimeVersionsCompareNumericComponents(t *testing.T) {
	program := os.Getenv("DOTFILES_TEST_POWERSHELL")
	if program == "" {
		var err error
		program, err = exec.LookPath("pwsh")
		if err != nil {
			t.Skip("PowerShell is not available")
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	script := "$ErrorActionPreference='Stop'; " + windowsRuntimeVersionFunction + `
foreach ($value in @('v14.51.36247.00', '14.51.36247.0', '14.51.36247')) {
	if ((ConvertTo-RuntimeVersion $value) -cne '14.51.36247.0') { throw 'Equivalent versions differ' }
}
if ((ConvertTo-RuntimeVersion '14.51.36247.1') -ceq '14.51.36247.0') { throw 'Different revisions collapsed' }
foreach ($value in @('', '14.51', '14.51.36247.0.1', '14.51.text', '14.51.36247.-1')) {
	$rejected = $false
	try { $null = ConvertTo-RuntimeVersion $value } catch { $rejected = $true }
	if (-not $rejected) { throw 'Invalid version accepted' }
}
`
	command := exec.CommandContext(ctx, program, "-NoLogo", "-NoProfile", "-NonInteractive", "-Command", script)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("native registry version comparison: %s: %v", output, err)
	}
}

// This is an actual official bundle pin, verified against Microsoft's winget
// manifest and downloaded bytes. It is test data, not a production pin source.
func windowsVendorNativePin() WindowsVendorPin {
	return WindowsVendorPin{
		Version: "14.51.36247.0", BundleID: "{0e3bb569-69d6-4c34-bff9-c2f81db5e5f0}", Size: 18731856,
		SHA256: "843068991daaa1f73ad9f6239bce4d0f6a07a51f18c37ea2a867e9beca71295c",
		URL:    "https://download.visualstudio.microsoft.com/download/pr/ebdab8e5-1d7b-4d9f-a11b-cbb1720c3b12/843068991DAAA1F73AD9F6239BCE4D0F6A07A51F18C37EA2A867E9BECA71295C/VC_redist.x64.exe",
	}
}

func windowsVendorNativeFixture(t *testing.T) (*WindowsVendorDriver, Resource) {
	t.Helper()
	if runtime.GOOS != "windows" || runtime.GOARCH != "amd64" || os.Getenv("GITHUB_ACTIONS") != "true" || os.Getenv("RUNNER_ENVIRONMENT") != "github-hosted" || os.Getenv("DOTFILES_NATIVE_WINDOWS_VENDOR") != "1" {
		t.Skip("requires explicit opt-in on a disposable GitHub-hosted Windows amd64 runner")
	}
	root, err := resolveConfigPath(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	powershell := filepath.Join(os.Getenv("SystemRoot"), "System32", "WindowsPowerShell", "v1.0", "powershell.exe")
	driver := &WindowsVendorDriver{Directory: filepath.Join(root, "vendor"), WorkerDirectory: filepath.Join(root, "worker"), PowerShell: powershell,
		Pins: map[string]WindowsVendorPin{"tool.vcredist": windowsVendorNativePin()}}
	driver.Query = queryWindowsVendor
	return driver, Resource{ID: "tool.vcredist", Action: "vcredist", Scope: "machine", Shared: true}
}

func TestWindowsVendorNativeRegistrationAndSignature(t *testing.T) {
	driver, resource := windowsVendorNativeFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	observed, err := driver.Observe(ctx, resource, Receipt{})
	if err != nil || observed.Present && !observed.Healthy {
		t.Fatalf("native runtime inspection: %+v: %v", observed, err)
	}
	if err := driver.prepare(ctx, windowsVendorIntent{Pin: driver.Pins[resource.ID]}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(driver.Directory, "payloads", driver.Pins[resource.ID].SHA256, "installer.exe")
	input, err := json.Marshal(path)
	if err != nil {
		t.Fatal(err)
	}
	script := windowsVendorPrelude + "\n$path = [Console]::In.ReadToEnd() | ConvertFrom-Json; Assert-MicrosoftSignature $path"
	if _, err := driver.Query(ctx, false, driver.PowerShell, input, windowsVendorArguments(script)...); err != nil {
		t.Fatal("official pinned runtime signature did not pass the native boundary", err)
	}
}

func TestWindowsVendorNativeOwnedLifecycle(t *testing.T) {
	driver, resource := windowsVendorNativeFixture(t)
	if os.Getenv("DOTFILES_NATIVE_WINDOWS_VENDOR_INSTALL") != "1" {
		t.Skip("installation additionally requires DOTFILES_NATIVE_WINDOWS_VENDOR_INSTALL=1")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()
	observed, err := driver.Observe(ctx, resource, Receipt{})
	if err != nil {
		t.Fatal(err)
	}
	if observed.Present {
		t.Fatal("fresh lifecycle prerequisite failed: runtime is already present; preserve it and use an isolated disposable Windows container")
	}
	root := filepath.Dir(driver.Directory)
	unlock, err := Lock(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := unlock(); err != nil {
			t.Error(err)
		}
	})
	session := &nativeSession{Directory: driver.WorkerDirectory, Start: func(context.Context) (*nativeWorkerClient, error) { return workerFixture(driver.WorkerDirectory) }}
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
	driver.Run = session.run
	// Exercise an actual changed-version upgrade, not a second installation of
	// the same bundle. This older Microsoft winget pin was downloaded and hashed.
	driver.Pins[resource.ID] = WindowsVendorPin{Version: "14.50.35719.0", BundleID: "{91ee571b-0e8a-4c65-9eaf-2e2f5fc60c00}", Size: 18558944,
		SHA256: "8995548dfffcde7c49987029c764355612ba6850ee09a7b6f0fddc85bdc5c280",
		URL:    "https://download.visualstudio.microsoft.com/download/pr/6f02464a-5e9b-486d-a506-c99a17db9a83/8995548DFFFCDE7C49987029C764355612BA6850EE09A7B6F0FDDC85BDC5C280/VC_redist.x64.exe"}
	receipt := Receipt{OperationID: strings.Repeat("a", 64), Status: "in-progress", Ownership: "uncertain"}
	after, err := driver.Apply(ctx, resource, Operation{Action: "install"}, receipt)
	if err != nil || !after.Present || !after.Healthy || after.Pending != "" || after.CompletedOperation != receipt.OperationID {
		t.Fatalf("native installation has not proved readiness: %+v: %v", after, err)
	}
	receipt.Ownership, receipt.After, receipt.OperationID = "created", after, strings.Repeat("b", 64)
	driver.Pins[resource.ID] = windowsVendorNativePin()
	after, err = driver.Apply(ctx, resource, Operation{Action: "update"}, receipt)
	if err != nil || !after.Healthy || after.Pending != "" || after.Version != windowsVendorNativePin().Version || after.Version == receipt.After.Version {
		t.Fatalf("native changed-version update has not proved readiness: %+v: %v", after, err)
	}
	receipt.After, receipt.OperationID = after, strings.Repeat("c", 64)
	after, err = driver.Apply(ctx, resource, Operation{Action: "repair"}, receipt)
	if err != nil || !after.Healthy || after.Pending != "" {
		t.Fatalf("native repair has not proved readiness: %+v: %v", after, err)
	}
	receipt.After, receipt.OperationID = after, strings.Repeat("d", 64)
	after, err = driver.Remove(ctx, resource, receipt)
	if err != nil || after.Present || after.CompletedOperation != receipt.OperationID {
		t.Fatalf("native owned removal has not proved absence: %+v: %v", after, err)
	}
}
