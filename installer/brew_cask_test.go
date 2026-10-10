package installer

import (
	"context"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestBrewAffectedCaskDoesNotStrandCompletedPackageCommand(t *testing.T) {
	for _, inactive := range []bool{false, true} {
		t.Run(map[bool]string{false: "healthy", true: "recover-inactive-dependency"}[inactive], func(t *testing.T) {
			installed := map[string]nativePackage{
				"shared":   {Name: "shared", Source: "shared", Version: "1.0", Automatic: true, Healthy: true},
				"other":    {Name: "other", Source: "other", Version: "1.0", Healthy: true},
				"cask/app": {Name: "cask/app", Source: "app", Version: "1.0", Healthy: true, Dependencies: []string{"shared", "other"}},
			}
			d, resource, receipt := brewMutationInventoryFixture(t, installed, func(packages map[string]nativePackage) {
				other := packages["other"]
				other.Healthy = !inactive
				packages["other"] = other
			})
			query := d.Query
			d.Query = func(ctx context.Context, privileged bool, program string, input []byte, args ...string) ([]byte, error) {
				if len(args) > 0 && args[0] == "linkage" {
					if len(args) <= 2 {
						t.Fatal("cask verification broadened to every installed formula")
					}
					for _, name := range args[2:] {
						if name == "app" || strings.HasPrefix(name, "cask/") {
							t.Fatal("application was passed to the formula linkage command", args)
						}
					}
				}
				return query(ctx, privileged, program, input, args...)
			}
			after, err := d.Apply(context.Background(), resource, Operation{Action: "install"}, receipt)
			if inactive {
				if err == nil {
					t.Fatal("completion accepted an inactive declared cask dependency")
				}
				other := installed["other"]
				other.Healthy = true
				installed["other"] = other
				after, err = d.ResumeResource(context.Background(), resource, Operation{Action: "install"}, receipt)
			}
			if err != nil || !after.Healthy || after.CompletedOperation != receipt.OperationID {
				t.Fatal("healthy affected cask stranded a completed package command", after, err)
			}
			if !slices.Equal(after.UnverifiedApplications, []string{"app"}) {
				t.Fatal("external application verification gap was concealed", after)
			}
			intent, err := d.intent(receipt.OperationID)
			if err != nil || !slices.Equal(intent.UncheckedCasks, []string{"cask/app"}) {
				t.Fatal("application disclosure was not durable", intent.UncheckedCasks, err)
			}
			ledger, err := d.ledger()
			if err != nil || len(ledger.Roots) != 1 || len(ledger.Pool) != 0 || installed["cask/app"].Version != "1.0" {
				t.Fatal("cask maintenance changed ownership or application identity", ledger, err)
			}
		})
	}
}

func TestBrewUnchangedExternalApplicationDoesNotBroadenMaintenanceDisclosure(t *testing.T) {
	d, resource, receipt := brewMutationInventoryFixture(t, map[string]nativePackage{
		"shared":   {Name: "shared", Source: "shared", Version: "1.0", Automatic: true, Healthy: true},
		"other":    {Name: "other", Source: "other", Version: "1.0", Healthy: true},
		"cask/app": {Name: "cask/app", Source: "app", Version: "1.0", Healthy: true, Dependencies: []string{"other"}},
	}, nil)
	after, err := d.Apply(context.Background(), resource, Operation{Action: "install"}, receipt)
	if err != nil || !after.Healthy || len(after.UnverifiedApplications) != 0 {
		t.Fatal("unaffected application entered formula maintenance disclosure", after, err)
	}
}

func TestBrewApplicationDisclosureSurvivesControllerCompletionAndCheck(t *testing.T) {
	d, resource, _ := brewMutationInventoryFixture(t, map[string]nativePackage{
		"shared":   {Name: "shared", Source: "shared", Version: "1.0", Automatic: true, Healthy: true},
		"cask/app": {Name: "cask/app", Source: "app", Version: "1.0", Healthy: true, Dependencies: []string{"shared"}},
	}, nil)
	home := filepath.Dir(d.Directory)
	c := Controller{Catalog: &Catalog{Schema: 1, Resources: []Resource{{ID: "first", Name: "First", Capability: true, Requires: []string{resource.ID}}, resource}},
		Context: Context{OS: "darwin", Arch: "arm64"}, Source: "cask-disclosure", Home: home, StatePath: filepath.Join(home, "state.json"), Driver: d}
	result := dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"first"}})
	if !strings.Contains(result.Message, "External application not exercised: app") {
		t.Fatal("successful mutation concealed external application verification limits", result)
	}
	result, err := c.Dispatch(context.Background(), Request{Schema: 1, Mode: "check"})
	if err != nil || result.Status != "ready" || !strings.Contains(result.Message, "External application not exercised: app") {
		t.Fatal("read-only check lost or misclassified application disclosure", result, err)
	}
}

func TestBrewApplicationDisclosureAcceptsLegacyAbsenceAndRejectsInvalidNames(t *testing.T) {
	var old Observation
	if err := Decode([]byte(`{"present":true,"healthy":true}`), &old); err != nil || len(old.UnverifiedApplications) != 0 {
		t.Fatal("old observations require no application disclosure", old, err)
	}
	d, _, receipt, intent := brewDriverFixture(t)
	if err := saveDocument(d.intentPath(receipt.OperationID), intent); err != nil {
		t.Fatal(err)
	}
	if legacy, err := d.intent(receipt.OperationID); err != nil || len(legacy.UncheckedCasks) != 0 {
		t.Fatal("existing schema-3 intents require no new disclosure", legacy, err)
	}
	for _, invalid := range []string{"../outside", "cask/unrecorded", "first"} {
		intent.UncheckedCasks = []string{invalid}
		if err := saveDocument(d.intentPath(receipt.OperationID), intent); err != nil {
			t.Fatal(err)
		}
		if _, err := d.intent(receipt.OperationID); err == nil {
			t.Fatal("invalid external application disclosure was accepted", invalid)
		}
	}
}

func TestBrewFormulaMaintenanceCannotChangeAffectedCaskIdentity(t *testing.T) {
	for _, change := range []string{"version", "source"} {
		t.Run(change, func(t *testing.T) {
			d, resource, receipt := brewMutationInventoryFixture(t, map[string]nativePackage{
				"shared":   {Name: "shared", Source: "shared", Version: "1.0", Automatic: true, Healthy: true},
				"cask/app": {Name: "cask/app", Source: "app", Version: "1.0", Healthy: true, Dependencies: []string{"shared"}},
			}, func(installed map[string]nativePackage) {
				app := installed["cask/app"]
				if change == "version" {
					app.Version = "2.0"
				} else {
					app.Source = "other/tap/app"
				}
				installed["cask/app"] = app
			})
			if _, err := d.Apply(context.Background(), resource, Operation{Action: "install"}, receipt); err == nil || !strings.Contains(err.Error(), "application cask/app changed") {
				t.Fatal("formula maintenance accepted an external application change", err)
			}
		})
	}
}
