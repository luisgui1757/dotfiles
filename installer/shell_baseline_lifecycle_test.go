package installer

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"testing"
)

// Exercise the real application composer and profile controller across fresh
// instances without provisioning tool packages on the developer workstation.
func TestGeneratedShellProfilesRestoreAfterFreshControllerSelectionChanges(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX generated environment lifecycle")
	}
	for _, personal := range []bool{false, true} {
		t.Run(map[bool]string{false: "absent", true: "personal"}[personal], func(t *testing.T) {
			home, err := resolveConfigPath(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			repository, err := filepath.Abs("..")
			if err != nil {
				t.Fatal(err)
			}
			folders := ConfigFolders{Home: home, Config: filepath.Join(home, ".config"), Zsh: home}
			platform := nativePlatform(t)
			// Supply a private Homebrew prefix only for selected PATH composition.
			// The declarative Git fixture below never queries or mutates packages.
			if platform.OS == "darwin" {
				platform.Homebrew = &HomebrewLocation{Program: filepath.Join(home, "brew", "bin", "brew"), Prefix: filepath.Join(home, "brew"), Cellar: filepath.Join(home, "brew", "Cellar")}
			}
			fresh := func() Controller {
				c, err := NewNativeController(repository, "shell-baseline-test", filepath.Join(home, "state"), platform, folders)
				if err != nil {
					t.Fatal(err)
				}
				git, _ := c.Catalog.Resource("tool.git")
				git.Action = ""
				git.Bindings = nil
				git.Requires = nil
				git.PlatformRequires = nil
				resources := slices.Clone(c.Catalog.Resources)
				for i := range resources {
					if resources[i].ID == git.ID {
						resources[i] = git
					}
				}
				first := Resource{ID: "first", Name: "First", Category: "Shell fixtures", Capability: true, Requires: []string{"integration.shells", "tool.git"}}
				second := Resource{ID: "second", Name: "Second", Category: "Shell fixtures", Capability: true, Requires: []string{"integration.shells"}}
				if platform.OS == "darwin" {
					// Use the real read-only macOS clipboard provider. Its reused
					// OS commands must survive without retaining shell profiles.
					first.Requires = append(first.Requires, "tool.clipboard")
				}
				c.Catalog = &Catalog{Schema: 1, Resources: append(resources, first, second)}
				c.Driver.(*NativeDriver).Catalog = c.Catalog
				return c
			}
			c := fresh()
			before := map[string]configSnapshot{}
			for _, p := range c.Driver.(*NativeDriver).Profiles.Targets["integration.shells"] {
				if personal {
					writeConfigFixture(t, p.Path, "# original personal shell bytes\n")
				}
				before[p.Path], err = snapshotConfig(p.Path)
				if err != nil {
					t.Fatal(err)
				}
			}
			apply := func(request Request) {
				c := fresh()
				dispatchApproved(t, c, request)
				if result, err := fresh().Dispatch(context.Background(), Request{Schema: 1, Mode: "check"}); err != nil || result.Status != "ready" {
					t.Fatal(result, err)
				}
			}
			apply(Request{Schema: 1, Mode: "apply", Selected: []string{"first"}})
			apply(Request{Schema: 1, Mode: "update"})
			apply(Request{Schema: 1, Mode: "apply", Selected: []string{"first", "second"}})
			apply(Request{Schema: 1, Mode: "apply", Selected: []string{"second"}})
			apply(Request{Schema: 1, Mode: "apply", Selected: []string{}})
			for _, profile := range c.Driver.(*NativeDriver).Profiles.Targets["integration.shells"] {
				if err := verifyConfigSnapshot(profile.Path, before[profile.Path]); err != nil {
					t.Fatal(profile.Path, err)
				}
			}
			c = fresh()
			state, err := LoadState(c.StatePath, c.Home)
			if err != nil || state.Transaction != nil || len(state.Selected) != 0 {
				t.Fatal("final removal did not complete", state, err)
			}
			if _, owned := state.Receipts["integration.shells"]; owned {
				t.Fatal("retained OS clipboard kept shell ownership", state.Receipts)
			}
			if platform.OS == "darwin" && state.Receipts["tool.clipboard"].Ownership != "reused" {
				t.Fatal("native clipboard did not remain unowned", state.Receipts)
			}
			if _, err := os.Stat(filepath.Join(home, "state", "shells", "env.sh")); !personal && !os.IsNotExist(err) {
				t.Fatal("fresh construction recreated removed environment", err)
			}
		})
	}
}

// Retention must follow real runtime dependencies: system clipboard commands
// need no private PATH; the Windows archive still does. Use the actual complete
// Neovim graph so a retained helper cannot conceal a shell restoration failure.
func TestRetainedClipboardShellDependencyMatchesProvider(t *testing.T) {
	catalog, err := DefaultCatalog()
	if err != nil {
		t.Fatal(err)
	}
	for _, target := range []Context{{OS: "darwin", Arch: "arm64"}, {OS: "linux", Arch: "amd64"}, {OS: "linux", Arch: "arm64"}, {OS: "windows", Arch: "amd64"}} {
		for _, retained := range []string{"pre-existing", "changed", "explicit-keep"} {
			t.Run(target.OS+"-"+target.Arch+"/"+retained, func(t *testing.T) {
				closure, err := catalog.Closure([]string{"neovim"}, target)
				if err != nil {
					t.Fatal(err)
				}
				state := State{Schema: 1, Context: target, Selected: []string{"neovim"}, Receipts: map[string]Receipt{}}
				observed := map[string]Observation{}
				request := Request{Schema: 1, Mode: "apply", Selected: []string{}}
				for _, id := range closure {
					r, _ := catalog.Resource(id)
					if r.Action == "" {
						continue
					}
					o := Observation{Present: true, Healthy: true, Provider: "fixture", Identity: id, Fingerprint: id + "-v1"}
					state.Receipts[id] = Receipt{Ownership: "created", After: o, Status: "ready"}
					observed[id] = o
					if r.Shared || r.Scope == "machine" {
						request.RemoveShared = append(request.RemoveShared, id)
					}
				}
				clipboard := state.Receipts["tool.clipboard"]
				wantAction := "retain"
				switch retained {
				case "pre-existing":
					clipboard.Ownership = "reused"
				case "changed":
					o := observed["tool.clipboard"]
					o.Fingerprint += "-external"
					observed["tool.clipboard"] = o
				case "explicit-keep":
					request.Keep = []string{"tool.clipboard"}
					request.RemoveShared = slices.DeleteFunc(request.RemoveShared, func(id string) bool { return id == "tool.clipboard" })
					wantAction = "keep"
				}
				state.Receipts["tool.clipboard"] = clipboard
				plan, err := PlanChanges(catalog, target, state, request, observed, "fixture")
				if err != nil {
					t.Fatal(err)
				}
				if op := operation(t, plan, "tool.clipboard"); op.Action != wantAction {
					t.Fatal("clipboard retention changed", op)
				}
				wantShell := "remove"
				if target.OS == "windows" {
					wantShell = "retain"
					if retained == "explicit-keep" {
						wantShell = "keep"
					}
				}
				if op := operation(t, plan, "integration.shells"); op.Action != wantShell {
					t.Fatalf("clipboard provider dependency must allow correct shell restoration: %+v; want %s", op, wantShell)
				}
				if target.OS == "linux" {
					if op := operation(t, plan, "tool.clipboard-wayland"); op.Action != wantAction {
						t.Fatal("retained clipboard lost its fixed Wayland helper", op)
					}
				}
			})
		}
	}
}
