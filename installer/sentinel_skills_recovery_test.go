package installer

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

type sentinelSkillsCrashFixture struct {
	Catalog Catalog
	Driver  SentinelSkillsDriver
	Source  string
}

func (f sentinelSkillsCrashFixture) controller() Controller {
	statePath := filepath.Join(f.Driver.Directory, "state.json")
	d := &NativeDriver{Catalog: &f.Catalog, Context: f.Driver.Target, StatePath: statePath, SentinelSkills: &f.Driver, Configurations: &ConfigDriver{Directory: f.Driver.Directory}}
	return Controller{Catalog: &f.Catalog, Context: f.Driver.Target, Source: f.Source, Home: f.Driver.Home, StatePath: statePath, Driver: d}
}

func TestSentinelSkillsCrashChild(t *testing.T) {
	path := os.Getenv("DOTFILES_SENTINEL_SKILLS_CRASH_FIXTURE")
	if path == "" {
		return
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var fixture sentinelSkillsCrashFixture
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	dispatchApproved(t, fixture.controller(), Request{Schema: 1, Mode: "apply", Selected: []string{"sentinel-skills"}})
	t.Fatal("parent did not interrupt skill publication")
}

func TestSentinelSkillsRecoverAfterActualProcessDeath(t *testing.T) {
	for _, recovery := range []string{"resume", "restore"} {
		t.Run(recovery, func(t *testing.T) {
			c, d := sentinelSkillsController(t)
			fixture := filepath.Join(filepath.Dir(d.Directory), "skill-crash.json")
			if err := saveDocument(fixture, sentinelSkillsCrashFixture{*c.Catalog, *d, c.Source}); err != nil {
				t.Fatal(err)
			}
			binary, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			child := exec.Command(binary, "-test.run=^TestSentinelSkillsCrashChild$", "-test.timeout=30s")
			child.Env = append(os.Environ(), "DOTFILES_SENTINEL_SKILLS_CRASH_FIXTURE="+fixture)
			var output bytes.Buffer
			child.Stdout, child.Stderr = &output, &output
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
						t.Error("child completed before interruption")
					}
				}
			})
			first := filepath.Join(d.Home, ".agents", "skills", "beast-ui", "SKILL.md")
			published := false
			deadline := time.Now().Add(15 * time.Second)
			for time.Now().Before(deadline) {
				if _, err := os.Lstat(first); err == nil {
					published = true
					break
				} else if !errors.Is(err, os.ErrNotExist) {
					t.Fatal(err)
				}
				time.Sleep(time.Millisecond)
			}
			if !published {
				t.Fatal("child did not publish the first skill")
			}
			if err := child.Process.Kill(); err != nil {
				t.Fatal(err)
			}
			if err := child.Wait(); err == nil {
				t.Fatal("child survived termination", output.String())
			}
			waited = true
			state, err := LoadState(c.StatePath, c.Home)
			if err != nil || state.Transaction == nil || state.Transaction.InFlight != "skills.sentinel-agents" {
				t.Fatal("kill missed the first resource publication", err, output.String())
			}
			configuration, resource, _, err := d.configuration(Resource{ID: "skills.sentinel-agents"})
			if err != nil {
				t.Fatal(err)
			}
			journal, err := configuration.readJournal(resource, state.Receipts[resource.ID])
			if err != nil || journal.Complete || !journal.Staged {
				t.Fatal("fixture did not interrupt a staged copy publication", err)
			}
			request := Request{Schema: 1, Mode: "apply", Retry: true}
			if recovery == "restore" {
				request = Request{Schema: 1, Mode: "restore"}
			}
			plan, err := c.Preview(context.Background(), request)
			if err != nil {
				t.Fatal(err)
			}
			request.ExpectedPlan = plan.ID
			result, err := c.Dispatch(context.Background(), request)
			if err != nil {
				t.Fatal(err)
			}
			if result.Status == "needs-action" && recovery == "resume" {
				request.ExpectedPlan = ""
				result = dispatchApproved(t, c, request)
			}
			if result.Status != "ready" {
				t.Fatal("skill recovery did not converge", result)
			}
			state, err = LoadState(c.StatePath, c.Home)
			if err != nil || state.Transaction != nil || len(state.PastTransactions) == 0 {
				t.Fatal("recovery lost original transaction history", err)
			}
			for _, entry := range journal.Entries {
				if recovery == "resume" {
					got, err := snapshotConfig(entry.State.Target.Destination)
					if err != nil || got != entry.State.Desired {
						t.Fatal("resume changed approved bytes or permissions", err)
					}
				} else if _, err := os.Lstat(entry.State.Target.Destination); !errors.Is(err, os.ErrNotExist) {
					t.Fatal("restore retained an interrupted publication", err)
				}
			}
			if recovery == "resume" {
				check, err := c.Dispatch(context.Background(), Request{Schema: 1, Mode: "check"})
				if err != nil || check.Status != "ready" {
					t.Fatal("resumed skills are unhealthy", check, err)
				}
			}
		})
	}
}
