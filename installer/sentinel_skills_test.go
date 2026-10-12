package installer

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func sentinelSkillsController(t *testing.T) (Controller, *SentinelSkillsDriver) {
	t.Helper()
	root, err := resolveConfigPath(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	repository := filepath.Join(root, "source")
	if err := os.MkdirAll(filepath.Join(repository, "vendor"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := copyConfigPayload(filepath.Join("..", "vendor", "sentinel"), filepath.Join(repository, "vendor", "sentinel")); err != nil {
		t.Fatal(err)
	}
	catalog := &Catalog{Schema: 1, Resources: []Resource{
		{ID: "sentinel-skills", Name: "Sentinel skills", Capability: true, Requires: []string{"skills.sentinel-agents", "skills.sentinel-claude"}},
		{ID: "skills.sentinel-agents", Name: "Shared skills", Action: "sentinel-skills", Scope: "user"},
		{ID: "skills.sentinel-claude", Name: "Claude skills", Action: "sentinel-skills", Scope: "user"},
	}}
	d := &SentinelSkillsDriver{Target: Context{OS: runtime.GOOS, Arch: runtime.GOARCH}, Home: filepath.Join(root, "home"), ClaudeConfigDir: filepath.Join(root, "home", "custom claude"), Repository: repository, Directory: filepath.Join(root, "state"), Node: filepath.Join(root, "private node")}
	native := &NativeDriver{Catalog: catalog, Context: d.Target, StatePath: filepath.Join(d.Directory, "state.json"), SentinelSkills: d, Configurations: &ConfigDriver{Directory: d.Directory}}
	return Controller{Catalog: catalog, Context: d.Target, Source: "sentinel-skills-test", Home: d.Home, StatePath: filepath.Join(d.Directory, "state.json"), Driver: native}, d
}

func TestSentinelVendoredSkillsMatchPinnedProvenance(t *testing.T) {
	var source struct {
		Skills   []string          `json:"skills"`
		Vendored map[string]string `json:"vendored"`
	}
	if err := json.Unmarshal(sentinelProvenance, &source); err != nil {
		t.Fatal(err)
	}
	var catalog struct {
		Groups map[string][]string `json:"groups"`
	}
	root := filepath.Join("..", "vendor", "sentinel")
	data, err := os.ReadFile(filepath.Join(root, "skills", "catalog.json"))
	if err != nil || json.Unmarshal(data, &catalog) != nil {
		t.Fatal("cannot read upstream skill catalog", err)
	}
	if strings.Join(source.Skills, "\n") != strings.Join(catalog.Groups["all"], "\n") || len(source.Skills) != 12 {
		t.Fatal("bundle differs from the pinned upstream all group")
	}
	seen := map[string]bool{}
	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return errors.New("vendored skill source cannot contain redirects")
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		key := filepath.ToSlash(relative)
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(data)
		if source.Vendored[key] != hex.EncodeToString(sum[:]) {
			t.Errorf("vendored source differs from reviewed upstream bytes: %s", key)
		}
		seen[key] = true
		return nil
	})
	if err != nil || len(seen) != len(source.Vendored) {
		t.Fatal("vendored source inventory changed", len(seen), len(source.Vendored), err)
	}
}

func TestSentinelSkillsNativeCopyLifecyclePreservesPersonalData(t *testing.T) {
	c, d := sentinelSkillsController(t)
	request := Request{Schema: 1, Mode: "apply", Selected: []string{"sentinel-skills"}}
	if _, err := c.Preview(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(d.Home); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("preview created a consumer directory", err)
	}
	roots := []string{filepath.Join(d.Home, ".agents", "skills"), filepath.Join(d.ClaudeConfigDir, "skills")}
	for _, root := range roots {
		writeConfigFixture(t, filepath.Join(root, "personal", "SKILL.md"), "Personal skill before install.\n")
	}
	dispatchApproved(t, c, request)
	names, err := sentinelSkillNames()
	if err != nil {
		t.Fatal(err)
	}
	for _, root := range roots {
		for _, name := range names {
			want, err := snapshotConfig(filepath.Join(d.Repository, "vendor", "sentinel", "skills", name))
			got, gotErr := snapshotConfig(filepath.Join(root, name))
			if err != nil || gotErr != nil || got != want || got.Kind != "directory" {
				t.Fatal("installed skill is not an exact independent copy", name, want, got, err, gotErr)
			}
		}
	}
	before, err := snapshotConfig(d.Directory)
	if err != nil {
		t.Fatal(err)
	}
	check, err := c.Dispatch(context.Background(), Request{Schema: 1, Mode: "check"})
	after, afterErr := snapshotConfig(d.Directory)
	if err != nil || afterErr != nil || check.Status != "ready" || before != after {
		t.Fatal("Check changed state or rejected installed skills", check, err, afterErr)
	}
	dispatchApproved(t, c, Request{Schema: 1, Mode: "update"})
	path := filepath.Join(roots[0], "helix", "SKILL.md")
	writeConfigFixture(t, path, "Later personal skill edit.\n")
	check, err = c.Dispatch(context.Background(), Request{Schema: 1, Mode: "check"})
	if err != nil || check.Status != "needs-action" {
		t.Fatal("edited skill reported healthy", check, err)
	}
	dispatchApproved(t, c, Request{Schema: 1, Mode: "repair", Adopt: []string{"skills.sentinel-agents"}})
	state, err := LoadState(c.StatePath, c.Home)
	if err != nil || len(preservedPaths(state)) != len(names) {
		t.Fatal("repair lost the edited skill copy", state, err)
	}
	configuration, resource, _, err := d.configuration(Resource{ID: "skills.sentinel-agents"})
	if err != nil {
		t.Fatal(err)
	}
	journal, err := configuration.readJournal(resource, state.Receipts[resource.ID])
	if err != nil {
		t.Fatal(err)
	}
	saved := ""
	for _, entry := range journal.Entries {
		if filepath.Base(entry.State.Target.Destination) == "helix" {
			saved = filepath.Join(entry.Workspace, "previous", "SKILL.md")
		}
	}
	for cycle := 0; cycle < 2; cycle++ {
		dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{}})
		for _, root := range roots {
			for _, name := range names {
				if _, err := os.Lstat(filepath.Join(root, name)); !errors.Is(err, os.ErrNotExist) {
					t.Fatal("removal retained owned skill", name, err)
				}
			}
			personal := filepath.Join(root, "personal", "SKILL.md")
			data, err := os.ReadFile(personal)
			info, statErr := os.Stat(personal)
			if err != nil || statErr != nil || string(data) != "Personal skill before install.\n" || runtime.GOOS != "windows" && info.Mode().Perm() != 0600 {
				t.Fatal("removal lost personal bytes or permissions", err, statErr)
			}
		}
		data, err := os.ReadFile(saved)
		if err != nil || string(data) != "Later personal skill edit.\n" {
			t.Fatal("removal lost the preserved later edit", err)
		}
		if cycle == 0 {
			// A removed receipt still records preserved edits. It must not grant
			// ownership of a later outside installation, even identical copies.
			for _, identical := range []bool{false, true} {
				for _, name := range names {
					path := filepath.Join(roots[0], name)
					if identical {
						if err := copyConfigPayload(filepath.Join(d.Repository, "vendor", "sentinel", "skills", name), path); err != nil {
							t.Fatal(err)
						}
					} else if name == "helix" {
						writeConfigFixture(t, filepath.Join(path, "SKILL.md"), "Outside installation after removal.\n")
					}
				}
				plan, err := c.Preview(context.Background(), request)
				if err != nil {
					t.Fatal(err)
				}
				for _, op := range plan.Operations {
					if op.Resource == "skills.sentinel-agents" && (op.Action != "pending" || op.Observed.Adoptable) {
						t.Fatal("removed receipt accepted outside skills", identical, op)
					}
				}
				adopt := request
				adopt.Adopt = []string{"skills.sentinel-agents"}
				if _, err := c.Preview(context.Background(), adopt); err == nil {
					t.Fatal("removed receipt offered adoption of outside skills", identical)
				}
				for _, name := range names {
					if err := os.RemoveAll(filepath.Join(roots[0], name)); err != nil {
						t.Fatal(err)
					}
				}
			}
		}
		dispatchApproved(t, c, request)
	}
}

