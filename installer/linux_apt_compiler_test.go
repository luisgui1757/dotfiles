package installer

import "testing"

func TestLinuxAPTCompilerOnlyOwnsMakeAsANativeIncidental(t *testing.T) {
	f, c := aptDependencyFixture(t, "tool.compiler", "tool.make")
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"image-only"}})
	ledger, err := f.driver.ledger()
	if err != nil || ledger.Roots["tool.compiler"].Name != "build-essential:amd64" || ledger.Roots["tool.make"].Name != "" || ledger.Pool["make:amd64"].Name == "" {
		t.Fatal("native transitive dependency became an extra selected root", ledger, err)
	}
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{}, RemoveShared: []string{"tool.compiler"}})
	if _, present := f.installed["make:amd64"]; present {
		t.Fatal("compiler removal stranded its originally introduced Make dependency")
	}
}

func TestLinuxAPTCompilerAndMakeLifecycleInEitherNativeInstallOrder(t *testing.T) {
	for _, order := range []string{"compiler-first", "make-first"} {
		t.Run(order, func(t *testing.T) {
			f, c := aptDependencyFixture(t, "tool.compiler", "tool.make")
			if order == "make-first" {
				c.Catalog.Resources = append(c.Catalog.Resources, Resource{ID: "make-only", Name: "Make fixture", Platforms: []string{"linux"}, Capability: true, Requires: []string{"tool.make"}})
				dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"make-only"}})
			}
			dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"desktop-fixture"}})
			state, err := LoadState(c.StatePath, c.Home)
			if err != nil {
				t.Fatal(err)
			}
			for _, id := range []string{"tool.compiler", "tool.make"} {
				if receipt := state.Receipts[id]; receipt.Ownership != "created" || receipt.Before.Present {
					t.Fatal("selected tool lost its absent baseline or ownership", id, receipt)
				}
			}
			dispatchApproved(t, c, Request{Schema: 1, Mode: "update"})
			// Removing explicit Make releases its root, but native
			// build-essential still needs the physical package.
			dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"image-only"}, RemoveShared: []string{"tool.make"}})
			ledger, err := f.driver.ledger()
			if err != nil || ledger.Roots["tool.make"].Name != "" || ledger.Pool["make:amd64"].Name == "" {
				t.Fatal("Make ownership was lost while the compiler still needed it", ledger, err)
			}
			dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{}, RemoveShared: []string{"tool.compiler"}})
			ledger, err = f.driver.ledger()
			if err != nil || len(ledger.Roots) != 0 || len(ledger.Pool) != 0 {
				t.Fatal("final native removal left ownership behind", ledger, err)
			}
			for _, name := range []string{"make:amd64", "build-essential:amd64"} {
				if _, present := f.installed[name]; present {
					t.Fatal("owned native package survived final removal", name)
				}
			}
		})
	}
}
