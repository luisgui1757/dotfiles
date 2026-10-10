package installer

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"
)

func TestRecoveryMenuOffersOnlyTheCommittedRestorationDirection(t *testing.T) {
	for _, kind := range []string{"configuration", "profile"} {
		for _, phase := range []string{"started", "provider-completed"} {
			t.Run(kind+"/"+phase, func(t *testing.T) {
				var c Controller
				if kind == "configuration" {
					var d *ConfigDriver
					var j configJournal
					c, d, _, j = interruptedConfigController(t, "update", "after-backup")
					j.Restoring = true
					if err := saveDocument(d.journalPath(j.Operation), j); err != nil {
						t.Fatal(err)
					}
				} else {
					var d *ProfileDriver
					var j profileJournal
					c, d, j, _ = interruptedProfileController(t, "update", "moved")
					j.Restoring = true
					if err := saveDocument(d.journalPath(j.Operation), j); err != nil {
						t.Fatal(err)
					}
				}
				if phase == "provider-completed" {
					state, err := LoadState(c.StatePath, c.Home)
					if err != nil {
						t.Fatal(err)
					}
					id := state.Transaction.InFlight
					receipt := state.Receipts[id]
					provider := c.Driver.(ResourceRestoreDriver)
					observed, err := provider.ObserveRestore(context.Background(), id, receipt)
					if err != nil {
						t.Fatal(err)
					}
					if err := provider.RestoreResource(context.Background(), id, observed, receipt); err != nil {
						t.Fatal(err)
					}
				}
				ui := &scriptedInteraction{t: t, answers: [][]string{{"exit"}}}
				if _, err := c.Run(context.Background(), ui); err != nil {
					t.Fatal(err)
				}
				seen := map[string]bool{}
				for _, choice := range ui.menus[0] {
					seen[choice.ID] = true
				}
				if seen["retry"] || seen["abandon"] || !seen["restore"] || !seen["check"] {
					t.Fatal("recovery menu offered a direction the provider refuses", seen)
				}
				if _, err := c.Preview(context.Background(), Request{Schema: 1, Mode: "update", Retry: true}); err == nil || !strings.Contains(err.Error(), "restor") {
					t.Fatal("forward retry did not direct the user to restoration", err)
				}
			})
		}
	}
}

func TestConfigRestoreInspectsFilesBeforeChangingDirection(t *testing.T) {
	c, d, _, j := interruptedConfigController(t, "update", "after-backup")
	workspace := j.Entries[0].Workspace
	if err := moveConfigExclusive(workspace, workspace+"-saved"); err != nil {
		t.Fatal(err)
	}
	if err := createDirectoryLink(workspace+"-saved", workspace); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(d.journalPath(j.Operation))
	if err != nil {
		t.Fatal(err)
	}
	request := Request{Schema: 1, Mode: "restore"}
	preview, err := c.Preview(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	request.ExpectedPlan = preview.ID
	if _, err := c.Dispatch(context.Background(), request); err == nil {
		t.Fatal("accepted uninspectable configuration recovery")
	}
	after, err := os.ReadFile(d.journalPath(j.Operation))
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("failed inspection changed configuration recovery direction", err)
	}
}

func TestDamagedConfigRecoveryNamesItsJournalAndBaseline(t *testing.T) {
	c, d, receipt, j := interruptedConfigController(t, "update", "after-backup")
	path := d.journalPath(j.Operation)
	writeConfigFixture(t, path, "{damaged recovery journal")
	for _, request := range []Request{{Schema: 1, Mode: "restore"}, {Schema: 1, Mode: "abandon"}} {
		preview, err := c.Preview(context.Background(), request)
		if err == nil {
			request.ExpectedPlan = preview.ID
			_, err = c.Dispatch(context.Background(), request)
		}
		if err == nil || !strings.Contains(err.Error(), path) || !strings.Contains(err.Error(), d.baselinePath(receipt.Recovery)) {
			t.Fatal("damaged configuration evidence has no usable recovery locations", request.Mode, err)
		}
	}
}

func TestConfigurationCheckNamesAValidJSONJournalWithWrongIdentity(t *testing.T) {
	c, d, receipt, j := interruptedConfigController(t, "update", "after-backup")
	path := d.journalPath(j.Operation)
	j.Resource = "config.unrelated"
	if err := saveDocument(path, j); err != nil {
		t.Fatal(err)
	}
	for _, request := range []Request{{Schema: 1, Mode: "check"}, {Schema: 1, Mode: "update", Retry: true}} {
		plan, err := c.Preview(context.Background(), request)
		message := ""
		if err != nil {
			message = err.Error()
		} else {
			for _, op := range plan.Operations {
				message += op.Reason + op.Observed.Pending
			}
		}
		if !strings.Contains(message, path) || !strings.Contains(message, d.baselinePath(receipt.Recovery)) {
			t.Fatal("valid JSON with wrong identity omitted saved-evidence locations", request.Mode, message)
		}
	}
}
