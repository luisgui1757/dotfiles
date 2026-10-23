package installer

import (
	"encoding/json"
	"math/rand/v2"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestCIToolchainMatchesModule(t *testing.T) {
	module, err := os.ReadFile("go.mod")
	if err != nil {
		t.Fatal(err)
	}
	version := ""
	for _, line := range strings.Split(string(module), "\n") {
		if strings.HasPrefix(line, "go ") {
			version = strings.TrimPrefix(line, "go ")
		}
	}
	for _, name := range []string{"installer-engine.yml"} {
		workflow, err := os.ReadFile(filepath.Join("../.github/workflows", name))
		if err != nil {
			t.Fatal(err)
		}
		if version == "" || !strings.Contains(string(workflow), "          go-version: "+version+"\n") {
			t.Fatalf("%s must use the Go version declared by the module", name)
		}
	}
}

func testCatalog(t *testing.T) *Catalog {
	t.Helper()
	c := &Catalog{Schema: 1, Resources: []Resource{
		{ID: "editor", Name: "Editor", Capability: true, Requires: []string{"plugins"}},
		{ID: "agent", Name: "Agent", Capability: true, Requires: []string{"node"}},
		{ID: "plugins", Name: "Plugins", Action: "plugins", Requires: []string{"node"}},
		{ID: "node", Name: "Node", Action: "node", Shared: true},
	}}
	if err := c.Validate(); err != nil {
		t.Fatal(err)
	}
	return c
}

var linux = Context{OS: "linux", Arch: "amd64"}

func TestRetirementPrivilegeFollowsVerifiedSteps(t *testing.T) {
	for _, test := range []struct {
		action     string
		privileged bool
		want       bool
	}{{"retire", false, false}, {"retire", true, true}, {"archive", false, true}} {
		t.Run(test.action+"-"+map[bool]string{true: "privileged", false: "user"}[test.privileged], func(t *testing.T) {
			catalog := &Catalog{Schema: 1, Resources: []Resource{{ID: "item", Name: "Item", Capability: true, Action: test.action, Scope: "machine"}}}
			observed := map[string]Observation{"item": {Scope: "machine", Privileged: test.privileged}}
			plan, err := PlanChanges(catalog, linux, State{}, Request{Schema: 1, Mode: "apply", Selected: []string{"item"}}, observed, "source")
			if err != nil || len(plan.Operations) != 1 || plan.Operations[0].Privileged != test.want {
				t.Fatal("preview privilege differs from the actual retirement steps or ordinary machine policy", plan, err)
			}
		})
	}
}

func TestRetirementRecoveryCountsOnlyRemainingPrivilegedSteps(t *testing.T) {
	catalog := &Catalog{Schema: 1, Resources: []Resource{{ID: "retire.fixture", Name: "Retire fixture", Capability: true, Action: "retire", Scope: "machine"}}}
	observed := map[string]Observation{"retire.fixture": {Scope: "machine", Privileged: true}}
	request := Request{Schema: 1, Mode: "apply", Selected: []string{"retire.fixture"}}
	original, err := PlanChanges(catalog, linux, State{}, request, observed, "source")
	if err != nil {
		t.Fatal(err)
	}
	opID, err := digest(struct{ Plan, Resource, Action string }{original.ID, "retire.fixture", "install"})
	if err != nil {
		t.Fatal(err)
	}
	state := State{Schema: 1, Context: linux, Transaction: &Transaction{Plan: original, InFlight: "retire.fixture"},
		Receipts: map[string]Receipt{"retire.fixture": {Status: "in-progress", Ownership: "uncertain", OperationID: opID}}}
	observed["retire.fixture"] = Observation{Scope: "machine", Pending: "remaining user move", ResourceResume: &ResourceResume{Operation: opID, Token: strings.Repeat("a", 64)}}
	plan, err := PlanChanges(catalog, linux, state, Request{Schema: 1, Mode: "apply", Retry: true}, observed, "source")
	if err != nil || plan.ResourceResume == nil || len(plan.Operations) != 1 || plan.Operations[0].Privileged || plan.ResourceResume.Operation != opID {
		t.Fatal("recovery counted completed administrator steps or changed original operation", plan, err)
	}
	if !original.Operations[0].Privileged || original.Operations[0].Action != "install" {
		t.Fatal("recovery changed the historical plan")
	}
}

func installed() (State, map[string]Observation) {
	state := State{Schema: 1, Context: linux, Selected: []string{"editor", "agent"}, Receipts: map[string]Receipt{}}
	observed := map[string]Observation{}
	for _, id := range []string{"node", "plugins"} {
		o := Observation{Present: true, Healthy: true, Provider: "test-provider", Identity: id, Fingerprint: id + "-v1"}
		observed[id] = o
		state.Receipts[id] = Receipt{Ownership: "created", After: o, Status: "ready"}
	}
	return state, observed
}

func operation(t *testing.T, p Plan, id string) Operation {
	t.Helper()
	for _, op := range p.Operations {
		if op.Resource == id {
			return op
		}
	}
	t.Fatalf("missing operation for %s", id)
	return Operation{}
}

func TestSharedRuntimeSurvivesConsumerRemoval(t *testing.T) {
	state, observed := installed()
	p, err := PlanChanges(testCatalog(t), linux, state, Request{Schema: 1, Mode: "plan", Selected: []string{"agent"}, RemoveShared: []string{"node"}}, observed, "source")
	if err != nil {
		t.Fatal(err)
	}
	if got := operation(t, p, "node").Action; got != "keep" {
		t.Fatalf("Node action: %s", got)
	}
	if got := operation(t, p, "plugins").Action; got != "remove" {
		t.Fatalf("plugin action: %s", got)
	}
}

func TestLastConsumerRemovalOrdersCleanupBeforeRuntime(t *testing.T) {
	state, observed := installed()
	p, err := PlanChanges(testCatalog(t), linux, state, Request{Schema: 1, Mode: "plan", RemoveShared: []string{"node"}}, observed, "source")
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Operations) != 2 || p.Operations[0].Resource != "plugins" || p.Operations[1].Resource != "node" {
		t.Fatalf("unsafe cleanup order: %+v", p.Operations)
	}
	for _, op := range p.Operations {
		if op.Action != "remove" {
			t.Fatalf("unexpected action %+v", op)
		}
	}
}

