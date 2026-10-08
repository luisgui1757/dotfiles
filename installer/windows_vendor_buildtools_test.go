package installer

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

type windowsBuildToolsFixture struct {
	driver           *WindowsBuildToolsDriver
	state            windowsBuildToolsSnapshot
	receipt          Receipt
	code, runs       int
	missingComponent bool
}

func newWindowsBuildToolsFixture(t *testing.T) *windowsBuildToolsFixture {
	t.Helper()
	root, err := resolveConfigPath(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	program, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	data := []byte("official Microsoft Build Tools fixture")
	hash := sha256.Sum256(data)
	f := &windowsBuildToolsFixture{state: windowsBuildToolsSnapshot{Boot: "boot-one"}, receipt: Receipt{OperationID: strings.Repeat("a", 64), Status: "in-progress", Ownership: "uncertain"}}
	f.driver = &WindowsBuildToolsDriver{Directory: filepath.Join(root, "vendor"), WorkerDirectory: filepath.Join(root, "worker"), PowerShell: program, InstallDirectory: filepath.Join(root, "Program Files", "DotfilesBuildTools"), Pin: WindowsBuildToolsPin{Version: "17.14.37710.0", URL: "https://download.visualstudio.microsoft.com/fixture/vs_BuildTools.exe", SHA256: hex.EncodeToString(hash[:]), Size: int64(len(data)), Components: append([]string{}, windowsBuildToolsComponents...)}}
	f.driver.Client = &http.Client{Transport: windowsVendorTransport(func(request *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, ContentLength: int64(len(data)), Header: http.Header{}, Body: io.NopCloser(bytes.NewReader(data)), Request: request}, nil
	})}
	f.driver.Query = func(_ context.Context, mutate bool, program string, input []byte, args ...string) ([]byte, error) {
		if mutate || program != f.driver.PowerShell {
			t.Fatal("inspection escaped its process boundary")
		}
		var request map[string]any
		if err := json.Unmarshal(input, &request); err != nil {
			t.Fatal(err)
		}
		if request["Probe"] != nil {
			return json.Marshal(map[string]string{"PATH": "approved-bin", "INCLUDE": "approved-include", "LIB": "approved-lib", "LIBPATH": "approved-libpath", "VCINSTALLDIR": "approved-vc", "VCToolsInstallDir": "approved-vctools", "WindowsSdkDir": "approved-sdk", "WindowsSDKVersion": "10.0.26100.0\\"})
		}
		return json.Marshal(f.state)
	}
	f.driver.Run = func(_ context.Context, command nativeCommand) ([]byte, error) {
		f.runs++
		intent, err := f.driver.readIntent(f.receipt.OperationID)
		if err != nil {
			t.Fatal("command ran before saved intent", err)
		}
		if f.code != 1223 {
			if intent.Action == "remove" {
				f.state = windowsBuildToolsSnapshot{Boot: "boot-one"}
			} else {
				f.installed()
			}
		}
		key, err := digest(command)
		if err != nil {
			t.Fatal(err)
		}
		code := f.code
		reply := nativeReply{ExitCode: &code}
		if code != 0 {
			reply.Error = "native vendor returned nonzero"
		}
		if err := saveDocument(filepath.Join(f.driver.WorkerDirectory, "commands", command.Operation+".json"), nativeCommandRecord{Schema: 1, Command: key, Reply: &reply}); err != nil {
			t.Fatal(err)
		}
		return nil, nil
	}
	return f
}

