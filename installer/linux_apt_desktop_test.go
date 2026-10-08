package installer

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// Model the official Debian/Ubuntu libxcb-image0 -> libxcb-util1 dependency at
// the native command boundary, including operation-specific APT/dpkg evidence.
// No native package process runs in this controller regression.
func aptDesktopFixture(t *testing.T) (*linuxAPTFixture, Controller) {
	t.Helper()
	return aptDependencyFixture(t, "library.libxcb-image0", "library.libxcb-util1")
}

// Both real package relations exercise the same native command boundary: APT
// installs an incidental dependency that can also be an independent selection.
func aptDependencyFixture(t *testing.T, consumerID, dependencyID string) (*linuxAPTFixture, Controller) {
	t.Helper()
	f := newLinuxAPTFixture(t)
	catalog, err := DefaultCatalog()
	if err != nil {
		t.Fatal(err)
	}
	// This real native dependency pair is fixture data, independent of which
	// desktop applications the current product exposes.
	for _, name := range []string{"libxcb-image0", "libxcb-util1"} {
		catalog.Resources = append(catalog.Resources, Resource{
			ID: "library." + name, Name: name, Platforms: []string{"linux"},
			Action: "apt-library", Scope: "machine",
			Bindings: map[string]Binding{"linux": {Provider: "apt", Package: name}},
		})
	}
	c := &Catalog{Schema: 1, Resources: []Resource{{ID: "desktop-fixture", Name: "Desktop fixture", Platforms: []string{"linux"}, Capability: true, Requires: []string{consumerID, dependencyID}}}}
	c.Resources = append(c.Resources, Resource{ID: "image-only", Name: "Image consumer", Platforms: []string{"linux"}, Capability: true, Requires: []string{consumerID}})
	f.driver.Packages = map[string]string{}
	for _, r := range catalog.Resources {
		if (strings.HasPrefix(r.ID, "library.") || r.ID == consumerID || r.ID == dependencyID) && r.Bindings["linux"].Provider == "apt" {
			r.Platforms = []string{"linux"}
			r.Bindings = map[string]Binding{"linux": r.Bindings["linux"]}
			r.PlatformRequires = map[string][]string{"linux": r.PlatformRequires["linux"]}
			c.Resources = append(c.Resources, r)
			f.driver.Packages[r.ID] = r.Bindings["linux"].Package
		}
	}
	consumer, dependency := f.driver.Packages[consumerID], f.driver.Packages[dependencyID]
	// Other native library prerequisites are already healthy system packages.
	for _, name := range f.driver.Packages {
		if name != consumer && name != dependency {
			f.installed[name+":amd64"] = nativePackage{Name: name + ":amd64", Source: name, Version: "1.0", Healthy: true}
		}
	}
	f.driver.Run = func(_ context.Context, command nativeCommand) ([]byte, error) {
		f.runs++
		root := command.Arguments[len(command.Arguments)-1]
		if root != "update" && !slices.Contains(command.Arguments, "--remove") && !slices.Contains(command.Arguments, "-W") {
			names := []string{root}
			if root == consumer {
				names = []string{dependency, root}
			}
			var installed []string
			var dpkg strings.Builder
			for _, name := range names {
				identity := name + ":amd64"
				if old, exists := f.installed[identity]; exists {
					// Real apt-get install promotes an explicit automatic root
					// unless the saved command preserves its classification.
					if name == root {
						if old.Version != f.version && !slices.Contains(command.Arguments, "--mark-auto") || old.Version == f.version && !slices.Contains(command.Arguments, "--only-upgrade") && !slices.Contains(command.Arguments, "--reinstall") {
							old.Automatic = false
						}
					}
					if old.Version != f.version {
						fmt.Fprintf(&dpkg, "2026-10-10 00:00:00 upgrade %s %s %s\n2026-10-10 00:00:00 status installed %s %s\n", identity, old.Version, f.version, identity, f.version)
						old.Version = f.version
					}
					f.installed[identity] = old
					continue
				}
				pkg := nativePackage{Name: identity, Source: name, Version: "1.0", Automatic: name != root, Healthy: true}
				if name == consumer {
					pkg.Dependencies = []string{dependency + ":amd64"}
				}
				f.installed[identity] = pkg
				entry := identity + " (1.0"
				if pkg.Automatic {
					entry += ", automatic"
				}
				installed = append(installed, entry+")")
				fmt.Fprintf(&dpkg, "2026-10-10 00:00:00 install %s <none> 1.0\n2026-10-10 00:00:00 status installed %s 1.0\n", identity, identity)
			}
			history := "Start-Date: 2026-10-10  00:00:00\nCommandline: apt-get -o Dotfiles::Operation=" + command.Operation + " install " + root + "\nInstall: " + strings.Join(installed, ", ") + "\nEnd-Date: 2026-10-10  00:00:01\n"
			for suffix, data := range map[string]string{".history": history, ".dpkg": dpkg.String()} {
				if err := os.WriteFile(filepath.Join(f.driver.Directory, "logs", command.Operation+suffix), []byte(data), 0600); err != nil {
					return nil, err
				}
			}
		} else if index := slices.Index(command.Arguments, "--remove"); index >= 0 {
			for _, name := range command.Arguments[index+1:] {
				delete(f.installed, name)
			}
		}
		code := 0
		hash, err := digest(command)
		if err != nil {
			return nil, err
		}
		return nil, saveDocument(filepath.Join(f.driver.WorkerDirectory, "commands", command.Operation+".json"), nativeCommandRecord{Schema: 1, Command: hash, Reply: &nativeReply{ExitCode: &code}})
	}
	home := filepath.Dir(f.driver.Directory)
	statePath := filepath.Join(home, "state.json")
	d := &NativeDriver{Catalog: c, Context: linux, APT: f.driver, StatePath: statePath, session: newNativeSession(f.driver.WorkerDirectory)}
	return f, Controller{Catalog: c, Context: linux, Source: "desktop-library-fixture", Home: home, StatePath: statePath, Driver: d}
}