func TestSentinelSkillsAllowSystemHomeRedirect(t *testing.T) {
	c, d := sentinelSkillsController(t)
	if err := os.MkdirAll(d.Home, 0700); err != nil {
		t.Fatal(err)
	}
	linkedHome := filepath.Join(filepath.Dir(d.Home), "linked-home")
	if runtime.GOOS == "windows" {
		if err := createDirectoryLink(d.Home, linkedHome); err != nil {
			t.Fatal(err)
		}
	} else if err := os.Symlink(d.Home, linkedHome); err != nil {
		t.Fatal(err)
	}
	c.Home, d.Home = linkedHome, linkedHome
	d.ClaudeConfigDir = filepath.Join(linkedHome, "custom claude")
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"sentinel-skills"}})
	check, err := c.Dispatch(context.Background(), Request{Schema: 1, Mode: "check"})
	if err != nil || check.Status != "ready" {
		t.Fatal("system home redirect rejected", check, err)
	}
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{}})
}

func TestSentinelSkillsRefuseForeignOwnershipWithoutBlockingOtherRoot(t *testing.T) {
	for _, collision := range []string{"different", "identical", "skill-link", "parent-link", "registry", "lock", "stage"} {
		t.Run(collision, func(t *testing.T) {
			c, d := sentinelSkillsController(t)
			root := filepath.Join(d.Home, ".agents", "skills")
			if err := os.MkdirAll(root, 0700); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(root, "helix")
			source := filepath.Join(d.Repository, "vendor", "sentinel", "skills", "helix")
			switch collision {
			case "different":
				writeConfigFixture(t, filepath.Join(path, "SKILL.md"), "Earlier independent Helix skill")
			case "identical":
				if err := copyConfigPayload(source, path); err != nil {
					t.Fatal(err)
				}
			case "skill-link", "parent-link":
				if collision == "parent-link" {
					path = root
					if err := os.Remove(root); err != nil {
						t.Fatal(err)
					}
				}
				if runtime.GOOS == "windows" {
					if err := createDirectoryLink(source, path); err != nil {
						t.Fatal(err)
					}
				} else if err := os.Symlink(source, path); err != nil {
					t.Fatal(err)
				}
			default:
				name := map[string]string{"registry": ".sentinel-skills.json", "lock": ".sentinel-skills.lock", "stage": ".sentinel-stage-evidence"}[collision]
				writeConfigFixture(t, filepath.Join(root, name), "foreign ownership and recovery evidence")
			}
			before, err := snapshotConfig(root)
			if err != nil {
				t.Fatal(err)
			}
			request := Request{Schema: 1, Mode: "apply", Selected: []string{"sentinel-skills"}, Adopt: []string{"skills.sentinel-agents"}}
			if _, err := c.Preview(context.Background(), request); err == nil {
				t.Fatal("explicit adoption of an outside skill installation was accepted")
			}
			request.Adopt = nil
			plan, err := c.Preview(context.Background(), request)
			if err != nil {
				t.Fatal(err)
			}
			for _, op := range plan.Operations {
				if op.Resource == "skills.sentinel-agents" && (op.Action != "pending" || op.Observed.Adoptable) {
					t.Fatal("foreign skill ownership was accepted", op)
				}
			}
			request.ExpectedPlan = plan.ID
			result, err := c.Dispatch(context.Background(), request)
			if err != nil || result.Status != "needs-action" {
				t.Fatal(result, err)
			}
			after, err := snapshotConfig(root)
			if err != nil || before != after {
				t.Fatal("blocked root changed", err)
			}
			data, err := os.ReadFile(filepath.Join(d.ClaudeConfigDir, "skills", "helix", "SKILL.md"))
			want, wantErr := os.ReadFile(filepath.Join(source, "SKILL.md"))
			if err != nil || wantErr != nil || !bytes.Equal(data, want) {
				t.Fatal("independent root did not install", err, wantErr)
			}
		})
	}
}

