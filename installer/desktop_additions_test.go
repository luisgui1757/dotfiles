package installer

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestAdditionalDesktopChoicesHaveIndependentProviders(t *testing.T) {
	catalog, err := DefaultCatalog()
	if err != nil {
		t.Fatal(err)
	}
	for _, target := range []Context{{OS: "darwin", Arch: "arm64"}, linux, {OS: "linux", Arch: "arm64"}, {OS: "windows", Arch: "amd64"}} {
		for _, id := range []string{"openwhispr", "obsidian", "tailscale", "zen"} {
			available := target.OS == "darwin" || target.OS == "linux" && (id != "openwhispr" || target.Arch == "amd64")
			closure, err := catalog.Closure([]string{id}, target)
			if (err == nil) != available {
				t.Fatalf("%s on %+v: %v", id, target, err)
			}
			if !available {
				continue
			}
			if !slices.Contains(closure, "tool."+id) {
				t.Fatal("missing application", id, closure)
			}
			for _, dependency := range closure {
				r, _ := catalog.Resource(dependency)
				if r.Capability && r.ID != id {
					t.Fatal("selected unrelated product", id, dependency)
				}
			}
			if target.OS == "linux" && id == "tailscale" && slices.Contains(closure, "integration.shells") {
				t.Fatal("system package must not own a personal profile")
			}
		}
	}
}

func TestAdditionalDesktopLaunchersUseDeclaredPayloads(t *testing.T) {
	for _, target := range []Context{{OS: "darwin", Arch: "arm64"}, linux} {
		pins, err := DefaultArchivePins(NativePlatform{Context: target, Libc: "glibc"})
		if err != nil {
			t.Fatal(err)
		}
		root, err := resolveConfigPath(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		d := DesktopDriver{Target: target, Directory: root, Folders: ConfigFolders{Home: root, Data: filepath.Join(root, "data")}, Archives: &ArchiveDriver{Directory: filepath.Join(root, "packages"), Pins: pins}}
		ids := []string{"zen"}
		if target.OS == "darwin" {
			ids = append(ids, "obsidian", "openwhispr")
		} else {
			ids = append(ids, "openwhispr")
		}
		for _, id := range ids {
			recipe, err := d.recipe("desktop." + id)
			if err != nil {
				t.Fatal(id, err)
			}
			if target.OS == "linux" {
				if id == "zen" && (recipe.Command != filepath.Join(root, "packages", "tool.zen", "current", "zen") || !strings.HasSuffix(recipe.Icon, "default128.png")) {
					t.Fatal(recipe)
				}
				if id == "openwhispr" && (recipe.Command != filepath.Join(root, "packages", "tool.openwhispr", "current", "open-whispr") || !strings.HasSuffix(recipe.Icon, filepath.Join("resources", "src", "assets", "icon.png"))) {
					t.Fatal(recipe)
				}
			} else if !strings.HasSuffix(recipe.Destination, ".app") || !strings.Contains(filepath.ToSlash(recipe.Source), "/tool."+id+"/current/") {
				t.Fatal(recipe)
			}
		}
	}
}

type packageBytesTransport struct{ data []byte }

func TestCatalogArchitectureRestrictionsKeepLegacyResourcesAvailable(t *testing.T) {
	legacy := Resource{ID: "app", Name: "App", Platforms: []string{"linux"}}
	if !legacy.Available(Context{OS: "linux", Arch: "arm64"}) {
		t.Fatal("legacy catalog shape changed")
	}
	limited := legacy
	limited.PlatformArchitectures = map[string][]string{"linux": {"amd64"}}
	if !limited.Available(linux) || limited.Available(Context{OS: "linux", Arch: "arm64"}) {
		t.Fatal("architecture gate did not match vendor targets")
	}
	c := Catalog{Schema: 1, Resources: []Resource{{ID: "parent", Name: "Parent", Platforms: []string{"linux"}, Requires: []string{"app"}}, limited}}
	if err := c.Validate(); err == nil {
		t.Fatal("accepted an unavailable architecture dependency")
	}
}

func TestTailscaleHealthPreservesPersonalConffiles(t *testing.T) {
	pins, err := defaultLinuxPackagePins("amd64")
	if err != nil {
		t.Fatal(err)
	}
	r := Resource{ID: "tool.tailscale", Name: "Tailscale", Action: "package"}
	for _, tc := range []struct {
		name, output string
		healthy      bool
	}{
		{"unchanged", "", true},
		{"personal service options", "??5?????? c /etc/default/tailscaled\n", true},
		{"missing personal config", "missing   c /etc/default/tailscaled\n", true},
		{"modified daemon", "??5??????   /usr/sbin/tailscaled\n", false},
		{"mixed", "??5?????? c /etc/default/tailscaled\n??5??????   /usr/sbin/tailscaled\n", false},
		{"unrecognized output", "unexpected output\n", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := LinuxAPTDriver{Debs: pins, Checks: map[string][]string{r.ID: {"/usr/bin/dpkg", "--verify", "tailscale"}}, Query: func(context.Context, bool, string, []byte, ...string) ([]byte, error) { return []byte(tc.output), nil }}
			if err := d.check(t.Context(), r); (err == nil) != tc.healthy {
				t.Fatal(err)
			}
		})
	}
}

