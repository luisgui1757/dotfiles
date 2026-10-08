package installer

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

type profileCrashFixture struct {
	Home, StatePath, Source string
	Catalog                 Catalog
	Target                  Context
	Driver                  ProfileDriver
	Request                 Request
}

func TestProfileCrashChild(t *testing.T) {
	path := os.Getenv("DOTFILES_PROFILE_CRASH_FIXTURE")
	if path == "" {
		return
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var f profileCrashFixture
	if err := json.Unmarshal(data, &f); err != nil {
		t.Fatal(err)
	}
	c := Controller{Catalog: &f.Catalog, Context: f.Target, Source: f.Source, Home: f.Home, StatePath: f.StatePath, Driver: &f.Driver}
	dispatchApproved(t, c, f.Request)
	t.Fatal("parent did not interrupt the profile publication")
}

func TestProfileRecoversAfterProcessDeathDuringPublication(t *testing.T) {
	for _, action := range []string{"install", "update", "adopt", "remove", "remove-after-adopt"} {
		for _, recovery := range []string{"resume", "restore"} {
			for _, firstPublished := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/%s/first-published=%t", action, recovery, firstPublished), func(t *testing.T) {
					c, d, _ := profileController(t)
					preserved := map[string][]byte{}
					d.Targets["profile.test"] = nil
					for i := range 8 {
						path := filepath.Join(c.Home, fmt.Sprintf("profile-%d", i))
						writeConfigFixture(t, path, "# personal settings\n")
						d.Targets["profile.test"] = append(d.Targets["profile.test"], ProfileTarget{path, "# original integration"})
					}
					request := Request{Schema: 1, Mode: "apply", Selected: []string{"test"}}
					if action != "install" {
						dispatchApproved(t, c, request)
						request = Request{Schema: 1, Mode: "update"}
						for i := range d.Targets["profile.test"] {
							d.Targets["profile.test"][i].Script = "# updated integration"
						}
						if action == "remove" {
							request = Request{Schema: 1, Mode: "apply", Selected: []string{}}
						} else if action == "adopt" || action == "remove-after-adopt" {
							for _, target := range d.Targets["profile.test"] {
								data, err := os.ReadFile(target.Path)
								if err != nil {
									t.Fatal(err)
								}
								writeConfigFixture(t, target.Path, string(bytes.ReplaceAll(data, []byte("# original integration"), []byte("# personal integration"))))
							}
							request = Request{Schema: 1, Mode: "update", Adopt: []string{"profile.test"}}
							if action == "remove-after-adopt" {
								dispatchApproved(t, c, request)
								state, err := LoadState(c.StatePath, c.Home)
								if err != nil {
									t.Fatal(err)
								}
								j, err := d.readJournal("profile.test", state.Receipts["profile.test"])
								if err != nil {
									t.Fatal(err)
								}
								for _, e := range j.Entries {
									path := filepath.Join(e.Workspace, "previous")
									preserved[path], err = os.ReadFile(path)
									if err != nil {
										t.Fatal(err)
									}
								}
								request = Request{Schema: 1, Mode: "apply", Selected: []string{}}
							}
						}
					}
					original := map[string][]byte{}
					for _, target := range d.Targets["profile.test"] {
						data, err := os.ReadFile(target.Path)
						if err != nil {
							t.Fatal(err)
						}
						original[target.Path] = data
					}
					watchedIndex := 0
					if firstPublished {
						watchedIndex = 1
					}
					killProfileChildAfterMove(t, c, d, request, "previous", watchedIndex)
					state, err := LoadState(c.StatePath, c.Home)
					if err != nil || state.Transaction == nil {
						t.Fatal("kill missed the active transaction", err)
					}
					j, err := d.readJournal("profile.test", state.Receipts["profile.test"])
					if err != nil || j.Complete || j.Entries[len(j.Entries)-1].Published {
						t.Fatal("kill did not interrupt actual publication", j, err)
					}
					if firstPublished && !j.Entries[0].Published {
						t.Fatal("second-profile cut did not prove publication of the first profile")
					}
					request = Request{Schema: 1, Mode: request.Mode, Retry: true}
					var recreated []byte
					unpublished := false
					if recovery == "restore" {
						next, err := snapshotTree(filepath.Join(j.Entries[0].Workspace, "next"), maxProfileBytes, 1)
						if err != nil {
							t.Fatal(err)
						}
						unpublished = !j.Entries[0].Published && next.Kind != "absent"
						t.Logf("first profile unpublished at actual process death: %t", unpublished)
						recreated = []byte("# recreated by the user\n")
						writeConfigFixture(t, j.Entries[0].Path, string(recreated))
						request = Request{Schema: 1, Mode: "restore"}
					}
					preview, err := c.Preview(context.Background(), request)
					if err != nil {
						t.Fatal(err)
					}
					request.ExpectedPlan = preview.ID
					result, err := c.Dispatch(context.Background(), request)
					if err != nil {
						t.Fatal(err)
					}
					request.ExpectedPlan = ""
					if recovery == "resume" && result.Status == "needs-action" {
						result = dispatchApproved(t, c, request)
					}
					if result.Status != "ready" && !(recovery == "restore" && (!unpublished || action == "adopt") && result.Status == "needs-action") {
						t.Fatal("recovery did not converge", result)
					}
					if action == "adopt" && recovery == "resume" {
						for _, e := range j.Entries {
							path := filepath.Join(e.Workspace, "previous")
							data, err := os.ReadFile(path)
							if err != nil || !bytes.Equal(data, original[e.Path]) || !strings.Contains(result.Message, path) {
								t.Fatal("resumed replacement lost or did not disclose original edits", path, string(data), result, err)
							}
							preserved[path] = bytes.Clone(original[e.Path])
						}
					}
					if recovery == "restore" {
						for path, want := range original {
							if path == j.Entries[0].Path && !unpublished {
								want = recreated
							}
							data, err := os.ReadFile(path)
							if err != nil || !bytes.Equal(data, want) {
								t.Fatalf("original profile not restored: %s %q %v", path, data, err)
							}
						}
						saved := filepath.Join(j.Entries[0].Workspace, "previous")
						want := original[j.Entries[0].Path]
						if unpublished {
							saved, want = filepath.Join(j.Entries[0].Workspace, "displaced"), recreated
						}
						data, err := os.ReadFile(saved)
						if err != nil || !bytes.Equal(data, want) || !strings.Contains(result.Message, saved) {
							t.Fatal("preserved profile was lost or not disclosed", string(data), err, result)
						}
					}
					state, err = LoadState(c.StatePath, c.Home)
					if err != nil || state.Transaction != nil {
						t.Fatal("recovery stranded the transaction", err)
					}
					reapply := Request{Schema: 1, Mode: "apply", Selected: []string{"test"}}
					if recovery == "restore" && (action == "adopt" || !unpublished && action != "install") {
						// Restoration preserved the recreated first profile. Replacing
						// this changed composite resource needs a new explicit choice.
						reapply.Adopt = []string{"profile.test"}
					}
					dispatchApproved(t, c, reapply)
					dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{}})
					state, err = LoadState(c.StatePath, c.Home)
					if err != nil {
						t.Fatal(err)
					}
					for path, want := range preserved {
						data, err := os.ReadFile(path)
						if err != nil || !bytes.Equal(data, want) || !slices.Contains(preservedPaths(state), path) {
							t.Fatal("later removal lost saved profile edits or their reference", path, string(data), err)
						}
					}
				})
			}
		}
	}
}

