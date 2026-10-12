package installer

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"testing"
)

func TestLegacyNixRootPersonalPackageBlocksRetirement(t *testing.T) {
	h := newLegacyNixHost(t, "linux")
	h.installLinuxUser()
	profile := "/nix/var/nix/profiles/per-user/root"
	h.link(profile+"/profile", "profile-1-link")
	h.link(profile+"/profile-1-link", h.path("/nix/store/root-user-environment"))
	h.file("/nix/store/root-user-environment/manifest.nix", `[ { name = "personal-project"; } ]`)
	h.link("/nix/var/nix/profiles/default", h.path(profile+"/profile"))
	value, err := h.d.nixConsumers(context.Background(), false, "")
	if err != nil || !strings.Contains(value, "account root") {
		t.Fatal("root's personal package was treated as an installer runtime", value, err)
	}
}

// The installer invokes nix-env with these store paths. Keep manifests as
// literal data; never evaluate a Nix expression to decide what may be deleted.
func legacyRootManifestFixture(paths ...string) string {
	entries := []string{}
	for _, path := range paths {
		name := filepath.Base(path)[33:]
		entries = append(entries, fmt.Sprintf(`{ meta = { }; name = %q; out = { outPath = %q; }; outPath = %q; outputs = [ "out" ]; type = "derivation"; }`, name, path, path))
	}
	return "[ " + strings.Join(entries, " ") + " ]"
}

func legacyRootProfileFixture(t *testing.T, h *legacyNixHost) string {
	t.Helper()
	profile := "/nix/var/nix/profiles/per-user/root"
	packages := []string{
		h.path("/nix/store/1kxmbqsah0bszi95w1ii633vzciqnanx-nix-2.34.0"),
		h.path("/nix/store/k5i9jidynqr8kwylgpqf5j8z8pfkr6fl-nix-manual-2.34.0-man"),
		h.path("/nix/store/cv910ahrajv1p2lvy6phasy2b1d9nxcd-nss-cacert-3.117"),
	}
	for i := range packages {
		environment := fmt.Sprintf("/nix/store/root%d-user-environment", i+1)
		h.file(environment+"/manifest.nix", legacyRootManifestFixture(packages[:i+1]...))
		h.link(fmt.Sprintf("%s/profile-%d-link", profile, i+1), h.path(environment))
	}
	h.link(profile+"/profile", "profile-3-link")
	h.link("/nix/var/nix/profiles/default", h.path(profile+"/profile"))
	return h.path("/nix/store/root3-user-environment/manifest.nix")
}

func TestLegacyNixRootDefaultProfileContent(t *testing.T) {
	for _, test := range []struct {
		name     string
		change   func(*legacyNixHost, string)
		accepted bool
	}{
		{"installer packages", func(*legacyNixHost, string) {}, true},
		{"serialized system field", func(h *legacyNixHost, path string) {
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			writeConfigFixture(t, path, strings.ReplaceAll(string(data), `type =`, `system = "unknown"; type =`))
		}, true},
		{"same name different store path", func(h *legacyNixHost, path string) {
			writeConfigFixture(t, path, legacyRootManifestFixture(h.path("/nix/store/00000000000000000000000000000000-nix-2.34.0")))
		}, false},
		{"expression", func(h *legacyNixHost, path string) { writeConfigFixture(t, path, "builtins.abort \"must never run\"") }, false},
		{"old generation personal package", func(h *legacyNixHost, path string) {
			h.file("/nix/store/root1-user-environment/manifest.nix", `[ { name = "personal"; } ]`)
		}, false},
		{"empty manifest", func(h *legacyNixHost, path string) { writeConfigFixture(t, path, "[ ]") }, false},
		{"missing manifest", func(h *legacyNixHost, path string) {
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
		}, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			h := newLegacyNixHost(t, "linux")
			h.installLinuxUser()
			path := legacyRootProfileFixture(t, h)
			test.change(h, path)
			value, err := h.d.nixConsumers(context.Background(), false, "")
			if err != nil || (value == "") != test.accepted {
				t.Fatal("unexpected root-profile consumer decision", value, err)
			}
		})
	}
}

