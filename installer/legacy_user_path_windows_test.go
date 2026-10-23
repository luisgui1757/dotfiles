package installer

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf16"

	"golang.org/x/sys/windows/registry"
)

// This fixture changes the real disposable account's registry, then restores
// the original raw value and type. It never broadcasts an environment change.
func TestLegacyWindowsUserPathRetirementComposesAcrossResources(t *testing.T) {
	legacyNativePackages(t, "windows")
	key, err := registry.OpenKey(registry.CURRENT_USER, `Environment`, registry.QUERY_VALUE|registry.SET_VALUE)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := key.Close(); err != nil {
			t.Error(err)
		}
	})
	original, kind, readErr := key.GetStringValue("Path")
	if readErr != nil && readErr != registry.ErrNotExist {
		t.Fatal(readErr)
	}
	if readErr == nil && kind != registry.SZ && kind != registry.EXPAND_SZ {
		t.Fatal("unsupported fixture baseline registry type", kind)
	}
	if readErr == nil {
		size, _, err := key.GetValue("Path", nil)
		if err != nil || size > 1<<20 {
			t.Fatal("cannot snapshot bounded original User PATH", err)
		}
		raw := make([]byte, size)
		n, rawKind, err := key.GetValue("Path", raw)
		encoded := append(utf16.Encode([]rune(original)), 0)
		roundtrip := make([]byte, 2*len(encoded))
		for i, unit := range encoded {
			binary.LittleEndian.PutUint16(roundtrip[2*i:], unit)
		}
		if err != nil || n != size || rawKind != kind || !bytes.Equal(raw, roundtrip) {
			t.Fatal("original User PATH cannot be restored byte-exactly; fixture made no change", err)
		}
	}
	t.Cleanup(func() {
		var err error
		switch {
		case readErr == registry.ErrNotExist:
			err = key.DeleteValue("Path")
		case kind == registry.EXPAND_SZ:
			err = key.SetExpandStringValue("Path", original)
		default:
			err = key.SetStringValue("Path", original)
		}
		if err != nil {
			t.Error("restore original User PATH", err)
		}
		actual, actualKind, err := key.GetStringValue("Path")
		if readErr == registry.ErrNotExist {
			if err != registry.ErrNotExist {
				t.Error("originally absent User PATH was not restored", err)
			}
		} else if err != nil || actual != original || actualKind != kind {
			t.Error("original raw User PATH or type was not restored", err)
		}
	})
	d := legacyNativeDriver(t)
	a, b := filepath.Join(d.Folders.Home, "herdr ü"), filepath.Join(d.Folders.Home, "tree-sitter ü")
	outside := original + `;;%SystemRoot%\outside;C:\Personal tools;`
	initial := a + ";" + outside + ";" + b + ";" + strings.ToUpper(b) + `\`
	plan := func(t *testing.T, entry string) legacyRetirementScope {
		t.Helper()
		scope := legacyRetirementScope{}
		if err := d.planUserPathEntry(context.Background(), &scope, entry); err != nil || len(scope.Steps) != 1 {
			t.Fatal(scope, err)
		}
		return scope
	}
	for _, change := range []string{"earlier approved removal", "unrelated addition", "matching entry changed", "value type changed"} {
		t.Run(change, func(t *testing.T) {
			if err := key.SetExpandStringValue("Path", initial); err != nil {
				t.Fatal(err)
			}
			before := plan(t, b)
			legacyBefore, err := d.userPathEntry(context.Background(), b)
			if err != nil {
				t.Fatal(err)
			}
			want := a + ";" + outside
			wantKind := uint32(registry.EXPAND_SZ)
			switch change {
			case "earlier approved removal":
				if !t.Run("remove-first-resource", func(t *testing.T) {
					runLegacyNativeSteps(t, d, plan(t, a))
				}) {
					t.Fatal("earlier resource removal failed")
				}
				want = outside
			case "unrelated addition":
				prefix := `C:\Added after approval;`
				if err := key.SetExpandStringValue("Path", prefix+initial); err != nil {
					t.Fatal(err)
				}
				want = prefix + want
			case "matching entry changed":
				if err := key.SetExpandStringValue("Path", initial+";"+b); err != nil {
					t.Fatal(err)
				}
			case "value type changed":
				if err := key.SetStringValue("Path", initial); err != nil {
					t.Fatal(err)
				}
				wantKind = registry.SZ
			}
			oldID, err := digest(before)
			if err != nil {
				t.Fatal(err)
			}
			freshID, err := digest(plan(t, b))
			if err != nil {
				t.Fatal(err)
			}
			if change == "matching entry changed" {
				if oldID == freshID || d.evaluateGates(context.Background(), before.Steps[0].Gates) == nil {
					t.Fatal("target change retained stale approval")
				}
				command := *before.Steps[0].Command
				command.Operation, err = digest("direct native refusal " + t.Name())
				if err != nil {
					t.Fatal(err)
				}
				_, err = d.Run(context.Background(), command)
				var nativeError *nativeCommandError
				if !errors.As(err, &nativeError) || nativeError.ExitCode == nil || *nativeError.ExitCode != 72 {
					t.Fatal("fixed removal script did not independently refuse changed owned entries", err)
				}
				actual, actualKind, err := key.GetStringValue("Path")
				if err != nil || actual != initial+";"+b || actualKind != registry.EXPAND_SZ {
					t.Fatal("refused native script changed User PATH", err)
				}
				runLegacyNativeSteps(t, d, plan(t, b))
				actual, actualKind, err = key.GetStringValue("Path")
				if err != nil || actual != want || actualKind != wantKind {
					t.Fatal("fresh approval failed after owned entry change", err)
				}
				return
			}
			if oldID != freshID {
				t.Fatal("unrelated User PATH change invalidated exact-entry approval")
			}
			legacyAfter, err := d.userPathEntry(context.Background(), b)
			if err != nil || legacyBefore == legacyAfter {
				t.Fatal("historical whole-PATH gate lost its original semantics", err)
			}
			runLegacyNativeSteps(t, d, before)
			actual, actualKind, err := key.GetStringValue("Path")
			if err != nil || actual != want || actualKind != wantKind {
				t.Fatal("unrelated raw User PATH entries, order or type changed", err)
			}
		})
	}
	for _, change := range []bool{false, true} {
		name := "historical unchanged"
		if change {
			name = "historical unrelated edit"
		}
		t.Run(name, func(t *testing.T) {
			if err := key.SetExpandStringValue("Path", initial); err != nil {
				t.Fatal(err)
			}
			value, err := d.userPathEntry(context.Background(), b)
			if err != nil {
				t.Fatal(err)
			}
			_, hash, _ := strings.Cut(value, ":")
			command, err := d.windowsScript(legacyUserPathEdit, struct {
				Entry string `json:"entry"`
				Hash  string `json:"hash"`
			}{b, hash})
			if err != nil {
				t.Fatal(err)
			}
			step := legacyRetirementStep{Kind: "command", Label: "Historical whole-PATH removal", Command: &command,
				Gates:  []legacyRetirementGate{{Kind: "path-absent", Subject: b, Expected: "absent"}, {Kind: "user-path", Subject: b, Expected: value}},
				Checks: []legacyRetirementGate{{Kind: "user-path", Subject: b, Expected: "absent"}}}
			intent := legacyTestIntent(t, d, step)
			receipt := Receipt{Ownership: "uncertain", Before: intent.Before, OperationID: intent.Operation, Status: "in-progress"}
			if _, err := d.readIntent("retire.tools", receipt); err != nil {
				t.Fatal("historical saved intent is unreadable", err)
			}
			want := a + ";" + outside
			if change {
				want = initial + `;C:\Unrelated new entry`
				if err := key.SetExpandStringValue("Path", want); err != nil {
					t.Fatal(err)
				}
			}
			err = d.executeStep(context.Background(), intent, 0, false)
			if (err != nil) != change {
				t.Fatal("historical whole-PATH approval semantics changed", err)
			}
			actual, actualKind, err := key.GetStringValue("Path")
			if err != nil || actual != want || actualKind != registry.EXPAND_SZ {
				t.Fatal("historical removal changed unapproved bytes", err)
			}
		})
	}
	t.Run("controller transaction", func(t *testing.T) {
		folders := d.Folders
		folders.LocalAppData = filepath.Join(folders.Home, "Local data ü")
		normalDriver := &fileDriver{dir: folders.Home, home: folders.Home, statePath: filepath.Join(folders.Home, "state", "state.json")}
		normal := Controller{Catalog: &Catalog{Schema: 1, Resources: []Resource{
			{ID: "tool.herdr", Name: "Herdr", Capability: true, Action: "fixture"},
			{ID: "tool.tree-sitter", Name: "Tree-sitter", Capability: true, Action: "fixture"},
		}}, Context: d.Platform.Context, Source: "native-path-fixture", Home: folders.Home, StatePath: normalDriver.statePath, Driver: normalDriver}
		dispatchApproved(t, normal, Request{Schema: 1, Mode: "apply", Selected: []string{}})
		environment := d.Environment
		environment.Executable, err = os.Executable()
		if err != nil {
			t.Fatal(err)
		}
		retirement, err := newLegacyRetirement(normal, d.Platform, folders, folders.Home, environment)
		if err != nil {
			t.Fatal(err)
		}
		// Launch the real native worker through the test executable's process
		// entrypoint; the production controller/session/records stay intact.
		retirement.driver.session.Start = func(context.Context) (*nativeWorkerClient, error) {
			return workerFixture(retirement.driver.WorkerDirectory)
		}
		herdr := filepath.Join(folders.LocalAppData, "Programs", "Herdr", "bin")
		treesitter := filepath.Join(folders.LocalAppData, "dotfiles", "bin")
		value := outside + ";" + herdr + ";" + treesitter + ";" + treesitter + `\`
		if err := key.SetExpandStringValue("Path", value); err != nil {
			t.Fatal(err)
		}
		request := Request{Schema: 1, Mode: "apply", Selected: []string{"retire.delta.herdr", "retire.delta.tree-sitter"}}
		dispatchApproved(t, retirement.Controller, request)
		actual, actualKind, err := key.GetStringValue("Path")
		if err != nil || actual != outside || actualKind != registry.EXPAND_SZ {
			t.Fatal("one approved controller transaction failed to preserve unrelated PATH", err)
		}
		state, err := LoadState(retirement.Controller.StatePath, folders.Home)
		if err != nil || state.Transaction != nil || state.Receipts[request.Selected[0]].Status != "ready" || state.Receipts[request.Selected[1]].Status != "ready" {
			t.Fatal("both retirement resources must have completed receipts", err)
		}
	})
}
