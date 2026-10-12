package installer

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestNvimPublicationMustRecoverBeforeAbandon(t *testing.T) {
	for _, action := range []string{"install", "update", "repair", "remove"} {
		for _, boundary := range []string{"unstarted", "previous", "next", "published", "complete"} {
			if action != "remove" && boundary == "unstarted" || action == "install" && boundary == "previous" || action == "remove" && boundary == "next" {
				continue
			}
			t.Run(action+"/"+boundary, func(t *testing.T) {
				d, r, _, commands := nvimSyncFixture(t)
				r.Name, r.Capability = "Neovim runtime", true
				c := Controller{Catalog: &Catalog{Schema: 1, Resources: []Resource{r}}, Context: nativePlatform(t).Context,
					Source: "original-source", Home: filepath.Dir(d.Repository), StatePath: filepath.Join(d.Directory, "engine", "state.json"), Driver: d}
				if action != "install" {
					dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{r.ID}})
				}
				request := Request{Schema: 1, Mode: "apply", Selected: []string{r.ID}}
				if action == "remove" {
					request.Selected = []string{}
				} else if action == "repair" {
					writeConfigFixture(t, filepath.Join(d.Repository, "nvim", "init.lua"), "-- changed source\n")
					c.Source, request.Mode = "changed-source", action
				} else if action == "update" {
					// Historical receipts without Desired still authorize an
					// explicit update of their unchanged, owned runtime.
					state, err := LoadState(c.StatePath, c.Home)
					if err != nil {
						t.Fatal(err)
					}
					receipt := state.Receipts[r.ID]
					receipt.After.Desired = ""
					state.Receipts[r.ID] = receipt
					if err := SaveState(c.StatePath, state); err != nil {
						t.Fatal(err)
					}
					request.Mode = action
				}
				preview, err := c.Dispatch(context.Background(), request)
				if err != nil || len(preview.Plan.Operations) != 1 || preview.Plan.Operations[0].Action != action {
					t.Fatal("fixture did not approve the requested operation", preview, err)
				}
				state, err := LoadState(c.StatePath, c.Home)
				if err != nil {
					t.Fatal(err)
				}
				operation, err := digest(struct{ Plan, Resource, Action string }{preview.Plan.ID, r.ID, action})
				if err != nil {
					t.Fatal(err)
				}
				receipt := state.Receipts[r.ID]
				if action == "install" {
					receipt = Receipt{Ownership: "uncertain", Before: preview.Plan.Operations[0].Observed, After: preview.Plan.Operations[0].Observed}
				}
				receipt.Status, receipt.OperationID = "in-progress", operation
				state.Schema, state.Context, state.Target, state.Source = 1, c.Context, c.Home, c.Source
				state.Receipts = map[string]Receipt{r.ID: receipt}
				state.Transaction = &Transaction{Plan: preview.Plan, InFlight: r.ID}
				if err := SaveState(c.StatePath, state); err != nil {
					t.Fatal(err)
				}
				previous, err := archiveLinkTarget(d.current())
				if err != nil {
					t.Fatal(err)
				}
				desired, err := d.source()
				if err != nil {
					t.Fatal(err)
				}
				intent := nvimSyncIntent{Schema: 1, Operation: operation, Action: action, Desired: desired, PreviousTarget: previous}
				if action != "remove" {
					intent.Step, intent.Reserved = 3, true
					writeConfigFixture(t, filepath.Join(d.payload(operation), "owned"), "prepared runtime")
					writeConfigFixture(t, filepath.Join(d.payload(operation), ".lazy-lock-0.json"), "{}\n")
					entries, err := inspectTree(d.payload(operation), maxPackageBytes, maxNvimEntries)
					if err != nil {
						t.Fatal(err)
					}
					intent.Manifest, err = d.writeManifest(operation, entries)
					if err != nil {
						t.Fatal(err)
					}
				}
				publication := archiveIntent{Resource: r.ID, Operation: operation, Generation: operation, PreviousTarget: previous}
				if previous != "" && boundary != "unstarted" {
					if err := moveConfigExclusive(d.current(), d.links().linkWorkspace(publication, "previous")); err != nil {
						t.Fatal(err)
					}
				}
				if action != "remove" && boundary != "previous" {
					if err := os.MkdirAll(filepath.Dir(d.current()), 0700); err != nil {
						t.Fatal(err)
					}
					next := d.links().linkWorkspace(publication, "next")
					if err := createDirectoryLink(d.payload(operation), next); err != nil {
						t.Fatal(err)
					}
					if boundary != "next" {
						if err := moveConfigExclusive(next, d.current()); err != nil {
							t.Fatal(err)
						}
					}
				}
				intent.Complete = boundary == "complete"
				if err := d.save(intent); err != nil {
					t.Fatal(err)
				}
				before, err := snapshotTree(d.RuntimeDirectory, maxPackageBytes, maxNvimEntries)
				if err != nil {
					t.Fatal(err)
				}
				stateBytes, err := os.ReadFile(c.StatePath)
				if err != nil {
					t.Fatal(err)
				}
				abandon := Request{Schema: 1, Mode: "abandon"}
				preview, err = c.Dispatch(context.Background(), abandon)
				if err != nil {
					t.Fatal(err)
				}
				abandon.ExpectedPlan = preview.Plan.ID
				result, err := c.Dispatch(context.Background(), abandon)
				if action == "remove" && boundary == "unstarted" {
					if err != nil || result.Status != "ready" {
						t.Fatal("unstarted removal could not be abandoned", result, err)
					}
					state, err := LoadState(c.StatePath, c.Home)
					if err != nil {
						t.Fatal(err)
					}
					for _, path := range state.Receipts[r.ID].After.Preserved {
						if path == d.payload(operation) {
							t.Fatal("abandoned removal disclosed a nonexistent payload")
						}
					}
					dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{r.ID}})
					dispatchApproved(t, c, request)
					if current, err := archiveLinkTarget(d.current()); err != nil || current != "" {
						t.Fatal("fresh removal did not complete", current, err)
					}
					return
				}
				if action == "install" && boundary == "next" {
					if err != nil || result.Status != "ready" {
						t.Fatal("unpublished first installation could not be abandoned", result, err)
					}
					dispatchApproved(t, c, request)
					if target, err := archiveLinkTarget(d.links().linkWorkspace(publication, "next")); err != nil || target != d.payload(operation) {
						t.Fatal("fresh publication changed the abandoned link", target, err)
					}
					return
				}
				if err == nil {
					t.Fatal("abandon bypassed incomplete publication")
				}
				if data, err := os.ReadFile(c.StatePath); err != nil || string(data) != string(stateBytes) {
					t.Fatal("refused abandonment changed the engine intent", err)
				}
				if after, err := snapshotTree(d.RuntimeDirectory, maxPackageBytes, maxNvimEntries); err != nil || before != after {
					t.Fatal("refused abandonment changed links or runtime files", err)
				}
				count := len(*commands)
				request.Retry = true
				preview, err = c.Dispatch(context.Background(), request)
				if err != nil {
					t.Fatal(err)
				}
				request.ExpectedPlan = preview.Plan.ID
				result, err = c.Dispatch(context.Background(), request)
				if err != nil {
					t.Fatal("publication could not recover", result, err)
				}
				if result.Status == "needs-action" {
					request.ExpectedPlan = ""
					dispatchApproved(t, c, request)
				} else if result.Status != "ready" {
					t.Fatal("unexpected recovery status", result)
				}
				if len(*commands) != count {
					t.Fatal("publication recovery reran completed native phases")
				}
			})
		}
	}
}
