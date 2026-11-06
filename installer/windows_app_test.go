package installer

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type windowsAppFixture struct {
	driver   *WindowsAppDriver
	resource Resource
	receipt  Receipt
	snapshot windowsAppSnapshot
	payload  []byte
	exit     int
	runs     int
}

func newWindowsAppFixture(t *testing.T) *windowsAppFixture {
	t.Helper()
	root, err := resolveConfigPath(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	program, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	data := []byte("reviewed T3 Code installer fixture")
	hash := sha256.Sum256(data)
	pin := WindowsAppPin{Version: "0.1.7", URL: "https://github.com/pingdotgg/t3code/releases/download/v0.1.7/T3-Code-0.1.7-x64.exe", SHA256: hex.EncodeToString(hash[:]), Size: int64(len(data)),
		ProductCode: "e9197887-efb3-55e0-985e-d6d3b5dd594a", Publisher: "T3 Tools Inc", Folder: "t3code", Executable: "T3 Code (Beta).exe", Uninstaller: "Uninstall T3 Code (Beta).exe"}
	f := &windowsAppFixture{resource: Resource{ID: "app.t3-code-windows", Action: "package", Scope: "user"}, receipt: Receipt{OperationID: strings.Repeat("a", 64), Status: "in-progress", Ownership: "uncertain"}, payload: data}
	f.driver = &WindowsAppDriver{Directory: filepath.Join(root, "windows-app"), WorkerDirectory: filepath.Join(root, "worker"), PowerShell: program, Programs: filepath.Join(root, "Programs"),
		Pins: map[string]WindowsAppPin{f.resource.ID: pin}}
	f.driver.Client = &http.Client{Transport: windowsVendorTransport(func(request *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, ContentLength: int64(len(f.payload)), Header: http.Header{}, Body: io.NopCloser(bytes.NewReader(f.payload)), Request: request}, nil
	})}
	f.driver.Query = func(_ context.Context, mutate bool, program string, input []byte, args ...string) ([]byte, error) {
		var fields map[string]string
		if mutate || program != f.driver.PowerShell || json.Unmarshal(input, &fields) != nil || fields["ProductCode"] != pin.ProductCode || fields["Action"] != "" || len(args) != 5 || args[3] != "-EncodedCommand" {
			t.Fatal("inspection widened its process boundary")
		}
		return json.Marshal(f.snapshot)
	}
	f.driver.Run = func(_ context.Context, command nativeCommand) ([]byte, error) {
		f.runs++
		intent, err := f.driver.readIntent(f.resource, f.receipt.OperationID)
		if err != nil {
			t.Fatal("native command started before durable intent", err)
		}
		if f.exit == 0 {
			if intent.Action == "remove" {
				f.snapshot = windowsAppSnapshot{}
			} else {
				f.installed()
			}
		}
		key, err := digest(command)
		if err != nil {
			t.Fatal(err)
		}
		code := f.exit
		if err := saveDocument(filepath.Join(f.driver.WorkerDirectory, "commands", command.Operation+".json"), nativeCommandRecord{Schema: 1, Command: key, Reply: &nativeReply{ExitCode: &code}}); err != nil {
			t.Fatal(err)
		}
		return nil, nil
	}
	return f
}

func (f *windowsAppFixture) installed() {
	pin := f.driver.Pins[f.resource.ID]
	f.snapshot = windowsAppSnapshot{Present: true, Healthy: true, Version: pin.Version, Scope: "user", Location: filepath.Join(f.driver.Programs, pin.Folder)}
}

func (f *windowsAppFixture) owned(t *testing.T) {
	t.Helper()
	after, err := f.driver.Apply(context.Background(), f.resource, Operation{Action: "install"}, f.receipt)
	if err != nil || !after.Healthy || after.CompletedOperation != f.receipt.OperationID {
		t.Fatalf("install did not prove completion: %+v: %v", after, err)
	}
	f.receipt = Receipt{Ownership: "created", After: after, Status: "in-progress", OperationID: strings.Repeat("b", 64)}
}

func TestWindowsAppInstallsAndRemovesOnlyItsOwnSignedApplication(t *testing.T) {
	f := newWindowsAppFixture(t)
	f.owned(t)
	selfUpdated := f.snapshot
	selfUpdated.Version = "0.1.9"
	f.snapshot = selfUpdated
	observed, err := f.driver.Observe(context.Background(), f.resource, Receipt{})
	if err != nil || !observed.Healthy || observed.Fingerprint != f.receipt.After.Fingerprint || observed.Desired != f.receipt.After.Desired {
		t.Fatalf("a vendor update changed the owned identity: %+v: %v", observed, err)
	}
	after, err := f.driver.Remove(context.Background(), f.resource, f.receipt)
	if err != nil || after.Present || after.CompletedOperation != f.receipt.OperationID || f.runs != 2 {
		t.Fatalf("owned removal did not prove absence: %+v: %v", after, err)
	}
}