func (f *windowsBuildToolsFixture) installed() {
	packages := []windowsBuildToolsPackage{}
	for _, component := range windowsBuildToolsComponents {
		packages = append(packages, windowsBuildToolsPackage{ID: component, Version: "17.14.0.0", Type: "Component", Count: 1})
	}
	if f.missingComponent {
		packages = packages[:2]
	}
	f.state = windowsBuildToolsSnapshot{Selected: "1234abcd", Healthy: true, Occupied: true, Boot: "boot-one", Instances: []windowsBuildToolsInstance{{ID: "1234abcd", Path: f.driver.InstallDirectory, Product: "Microsoft.VisualStudio.Product.BuildTools", Version: "17.14.37710.0", Complete: true, Packages: packages, Files: map[string]string{"cl.exe": strings.Repeat("1", 64), "link.exe": strings.Repeat("2", 64), "VsDevCmd.bat": strings.Repeat("3", 64), "Microsoft.VCToolsVersion.default.txt": strings.Repeat("4", 64)}}}}
}
func (f *windowsBuildToolsFixture) apply() (Observation, error) {
	return f.driver.Apply(context.Background(), Resource{ID: "tool.compiler"}, Operation{Action: "install"}, f.receipt)
}

func TestWindowsBuildToolsReusesPreexistingCompilerWithoutOwnership(t *testing.T) {
	f := newWindowsBuildToolsFixture(t)
	f.installed()
	f.state.Instances[0].Path = filepath.Join(filepath.Dir(f.driver.InstallDirectory), "Visual Studio Enterprise")
	o, err := f.driver.Observe(context.Background(), Resource{ID: "tool.compiler"}, Receipt{})
	if err != nil || !o.Healthy || o.CompletedOperation != "" {
		t.Fatal(o, err)
	}
	if _, err := f.apply(); err == nil || f.runs != 0 {
		t.Fatal("claimed pre-existing compiler", err)
	}
}

func TestWindowsBuildToolsInstallAndResolveRestrictedCompilerEnvironment(t *testing.T) {
	f := newWindowsBuildToolsFixture(t)
	if _, err := f.driver.CompilerEnvironment(context.Background()); err == nil {
		t.Fatal("resolved compiler environment before installation")
	}
	o, err := f.apply()
	if err != nil || !o.Healthy || o.CompletedOperation != f.receipt.OperationID {
		t.Fatal(o, err)
	}
	environment, err := f.driver.CompilerEnvironment(context.Background())
	if err != nil || len(environment) != 8 || environment["PATH"] != "approved-bin" {
		t.Fatal(environment, err)
	}
}

func TestWindowsBuildToolsDoesNotClaimMissingRequestedSDK(t *testing.T) {
	f := newWindowsBuildToolsFixture(t)
	f.missingComponent = true
	if _, err := f.apply(); err == nil {
		t.Fatal("accepted missing pinned SDK component")
	}
	intent, err := f.driver.readIntent(f.receipt.OperationID)
	if err != nil || intent.Complete {
		t.Fatal("falsely completed missing component", err)
	}
}

func TestWindowsBuildToolsUACRetryAndRebootRemainExplicit(t *testing.T) {
	f := newWindowsBuildToolsFixture(t)
	f.code = 1223
	if _, err := f.apply(); err == nil {
		t.Fatal("accepted cancelled elevation")
	}
	o, err := f.driver.Observe(context.Background(), Resource{ID: "tool.compiler"}, f.receipt)
	if err != nil || o.ResourceResume == nil || o.Present {
		t.Fatal(o, err)
	}
	f.code = 3010
	o, err = f.driver.ResumeResource(context.Background(), Resource{ID: "tool.compiler"}, Operation{Action: "install"}, f.receipt)
	if err != nil || o.Pending == "" || o.CompletedOperation != f.receipt.OperationID || f.runs != 2 {
		t.Fatal(o, err)
	}
	f.state.Boot = "boot-two"
	o, err = f.driver.Observe(context.Background(), Resource{ID: "tool.compiler"}, f.receipt)
	if err != nil || o.Pending != "" {
		t.Fatal(o, err)
	}
}

