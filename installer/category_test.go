package installer

import (
	"context"
	"slices"
	"testing"
)

func TestCapabilityCategoriesFollowPlatformAvailability(t *testing.T) {
	for _, target := range []Context{{OS: "darwin", Arch: "arm64"}, linux, {OS: "linux", Arch: "arm64"}, {OS: "windows", Arch: "amd64"}} {
		t.Run(target.OS+"/"+target.Arch, func(t *testing.T) {
			c, _ := controllerFixture(t)
			var err error
			c.Catalog, err = DefaultCatalog()
			if err != nil {
				t.Fatal(err)
			}
			c.Context = target
			ui := &scriptedInteraction{t: t, cancelAt: 1}
			if _, err := c.Run(context.Background(), ui); err != nil {
				t.Fatal(err)
			}
			choices := ui.menus[0]
			categories := []string{}
			for _, choice := range choices {
				r, ok := c.Catalog.Resource(choice.ID)
				if !ok || !r.Capability || !r.Available(target) || choice.Category == "" || choice.Category != r.Category {
					t.Fatal("selector lost a category or exposed an unavailable/non-capability resource", choice)
				}
				if len(categories) == 0 || categories[len(categories)-1] != choice.Category {
					categories = append(categories, choice.Category)
				}
			}
			wantCategories := []string{"Editors", "Shell essentials", "Version control", "Terminal apps", "Terminal sessions", "AI coding", "Desktop apps", "Dotfiles development"}
			if target.OS == "windows" {
				wantCategories = slices.DeleteFunc(wantCategories, func(category string) bool { return category == "Desktop apps" })
			}
			if !slices.Equal(categories, wantCategories) {
				t.Fatal("category order differs across platforms or splits a category", categories)
			}
			if len(choices) != len(c.Catalog.Capabilities(target)) {
				t.Fatal("categorization changed selectable tools")
			}
			for id, want := range map[string]bool{
				"zsh": target.OS != "windows", "powershell": target.OS == "windows",
				"aerospace": target.OS == "darwin", "tmux": target.OS != "windows",
				"windows-terminal": target.OS == "windows",
				"openwhispr":       target.OS == "darwin" || target.OS == "linux" && target.Arch == "amd64",
			} {
				if got := slices.ContainsFunc(choices, func(choice Choice) bool { return choice.ID == id }); got != want {
					t.Fatal("platform filtering changed", target, id, got)
				}
			}
			terminal, _ := c.Catalog.Resource("windows-terminal")
			if terminal.Category != "Terminal apps" {
				t.Fatal("Windows Terminal has the wrong category", terminal.Category)
			}
			for _, id := range []string{"zsh", "powershell", "starship", "lsd", "zoxide", "fzf", "ripgrep", "fd"} {
				r, _ := c.Catalog.Resource(id)
				if r.Category != "Shell essentials" {
					t.Fatal("shell tool is outside its bulk selection", id, r.Category)
				}
			}
		})
	}
}

func TestCatalogCategoryValidation(t *testing.T) {
	for _, test := range []struct {
		name, first, second, hidden string
		valid                       bool
	}{
		{name: "legacy flat catalog", valid: true},
		{name: "complete grouping", first: "Tools", second: "Other tools", valid: true},
		{name: "missing category", first: "Tools"},
		{name: "hidden category", first: "Tools", second: "Tools", hidden: "Tools"},
		{name: "padded category", first: " Tools", second: "Tools"},
		{name: "blank category", first: " ", second: "Tools"},
		{name: "control character", first: "Shell\x1b[2Jessentials", second: "Tools"},
	} {
		t.Run(test.name, func(t *testing.T) {
			c := &Catalog{Schema: 1, Resources: []Resource{
				{ID: "first", Name: "First", Capability: true, Category: test.first},
				{ID: "second", Name: "Second", Capability: true, Category: test.second},
				{ID: "hidden", Name: "Hidden", Action: "archive", Category: test.hidden},
			}}
			if err := c.Validate(); (err == nil) != test.valid {
				t.Fatal("unexpected category validation", err)
			}
		})
	}
}

