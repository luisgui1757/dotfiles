package installer

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Stop after real metadata-preserving publication, then recreate each durable
// boundary. These are the same filesystem states reached by process interruption.
func interruptedWindowsTerminal(t *testing.T, action, cut string) (Controller, *ProfileDriver, profileJournal, []byte) {
	t.Helper()
	c, d, path := windowsTerminalController(t)
	writeConfigFixture(t, path, `{"defaultProfile":"{personal}","profiles":{"defaults":{"font":{"weight":"bold"}}}}`)
	request := Request{Schema: 1, Mode: "apply", Selected: []string{"test"}}
	if action != "install" {
		dispatchApproved(t, c, request)
		request = Request{Schema: 1, Mode: "update"}
		p, err := decodeWindowsTerminalProjection([]byte(d.Targets[windowsTerminalResource][0].Script))
		if err != nil {
			t.Fatal(err)
		}
		p.Values["initialRows"] = []byte("60")
		data, err := windowsTerminalProjectionData(p)
		if err != nil {
			t.Fatal(err)
		}
		d.Targets[windowsTerminalResource][0].Script = string(data)
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
		t.Fatal("publication did not stop", err)
	}
	c.Driver = d
	state, err := LoadState(c.StatePath, c.Home)
	if err != nil {
		t.Fatal(err)
	}
	j, err := d.readJournal(windowsTerminalResource, state.Receipts[windowsTerminalResource])
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

func TestWindowsTerminalInterruptedPublicationCanResumeOrRestoreWithoutCurrentRecipe(t *testing.T) {
	for _, action := range []string{"install", "update", "remove"} {
		for _, cut := range []string{"staged", "moved", "published"} {
			for _, direction := range []string{"resume", "restore"} {
				t.Run(action+"/"+cut+"/"+direction, func(t *testing.T) {
					c, d, j, original := interruptedWindowsTerminal(t, action, cut)
					if direction == "restore" {
						// Recovery is driven by the saved operation, even after a later
						// installer no longer configures this integration.
						d.Targets, d.JSONFields = nil, nil
						c.Catalog = &Catalog{Schema: 1}
						c.Driver = &NativeDriver{Catalog: c.Catalog, Profiles: d, StatePath: c.StatePath}
					}
					request := Request{Schema: 1, Mode: "apply", Retry: true}
					if action == "update" {
						request.Mode = "update"
					}
					if direction == "restore" {
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
					if direction == "resume" && result.Status == "needs-action" {
						request.ExpectedPlan = ""
						result = dispatchApproved(t, c, request)
					}
					if result.Status != "ready" {
						t.Fatal("recovery did not converge", result)
					}
					data, err := os.ReadFile(j.Entries[0].Path)
					if err != nil {
						t.Fatal(err)
					}
					if direction == "restore" && !bytes.Equal(data, original) {
						t.Fatal("restore lost original settings", string(data))
					}
					if !bytes.Contains(data, []byte(`"weight":"bold"`)) || !bytes.Contains(data, []byte(`"defaultProfile":"{personal}"`)) {
						t.Fatal("recovery lost personal settings", string(data))
					}
					state, err := LoadState(c.StatePath, c.Home)
					if err != nil || state.Transaction != nil {
						t.Fatal("recovery stranded transaction", state, err)
					}
				})
			}
		}
	}
}

func TestWindowsTerminalInterruptedRestoreRejectsRehashedBookkeepingTamper(t *testing.T) {
	c, d, j, _ := interruptedWindowsTerminal(t, "update", "moved")
	state, err := LoadState(c.StatePath, c.Home)
	if err != nil {
		t.Fatal(err)
	}
	receipt := state.Receipts[windowsTerminalResource]
	baseline, err := d.readBaseline(windowsTerminalResource, receipt)
	if err != nil {
		t.Fatal(err)
	}
	p, err := decodeWindowsTerminalProjection(baseline.Entries[0].Block)
	if err != nil {
		t.Fatal(err)
	}
	p.Default = []byte(`"{tampered}"`)
	baseline.Entries[0].Block, err = windowsTerminalProjectionData(p)
	if err != nil {
		t.Fatal(err)
	}
	baseline.Before.Inventory, err = digest([][]byte{baseline.Entries[0].Block})
	if err != nil {
		t.Fatal(err)
	}
	if err := saveDocument(d.baselinePath(receipt.Recovery), baseline); err != nil {
		t.Fatal(err)
	}
	before := profileRecoveryFiles(j)
	if _, err := d.ObserveRestore(context.Background(), windowsTerminalResource, receipt); err == nil {
		t.Fatal("tampered bookkeeping accepted for restoration")
	}
	beforeDigest, err := digest(before)
	if err != nil {
		t.Fatal(err)
	}
	afterDigest, err := digest(profileRecoveryFiles(j))
	if err != nil || beforeDigest != afterDigest {
		t.Fatal("tamper check changed saved files", err)
	}
}

func TestWindowsTerminalFreshRemovalDeletesOnlyTheEmptyOwnedDocument(t *testing.T) {
	for _, comment := range []bool{false, true} {
		c, _, path := windowsTerminalController(t)
		dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"test"}})
		if comment {
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			writeConfigFixture(t, path, "// later personal comment\n"+string(data))
		}
		dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{}})
		data, err := os.ReadFile(path)
		if comment {
			if err != nil || !bytes.Contains(data, []byte("// later personal comment")) {
				t.Fatal("lost personal comment", err)
			}
		} else if !os.IsNotExist(err) {
			t.Fatal("fresh removal retained empty document", string(data), err)
		}
	}
}
