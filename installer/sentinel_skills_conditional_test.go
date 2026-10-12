package installer

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The native catalog adds the conditional Kiro and Copilot skills consumers and
// lets Kiro CLI or Copilot CLI make its agent present in the same selection.
// The capability requires only the given skill roots: every full copy of the
// twelve bundles is slow on Windows, and the unconditional roots have their own
// lifecycle tests.
func sentinelKiroSkillsController(t *testing.T, requires ...string) (Controller, *SentinelSkillsDriver) {
	t.Helper()
	c, d := sentinelSkillsController(t)
	c.Catalog = &Catalog{Schema: 1, Resources: []Resource{
		{ID: "sentinel-skills", Name: "Sentinel skills", Capability: true, Requires: requires},
		{ID: "kiro-cli", Name: "Kiro CLI", Capability: true, Requires: []string{"tool.kiro"}},
		{ID: "copilot-cli", Name: "Copilot CLI", Capability: true, Requires: []string{"tool.copilot"}},
		{ID: "tool.kiro", Name: "Kiro CLI"},
		{ID: "tool.copilot", Name: "Copilot CLI"},
		{ID: "skills.sentinel-agents", Name: "Shared skills", Action: "sentinel-skills", Scope: "user"},
		{ID: "skills.sentinel-claude", Name: "Claude skills", Action: "sentinel-skills", Scope: "user"},
		{ID: sentinelKiroSkills, Name: "Kiro skills", Action: "sentinel-skills", Scope: "user"},
		{ID: sentinelCopilotSkills, Name: "Copilot skills", Action: "sentinel-skills", Scope: "user"},
	}}
	if err := c.Catalog.Validate(); err != nil {
		t.Fatal(err)
	}
	c.Driver.(*NativeDriver).Catalog = c.Catalog
	return c, d
}

func requireKiroSkills(t *testing.T, d *SentinelSkillsDriver, root string) {
	t.Helper()
	names, err := sentinelSkillNames()
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range names {
		want, err := snapshotConfig(filepath.Join(d.Repository, "vendor", "sentinel", "skills", name))
		got, gotErr := snapshotConfig(filepath.Join(root, name))
		if err != nil || gotErr != nil || got != want || got.Kind != "directory" {
			t.Fatal("Kiro skill is not an exact copy", name, err, gotErr)
		}
	}
}

// previewAction reports what an apply of selected would do to one consumer.
// A preview only inspects, so presence variants need no copies.
func previewAction(t *testing.T, c Controller, id string, selected ...string) string {
	t.Helper()
	plan, err := c.Preview(context.Background(), Request{Schema: 1, Mode: "apply", Selected: selected})
	if err != nil {
		t.Fatal(err)
	}
	return operation(t, plan, id).Action
}

func TestSentinelSkillsLeaveAnAbsentKiroUntouched(t *testing.T) {
	c, d := sentinelKiroSkillsController(t, sentinelKiroSkills)
	kiro := filepath.Join(d.Home, ".kiro")
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"sentinel-skills"}})
	if _, err := os.Lstat(kiro); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("skills created an absent Kiro folder", err)
	}
	check, err := c.Dispatch(context.Background(), Request{Schema: 1, Mode: "check"})
	if err != nil || check.Status != "ready" || operation(t, check.Plan, sentinelKiroSkills).Action != "keep" {
		t.Fatal("an absent Kiro makes Sentinel skills need action", check, err)
	}
	update, err := c.Preview(context.Background(), Request{Schema: 1, Mode: "update"})
	if err != nil || operation(t, update, sentinelKiroSkills).Action != "keep" {
		t.Fatal("update wants to write skills for an absent Kiro", update, err)
	}
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{}})
	state, err := LoadState(c.StatePath, c.Home)
	if err != nil {
		t.Fatal(err)
	}
	if _, recorded := state.Receipts[sentinelKiroSkills]; recorded {
		t.Fatal("deselection kept bookkeeping for an absent Kiro", state.Receipts[sentinelKiroSkills])
	}
	if _, err := os.Lstat(kiro); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("removal created Kiro's folder", err)
	}
}