// Removal is unticking in Choose tools: the list keeps every available tool,
// with the saved leaf selections checked.
func TestCategorizedSelectionStartsFromExistingLeafSelections(t *testing.T) {
	c, d := controllerFixture(t)
	runController(t, c, []string{"editor"})
	before, err := LoadState(d.statePath, d.home)
	if err != nil {
		t.Fatal(err)
	}
	// Add display metadata after saving an old selection. No state migration is
	// needed, and unselected siblings must never enter the removal category.
	for i := range c.Catalog.Resources {
		if c.Catalog.Resources[i].Capability {
			c.Catalog.Resources[i].Category = "Tools"
		}
	}
	ui := &scriptedInteraction{t: t, answers: [][]string{{"apply"}}, cancelAt: 2}
	if _, err := c.Run(context.Background(), ui); err != nil {
		t.Fatal(err)
	}
	if len(ui.menus[1]) != 2 || slices.ContainsFunc(ui.menus[1], func(choice Choice) bool { return choice.Category != "Tools" }) || !slices.Equal(ui.initials[1], []string{"editor"}) {
		t.Fatal("tool selection lost categories or saved selections", ui.menus, ui.initials)
	}
	after, err := LoadState(d.statePath, d.home)
	if err != nil || !slices.Equal(before.Selected, after.Selected) || after.Generation != before.Generation {
		t.Fatal("opening categorized selection changed saved selection", after, err)
	}
}

func TestCategorizedSelectionFollowsCatalogOrder(t *testing.T) {
	c, _ := controllerFixture(t)
	var err error
	c.Catalog, err = DefaultCatalog()
	if err != nil {
		t.Fatal(err)
	}
	state := State{Schema: 1, Context: c.Context, Target: c.Home, Source: c.Source, Selected: []string{"claude-code", "neovim"}}
	if err := SaveState(c.StatePath, state); err != nil {
		t.Fatal(err)
	}
	u := &scriptedInteraction{t: t, answers: [][]string{{"apply"}}, cancelAt: 2}
	if _, err := c.Run(context.Background(), u); err != nil {
		t.Fatal(err)
	}
	neovim := slices.IndexFunc(u.menus[1], func(choice Choice) bool { return choice.ID == "neovim" && choice.Category == "Editors" })
	claude := slices.IndexFunc(u.menus[1], func(choice Choice) bool { return choice.ID == "claude-code" && choice.Category == "AI coding" })
	if neovim < 0 || claude < 0 || neovim > claude {
		t.Fatal("tool order follows saved IDs instead of the catalog", u.menus[1])
	}
}

func TestCategorizedOwnershipDecisionsRemainFlat(t *testing.T) {
	for _, scenario := range []string{"adoption", "shared removal and retention"} {
		t.Run(scenario, func(t *testing.T) {
			c, _ := controllerFixture(t)
			resource := "node"
			if scenario == "adoption" {
				c, _ = adoptionFixture(t)
				resource = "plugins"
			}
			// Give the resource in the ownership prompt its own category, so
			// copying metadata there would fail the scripted interaction guard.
			for i := range c.Catalog.Resources {
				r := &c.Catalog.Resources[i]
				if r.ID == resource {
					r.Capability = true
				}
				if r.Capability {
					r.Category = "Tools"
				}
			}
			if scenario == "adoption" {
				u := &scriptedInteraction{t: t, answers: [][]string{{"editor"}, {"plugins"}}, accept: true}
				result, err := c.Run(context.Background(), u)
				if err != nil || result.Status != "ready" || !slices.Contains(u.titles, ReviewAdopt) {
					t.Fatal("categorized adoption did not reach its ownership prompt", result, err, u.titles)
				}
				return
			}
			runController(t, c, []string{"agent"})
			u := &scriptedInteraction{t: t, answers: [][]string{{"apply"}, {}, {}}, accept: true}
			result, err := c.Run(context.Background(), u)
			if err != nil || result.Status != "ready" || !slices.Contains(u.titles, ReviewShared) {
				t.Fatal("categorized removal did not reach its ownership prompt", result, err, u.titles)
			}
			u = &scriptedInteraction{t: t, answers: [][]string{{"apply"}, {}, {}, {"node"}}, accept: true}
			result, err = c.Run(context.Background(), u)
			if err != nil || result.Status != "ready" || !slices.Contains(u.titles, ReviewKeep) {
				t.Fatal("categorized retention did not reach its ownership prompt", result, err, u.titles)
			}
		})
	}
}