func TestLinuxAPTDesktopLibrarySelectionCompletesOneApprovedPlan(t *testing.T) {
	f, c := aptDesktopFixture(t)
	request := Request{Schema: 1, Mode: "apply", Selected: []string{"desktop-fixture"}}
	preview, err := c.Dispatch(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	request.ExpectedPlan = preview.Plan.ID
	result, err := c.Dispatch(context.Background(), request)
	if err != nil || result.Status != "ready" {
		t.Fatal("selected library dependencies invalidated their own approved installation", result.Status, err)
	}
	ledger, err := f.driver.ledger()
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"library.libxcb-image0", "library.libxcb-util1"} {
		if owned := ledger.Roots[id]; owned.Name == "" {
			t.Fatal("selected library lacks independently removable ownership", id, ledger)
		}
	}
}

func TestLinuxAPTDesktopLibraryUpdateAndFullRemoval(t *testing.T) {
	f, c := aptDesktopFixture(t)
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"desktop-fixture"}})
	state, err := LoadState(c.StatePath, c.Home)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"library.libxcb-image0", "library.libxcb-util1"} {
		if state.Receipts[id].Before.Present || state.Receipts[id].Ownership != "created" {
			t.Fatal("lost absent baseline", id, state.Receipts[id])
		}
	}
	if f.runs != 2 {
		t.Fatal("pool promotion executed unnecessary native commands", f.runs)
	}
	dispatchApproved(t, c, Request{Schema: 1, Mode: "update"})
	if !f.installed["libxcb-util1:amd64"].Automatic {
		t.Fatal("no-op update promoted an automatic package")
	}
	f.version = "2.0"
	dispatchApproved(t, c, Request{Schema: 1, Mode: "update"})
	if !f.installed["libxcb-util1:amd64"].Automatic || f.installed["libxcb-util1:amd64"].Version != "2.0" {
		t.Fatal("upgrade changed classification or failed to update", f.installed)
	}
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{}, RemoveShared: []string{"library.libxcb-image0", "library.libxcb-util1"}})
	assertAPTDesktopRemoved(t, f, c)
}

func TestLinuxAPTDesktopLibrarySelectiveRemovalRetainsOwnership(t *testing.T) {
	f, c := aptDesktopFixture(t)
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"desktop-fixture"}})
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"image-only"}, RemoveShared: []string{"library.libxcb-util1"}})
	ledger, err := f.driver.ledger()
	if err != nil {
		t.Fatal(err)
	}
	if ledger.Roots["library.libxcb-util1"].Name != "" || ledger.Pool["libxcb-util1:amd64"].Name == "" {
		t.Fatal("demotion lost ownership", ledger)
	}
	state, err := LoadState(c.StatePath, c.Home)
	if err != nil {
		t.Fatal(err)
	}
	receipt := state.Receipts["library.libxcb-util1"]
	if receipt.Status != "removed" || !receipt.After.Present || !receipt.After.RemovalDeferred || !slices.Contains(receipt.After.Preserved, "apt:libxcb-util1:amd64") {
		t.Fatal("retention was not truthfully disclosed", receipt)
	}
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{}, RemoveShared: []string{"library.libxcb-image0"}})
	assertAPTDesktopRemoved(t, f, c)
}

