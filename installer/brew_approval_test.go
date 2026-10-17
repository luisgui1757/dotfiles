package installer

import (
	"context"
	"maps"
	"path/filepath"
	"strings"
	"testing"
)

func TestBrewApprovalRejectsConsumerDriftBeforeMutation(t *testing.T) {
	d, resource, receipt, _ := brewDriverFixture(t)
	d.approval = &brewApproval{}
	state := brewLedger{Schema: 1, Cellar: d.Cellar, Roots: map[string]brewOwnership{resource.ID: {Name: "first", Source: "first", CreatedBy: receipt.OperationID}}, Pool: map[string]brewOwnership{"shared": {Name: "shared", Source: "sample/tap/shared", CreatedBy: receipt.OperationID}}}
	if err := saveDocument(filepath.Join(d.Directory, "packages.json"), state); err != nil {
		t.Fatal(err)
	}
	before, err := d.Observe(context.Background(), resource, Receipt{})
	if err != nil {
		t.Fatal(err)
	}
	if err := d.approveInventory(context.Background(), []string{before.Inventory}); err != nil {
		t.Fatal(err)
	}
	d.Query = func(context.Context, bool, string, []byte, ...string) ([]byte, error) {
		return []byte(strings.Replace(brewInventoryFixture, `"formula":["sample/tap/shared"]`, `"formula":[]`, 1)), nil
	}
	if err := d.approveInventory(context.Background(), []string{before.Inventory}); err == nil {
		t.Fatal("departing outside consumer silently broadened an approved removal")
	}
	if _, err := d.Remove(context.Background(), resource, receipt); err == nil || !strings.Contains(err.Error(), "approved inventory") {
		t.Fatal("native removal bypassed changed approval", err)
	}
}

func TestBrewApprovalScopesOwnershipAndConsumers(t *testing.T) {
	ledger := brewLedger{Schema: 1, Cellar: "/fixture/Cellar", Roots: map[string]brewOwnership{"tool.first": {Name: "first", Source: "first", CreatedBy: strings.Repeat("a", 64)}}, Pool: map[string]brewOwnership{"shared": {Name: "shared", Source: "shared", CreatedBy: strings.Repeat("a", 64)}}}
	installed := map[string]nativePackage{
		"first":  {Name: "first", Source: "first", Version: "1", Healthy: true, Dependencies: []string{"shared"}},
		"shared": {Name: "shared", Source: "shared", Version: "1", Healthy: true, Automatic: true},
		"other":  {Name: "other", Source: "other", Version: "1", Healthy: true},
	}
	before := brewProviderSnapshot(ledger, installed, nil)
	for _, change := range []string{"unrelated-version", "root-update", "pool-version", "manual-promotion", "pin", "source", "new-consumer", "missing-pool-member", "new-ownership"} {
		t.Run(change, func(t *testing.T) {
			state := ledger
			state.Pool = maps.Clone(ledger.Pool)
			current := maps.Clone(installed)
			pkg := current["shared"]
			allowed := map[string]bool{"first": true}
			switch change {
			case "unrelated-version":
				other := current["other"]
				other.Version = "2"
				current["other"] = other
			case "root-update":
				root := current["first"]
				root.Version = "2"
				current["first"] = root
			case "pool-version":
				pkg.Version = "2"
			case "manual-promotion":
				pkg.Automatic = false
				allowed["shared"] = true // Even a touched keg cannot change this.
			case "pin":
				pkg.Held = true
				allowed["shared"] = true
			case "source":
				pkg.Source = "another/tap/shared"
				allowed["shared"] = true
			case "new-consumer":
				current["cask/app"] = nativePackage{Name: "cask/app", Version: "1", Dependencies: []string{"shared"}}
			case "missing-pool-member":
				delete(current, "shared")
			case "new-ownership":
				state.Pool["other"] = brewOwnership{Name: "other", Source: "other", CreatedBy: strings.Repeat("b", 64)}
			}
			if change != "missing-pool-member" {
				current["shared"] = pkg
			}
			after := brewProviderSnapshot(state, current, nil)
			err := verifyBrewMutation(before, after, allowed)
			if wantSuccess := change == "unrelated-version" || change == "root-update"; (err == nil) != wantSuccess {
				t.Fatal("wrong approval boundary", change, err)
			}
			if change == "pool-version" {
				allowed["shared"] = true
				if err := verifyBrewMutation(before, after, allowed); err != nil {
					t.Fatal("operation-recorded dependency update was rejected", err)
				}
			}
		})
	}
}
