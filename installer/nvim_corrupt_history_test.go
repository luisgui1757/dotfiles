package installer

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestNvimUnreadableEvidenceStaysLocalAndPreserved(t *testing.T) {
	for _, scenario := range []string{"abandoned JSON", "abandoned manifest", "completed missing pointer", "completed current pointer", "active JSON"} {
		t.Run(scenario, func(t *testing.T) {
			d, resource, _, commands := nvimSyncFixture(t)
			resource.Name, resource.Capability = "Neovim runtime", true
			c, profiles, target := profileControllerAt(t, filepath.Dir(d.Repository))
			c.Catalog.Resources = append(c.Catalog.Resources, resource)
			c.Driver = &NativeDriver{Catalog: c.Catalog, Context: c.Context, StatePath: c.StatePath, Profiles: profiles, NvimSync: d,
				session: newNativeSession(filepath.Join(d.Directory, "worker"))}
			request := Request{Schema: 1, Mode: "apply", Selected: []string{resource.ID}}
			if strings.HasPrefix(scenario, "completed") {
				dispatchApproved(t, c, request)
				if scenario == "completed missing pointer" {
					if err := os.Remove(d.current()); err != nil {
						t.Fatal(err)
					}
				}
			} else {
				run := d.Run
				d.Run = func(ctx context.Context, command nativeCommand) ([]byte, error) {
					output, err := run(ctx, command)
					return output, errors.Join(err, errors.New("injected native phase failure"))
				}
				preview, err := c.Dispatch(context.Background(), request)
				if err != nil {
					t.Fatal(err)
				}
				request.ExpectedPlan = preview.Plan.ID
				if _, err := c.Dispatch(context.Background(), request); err == nil {
					t.Fatal("native failure was not reproduced")
				}
				if scenario != "active JSON" {
					dispatchApproved(t, c, Request{Schema: 1, Mode: "abandon"})
				}
			}
			state, err := LoadState(c.StatePath, c.Home)
			if err != nil {
				t.Fatal(err)
			}
			op := state.Receipts[resource.ID].OperationID
			journal := d.intentPath(op)
			if scenario == "abandoned manifest" {
				intent, err := d.intent(op)
				if err != nil {
					t.Fatal(err)
				}
				entries, err := inspectTree(d.payload(op), maxPackageBytes, maxNvimEntries)
				if err != nil {
					t.Fatal(err)
				}
				intent.Manifest, err = d.writeManifest(op, entries)
				if err != nil {
					t.Fatal(err)
				}
				if err := d.save(intent); err != nil {
					t.Fatal(err)
				}
				corruptNvimManifest(t, d, op, "tampered")
			} else {
				writeConfigFixture(t, journal, "{damaged saved JSON")
			}
			before, err := snapshotTree(d.Directory, maxPackageBytes, maxNvimEntries)
			if err != nil {
				t.Fatal(err)
			}
			count := len(*commands)
			check, err := c.Dispatch(context.Background(), Request{Schema: 1, Mode: "check"})
			wantStatus := "needs-action"
			if strings.HasPrefix(scenario, "abandoned") {
				// A failed first install never changed the recorded empty
				// selection. Check preserves that selection and discloses history.
				wantStatus = "ready"
			}
			if err != nil || check.Status != wantStatus {
				t.Fatal("damaged Neovim evidence caused a global Check failure", check, err)
			}
			observed := operation(t, check.Plan, resource.ID).Observed
			if !observed.Unknown || observed.Present || observed.CompletedOperation != "" || observed.ResourceResume != nil || !strings.Contains(observed.Pending, journal) || !slices.Contains(observed.Preserved, journal) {
				t.Fatal("damaged evidence gained authority or omitted its location", observed)
			}
			if scenario == "active JSON" {
				stateBytes, err := os.ReadFile(c.StatePath)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := c.Preview(context.Background(), Request{Schema: 1, Mode: "apply", Retry: true}); err == nil || !strings.Contains(err.Error(), resource.ID) {
					t.Fatal("unreadable active intent allowed recovery", err)
				}
				request := Request{Schema: 1, Mode: "abandon"}
				preview, err := c.Dispatch(context.Background(), request)
				if err != nil {
					t.Fatal(err)
				}
				request.ExpectedPlan = preview.Plan.ID
				if _, err := c.Dispatch(context.Background(), request); err == nil || !strings.Contains(err.Error(), journal) || !strings.Contains(err.Error(), "prior publication cannot be proved") {
					t.Fatal("unreadable publication intent allowed abandonment or hid its evidence", err)
				}
				after, err := os.ReadFile(c.StatePath)
				if err != nil || !bytes.Equal(stateBytes, after) {
					t.Fatal("refused abandonment changed the engine state", err)
				}
			} else {
				if scenario == "completed current pointer" {
					request := Request{Schema: 1, Mode: "apply", Selected: []string{}}
					preview, err := c.Dispatch(context.Background(), request)
					if err != nil || operation(t, preview.Plan, resource.ID).Action != "retain" {
						t.Fatal("corrupt current runtime gained removal authority", preview, err)
					}
					request.ExpectedPlan = preview.Plan.ID
					if _, err := c.Dispatch(context.Background(), request); err != nil {
						t.Fatal("deselection started a doomed removal", err)
					}
					state, err := LoadState(c.StatePath, c.Home)
					if err != nil || state.Transaction != nil {
						t.Fatal("deselection left recovery intent", err)
					}
				}
				request := Request{Schema: 1, Mode: "apply", Selected: []string{"test", resource.ID}}
				preview, err := c.Dispatch(context.Background(), request)
				if err != nil || operation(t, preview.Plan, "profile.test").Action != "install" {
					t.Fatal("historical corruption blocked an unrelated preview", preview, err)
				}
				request.ExpectedPlan = preview.Plan.ID
				result, err := c.Dispatch(context.Background(), request)
				if err != nil || result.Status != "needs-action" {
					t.Fatal("unrelated configuration could not apply with a local pending resource", result, err)
				}
				if data, err := os.ReadFile(target); err != nil || !strings.Contains(string(data), "# first version") {
					t.Fatal("unrelated configuration was not published", err)
				}
				if check, err := c.Dispatch(context.Background(), Request{Schema: 1, Mode: "check"}); err != nil || check.Status != "needs-action" {
					t.Fatal("selected damaged resource was reported healthy", check, err)
				}
			}
			after, err := snapshotTree(d.Directory, maxPackageBytes, maxNvimEntries)
			if err != nil || before != after || len(*commands) != count {
				t.Fatal("damaged evidence was changed or used for a native mutation", err)
			}
		})
	}
}
