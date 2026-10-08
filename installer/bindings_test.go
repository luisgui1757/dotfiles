package installer

import (
	"slices"
	"testing"
)

func TestNeovimPrerequisitesHaveConnectedProviders(t *testing.T) {
	catalog, err := DefaultCatalog()
	if err != nil {
		t.Fatal(err)
	}
	for _, target := range []Context{{OS: "darwin", Arch: "arm64"}, linux, {OS: "linux", Arch: "arm64"}, {OS: "windows", Arch: "amd64"}} {
		pins, err := DefaultArchivePins(NativePlatform{Context: target, Libc: "glibc"})
		if err != nil {
			t.Fatal(err)
		}
		ids, err := catalog.Closure([]string{"neovim"}, target)
		if err != nil {
			t.Fatal(err)
		}
		for _, id := range []string{"tool.git", "tool.make", "tool.compiler", "tool.rust", "tool.latex2text"} {
			resource, _ := catalog.Resource(id)
			if resource.Capability || !slices.Contains(ids, id) {
				t.Fatal("missing hidden prerequisite", target, id)
			}
			if resource.Bindings[target.OS].Provider == "archive" {
				if _, ok := pins[id]; !ok {
					t.Fatal("archive route lacks native pin", target, id)
				}
			} else if resource.Bindings[target.OS].Provider == "" && !(target.OS == "darwin" && (id == "tool.make" || id == "tool.compiler")) && !(target.OS == "windows" && id == "tool.compiler") {
				t.Fatal("prerequisite has no native route", target, id)
			}
		}
		if target.OS == "darwin" && !slices.Contains(ids, "infra.apple-clt") {
			t.Fatal("Apple compiler cannot bootstrap")
		}
		if pins["tool.latex2text"].Latex2text.PythonPinID != archivePinID(pins["tool.python"]) {
			t.Fatal("prepared converter is not bound to its interpreter identity")
		}
	}
}

func TestLinuxNativeSelectionsUseAPTWithoutExposingPrerequisites(t *testing.T) {
	catalog, err := DefaultCatalog()
	if err != nil {
		t.Fatal(err)
	}
	for id, name := range map[string]string{"tool.git": "git", "tool.tmux": "tmux", "tool.zsh": "zsh", "tool.make": "make", "tool.compiler": "build-essential", "tool.clangd": "clangd"} {
		r, ok := catalog.Resource(id)
		if !ok || r.Capability || r.Bindings["linux"] != (Binding{Provider: "apt", Package: name}) {
			t.Errorf("%s lacks its private APT prerequisite: %+v", id, r)
		}
	}
}

func TestSearchToolsAreIndependentSelections(t *testing.T) {
	catalog, err := DefaultCatalog()
	if err != nil {
		t.Fatal(err)
	}
	for _, target := range []Context{{OS: "darwin", Arch: "arm64"}, linux, {OS: "linux", Arch: "arm64"}, {OS: "windows", Arch: "amd64"}} {
		for root, tool := range map[string]string{"ripgrep": "tool.rg", "fd": "tool.fd"} {
			if !slices.ContainsFunc(catalog.Capabilities(target), func(r Resource) bool { return r.ID == root }) {
				t.Fatalf("%s is missing from the %s/%s checkboxes", root, target.OS, target.Arch)
			}
			closure, err := catalog.Closure([]string{root}, target)
			if err != nil || !slices.Contains(closure, tool) || !slices.Contains(closure, "integration.shells") {
				t.Fatal("standalone search tool lacks its executable or shell path", root, closure, err)
			}
			for _, id := range closure {
				resource, _ := catalog.Resource(id)
				if resource.Capability && id != root {
					t.Fatal("standalone tool unexpectedly selects another capability", root, id)
				}
			}
		}
	}
}

func TestLinuxClipboardAlwaysProvidesBothDisplayHelpersWithoutAnotherCheckbox(t *testing.T) {
	catalog, err := DefaultCatalog()
	if err != nil {
		t.Fatal(err)
	}
	ids, err := catalog.Closure([]string{"neovim"}, linux)
	if err != nil {
		t.Fatal(err)
	}
	for id, name := range map[string]string{"tool.clipboard": "xclip", "tool.clipboard-wayland": "wl-clipboard"} {
		r, _ := catalog.Resource(id)
		if !slices.Contains(ids, id) || r.Capability || r.Bindings["linux"] != (Binding{Provider: "apt", Package: name}) {
			t.Fatal("clipboard helper must be a hidden automatic prerequisite in every Linux session", id, r)
		}
	}
}

func TestPiUsesPrivateStandaloneArchiveAndCompleteCommandPrerequisites(t *testing.T) {
	catalog, err := DefaultCatalog()
	if err != nil {
		t.Fatal(err)
	}
	for _, target := range []Context{{OS: "darwin", Arch: "arm64"}, linux, {OS: "linux", Arch: "arm64"}, {OS: "windows", Arch: "amd64"}} {
		closure, err := catalog.Closure([]string{"pi"}, target)
		if err != nil {
			t.Fatal(err)
		}
		for _, id := range []string{"tool.pi", "tool.node", "tool.git", "tool.fd", "tool.rg", "integration.shells", "config.pi", "integration.pi"} {
			if !slices.Contains(closure, id) {
				t.Fatalf("Pi lacks %s on %s/%s", id, target.OS, target.Arch)
			}
		}
		resource, _ := catalog.Resource("tool.pi")
		if resource.Shared || resource.Bindings[target.OS].Provider != "archive" {
			t.Fatal("Pi must use removable private archives, not a global npm prefix")
		}
		pins, err := DefaultArchivePins(NativePlatform{Context: target, Libc: "glibc"})
		if err != nil || pins["tool.pi"].Commands["pi"] == "" {
			t.Fatal("Pi lacks its native command artifact", target, err)
		}
	}
}