func TestScopedProfileRestorationRecoversAfterProcessDeath(t *testing.T) {
	c, d, _ := profileController(t)
	d.Targets["profile.test"] = nil
	for i := range 8 {
		path := filepath.Join(c.Home, fmt.Sprintf("profile-%d", i))
		writeConfigFixture(t, path, "# personal settings\n")
		d.Targets["profile.test"] = append(d.Targets["profile.test"], ProfileTarget{path, "# original integration"})
	}
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"test"}})
	originals := map[string][]byte{}
	for i, target := range d.Targets["profile.test"] {
		data, err := os.ReadFile(target.Path)
		if err != nil {
			t.Fatal(err)
		}
		originals[target.Path] = data
		d.Targets["profile.test"][i].Script = "# updated integration"
	}
	c.Driver = interruptedProfileDriver{d}
	request := Request{Schema: 1, Mode: "update"}
	plan, err := c.Preview(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	request.ExpectedPlan = plan.ID
	if _, err := c.Dispatch(context.Background(), request); err == nil || !strings.Contains(err.Error(), "profile fixture stopped") {
		t.Fatal("fixture did not stop after publication", err)
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
	if err := saveDocument(d.journalPath(j.Operation), j); err != nil {
		t.Fatal(err)
	}
	edit := []byte("# personal edit after installation\n")
	for _, target := range d.Targets["profile.test"] {
		data, err := os.ReadFile(target.Path)
		if err != nil {
			t.Fatal(err)
		}
		writeConfigFixture(t, target.Path, string(append(data, edit...)))
	}
	killProfileChildAfterMove(t, c, d, Request{Schema: 1, Mode: "restore"}, filepath.Join("restore", "previous"), 0)
	j, err = d.readRecoveryJournal("profile.test", state.Receipts["profile.test"])
	if err != nil || !j.Restoring || j.Complete || len(j.Restoration) == 0 || len(j.Restoration) == len(j.Entries) && j.Restoration[len(j.Restoration)-1].Published {
		t.Fatal("kill missed actual scoped restoration", j, err)
	}
	assertProfileRestorationCannotBeReversed(t, c, j)
	result := dispatchApproved(t, c, Request{Schema: 1, Mode: "restore"})
	if result.Status != "ready" {
		t.Fatal("restoration did not converge", result)
	}
	for path, original := range originals {
		actual, err := os.ReadFile(path)
		if err != nil || !bytes.Equal(actual, append(bytes.Clone(original), edit...)) {
			t.Fatal("process death lost active personal edits", path, string(actual), err)
		}
	}
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"test"}})
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{}})
}

