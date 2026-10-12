package installer

import (
	"fmt"
	"path/filepath"
	"reflect"
	"testing"
)

func TestNativeBrewLegacyPrerequisitesPreserveDependencies(t *testing.T) {
	f := newNativeBrewFixture(t)
	shared, consumer, missing := f.prefix+"-shared", f.prefix+"-consumer", f.prefix+"-missing"
	for _, name := range []string{shared, consumer, missing} {
		dependency := ""
		if name == consumer {
			dependency = fmt.Sprintf("  depends_on %q\n", f.tap+"/"+shared)
		}
		f.formula(name, "1.0", dependency, fmt.Sprintf("    (share/%q).install %q", name, "data.txt"))
	}
	f.require("", "trust", "--tap", f.tap)
	f.require("", "install", "--formula", f.tap+"/"+consumer)
	before, err := brewInventory(f.ctx, f.query)
	if err != nil || !before[shared].Automatic || before[consumer].Automatic || len(legacyBrewConsumers(before, shared, nil)) != 1 {
		t.Fatal("the real fixture must have an automatic dependency and its requested consumer", before[shared], before[consumer], err)
	}
	snapshots := map[string]configSnapshot{}
	for _, name := range []string{shared, consumer} {
		snapshots[name], err = snapshotTree(filepath.Join(f.cellar, name), 1<<20, 100)
		if err != nil {
			t.Fatal(err)
		}
	}

	prepareLegacyBrewFormulae(t, f.program, f.env, f.tap+"/"+shared, f.tap+"/"+consumer, f.tap+"/"+missing)
	after, err := brewInventory(f.ctx, f.query)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{shared, consumer} {
		if !reflect.DeepEqual(before[name], after[name]) {
			t.Errorf("prerequisite preparation changed existing package ownership: before=%+v after=%+v", before[name], after[name])
		}
		if err := verifyConfigSnapshot(filepath.Join(f.cellar, name), snapshots[name]); err != nil {
			t.Error("prerequisite preparation changed an existing package", name, err)
		}
	}
	if pkg, ok := after[missing]; !ok || pkg.Automatic || pkg.Version != "1.0" || pkg.Source != f.tap+"/"+missing {
		t.Fatal("the absent prerequisite was not genuinely installed", pkg)
	}
}
