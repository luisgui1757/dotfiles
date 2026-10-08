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
	"path/filepath"
	"strings"
	"testing"
)

type windowsVendorTransport func(*http.Request) (*http.Response, error)

func (f windowsVendorTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

type windowsVendorFixture struct {
	driver    *WindowsVendorDriver
	resource  Resource
	receipt   Receipt
	snapshot  windowsVendorSnapshot
	exit      int
	runs      int
	downloads int
	noLog     bool
	unhealthy bool
	interrupt bool
}

func newWindowsVendorFixture(t *testing.T) *windowsVendorFixture {
	t.Helper()
	root, err := resolveConfigPath(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	program, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	data := []byte("reviewed Microsoft installer fixture")
	hash := sha256.Sum256(data)
	pin := WindowsVendorPin{Version: "14.51.36247.0", URL: "https://download.visualstudio.microsoft.com/fixture/VC_redist.x64.exe", SHA256: hex.EncodeToString(hash[:]), Size: int64(len(data)), BundleID: "{0e3bb569-69d6-4c34-bff9-c2f81db5e5f0}"}
	f := &windowsVendorFixture{resource: Resource{ID: "tool.vcredist", Action: "vcredist", Scope: "machine", Shared: true}, receipt: Receipt{OperationID: strings.Repeat("a", 64), Status: "in-progress", Ownership: "uncertain"}, snapshot: windowsVendorSnapshot{Boot: "boot-one"}}
	f.driver = &WindowsVendorDriver{Directory: filepath.Join(root, "vendor"), WorkerDirectory: filepath.Join(root, "worker"), PowerShell: program, Pins: map[string]WindowsVendorPin{f.resource.ID: pin}}
	f.driver.Client = &http.Client{Transport: windowsVendorTransport(func(request *http.Request) (*http.Response, error) {
		f.downloads++
		return &http.Response{StatusCode: http.StatusOK, ContentLength: int64(len(data)), Header: http.Header{}, Body: io.NopCloser(bytes.NewReader(data)), Request: request}, nil
	})}
	f.driver.Query = func(_ context.Context, mutate bool, program string, input []byte, args ...string) ([]byte, error) {
		if mutate || program != f.driver.PowerShell || len(input) != 0 || len(args) != 5 || args[3] != "-EncodedCommand" {
			t.Fatal("inspection widened its process boundary")
		}
		return json.Marshal(f.snapshot)
	}
	f.driver.Run = func(_ context.Context, command nativeCommand) ([]byte, error) {
		f.runs++
		intent, err := f.driver.readIntent(f.receipt.OperationID)
		if err != nil {
			t.Fatal("native command started before durable intent", err)
		}
		if f.interrupt {
			return nil, errors.New("controller disconnected before native reply")
		}
		if f.exit != 1223 {
			if intent.Action == "remove" {
				f.snapshot = windowsVendorSnapshot{Boot: "boot-one"}
			} else {
				f.installed()
				f.snapshot.Healthy = !f.unhealthy
			}
		}
		if !f.noLog && f.exit != 1223 {
			mode := map[string]string{"install": "Install", "update": "Install", "repair": "Repair", "remove": "Uninstall"}[intent.Action]
			var log strings.Builder
			for _, name := range []string{"vcRuntimeMinimum_x64", "vcRuntimeAdditional_x64"} {
				fmt.Fprintf(&log, "[fixture]i301: Applying execute package: %s, action: %s, path: fixture\n[fixture]i319: Applied execute package: %s, result: 0x0, restart: None\n", name, mode, name)
			}
			if err := os.WriteFile(filepath.Join(f.driver.Directory, "operations", command.Operation+".log"), []byte(log.String()), 0600); err != nil {
				t.Fatal(err)
			}
		}
		key, err := digest(command)
		if err != nil {
			t.Fatal(err)
		}
		code := f.exit
		reply := nativeReply{ExitCode: &code}
		if code != 0 {
			reply.Error = fmt.Sprintf("exit status %d", code)
		}
		if err := saveDocument(filepath.Join(f.driver.WorkerDirectory, "commands", command.Operation+".json"), nativeCommandRecord{Schema: 1, Command: key, Reply: &reply}); err != nil {
			t.Fatal(err)
		}
		if code != 0 {
			return nil, &nativeCommandError{Message: reply.Error, ExitCode: &code}
		}
		return nil, nil
	}
	return f
}

func (f *windowsVendorFixture) installed() {
	pin := f.driver.Pins[f.resource.ID]
	f.snapshot = windowsVendorSnapshot{Present: true, Healthy: true, Version: pin.Version, BundleID: pin.BundleID, Boot: "boot-one",
		Packages: map[string]string{"minimum": "{11111111-1111-1111-1111-111111111111}", "additional": "{22222222-2222-2222-2222-222222222222}"},
		Files:    map[string]string{"vcruntime140.dll": strings.Repeat("1", 64), "vcruntime140_1.dll": strings.Repeat("2", 64), "msvcp140.dll": strings.Repeat("3", 64)}}
}

func (f *windowsVendorFixture) apply() (Observation, error) {
	return f.driver.Apply(context.Background(), f.resource, Operation{Action: "install"}, f.receipt)
}

func TestWindowsVendorReusesPreexistingRuntimeWithoutStateOrOwnership(t *testing.T) {
	f := newWindowsVendorFixture(t)
	f.installed()
	o, err := f.driver.Observe(context.Background(), f.resource, Receipt{})
	if err != nil || !o.Healthy || o.CompletedOperation != "" || len(o.Consumers) != 0 || len(o.UnverifiedApplications) != 1 {
		t.Fatal(o, err)
	}
	if _, err := f.apply(); err == nil {
		t.Fatal("acquired a pre-existing runtime")
	}
	if f.runs != 0 || f.downloads != 0 {
		t.Fatal("pre-existing runtime caused installation")
	}
	if _, err := os.Stat(f.driver.Directory); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("observation created state", err)
	}
}

func TestWindowsVendorInstallRequiresExactProcessAndPackageEvidence(t *testing.T) {
	f := newWindowsVendorFixture(t)
	o, err := f.apply()
	if err != nil || !o.Healthy || o.CompletedOperation != f.receipt.OperationID || f.runs != 1 {
		t.Fatal(o, err)
	}
	if _, err := f.driver.Observe(context.Background(), f.resource, f.receipt); err != nil {
		t.Fatal(err)
	}
	intent, err := f.driver.readIntent(f.receipt.OperationID)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(f.driver.WorkerDirectory, "commands", intent.Commands[0].Operation+".json")); err != nil {
		t.Fatal(err)
	}
	if _, err := f.driver.Observe(context.Background(), f.resource, f.receipt); err == nil {
		t.Fatal("completion survived missing native evidence")
	}
}

