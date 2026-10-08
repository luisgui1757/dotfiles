package installer

import (
	"encoding/json"
	"slices"
	"testing"
)

func TestExplicitKeepClearSurvivesJSONRoundTrip(t *testing.T) {
	state, observed := installed()
	state.Keep = []string{"plugins"}
	for _, keep := range [][]string{nil, {}} {
		request := Request{Schema: 1, Mode: "apply", Selected: []string{}, Keep: keep}
		data, err := json.Marshal(request)
		if err != nil {
			t.Fatal(err)
		}
		var decoded Request
		if err := Decode(data, &decoded); err != nil {
			t.Fatal(err)
		}
		p, err := PlanChanges(testCatalog(t), linux, state, decoded, observed, "source")
		if err != nil {
			t.Fatal(err)
		}
		want := "keep"
		if keep != nil {
			want = "remove"
		}
		if operation(t, p, "plugins").Action != want {
			t.Fatalf("round-trip changed nil/empty Keep semantics: %s", data)
		}
	}
}

func TestMaintenancePreservesRecordedSelectionsAndKeep(t *testing.T) {
	for _, mode := range []string{"update", "repair", "check"} {
		t.Run(mode, func(t *testing.T) {
			state, observed := installed()
			state.Selected, state.Keep = []string{"agent"}, []string{"plugins"}
			p, err := PlanChanges(testCatalog(t), linux, state, Request{Schema: 1, Mode: mode}, observed, "source")
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(p.Selected, state.Selected) || !slices.Equal(p.Keep, state.Keep) {
				t.Fatal("maintenance changed selection")
			}
			for _, op := range p.Operations {
				if op.Action == "remove" || op.Action == "forget" {
					t.Fatalf("maintenance removed %s", op.Resource)
				}
			}
			if _, err := PlanChanges(testCatalog(t), linux, state, Request{Schema: 1, Mode: mode, Selected: []string{}}, observed, "source"); err == nil {
				t.Fatal("maintenance accepted selection changes")
			}
		})
	}
}

func TestKeepPersistsUntilExplicitlyCleared(t *testing.T) {
	state, observed := installed()
	state.Keep = []string{"plugins"}
	p, err := PlanChanges(testCatalog(t), linux, state, Request{Schema: 1, Mode: "apply", Selected: []string{}}, observed, "source")
	if err != nil || operation(t, p, "plugins").Action != "keep" {
		t.Fatalf("lost Keep: %+v %v", p, err)
	}
	p, err = PlanChanges(testCatalog(t), linux, state, Request{Schema: 1, Mode: "apply", Keep: []string{}, Selected: []string{}}, observed, "source")
	if err != nil || operation(t, p, "plugins").Action != "remove" {
		t.Fatalf("explicit clear failed: %+v %v", p, err)
	}
}

func TestUpdateCannotTakeOverChangedProvider(t *testing.T) {
	state, observed := installed()
	o := observed["node"]
	o.Provider = "outside-provider"
	observed["node"] = o
	p, err := PlanChanges(testCatalog(t), linux, state, Request{Schema: 1, Mode: "update"}, observed, "source")
	if err != nil || operation(t, p, "node").Action != "pending" || operation(t, p, "plugins").Action != "pending" {
		t.Fatalf("update took over drift: %+v %v", p, err)
	}
}

func TestRepairDoesNotAdoptUnhealthyExternalTools(t *testing.T) {
	state, observed := installed()
	r, o := state.Receipts["node"], observed["node"]
	r.Ownership, o.Healthy = "reused", false
	state.Receipts["node"], observed["node"] = r, o
	p, err := PlanChanges(testCatalog(t), linux, state, Request{Schema: 1, Mode: "repair"}, observed, "source")
	if err != nil || operation(t, p, "node").Action != "pending" {
		t.Fatalf("repair took over external package: %+v %v", p, err)
	}
}

func TestCatalogRejectsWindowsHostOwnershipInLinuxGraph(t *testing.T) {
	c := testCatalog(t)
	c.Resources[3].Scope = "host"
	if err := c.Validate(); err == nil {
		t.Fatal("catalog still permits cross-host ownership")
	}
}

func TestLinuxChoicesDoNotDependOnSessionFacts(t *testing.T) {
	c, err := DefaultCatalog()
	if err != nil {
		t.Fatal(err)
	}
	for _, capability := range []string{"ghostty", "vscode"} {
		if _, err := c.Closure([]string{capability}, linux); err != nil {
			t.Fatal("Linux GUI selection depends on a graphical session", err)
		}
	}
	for _, field := range []string{`"wsl":true`, `"wsl_gui":true`, `"distro":"ubuntu"`} {
		var context Context
		if err := Decode([]byte(`{"os":"linux","arch":"amd64",`+field+`}`), &context); err == nil {
			t.Fatal("persisted a session fact as target identity", field)
		}
	}
}

func TestFontsBelongToRenderersAndClipboardDoesNotBlockPluginSync(t *testing.T) {
	c, err := DefaultCatalog()
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"starship", "lsd", "tmux", "nvim.sync"} {
		closure, err := c.Closure([]string{id}, linux)
		if err != nil {
			t.Fatal(err)
		}
		if slices.Contains(closure, "tool.font") || id == "nvim.sync" && slices.Contains(closure, "tool.clipboard") {
			t.Fatal("unrelated dependency blocks a CLI or plugin provisioning", id, closure)
		}
	}
	for _, id := range []string{"ghostty", "vscode"} {
		closure, err := c.Closure([]string{id}, linux)
		if err != nil || !slices.Contains(closure, "tool.font") {
			t.Fatal("renderer lost its font", id, closure, err)
		}
	}
	closure, err := c.Closure([]string{"neovim"}, linux)
	if err != nil || !slices.Contains(closure, "tool.clipboard") {
		t.Fatal("Neovim lost clipboard provisioning", closure, err)
	}
}

func TestUnsupportedArchitecturesFailBeforePlanning(t *testing.T) {
	for _, ctx := range []Context{{OS: "darwin", Arch: "amd64"}, {OS: "windows", Arch: "arm64"}, {OS: "linux", Arch: "386"}} {
		if _, err := PlanChanges(testCatalog(t), ctx, State{}, Request{Schema: 1, Mode: "apply", Selected: []string{}}, nil, "source"); err == nil {
			t.Fatalf("accepted unaudited target %+v", ctx)
		}
	}
}
