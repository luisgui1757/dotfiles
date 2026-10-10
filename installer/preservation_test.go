package installer

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Model a provider operation with an unintended effect outside its requested
// resource. It changes real files; the ordinary read-only probe sees the damage.
type collateralDriver struct {
	*fileDriver
	removeNode bool
}

func (d *collateralDriver) damageNode() error {
	path := filepath.Join(d.dir, "node")
	if d.removeNode {
		return os.Remove(path)
	}
	return os.WriteFile(path, []byte("unapproved replacement"), 0600)
}

func (d *collateralDriver) Remove(ctx context.Context, r Resource, receipt Receipt) (Observation, error) {
	o, err := d.fileDriver.Remove(ctx, r, receipt)
	if err == nil && r.ID == "plugins" {
		err = d.damageNode()
	}
	return o, err
}

func (d *collateralDriver) Apply(ctx context.Context, r Resource, op Operation, receipt Receipt) (Observation, error) {
	o, err := d.fileDriver.Apply(ctx, r, op, receipt)
	if err == nil && r.ID == "plugins" {
		err = d.damageNode()
	}
	return o, err
}

func TestRemovalCannotReportReadyAfterDamagingARetainedResource(t *testing.T) {
	for _, remove := range []bool{false, true} {
		for _, selected := range [][]string{{}, {"agent"}} {
			c, d := engineFixture(t)
			initial := approve(t, c, d, Request{Schema: 1, Mode: "apply", Selected: []string{"editor"}})
			if _, err := applyFixture(c, d, initial); err != nil {
				t.Fatal(err)
			}
			request := approve(t, c, d, Request{Schema: 1, Mode: "apply", Selected: selected})
			result, err := Execute(context.Background(), c, linux, "source", d.home, d.statePath, request, &collateralDriver{d, remove})
			if err == nil || result.Status == "ready" {
				t.Fatalf("collateral change reported success (remove=%v, selected=%v): %+v %v", remove, selected, result, err)
			}
			state, loadErr := LoadState(d.statePath, d.home)
			if loadErr != nil || state.Transaction == nil || state.Transaction.Error == "" {
				t.Fatalf("collateral damage lost recovery intent: %+v %v", state, loadErr)
			}
		}
	}
}

func TestLaterInstallCannotSilentlyReplaceAnAlreadyVerifiedResource(t *testing.T) {
	c, d := engineFixture(t)
	request := approve(t, c, d, Request{Schema: 1, Mode: "apply", Selected: []string{"editor"}})
	result, err := Execute(context.Background(), c, linux, "source", d.home, d.statePath, request, &collateralDriver{fileDriver: d})
	if err == nil || result.Status == "ready" || !strings.Contains(err.Error(), "preservation") {
		t.Fatalf("healthy replacement evaded final identity verification: %+v %v", result, err)
	}
}

type retainedPendingDriver struct{ *fileDriver }

func (d *retainedPendingDriver) Observe(ctx context.Context, r Resource, receipt Receipt) (Observation, error) {
	o, err := d.fileDriver.Observe(ctx, r, receipt)
	if r.ID == "node" {
		o.Pending = "external ownership review remains pending"
	}
	return o, err
}

func TestRetainedExistingPendingStateIsNotMistakenForCollateralDamage(t *testing.T) {
	c, d := engineFixture(t)
	if err := os.WriteFile(filepath.Join(d.dir, "node"), []byte("unchanged user runtime"), 0600); err != nil {
		t.Fatal(err)
	}
	driver := &retainedPendingDriver{d}
	r, _ := c.Resource("node")
	o, err := driver.Observe(context.Background(), r, Receipt{})
	if err != nil {
		t.Fatal(err)
	}
	state := State{Schema: 1, Context: linux, Target: d.home, Receipts: map[string]Receipt{"node": {Ownership: "reused", Before: o, After: o}}}
	if err := SaveState(d.statePath, state); err != nil {
		t.Fatal(err)
	}
	request := Request{Schema: 1, Mode: "apply", Selected: []string{}}
	observed, err := Observe(context.Background(), c, linux, state, request, driver)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := PlanChanges(c, linux, state, request, observed, "source")
	if err != nil {
		t.Fatal(err)
	}
	request.ExpectedPlan = plan.ID
	result, err := Execute(context.Background(), c, linux, "source", d.home, d.statePath, request, driver)
	if err != nil || result.Status != "ready" || len(d.calls) != 0 {
		t.Fatalf("unchanged unselected pending state looked like new damage: %+v %v", result, err)
	}
}
