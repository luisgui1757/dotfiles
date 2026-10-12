//go:build darwin || linux

package installer

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// Exercise the real planner, APT driver and desktop/profile publication. Only
// the native package process and download boundaries use fixtures. Theme/font
// resources are excluded so this test cannot register a workstation font.
func linuxCodeControllerFixture(t *testing.T) (*linuxAPTFixture, Controller) {
	t.Helper()
	f, _ := linuxCodeFixture(t)
	full, err := DefaultCatalog()
	if err != nil {
		t.Fatal(err)
	}
	catalog := &Catalog{Schema: 1}
	for _, id := range []string{"vscode", "tool.vscode", "desktop.vscode", "integration.shells"} {
		r, _ := full.Resource(id)
		if id == "vscode" {
			r.Requires = slices.DeleteFunc(slices.Clone(r.Requires), func(id string) bool { return id == "integration.vscode" })
		}
		catalog.Resources = append(catalog.Resources, r)
	}
	home := filepath.Join(filepath.Dir(f.driver.Directory), "home ü with spaces")
	state := filepath.Join(filepath.Dir(f.driver.Directory), "state.json")
	archives := &ArchiveDriver{Directory: filepath.Join(filepath.Dir(state), "packages"), Pins: map[string]ArchivePin{}}
	profiles := &ProfileDriver{Directory: filepath.Join(filepath.Dir(state), "profile-state"), Targets: map[string][]ProfileTarget{"integration.shells": {{Path: filepath.Join(home, ".bashrc"), Script: "fixture shell loader"}}}}
	d := &NativeDriver{Catalog: catalog, Context: linux, APT: f.driver, Archives: archives, Profiles: profiles, StatePath: state, session: newNativeSession(f.driver.WorkerDirectory)}
	d.Desktop = &DesktopDriver{Target: linux, Directory: filepath.Join(filepath.Dir(state), "desktop-state"), Archives: archives, Folders: ConfigFolders{Home: home, Data: filepath.Join(home, ".local", "share")}}
	return f, Controller{Catalog: catalog, Context: linux, Source: "native-code-controller", Home: home, StatePath: state, Driver: d}
}

func TestLinuxCodeControllerUsesPackageDesktopWithoutPrivateLauncher(t *testing.T) {
	_, c := linuxCodeControllerFixture(t)
	personal := filepath.Join(c.Home, ".local", "share", "applications", "personal-code.desktop")
	writeConfigFixture(t, personal, "personal launcher\n")
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"vscode"}})
	state, err := LoadState(c.StatePath, c.Home)
	if err != nil || state.Receipts["tool.vscode"].Ownership != "created" {
		t.Fatal(state, err)
	}
	for _, id := range []string{"desktop.vscode", "integration.shells"} {
		if _, exists := state.Receipts[id]; exists {
			t.Fatal("native package selected redundant private integration", id)
		}
	}
	dispatchApproved(t, c, Request{Schema: 1, Mode: "update", Selected: []string{"vscode"}})
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{}, RemoveShared: []string{"tool.vscode"}})
	if data, err := os.ReadFile(personal); err != nil || string(data) != "personal launcher\n" {
		t.Fatal("personal launcher changed", string(data), err)
	}
	if _, err := os.Lstat(filepath.Join(c.Home, ".bashrc")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("native Code modified the personal shell", err)
	}
}

func TestLinuxCodeVersionChangeDoesNotStrandOwnedRemoval(t *testing.T) {
	f, c := linuxCodeControllerFixture(t)
	// Isolate package ownership from the independently tested launcher graph.
	for i := range c.Catalog.Resources {
		r := &c.Catalog.Resources[i]
		if r.ID == "vscode" {
			r.Requires, r.PlatformRequires = []string{"tool.vscode"}, nil
		} else if r.ID == "tool.vscode" {
			r.Requires, r.PlatformRequires = nil, nil
		}
	}
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"vscode"}})
	pkg := f.installed["code:amd64"]
	pkg.Version = "1.142.0-12345"
	f.installed[pkg.Name] = pkg
	plan, err := c.Preview(context.Background(), Request{Schema: 1, Mode: "update", Selected: []string{"vscode"}})
	if err != nil || operation(t, plan, "tool.vscode").Action != "update" {
		t.Fatal("native version change demanded unsupported adoption", plan, err)
	}
	plan, err = c.Preview(context.Background(), Request{Schema: 1, Mode: "apply", Selected: []string{}, RemoveShared: []string{"tool.vscode"}})
	if err != nil || operation(t, plan, "tool.vscode").Action != "remove" {
		t.Fatal("native version change discarded ownership", plan, err)
	}
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{}, RemoveShared: []string{"tool.vscode"}})
}

