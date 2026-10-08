package installer

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// Real controller construction, state, observation and publication; expose only
// the actual shell integration to avoid installing unrelated native packages.
func shellLoginControllerFactory(t *testing.T) (string, func() Controller) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("POSIX Bash login profile ownership")
	}
	home, err := resolveConfigPath(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	repository, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	folders := ConfigFolders{Home: home, Config: filepath.Join(home, ".config"), Zsh: home}
	return home, func() Controller {
		t.Helper()
		c, err := NewNativeController(repository, "shell-login-test", filepath.Join(home, "state"), nativePlatform(t), folders)
		if err != nil {
			t.Fatal(err)
		}
		r, ok := c.Catalog.Resource("integration.shells")
		if !ok {
			t.Fatal("missing actual shell integration")
		}
		r.Capability = true
		c.Catalog = &Catalog{Schema: 1, Resources: []Resource{r}}
		c.Driver.(*NativeDriver).Catalog = c.Catalog
		return c
	}
}

func shellLoginState(t *testing.T, c Controller) (State, Receipt, *ProfileDriver) {
	t.Helper()
	state, err := LoadState(c.StatePath, c.Home)
	if err != nil {
		t.Fatal(err)
	}
	return state, state.Receipts["integration.shells"], c.Driver.(*NativeDriver).Profiles
}

func TestBashLoginOwnershipSurvivesHigherPriorityPersonalProfile(t *testing.T) {
	for _, names := range [][2]string{{".profile", ".bash_profile"}, {".profile", ".bash_login"}, {".bash_login", ".bash_profile"}} {
		t.Run(names[0]+"-then-"+names[1], func(t *testing.T) {
			home, fresh := shellLoginControllerFactory(t)
			old, newer := filepath.Join(home, names[0]), filepath.Join(home, names[1])
			original := "# existing personal login configuration\n"
			personal := "# later personal profile, no automatic adoption\n"
			writeConfigFixture(t, old, original)
			c := fresh()
			dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"integration.shells"}})
			_, receipt, d := shellLoginState(t, c)
			baseline, err := os.ReadFile(d.baselinePath(receipt.Recovery))
			if err != nil {
				t.Fatal(err)
			}
			// The existing schema-1 baseline is the whole migration contract.
			var saved profileBaseline
			if err := Decode(baseline, &saved); err != nil || saved.Schema != 1 {
				t.Fatal("invalid original schema-1 baseline", err)
			}
			writeConfigFixture(t, newer, personal)
			data, err := os.ReadFile(old)
			if err != nil {
				t.Fatal(err)
			}
			writeConfigFixture(t, old, string(data)+"# later edit in the original profile\n")
			dispatchApproved(t, fresh(), Request{Schema: 1, Mode: "update"})
			if after, err := os.ReadFile(d.baselinePath(receipt.Recovery)); err != nil || !bytes.Equal(after, baseline) {
				t.Fatal("update replaced the original baseline", err)
			}
			dispatchApproved(t, fresh(), Request{Schema: 1, Mode: "apply", Selected: []string{}})
			for path, want := range map[string]string{old: original + "# later edit in the original profile\n", newer: personal} {
				if data, err := os.ReadFile(path); err != nil || string(data) != want {
					t.Fatalf("personal profile was changed at %s: %q %v", path, data, err)
				}
			}
			if state, _, _ := shellLoginState(t, fresh()); state.Transaction != nil || len(state.Selected) != 0 || len(state.Receipts) != 0 {
				t.Fatal("scoped shell removal left ownership or recovery intent", state)
			}
		})
	}
}