func TestSentinelSkillsFindEveryPresentKiro(t *testing.T) {
	c, d := sentinelKiroSkillsController(t, sentinelKiroSkills)
	if action := previewAction(t, c, sentinelKiroSkills, "sentinel-skills"); action == "install" {
		t.Fatal("an absent Kiro would receive skills")
	}
	writeConfigFixture(t, filepath.Join(d.Home, ".kiro", "settings", "mcp.json"), "{}\n")
	if action := previewAction(t, c, sentinelKiroSkills, "sentinel-skills"); action != "install" {
		t.Fatal("an existing Kiro folder does not receive skills", action)
	}
	if err := os.RemoveAll(filepath.Join(d.Home, ".kiro")); err != nil {
		t.Fatal(err)
	}
	d.Agents = []string{"kiro"}
	if action := previewAction(t, c, sentinelKiroSkills, "sentinel-skills"); action != "install" {
		t.Fatal("a discovered Kiro command does not receive skills", action)
	}
	d.Agents = nil
	if action := previewAction(t, c, sentinelKiroSkills, "sentinel-skills", "kiro-cli"); action != "install" {
		t.Fatal("selecting Kiro CLI does not bring its skills", action)
	}
	work, err := resolveConfigPath(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	d.KiroHome = work
	if action := previewAction(t, c, sentinelKiroSkills, "sentinel-skills"); action != "install" {
		t.Fatal("an existing KIRO_HOME does not receive skills", action)
	}
}

func TestSentinelSkillsCopyToEachAgentHomeAndRemove(t *testing.T) {
	c, d := sentinelKiroSkillsController(t, sentinelKiroSkills, sentinelCopilotSkills)
	root, err := resolveConfigPath(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	d.KiroHome = filepath.Join(root, "kiro home")
	d.CopilotHome = filepath.Join(root, "copilot home")
	writeConfigFixture(t, filepath.Join(d.KiroHome, "settings", "mcp.json"), "{}\n")
	writeConfigFixture(t, filepath.Join(d.CopilotHome, "config.json"), "{}\n")
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"sentinel-skills"}})
	requireKiroSkills(t, d, filepath.Join(d.KiroHome, "skills"))
	requireKiroSkills(t, d, filepath.Join(d.CopilotHome, "skills"))
	for _, ignored := range []string{".kiro", ".copilot"} {
		if _, err := os.Lstat(filepath.Join(d.Home, ignored)); !errors.Is(err, os.ErrNotExist) {
			t.Fatal("skills ignored KIRO_HOME or COPILOT_HOME", ignored, err)
		}
	}
	check, err := c.Dispatch(context.Background(), Request{Schema: 1, Mode: "check"})
	if err != nil || check.Status != "ready" {
		t.Fatal("skills at KIRO_HOME and COPILOT_HOME are not healthy", check, err)
	}
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{}})
	for _, home := range []string{d.KiroHome, d.CopilotHome} {
		if _, err := os.Lstat(filepath.Join(home, "skills", "helix")); !errors.Is(err, os.ErrNotExist) {
			t.Fatal("removal left a skill copy", home, err)
		}
	}
}

func TestSentinelSkillsNeverRecreateAnUninstalledKiro(t *testing.T) {
	c, d := sentinelKiroSkillsController(t, sentinelKiroSkills)
	kiro := filepath.Join(d.Home, ".kiro")
	writeConfigFixture(t, filepath.Join(kiro, "settings", "mcp.json"), "{}\n")
	request := Request{Schema: 1, Mode: "apply", Selected: []string{"sentinel-skills"}}
	dispatchApproved(t, c, request)
	requireKiroSkills(t, d, filepath.Join(kiro, "skills"))
	if err := os.RemoveAll(kiro); err != nil {
		t.Fatal(err)
	}
	check, err := c.Dispatch(context.Background(), Request{Schema: 1, Mode: "check"})
	if err != nil || check.Status != "needs-action" || !strings.Contains(check.Message, "Kiro is no longer installed") {
		t.Fatal("uninstalled Kiro was not explained", check, err)
	}
	preview, err := c.Preview(context.Background(), request)
	if err != nil || operation(t, preview, sentinelKiroSkills).Action != "pending" {
		t.Fatal("setup planned to recreate an uninstalled Kiro", preview, err)
	}
	if _, err := os.Lstat(kiro); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("inspection recreated Kiro's folder", err)
	}
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{}})
	state, err := LoadState(c.StatePath, c.Home)
	if err != nil || state.Transaction != nil {
		t.Fatal("deselection did not finish", state.Transaction, err)
	}
	if _, err := os.Lstat(kiro); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("deselection recreated Kiro's folder", err)
	}
}

func TestSentinelSkillsReportTheImpeccableEngineOnce(t *testing.T) {
	c, d := sentinelKiroSkillsController(t, "skills.sentinel-agents", "skills.sentinel-claude", sentinelKiroSkills, sentinelCopilotSkills)
	d.Agents = []string{"kiro"}
	plan, err := c.Preview(context.Background(), Request{Schema: 1, Mode: "apply", Selected: []string{"sentinel-skills"}})
	if err != nil {
		t.Fatal(err)
	}
	notes := 0
	for _, op := range plan.Operations {
		for _, note := range op.Observed.Notes {
			if strings.Contains(note, "Impeccable needs its engine") {
				notes++
			}
		}
	}
	if notes != 1 {
		t.Fatal("the shared Impeccable engine note is repeated or missing", notes)
	}
}

