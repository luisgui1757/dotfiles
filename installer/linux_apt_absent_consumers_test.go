package installer

import (
	"context"
	"path/filepath"
	"slices"
	"testing"
)

func TestLinuxAPTAbsentAlternativeConsumersRemainVisibleThroughoutLifecycle(t *testing.T) {
	for _, arch := range []string{"amd64", "arm64"} {
		for _, qualifier := range []string{"unqualified", "native", "foreign"} {
			t.Run(arch+"/"+qualifier, func(t *testing.T) {
				f, c := aptDependencyFixtureForArch(t, "library.libadwaita-1-0", "library.libgtk-4-1", arch)
				dependency := "libgtk-4-1"
				if qualifier == "native" {
					dependency += ":" + arch
				} else if qualifier == "foreign" {
					foreign := "arm64"
					if arch == foreign {
						foreign = "amd64"
					}
					dependency += ":" + foreign
				}
				gtk3 := nativePackage{Name: "gtk3:" + arch, Source: "gtk3", Version: "1.0", Healthy: true}
				f.installed[gtk3.Name] = gtk3
				browser := nativePackage{Name: "browser:" + arch, Source: "browser", Version: "1.0", Healthy: true, Dependencies: []string{"gtk3 | " + dependency}}
				f.installed[browser.Name] = browser
				var consumers []string
				if qualifier != "foreign" {
					consumers = []string{browser.Name}
				}
				request := Request{Schema: 1, Mode: "apply", Selected: []string{"desktop-fixture"}}
				preview, err := c.Dispatch(context.Background(), request)
				if err != nil {
					t.Fatal(err)
				}
				request.ExpectedPlan = preview.Plan.ID
				if result, err := c.Dispatch(context.Background(), request); err != nil || result.Status != "ready" {
					t.Fatal("approved native introduction invalidated its own consumer observation", result.Status, err)
				}
				for _, op := range preview.Plan.Operations {
					if op.Resource == "library.libgtk-4-1" && (op.Observed.Present || !slices.Equal(op.Observed.Consumers, consumers)) {
						t.Fatal("absent alternative concealed or fabricated its existing outside consumer", op)
					}
				}
				state, err := LoadState(c.StatePath, c.Home)
				if err != nil {
					t.Fatal(err)
				}
				receipt := state.Receipts["library.libgtk-4-1"]
				if receipt.Before.Present || receipt.Ownership != "created" || !slices.Equal(receipt.Before.Consumers, receipt.After.Consumers) {
					t.Fatal("native pool claim lost its original absence or outside consumer", receipt)
				}
				dispatchApproved(t, c, Request{Schema: 1, Mode: "update"})
				dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{}, RemoveShared: []string{"library.libadwaita-1-0", "library.libgtk-4-1"}})
				state, err = LoadState(c.StatePath, c.Home)
				retained := qualifier != "foreign"
				if err != nil || (state.Receipts["library.libgtk-4-1"].Status == "retained") != retained || (f.installed["libgtk-4-1:"+arch].Name != "") != retained || f.installed[browser.Name].Name == "" || f.installed[gtk3.Name].Name == "" {
					t.Fatal("removal failed to respect the outside browser's exact dependency architecture", state, f.installed, err)
				}
				if _, present := f.installed["libadwaita-1-0:"+arch]; present {
					t.Fatal("unneeded owned root was retained")
				}
				if retained {
					delete(f.installed, browser.Name)
					dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{}, RemoveShared: []string{"library.libgtk-4-1"}})
				}
				if _, present := f.installed["libgtk-4-1:"+arch]; present {
					t.Fatal("formerly protected owned library could not be collected")
				}
				if f.installed[gtk3.Name].Name == "" {
					t.Fatal("outside native library was removed")
				}
			})
		}
	}
}

func TestLinuxAPTInventoryKeepsAbsentAlternativeNames(t *testing.T) {
	rows := "gtk3:amd64\t1\tinstall\tinstalled\tok\tgtk3\t\t\t\t\t\n" +
		"browser:amd64\t1\tinstall\tinstalled\tok\tbrowser\t\tgtk3 | gtk4 (>= 4), missing:any, absent:arm64\t\t\t\n"
	installed, err := parseLinuxAPTInventory([]byte(rows), nil)
	if err != nil || !slices.Equal(installed["browser:amd64"].Dependencies, []string{"absent:arm64", "gtk3:amd64", "gtk4", "missing"}) {
		t.Fatal("uninstalled dependency declarations were discarded", installed, err)
	}
	removal, protected, err := nativeRemovalCandidates([]string{"gtk3:amd64"}, nil, nil, installed)
	if err == nil || len(removal) != 0 || !slices.Equal(protected["gtk3:amd64"], []string{"browser:amd64"}) {
		t.Fatal("absent references widened native removal authority", removal, protected, err)
	}
}

