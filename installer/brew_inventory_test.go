package installer

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
)

const brewInventoryFixture = `{"formulae":[
	{"name":"first","full_name":"first","installed":[{"version":"1.0","installed_on_request":true,"runtime_dependencies":[{"full_name":"sample/tap/shared","pkg_version":"1.0"}]}]},
	{"name":"shared","full_name":"sample/tap/shared","installed":[{"version":"1.0","installed_on_request":false,"runtime_dependencies":[]}]},
	{"name":"orphan","full_name":"orphan","installed":[{"version":"1.0","installed_on_request":false,"runtime_dependencies":[]}]}
],"casks":[{"token":"outside","full_token":"outside","installed":"2.0","depends_on":{"formula":["sample/tap/shared"],"macos":">= :sonoma"}}]}`

func TestBrewInstalledInventoryProtectsCaskConsumers(t *testing.T) {
	run := func(_ context.Context, privileged bool, program string, input []byte, args ...string) ([]byte, error) {
		if privileged || program != "brew" || len(input) != 0 || !reflect.DeepEqual(args, []string{"info", "--json=v2", "--installed"}) {
			t.Fatal("unexpected inventory command", program, args)
		}
		return []byte(brewInventoryFixture), nil
	}
	installed, err := brewInventory(context.Background(), run)
	if err != nil {
		t.Fatal(err)
	}
	got, protected, err := nativeRemovalCandidates([]string{"first"}, []string{"shared"}, nil, installed)
	if err != nil || !reflect.DeepEqual(got, []string{"first"}) || !reflect.DeepEqual(protected["shared"], []string{"cask/outside"}) {
		t.Fatal("formula removal failed to preserve a cask dependency", got, protected, err)
	}
	delete(installed, "cask/outside")
	got, _, err = nativeRemovalCandidates([]string{"first"}, []string{"shared"}, nil, installed)
	if err != nil || !reflect.DeepEqual(got, []string{"first", "shared"}) {
		t.Fatal(got, err)
	}
}

func TestBrewMissingRuntimeMetadataUsesNativeRuntimeQuery(t *testing.T) {
	data := strings.Replace(brewInventoryFixture, `"runtime_dependencies":[{"full_name":"sample/tap/shared","pkg_version":"1.0"}]`, `"runtime_dependencies":null`, 1)
	queried := false
	run := func(_ context.Context, _ bool, _ string, _ []byte, args ...string) ([]byte, error) {
		if args[0] == "info" {
			return []byte(data), nil
		}
		if !reflect.DeepEqual(args, []string{"deps", "--installed", "--formula", "first"}) {
			t.Fatal("not an actual runtime dependency query", args)
		}
		queried = true
		return []byte("sample/tap/shared\n"), nil
	}
	installed, err := brewInventory(context.Background(), run)
	if err != nil || !queried || !reflect.DeepEqual(installed["first"].Dependencies, []string{"shared"}) {
		t.Fatal(installed, err)
	}
}

func TestBrewUnknownManualMarkAndPinCannotAuthorizeIncidentalRemoval(t *testing.T) {
	for _, change := range []string{"missing-manual", "pinned"} {
		t.Run(change, func(t *testing.T) {
			data := brewInventoryFixture
			if change == "missing-manual" {
				data = strings.Replace(data, `"installed_on_request":false,`, ``, 1)
			} else {
				data = strings.Replace(data, `"name":"shared",`, `"name":"shared","pinned":true,`, 1)
			}
			run := func(context.Context, bool, string, []byte, ...string) ([]byte, error) { return []byte(data), nil }
			installed, err := brewInventory(context.Background(), run)
			if err != nil {
				t.Fatal(err)
			}
			delete(installed, "cask/outside")
			got, _, err := nativeRemovalCandidates([]string{"first"}, []string{"shared"}, nil, installed)
			if err != nil || !reflect.DeepEqual(got, []string{"first"}) {
				t.Fatal(got, err)
			}
		})
	}
}

func TestBrewInventoryRefusesMissingCollectionsAndAmbiguousIdentities(t *testing.T) {
	for _, data := range []string{`{}`, `{"formulae":[],"casks":null}`, strings.Replace(brewInventoryFixture, `"name":"orphan","full_name":"orphan"`, `"name":"shared","full_name":"shared"`, 1), strings.ReplaceAll(brewInventoryFixture, "sample/tap/shared", "../../shared")} {
		run := func(context.Context, bool, string, []byte, ...string) ([]byte, error) { return []byte(data), nil }
		if _, err := brewInventory(context.Background(), run); err == nil {
			t.Fatal("invalid native inventory was accepted", data)
		}
	}
	run := func(context.Context, bool, string, []byte, ...string) ([]byte, error) {
		return nil, errors.New("native refusal")
	}
	if _, err := brewInventory(context.Background(), run); err == nil {
		t.Fatal("failed inspection authorized removal")
	}
}
