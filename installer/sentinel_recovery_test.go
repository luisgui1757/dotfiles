package installer

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSentinelRecoversEarlierPolicyAdoptionAfterActualProcessDeath(t *testing.T) {
	for _, recovery := range []string{"resume", "restore"} {
		t.Run(recovery, func(t *testing.T) {
			c, d := sentinelController(t)
			original := "Personal rules.\n" + sentinelBegin + "\nEarlier policy.\n" + sentinelEnd + "\nTrailing rules.\n"
			for _, target := range d.Targets[sentinelResource] {
				writeConfigFixture(t, target.Path, original)
			}
			request := Request{Schema: 1, Mode: "apply", Selected: []string{"sentinel"}, Adopt: []string{sentinelResource}}
			killSentinelChildAfterSecondMove(t, c, d, request)
			state, err := LoadState(c.StatePath, c.Home)
			if err != nil || state.Transaction == nil {
				t.Fatal("kill missed active transaction", err)
			}
			journal, err := d.readJournal(sentinelResource, state.Receipts[sentinelResource])
			if err != nil || journal.Complete || !journal.Entries[0].Published || journal.Entries[3].Published {
				t.Fatal("fixture did not interrupt after first policy publication", journal, err)
			}
			request = Request{Schema: 1, Mode: "apply", Retry: true}
			if recovery == "restore" {
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
			if recovery == "resume" && result.Status == "needs-action" {
				request.ExpectedPlan = ""
				result = dispatchApproved(t, c, request)
			}
			if result.Status != "ready" {
				t.Fatal("recovery did not converge", result)
			}
			if recovery == "resume" {
				for _, entry := range journal.Entries {
					saved := filepath.Join(entry.Workspace, "previous")
					got, err := os.ReadFile(saved)
					if err != nil || string(got) != original || !strings.Contains(result.Message, saved) {
						t.Fatal("adoption lost original policy or its reference", saved, err)
					}
				}
				dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{}})
			}
			for _, target := range d.Targets[sentinelResource] {
				got, err := os.ReadFile(target.Path)
				if err != nil || !bytes.Equal(got, []byte(original)) {
					t.Fatal("recovery/removal did not restore earlier policy", target.Path, err)
				}
			}
		})
	}
}

// Reuse the real ProfileDriver child with no test hooks in the publisher. Watch
// the second consumer's original rename, so the first policy has already been
// published; the remaining two consumers keep the transaction in flight.
func killSentinelChildAfterSecondMove(t *testing.T, c Controller, d *ProfileDriver, request Request) {
	t.Helper()
	fixture := filepath.Join(c.Home, "sentinel-crash-fixture.json")
	if err := saveDocument(fixture, profileCrashFixture{c.Home, c.StatePath, c.Source, *c.Catalog, c.Context, *d, request}); err != nil {
		t.Fatal(err)
	}
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	child := exec.Command(binary, "-test.run=^TestProfileCrashChild$", "-test.timeout=30s")
	child.Env = append(os.Environ(), "DOTFILES_PROFILE_CRASH_FIXTURE="+fixture)
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
	deadline := time.Now().Add(15 * time.Second)
	moved := false
	for time.Now().Before(deadline) && !moved {
		state, err := LoadState(c.StatePath, c.Home)
		if err != nil && strings.Contains(err.Error(), "installer document changed while opening; retry discovery") {
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		if state.Transaction == nil || state.Transaction.InFlight != sentinelResource {
			continue
		}
		workspace := profileWorkspace(d.Targets[sentinelResource][1].Path, state.Receipts[sentinelResource].OperationID, 1)
		_, err = os.Lstat(filepath.Join(workspace, "previous"))
		if err == nil {
			moved = true
		} else if !errors.Is(err, os.ErrNotExist) {
			t.Fatal(err)
		}
	}
	if !moved {
		t.Fatal("child did not rename the second consumer policy")
	}
	if err := child.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	if err := child.Wait(); err == nil {
		t.Fatal("child survived process kill")
	}
	waited = true
}