func TestBashLoginSavedTargetRejectsChangedBaselineOrRedirect(t *testing.T) {
	for _, change := range []string{"foreign-path", "different-allowed-path", "missing-entry", "receipt-identity", "redirect"} {
		t.Run(change, func(t *testing.T) {
			home, fresh := shellLoginControllerFactory(t)
			old := filepath.Join(home, ".profile")
			writeConfigFixture(t, old, "# original personal bytes\n")
			c := fresh()
			dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"integration.shells"}})
			state, receipt, d := shellLoginState(t, c)
			baseline, err := d.readBaseline("integration.shells", receipt)
			if err != nil {
				t.Fatal(err)
			}
			personal := filepath.Join(home, ".bash_profile")
			writeConfigFixture(t, personal, "# new personal login\n")
			foreign := filepath.Join(home, "foreign-profile")
			writeConfigFixture(t, foreign, "# preserve redirected file\n")
			switch change {
			case "foreign-path":
				baseline.Entries[len(baseline.Entries)-1].Path = foreign
			case "different-allowed-path":
				baseline.Entries[len(baseline.Entries)-1].Path = personal
			case "missing-entry":
				baseline.Entries = baseline.Entries[:len(baseline.Entries)-1]
			case "receipt-identity":
				receipt.Before.Identity = strings.Repeat("a", 64)
				state.Receipts["integration.shells"] = receipt
				if err := SaveState(c.StatePath, state); err != nil {
					t.Fatal(err)
				}
			case "redirect":
				if err := os.Remove(old); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(foreign, old); err != nil {
					t.Fatal(err)
				}
			}
			if err := saveDocument(d.baselinePath(receipt.Recovery), baseline); err != nil {
				t.Fatal(err)
			}
			before := map[string][]byte{}
			for _, path := range []string{old, personal, foreign, c.StatePath, d.baselinePath(receipt.Recovery)} {
				before[path], err = os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
			}
			for _, request := range []Request{{Schema: 1, Mode: "update"}, {Schema: 1, Mode: "apply", Selected: []string{}}} {
				result, err := fresh().Dispatch(context.Background(), request)
				if err == nil {
					for _, op := range result.Plan.Operations {
						if mutating(op.Action) || op.Observed.Pending == "" {
							t.Fatal("damaged ownership can authorize profile mutation", change, result)
						}
					}
				}
			}
			for path, want := range before {
				if got, err := os.ReadFile(path); err != nil || !bytes.Equal(got, want) {
					t.Fatal("refused observation changed a saved or personal file", path, err)
				}
			}
		})
	}
}

func TestBashLoginUncertainReceiptUsesExistingBaseline(t *testing.T) {
	home, fresh := shellLoginControllerFactory(t)
	old := filepath.Join(home, ".profile")
	writeConfigFixture(t, old, "# original\n")
	c := fresh()
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"integration.shells"}})
	state, receipt, _ := shellLoginState(t, c)
	receipt.Ownership, receipt.Status = "uncertain", "in-progress"
	state.Receipts["integration.shells"] = receipt
	writeConfigFixture(t, filepath.Join(home, ".bash_profile"), "# later personal profile\n")
	bound, err := fresh().Driver.(*NativeDriver).ForSelection([]string{"integration.shells"}, state)
	if err != nil {
		t.Fatal(err)
	}
	d := bound.(*NativeDriver).Profiles
	if _, err := d.readBaseline("integration.shells", receipt); err != nil {
		t.Fatal("saved interrupted baseline could not be verified", err)
	}
	if got := d.Targets["integration.shells"]; got[len(got)-1].Path != old {
		t.Fatal("interrupted receipt changed its login target", got)
	}
}

func TestBashLoginBeforeFirstBaselineRequiresTheOriginalTargets(t *testing.T) {
	home, fresh := shellLoginControllerFactory(t)
	writeConfigFixture(t, filepath.Join(home, ".profile"), "# original\n")
	c := fresh()
	d := c.Driver.(*NativeDriver).Profiles
	before, err := d.inspect("integration.shells")
	if err != nil {
		t.Fatal(err)
	}
	receipt := Receipt{Ownership: "uncertain", Status: "in-progress", Before: before,
		Recovery: filepath.Join("recovery", strings.Repeat("a", 64), "integration.shells"), OperationID: strings.Repeat("b", 64)}
	state := State{Receipts: map[string]Receipt{"integration.shells": receipt}}
	if _, err := fresh().Driver.(*NativeDriver).ForSelection([]string{"integration.shells"}, state); err != nil {
		t.Fatal("unchanged first-write intent cannot resume", err)
	}
	writeConfigFixture(t, filepath.Join(home, ".bash_profile"), "# personal higher-priority profile\n")
	if _, err := fresh().Driver.(*NativeDriver).ForSelection([]string{"integration.shells"}, state); err == nil {
		t.Fatal("missing baseline allowed a different login profile to replace saved intent")
	}
}
