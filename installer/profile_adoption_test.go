package installer

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestProfileMaintenancePreservesEditedBlocksAndTheFirstBaseline(t *testing.T) {
	for _, mode := range []string{"update", "repair"} {
		t.Run(mode, func(t *testing.T) {
			c, d, path := profileController(t)
			second := filepath.Join(c.Home, "second profile")
			d.Targets["profile.test"] = append(d.Targets["profile.test"], ProfileTarget{second, "# first version"})
			writeConfigFixture(t, path, "# personal settings\n")
			writeConfigFixture(t, second, "# second profile\n")
			dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"test"}})
			original, err := LoadState(c.StatePath, c.Home)
			if err != nil {
				t.Fatal(err)
			}
			baselinePath := d.baselinePath(original.Receipts["profile.test"].Recovery)
			baseline, err := os.ReadFile(baselinePath)
			if err != nil {
				t.Fatal(err)
			}
			preserved := map[string][]byte{}
			for _, edit := range []string{"# first personal block", "# second personal block"} {
				data, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				block, err := renderProfileBlock(data, "profile.test", edit)
				if err != nil {
					t.Fatal(err)
				}
				edited, _, err := replaceProfileBlock(data, "profile.test", block)
				if err != nil {
					t.Fatal(err)
				}
				writeConfigFixture(t, path, string(edited))
				// A partially missing integration has the same approval contract
				// as one whose marked block was edited.
				writeConfigFixture(t, second, "# recreated second profile\n")
				ui := &scriptedInteraction{t: t, answers: [][]string{{mode}, {}}, accept: true}
				result, err := c.Run(context.Background(), ui)
				if err != nil || result.Status != "needs-action" || len(ui.menus) != 2 {
					t.Fatal("maintenance did not offer a separately approved replacement", result, ui.menus, err)
				}
				actual, err := os.ReadFile(path)
				if err != nil || !bytes.Equal(actual, edited) {
					t.Fatal("declining replacement changed the edited block", err)
				}
				// The declined resource leaves an unfinished selection. Archive
				// that intent through the menu before making a different choice.
				runController(t, c, []string{"abandon"})
				result = runController(t, c, []string{mode}, []string{"profile.test"})
				state, err := LoadState(c.StatePath, c.Home)
				before, after := original.Receipts["profile.test"], state.Receipts["profile.test"]
				if err != nil || before.Recovery != after.Recovery || before.Adopted != after.Adopted || !sameArtifact(before.Before, after.Before) {
					t.Fatal("replacement reset the first uninstall baseline", before, after, err)
				}
				actualBaseline, err := os.ReadFile(baselinePath)
				if err != nil || !bytes.Equal(actualBaseline, baseline) {
					t.Fatal("replacement rewrote the original baseline", err)
				}
				j, err := d.readJournal("profile.test", after)
				if err != nil {
					t.Fatal(err)
				}
				for i, e := range j.Entries {
					saved := filepath.Join(e.Workspace, "previous")
					want := edited
					if i == 1 {
						want = []byte("# recreated second profile\n")
					}
					actual, err := os.ReadFile(saved)
					if err != nil || !bytes.Equal(actual, want) || !strings.Contains(result.Message, saved) {
						t.Fatal("replacement lost or did not disclose saved profile edits", saved, string(actual), result, err)
					}
					preserved[saved] = bytes.Clone(want)
				}
			}
			d.Targets["profile.test"][0].Script = "# next release"
			dispatchApproved(t, c, Request{Schema: 1, Mode: "update"})
			dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{}})
			for path, want := range map[string]string{path: "# personal settings\n", second: "# recreated second profile\n"} {
				actual, err := os.ReadFile(path)
				if err != nil || string(actual) != want {
					t.Fatal("removal did not preserve outside bytes and remove the owned block", path, string(actual), err)
				}
			}
			dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"test"}})
			state, err := LoadState(c.StatePath, c.Home)
			if err != nil {
				t.Fatal(err)
			}
			for path, want := range preserved {
				actual, err := os.ReadFile(path)
				if err != nil || !bytes.Equal(actual, want) || !slices.Contains(preservedPaths(state), path) {
					t.Fatal("later lifecycle lost a preserved profile or its reference", path, string(actual), err)
				}
			}
		})
	}
}

func TestProfileReplacementRequiresAnOwnedIntactBaseline(t *testing.T) {
	for _, damage := range []string{"unowned", "missing", "wrong-target", "malformed-block", "changed-block"} {
		t.Run(damage, func(t *testing.T) {
			c, d, path := profileController(t)
			baselinePath := ""
			if damage != "unowned" {
				writeConfigFixture(t, path, "# original personal profile\n")
				dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"test"}})
				state, err := LoadState(c.StatePath, c.Home)
				if err != nil {
					t.Fatal(err)
				}
				baselinePath = d.baselinePath(state.Receipts["profile.test"].Recovery)
				if damage == "missing" {
					if err := os.Remove(baselinePath); err != nil {
						t.Fatal(err)
					}
				} else {
					data, err := os.ReadFile(baselinePath)
					var baseline profileBaseline
					if err != nil || Decode(data, &baseline) != nil {
						t.Fatal("cannot read fixture baseline", err)
					}
					if damage == "wrong-target" {
						baseline.Entries[0].Path += ".elsewhere"
					} else if damage == "malformed-block" {
						baseline.Entries[0].Block = []byte("# unterminated baseline block")
					} else {
						baseline.Entries[0].Block, err = renderProfileBlock(nil, "profile.test", "# different original integration")
						if err != nil {
							t.Fatal(err)
						}
					}
					if err := saveDocument(baselinePath, baseline); err != nil {
						t.Fatal(err)
					}
				}
			}
			block, err := renderProfileBlock(nil, "profile.test", "# user integration")
			if err != nil {
				t.Fatal(err)
			}
			writeConfigFixture(t, path, string(block))
			request := Request{Schema: 1, Mode: "apply", Selected: []string{"test"}, Adopt: []string{"profile.test"}}
			if _, err := c.Preview(context.Background(), request); err == nil {
				t.Fatal("offered replacement without a restorable owned baseline", damage)
			}
			if baselinePath != "" {
				check, err := c.Dispatch(context.Background(), Request{Schema: 1, Mode: "check"})
				if err != nil || check.Status != "needs-action" || !strings.Contains(check.Message, baselinePath) {
					t.Fatal("damaged baseline check did not name the evidence to inspect", check, err)
				}
			}
			actual, err := os.ReadFile(path)
			if err != nil || !bytes.Equal(actual, block) {
				t.Fatal("refusal changed the profile", err)
			}
		})
	}
}