// Kill the actual child after its first physical rename, without provider hooks.
func killProfileChildAfterMove(t *testing.T, c Controller, d *ProfileDriver, request Request, watched string, watchedIndex int) {
	t.Helper()
	f := profileCrashFixture{c.Home, c.StatePath, c.Source, *c.Catalog, c.Context, *d, request}
	fixture := filepath.Join(c.Home, "crash-fixture.json")
	if err := saveDocument(fixture, f); err != nil {
		t.Fatal(err)
	}
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	child := exec.Command(binary, "-test.run=^TestProfileCrashChild$", "-test.timeout=30s")
	child.Env = append(os.Environ(), "DOTFILES_PROFILE_CRASH_FIXTURE="+fixture)
	log, err := os.Create(filepath.Join(c.Home, "child.log"))
	if err != nil {
		t.Fatal(err)
	}
	child.Stdout, child.Stderr = log, log
	if err := child.Start(); err != nil {
		t.Fatal(err)
	}
	waited := false
	t.Cleanup(func() {
		if !waited {
			if err := child.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
				t.Error(err)
			}
			if err := child.Wait(); err == nil {
				t.Error("uninterrupted child succeeded")
			}
		}
		if err := log.Close(); err != nil {
			t.Error(err)
		}
	})
	// Observe a real rename; watching the second target guarantees that the
	// first target already published. The child has no fixture hooks or
	// publication wrapper. Later targets keep publication in progress;
	// assert the actual persisted cut after the kill, never assume it.
	deadline := time.Now().Add(15 * time.Second)
	moved := false
	for time.Now().Before(deadline) && !moved {
		state, err := LoadState(c.StatePath, c.Home)
		// The observer deliberately races atomic state publication. Retry only
		// its explicit changed-inode result within the existing deadline.
		if err != nil && strings.Contains(err.Error(), "installer document changed while opening; retry discovery") {
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		if state.Transaction == nil || state.Transaction.InFlight != "profile.test" {
			continue
		}
		// Earlier adoptions deliberately retain previous copies. Watch only
		// the operation the real engine has just recorded as in flight.
		operation := state.Receipts["profile.test"].OperationID
		workspace := profileWorkspace(d.Targets["profile.test"][watchedIndex].Path, operation, watchedIndex)
		_, err = os.Lstat(filepath.Join(workspace, watched))
		if err == nil {
			moved = true
		} else if !errors.Is(err, os.ErrNotExist) {
			t.Fatal(err)
		}
	}
	if !moved {
		output, err := os.ReadFile(log.Name())
		t.Fatalf("child did not move a real profile: %s (%v)", output, err)
	}
	if err := child.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	if err := child.Wait(); err == nil {
		t.Fatal("child survived process kill")
	}
	waited = true
}
