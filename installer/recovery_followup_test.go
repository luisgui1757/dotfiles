package installer

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProfileResumePreservesTornStagingWithoutParsingIt(t *testing.T) {
	for name, torn := range map[string][]byte{"utf8": {0xe2, 0x82}, "utf16": {0xff, 0xfe, 0x61}, "nul": {0}} {
		t.Run(name, func(t *testing.T) {
			c, d, j, original := interruptedProfileController(t, "install", "staged")
			e := &j.Entries[0]
			next := filepath.Join(e.Workspace, "next")
			var err error
			e.Content, err = os.ReadFile(next)
			if err != nil {
				t.Fatal(err)
			}
			e.Staged = false
			if err := os.WriteFile(next, torn, 0600); err != nil {
				t.Fatal(err)
			}
			if err := saveDocument(d.journalPath(j.Operation), j); err != nil {
				t.Fatal(err)
			}
			request := Request{Schema: 1, Mode: "apply", Retry: true}
			plan, err := c.Preview(context.Background(), request)
			if err != nil || plan.ResourceResume == nil {
				t.Fatal("no recovery preview", plan, err)
			}
			request.ExpectedPlan = plan.ID
			if _, err := c.Dispatch(context.Background(), request); err != nil {
				t.Fatal("torn staging blocked resume", err)
			}
			dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Retry: true})
			dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{}})
			data, err := os.ReadFile(e.Path)
			if err != nil || !bytes.Equal(data, original) {
				t.Fatal("resume changed the original profile", err)
			}
			files, err := os.ReadDir(e.Workspace)
			if err != nil {
				t.Fatal(err)
			}
			found := false
			for _, file := range files {
				if strings.HasPrefix(file.Name(), "unfinished-") {
					data, err := os.ReadFile(filepath.Join(e.Workspace, file.Name()))
					if err != nil || !bytes.Equal(data, torn) {
						t.Fatal("torn bytes were changed", err)
					}
					found = true
				}
			}
			if !found {
				t.Fatal("torn bytes were not preserved")
			}
		})
	}
}

func TestRestoreRetainsUninspectableRecoveryForRetry(t *testing.T) {
	for _, kind := range []string{"configuration", "profile"} {
		t.Run(kind, func(t *testing.T) {
			var c Controller
			var workspace string
			if kind == "configuration" {
				var j configJournal
				c, _, _, j = interruptedConfigController(t, "update", "after-backup")
				workspace = j.Entries[0].Workspace
			} else {
				var j profileJournal
				c, _, j, _ = interruptedProfileController(t, "update", "moved")
				workspace = j.Entries[0].Workspace
			}
			saved := workspace + "-temporarily-moved"
			if err := moveConfigExclusive(workspace, saved); err != nil {
				t.Fatal(err)
			}
			if err := createDirectoryLink(saved, workspace); err != nil {
				t.Fatal(err)
			}
			request := Request{Schema: 1, Mode: "restore"}
			plan, err := c.Preview(context.Background(), request)
			if err != nil {
				t.Fatal(err)
			}
			request.ExpectedPlan = plan.ID
			if _, err := c.Dispatch(context.Background(), request); err == nil {
				t.Fatal("restoration was sealed despite uninspectable recovery files")
			}
			state, err := LoadState(c.StatePath, c.Home)
			if err != nil || state.Transaction == nil || len(state.PastTransactions) != 0 {
				t.Fatal("uninspectable recovery was discarded", state, err)
			}
			if err := os.Remove(workspace); err != nil {
				t.Fatal(err)
			}
			if err := moveConfigExclusive(saved, workspace); err != nil {
				t.Fatal(err)
			}
			dispatchApproved(t, c, Request{Schema: 1, Mode: "restore"})
		})
	}
}

func TestProfileRestoreKeepsLaterOutsideEditsInTheActiveProfile(t *testing.T) {
	for _, action := range []string{"install", "update", "remove", "new-file"} {
		t.Run(action, func(t *testing.T) {
			var c Controller
			var d *ProfileDriver
			var j profileJournal
			var original []byte
			if action == "new-file" {
				var path string
				c, d, path = profileController(t)
				c.Driver = interruptedProfileDriver{d}
				request := Request{Schema: 1, Mode: "apply", Selected: []string{"test"}}
				plan, err := c.Preview(context.Background(), request)
				if err != nil {
					t.Fatal(err)
				}
				request.ExpectedPlan = plan.ID
				if _, err := c.Dispatch(context.Background(), request); err == nil {
					t.Fatal("fixture did not interrupt")
				}
				c.Driver = d
				state, err := LoadState(c.StatePath, c.Home)
				if err != nil {
					t.Fatal(err)
				}
				j, err = d.readJournal("profile.test", state.Receipts["profile.test"])
				if err != nil {
					t.Fatal(err)
				}
				j.Complete, j.Entries[0].Published = false, false
				if err := saveDocument(d.journalPath(j.Operation), j); err != nil {
					t.Fatal(err)
				}
				data, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				original, _, err = replaceProfileBlock(data, "profile.test", nil)
				if err != nil {
					t.Fatal(err)
				}
			} else {
				c, d, j, original = interruptedProfileController(t, action, "published")
			}
			path := j.Entries[0].Path
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			edit := []byte("\n# later personal change\n")
			if err := os.WriteFile(path, append(data, edit...), 0600); err != nil {
				t.Fatal(err)
			}
			request := Request{Schema: 1, Mode: "restore"}
			plan, err := c.Preview(context.Background(), request)
			if err != nil {
				t.Fatal(err)
			}
			request.ExpectedPlan = plan.ID
			if _, err := c.Dispatch(context.Background(), request); err != nil {
				t.Fatal(err)
			}
			want := append(bytes.Clone(original), edit...)
			actual, err := os.ReadFile(path)
			if err != nil || !bytes.Equal(actual, want) {
				t.Fatalf("restoration changed active personal bytes: got %q, want %q (%v)", actual, want, err)
			}
			dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"test"}})
			dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{}})
			actual, err = os.ReadFile(path)
			if err != nil || !bytes.Contains(actual, edit) {
				t.Fatal("later lifecycle removed personal edits", string(actual), err)
			}
		})
	}
}

func TestDamagedProfileJournalDisclosesItsRecoveryLocations(t *testing.T) {
	c, d, j, _ := interruptedProfileController(t, "update", "moved")
	path := d.journalPath(j.Operation)
	writeConfigFixture(t, path, "{broken active journal")
	_, err := c.Preview(context.Background(), Request{Schema: 1, Mode: "restore"})
	if err == nil || !strings.Contains(err.Error(), path) || !strings.Contains(err.Error(), j.Entries[0].Workspace) {
		t.Fatal("damaged active evidence did not disclose journal and original-file locations", err)
	}
}