func TestLinuxAPTDesktopLibraryOutsideConsumerSurvives(t *testing.T) {
	f, c := aptDesktopFixture(t)
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"desktop-fixture"}})
	outside := nativePackage{Name: "personal:amd64", Source: "personal", Version: "1", Healthy: true, Dependencies: []string{"libxcb-util1:amd64"}}
	f.installed[outside.Name] = outside
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{}, RemoveShared: []string{"library.libxcb-image0", "library.libxcb-util1"}})
	if _, ok := f.installed[outside.Name]; !ok {
		t.Fatal("removed an outside consumer")
	}
	if _, ok := f.installed["libxcb-util1:amd64"]; !ok {
		t.Fatal("removed an outside consumer's library")
	}
	state, err := LoadState(c.StatePath, c.Home)
	if err != nil {
		t.Fatal(err)
	}
	if state.Receipts["library.libxcb-util1"].Status != "retained" {
		t.Fatal("outside dependency was not retained", state.Receipts)
	}
}

func assertAPTDesktopRemoved(t *testing.T, f *linuxAPTFixture, c Controller) {
	t.Helper()
	ledger, err := f.driver.ledger()
	if err != nil {
		t.Fatal(err)
	}
	if len(ledger.Roots) != 0 || len(ledger.Pool) != 0 {
		t.Fatal("native ownership was left behind", ledger)
	}
	for _, name := range []string{"libxcb-image0:amd64", "libxcb-util1:amd64"} {
		if _, ok := f.installed[name]; ok {
			t.Fatal("owned package survived final removal", name)
		}
	}
	state, err := LoadState(c.StatePath, c.Home)
	if err != nil {
		t.Fatal(err)
	}
	if len(state.Receipts) != 0 {
		t.Fatal("stale retained-root receipt survived collection", state.Receipts)
	}
}

func TestLinuxAPTDesktopRejectsForeignChangesAfterItsOwnInstall(t *testing.T) {
	for _, change := range []string{"manual", "source", "outside-consumer"} {
		t.Run(change, func(t *testing.T) {
			f, c := aptDesktopFixture(t)
			query := f.driver.Query
			changed := false
			f.driver.Query = func(ctx context.Context, privileged bool, program string, input []byte, args ...string) ([]byte, error) {
				ledger, err := f.driver.ledger()
				if err != nil {
					return nil, err
				}
				if !changed && ledger.Roots["library.libxcb-image0"].Name != "" {
					intent, err := f.driver.intent(ledger.Roots["library.libxcb-image0"].CreatedBy)
					if err != nil {
						return nil, err
					}
					if intent.Complete {
						changed = true
						pkg := f.installed["libxcb-util1:amd64"]
						switch change {
						case "manual":
							pkg.Automatic = false
						case "source":
							pkg.Source = "personal-source"
						case "outside-consumer":
							f.installed["outside:amd64"] = nativePackage{Name: "outside:amd64", Source: "outside", Version: "1", Healthy: true, Dependencies: []string{pkg.Name}}
						}
						f.installed[pkg.Name] = pkg
					}
				}
				return query(ctx, privileged, program, input, args...)
			}
			request := Request{Schema: 1, Mode: "apply", Selected: []string{"desktop-fixture"}}
			preview, err := c.Dispatch(context.Background(), request)
			if err != nil {
				t.Fatal(err)
			}
			request.ExpectedPlan = preview.Plan.ID
			if _, err := c.Dispatch(context.Background(), request); err == nil || !strings.Contains(err.Error(), "changed") {
				t.Fatal("outside change widened approval", err)
			}
			ledger, err := f.driver.ledger()
			if err != nil || !changed || ledger.Roots["library.libxcb-util1"].Name != "" || f.runs != 2 {
				t.Fatal("foreign change was promoted or mutated", ledger, f.runs, err)
			}
		})
	}
}