func TestSentinelSkillsReachCopilotOnlyAtItsCopilotHome(t *testing.T) {
	t.Run("default-home", func(t *testing.T) {
		// Without COPILOT_HOME, Copilot reads the already covered ~/.agents/skills;
		// a second copy would load every skill twice.
		c, d := sentinelKiroSkillsController(t, "skills.sentinel-agents", sentinelCopilotSkills)
		d.Agents = []string{"copilot"}
		writeConfigFixture(t, filepath.Join(d.Home, ".copilot", "config.json"), "{}\n")
		dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"sentinel-skills", "copilot-cli"}})
		requireKiroSkills(t, d, filepath.Join(d.Home, ".agents", "skills"))
		if _, err := os.Lstat(filepath.Join(d.Home, ".copilot", "skills")); !errors.Is(err, os.ErrNotExist) {
			t.Fatal("Copilot received a duplicate skills copy without COPILOT_HOME", err)
		}
		check, err := c.Dispatch(context.Background(), Request{Schema: 1, Mode: "check"})
		if err != nil || check.Status != "ready" || operation(t, check.Plan, sentinelCopilotSkills).Action != "keep" {
			t.Fatal("Copilot without COPILOT_HOME needs action", check, err)
		}
		dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{}})
		state, err := LoadState(c.StatePath, c.Home)
		if _, recorded := state.Receipts[sentinelCopilotSkills]; err != nil || recorded {
			t.Fatal("deselection kept bookkeeping for the covered Copilot root", err)
		}
	})
	t.Run("copilot-home", func(t *testing.T) {
		c, d := sentinelKiroSkillsController(t, sentinelCopilotSkills)
		root, err := resolveConfigPath(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		d.CopilotHome = filepath.Join(root, "copilot home")
		if action := previewAction(t, c, sentinelCopilotSkills, "sentinel-skills"); action == "install" {
			t.Fatal("an absent Copilot would receive skills at COPILOT_HOME")
		}
		d.Agents = []string{"copilot"}
		if action := previewAction(t, c, sentinelCopilotSkills, "sentinel-skills"); action != "install" {
			t.Fatal("a discovered Copilot command does not receive skills at COPILOT_HOME", action)
		}
		d.Agents = nil
		if action := previewAction(t, c, sentinelCopilotSkills, "sentinel-skills", "copilot-cli"); action != "install" {
			t.Fatal("selecting Copilot CLI does not bring its skills to COPILOT_HOME", action)
		}
		dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"sentinel-skills"}})
		if _, err := os.Lstat(d.CopilotHome); !errors.Is(err, os.ErrNotExist) {
			t.Fatal("skills created COPILOT_HOME for an absent Copilot", err)
		}
		writeConfigFixture(t, filepath.Join(d.CopilotHome, "config.json"), "{}\n")
		if action := previewAction(t, c, sentinelCopilotSkills, "sentinel-skills"); action != "install" {
			t.Fatal("an existing COPILOT_HOME does not receive skills", action)
		}
	})
}

func TestSentinelSkillsKeepCopilotCopiesBoundToTheirCopilotHome(t *testing.T) {
	c, d := sentinelKiroSkillsController(t, sentinelCopilotSkills)
	root, err := resolveConfigPath(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	work := filepath.Join(root, "copilot home")
	writeConfigFixture(t, filepath.Join(work, "config.json"), "{}\n")
	d.CopilotHome = work
	request := Request{Schema: 1, Mode: "apply", Selected: []string{"sentinel-skills"}}
	dispatchApproved(t, c, request)
	d.CopilotHome = ""
	check, err := c.Dispatch(context.Background(), Request{Schema: 1, Mode: "check"})
	if err != nil || check.Status != "needs-action" || !strings.Contains(check.Message, "run setup with the COPILOT_HOME") {
		t.Fatal("owned Copilot copies lost their COPILOT_HOME silently", check, err)
	}
	preview, err := c.Preview(context.Background(), request)
	if err != nil || operation(t, preview, sentinelCopilotSkills).Action != "pending" {
		t.Fatal("owned Copilot copies were retargeted", preview, err)
	}
	d.CopilotHome = work
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{}})
	if _, err := os.Lstat(filepath.Join(work, "skills", "helix")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("removal with the original COPILOT_HOME left a copy", err)
	}
}
