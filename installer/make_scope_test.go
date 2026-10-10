package installer

import "testing"

func TestMakeApprovalFollowsNativeProviderScope(t *testing.T) {
	catalog, err := DefaultCatalog()
	if err != nil {
		t.Fatal(err)
	}
	for _, target := range []Context{{OS: "windows", Arch: "amd64"}, linux, {OS: "darwin", Arch: "arm64"}} {
		t.Run(target.OS, func(t *testing.T) {
			scope, provider := "machine", "apt"
			if target.OS == "windows" {
				scope, provider = "user", "archive"
			} else if target.OS == "darwin" {
				provider = "apple-developer-infrastructure"
			}
			state := State{Schema: 1, Context: target, Receipts: map[string]Receipt{}}
			observed := map[string]Observation{"tool.make": {Scope: scope, Provider: provider, Identity: "make"}}
			closure, err := catalog.Closure([]string{"neovim"}, target)
			if err != nil {
				t.Fatal(err)
			}
			for _, id := range closure {
				if id != "tool.make" {
					observed[id] = Observation{Provider: "fixture", Identity: id}
				}
			}
			plan, err := PlanChanges(catalog, target, state, Request{Schema: 1, Mode: "apply", Selected: []string{"neovim"}}, observed, "fixture")
			if err != nil {
				t.Fatal(err)
			}
			if op := operation(t, plan, "tool.make"); op.Action != "install" || op.Privileged != (scope == "machine") {
				t.Fatalf("installation approval disagrees with provider scope: %+v", op)
			}
			o := observed["tool.make"]
			o.Present, o.Healthy, o.Fingerprint = true, true, "owned-make"
			observed["tool.make"] = o
			state.Receipts["tool.make"] = Receipt{Ownership: "created", Status: "ready", After: o}
			for _, approve := range []bool{false, true} {
				request := Request{Schema: 1, Mode: "apply", Selected: []string{}}
				if approve {
					request.RemoveShared = []string{"tool.make"}
				}
				plan, err := PlanChanges(catalog, target, state, request, observed, "fixture")
				if err != nil {
					t.Fatal(err)
				}
				want := "remove"
				if scope == "machine" && !approve {
					want = "retain"
				}
				if op := operation(t, plan, "tool.make"); op.Action != want || op.Privileged != (scope == "machine") {
					t.Fatalf("removal approval disagrees with provider scope (approved=%v): %+v", approve, op)
				}
			}
		})
	}
}
