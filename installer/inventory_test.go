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

func TestInventoryPreservesUnrecordedPackagesOutsideTheSelection(t *testing.T) {
	c, d := inventoryFixture(t)
	path := filepath.Join(d.dir, "fzf")
	if err := os.WriteFile(path, []byte("pre-existing unselected package"), 0600); err != nil {
		t.Fatal(err)
	}
	result, err := applyInventoryFixture(t, c, d, Request{Schema: 1, Mode: "apply", Selected: []string{"agent"}})
	if err != nil || result.Status != "ready" || operation(t, result.Plan, "fzf").Action != "retain" {
		t.Fatalf("discovery omitted an unselected provider member: %+v %v", result, err)
	}
	state, err := LoadState(d.statePath, d.home)
	if err != nil || state.Receipts["fzf"].Ownership != "reused" {
		t.Fatalf("discovery claimed or lost a pre-existing member: %+v %v", state, err)
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "pre-existing unselected package" {
		t.Fatal("selection removed or changed an unselected provider member")
	}
}

func TestWiderProviderDriftInvalidatesApprovalBeforeMutation(t *testing.T) {
	c, d := inventoryFixture(t)
	request := approveInventory(t, c, d, Request{Schema: 1, Mode: "apply", Selected: []string{"editor"}})
	if err := os.WriteFile(filepath.Join(d.dir, "external"), []byte("unrelated package added after preview"), 0600); err != nil {
		t.Fatal(err)
	}
	result, err := Execute(context.Background(), c, linux, "source", d.home, d.statePath, request, d)
	if err == nil || result.Status == "ready" || !strings.Contains(err.Error(), "changed") || len(d.calls) != 0 {
		t.Fatalf("stale whole-provider approval allowed mutation: %+v %v", result, err)
	}
}

type collateralInventoryDriver struct{ *fileInventoryDriver }

func (d *collateralInventoryDriver) Apply(ctx context.Context, r Resource, op Operation, receipt Receipt) (Observation, error) {
	o, err := d.fileDriver.Apply(ctx, r, op, receipt)
	if err == nil && r.ID == "plugins" {
		err = os.WriteFile(filepath.Join(d.dir, "external"), []byte("unapproved external change"), 0600)
	}
	return o, err
}

func TestFinalInventoryDetectsDamageOutsideKnownResourceIdentities(t *testing.T) {
	c, d := inventoryFixture(t)
	request := approveInventory(t, c, d, Request{Schema: 1, Mode: "apply", Selected: []string{"editor"}})
	result, err := Execute(context.Background(), c, linux, "source", d.home, d.statePath, request, &collateralInventoryDriver{d})
	if err == nil || result.Status == "ready" || !strings.Contains(err.Error(), "provider inventory") {
		t.Fatalf("selected health hid changed protected provider inputs: %+v %v", result, err)
	}
	state, err := LoadState(d.statePath, d.home)
	if err != nil || state.Transaction == nil || state.Transaction.Error == "" {
		t.Fatalf("provider-wide failure lost recovery intent: %+v %v", state, err)
	}
}

type unmappedInventoryDriver struct{ *fileInventoryDriver }

func (d *unmappedInventoryDriver) ObserveInventory(ctx context.Context, c *Catalog, target Context, state State) (map[string]Observation, error) {
	observed, err := d.fileInventoryDriver.ObserveInventory(ctx, c, target, state)
	if err != nil {
		return nil, err
	}
	observed["unknown-package"] = observed["node"]
	return observed, nil
}

func TestInventoryCannotSilentlyDropAnUnmappedMember(t *testing.T) {
	c, d := inventoryFixture(t)
	state, err := LoadState(d.statePath, d.home)
	if err != nil {
		t.Fatal(err)
	}
	_, err = Observe(context.Background(), c, linux, state, Request{Schema: 1, Mode: "apply", Selected: []string{"editor"}}, &unmappedInventoryDriver{d})
	if err == nil || !strings.Contains(err.Error(), "unmapped provider member") || len(d.calls) != 0 {
		t.Fatalf("unmapped member was ignored: %v", err)
	}
}

type driftBeforePackageDriver struct{ *fileInventoryDriver }

func (d *driftBeforePackageDriver) Apply(ctx context.Context, r Resource, op Operation, receipt Receipt) (Observation, error) {
	o, err := d.fileDriver.Apply(ctx, r, op, receipt)
	if err == nil && r.ID == "infra.packages" {
		err = os.WriteFile(filepath.Join(d.dir, "external"), []byte("provider changed while prerequisite installed"), 0600)
	}
	return o, err
}

func TestProviderRechecksWiderInventoryAfterPrerequisiteWork(t *testing.T) {
	c, d := inventoryFixture(t)
	request := approveInventory(t, c, d, Request{Schema: 1, Mode: "apply", Selected: []string{"editor"}})
	result, err := Execute(context.Background(), c, linux, "source", d.home, d.statePath, request, &driftBeforePackageDriver{d})
	if err == nil || result.Status == "ready" || !slices.Equal(d.calls, []string{"apply:infra.packages"}) {
		t.Fatalf("package mutation incorporated an unapproved wider provider change: %+v %v", result, err)
	}
}

func TestInventoryBindsApprovalEvenWithoutPackageOperations(t *testing.T) {
	c, d := inventoryFixture(t)
	request := approveInventory(t, c, d, Request{Schema: 1, Mode: "apply", Selected: []string{}})
	if err := os.WriteFile(filepath.Join(d.dir, "external"), []byte("changed foreign input"), 0600); err != nil {
		t.Fatal(err)
	}
	changed := approveInventory(t, c, d, Request{Schema: 1, Mode: "apply", Selected: []string{}})
	if request.ExpectedPlan == changed.ExpectedPlan {
		t.Fatal("an empty package selection omitted complete inventory from its approval")
	}
}

type unusedDependencyDriver struct{ *fileInventoryDriver }

func (d *unusedDependencyDriver) Observe(ctx context.Context, r Resource, receipt Receipt) (Observation, error) {
	if r.ID == "unused-prerequisite" {
		return Observation{}, errors.New("absent unselected tool must not discover its native prerequisite")
	}
	return d.fileInventoryDriver.Observe(ctx, r, receipt)
}

func TestAbsentUnselectedInventoryMembersDoNotDiscoverTheirPrerequisites(t *testing.T) {
	c, d := inventoryFixture(t)
	c.Resources = append(c.Resources,
		Resource{ID: "unused-prerequisite", Name: "Unused native prerequisite", Action: "unused"},
		Resource{ID: "unused-tool", Name: "Absent unselected package", Action: "unused", Requires: []string{"infra.packages", "unused-prerequisite"}, Bindings: map[string]Binding{"linux": {Provider: "archive", Package: "unused"}}})
	if err := c.Validate(); err != nil {
		t.Fatal(err)
	}
	state, err := LoadState(d.statePath, d.home)
	if err != nil {
		t.Fatal(err)
	}
	observed, err := Observe(context.Background(), c, linux, state, Request{Schema: 1, Mode: "apply", Selected: []string{"agent"}}, &unusedDependencyDriver{d})
	if err != nil {
		t.Fatal(err)
	}
	if observed["unused-tool"].Inventory == "" {
		t.Fatal("complete inventory was lost")
	}
	if _, found := observed["unused-prerequisite"]; found {
		t.Fatal("unneeded prerequisite entered discovery")
	}
}