func TestSentinelSkillsRefuseSkillsCLILockClaims(t *testing.T) {
	cases := map[string]string{
		"unrelated": `{"version":3,"skills":{"adapt":{"source":"pbakaus/impeccable"}}}`,
		"claimed":   `{"version":3,"skills":{"adapt":{},"impeccable":{"source":"pbakaus/impeccable"}}}`,
		"malformed": `{"skills":`,
	}
	for _, location := range []string{"agents", "xdg-state"} {
		for name, contents := range cases {
			t.Run(location+"-"+name, func(t *testing.T) {
				c, d := sentinelSkillsController(t)
				d.SkillsLocks = skillsCLILocks(d.Home, filepath.Join(d.Home, "state home"))
				lock := d.SkillsLocks[0]
				if location == "xdg-state" {
					lock = d.SkillsLocks[1]
				}
				writeConfigFixture(t, lock, contents)
				before, err := snapshotConfig(lock)
				if err != nil {
					t.Fatal(err)
				}
				request := Request{Schema: 1, Mode: "apply", Selected: []string{"sentinel-skills"}}
				plan, err := c.Preview(context.Background(), request)
				if err != nil {
					t.Fatal(err)
				}
				for _, id := range []string{"skills.sentinel-agents", "skills.sentinel-claude"} {
					op := operation(t, plan, id)
					if name == "unrelated" {
						// A standalone skill under an unbundled name is not a collision.
						if op.Action != "install" {
							t.Fatal("an unrelated skills CLI entry blocked installation", op)
						}
						continue
					}
					if op.Action != "pending" || op.Observed.Adoptable || !strings.Contains(op.Observed.Pending, lock) {
						t.Fatal("a skills CLI claim did not block both roots", op)
					}
					if name == "claimed" && !strings.Contains(op.Observed.Pending, "claims impeccable;") {
						t.Fatal("the refusal does not name the claimed skill", op.Observed.Pending)
					}
				}
				if name == "unrelated" {
					return
				}
				request.ExpectedPlan = plan.ID
				if result, err := c.Dispatch(context.Background(), request); err != nil || result.Status != "needs-action" {
					t.Fatal(result, err)
				}
				for _, root := range []string{filepath.Join(d.Home, ".agents", "skills"), filepath.Join(d.ClaudeConfigDir, "skills")} {
					if _, err := os.Lstat(root); !errors.Is(err, os.ErrNotExist) {
						t.Fatal("a claimed bundle was written", root, err)
					}
				}
				if after, err := snapshotConfig(lock); err != nil || after != before {
					t.Fatal("the skills CLI lock changed", err)
				}
			})
		}
	}
}