func TestLinuxCodeReusedNativeProgramDoesNotRetainShellAttachment(t *testing.T) {
	catalog, err := DefaultCatalog()
	if err != nil {
		t.Fatal(err)
	}
	state := State{Schema: 1, Context: linux, Selected: []string{"vscode"}, Receipts: map[string]Receipt{}}
	observed := map[string]Observation{}
	for _, id := range []string{"tool.vscode", "integration.shells"} {
		o := Observation{Present: true, Healthy: true, Provider: "fixture", Identity: id, Fingerprint: "same"}
		observed[id] = o
		ownership := "created"
		if id == "tool.vscode" {
			ownership = "reused"
		}
		state.Receipts[id] = Receipt{Ownership: ownership, After: o, Status: "ready"}
	}
	plan, err := PlanChanges(catalog, linux, state, Request{Schema: 1, Mode: "apply", Selected: []string{}}, observed, "fixture")
	if err != nil || operation(t, plan, "tool.vscode").Action != "retain" || operation(t, plan, "integration.shells").Action != "remove" {
		t.Fatal("reused native Code retained obsolete shell attachment", plan, err)
	}
}

func TestLinuxCodeLegacyVersionFingerprintUsesCompletedOperation(t *testing.T) {
	for _, changed := range []string{"version", "source", "manual", "forged"} {
		t.Run(changed, func(t *testing.T) {
			f, c := linuxCodeControllerFixture(t)
			dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"vscode"}})
			state, err := LoadState(c.StatePath, c.Home)
			if err != nil {
				t.Fatal(err)
			}
			receipt := state.Receipts["tool.vscode"]
			receipt.After.Fingerprint, err = digest(struct{ Native, Version string }{receipt.After.Fingerprint, f.driver.Code.Version})
			if err != nil {
				t.Fatal(err)
			}
			if changed == "forged" {
				receipt.After.CompletedOperation = ""
			}
			state.Receipts["tool.vscode"] = receipt
			if err := SaveState(c.StatePath, state); err != nil {
				t.Fatal(err)
			}
			pkg := f.installed["code:amd64"]
			switch changed {
			case "version":
				pkg.Version = "1.142.0-12345"
			case "source":
				pkg.Source = "personal-code"
			case "manual":
				pkg.Automatic = true
			}
			f.installed[pkg.Name] = pkg
			plan, err := c.Preview(context.Background(), Request{Schema: 1, Mode: "update", Selected: []string{"vscode"}})
			if err != nil {
				t.Fatal(err)
			}
			op := operation(t, plan, "tool.vscode")
			if changed != "version" {
				if op.Action != "pending" {
					t.Fatal("changed identity was accepted through legacy compatibility", op)
				}
				return
			}
			if op.Action != "update" || !strings.Contains(op.Reason, "downgrade") || !strings.Contains(op.Reason, pkg.Version) || !strings.Contains(op.Reason, f.driver.Code.Version) {
				t.Fatal("review omitted exact downgrade", op)
			}
			dispatchApproved(t, c, Request{Schema: 1, Mode: "update", Selected: []string{"vscode"}})
			state, err = LoadState(c.StatePath, c.Home)
			if err != nil || state.Receipts["tool.vscode"].After.Fingerprint == receipt.After.Fingerprint {
				t.Fatal("legacy fingerprint did not normalize after actual update", state, err)
			}
			intent, err := f.driver.intent(state.Receipts["tool.vscode"].OperationID)
			if err != nil || !intent.Code.AllowDowngrade || !slices.Contains(intent.Commands[len(intent.Commands)-1].Command.Arguments, "--allow-downgrades") {
				t.Fatal(intent, err)
			}
			if f.installed[pkg.Name].Version != f.driver.Code.Version {
				t.Fatal("reviewed pin was not reconciled")
			}
			dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{}, RemoveShared: []string{"tool.vscode"}})
		})
	}
}

func TestApplyDetailIsOptionalDisclosureNotOwnership(t *testing.T) {
	var old Observation
	if err := Decode([]byte(`{"present":true,"healthy":true,"provider":"apt","identity":"apt:code","fingerprint":"native"}`), &old); err != nil {
		t.Fatal(err)
	}
	current := old
	current.ApplyDetail = "downgrade Code to reviewed pin"
	if old.ApplyDetail != "" || !sameArtifact(old, current) {
		t.Fatal("optional disclosure changed legacy ownership", old, current)
	}
	catalog, err := DefaultCatalog()
	if err != nil {
		t.Fatal(err)
	}
	state := State{Schema: 1, Context: linux, Receipts: map[string]Receipt{"tool.vscode": {Ownership: "created", After: old, Status: "ready"}}}
	current.ApplyDetail = strings.Repeat("x", 1025)
	if _, err := PlanChanges(catalog, linux, state, Request{Schema: 1, Mode: "update", Selected: []string{"vscode"}}, map[string]Observation{"tool.vscode": current}, "fixture"); err == nil {
		t.Fatal("oversized disclosure accepted")
	}
}
