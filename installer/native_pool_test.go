package installer

import (
	"reflect"
	"slices"
	"testing"
)

func TestNativePoolSharedDependencySurvivesUntilItsLastConsumer(t *testing.T) {
	installed := map[string]nativePackage{
		"first":     {Name: "first", Version: "1", Dependencies: []string{"shared"}},
		"second":    {Name: "second", Version: "1", Dependencies: []string{"shared"}},
		"shared":    {Name: "shared", Version: "1", Automatic: true},
		"unrelated": {Name: "unrelated", Version: "1", Automatic: true},
	}
	pool := []string{"shared"}
	first, protected, err := nativeRemovalCandidates([]string{"first"}, pool, nil, installed)
	if err != nil || !reflect.DeepEqual(first, []string{"first"}) || !reflect.DeepEqual(protected["shared"], []string{"second"}) {
		t.Fatal("first removal broke another consumer", first, protected, err)
	}
	delete(installed, "first")
	last, _, err := nativeRemovalCandidates([]string{"second"}, pool, nil, installed)
	if err != nil || !reflect.DeepEqual(last, []string{"second", "shared"}) {
		t.Fatal("last removal stranded the originally introduced dependency", last, err)
	}
	if slices.Contains(last, "unrelated") {
		t.Fatal("pre-existing orphan was swept")
	}
}

func TestNativePoolProtectsExternalConsumersManualPromotionsAndKeptRoots(t *testing.T) {
	for _, condition := range []string{"external", "manual", "held", "kept-incidental-root"} {
		t.Run(condition, func(t *testing.T) {
			installed := map[string]nativePackage{
				"root":   {Name: "root", Version: "1", Dependencies: []string{"middle"}},
				"middle": {Name: "middle", Version: "1", Automatic: true, Dependencies: []string{"leaf"}},
				"leaf":   {Name: "leaf", Version: "2", Automatic: true},
			}
			var keep []string
			switch condition {
			case "external":
				installed["outside"] = nativePackage{Name: "outside", Version: "1", Dependencies: []string{"middle"}}
			case "manual":
				pkg := installed["middle"]
				pkg.Automatic = false
				installed["middle"] = pkg
			case "held":
				pkg := installed["middle"]
				pkg.Held = true
				installed["middle"] = pkg
			case "kept-incidental-root":
				keep = []string{"middle"}
			}
			got, _, err := nativeRemovalCandidates([]string{"root"}, []string{"middle", "leaf"}, keep, installed)
			if err != nil || !reflect.DeepEqual(got, []string{"root"}) {
				t.Fatal("protected dependency chain was removed", got, err)
			}
		})
	}
}

func TestNativePoolCyclesRequireNoOutsideConsumer(t *testing.T) {
	installed := map[string]nativePackage{
		"root": {Name: "root", Version: "1", Dependencies: []string{"a"}},
		"a":    {Name: "a", Version: "1", Automatic: true, Dependencies: []string{"b"}},
		"b":    {Name: "b", Version: "1", Automatic: true, Dependencies: []string{"a"}},
	}
	got, _, err := nativeRemovalCandidates([]string{"root"}, []string{"a", "b"}, nil, installed)
	if err != nil || !reflect.DeepEqual(got, []string{"a", "b", "root"}) {
		t.Fatal(got, err)
	}
	installed["outside"] = nativePackage{Name: "outside", Version: "1", Dependencies: []string{"b"}}
	got, _, err = nativeRemovalCandidates([]string{"root"}, []string{"a", "b"}, nil, installed)
	if err != nil || !reflect.DeepEqual(got, []string{"root"}) {
		t.Fatal(got, err)
	}
}

func TestNativeRootRemovalRefusesNewExternalConsumer(t *testing.T) {
	installed := map[string]nativePackage{
		"root":    {Name: "root", Version: "1"},
		"outside": {Name: "outside", Version: "1", Dependencies: []string{"root"}},
	}
	if _, protected, err := nativeRemovalCandidates([]string{"root"}, nil, nil, installed); err == nil || !reflect.DeepEqual(protected["root"], []string{"outside"}) {
		t.Fatal("used root was approved for removal", protected, err)
	}
	if got, _, err := nativeRemovalCandidates(nil, nil, nil, installed); err != nil || len(got) != 0 {
		t.Fatal(got, err)
	}
}
