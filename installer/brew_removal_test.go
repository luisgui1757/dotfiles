package installer

import (
	"context"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestBrewRemovalDeletesRetainedVersionsAndPreservesOutsideConsumers(t *testing.T) {
	var installed map[string]nativePackage
	d, resource, receipt := brewMutationFixture(t, func(current map[string]nativePackage) { installed = current })
	after, err := d.Apply(context.Background(), resource, Operation{Action: "install"}, receipt)
	if err != nil {
		t.Fatal(err)
	}
	// Homebrew keeps older kegs when automatic cleanup is disabled. Its ordinary
	// uninstall removes only the active keg; --force selects all formula kegs.
	root := installed["first"]
	root.Version = "1.0,2.0"
	installed["first"] = root
	receipt.After, receipt.Ownership, receipt.OperationID = after, "created", strings.Repeat("c", 64)
	d.Run = func(_ context.Context, command nativeCommand) ([]byte, error) {
		if slices.Contains(command.Arguments, "--ignore-dependencies") {
			t.Fatal("removal bypassed the native dependency guard")
		}
		if slices.Contains(command.Arguments, "--force") {
			delete(installed, "first")
		} else {
			root.Version = "1.0"
			installed["first"] = root
		}
		hash, err := digest(command)
		if err != nil {
			return nil, err
		}
		code := 0
		return nil, saveDocument(filepath.Join(d.Directory, "worker", "commands", command.Operation+".json"), nativeCommandRecord{Schema: 1, Command: hash, Reply: &nativeReply{ExitCode: &code}})
	}
	observed, err := d.Remove(context.Background(), resource, receipt)
	if err != nil || observed.Present || observed.CompletedOperation != receipt.OperationID {
		t.Fatal("uninstall left a retired version installed", observed, err)
	}
	if _, present := installed["outside"]; !present {
		t.Fatal("uninstall removed an outside consumer")
	}
	if _, present := installed["shared"]; !present {
		t.Fatal("uninstall removed a pre-existing dependency")
	}
}

func TestBrewRemovalRechecksProtectedInputsBeforeDispatchAndRecovery(t *testing.T) {
	for _, change := range []string{"pin", "source", "manual", "consumer"} {
		t.Run(change, func(t *testing.T) {
			var installed map[string]nativePackage
			d, resource, receipt := brewMutationFixture(t, func(current map[string]nativePackage) { installed = current })
			after, err := d.Apply(context.Background(), resource, Operation{Action: "install"}, receipt)
			if err != nil {
				t.Fatal(err)
			}
			receipt.After, receipt.Ownership, receipt.OperationID = after, "created", strings.Repeat("d", 64)
			query, calls := d.Query, 0
			d.Query = func(ctx context.Context, privileged bool, program string, input []byte, args ...string) ([]byte, error) {
				output, err := query(ctx, privileged, program, input, args...)
				calls++
				if calls == 1 {
					root := installed["first"]
					switch change {
					case "pin":
						root.Held = true
					case "source":
						root.Source = "outside/tap/first"
					case "manual":
						root.Automatic = !root.Automatic
					case "consumer":
						outside := installed["outside"]
						outside.Dependencies = append(outside.Dependencies, "first")
						installed["outside"] = outside
					}
					installed["first"] = root
				}
				return output, err
			}
			mutations := 0
			d.Run = func(context.Context, nativeCommand) ([]byte, error) { mutations++; return nil, nil }
			if _, err := d.Remove(context.Background(), resource, receipt); err == nil || mutations != 0 {
				t.Fatal("changed protected input reached native uninstall", change, mutations, err)
			}
			if _, err := d.ResumeResource(context.Background(), resource, Operation{Action: "remove"}, receipt); err == nil || mutations != 0 {
				t.Fatal("recovery bypassed removal protection", change, mutations, err)
			}
		})
	}
}

func TestBrewRemovalReportsOnlySurvivingIncidentalPackages(t *testing.T) {
	for _, change := range []string{"outside-consumer", "pin", "manual-promotion", "new-source", "subsequently-removed"} {
		t.Run(change, func(t *testing.T) {
			installed := map[string]nativePackage{}
			d, resource, receipt := brewMutationInventoryFixture(t, installed, func(current map[string]nativePackage) {
				current["shared"] = nativePackage{Name: "shared", Source: "shared", Version: "2.0", Automatic: true, Healthy: true}
			})
			after, err := d.Apply(context.Background(), resource, Operation{Action: "install"}, receipt)
			if err != nil {
				t.Fatal(err)
			}
			dependency := installed["shared"]
			switch change {
			case "pin":
				dependency.Held = true
			case "manual-promotion":
				dependency.Automatic = false
			case "new-source":
				dependency.Source = "outside/tap/shared"
			default:
				installed["outside"] = nativePackage{Name: "outside", Source: "outside", Version: "1.0", Healthy: true, Dependencies: []string{"shared"}}
			}
			installed["shared"] = dependency
			receipt.After, receipt.Ownership, receipt.OperationID = after, "created", strings.Repeat("e", 64)
			d.Run = func(_ context.Context, command nativeCommand) ([]byte, error) {
				if !slices.Equal(command.Arguments, []string{"uninstall", "--formula", "--force", "first"}) {
					t.Fatal("removal exceeded its unconsumed owned set", command.Arguments)
				}
				delete(installed, "first")
				hash, err := digest(command)
				if err != nil {
					return nil, err
				}
				code := 0
				return nil, saveDocument(filepath.Join(d.Directory, "worker", "commands", command.Operation+".json"), nativeCommandRecord{Schema: 1, Command: hash, Reply: &nativeReply{ExitCode: &code}})
			}
			observed, err := d.Remove(context.Background(), resource, receipt)
			if err != nil {
				t.Fatal(err)
			}
			if change == "subsequently-removed" {
				// Exercise the persisted report shape independently of creation;
				// an older completed receipt can outlive its retained package.
				intent, err := d.intent(receipt.OperationID)
				if err != nil {
					t.Fatal(err)
				}
				intent.Preserved = []string{"shared"}
				if err := saveDocument(d.intentPath(receipt.OperationID), intent); err != nil {
					t.Fatal(err)
				}
				delete(installed, "shared")
				observed, err = d.Observe(context.Background(), resource, receipt)
				if err != nil || len(observed.Preserved) != 0 {
					t.Fatal("missing dependency is still reported as retained", observed.Preserved, err)
				}
				return
			}
			if !slices.Equal(observed.Preserved, []string{filepath.Join(d.Cellar, "shared")}) {
				t.Fatal("removal concealed its retained dependency", observed.Preserved)
			}
			state, err := d.ledger()
			_, owned := state.Pool["shared"]
			if err != nil || owned != (change == "outside-consumer" || change == "pin") {
				t.Fatal("pool did not preserve or relinquish the dependency's current ownership", state.Pool, err)
			}
			// Model death after ledger publication but before the final completed
			// intent: disclosure was durable first and must survive recovery.
			intent, err := d.intent(receipt.OperationID)
			if err != nil {
				t.Fatal(err)
			}
			intent.Complete = false
			if err := saveDocument(d.intentPath(receipt.OperationID), intent); err != nil {
				t.Fatal(err)
			}
			d.Run = func(context.Context, nativeCommand) ([]byte, error) {
				t.Fatal("completed native removal was replayed")
				return nil, nil
			}
			resumed, err := d.ResumeResource(context.Background(), resource, Operation{Action: "remove"}, receipt)
			if err != nil || !slices.Equal(resumed.Preserved, observed.Preserved) {
				t.Fatal("recovery lost retained-package disclosure", resumed.Preserved, err)
			}
		})
	}
}
