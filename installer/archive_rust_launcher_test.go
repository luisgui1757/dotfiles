package installer

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestArchiveRustCheckAndUpdateSurviveUnrelatedInstallerRebuild(t *testing.T) {
	c, _, _ := archiveController(t)
	d, _ := rustPreparationFixture(t)
	c.Driver = d
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"first"}})
	payload, err := d.PayloadPath("tool.shared")
	if err != nil {
		t.Fatal(err)
	}
	before, err := snapshotTree(d.resourceDirectory("tool.shared"), maxPackageBytes, maxPackageEntries)
	if err != nil {
		t.Fatal(err)
	}
	// The installed generation retains its original launcher. Only the process
	// supplying future preparations has changed, with the same launcher recipe.
	newBytes := []byte("unrelated installer rebuild")
	if err := os.WriteFile(d.WindowsRustLauncher, newBytes, 0755); err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256(newBytes)
	d.Pins["tool.shared"].WindowsRustLauncher.SHA256 = hex.EncodeToString(hash[:])
	c.Source = "archive-fixture-rebuilt"
	check, err := c.Dispatch(context.Background(), Request{Schema: 1, Mode: "check"})
	if err != nil || check.Status != "ready" {
		t.Fatal("unrelated installer rebuild invalidated installed Rust", check, err)
	}
	dispatchApproved(t, c, Request{Schema: 1, Mode: "update"})
	after, err := snapshotTree(d.resourceDirectory("tool.shared"), maxPackageBytes, maxPackageEntries)
	if err != nil || before != after {
		t.Fatal("unrelated installer rebuild rewrote owned Rust evidence", err)
	}
	current, err := d.PayloadPath("tool.shared")
	if err != nil || current != payload {
		t.Fatal("unrelated installer rebuild redownloaded Rust", current, err)
	}
	launcher, err := os.ReadFile(filepath.Join(payload, "bin", "rustc.exe"))
	if err != nil || string(launcher) != "trusted fixture launcher" {
		t.Fatal("existing launcher bytes were replaced", err)
	}
	writeConfigFixture(t, filepath.Join(payload, "bin", "rustc.exe"), "changed installed launcher")
	check, err = c.Dispatch(context.Background(), Request{Schema: 1, Mode: "check"})
	if err != nil || check.Status != "needs-action" {
		t.Fatal("installed launcher tampering went unnoticed", check, err)
	}
}

func TestArchiveRustLauncherBindsExactBytesAndPreservesOriginals(t *testing.T) {
	d, pin := rustPreparationFixture(t)
	payload := filepath.Join(d.Directory, "payload")
	if err := d.preparePayload(context.Background(), archiveIntent{Pin: pin}, payload); err != nil {
		t.Fatal(err)
	}
	for name, original := range map[string]string{"rustc.exe": "rust compiler", "rustdoc.exe": "rust documentation", "clippy-driver.exe": "clippy-driver"} {
		data, err := os.ReadFile(filepath.Join(payload, "bin", name))
		if err != nil || string(data) != "trusted fixture launcher" {
			t.Fatal(name, string(data), err)
		}
		data, err = os.ReadFile(filepath.Join(payload, "bin", rustOriginalCommand(name)))
		if err != nil || string(data) != original {
			t.Fatal("original changed", name, string(data), err)
		}
	}
}

func TestArchiveRustLauncherRefusesUnboundChangedAndOldPreparationBeforeDownload(t *testing.T) {
	for _, kind := range []string{"old", "unbound", "changed", "relative", "unknown-revision", "bad-hash"} {
		t.Run(kind, func(t *testing.T) {
			d, pin := rustPreparationFixture(t)
			switch kind {
			case "old":
				pin.WindowsRustLauncher = nil
			case "unbound":
				pin.WindowsRustLauncher.SHA256 = ""
			case "changed":
				if err := os.WriteFile(d.WindowsRustLauncher, []byte("changed"), 0755); err != nil {
					t.Fatal(err)
				}
			case "relative":
				d.WindowsRustLauncher = "installer.exe"
			case "unknown-revision":
				pin.WindowsRustLauncher.Revision = 2
			case "bad-hash":
				pin.WindowsRustLauncher.SHA256 = "invalid"
			}
			downloads := 0
			d.Client = &http.Client{Transport: portableGitTransport(func(*http.Request) (*http.Response, error) {
				downloads++
				return nil, errors.New("unexpected download")
			})}
			payload := t.TempDir()
			err := d.preparePayload(context.Background(), archiveIntent{Pin: pin}, payload)
			if err == nil || downloads != 0 || kind == "old" && !strings.Contains(err.Error(), "preserve the original operation and payload") {
				t.Fatal(kind, downloads, err)
			}
			entries, readErr := os.ReadDir(payload)
			if readErr != nil || len(entries) != 0 {
				t.Fatal("invalid launcher modified payload", entries, readErr)
			}
		})
	}
}

