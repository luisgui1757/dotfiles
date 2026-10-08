package installer

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func bootstrapFixture(t *testing.T) (Controller, *fileDriver) {
	t.Helper()
	c, d := controllerFixture(t)
	for i := range c.Catalog.Resources {
		if c.Catalog.Resources[i].ID == "node" {
			c.Catalog.Resources[i].Retain = true
			c.Catalog.Resources[i].ReplanAfter = true
		}
	}
	if err := c.Catalog.Validate(); err != nil {
		t.Fatal(err)
	}
	return c, d
}

func TestBootstrapRequiresFreshApprovalBeforeConsumerMutation(t *testing.T) {
	c, d := bootstrapFixture(t)
	ctx := context.Background()
	request := Request{Schema: 1, Mode: "apply", Selected: []string{"editor"}}
	preview, err := c.Dispatch(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Dir(d.statePath)); !os.IsNotExist(err) {
		t.Fatal("preview initialized state", err)
	}
	request.ExpectedPlan = preview.Plan.ID
	result, err := c.Dispatch(ctx, request)
	if err != nil || result.Status != "needs-action" || !strings.Contains(result.Message, "Resume") || !slices.Equal(d.calls, []string{"apply:node"}) {
		t.Fatal(result, err, d.calls)
	}
	state, err := LoadState(d.statePath, d.home)
	if err != nil || len(state.Selected) != 0 || state.Generation != 0 || state.Transaction == nil || state.Transaction.Plan.ID != preview.Plan.ID || state.Transaction.Next != 1 || state.Transaction.InFlight != "" || state.Receipts["node"].Ownership != "created" {
		t.Fatal("bootstrap lost original intent or claimed full readiness", state, err)
	}
	if _, err := c.Dispatch(ctx, request); err == nil {
		t.Fatal("old approval continued past bootstrap")
	}
	request = Request{Schema: 1, Mode: "apply", Retry: true}
	preview, err = c.Dispatch(ctx, request)
	if err != nil || preview.Plan.ID == result.Plan.ID {
		t.Fatal("bootstrap reused the old approval", err)
	}
	request.ExpectedPlan = preview.Plan.ID
	result, err = c.Dispatch(ctx, request)
	if err != nil || result.Status != "ready" || !slices.Equal(d.calls, []string{"apply:node", "apply:plugins"}) {
		t.Fatal(result, err, d.calls)
	}
	state, err = LoadState(d.statePath, d.home)
	if err != nil || state.Transaction != nil || !slices.Equal(state.Selected, []string{"editor"}) {
		t.Fatal(state, err)
	}
}

func TestBootstrapRetryAfterUpdateDoesNotRepeatCompletedInfrastructure(t *testing.T) {
	c, d := bootstrapFixture(t)
	for _, request := range []Request{
		{Schema: 1, Mode: "apply", Selected: []string{"editor"}},
		{Schema: 1, Mode: "apply", Retry: true},
		{Schema: 1, Mode: "update"},
		{Schema: 1, Mode: "update", Retry: true},
	} {
		preview, err := c.Dispatch(context.Background(), request)
		if err != nil {
			t.Fatal(err)
		}
		request.ExpectedPlan = preview.Plan.ID
		result, err := c.Dispatch(context.Background(), request)
		want := "needs-action"
		if request.Retry {
			want = "ready"
		}
		if err != nil || result.Status != want {
			t.Fatal(result, err)
		}
	}
	if !slices.Equal(d.calls, []string{"apply:node", "apply:plugins", "apply:node", "apply:plugins"}) {
		t.Fatal("retry repeated infrastructure", d.calls)
	}
}

func TestCatalogLegacyShapeDoesNotInventBootstrapBoundary(t *testing.T) {
	var c Catalog
	if err := Decode([]byte(`{"schema":1,"resources":[{"id":"manager","name":"Manager","action":"manager","retain":true}]}`), &c); err != nil {
		t.Fatal(err)
	}
	if err := c.Validate(); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(c)
	if err != nil || strings.Contains(string(data), "replan_after") {
		t.Fatal("legacy catalog shape changed", string(data), err)
	}
	for i := range c.Resources {
		c.Resources[i].ReplanAfter, c.Resources[i].Retain = true, false
	}
	if c.Validate() == nil {
		t.Fatal("removable resource accepted as bootstrap boundary")
	}
}
