package installer

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProfileMissingOriginalPreservesTheActivePublication(t *testing.T) {
	for _, action := range []string{"install", "update", "remove"} {
		for _, cut := range []string{"moved", "published"} {
			t.Run(action+"/"+cut, func(t *testing.T) {
				c, _, j, _ := interruptedProfileController(t, action, cut)
				e := j.Entries[0]
				before := inspectConfigRecovery(e.Path)
				if before.Problem != "" {
					t.Fatal(before.Problem)
				}
				if err := os.Remove(filepath.Join(e.Workspace, "previous")); err != nil {
					t.Fatal(err)
				}
				request := Request{Schema: 1, Mode: "restore"}
				plan, err := c.Preview(context.Background(), request)
				if err != nil {
					t.Fatal(err)
				}
				request.ExpectedPlan = plan.ID
				result, err := c.Dispatch(context.Background(), request)
				if after := inspectConfigRecovery(e.Path); after != before {
					t.Fatal("missing original displaced the active publication", before, after)
				}
				if err != nil || result.Status != "needs-action" || !strings.Contains(result.Message, e.Path) {
					t.Fatal("missing original was reported as restored or without a recovery path", result, err)
				}
				state, err := LoadState(c.StatePath, c.Home)
				if err != nil || state.Transaction != nil {
					t.Fatal("missing original stranded the recovery transaction", err)
				}
			})
		}
	}
}

func TestRemovedNewProfileMissingOriginalNamesTheActivePath(t *testing.T) {
	for _, recreated := range []bool{false, true} {
		t.Run(map[bool]string{false: "absent", true: "recreated"}[recreated], func(t *testing.T) {
			c, d, path := profileController(t)
			dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"test"}})
			c.Driver = interruptedProfileDriver{d}
			request := Request{Schema: 1, Mode: "apply", Selected: []string{}}
			plan, err := c.Preview(context.Background(), request)
			if err != nil {
				t.Fatal(err)
			}
			request.ExpectedPlan = plan.ID
			if _, err := c.Dispatch(context.Background(), request); err == nil || !strings.Contains(err.Error(), "profile fixture stopped") {
				t.Fatal("removal fixture did not interrupt", err)
			}
			c.Driver = d
			state, err := LoadState(c.StatePath, c.Home)
			if err != nil {
				t.Fatal(err)
			}
			j, err := d.readJournal("profile.test", state.Receipts["profile.test"])
			if err != nil || j.Entries[0].After.Kind != "absent" {
				t.Fatal("fixture did not remove an installer-created profile", j, err)
			}
			j.Complete = false
			if err := saveDocument(d.journalPath(j.Operation), j); err != nil {
				t.Fatal(err)
			}
			if err := os.Remove(filepath.Join(j.Entries[0].Workspace, "previous")); err != nil {
				t.Fatal(err)
			}
			data := []byte("# personal profile recreated after removal\n")
			if recreated {
				writeConfigFixture(t, path, string(data))
			}
			request = Request{Schema: 1, Mode: "restore"}
			plan, err = c.Preview(context.Background(), request)
			if err != nil {
				t.Fatal(err)
			}
			request.ExpectedPlan = plan.ID
			result, err := c.Dispatch(context.Background(), request)
			if err != nil || result.Status != "needs-action" || !strings.Contains(result.Message, path) {
				t.Fatal("missing original after removal omitted its target path", result, err)
			}
			actual, err := os.ReadFile(path)
			if recreated && (err != nil || !bytes.Equal(actual, data)) || !recreated && !os.IsNotExist(err) {
				t.Fatal("recovery changed the user's active profile", string(actual), err)
			}
		})
	}
}

func TestConfigurationMissingOriginalPreservesTheActivePublication(t *testing.T) {
	for _, action := range []string{"adopt", "update", "remove"} {
		t.Run(action, func(t *testing.T) {
			c, _, _, j := interruptedConfigController(t, action, "after-publication")
			e := j.Entries[0]
			path := e.State.Target.Destination
			before := inspectConfigRecovery(path)
			if before.Problem != "" {
				t.Fatal(before.Problem)
			}
			if err := os.Remove(filepath.Join(e.Workspace, "previous")); err != nil {
				t.Fatal(err)
			}
			result := restoreConfigController(t, c)
			if after := inspectConfigRecovery(path); after != before {
				t.Fatal("missing original displaced the active configuration", before, after)
			}
			if result.Status != "needs-action" || !strings.Contains(result.Message, path) {
				t.Fatal("missing configuration original lacks an actionable result", result)
			}
		})
	}
}
