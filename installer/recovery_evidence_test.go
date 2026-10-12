package installer

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInvalidHistoricalJournalBodyIsPreservedWithoutBlocking(t *testing.T) {
	for _, provider := range []string{"profiles", "configuration"} {
		t.Run(provider, func(t *testing.T) {
			var c Controller
			if provider == "profiles" {
				c, _, _ = profileController(t)
			} else {
				c, _, _ = nativeConfigFixture(t, "copy")
			}
			dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"test"}})
			state, err := LoadState(c.StatePath, c.Home)
			if err != nil {
				t.Fatal(err)
			}
			id := "profile.test"
			if provider == "configuration" {
				id = "config.test"
			}
			operation := state.Receipts[id].OperationID
			dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{}})
			path := filepath.Join(filepath.Dir(c.StatePath), provider, "operations", operation+".json")
			data, err := readDocument(path)
			if err != nil {
				t.Fatal(err)
			}
			var journal map[string]any
			if err := Decode(data, &journal); err != nil {
				t.Fatal(err)
			}
			journal["sealed"], journal["entries"] = false, []any{}
			if err := saveDocument(path, journal); err != nil {
				t.Fatal(err)
			}
			before, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			result := dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"test"}})
			after, err := os.ReadFile(path)
			if err != nil || string(after) != string(before) || !strings.Contains(result.Message, path) {
				t.Fatal("invalid historical body was not retained and disclosed", result, err)
			}
		})
	}
}

// Simulate damage at the external persistence boundary after engine completion
// proof, before provider finalization. The real finalizer must still reject it.
type corruptCompletedJournal struct {
	Driver
	finalizer TransactionDriver
	directory string
	id        string
	damage    string
}

func (d corruptCompletedJournal) FinishTransaction(ctx context.Context, plan Plan, receipts map[string]Receipt) (map[string][]string, error) {
	receipt := receipts[d.id]
	if receipt.Status != "ready" {
		return nil, errors.New("fixture did not reach completed receipt")
	}
	path := filepath.Join(d.directory, receipt.OperationID+".json")
	switch d.damage {
	case "missing-journal":
		if err := os.Rename(path, path+".saved"); err != nil {
			return nil, err
		}
	case "replaced-directory":
		if err := os.Rename(d.directory, d.directory+"-saved"); err != nil {
			return nil, err
		}
		if err := os.WriteFile(d.directory, []byte("unexpected personal file"), 0600); err != nil {
			return nil, err
		}
	default:
		if err := os.WriteFile(path, []byte("{damaged just-completed journal"), 0600); err != nil {
			return nil, err
		}
	}
	return d.finalizer.FinishTransaction(ctx, plan, receipts)
}

func TestJustCompletedJournalIsRequiredBeforeTransactionFinalization(t *testing.T) {
	for _, provider := range []string{"profiles", "configuration"} {
		for _, damage := range []string{"corrupt-journal", "missing-journal", "replaced-directory"} {
			t.Run(provider+"/"+damage, func(t *testing.T) {
				var c Controller
				id := "profile.test"
				if provider == "profiles" {
					c, _, _ = profileController(t)
				} else {
					c, _, _ = nativeConfigFixture(t, "copy")
					id = "config.test"
				}
				c.Driver = corruptCompletedJournal{c.Driver, c.Driver.(TransactionDriver), filepath.Join(filepath.Dir(c.StatePath), provider, "operations"), id, damage}
				request := Request{Schema: 1, Mode: "apply", Selected: []string{"test"}}
				plan, err := c.Preview(context.Background(), request)
				if err != nil {
					t.Fatal(err)
				}
				request.ExpectedPlan = plan.ID
				if _, err := c.Dispatch(context.Background(), request); err == nil {
					t.Fatal("just-completed damaged proof did not block finalization", err)
				}
				state, err := LoadState(c.StatePath, c.Home)
				if err != nil || state.Transaction == nil || state.Receipts[id].Status != "ready" {
					t.Fatal("failed finalization lost the completed receipt or recovery intent", state, err)
				}
			})
		}
	}
}

func TestRecoveryRejectsProviderIdentityDisagreement(t *testing.T) {
	d := &NativeDriver{}
	receipt := Receipt{Before: Observation{Provider: "profile-block"}, After: Observation{Provider: "configuration"}}
	if _, err := d.ObserveRestore(context.Background(), "profile.test", receipt); err == nil || !strings.Contains(err.Error(), "identities disagree") {
		t.Fatal("ambiguous saved identities reached a provider", err)
	}
	if err := d.RestoreResource(context.Background(), "profile.test", Observation{}, receipt); err == nil || !strings.Contains(err.Error(), "identities disagree") {
		t.Fatal("ambiguous saved identities reached a mutation provider", err)
	}
}

func TestProfileRecoveryRejectsOverlappingTargets(t *testing.T) {
	c, d, j, _ := interruptedProfileController(t, "update", "moved")
	state, err := LoadState(c.StatePath, c.Home)
	if err != nil {
		t.Fatal(err)
	}
	entry := j.Entries[0]
	entry.Path = filepath.Join(entry.Path, "nested")
	entry.Workspace = profileWorkspace(entry.Path, j.Operation, 1)
	j.Entries = append(j.Entries, entry)
	if err := saveDocument(d.journalPath(j.Operation), j); err != nil {
		t.Fatal(err)
	}
	if _, err := d.ObserveRestore(context.Background(), "profile.test", state.Receipts["profile.test"]); err == nil || !strings.Contains(err.Error(), "overlapping") {
		t.Fatal("overlapping saved targets authorized restoration", err)
	}
}
