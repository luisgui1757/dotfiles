package installer

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

type adoptingFileDriver struct{ *fileDriver }

func (d adoptingFileDriver) Observe(ctx context.Context, r Resource, receipt Receipt) (Observation, error) {
	o, err := d.fileDriver.Observe(ctx, r, receipt)
	o.Adoptable = r.ID == "plugins" && o.Present
	return o, err
}

func (d adoptingFileDriver) Apply(ctx context.Context, r Resource, op Operation, receipt Receipt) (Observation, error) {
	if op.Action == "adopt" {
		baseline, err := os.ReadFile(filepath.Join(d.dir, r.ID))
		if err != nil {
			return Observation{}, err
		}
		if err := saveDocument(filepath.Join(filepath.Dir(d.statePath), receipt.Recovery, "baseline.json"), baseline); err != nil {
			return Observation{}, err
		}
	}
	return d.fileDriver.Apply(ctx, r, op, receipt)
}

func (d adoptingFileDriver) Remove(ctx context.Context, r Resource, receipt Receipt) (Observation, error) {
	if !receipt.Before.Present {
		return d.fileDriver.Remove(ctx, r, receipt)
	}
	if !receipt.Adopted {
		return Observation{}, errors.New("pre-existing file was not adopted")
	}
	data, err := readDocument(filepath.Join(filepath.Dir(d.statePath), receipt.Recovery, "baseline.json"))
	var baseline []byte
	if err == nil {
		err = Decode(data, &baseline)
	}
	if err != nil {
		return Observation{}, err
	}
	if err := os.WriteFile(filepath.Join(d.dir, r.ID), baseline, 0600); err != nil {
		return Observation{}, err
	}
	return d.Observe(ctx, r, receipt)
}

func adoptionFixture(t *testing.T) (Controller, *fileDriver) {
	t.Helper()
	c, d := controllerFixture(t)
	c.Driver = adoptingFileDriver{d}
	if err := os.WriteFile(filepath.Join(d.dir, "plugins"), []byte("original user configuration"), 0600); err != nil {
		t.Fatal(err)
	}
	return c, d
}

func dispatchApproved(t *testing.T, c Controller, request Request) Result {
	t.Helper()
	preview, err := c.Dispatch(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	request.ExpectedPlan = preview.Plan.ID
	result, err := c.Dispatch(context.Background(), request)
	if err != nil || result.Status != "ready" {
		t.Fatal(result, err)
	}
	return result
}

func TestExplicitAdoptionPreservesAndRestoresExistingBaseline(t *testing.T) {
	c, d := adoptionFixture(t)
	preview, err := c.Dispatch(context.Background(), Request{Schema: 1, Mode: "apply", Selected: []string{"editor"}})
	if err != nil {
		t.Fatal(err)
	}
	for _, op := range preview.Plan.Operations {
		if op.Resource == "plugins" && op.Action != "keep" {
			t.Fatal("existing healthy resource was adopted without consent", op)
		}
	}
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"editor"}, Adopt: []string{"plugins"}})
	state, err := LoadState(c.StatePath, c.Home)
	receipt := state.Receipts["plugins"]
	if err != nil || !receipt.Adopted || !receipt.Before.Present || receipt.Ownership != "created" {
		t.Fatal("adoption lost its original baseline", receipt, err)
	}
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{}, RemoveShared: []string{"node"}})
	data, err := os.ReadFile(filepath.Join(d.dir, "plugins"))
	if err != nil || string(data) != "original user configuration" {
		t.Fatal("uninstall did not restore adopted user bytes", string(data), err)
	}
}

func TestAdoptionRefusesUnavailableProviderUnselectedAndMaintenanceRequests(t *testing.T) {
	c, _ := adoptionFixture(t)
	for _, request := range []Request{
		{Schema: 1, Mode: "apply", Selected: []string{"editor"}, Adopt: []string{"node"}},
		{Schema: 1, Mode: "apply", Selected: []string{}, Adopt: []string{"plugins"}},
		{Schema: 1, Mode: "update", Adopt: []string{"plugins"}},
		{Schema: 1, Mode: "repair", Adopt: []string{"plugins"}},
	} {
		if _, err := c.Dispatch(context.Background(), request); err == nil {
			t.Fatal("unapproved adoption was accepted", request)
		}
	}
}

func TestInteractiveAdoptionIsAnExplicitUnselectedChoice(t *testing.T) {
	c, d := adoptionFixture(t)
	u := &scriptedInteraction{t: t, answers: [][]string{{"editor"}, {"plugins"}}, accept: true}
	result, err := c.Run(context.Background(), u)
	if err != nil || result.Status != "ready" || len(u.menus) != 2 || len(u.answers) != 0 {
		t.Fatal(result, err, u.menus)
	}
	data, err := os.ReadFile(filepath.Join(d.dir, "plugins"))
	if err != nil || string(data) != "installed" {
		t.Fatal("approved adoption did not apply", string(data), err)
	}
}