func TestWindowsVendorPresentBytesOrSuccessfulExitDoNotProveInstallation(t *testing.T) {
	for _, scenario := range []string{"no-log", "unhealthy", "unrecorded"} {
		t.Run(scenario, func(t *testing.T) {
			f := newWindowsVendorFixture(t)
			f.noLog, f.unhealthy, f.interrupt = scenario == "no-log", scenario == "unhealthy", scenario == "unrecorded"
			if _, err := f.apply(); err == nil {
				t.Fatal("accepted incomplete vendor proof")
			}
			intent, err := f.driver.readIntent(f.receipt.OperationID)
			if err != nil || intent.Complete {
				t.Fatal("lost or falsely completed pending intent", err)
			}
		})
	}
}

func TestWindowsVendorUACDenialPreservesUnownedIntentAndRetriesNewAttempt(t *testing.T) {
	f := newWindowsVendorFixture(t)
	f.exit = 1223
	_, err := f.apply()
	var failure *nativeCommandError
	if !errors.As(err, &failure) || failure.ExitCode == nil || *failure.ExitCode != 1223 {
		t.Fatal(err)
	}
	o, err := f.driver.Observe(context.Background(), f.resource, f.receipt)
	if err != nil || o.Present || o.CompletedOperation != "" || o.ResourceResume == nil {
		t.Fatal(o, err)
	}
	f.exit = 0
	o, err = f.driver.ResumeResource(context.Background(), f.resource, Operation{Action: "install"}, f.receipt)
	if err != nil || !o.Healthy || o.CompletedOperation != f.receipt.OperationID {
		t.Fatal(o, err)
	}
	intent, err := f.driver.readIntent(f.receipt.OperationID)
	if err != nil || len(intent.Commands) != 2 || intent.Commands[0].Operation == intent.Commands[1].Operation {
		t.Fatal("UAC retry reused denied command", err)
	}
}

func TestWindowsVendorRebootPendingClearsOnlyAfterAnotherBoot(t *testing.T) {
	f := newWindowsVendorFixture(t)
	f.exit = 3010
	f.unhealthy = true
	o, err := f.apply()
	if err != nil || o.Pending == "" || o.Healthy || o.CompletedOperation != f.receipt.OperationID {
		t.Fatal(o, err)
	}
	f.snapshot.Healthy = true
	o, err = f.driver.Observe(context.Background(), f.resource, f.receipt)
	if err != nil || o.Pending == "" {
		t.Fatal("runtime loading bypassed required reboot", o, err)
	}
	f.snapshot.Boot = "boot-two"
	f.snapshot.Files["vcruntime140.dll"] = strings.Repeat("4", 64)
	o, err = f.driver.Observe(context.Background(), f.resource, f.receipt)
	if err != nil || o.Pending != "" || !o.Healthy || o.CompletedOperation != f.receipt.OperationID {
		t.Fatal(o, err)
	}
	f.snapshot.Packages["minimum"] = "{44444444-4444-4444-4444-444444444444}"
	o, err = f.driver.Observe(context.Background(), f.resource, f.receipt)
	if err != nil || o.CompletedOperation != "" {
		t.Fatal("reboot claimed an outside package replacement", o, err)
	}
}