func TestExternalRuntimeConsumerBlocksExplicitRemoval(t *testing.T) {
	state, observed := installed()
	node := observed["node"]
	node.Consumers = []string{"unrelated-global-npm-package"}
	observed["node"] = node
	p, err := PlanChanges(testCatalog(t), linux, state, Request{Schema: 1, Mode: "plan", RemoveShared: []string{"node"}}, observed, "source")
	if err != nil {
		t.Fatal(err)
	}
	if got := operation(t, p, "node").Action; got != "retain" {
		t.Fatalf("removed an external consumer's runtime: %s", got)
	}
}

func TestChangedArtifactAndPreexistingPackageSurviveRemoval(t *testing.T) {
	for _, kind := range []string{"changed", "preexisting", "uncertain", "provider-changed"} {
		t.Run(kind, func(t *testing.T) {
			state, observed := installed()
			r, o := state.Receipts["plugins"], observed["plugins"]
			switch kind {
			case "changed":
				o.Fingerprint = "user-edited"
			case "preexisting":
				r.Ownership = "reused"
			case "uncertain":
				r.Ownership = "uncertain"
			case "provider-changed":
				o.Provider = "outside-provider"
			}
			state.Receipts["plugins"], observed["plugins"] = r, o
			p, err := PlanChanges(testCatalog(t), linux, state, Request{Schema: 1, Mode: "plan"}, observed, "source")
			if err != nil {
				t.Fatal(err)
			}
			if got := operation(t, p, "plugins").Action; got != "retain" {
				t.Fatalf("unsafe removal: %s", got)
			}
		})
	}
}

func TestKeepResourceKeepsItsPrerequisites(t *testing.T) {
	state, observed := installed()
	p, err := PlanChanges(testCatalog(t), linux, state, Request{Schema: 1, Mode: "plan", Keep: []string{"plugins"}}, observed, "source")
	if err != nil {
		t.Fatal(err)
	}
	for _, op := range p.Operations {
		if op.Action != "keep" {
			t.Fatalf("kept resource lost a dependency: %+v", op)
		}
	}
}

