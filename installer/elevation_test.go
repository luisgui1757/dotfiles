package installer

import (
	"context"
	"errors"
	"slices"
	"testing"
)

// Only providers that authenticate in the foreground need sudo up front.
// Machine scope alone, as for Homebrew formulae, never prompts.
func TestNativeElevationFollowsForegroundProviders(t *testing.T) {
	catalog := &Catalog{Schema: 1, Resources: []Resource{
		{ID: "package", Name: "Package", Action: "native", Scope: "machine", Bindings: map[string]Binding{"linux": {Provider: "apt", Package: "package"}}},
		{ID: "tool", Name: "Tool", Action: "archive", Bindings: map[string]Binding{"linux": {Provider: "archive", Package: "tool"}}},
	}}
	if err := catalog.Validate(); err != nil {
		t.Fatal(err)
	}
	calls := 0
	d := &NativeDriver{Catalog: catalog, Context: linux, APT: &LinuxAPTDriver{Authenticate: func(context.Context) error { return nil }}, Archives: &ArchiveDriver{},
		Elevation: func(context.Context) (func() error, error) { calls++; return noElevation, nil }}
	plan := func(id, action string) Plan {
		return Plan{Operations: []Operation{{Resource: id, Action: action, Privileged: true}}}
	}
	for _, test := range []struct {
		plan Plan
		want int
	}{
		{plan("tool", "install"), 0},
		{plan("package", "keep"), 0},
		{plan("package", "install"), 1},
		{plan("package", "remove"), 2},
	} {
		if _, err := d.Elevate(context.Background(), test.plan); err != nil || calls != test.want {
			t.Fatal("unexpected elevation", test.plan.Operations, calls, err)
		}
	}
	d.APT.Authenticate = nil // Running as root, APT needs no sudo.
	if _, err := d.Elevate(context.Background(), plan("package", "install")); err != nil || calls != 2 {
		t.Fatal("root APT requested sudo", calls, err)
	}
}

type elevatingDriver struct {
	*fileDriver
	events *[]string
}

func (d elevatingDriver) Elevate(context.Context, Plan) (func() error, error) {
	*d.events = append(*d.events, "elevate")
	return func() error { *d.events = append(*d.events, "release"); return nil }, nil
}

func (d elevatingDriver) Apply(ctx context.Context, r Resource, op Operation, receipt Receipt) (Observation, error) {
	*d.events = append(*d.events, "apply:"+r.ID)
	return d.fileDriver.Apply(ctx, r, op, receipt)
}

func TestExecuteElevatesOnceAfterReproducingThePlan(t *testing.T) {
	c, d := engineFixture(t)
	events := []string{}
	driver := elevatingDriver{fileDriver: d, events: &events}
	request := approve(t, c, d, Request{Schema: 1, Mode: "apply", Selected: []string{"editor"}})
	stale := request
	stale.ExpectedPlan = "0000000000000000000000000000000000000000000000000000000000000000"
	if _, err := Execute(context.Background(), c, linux, "source", d.home, d.statePath, stale, driver); !errors.Is(err, ErrPlanChanged) || len(events) != 0 {
		t.Fatal("a stale approval prompted or mutated", events, err)
	}
	if _, err := Execute(context.Background(), c, linux, "source", d.home, d.statePath, request, driver); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(events, []string{"elevate", "apply:node", "apply:plugins", "release"}) {
		t.Fatal("elevation did not span exactly the approved work", events)
	}
}