func TestLinuxAPTAbsentReferencesPreserveArchitectureAndProvidesResolution(t *testing.T) {
	rows := "shared:amd64\t1\tinstall\tinstalled\tok\tshared\t\t\tvirtual (= 1)\t\t\n" +
		"shared:arm64\t1\tinstall\tinstalled\tok\tshared\t\t\t\t\t\n" +
		"data:all\t1\tinstall\tinstalled\tok\tdata\t\t\t\t\t\n" +
		"browser:amd64\t1\tinstall\tinstalled\tok\tbrowser\t\tshared:any | missing, shared:arm64, data, virtual\t\t\t\n"
	installed, err := parseLinuxAPTInventory([]byte(rows), nil)
	if err != nil || !slices.Equal(installed["browser:amd64"].Dependencies, []string{"data:all", "missing", "shared:amd64", "shared:arm64"}) {
		t.Fatal("unresolved alternatives changed real architecture or virtual resolution", installed, err)
	}
	if _, err := linuxAPTIdentity("shared", installed); err == nil {
		t.Fatal("unqualified multiarchitecture identity became unambiguous")
	}
	if identity, err := linuxAPTIdentity("shared:arm64", installed); err != nil || identity != "shared:arm64" {
		t.Fatal("exact architecture changed", identity, err)
	}
	if identity, err := linuxAPTIdentity("missing", installed); err != nil || identity != "" {
		t.Fatal("unresolved declaration fabricated an installed package", identity, err)
	}
	candidates, protected, err := nativePoolCandidates([]string{"data:all", "shared:amd64", "shared:arm64"}, nil, nil, installed)
	if err != nil || len(candidates) != 0 || len(protected) != 3 {
		t.Fatal("declared dependency architecture lost removal protection", candidates, protected, err)
	}
}

func TestLinuxAPTLegacyPendingInventoryWithoutAbsentReferencesStillRecovers(t *testing.T) {
	f := newLinuxAPTFixture(t)
	browser := nativePackage{Name: "browser:amd64", Source: "browser", Version: "1", Healthy: true, Dependencies: []string{"shared | absent"}}
	f.installed[browser.Name] = browser
	f.approve()
	f.lostReply = true
	r, receipt := aptFixtureResource("first"), aptFixtureReceipt("legacy-absent-alternative")
	if _, err := f.driver.Apply(context.Background(), r, Operation{Action: "install"}, receipt); err == nil {
		t.Fatal("fixture did not preserve interrupted intent")
	}
	intent, err := f.driver.intent(receipt.OperationID)
	if err != nil {
		t.Fatal(err)
	}
	// Older inventories discarded both absent names. The saved command and
	// operation-specific introduction evidence remain unchanged.
	before := intent.Before[browser.Name]
	before.Dependencies = nil
	intent.Before[browser.Name] = before
	if err := saveDocument(f.driver.intentPath(intent.Operation), intent); err != nil {
		t.Fatal(err)
	}
	f.approve()
	got, err := f.driver.ResumeResource(context.Background(), r, Operation{Action: "install"}, receipt)
	if err != nil || !got.Healthy || got.CompletedOperation != receipt.OperationID || f.runs != 2 {
		t.Fatal("legacy intent repeated native installation or lost completion", got, err, f.runs)
	}
	f.approve()
	if _, err := f.driver.Remove(context.Background(), r, aptFixtureReceipt("remove-legacy-alternative")); err != nil {
		t.Fatal(err)
	}
	ledger, err := f.driver.ledger()
	if err != nil || ledger.Pool["shared:amd64"].Name == "" || f.installed["shared:amd64"].Name == "" || f.installed[browser.Name].Name == "" {
		t.Fatal("legacy recovery erased outside consumer protection", ledger, err)
	}
}

func TestLinuxAPTConfigurationRetainsObservedArchitecture(t *testing.T) {
	for _, arch := range []string{"amd64", "arm64"} {
		t.Run(arch, func(t *testing.T) {
			f := newLinuxAPTFixture(t)
			catalog := &Catalog{Resources: []Resource{{ID: "tool.git", Bindings: map[string]Binding{"linux": {Provider: "apt", Package: "git"}}}}}
			d, err := configureLinuxAPT(&f.driver.Location, catalog, &nativeSession{Directory: filepath.Join(t.TempDir(), "worker")}, arch)
			if err != nil || d.Architecture != arch {
				t.Fatal("APT lost the observed supported architecture", d, err)
			}
		})
	}
	for _, arch := range []string{"", "i386", "unknown"} {
		f := newLinuxAPTFixture(t)
		f.driver.Architecture = arch
		if _, err := f.driver.Observe(context.Background(), aptFixtureResource("first"), Receipt{}); err == nil {
			t.Fatal("APT observed consumers without a supported architecture", arch)
		}
	}
}