func TestRetainingNativeProgramDoesNotRetainRemovedShellConfiguration(t *testing.T) {
	catalog, err := DefaultCatalog()
	if err != nil {
		t.Fatal(err)
	}
	for _, target := range []Context{{OS: "darwin", Arch: "arm64"}, linux, {OS: "linux", Arch: "arm64"}, {OS: "windows", Arch: "amd64"}} {
		for _, capability := range []string{"git", "tmux"} {
			root, _ := catalog.Resource(capability)
			if !root.Available(target) {
				continue
			}
			for _, retention := range []string{"pre-existing", "shared", "explicit-keep", "selected"} {
				t.Run(target.OS+"-"+target.Arch+"/"+capability+"/"+retention, func(t *testing.T) {
					closure, err := catalog.Closure([]string{capability}, target)
					if err != nil {
						t.Fatal(err)
					}
					state := State{Schema: 1, Context: target, Selected: []string{capability}, Receipts: map[string]Receipt{}}
					observed := map[string]Observation{}
					for _, id := range closure {
						resource, _ := catalog.Resource(id)
						if resource.Action == "" {
							continue
						}
						observation := Observation{Present: true, Healthy: true, Provider: "fixture", Identity: id, Fingerprint: id + "-v1"}
						observed[id] = observation
						state.Receipts[id] = Receipt{Ownership: "created", After: observation, Status: "ready"}
					}
					tool := "tool." + capability
					request := Request{Schema: 1, Mode: "apply", Selected: []string{}}
					toolAction, shellAction := "retain", "remove"
					switch retention {
					case "pre-existing":
						receipt := state.Receipts[tool]
						receipt.Ownership = "reused"
						state.Receipts[tool] = receipt
					case "explicit-keep":
						request.Keep, toolAction = []string{tool}, "keep"
					case "selected":
						request.Selected = []string{capability}
						toolAction, shellAction = "keep", "keep"
					}
					plan, err := PlanChanges(catalog, target, state, request, observed, "fixture")
					if err != nil {
						t.Fatal(err)
					}
					if op := operation(t, plan, tool); op.Action != toolAction {
						t.Fatalf("native program should survive: %+v", op)
					}
					if op := operation(t, plan, "integration.shells"); op.Action != shellAction {
						t.Fatalf("shell configuration should follow selection: %+v", op)
					}
					configuration := "config." + capability
					if capability == "git" {
						configuration = gitSettingsResource
					}
					if op := operation(t, plan, configuration); op.Action != shellAction {
						t.Fatalf("tool configuration should follow selection: %+v", op)
					}
				})
			}
		}
	}
}

func TestPlanStableAcrossSelectionOrderAndDuplicates(t *testing.T) {
	state, observed := installed()
	a, err := PlanChanges(testCatalog(t), linux, state, Request{Schema: 1, Mode: "plan", Selected: []string{"agent", "editor", "agent"}}, observed, "source")
	if err != nil {
		t.Fatal(err)
	}
	b, err := PlanChanges(testCatalog(t), linux, state, Request{Schema: 1, Mode: "plan", Selected: []string{"editor", "agent"}}, observed, "source")
	if err != nil {
		t.Fatal(err)
	}
	if a.ID != b.ID {
		t.Fatal("equivalent selection produced a different approval identity")
	}
	observed["node"] = Observation{Present: true, Healthy: true, Fingerprint: "changed"}
	c, err := PlanChanges(testCatalog(t), linux, state, Request{Schema: 1, Mode: "plan", Selected: []string{"editor", "agent"}}, observed, "source")
	if err != nil {
		t.Fatal(err)
	}
	if a.ID == c.ID {
		t.Fatal("changed observation did not invalidate approval identity")
	}
}

func TestInvalidGraphFailsBeforePlanning(t *testing.T) {
	for _, mutation := range []string{"missing", "cycle", "duplicate", "bad-platform"} {
		t.Run(mutation, func(t *testing.T) {
			c := testCatalog(t)
			switch mutation {
			case "missing":
				c.Resources[0].Requires = []string{"unknown"}
			case "cycle":
				c.Resources[3].Requires = []string{"editor"}
			case "duplicate":
				c.Resources = append(c.Resources, c.Resources[0])
			case "bad-platform":
				c.Resources[0].Platforms = []string{"unknown"}
			}
			if c.Validate() == nil {
				t.Fatal("invalid graph accepted")
			}
		})
	}
}