// Upstream's complete --remove leaves this exact registry behind. It owns
// nothing, so it allows installation and is never edited or removed.
func TestSentinelSkillsAcceptEmptyUpstreamRegistry(t *testing.T) {
	const empty = "{\n  \"schema_version\": 1,\n  \"roots\": [],\n  \"skills\": {}\n}\n"
	for _, registry := range []string{empty, "{\n  \"schema_version\": 1,\n  \"roots\": [\"helix\"],\n  \"skills\": {\"helix\": {}}\n}\n", "{\"schema_version\": 1}"} {
		c, d := sentinelSkillsController(t)
		roots := []string{filepath.Join(d.Home, ".agents", "skills"), filepath.Join(d.ClaudeConfigDir, "skills")}
		for _, root := range roots {
			writeConfigFixture(t, filepath.Join(root, ".sentinel-skills.json"), registry)
		}
		request := Request{Schema: 1, Mode: "apply", Selected: []string{"sentinel-skills"}}
		plan, err := c.Preview(context.Background(), request)
		if err != nil {
			t.Fatal(err)
		}
		for _, id := range []string{"skills.sentinel-agents", "skills.sentinel-claude"} {
			op := operation(t, plan, id)
			if registry == empty && op.Action != "install" || registry != empty && (op.Action != "pending" || !strings.Contains(op.Observed.Pending, ".sentinel-skills.json")) {
				t.Fatal("upstream registry ownership was misread", registry, op)
			}
		}
		if registry != empty {
			continue
		}
		dispatchApproved(t, c, request)
		dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{}})
		for _, root := range roots {
			if data, err := os.ReadFile(filepath.Join(root, ".sentinel-skills.json")); err != nil || string(data) != empty {
				t.Fatal("the upstream registry changed", root, err)
			}
			if _, err := os.Lstat(filepath.Join(root, "helix")); !errors.Is(err, os.ErrNotExist) {
				t.Fatal("removal left a managed skill", root, err)
			}
		}
	}
}

// A later skills CLI claim on owned copies is disclosed; dotfiles can still
// update or remove what it owns through its own journal.
func TestSentinelSkillsLaterLockClaimKeepsOwnedRemoval(t *testing.T) {
	c, d := sentinelSkillsController(t)
	d.SkillsLocks = skillsCLILocks(d.Home, "")
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"sentinel-skills"}})
	writeConfigFixture(t, d.SkillsLocks[0], `{"version":3,"skills":{"impeccable":{"source":"pbakaus/impeccable"}}}`)
	check, err := c.Dispatch(context.Background(), Request{Schema: 1, Mode: "check"})
	if err != nil || check.Status != "ready" || !strings.Contains(check.Message, "Note: The skills CLI lock "+d.SkillsLocks[0]+" also claims impeccable") {
		t.Fatal("a later claim on owned skills was not disclosed", check, err)
	}
	preview, err := c.Dispatch(context.Background(), Request{Schema: 1, Mode: "apply", Selected: []string{}})
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"skills.sentinel-agents", "skills.sentinel-claude"} {
		if op := operation(t, preview.Plan, id); op.Action != "remove" {
			t.Fatal("a later claim blocked removal of owned skills", op)
		}
	}
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{}})
	if _, err := os.Lstat(filepath.Join(d.Home, ".agents", "skills", "impeccable")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("owned skills were not removed", err)
	}
}
