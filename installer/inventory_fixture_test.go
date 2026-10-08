package installer

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// Individual resource fingerprints protect known packages. This independent
// inventory represents provider inputs outside those resource identities.
type fileInventoryDriver struct{ *fileDriver }

func (d *fileInventoryDriver) ObserveInventory(ctx context.Context, c *Catalog, target Context, state State) (map[string]Observation, error) {
	data, err := os.ReadFile(filepath.Join(d.dir, "external"))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	hash, err := digest(data)
	if err != nil {
		return nil, err
	}
	observed := map[string]Observation{}
	for _, r := range c.Resources {
		if _, bound := r.Bindings[target.OS]; !bound {
			continue
		}
		o, err := d.Observe(ctx, r, state.Receipts[r.ID])
		if err != nil {
			return nil, err
		}
		o.Inventory = hash
		observed[r.ID] = o
	}
	return observed, nil
}

func (d *fileInventoryDriver) VerifyInventory(ctx context.Context, c *Catalog, target Context, state State, plan Plan) error {
	observed, err := d.ObserveInventory(ctx, c, target, state)
	if err != nil {
		return err
	}
	for _, o := range observed {
		if !slices.Contains(plan.Inventories, o.Inventory) {
			return errors.New("protected provider inputs changed")
		}
	}
	return nil
}

func inventoryFixture(t *testing.T) (*Catalog, *fileInventoryDriver) {
	t.Helper()
	c, d := engineFixture(t)
	c.Resources = append(c.Resources,
		Resource{ID: "infra.packages", Name: "Package infrastructure", Action: "infra", Retain: true},
		Resource{ID: "fzf", Name: "fzf", Action: "fzf", Shared: true, Requires: []string{"infra.packages"}, Bindings: map[string]Binding{"linux": {Provider: "archive", Package: "fzf"}}},
	)
	for i := range c.Resources {
		r := &c.Resources[i]
		if r.ID == "node" {
			r.Requires = []string{"infra.packages"}
			r.Bindings = map[string]Binding{"linux": {Provider: "archive", Package: "node"}}
		}
		if r.ID == "plugins" {
			r.Requires = append(r.Requires, "fzf")
		}
	}
	if err := c.Validate(); err != nil {
		t.Fatal(err)
	}
	return c, &fileInventoryDriver{d}
}

func approveInventory(t *testing.T, c *Catalog, d *fileInventoryDriver, request Request) Request {
	t.Helper()
	state, err := LoadState(d.statePath, d.home)
	if err != nil {
		t.Fatal(err)
	}
	observed, err := Observe(context.Background(), c, linux, state, request, d)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := PlanChanges(c, linux, state, request, observed, "source")
	if err != nil {
		t.Fatal(err)
	}
	request.ExpectedPlan = plan.ID
	return request
}

func applyInventoryFixture(t *testing.T, c *Catalog, d *fileInventoryDriver, request Request) (Result, error) {
	t.Helper()
	request = approveInventory(t, c, d, request)
	return Execute(context.Background(), c, linux, "source", d.home, d.statePath, request, d)
}