func TestWindowsAppPreservesInstallationsThatItCannotProve(t *testing.T) {
	for name, prepare := range map[string]func(*windowsAppFixture){
		"pre-existing install": func(f *windowsAppFixture) {
			f.installed()
			f.receipt = Receipt{Ownership: "reused", After: Observation{}, Status: "in-progress", OperationID: strings.Repeat("b", 64)}
		},
		"moved installation": func(f *windowsAppFixture) {
			f.snapshot.Location = filepath.Join(f.driver.Programs, "Elsewhere")
		},
		"machine installation": func(f *windowsAppFixture) {
			f.snapshot.Scope = "machine"
		},
		"unsigned files": func(f *windowsAppFixture) {
			f.snapshot.Healthy, f.snapshot.HealthIssue = false, "The file is not signed by T3 Tools Inc"
		},
		"running application": func(f *windowsAppFixture) {
			f.snapshot.Running = true
		},
	} {
		t.Run(name, func(t *testing.T) {
			f := newWindowsAppFixture(t)
			f.owned(t)
			prepare(f)
			if _, err := f.driver.Remove(context.Background(), f.resource, f.receipt); err == nil || f.runs != 1 {
				t.Fatal("removal ran the uninstaller without proof", err)
			}
		})
	}
	f := newWindowsAppFixture(t)
	f.installed()
	if _, err := f.driver.Apply(context.Background(), f.resource, Operation{Action: "install"}, f.receipt); err == nil || f.runs != 0 {
		t.Fatal("install ran over a pre-existing application", err)
	}
}

func TestWindowsAppRefusesChangedInstallerBytes(t *testing.T) {
	f := newWindowsAppFixture(t)
	f.payload = []byte("changed T3 Code installer fixture")
	if _, err := f.driver.Apply(context.Background(), f.resource, Operation{Action: "install"}, f.receipt); err == nil || f.runs != 0 {
		t.Fatal("installer ran without its reviewed SHA-256", err)
	}
}

func TestWindowsAppReportsFailedInstallerWithoutCompletion(t *testing.T) {
	f := newWindowsAppFixture(t)
	f.exit = 2
	var failure *nativeCommandError
	if _, err := f.driver.Apply(context.Background(), f.resource, Operation{Action: "install"}, f.receipt); !errors.As(err, &failure) || *failure.ExitCode != 2 {
		t.Fatal("failed installer was not reported", err)
	}
	observed, err := f.driver.Observe(context.Background(), f.resource, f.receipt)
	if err != nil || observed.CompletedOperation != "" || observed.ResourceResume == nil {
		t.Fatalf("failed installer claimed completion: %+v: %v", observed, err)
	}
}

func TestDefaultWindowsAppPinsAreReviewed(t *testing.T) {
	pins, err := DefaultWindowsAppPins()
	if err != nil || len(pins) != 1 || pins["app.t3-code-windows"].Publisher != "T3 Tools Inc" {
		t.Fatal(pins, err)
	}
	catalog, err := DefaultCatalog()
	if err != nil {
		t.Fatal(err)
	}
	for _, binding := range []struct {
		ID, OS, Package string
		Valid           bool
	}{{"app.t3-code-windows", "windows", "t3-code", true}, {"app.other", "windows", "t3-code", false}, {"app.t3-code-windows", "windows", "zen", false}} {
		c := *catalog
		c.Resources = append([]Resource{}, catalog.Resources...)
		for i, r := range c.Resources {
			if r.ID == "app.t3-code-windows" {
				r.ID = binding.ID
				r.Bindings = map[string]Binding{binding.OS: {Provider: "windows-app", Package: binding.Package}}
				c.Resources[i] = r
			}
		}
		if binding.ID != "app.t3-code-windows" {
			for i, r := range c.Resources {
				if r.ID == "t3-code-app" {
					r.PlatformRequires = map[string][]string{"windows": {binding.ID}}
					c.Resources[i] = r
				}
			}
		}
		if err := c.Validate(); (err == nil) != binding.Valid {
			t.Fatal(binding, err)
		}
	}
}

func TestWindowsAppSelfUpdateKeepsOwnershipThroughUpdateAndRemoval(t *testing.T) {
	f := newWindowsAppFixture(t)
	f.owned(t)
	f.snapshot.Version = "0.1.9"
	updated, err := f.driver.Observe(context.Background(), f.resource, Receipt{})
	if err != nil {
		t.Fatal(err)
	}
	catalog, err := DefaultCatalog()
	if err != nil {
		t.Fatal(err)
	}
	target := Context{OS: "windows", Arch: "amd64"}
	closure, err := catalog.Closure([]string{"t3-code-app"}, target)
	if err != nil {
		t.Fatal(err)
	}
	state := State{Schema: 1, Context: target, Selected: []string{"t3-code-app"}, Receipts: map[string]Receipt{}}
	observed := map[string]Observation{}
	for _, id := range closure {
		if resource, _ := catalog.Resource(id); resource.Action != "" {
			observation := Observation{Present: true, Healthy: true, Provider: "fixture", Identity: id, Fingerprint: id}
			state.Receipts[id], observed[id] = Receipt{Ownership: "created", After: observation, Status: "ready"}, observation
		}
	}
	state.Receipts[f.resource.ID] = Receipt{Ownership: "created", After: f.receipt.After, Status: "ready"}
	observed[f.resource.ID] = updated
	for request, want := range map[string]string{"update": "keep", "apply": "remove"} {
		selected := []string{"t3-code-app"}
		if request == "apply" {
			selected = []string{}
		}
		plan, err := PlanChanges(catalog, target, state, Request{Schema: 1, Mode: request, Selected: selected}, observed, "fixture")
		if err != nil {
			t.Fatal(err)
		}
		if op := operation(t, plan, f.resource.ID); op.Action != want {
			t.Fatalf("%s after a vendor self-update: %s: %s", request, op.Action, op.Reason)
		}
	}
}