func TestPackageBindingsKeepInfrastructureOutOfUserChoices(t *testing.T) {
	c, err := DefaultCatalog()
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"infra.nix", "infra.chezmoi"} {
		if _, found := c.Resource(id); found {
			t.Fatalf("retired runtime infrastructure remains in the new graph: %s", id)
		}
	}
	for _, target := range []Context{{OS: "darwin", Arch: "arm64"}, linux, {OS: "windows", Arch: "amd64"}} {
		for _, root := range c.Capabilities(target) {
			if root.ID == "infra.homebrew" || root.ID == "tool.clangd" {
				t.Fatalf("prerequisite exposed as a user choice: %s", root.ID)
			}
		}
		ids, err := c.Closure([]string{"neovim"}, target)
		if err != nil {
			t.Fatal(err)
		}
		if slices.Contains(ids, "tool.clangd") != (target.OS == "linux") {
			t.Fatalf("clangd provider changed on %s: %v", target.OS, ids)
		}
	}
}

func TestCatalogRejectsAmbiguousOrUnprovisionablePackageBindings(t *testing.T) {
	for _, kind := range []string{"duplicate", "unknown-provider", "retired-provider", "empty-package", "unavailable"} {
		t.Run(kind, func(t *testing.T) {
			c := &Catalog{Schema: 1, Resources: []Resource{
				{ID: "package", Name: "Package", Action: "node", Bindings: map[string]Binding{"linux": {Provider: "archive", Package: "node"}}},
			}}
			resource := &c.Resources[0]
			switch kind {
			case "duplicate":
				duplicate := *resource
				duplicate.ID = "alias"
				c.Resources = append(c.Resources, duplicate)
			case "unknown-provider":
				resource.Bindings["linux"] = Binding{Provider: "native", Package: "node"}
			case "retired-provider":
				resource.Bindings["linux"] = Binding{Provider: "nix-home", Package: "node"}
			case "empty-package":
				resource.Bindings["linux"] = Binding{Provider: "archive"}
			case "unavailable":
				resource.Platforms = []string{"windows"}
			}
			if err := c.Validate(); err == nil {
				t.Fatal("invalid binding accepted")
			}
		})
	}
}

func TestMultiplexerChoicesMatchSupportedPlatforms(t *testing.T) {
	c, err := DefaultCatalog()
	if err != nil {
		t.Fatal(err)
	}
	for _, target := range []Context{{OS: "darwin", Arch: "arm64"}, linux, {OS: "windows", Arch: "amd64"}} {
		herdr, err := c.Closure([]string{"herdr"}, target)
		if err != nil || !slices.Contains(herdr, "tool.herdr") {
			t.Fatalf("Herdr unavailable on %s: %v %v", target.OS, herdr, err)
		}
		if slices.Contains(herdr, "tool.tmux") {
			t.Fatalf("Herdr unexpectedly depends on tmux: %v", herdr)
		}
		var choices []string
		for _, capability := range c.Capabilities(target) {
			choices = append(choices, capability.ID)
		}
		wantTmux := target.OS != "windows"
		if slices.Contains(choices, "tmux") != wantTmux {
			t.Fatalf("unexpected multiplexer choices on %s: %v", target.OS, choices)
		}
		for _, id := range []string{"tmux", "tool.tmux", "config.tmux", "tmux.plugins", "tmux.sensible", "tmux.yank", "tmux.resurrect", "tmux.continuum"} {
			_, err := c.Closure([]string{id}, target)
			if (err == nil) != wantTmux {
				t.Fatalf("%s availability on %s: %v", id, target.OS, err)
			}
		}
		if wantTmux {
			ids, err := c.Closure([]string{"tmux"}, target)
			if err != nil || slices.Contains(ids, "tool.git") {
				t.Fatal("verified tmux archives still require an independent Git/plugin installer", ids, err)
			}
		}
	}
}

func TestDashboardUsesItsVerifiedExecutableWithoutExtensionRegistration(t *testing.T) {
	c, err := DefaultCatalog()
	if err != nil {
		t.Fatal(err)
	}
	for _, target := range []Context{{OS: "darwin", Arch: "arm64"}, linux, {OS: "windows", Arch: "amd64"}} {
		ids, err := c.Closure([]string{"gh-dash"}, target)
		if err != nil {
			t.Fatal(err)
		}
		for _, required := range []string{"tool.gh", "tool.gh-dash", "config.gh-dash", "integration.shells"} {
			if !slices.Contains(ids, required) {
				t.Fatalf("dashboard on %s is missing %s", target.OS, required)
			}
		}
		if slices.Contains(ids, "gh.extension") {
			t.Fatalf("dashboard on %s still requires authenticated extension installation", target.OS)
		}
	}
}

func TestPiKeepsRuntimeDependenciesWithoutChangingTheUserNpmPrefix(t *testing.T) {
	c, err := DefaultCatalog()
	if err != nil {
		t.Fatal(err)
	}
	for _, target := range []Context{{OS: "darwin", Arch: "arm64"}, linux, {OS: "windows", Arch: "amd64"}} {
		ids, err := c.Closure([]string{"pi"}, target)
		if err != nil || !slices.Contains(ids, "tool.node") || !slices.Contains(ids, "integration.shells") {
			t.Fatal("Pi lacks its private runtime or shell integration", ids, err)
		}
		if slices.Contains(ids, "integration.npm-prefix") {
			t.Fatal("Pi still changes the user's global npm installation directory")
		}
	}
}