func TestWindowsBuildToolsOwnedRemovalPreservesPreexistingAndChangedInstances(t *testing.T) {
	for _, scenario := range []string{"reused", "baseline", "outside-component", "multiplicity", "extension-flag", "other-path", "consumer"} {
		t.Run(scenario, func(t *testing.T) {
			f := newWindowsBuildToolsFixture(t)
			o, err := f.apply()
			if err != nil {
				t.Fatal(err)
			}
			f.receipt.Ownership, f.receipt.After, f.receipt.OperationID = "created", o, strings.Repeat("b", 64)
			switch scenario {
			case "reused":
				f.receipt.Ownership = "reused"
			case "baseline":
				f.receipt.Before.Present = true
			case "outside-component":
				f.state.Instances[0].Packages = append(f.state.Instances[0].Packages, windowsBuildToolsPackage{ID: "Outside.Extension", Version: "1.0", Type: "Vsix", Extension: true, Count: 1})
			case "multiplicity":
				f.state.Instances[0].Packages[0].Count++
			case "extension-flag":
				f.state.Instances[0].Packages[0].Extension = true
			case "other-path":
				f.state.Instances[0].Path = filepath.Join(filepath.Dir(f.driver.InstallDirectory), "Other")
			case "consumer":
				f.state.Consumers = []string{"Running cl.exe (PID 123)"}
			}
			if _, err := f.driver.Remove(context.Background(), Resource{ID: "tool.compiler"}, f.receipt); err == nil || f.runs != 1 {
				t.Fatal("unsafe removal reached vendor", err)
			}
		})
	}
}

func TestWindowsBuildToolsRemovesOnlyExactOwnedInstance(t *testing.T) {
	f := newWindowsBuildToolsFixture(t)
	o, err := f.apply()
	if err != nil {
		t.Fatal(err)
	}
	f.receipt.Ownership, f.receipt.After, f.receipt.OperationID = "created", o, strings.Repeat("b", 64)
	o, err = f.driver.Remove(context.Background(), Resource{ID: "tool.compiler"}, f.receipt)
	if err != nil || o.Present || o.CompletedOperation != f.receipt.OperationID || len(o.Preserved) == 0 {
		t.Fatal(o, err)
	}
}

func TestWindowsBuildToolsCompletionNeedsRecordedNativeExit(t *testing.T) {
	f := newWindowsBuildToolsFixture(t)
	if _, err := f.apply(); err != nil {
		t.Fatal(err)
	}
	intent, err := f.driver.readIntent(f.receipt.OperationID)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(f.driver.WorkerDirectory, "commands", intent.Commands[0].Operation+".json")); err != nil {
		t.Fatal(err)
	}
	if _, err := f.driver.Observe(context.Background(), Resource{ID: "tool.compiler"}, f.receipt); err == nil {
		t.Fatal("claimed completion without native evidence")
	}
}

func TestWindowsBuildToolsPowerShellSyntax(t *testing.T) {
	program := os.Getenv("DOTFILES_TEST_POWERSHELL")
	if program == "" {
		var err error
		program, err = exec.LookPath("pwsh")
		if err != nil {
			t.Skip("PowerShell parser is not available")
		}
	}
	for _, script := range []string{windowsBuildToolsObserveScript, windowsBuildToolsInstallScript, windowsBuildToolsEnvironmentScript, windowsBuildToolsEnvironmentProbe} {
		if len(strings.Join(windowsVendorArguments(script), " "))+len(program)+4 >= 32767 {
			t.Fatal("Build Tools script exceeds Windows command-line limit")
		}
		command := exec.Command(program, "-NoLogo", "-NoProfile", "-NonInteractive", "-Command", `$source=[Console]::In.ReadToEnd();$tokens=$null;$problems=$null;$null=[Management.Automation.Language.Parser]::ParseInput($source,[ref]$tokens,[ref]$problems);if($problems.Count){$problems | Format-List | Out-String | Write-Output;exit 1}`)
		command.Stdin = strings.NewReader(script)
		output, err := command.CombinedOutput()
		if err != nil {
			t.Fatal(string(output), err)
		}
	}
}

