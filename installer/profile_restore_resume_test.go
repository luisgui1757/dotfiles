package installer

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestScopedProfileRestorationResumesEveryPublicationCut(t *testing.T) {
	for _, cut := range []string{"intent", "staged", "moved", "published"} {
		t.Run(cut, func(t *testing.T) {
			c, d, j, expected := interruptedScopedRestore(t, cut)
			path := j.Entries[0].Path
			state, err := LoadState(c.StatePath, c.Home)
			if err != nil {
				t.Fatal(err)
			}
			receipt := state.Receipts["profile.test"]
			assertProfileRestorationCannotBeReversed(t, c, j)
			result := dispatchApproved(t, c, Request{Schema: 1, Mode: "restore"})
			if result.Status != "ready" {
				t.Fatal("restoration did not converge", result)
			}
			actual, err := os.ReadFile(path)
			if err != nil || !bytes.Equal(actual, expected) {
				t.Fatal("resumed restoration lost active personal settings", string(actual), err)
			}
			j, err = d.readRecoveryJournal("profile.test", receipt)
			if err != nil || len(j.Restoration[0].Content) != 0 {
				t.Fatal("restoration retained full profile bytes in metadata", err)
			}
			dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"test"}})
			dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{}})
		})
	}
}

func assertProfileRestorationCannotBeReversed(t *testing.T, c Controller, j profileJournal) {
	t.Helper()
	for _, request := range []Request{{Schema: 1, Mode: "update", Retry: true}, {Schema: 1, Mode: "abandon"}} {
		stateBefore, err := os.ReadFile(c.StatePath)
		if err != nil {
			t.Fatal(err)
		}
		before, err := digest(profileRecoveryFiles(j))
		if err != nil {
			t.Fatal(err)
		}
		plan, err := c.Preview(context.Background(), request)
		if err == nil {
			request.ExpectedPlan = plan.ID
			_, err = c.Dispatch(context.Background(), request)
		}
		if err == nil {
			t.Fatal("unfinished restoration allowed a different direction", request.Mode)
		}
		if request.Mode == "abandon" && !strings.Contains(err.Error(), "retry restoration") {
			t.Fatal("abandon gave the wrong recovery instruction", err)
		}
		after, err := digest(profileRecoveryFiles(j))
		if err != nil || before != after {
			t.Fatal("refused action changed profile recovery files", err)
		}
		stateAfter, err := os.ReadFile(c.StatePath)
		if err != nil || !bytes.Equal(stateBefore, stateAfter) {
			t.Fatal("refused action changed the saved state or transaction", err)
		}
	}
}

func TestProfileRestorationJournalValidatesSavedPathsAndLegacyShape(t *testing.T) {
	_, _, j, _ := interruptedProfileController(t, "update", "published")
	encode := func(j profileJournal) []byte {
		t.Helper()
		data, err := json.Marshal(j)
		if err != nil {
			t.Fatal(err)
		}
		return data
	}
	legacy := encode(j)
	if bytes.Contains(legacy, []byte(`"restoration"`)) {
		t.Fatal("fixture unexpectedly contains the new optional field")
	}
	if _, err := decodeProfileJournal(legacy, j.Operation); err != nil {
		t.Fatal("journal without optional restoration entries was rejected", err)
	}
	e := j.Entries[0]
	e.Workspace = filepath.Join(e.Workspace, "restore")
	j.Restoring, j.Restoration = true, []profilePublication{e}
	if _, err := decodeProfileJournal(encode(j), j.Operation); err != nil {
		t.Fatal("valid scoped restoration rejected", err)
	}
	for _, change := range []struct {
		name string
		edit func(*profileJournal)
	}{
		{"different-target", func(j *profileJournal) { j.Restoration[0].Path += "-unapproved" }},
		{"different-workspace", func(j *profileJournal) { j.Restoration[0].Workspace += "-unapproved" }},
		{"duplicate-target", func(j *profileJournal) { j.Restoration = append(j.Restoration, j.Restoration[0]) }},
		{"not-restoring", func(j *profileJournal) { j.Restoring = false }},
		{"incomplete-sealed-restoration", func(j *profileJournal) { j.Complete, j.Sealed = true, true }},
	} {
		t.Run(change.name, func(t *testing.T) {
			var changed profileJournal
			if err := Decode(encode(j), &changed); err != nil {
				t.Fatal(err)
			}
			change.edit(&changed)
			if _, err := decodeProfileJournal(encode(changed), j.Operation); err == nil {
				t.Fatal("invalid scoped restoration was accepted")
			}
		})
	}
}