func TestLegacyNixRootProfileSurvivesPartialDeletion(t *testing.T) {
	h := newLegacyNixHost(t, "linux")
	h.installLinuxUser()
	h.passwd["fixture"] = strings.TrimSuffix(h.passwd["fixture"], h.path("/nix/store/bbb-bash-5.2/bin/bash")) + "/bin/bash"
	manifest := legacyRootProfileFixture(t, h)
	interruptedLegacyNixDeletion(t, h)
	if err := os.Remove(manifest); err != nil {
		t.Fatal(err)
	}
	value, err := h.d.nixConsumers(context.Background(), false, "")
	if err != nil || value != "" {
		t.Fatal("approved partial profile could not recover", value, err)
	}
	// Absence in an approved deletion is distinct from changed content.
	writeConfigFixture(t, manifest, `[ { name = "personal"; } ]`)
	value, err = h.d.nixConsumers(context.Background(), false, "")
	if err != nil || !strings.Contains(value, "account root") {
		t.Fatal("recovery accepted a changed manifest", value, err)
	}
}

func TestLegacyNixOldApprovalHasNoRootProfileProof(t *testing.T) {
	h := newLegacyNixHost(t, "linux")
	h.installLinuxUser()
	h.passwd["fixture"] = strings.TrimSuffix(h.passwd["fixture"], h.path("/nix/store/bbb-bash-5.2/bin/bash")) + "/bin/bash"
	manifest := legacyRootProfileFixture(t, h)
	// An older membership document has the same schema but no semantic proof.
	records, err := h.d.nixMembership(context.Background(), h.path("/nix"))
	if err != nil {
		t.Fatal(err)
	}
	legacy := []string{}
	for _, record := range records {
		if record != legacyNixRootProfileProof {
			legacy = append(legacy, record)
		}
	}
	approval, err := digest(legacy)
	if err != nil {
		t.Fatal(err)
	}
	if err := h.d.saveNixMembership(approval, legacy); err != nil {
		t.Fatal(err)
	}
	step, err := h.d.nixDeletionStep("Previously approved deletion", approval)
	if err != nil {
		t.Fatal(err)
	}
	scope := legacyRetirementScope{Steps: []legacyRetirementStep{step}}
	scope.ReplacementState, err = h.d.replacementState()
	if err != nil {
		t.Fatal(err)
	}
	intent := h.approved(scope)
	command, err := h.d.stepCommand(*intent, 0)
	if err != nil {
		t.Fatal(err)
	}
	commandHash, err := digest(command)
	if err != nil {
		t.Fatal(err)
	}
	if err := saveDocument(filepath.Join(h.d.WorkerDirectory, "commands", command.Operation+".json"), nativeCommandRecord{Schema: 1, Command: commandHash}); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(manifest); err != nil {
		t.Fatal(err)
	}
	value, err := h.d.nixConsumers(context.Background(), false, "")
	if err != nil || !strings.Contains(value, "account root") {
		t.Fatal("an old membership file invented root-profile authorization", value, err)
	}
}

// Determinate installers place the Nix from their nix-src input in root's
// default profile. Installer 3.21.2 bundled determinate-nix-3.21.1.
func TestLegacyNixDeterminateRootProfileUsesBundledNixVersion(t *testing.T) {
	for _, test := range []struct {
		nix      string
		accepted bool
	}{{"3.21.1", true}, {"3.21.2", false}} {
		t.Run(test.nix, func(t *testing.T) {
			h, receipt := newLegacyNixHost(t, runtime.GOOS), legacyDeterminateLinuxReceipt
			if runtime.GOOS == "darwin" {
				h.installDarwin(false)
				receipt = legacyDeterminateMacReceipt
				if err := os.Remove(h.path("/nix/var/nix/profiles/default")); err != nil {
					t.Fatal(err)
				}
			} else {
				h.installLinuxUser()
			}
			h.file("/nix/receipt.json", strings.Replace(receipt, `"version":"3.23.3"`, `"version":"3.21.2"`, 1))
			profile := "/nix/var/nix/profiles/per-user/root"
			h.file("/nix/store/root1-user-environment/manifest.nix", legacyRootManifestFixture(
				h.path("/nix/store/s6x1s57vgmiccc1grwwcmphzbffs76cd-nss-cacert-3.121"),
				h.path("/nix/store/byw5n4m44bj1glwx3rldq9j7spz1acp6-determinate-nix-"+test.nix)))
			h.link(profile+"/profile-1-link", h.path("/nix/store/root1-user-environment"))
			h.link(profile+"/profile", "profile-1-link")
			h.link("/nix/var/nix/profiles/default", h.path(profile+"/profile"))
			value, err := h.d.gateValue(context.Background(), legacyRetirementGate{Kind: "nix-consumers"})
			if err != nil || (value == "") != test.accepted {
				t.Fatalf("unexpected root default-profile decision for determinate-nix-%s: %q %v", test.nix, value, err)
			}
		})
	}
}