func TestWindowsBuildToolsCompilerEnvironmentRejectsExtraVariables(t *testing.T) {
	f := newWindowsBuildToolsFixture(t)
	f.installed()
	original := f.driver.Query
	f.driver.Query = func(ctx context.Context, mutate bool, program string, input []byte, args ...string) ([]byte, error) {
		var req map[string]any
		if err := json.Unmarshal(input, &req); err != nil {
			return nil, err
		}
		if req["Probe"] != nil {
			return []byte(`{"PERSONAL_TOKEN":"redacted"}`), nil
		}
		return original(ctx, mutate, program, input, args...)
	}
	if _, err := f.driver.CompilerEnvironment(context.Background()); err == nil || errors.Is(err, os.ErrNotExist) {
		t.Fatal("accepted unrelated environment", err)
	}
}

// Exercise the production inventory projection in PowerShell; only its native
// vswhere process boundary is replaced by JSON returned from that process.
func TestWindowsBuildToolsPackageInventoryPreservesMultiplicity(t *testing.T) {
	program := os.Getenv("DOTFILES_TEST_POWERSHELL")
	if program == "" {
		var err error
		program, err = exec.LookPath("pwsh")
		if err != nil {
			t.Skip("PowerShell is unavailable")
		}
	}
	start := strings.Index(windowsBuildToolsPrelude, "$packages = @()")
	end := strings.Index(windowsBuildToolsPrelude, "$instances +=")
	if start < 0 || end <= start {
		t.Fatal("production inventory projection missing")
	}
	script := "$ErrorActionPreference='Stop';" + windowsBuildToolsPackagePrelude + "$item=[Console]::In.ReadToEnd() | ConvertFrom-Json;" + windowsBuildToolsPrelude[start:end] + "ConvertTo-Json -InputObject $packages -Depth 3 -Compress"
	for _, scenario := range []struct {
		name, packages string
		count, total   int
	}{
		{"parallel versions", `[{"id":"SDK","version":"1.0","type":"Msi"},{"id":"SDK","version":"2.0","type":"Msi"}]`, 2, 2},
		{"parallel branches", `[{"id":"SDK","version":"1.0","branch":"stable","type":"Msi"},{"id":"SDK","version":"1.0","branch":"preview","type":"Msi"}]`, 2, 2},
		{"identical record", `[{"id":"SDK","version":"1.0","type":"Msi"},{"id":"SDK","version":"1.0","type":"Msi"}]`, 1, 2},
		{"extension distinction", `[{"id":"SDK","version":"1.0","type":"Vsix"},{"id":"SDK","version":"1.0","type":"Vsix","extension":true}]`, 2, 2},
		{"ordinal case distinction", `[{"id":"SDK","version":"1.0","type":"Msi"},{"id":"sdk","version":"1.0","type":"Msi"}]`, 2, 2},
		{"empty inventory", `[]`, 0, 0},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			command := exec.Command(program, "-NoLogo", "-NoProfile", "-NonInteractive", "-Command", script)
			command.Stdin = strings.NewReader(`{"packages":` + scenario.packages + `}`)
			output, err := command.CombinedOutput()
			if err != nil {
				t.Fatal(string(output), err)
			}
			var packages []map[string]any
			if err := json.Unmarshal(output, &packages); err != nil || len(packages) != scenario.count {
				t.Fatal(packages, string(output), err)
			}
			total := 0
			for _, entry := range packages {
				count, ok := entry["count"].(float64)
				if !ok || count < 1 {
					t.Fatal("invalid multiplicity", entry)
				}
				total += int(count)
			}
			if total != scenario.total {
				t.Fatal("inventory lost package occurrences", total, scenario.total)
			}
		})
	}
}

func TestWindowsBuildToolsPackageOrderDoesNotChangeFingerprint(t *testing.T) {
	f := newWindowsBuildToolsFixture(t)
	f.installed()
	f.state.Instances[0].Packages[0].Count = 2
	before, err := f.driver.Observe(context.Background(), Resource{ID: "tool.compiler"}, Receipt{})
	if err != nil {
		t.Fatal(err)
	}
	slices.Reverse(f.state.Instances[0].Packages)
	after, err := f.driver.Observe(context.Background(), Resource{ID: "tool.compiler"}, Receipt{})
	if err != nil || before.Fingerprint != after.Fingerprint {
		t.Fatal("package enumeration order changed identity", before, after, err)
	}
}