func TestWindowsVendorUACRetryPreservesRuntimeInstalledByAnotherApplication(t *testing.T) {
	f := newWindowsVendorFixture(t)
	f.exit = 1223
	if _, err := f.apply(); err == nil {
		t.Fatal("expected denied elevation")
	}
	f.installed()
	f.exit = 0
	if _, err := f.driver.ResumeResource(context.Background(), f.resource, Operation{Action: "install"}, f.receipt); err == nil {
		t.Fatal("claimed an outside runtime after denial")
	}
	if f.runs != 1 {
		t.Fatal("launched after baseline changed")
	}
}

func TestWindowsVendorCompletionAcceptsNativeUTF16Log(t *testing.T) {
	f := newWindowsVendorFixture(t)
	if _, err := f.apply(); err != nil {
		t.Fatal(err)
	}
	intent, err := f.driver.readIntent(f.receipt.OperationID)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(f.driver.Directory, "operations", intent.Commands[0].Operation+".log")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	encode, _, err := profileEncoding([]byte{0xff, 0xfe})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append([]byte{0xff, 0xfe}, encode(string(data))...), 0600); err != nil {
		t.Fatal(err)
	}
	o, err := f.driver.Observe(context.Background(), f.resource, f.receipt)
	if err != nil || o.CompletedOperation != f.receipt.OperationID {
		t.Fatal(o, err)
	}
}

func TestWindowsVendorRemovalRequiresOwnershipExactBundleAndNoRegisteredConsumers(t *testing.T) {
	for _, scenario := range []string{"reused", "baseline", "changed", "consumer"} {
		t.Run(scenario, func(t *testing.T) {
			f := newWindowsVendorFixture(t)
			f.installed()
			o, err := f.driver.Observe(context.Background(), f.resource, Receipt{})
			if err != nil {
				t.Fatal(err)
			}
			f.receipt.Ownership = "created"
			f.receipt.After = o
			switch scenario {
			case "reused":
				f.receipt.Ownership = "reused"
			case "baseline":
				f.receipt.Before.Present = true
			case "changed":
				f.snapshot.BundleID = "{33333333-3333-3333-3333-333333333333}"
			case "consumer":
				f.snapshot.Consumers = []string{"registered-outside-bundle"}
			}
			if _, err := f.driver.Remove(context.Background(), f.resource, f.receipt); err == nil {
				t.Fatal("unsafe removal was allowed")
			}
			if f.runs != 0 || f.downloads != 0 {
				t.Fatal("unsafe removal dispatched native work")
			}
		})
	}
}

func TestWindowsVendorExactOwnedRemovalProvesAbsence(t *testing.T) {
	f := newWindowsVendorFixture(t)
	o, err := f.apply()
	if err != nil {
		t.Fatal(err)
	}
	f.receipt.Ownership = "created"
	f.receipt.After = o
	f.receipt.OperationID = strings.Repeat("b", 64)
	o, err = f.driver.Remove(context.Background(), f.resource, f.receipt)
	if err != nil || o.Present || o.CompletedOperation != f.receipt.OperationID {
		t.Fatal(o, err)
	}
}

func TestWindowsVendorRejectsDamagedDownloadBeforeProcessExecution(t *testing.T) {
	f := newWindowsVendorFixture(t)
	pin := f.driver.Pins[f.resource.ID]
	pin.SHA256 = strings.Repeat("0", 64)
	f.driver.Pins[f.resource.ID] = pin
	if _, err := f.apply(); err == nil {
		t.Fatal("corrupt downloaded payload accepted")
	}
	if f.runs != 0 {
		t.Fatal("executed unverified payload")
	}
}

func TestWindowsVendorRetryCannotBroadenSavedCommand(t *testing.T) {
	f := newWindowsVendorFixture(t)
	f.interrupt = true
	if _, err := f.apply(); err == nil {
		t.Fatal("expected interrupted command")
	}
	intent, err := f.driver.readIntent(f.receipt.OperationID)
	if err != nil {
		t.Fatal(err)
	}
	intent.Commands[0].Input = []byte(`{"Action":"remove"}`)
	if err := saveDocument(f.driver.intentPath(intent.Operation), intent); err != nil {
		t.Fatal(err)
	}
	if _, err := f.driver.readIntent(f.receipt.OperationID); err == nil {
		t.Fatal("saved command changed vendor authority")
	}
}
