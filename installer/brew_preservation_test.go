package installer

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Model only native process IO. The adapter still creates and validates its own
// intent, completion evidence, ownership ledger and observations.
func brewMutationFixture(t *testing.T, mutate func(map[string]nativePackage)) (*BrewDriver, Resource, Receipt) {
	t.Helper()
	return brewMutationInventoryFixture(t, map[string]nativePackage{
		"shared":  {Name: "shared", Source: "shared", Version: "1.0", Automatic: true, Healthy: true},
		"outside": {Name: "outside", Source: "outside", Version: "1.0", Healthy: true, Dependencies: []string{"shared"}},
	}, mutate)
}

func brewMutationInventoryFixture(t *testing.T, installed map[string]nativePackage, mutate func(map[string]nativePackage)) (*BrewDriver, Resource, Receipt) {
	t.Helper()
	d, resource, receipt, _ := brewDriverFixture(t)
	d.Query = func(_ context.Context, _ bool, program string, _ []byte, args ...string) ([]byte, error) {
		if program == "brew" && len(args) >= 3 && args[0] == "linkage" && args[1] == "--test" {
			return nil, nil
		}
		if program != "brew" || strings.Join(args, " ") != "info --json=v2 --installed" {
			t.Fatalf("unexpected native query %s %v", program, args)
		}
		formulae, casks := []any{}, []any{}
		for _, pkg := range installed {
			if strings.HasPrefix(pkg.Name, "cask/") {
				casks = append(casks, map[string]any{"token": strings.TrimPrefix(pkg.Name, "cask/"), "full_token": pkg.Source, "installed": pkg.Version, "depends_on": map[string]any{"formula": pkg.Dependencies}})
				continue
			}
			dependencies := []any{}
			for _, name := range pkg.Dependencies {
				dependencies = append(dependencies, map[string]any{"full_name": name})
			}
			linked := ""
			if pkg.Healthy {
				linked = pkg.Version
			}
			formulae = append(formulae, map[string]any{
				"name": pkg.Name, "full_name": pkg.Source, "pinned": pkg.Held, "linked_keg": linked,
				"installed": []any{map[string]any{"version": pkg.Version, "installed_on_request": !pkg.Automatic, "runtime_dependencies": dependencies}},
			})
		}
		return json.Marshal(map[string]any{"formulae": formulae, "casks": casks})
	}
	d.Run = func(_ context.Context, command nativeCommand) ([]byte, error) {
		installed["first"] = nativePackage{Name: "first", Source: "first", Version: "1.0", Healthy: true, Dependencies: []string{"shared"}}
		if mutate != nil {
			mutate(installed)
		}
		output := []byte(fmt.Sprintf("dotfiles-operation-%s  %s/first/1.0: fixture\ndotfiles-operation-%s  %s/shared/2.0: fixture\n", command.Operation, filepath.ToSlash(d.Cellar), command.Operation, filepath.ToSlash(d.Cellar)))
		hash, err := digest(command)
		if err != nil {
			return nil, err
		}
		code := 0
		err = saveDocument(filepath.Join(d.Directory, "worker", "commands", command.Operation+".json"), nativeCommandRecord{Schema: 1, Command: hash, Reply: &nativeReply{Output: output, ExitCode: &code}})
		return output, err
	}
	return d, resource, receipt
}

func TestBrewCompletionPreservesPreexistingPackages(t *testing.T) {
	for _, change := range []string{"source", "pin", "manual", "missing-consumer", "missing-dependency", "version"} {
		t.Run(change, func(t *testing.T) {
			d, r, receipt := brewMutationFixture(t, func(installed map[string]nativePackage) {
				shared := installed["shared"]
				switch change {
				case "source":
					shared.Source = "other/tap/shared"
				case "pin":
					shared.Held = true
				case "manual":
					shared.Automatic = false
				case "missing-consumer":
					delete(installed, "outside")
				case "missing-dependency":
					delete(installed, "shared")
					return
				case "version":
					shared.Version = "2.0"
				}
				installed["shared"] = shared
			})
			_, err := d.Apply(context.Background(), r, Operation{Action: "install"}, receipt)
			if change == "version" {
				if err != nil {
					t.Fatal("native dependency maintenance must allow version changes", err)
				}
				state, err := d.ledger()
				if err != nil || len(state.Pool) != 0 {
					t.Fatal("pre-existing maintained dependency became owned", state, err)
				}
			} else {
				if err == nil || !strings.Contains(err.Error(), "pre-existing") {
					t.Fatal("completion accepted damaged pre-existing packages", change, err)
				}
				if _, err := os.Stat(filepath.Join(d.Directory, "packages.json")); !os.IsNotExist(err) {
					t.Fatal("failed preservation check published ownership", err)
				}
			}
		})
	}
}

func TestBrewLegacyNamesOnlyIntentCannotAuthorizeRecovery(t *testing.T) {
	d, _, receipt, intent := brewDriverFixture(t)
	data, err := json.Marshal(intent)
	if err != nil {
		t.Fatal(err)
	}
	var legacy map[string]any
	if err := json.Unmarshal(data, &legacy); err != nil {
		t.Fatal(err)
	}
	legacy["schema"], legacy["before"] = 1, []string{"shared"}
	if err := saveDocument(d.intentPath(receipt.OperationID), legacy); err != nil {
		t.Fatal(err)
	}
	if _, err := d.intent(receipt.OperationID); err == nil {
		t.Fatal("old names-only intent proved protected classifications")
	}
}

func TestBrewClassificationOnlyIntentCannotAuthorizeConsumerRepair(t *testing.T) {
	d, _, receipt, intent := brewDriverFixture(t)
	data, err := json.Marshal(intent)
	if err != nil {
		t.Fatal(err)
	}
	var legacy map[string]any
	if err := json.Unmarshal(data, &legacy); err != nil {
		t.Fatal(err)
	}
	legacy["schema"] = 2
	legacy["before"] = map[string]any{"shared": map[string]any{"source": "shared", "held": false, "automatic": true}}
	if err := saveDocument(d.intentPath(receipt.OperationID), legacy); err != nil {
		t.Fatal(err)
	}
	if _, err := d.intent(receipt.OperationID); err == nil {
		t.Fatal("classification-only intent supplied no native consumer graph")
	}
}