func (p packageBytesTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(string(p.data))), Header: http.Header{}, Request: request, ContentLength: int64(len(p.data))}, nil
}

func TestNativePackageInputRequiresReviewedBytesAndPreservesChanges(t *testing.T) {
	pins, err := defaultLinuxPackagePins("amd64")
	if err != nil {
		t.Fatal(err)
	}
	pin := pins["tool.tailscale"]
	data := []byte("fixture at the authenticated download boundary")
	hash := sha256.Sum256(data)
	pin.SHA256, pin.Size = hex.EncodeToString(hash[:]), int64(len(data))
	root, err := resolveConfigPath(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	d := LinuxAPTDriver{Directory: root, Client: &http.Client{Transport: packageBytesTransport{data}}}
	if err := d.preparePackage(context.Background(), pin); err != nil {
		t.Fatal(err)
	}
	path := d.packagePayload(pin)
	if err := os.WriteFile(path, []byte("personal changed input"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := d.cleanupPackagePayload(pin); err == nil {
		t.Fatal("changed bytes were eligible for deletion")
	}
	if got, err := os.ReadFile(path); err != nil || string(got) != "personal changed input" {
		t.Fatal(string(got), err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	if err := d.cleanupPackagePayload(pin); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
	pin.SHA256 = strings.Repeat("0", 64)
	if err := d.preparePackage(context.Background(), pin); err == nil {
		t.Fatal("unverified download was accepted")
	}
}

func TestNativePackagePinRejectsForeignSourcesAndTargets(t *testing.T) {
	for _, arch := range []string{"amd64", "arm64"} {
		pins, err := defaultLinuxPackagePins(arch)
		if err != nil {
			t.Fatal(err)
		}
		base := pins["tool.tailscale"]
		for _, change := range []func(*LinuxPackagePin){
			func(p *LinuxPackagePin) { p.URL = "https://example.org/tailscale.deb" },
			func(p *LinuxPackagePin) { p.Architecture = "386" },
			func(p *LinuxPackagePin) { p.Package = "other" },
			func(p *LinuxPackagePin) { p.Version = "latest" },
			func(p *LinuxPackagePin) { p.Size = maxArchiveBytes + 1 },
		} {
			pin := base
			change(&pin)
			if pin.validate() == nil {
				t.Fatal("invalid vendor input accepted", pin)
			}
		}
	}
}

func TestDiskImagePinRequiresOneSafeBundle(t *testing.T) {
	pins, err := DefaultArchivePins(NativePlatform{Context: Context{OS: "darwin", Arch: "arm64"}})
	if err != nil {
		t.Fatal(err)
	}
	base := pins["tool.obsidian"]
	for _, bundle := range []string{"", "../Obsidian.app", "/Obsidian.app", "Obsidian.app/Contents", "Other"} {
		pin := base
		pin.AppBundle = bundle
		if pin.Validate() == nil {
			t.Fatal("unsafe disk-image bundle accepted", bundle)
		}
	}
}