func TestCatalogEverySingletonHasCompleteDependencyClosure(t *testing.T) {
	c, err := DefaultCatalog()
	if err != nil {
		t.Fatal(err)
	}
	for _, ctx := range []Context{{OS: "darwin", Arch: "arm64"}, linux, {OS: "linux", Arch: "arm64"}, {OS: "windows", Arch: "amd64"}} {
		for _, root := range c.Capabilities(ctx) {
			t.Run(ctx.OS+"/"+ctx.Arch+"/"+root.ID, func(t *testing.T) {
				closure, err := c.Closure([]string{root.ID}, ctx)
				if err != nil {
					t.Fatal(err)
				}
				for i, id := range closure {
					r, _ := c.Resource(id)
					for _, dep := range r.Dependencies(ctx) {
						if !slices.Contains(closure[:i], dep) {
							t.Fatalf("%s precedes dependency %s", id, dep)
						}
					}
				}
			})
		}
	}
}

func TestReachabilityNeverRemovesSelectedResources(t *testing.T) {
	c, err := DefaultCatalog()
	if err != nil {
		t.Fatal(err)
	}
	random := rand.New(rand.NewPCG(42, 51))
	for _, os := range []string{"darwin", "linux", "windows"} {
		ctx := Context{OS: os, Arch: "amd64"}
		if os == "darwin" {
			ctx.Arch = "arm64"
		}
		caps := c.Capabilities(ctx)
		all := []string{}
		for _, capability := range caps {
			all = append(all, capability.ID)
		}
		closure, err := c.Closure(all, ctx)
		if err != nil {
			t.Fatal(err)
		}
		state := State{Schema: 1, Context: ctx, Receipts: map[string]Receipt{}}
		observed := map[string]Observation{}
		for _, id := range closure {
			o := Observation{Present: true, Healthy: true, Fingerprint: id}
			state.Receipts[id] = Receipt{Ownership: "created", After: o}
			observed[id] = o
		}
		for range 100 {
			selected := []string{}
			for _, capability := range caps {
				if random.IntN(2) == 1 {
					selected = append(selected, capability.ID)
				}
			}
			p, err := PlanChanges(c, ctx, state, Request{Schema: 1, Mode: "plan", Selected: selected, RemoveShared: closure}, observed, "source")
			if err != nil {
				t.Fatal(err)
			}
			wanted, err := c.Closure(selected, ctx)
			if err != nil {
				t.Fatal(err)
			}
			for _, op := range p.Operations {
				if op.Action == "remove" && slices.Contains(wanted, op.Resource) {
					t.Fatalf("removed reachable %s", op.Resource)
				}
			}
		}
	}
}

func TestStateRoundTripAndUnknownSchemaPreservation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	state, _ := installed()
	state.Target = "test-home"
	if err := SaveState(path, state); err != nil {
		t.Fatal(err)
	}
	got, err := LoadState(path, "test-home")
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Receipts) != 2 {
		t.Fatal("lost receipts")
	}
	state.Schema = 2
	data, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadState(path, "test-home"); err == nil {
		t.Fatal("newer schema accepted")
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != string(after) {
		t.Fatal("rejected state was modified")
	}
}

func TestLegacyMissingOptionalFieldsAndInvalidInput(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	if err := os.WriteFile(path, []byte(`{"schema":1,"target":"home","context":{"os":"linux","arch":"amd64"}}`), 0600); err != nil {
		t.Fatal(err)
	}
	s, err := LoadState(path, "home")
	if err != nil || s.Receipts == nil {
		t.Fatalf("legacy shape: %+v %v", s, err)
	}
	for _, input := range []string{`{"schema":1,"unexpected":true}`, `{"schema":1} {}`, `null`, `[]`} {
		var req Request
		err := Decode([]byte(input), &req)
		if err == nil && req.Schema != 1 {
			err = os.ErrInvalid
		}
		if err == nil {
			t.Fatalf("accepted %s", input)
		}
	}
}
