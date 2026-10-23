package installer

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type interruptedProfileDriver struct{ *ProfileDriver }

func (d interruptedProfileDriver) Apply(ctx context.Context, r Resource, op Operation, receipt Receipt) (Observation, error) {
	o, err := d.ProfileDriver.Apply(ctx, r, op, receipt)
	return o, errors.Join(err, errors.New("profile fixture stopped before engine completion"))
}

func (d interruptedProfileDriver) Remove(ctx context.Context, r Resource, receipt Receipt) (Observation, error) {
	o, err := d.ProfileDriver.Remove(ctx, r, receipt)
	return o, errors.Join(err, errors.New("profile fixture stopped before engine completion"))
}

func interruptedProfileController(t *testing.T, action, cut string) (Controller, *ProfileDriver, profileJournal, []byte) {
	t.Helper()
	c, d, path := profileController(t)
	writeConfigFixture(t, path, "# original personal settings")
	request := Request{Schema: 1, Mode: "apply", Selected: []string{"test"}}
	if action != "install" {
		dispatchApproved(t, c, request)
		request = Request{Schema: 1, Mode: "update"}
		d.Targets["profile.test"][0].Script = "# updated integration"
		if action == "remove" {
			request = Request{Schema: 1, Mode: "apply", Selected: []string{}}
		}
	}
	original, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	c.Driver = interruptedProfileDriver{d}
	preview, err := c.Preview(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	request.ExpectedPlan = preview.ID
	if _, err := c.Dispatch(context.Background(), request); err == nil || !strings.Contains(err.Error(), "profile fixture stopped") {
		t.Fatal("fixture did not interrupt after real profile publication", err)
	}
	c.Driver = d
	state, err := LoadState(c.StatePath, c.Home)
	if err != nil {
		t.Fatal(err)
	}
	j, err := d.readJournal("profile.test", state.Receipts["profile.test"])
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
	if cut == "staged" {
		if err := moveConfigExclusive(filepath.Join(e.Workspace, "previous"), path); err != nil {
			t.Fatal(err)
		}
		e.Moved = false
	}
	if err := saveDocument(d.journalPath(j.Operation), j); err != nil {
		t.Fatal(err)
	}
	return c, d, j, original
}

func TestProfileRestorePreservesConflictsWithoutSourceOrCurrentTargets(t *testing.T) {
	for _, action := range []string{"install", "update", "remove"} {
		for _, cut := range []string{"staged", "moved", "published"} {
			t.Run(action+"/"+cut, func(t *testing.T) {
				c, d, j, original := interruptedProfileController(t, action, cut)
				catalog, targets := c.Catalog, d.Targets
				e := j.Entries[0]
				conflict := []byte("# recreated or edited by the user")
				writeConfigFixture(t, e.Path, string(conflict))
				c.Source = "new-installer-without-the-original-source"
				d.Targets = nil
				c.Catalog = &Catalog{Schema: 1}
				if err := c.Catalog.Validate(); err != nil {
					t.Fatal(err)
				}
				c.Driver = &NativeDriver{Catalog: c.Catalog, Profiles: d, StatePath: c.StatePath}
				request := Request{Schema: 1, Mode: "restore"}
				preview, err := c.Preview(context.Background(), request)
				if err != nil {
					t.Fatal("profile restoration preview failed", err)
				}
				request.ExpectedPlan = preview.ID
				result, err := c.Dispatch(context.Background(), request)
				if err != nil || result.Status != "ready" && result.Status != "needs-action" {
					t.Fatal("profile restoration failed", result, err)
				}
				actual, err := os.ReadFile(e.Path)
				want := conflict
				if cut == "moved" {
					want = original
				}
				if err != nil || !bytes.Equal(actual, want) {
					t.Fatalf("restoration replaced the user's active profile: %q %v", actual, err)
				}
				if cut != "staged" {
					saved := filepath.Join(e.Workspace, "previous")
					want := original
					if cut == "moved" {
						saved, want = filepath.Join(e.Workspace, "displaced"), conflict
					}
					data, err := os.ReadFile(saved)
					if err != nil || !bytes.Equal(data, want) || !strings.Contains(result.Message, saved) {
						t.Fatal("preserved profile was lost or not disclosed", string(data), result, err)
					}
				}
				state, err := LoadState(c.StatePath, c.Home)
				if err != nil || state.Transaction != nil || state.Receipts["profile.test"].Status == "in-progress" {
					t.Fatal("restoration stranded the transaction", state, err)
				}
				c.Catalog, d.Targets, c.Driver = catalog, targets, d
				dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"test"}})
			})
		}
	}
}

func TestProfileRestoreRejectsChangesAfterApproval(t *testing.T) {
	for _, name := range []string{"target", "previous", "next", "displaced"} {
		t.Run(name, func(t *testing.T) {
			c, _, j, _ := interruptedProfileController(t, "update", "moved")
			e := j.Entries[0]
			request := Request{Schema: 1, Mode: "restore"}
			preview, err := c.Preview(context.Background(), request)
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(e.Workspace, name)
			if name == "target" {
				path = e.Path
			}
			writeConfigFixture(t, path, "# changed after approval")
			before, err := digest(profileRecoveryFiles(j))
			if err != nil {
				t.Fatal(err)
			}
			request.ExpectedPlan = preview.ID
			if _, err := c.Dispatch(context.Background(), request); err == nil {
				t.Fatal("stale profile restoration approval mutated files")
			}
			after, err := digest(profileRecoveryFiles(j))
			if err != nil || before != after {
				t.Fatal("rejected restoration changed saved files", err)
			}
		})
	}
}

type interruptedProfileRestoration struct{ *ProfileDriver }

func (d interruptedProfileRestoration) RestoreResource(ctx context.Context, id string, observed Observation, receipt Receipt) error {
	err := d.ProfileDriver.RestoreResource(ctx, id, observed, receipt)
	return errors.Join(err, errors.New("fixture stopped after restoration before engine completion"))
}

func TestProfileRestorationCanFinishAfterItsProviderCompleted(t *testing.T) {
	c, d, j, original := interruptedProfileController(t, "update", "moved")
	c.Driver = interruptedProfileRestoration{d}
	request := Request{Schema: 1, Mode: "restore"}
	preview, err := c.Preview(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	request.ExpectedPlan = preview.ID
	if _, err := c.Dispatch(context.Background(), request); err == nil || !strings.Contains(err.Error(), "fixture stopped after restoration") {
		t.Fatal("fixture did not interrupt after real restoration", err)
	}
	c.Driver = d
	dispatchApproved(t, c, Request{Schema: 1, Mode: "restore"})
	data, err := os.ReadFile(j.Entries[0].Path)
	if err != nil || !bytes.Equal(data, original) {
		t.Fatal("repeated restoration changed the recovered profile", err)
	}
}