func TestLinuxAPTDesktopDoesNotAbsorbUnattributedInstallations(t *testing.T) {
	f, c := aptDesktopFixture(t)
	run := f.driver.Run
	f.driver.Run = func(ctx context.Context, command nativeCommand) ([]byte, error) {
		out, err := run(ctx, command)
		if slices.Contains(command.Arguments, "install") {
			f.installed["outside:amd64"] = nativePackage{Name: "outside:amd64", Source: "outside", Version: "1", Healthy: true}
		}
		return out, err
	}
	request := Request{Schema: 1, Mode: "apply", Selected: []string{"desktop-fixture"}}
	preview, err := c.Dispatch(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	request.ExpectedPlan = preview.Plan.ID
	if _, err := c.Dispatch(context.Background(), request); err == nil || !strings.Contains(err.Error(), "without operation-specific introduction evidence") {
		t.Fatal("foreign install was absorbed into approval", err)
	}
}

func TestLinuxAPTDesktopChangedConsumerRemainsExternal(t *testing.T) {
	for _, change := range []string{"source", "classification", "held"} {
		t.Run(change, func(t *testing.T) {
			f, c := aptDesktopFixture(t)
			dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"desktop-fixture"}})
			pkg := f.installed["libxcb-image0:amd64"]
			switch change {
			case "source":
				pkg.Source = "personal-source"
			case "classification":
				pkg.Automatic = true
			case "held":
				pkg.Held = true
			}
			f.installed[pkg.Name] = pkg
			dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{}, RemoveShared: []string{"library.libxcb-image0", "library.libxcb-util1"}})
			if _, ok := f.installed["libxcb-util1:amd64"]; !ok {
				t.Fatal("removed library of changed consumer")
			}
			state, err := LoadState(c.StatePath, c.Home)
			if err != nil || state.Receipts["library.libxcb-util1"].Status != "retained" {
				t.Fatal("changed consumer disappeared", state, err)
			}
		})
	}
}

func TestLinuxAPTDesktopManualRootCanBeRetainedAndCollected(t *testing.T) {
	f, c := aptDesktopFixture(t)
	// Install the library first as an explicit manual root. This is a genuine
	// second lifecycle, not a manual promotion of an incidental dependency.
	c.Catalog.Resources = append(c.Catalog.Resources, Resource{ID: "util-only", Name: "Utility", Platforms: []string{"linux"}, Capability: true, Requires: []string{"library.libxcb-util1"}})
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"util-only"}})
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"desktop-fixture"}})
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"image-only"}, RemoveShared: []string{"library.libxcb-util1"}})
	ledger, err := f.driver.ledger()
	if err != nil || ledger.Pool["libxcb-util1:amd64"].Name == "" || f.installed["libxcb-util1:amd64"].Automatic {
		t.Fatal("manual root was lost or reclassified", ledger, err)
	}
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{}, RemoveShared: []string{"library.libxcb-image0"}})
	assertAPTDesktopRemoved(t, f, c)
}

func TestLinuxAPTDesktopReselectionReclaimsOnlyUnchangedRetainedRoots(t *testing.T) {
	for _, change := range []string{"unchanged", "manual", "source"} {
		t.Run(change, func(t *testing.T) {
			f, c := aptDesktopFixture(t)
			c.Catalog.Resources = append(c.Catalog.Resources, Resource{ID: "util-only", Name: "Utility", Platforms: []string{"linux"}, Capability: true, Requires: []string{"library.libxcb-util1"}})
			dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"desktop-fixture"}})
			dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"image-only"}, RemoveShared: []string{"library.libxcb-util1"}})
			pkg := f.installed["libxcb-util1:amd64"]
			if change == "manual" {
				pkg.Automatic = false
			}
			if change == "source" {
				pkg.Source = "personal-source"
			}
			f.installed[pkg.Name] = pkg
			runs := f.runs
			dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"desktop-fixture"}})
			ledger, err := f.driver.ledger()
			if err != nil || (ledger.Roots["library.libxcb-util1"].Name != "") != (change == "unchanged") || f.runs != runs {
				t.Fatal("reselection did not respect retained ownership", ledger, err, f.runs)
			}
			dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"util-only"}, RemoveShared: []string{"library.libxcb-image0"}})
			if _, present := f.installed[pkg.Name]; !present {
				t.Fatal("removing another root collected the selected library")
			}
			state, err := LoadState(c.StatePath, c.Home)
			if err != nil {
				t.Fatal(err)
			}
			receipt := state.Receipts["library.libxcb-util1"]
			if change == "unchanged" && (receipt.Ownership != "created" || receipt.Before.Present) {
				t.Fatal("reselection lost the original absent baseline", receipt)
			}
		})
	}
}
