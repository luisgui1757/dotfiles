//go:build darwin || linux

package installer

import (
	"context"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestLinuxCodeCurrentPinUpdateNeedsNeitherDownloadNorMutation(t *testing.T) {
	f, c := linuxCodeControllerFixture(t)
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"vscode"}})
	calls := f.runs
	f.driver.Client = &http.Client{Transport: windowsVendorTransport(func(*http.Request) (*http.Response, error) {
		t.Error("healthy owned Code at its reviewed pin attempted a download")
		return nil, errors.New("fixture is offline")
	})}
	for range 2 {
		preview, err := c.Preview(context.Background(), Request{Schema: 1, Mode: "update"})
		if err != nil || operation(t, preview, "tool.vscode").Action != "keep" {
			t.Fatal("matching Code pin was not kept", preview, err)
		}
		dispatchApproved(t, c, Request{Schema: 1, Mode: "update"})
	}
	if f.runs != calls {
		t.Fatal("matching Code update executed native commands", calls, f.runs)
	}
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{}, RemoveShared: []string{"tool.vscode"}})
}

func TestLinuxCodeCompletedPackagePreservesChangedCleanupInputs(t *testing.T) {
	for _, change := range []string{"extra", "payload", "redirect", "debconf"} {
		t.Run(change, func(t *testing.T) {
			f, c := linuxCodeControllerFixture(t)
			file := f.driver.codePayload(*f.driver.Code)
			kept := filepath.Dir(file)
			personal := filepath.Join(kept, "personal")
			var original []byte
			run := f.driver.Run
			changed := false
			f.driver.Run = func(ctx context.Context, command nativeCommand) ([]byte, error) {
				output, err := run(ctx, command)
				if err != nil || changed || !slices.Contains(command.Arguments, "install") {
					return output, err
				}
				changed = true
				switch change {
				case "payload":
					personal = file
					original, err = os.ReadFile(file)
					if err != nil {
						t.Fatal(err)
					}
				case "redirect":
					outside := filepath.Join(t.TempDir(), "outside")
					if err := os.Rename(kept, outside); err != nil {
						t.Fatal(err)
					}
					if err := os.Symlink(outside, kept); err != nil {
						t.Fatal(err)
					}
					personal = filepath.Join(outside, "personal")
				case "debconf":
					files, err := filepath.Glob(filepath.Join(f.driver.Directory, "operations", "*.json"))
					if err != nil || len(files) != 1 {
						t.Fatal(files, err)
					}
					intent, err := f.driver.intent(strings.TrimSuffix(filepath.Base(files[0]), ".json"))
					if err != nil || intent.Code == nil {
						t.Fatal(intent, err)
					}
					kept = intent.Code.Directory
					personal = filepath.Join(kept, "personal")
					t.Cleanup(func() {
						if err := os.Remove(personal); err != nil {
							t.Error(err)
						}
					})
				}
				writeConfigFixture(t, personal, "preserve these personal bytes")
				return output, nil
			}
			result := dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"vscode"}})
			state, err := LoadState(c.StatePath, c.Home)
			receipt := state.Receipts["tool.vscode"]
			if err != nil || !changed || state.Transaction != nil || receipt.Ownership != "created" || receipt.After.CompletedOperation != receipt.OperationID || !slices.Contains(receipt.After.Preserved, kept) || !strings.Contains(result.Message, kept) {
				t.Fatal("optional input cleanup stranded package completion or lost disclosure", result, state, err)
			}
			dispatchApproved(t, c, Request{Schema: 1, Mode: "update"})
			if data, err := os.ReadFile(personal); err != nil || string(data) != "preserve these personal bytes" {
				t.Fatal("completed install/update modified personal data", string(data), err)
			}
			expected := "preserve these personal bytes"
			if change == "payload" {
				// Even restoring the original input bytes does not revoke the
				// already recorded preservation decision or authorize deletion.
				if err := os.WriteFile(personal, original, 0600); err != nil {
					t.Fatal(err)
				}
				expected = string(original)
			}
			result = dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{}, RemoveShared: []string{"tool.vscode"}})
			state, err = LoadState(c.StatePath, c.Home)
			if err != nil || state.Transaction != nil || state.Receipts["tool.vscode"].After.Present || !slices.Contains(state.Receipts["tool.vscode"].After.Preserved, kept) || !strings.Contains(result.Message, kept) {
				t.Fatal("package removal lost retained input disclosure", result, state, err)
			}
			if data, err := os.ReadFile(personal); err != nil || string(data) != expected {
				t.Fatal("input cleanup modified personal data", string(data), err)
			}
		})
	}
}

