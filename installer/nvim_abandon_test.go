package installer

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestNvimAbandonPreservesFailedStageAndAllowsFreshSource(t *testing.T) {
	for _, scenario := range []struct{ previous, changed, postcondition bool }{{false, false, false}, {false, true, false}, {true, true, false}, {false, true, true}} {
		previous := scenario.previous
		name := "first installation same source"
		if scenario.changed {
			name = "first installation changed source"
		}
		if previous {
			name = "update of owned runtime"
		}
		if scenario.postcondition {
			name = "failed postcondition"
		}
		t.Run(name, func(t *testing.T) {
			d, r, _, _ := nvimSyncFixture(t)
			r.Name, r.Capability = "Neovim runtime", true
			c := Controller{Catalog: &Catalog{Schema: 1, Resources: []Resource{r}}, Context: nativePlatform(t).Context,
				Source: "first-source", Home: filepath.Dir(d.Repository), StatePath: filepath.Join(d.Directory, "engine", "state.json"), Driver: d}
			if previous {
				dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{r.ID}})
				writeConfigFixture(t, filepath.Join(d.Repository, "nvim", "init.lua"), "-- updated configuration\n")
				c.Source = "second-source"
			}
			beforeTarget, err := archiveLinkTarget(d.current())
			if err != nil {
				t.Fatal(err)
			}
			run := d.Run
			query := d.query
			if scenario.postcondition {
				d.query = func(context.Context, nativeCommand) ([]byte, error) {
					return []byte("formatter missing"), errors.New("exit 1")
				}
			}
			d.Run = func(ctx context.Context, command nativeCommand) ([]byte, error) {
				if _, err := run(ctx, command); err != nil {
					return nil, err
				}
				if !scenario.postcondition && slices.Contains(command.Arguments, "+lua require('util.mason_tools').run_checked('MasonToolsInstallSync')") {
					return []byte("formatter preparation failed"), errors.New("exit 1")
				}
				return nil, nil
			}
			request := Request{Schema: 1, Mode: "apply", Selected: []string{r.ID}}
			preview, err := c.Dispatch(context.Background(), request)
			if err != nil {
				t.Fatal(err)
			}
			request.ExpectedPlan = preview.Plan.ID
			if _, err := c.Dispatch(context.Background(), request); err == nil {
				t.Fatal("failed native phase reported success")
			}
			state, err := LoadState(c.StatePath, c.Home)
			if err != nil {
				t.Fatal(err)
			}
			operation := state.Receipts[r.ID].OperationID
			journal, err := os.ReadFile(d.intentPath(operation))
			if err != nil {
				t.Fatal(err)
			}
			stage, err := snapshotTree(d.payload(operation), maxPackageBytes, maxNvimEntries)
			if err != nil {
				t.Fatal(err)
			}
			abandoned := dispatchApproved(t, c, Request{Schema: 1, Mode: "abandon"})
			if !strings.Contains(abandoned.Message, d.payload(operation)) {
				t.Fatal("Abandon did not disclose its stage", abandoned.Message)
			}
			state, err = LoadState(c.StatePath, c.Home)
			if err != nil || state.Transaction != nil {
				t.Fatal("abandon did not archive the transaction", err)
			}
			observed, err := d.Observe(context.Background(), r, state.Receipts[r.ID])
			if err != nil || observed.Pending != "" || observed.ResourceResume != nil {
				t.Fatal("abandoned intent still traps the resource in recovery", observed, err)
			}
			if current, err := archiveLinkTarget(d.current()); err != nil || current != beforeTarget {
				t.Fatal("abandon changed the published runtime", current, err)
			}
			for _, path := range []string{d.intentPath(operation), d.payload(operation)} {
				if !slices.Contains(observed.Preserved, path) {
					t.Fatal("abandoned evidence was not disclosed", path, observed)
				}
			}
			if scenario.changed {
				writeConfigFixture(t, filepath.Join(d.Repository, "nvim", "init.lua"), "-- corrected configuration\n")
				c.Source = "corrected-source"
			}
			d.Run, d.query = run, query
			dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{r.ID}})
			final, err := LoadState(c.StatePath, c.Home)
			if err != nil || final.Receipts[r.ID].OperationID == operation || !slices.Contains(final.Receipts[r.ID].After.Preserved, d.payload(operation)) {
				t.Fatal("fresh operation lost the abandoned stage or reused its identity", final, err)
			}
			if check, err := c.Dispatch(context.Background(), Request{Schema: 1, Mode: "check"}); err != nil || check.Status != "ready" {
				t.Fatal(check, err)
			}
			if data, err := os.ReadFile(d.intentPath(operation)); err != nil || string(data) != string(journal) {
				t.Fatal("fresh operation rewrote the failed journal", err)
			}
			if actual, err := snapshotTree(d.payload(operation), maxPackageBytes, maxNvimEntries); err != nil || actual != stage {
				t.Fatal("fresh operation changed the abandoned stage", err)
			}
		})
	}
}

func TestNvimAbandonRefusesUnfinishedPublication(t *testing.T) {
	for _, previous := range []bool{false, true} {
		d, r, receipt, _ := nvimSyncFixture(t)
		if previous {
			after, err := d.Apply(context.Background(), r, Operation{Action: "install"}, receipt)
			if err != nil {
				t.Fatal(err)
			}
			receipt.After, receipt.Ownership, receipt.OperationID = after, "created", strings.Repeat("b", 64)
		}
		if _, err := d.Apply(context.Background(), r, Operation{Action: "install"}, receipt); err != nil {
			t.Fatal(err)
		}
		intent, err := d.intent(receipt.OperationID)
		if err != nil {
			t.Fatal(err)
		}
		// The process can stop after switching current, before saving completion.
		intent.Complete = false
		if err := d.save(intent); err != nil {
			t.Fatal(err)
		}
		if _, err := d.FinishTransaction(context.Background(), Plan{Mode: "abandon"}, map[string]Receipt{r.ID: receipt}); err == nil {
			t.Fatal("abandon discarded recovery authority during publication")
		}
		if current, err := archiveLinkTarget(d.current()); err != nil || current != d.payload(receipt.OperationID) {
			t.Fatal("refused abandonment changed the published target", current, err)
		}
	}
}