func TestArchiveRustLauncherRejectsNestedAndNonWindowsRecipes(t *testing.T) {
	_, pin := rustPreparationFixture(t)
	pin.RustComponents[0].WindowsRustLauncher = &WindowsRustLauncherPin{Revision: 1}
	if err := pin.Validate(); err == nil {
		t.Fatal("nested launcher accepted")
	}
	pin.RustComponents[0].WindowsRustLauncher = nil
	pin.RustTarget = "aarch64-apple-darwin"
	if err := validateWindowsRustLauncherPin(pin); err == nil {
		t.Fatal("non-Windows launcher accepted")
	}
}

func TestArchiveRustLauncherPreservesOldProvenanceAcrossPreparedGenerations(t *testing.T) {
	for _, oldShape := range []bool{true, false} {
		t.Run(map[bool]string{true: "old-shape", false: "changed-installer"}[oldShape], func(t *testing.T) {
			d, _ := rustPreparationFixture(t)
			r := Resource{ID: "tool.shared"}
			receipt := Receipt{OperationID: strings.Repeat("a", 64), Status: "in-progress"}
			before, err := d.Observe(context.Background(), r, receipt)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := d.Apply(context.Background(), r, Operation{Action: "install", Observed: before}, receipt); err != nil {
				t.Fatal(err)
			}
			version, err := d.version(r.ID, receipt.OperationID)
			if err != nil {
				t.Fatal(err)
			}
			intent, err := d.readIntent(r.ID, receipt.OperationID)
			if err != nil {
				t.Fatal(err)
			}
			versionPath := filepath.Join(d.versionDirectory(r.ID, receipt.OperationID), "version.json")
			if oldShape {
				version.Pin.WindowsRustLauncher = nil
				intent.Pin.WindowsRustLauncher = nil
				data, err := json.Marshal(version.Pin)
				if err != nil || strings.Contains(string(data), "windows_rust_launcher") {
					t.Fatal("old pin shape changed", err)
				}
				var restored ArchivePin
				if err := Decode(data, &restored); err != nil || restored.Validate() != nil || archivePinID(restored) != archivePinID(version.Pin) {
					t.Fatal("old pin identity changed", err)
				}
				if err := saveDocument(versionPath, version); err != nil {
					t.Fatal(err)
				}
				if err := saveDocument(d.intentPath(r.ID, receipt.OperationID), intent); err != nil {
					t.Fatal(err)
				}
			} else {
				newBytes := []byte("new trusted installer executable")
				if err := os.WriteFile(d.WindowsRustLauncher, newBytes, 0755); err != nil {
					t.Fatal(err)
				}
				hash := sha256.Sum256(newBytes)
				d.Pins[r.ID].WindowsRustLauncher.SHA256 = hex.EncodeToString(hash[:])
			}
			saved, err := os.ReadFile(versionPath)
			if err != nil {
				t.Fatal(err)
			}
			observed, err := d.Observe(context.Background(), r, receipt)
			if err != nil || !observed.Present || observed.Healthy == oldShape || observed.Adoptable || observed.Pending != "" {
				t.Fatal("old generation is not available for normal update", observed, err)
			}
			receipt.After, receipt.Ownership, receipt.OperationID = observed, "created", strings.Repeat("b", 64)
			after, err := d.Apply(context.Background(), r, Operation{Action: "update", Observed: observed}, receipt)
			if err != nil || !after.Healthy {
				t.Fatal("launcher change did not reconcile", after, err)
			}
			if data, err := os.ReadFile(versionPath); err != nil || string(data) != string(saved) {
				t.Fatal("old provenance changed", err)
			}
		})
	}
}

func TestWindowsRustCatalogBindsBytesWithoutChangingDesiredRecipe(t *testing.T) {
	platform := NativePlatform{Context: Context{OS: "windows", Arch: "amd64"}, WindowsRustLauncherSHA256: strings.Repeat("a", 64)}
	first, err := DefaultArchivePins(platform)
	if err != nil {
		t.Fatal(err)
	}
	platform.WindowsRustLauncherSHA256 = strings.Repeat("b", 64)
	second, err := DefaultArchivePins(platform)
	if err != nil {
		t.Fatal(err)
	}
	if first["tool.rust"].WindowsRustLauncher.SHA256 != strings.Repeat("a", 64) || archivePinID(first["tool.rust"]) == archivePinID(second["tool.rust"]) {
		t.Fatal("installer hash does not bind Rust preparation")
	}
	if archiveDesiredID(first["tool.rust"]) != archiveDesiredID(second["tool.rust"]) {
		t.Fatal("installer rebuild changes the Rust recipe")
	}
	changed := second["tool.rust"]
	launcher := *changed.WindowsRustLauncher
	launcher.Revision++
	changed.WindowsRustLauncher = &launcher
	if archiveDesiredID(first["tool.rust"]) == archiveDesiredID(changed) {
		t.Fatal("launcher recipe revision failed to change desired identity")
	}
	if archivePinID(first["tool.python"]) != archivePinID(second["tool.python"]) {
		t.Fatal("unrelated archive identity changed")
	}
}
