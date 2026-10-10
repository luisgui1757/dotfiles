package installer

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The boundary returns an error only after the actual filesystem provider has
// published. Saved-shape cases below move the same real files back to each
// interruption boundary; no planner or publication function is mocked.
type profileUnrecordedPublication struct{ Driver }

func (d profileUnrecordedPublication) Apply(ctx context.Context, r Resource, op Operation, receipt Receipt) (Observation, error) {
	o, err := d.Driver.Apply(ctx, r, op, receipt)
	if err != nil {
		return o, err
	}
	return o, errors.New("fixture stopped before engine completion")
}

func TestProfileRecoveryPreservesOriginalBytesAcrossPublicationBoundaries(t *testing.T) {
	for _, cut := range []string{"partial-stage", "partial-metadata", "staged", "moved", "published"} {
		t.Run(cut, func(t *testing.T) {
			c, d, path := profileController(t)
			writeConfigFixture(t, path, "# original without final newline")
			c.Driver = profileUnrecordedPublication{d}
			request := Request{Schema: 1, Mode: "apply", Selected: []string{"test"}}
			plan, err := c.Preview(context.Background(), request)
			if err != nil {
				t.Fatal(err)
			}
			request.ExpectedPlan = plan.ID
			if _, err := c.Dispatch(context.Background(), request); err == nil {
				t.Fatal("fixture did not interrupt")
			}
			state, err := LoadState(c.StatePath, c.Home)
			if err != nil {
				t.Fatal(err)
			}
			receipt := state.Receipts["profile.test"]
			j, err := d.readJournal("profile.test", receipt)
			if err != nil {
				t.Fatal(err)
			}
			j.Complete = false
			e := &j.Entries[0]
			e.Published = false
			if cut != "published" {
				if err := moveConfigExclusive(path, filepath.Join(e.Workspace, "next")); err != nil {
					t.Fatal(err)
				}
			}
			if cut == "partial-stage" || cut == "partial-metadata" || cut == "staged" {
				if err := moveConfigExclusive(filepath.Join(e.Workspace, "previous"), path); err != nil {
					t.Fatal(err)
				}
				e.Moved = false
			}
			if cut == "partial-stage" || cut == "partial-metadata" {
				e.Content, err = os.ReadFile(filepath.Join(e.Workspace, "next"))
				if err != nil {
					t.Fatal(err)
				}
				e.Staged = false
				if cut == "partial-stage" {
					if err := os.WriteFile(filepath.Join(e.Workspace, "next"), []byte("interrupted bytes"), 0600); err != nil {
						t.Fatal(err)
					}
				} else if err := os.Chmod(filepath.Join(e.Workspace, "next"), 0400); err != nil {
					t.Fatal(err)
				}
			}
			if err := saveDocument(d.journalPath(j.Operation), j); err != nil {
				t.Fatal(err)
			}
			c.Driver = d
			request = Request{Schema: 1, Mode: "apply", Retry: true}
			plan, err = c.Preview(context.Background(), request)
			if err != nil {
				t.Fatal(err)
			}
			if plan.ResourceResume == nil {
				t.Fatal("no scoped resume", plan)
			}
			request.ExpectedPlan = plan.ID
			result, err := c.Dispatch(context.Background(), request)
			if err != nil || result.Status != "needs-action" {
				t.Fatal(result, err)
			}
			dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Retry: true})
			dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{}})
			data, err := os.ReadFile(path)
			if err != nil || string(data) != "# original without final newline" {
				t.Fatalf("recovery changed original bytes: %q %v", data, err)
			}
			if err := verifyConfigSnapshot(path, e.Before); err != nil {
				t.Fatal("recovery lost original profile permissions", err)
			}
			if cut == "partial-stage" {
				files, err := os.ReadDir(e.Workspace)
				if err != nil {
					t.Fatal(err)
				}
				found := false
				for _, file := range files {
					if strings.HasPrefix(file.Name(), "unfinished-") {
						data, err := os.ReadFile(filepath.Join(e.Workspace, file.Name()))
						if err != nil || string(data) != "interrupted bytes" {
							t.Fatal("lost interrupted bytes", err)
						}
						found = true
					}
				}
				if !found {
					t.Fatal("incomplete staged bytes were not retained")
				}
			}
		})
	}
}

func TestProfileRefusesOwnershipOfMatchingFilePublishedByAnotherWriter(t *testing.T) {
	c, d, path := profileController(t)
	writeConfigFixture(t, path, "# original")
	c.Driver = profileUnrecordedPublication{d}
	request := Request{Schema: 1, Mode: "apply", Selected: []string{"test"}}
	plan, err := c.Preview(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	request.ExpectedPlan = plan.ID
	if _, err := c.Dispatch(context.Background(), request); err == nil {
		t.Fatal("fixture did not interrupt")
	}
	state, err := LoadState(c.StatePath, c.Home)
	if err != nil {
		t.Fatal(err)
	}
	j, err := d.readJournal("profile.test", state.Receipts["profile.test"])
	if err != nil {
		t.Fatal(err)
	}
	j.Complete = false
	j.Entries[0].Published = false
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(j.Entries[0].Workspace, "next"), data, 0600); err != nil {
		t.Fatal(err)
	}
	if err := saveDocument(d.journalPath(j.Operation), j); err != nil {
		t.Fatal(err)
	}
	c.Driver = d
	request = Request{Schema: 1, Mode: "apply", Retry: true}
	plan, err = c.Preview(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	request.ExpectedPlan = plan.ID
	if _, err := c.Dispatch(context.Background(), request); err == nil || !strings.Contains(err.Error(), "another profile appeared") {
		t.Fatal("claimed another writer's matching file", err)
	}
}