func TestWindowsBuildToolsRejectsOldInventoryJournalWithoutReinterpretation(t *testing.T) {
	f := newWindowsBuildToolsFixture(t)
	path := f.driver.intentPath(f.receipt.OperationID)
	old := []byte(`{"Schema":1,"Before":{"instances":[{"packages":{"SDK|1.0||||Msi":"1.0"}}]}}`)
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, old, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := f.apply(); err == nil || !strings.Contains(err.Error(), "preserve the instance and original journal for explicit recovery") {
		t.Fatal("old package map was reinterpreted", err)
	}
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(old, after) || f.runs != 0 {
		t.Fatal("legacy operation changed", err, f.runs)
	}
}

func TestWindowsBuildToolsPackageOccurrenceBounds(t *testing.T) {
	for _, count := range []int{0, -1, 10001, 10000} {
		t.Run(fmt.Sprint(count), func(t *testing.T) {
			f := newWindowsBuildToolsFixture(t)
			f.installed()
			f.state.Instances[0].Packages[0].Count = count
			if _, err := f.driver.query(context.Background()); err == nil {
				t.Fatal("accepted invalid individual or total multiplicity", count)
			}
		})
	}
	f := newWindowsBuildToolsFixture(t)
	f.installed()
	f.state.Instances[0].Packages[0].Count = 9998
	if _, err := f.driver.query(context.Background()); err != nil {
		t.Fatal("rejected exact 10000 occurrence bound", err)
	}
}

func TestWindowsBuildToolsWorkerRechecksPackageMultiplicity(t *testing.T) {
	program := os.Getenv("DOTFILES_TEST_POWERSHELL")
	if program == "" {
		var err error
		program, err = exec.LookPath("pwsh")
		if err != nil {
			t.Skip("PowerShell is unavailable")
		}
	}
	start := strings.Index(windowsBuildToolsInstallScript, "$expectedPackages =")
	end := strings.Index(windowsBuildToolsInstallScript[start:], "foreach ($field in @('files'))") + start
	if start < 0 || end <= start {
		t.Fatal("production package guard missing")
	}
	script := "$ErrorActionPreference='Stop';" + windowsBuildToolsPackagePrelude + `$data=[Console]::In.ReadToEnd() | ConvertFrom-Json;$before=@([pscustomobject]@{packages=$data.before});$instance=@([pscustomobject]@{packages=$data.after});` + windowsBuildToolsInstallScript[start:end]
	base := []windowsBuildToolsPackage{{ID: "SDK", Version: "1.0", Type: "Msi", Count: 2}, {ID: "Extension", Version: "2.0", Type: "Vsix", Extension: true, Count: 1}}
	for _, scenario := range []string{"reordered", "count changed", "record missing", "extension changed"} {
		t.Run(scenario, func(t *testing.T) {
			after := slices.Clone(base)
			switch scenario {
			case "reordered":
				slices.Reverse(after)
			case "count changed":
				after[0].Count--
			case "record missing":
				after = after[:1]
			case "extension changed":
				after[1].Extension = false
			}
			input, err := json.Marshal(map[string]any{"before": base, "after": after})
			if err != nil {
				t.Fatal(err)
			}
			command := exec.Command(program, "-NoLogo", "-NoProfile", "-NonInteractive", "-Command", script)
			command.Stdin = bytes.NewReader(input)
			output, err := command.CombinedOutput()
			if scenario == "reordered" {
				if err != nil {
					t.Fatal(string(output), err)
				}
			} else if err == nil || !strings.Contains(string(output), "dedicated instance package") {
				t.Fatal("changed package bag reached vendor execution", string(output), err)
			}
		})
	}
}