func TestLinuxCodeLegacyReceiptWithoutDesiredReconcilesOnce(t *testing.T) {
	f, c := linuxCodeControllerFixture(t)
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"vscode"}})
	state, err := LoadState(c.StatePath, c.Home)
	if err != nil {
		t.Fatal(err)
	}
	receipt := state.Receipts["tool.vscode"]
	receipt.After.Desired = "" // Existing schema-1 receipts omit this field.
	state.Receipts["tool.vscode"] = receipt
	if err := SaveState(c.StatePath, state); err != nil {
		t.Fatal(err)
	}
	preview, err := c.Preview(context.Background(), Request{Schema: 1, Mode: "update"})
	if err != nil || operation(t, preview, "tool.vscode").Action != "update" {
		t.Fatal("legacy receipt acquired unrecorded desired-input evidence", preview, err)
	}
	dispatchApproved(t, c, Request{Schema: 1, Mode: "update"})
	calls := f.runs
	dispatchApproved(t, c, Request{Schema: 1, Mode: "update"})
	state, err = LoadState(c.StatePath, c.Home)
	if err != nil || f.runs != calls || state.Receipts["tool.vscode"].After.Desired == "" || state.Receipts["tool.vscode"].Before.Present {
		t.Fatal("legacy reconciliation repeated mutation or lost ownership baseline", state, err)
	}
}

func TestLinuxCodeDesiredNeverSuppressesDriftOrRepair(t *testing.T) {
	for _, change := range []string{"pin", "older", "newer", "unhealthy", "missing", "source", "reused"} {
		t.Run(change, func(t *testing.T) {
			f, c := linuxCodeControllerFixture(t)
			if change != "reused" {
				dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"vscode"}})
			} else {
				f.installed["code:amd64"] = nativePackage{Name: "code:amd64", Source: "code", Version: f.driver.Code.Version, Healthy: true}
				dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"vscode"}})
			}
			pkg := f.installed["code:amd64"]
			expected := "update"
			switch change {
			case "pin":
				pin := *f.driver.Code
				pin.Version = "1.142.0-12345"
				pin.URL = strings.Replace(pin.URL, f.driver.Code.Version, pin.Version, 1)
				f.driver.Code, f.version = &pin, pin.Version
			case "older":
				pkg.Version = "1.140.0-12345"
			case "newer":
				pkg.Version = "1.142.0-12345"
			case "unhealthy":
				pkg.Healthy, expected = false, "repair"
			case "missing":
				expected = "install"
			case "source":
				pkg.Source, expected = "personal-source", "pending"
			case "reused":
				expected = "keep"
			}
			f.installed[pkg.Name] = pkg
			if change == "missing" {
				delete(f.installed, pkg.Name)
			}
			preview, err := c.Preview(context.Background(), Request{Schema: 1, Mode: "update", Selected: []string{"vscode"}})
			if err != nil {
				t.Fatal(err)
			}
			op := operation(t, preview, "tool.vscode")
			if op.Action != expected || change != "pin" && op.Observed.Desired != "" {
				t.Fatal("desired-input optimization suppressed native drift or granted ownership", op)
			}
			if change == "pin" {
				dispatchApproved(t, c, Request{Schema: 1, Mode: "update"})
				if f.installed[pkg.Name].Version != f.driver.Code.Version {
					t.Fatal("new reviewed pin was not installed")
				}
			}
		})
	}
}

func TestLinuxCodeCompletedIntentBeforeCleanupRemainsReadable(t *testing.T) {
	f, r := linuxCodeFixture(t)
	f.lostReply = true
	f.approve()
	receipt := aptFixtureReceipt("complete-before-cleanup")
	if _, err := f.driver.Apply(context.Background(), r, Operation{Action: "install"}, receipt); err == nil {
		t.Fatal("missing saved successful worker outcome")
	}
	intent, err := f.driver.intent(receipt.OperationID)
	if err != nil {
		t.Fatal(err)
	}
	// Model process death after the already-proved ownership/completion writes,
	// before optional input cleanup. This is the existing intent shape.
	ledger, err := f.driver.ledger()
	if err != nil {
		t.Fatal(err)
	}
	pkg := f.installed["code:amd64"]
	ledger.Roots[r.ID] = linuxAPTOwnership{pkg.Name, pkg.Source, intent.Operation}
	shared := f.installed["shared:amd64"]
	ledger.Pool[shared.Name] = linuxAPTOwnership{shared.Name, shared.Source, intent.Operation}
	if err := saveDocument(filepath.Join(f.driver.Directory, "packages.json"), ledger); err != nil {
		t.Fatal(err)
	}
	intent.Complete = true
	if err := saveDocument(f.driver.intentPath(intent.Operation), intent); err != nil {
		t.Fatal(err)
	}
	calls := f.runs
	got, err := f.driver.Observe(context.Background(), r, receipt)
	if err != nil || got.CompletedOperation != receipt.OperationID || !got.Healthy || len(got.Preserved) != 2 || !slices.Contains(got.Preserved, intent.Code.Directory) || !slices.Contains(got.Preserved, filepath.Dir(f.driver.codePayload(intent.Code.Pin))) || f.runs != calls {
		t.Fatal("completed journal could not disclose interrupted cleanup read-only", got, err)
	}
}