// Nix registers its downloaded flake registry in the per-user cache as an
// indirect root. Retirement moves that cache aside; other roots stay consumers.
func TestLegacyNixUserCacheRootIsRetiredState(t *testing.T) {
	h := newLegacyNixHost(t, runtime.GOOS)
	registry := filepath.Join(h.home, ".cache", "nix", "flake-registry.json")
	h.file("/nix/store/registry-flake-registry.json", "{}")
	h.linkAt(registry, h.path("/nix/store/registry-flake-registry.json"))
	h.link("/nix/var/nix/gcroots/auto/registry", registry)
	if problems, err := h.d.nixOutsideRoots(context.Background()); err != nil || len(problems) != 0 {
		t.Fatal("Nix's own cache root was treated as an outside consumer", problems, err)
	}
	result := filepath.Join(h.home, "project", "result")
	h.linkAt(result, h.path("/nix/store/registry-flake-registry.json"))
	h.link("/nix/var/nix/gcroots/auto/project", result)
	if problems, err := h.d.nixOutsideRoots(context.Background()); err != nil || len(problems) != 1 || !strings.Contains(problems[0], result) {
		t.Fatal("a project build root lost its protection", problems, err)
	}
}

// After a boot without nix-darwin's activation daemon, /run/current-system is
// gone. The system profile still names the generation whose etc /etc/static
// points into, so its /etc links are restored exactly as before the reboot.
func TestLegacyNixDarwinSystemProfileReplacesMissingActivationLink(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("macOS nix-darwin volume fixture")
	}
	// Fixture roots and the backup names derived from them differ per host.
	retired := regexp.MustCompile(`\.dotfiles-retired-[0-9a-f]+-`)
	normalize := func(h *legacyNixHost, labels []string) []string {
		result := []string{}
		for _, label := range labels {
			if !strings.Contains(label, "/run/current-system") {
				result = append(result, retired.ReplaceAllString(strings.ReplaceAll(label, h.base, "<base>"), ".dotfiles-retired-*-"))
			}
		}
		return result
	}
	before := newLegacyNixHost(t, "darwin")
	before.installDarwin(false)
	want := normalize(before, legacyStepLabels(before.inspect().Steps))
	for _, variant := range []string{"system-profile", "foreign-static", "no-profile"} {
		t.Run(variant, func(t *testing.T) {
			h := newLegacyNixHost(t, "darwin")
			h.installDarwin(false)
			if err := os.Remove(h.path("/run/current-system")); err != nil {
				t.Fatal(err)
			}
			if variant != "no-profile" {
				h.link("/nix/var/nix/profiles/system-1-link", h.path("/nix/store/ggg-darwin-system-26.05"))
				h.link("/nix/var/nix/profiles/system", "system-1-link")
			}
			if variant == "foreign-static" {
				if err := os.Remove(h.path("/etc/static")); err != nil {
					t.Fatal(err)
				}
				h.file("/nix/store/zzz-etc/zshrc", "# another generation\n")
				h.link("/etc/static", h.path("/nix/store/zzz-etc"))
			}
			scope := h.inspect()
			labels := normalize(h, legacyStepLabels(scope.Steps))
			restored := slices.ContainsFunc(labels, func(label string) bool { return strings.Contains(label, "Remove the nix-darwin /etc link") })
			switch variant {
			case "system-profile":
				if scope.Blocked != "" || !slices.Equal(labels, want) {
					t.Fatalf("the system profile did not restore the same nix-darwin scope\ngot:  %q\nwant: %q\n%s", labels, want, scope.Blocked)
				}
			default:
				if restored {
					t.Fatal("nix-darwin was inferred without matching evidence", variant, labels)
				}
			}
		})
	}
}
