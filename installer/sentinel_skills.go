package installer

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

//go:embed sentinel/provenance.json
var sentinelProvenance []byte

// SentinelSkillsDriver copies the pinned upstream bundle through the ordinary
// configuration journal. It never adopts another installer's skill folders or
// registry, follows consumer redirects, or invokes the upstream installer.
type SentinelSkillsDriver struct {
	Target                Context
	Home, ClaudeConfigDir string
	// KiroHome and CopilotHome are the raw KIRO_HOME and COPILOT_HOME values.
	// Agents are the agents discovered at the process boundary, and Selected is
	// the bound resource closure.
	KiroHome, CopilotHome string
	Agents, Selected      []string
	Repository, Directory string
	Node                  string
	// SkillsLocks are the global locks of the skills CLI (npx skills): its
	// ~/.agents default and, when XDG_STATE_HOME is set, its XDG location.
	SkillsLocks []string
}

const sentinelKiroSkills = "skills.sentinel-kiro"
const sentinelCopilotSkills = "skills.sentinel-copilot"

// conditionalSkills is a consumer root that needs its own copy only while its
// agent is present. Kiro reads ${KIRO_HOME:-~/.kiro}/skills. Copilot reads the
// already covered ~/.agents/skills unless COPILOT_HOME is set; then it reads
// only $COPILOT_HOME/skills. Present means the root exists, the agent's command
// or application was found, or this selection installs its CLI. Setup never
// creates an absent agent's root.
type conditionalSkills struct {
	Agent, Name, Tool string
	// Root is empty when the consumer has no separate location.
	Root   string
	Needed bool
}

func (d *SentinelSkillsDriver) conditional(id string) (conditionalSkills, bool, error) {
	var c conditionalSkills
	switch id {
	case sentinelKiroSkills:
		c = conditionalSkills{Agent: "kiro", Name: "Kiro", Tool: "tool.kiro", Root: kiroHome(d.Home, d.KiroHome)}
	case sentinelCopilotSkills:
		c = conditionalSkills{Agent: "copilot", Name: "GitHub Copilot", Tool: "tool.copilot", Root: d.CopilotHome}
		if c.Root == "" {
			return c, true, nil
		}
		if !filepath.IsAbs(c.Root) || filepath.Clean(c.Root) != c.Root || strings.ContainsAny(c.Root, "\x00\r\n") {
			return c, true, errors.New("COPILOT_HOME must be an absolute canonical path for Sentinel skills")
		}
	default:
		return c, false, nil
	}
	_, err := os.Lstat(c.Root)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return c, true, fmt.Errorf("cannot check whether %s is installed at %s: %w", c.Name, c.Root, err)
	}
	c.Needed = err == nil || slices.Contains(d.Agents, c.Agent) || slices.Contains(d.Selected, c.Tool)
	return c, true, nil
}

// Ownership, including an interrupted first copy, keeps the real targets so
// update, recovery and removal still reach every copied folder.
func sentinelSkillsOwned(receipt Receipt) bool {
	return receipt.Ownership == "created" || receipt.Ownership == "uncertain" || receipt.Status == "in-progress"
}

// A consumer without a need has nothing to receive. Selected without
// ownership, it is satisfied without writing anything; deselection or a later
// install observes the real folders again. This is not an observed copy and
// grants nothing: the executor records it as reused bookkeeping.
func (d *SentinelSkillsDriver) unneeded(c conditionalSkills) (Observation, error) {
	fingerprint, err := digest(struct{ Consumer, Root string }{c.Agent, c.Root})
	return Observation{Present: true, Healthy: true, Provider: "sentinel-skills", Identity: "unneeded-consumer:" + c.Agent + ":" + c.Root, Fingerprint: fingerprint, Scope: "user"}, err
}

// skillsCLILocks returns the skills CLI's global lock paths: the ~/.agents
// default always, and the XDG location while XDG_STATE_HOME is set.
func skillsCLILocks(home, stateHome string) []string {
	locks := []string{filepath.Join(home, ".agents", ".skill-lock.json")}
	if stateHome != "" {
		locks = append(locks, filepath.Join(stateHome, "skills", ".skill-lock.json"))
	}
	return locks
}

// The skills CLI can reinstall or delete every agent's folder of a skill it
// records, without per-agent detail. A claim on a bundled name is therefore
// another installer's ownership of both roots: it blocks a new selection, and
// for copies dotfiles already owns it is a note, so dotfiles can still update
// or remove them through its own journal. The lock is never edited.
func (d *SentinelSkillsDriver) guardSkillsLocks(names []string) error {
	lock, claimed, err := d.skillsLockClaims(names)
	if err != nil || len(claimed) == 0 {
		return err
	}
	return fmt.Errorf("the skills CLI lock %s claims %s; remove them with `npx skills remove --global %s` (it deletes every agent's copy of those names) before selecting dotfiles skills", lock, strings.Join(claimed, ", "), strings.Join(claimed, " "))
}

// skillsLockClaims returns the first lock that claims a bundled name, and those
// names. An unreadable or irregular lock is an error: its claims are unknown.
func (d *SentinelSkillsDriver) skillsLockClaims(names []string) (string, []string, error) {
	for _, path := range d.SkillsLocks {
		info, err := os.Lstat(path)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return path, nil, err
		}
		if !info.Mode().IsRegular() {
			return path, nil, fmt.Errorf("skills CLI lock %s is not a regular file; inspect it before selecting dotfiles skills", path)
		}
		data, err := readDocument(path)
		var lock struct {
			Skills map[string]json.RawMessage `json:"skills"`
		}
		if err == nil {
			err = json.Unmarshal(data, &lock)
		}
		if err != nil {
			return path, nil, fmt.Errorf("cannot read skills CLI lock %s: %w; inspect it before selecting dotfiles skills", path, err)
		}
		claimed := []string{}
		for _, name := range names {
			if _, ok := lock.Skills[name]; ok {
				claimed = append(claimed, name)
			}
		}
		if len(claimed) != 0 {
			return path, claimed, nil
		}
	}
	return "", nil, nil
}

func sentinelSkillNames() ([]string, error) {
	var provenance struct {
		Skills []string `json:"skills"`
	}
	if err := json.Unmarshal(sentinelProvenance, &provenance); err != nil {
		return nil, err
	}
	if len(provenance.Skills) == 0 {
		return nil, errors.New("Sentinel skill provenance has no bundle")
	}
	for _, name := range provenance.Skills {
		if !resourceID.MatchString(name) || strings.Contains(name, ".") {
			return nil, errors.New("invalid Sentinel skill name")
		}
	}
	return provenance.Skills, nil
}

func (d *SentinelSkillsDriver) configuration(r Resource) (*ConfigDriver, Resource, string, error) {
	consumer := filepath.Join(d.Home, ".agents")
	switch r.ID {
	case "skills.sentinel-agents":
	case "skills.sentinel-claude":
		consumer = d.ClaudeConfigDir
		if consumer == "" {
			consumer = filepath.Join(d.Home, ".claude")
		}
	case sentinelKiroSkills:
		consumer = kiroHome(d.Home, d.KiroHome)
	case sentinelCopilotSkills:
		consumer = d.CopilotHome
		if consumer == "" {
			return nil, r, "", errors.New("these Copilot skills were installed for a COPILOT_HOME; run setup with the COPILOT_HOME used when they were installed to update or remove them")
		}
	default:
		return nil, r, "", errors.New("unknown Sentinel skills resource")
	}
	if !filepath.IsAbs(consumer) || filepath.Clean(consumer) != consumer || strings.ContainsAny(consumer, "\x00\r\n") {
		return nil, r, "", errors.New("Sentinel skills require an absolute canonical consumer directory")
	}
	root := filepath.Join(consumer, "skills")
	names, err := sentinelSkillNames()
	if err != nil {
		return nil, r, root, err
	}
	targets := []ConfigTarget{}
	for _, name := range names {
		targets = append(targets, ConfigTarget{Source: "vendor/sentinel/skills/" + name, Folder: "home", Path: name, Directory: true, Mode: "copy"})
	}
	configuration, resource, err := integrationConfiguration(d.Repository, d.Target, ConfigFolders{Home: root}, r, targets)
	configuration.Directory = d.Directory
	return configuration, resource, root, err
}

func (d *SentinelSkillsDriver) guardRoot(root string) error {
	// Stop at the observed home, whose system-level canonicalization belongs to
	// folder discovery. An independently overridden consumer is checked to root.
	for path := root; ; path = filepath.Dir(path) {
		if path == d.Home {
			break
		}
		info, err := os.Lstat(path)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		if err == nil && (!info.IsDir() || info.Mode()&(os.ModeSymlink|os.ModeIrregular) != 0) {
			return fmt.Errorf("Sentinel skills preserve redirected or non-directory consumer path %s; reconcile it manually", path)
		}
		if filepath.Dir(path) == path {
			break
		}
	}
	entries, err := os.ReadDir(root)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, entry := range entries {
		path := filepath.Join(root, entry.Name())
		if entry.Name() == ".sentinel-skills.json" && emptySentinelRegistry(path) {
			continue
		}
		if entry.Name() == ".sentinel-skills.json" || entry.Name() == ".sentinel-skills.lock" || strings.HasPrefix(entry.Name(), ".sentinel-stage-") {
			return fmt.Errorf("Sentinel upstream ownership or recovery evidence exists at %s; finish its recovery/removal before selecting dotfiles skills", path)
		}
	}
	return nil
}

// Upstream's complete --remove leaves a valid registry that owns nothing. It
// stays in place, untouched; any other content, or an unreadable file, is
// ownership evidence.
func emptySentinelRegistry(path string) bool {
	data, err := readDocument(path)
	var registry struct {
		Schema int               `json:"schema_version"`
		Roots  []json.RawMessage `json:"roots"`
		Skills map[string]any    `json:"skills"`
	}
	return err == nil && json.Unmarshal(data, &registry) == nil && registry.Schema == 1 && registry.Roots != nil && len(registry.Roots) == 0 && registry.Skills != nil && len(registry.Skills) == 0
}

func (d *SentinelSkillsDriver) guarded(r Resource, receipt Receipt) (*ConfigDriver, Resource, string, error) {
	c, resource, root, err := d.configuration(r)
	if err != nil {
		return c, resource, root, err
	}
	if err := d.guardRoot(root); err != nil {
		return c, resource, root, err
	}
	// Existing owned copies, including interrupted publications, are validated
	// against the saved configuration baseline by ConfigDriver itself.
	if receipt.Ownership != "created" && receipt.Status != "in-progress" {
		names, err := sentinelSkillNames()
		if err == nil {
			err = d.guardSkillsLocks(names)
		}
		if err != nil {
			return c, resource, root, err
		}
		for _, target := range c.Manifest.Targets {
			path := filepath.Join(root, target.Path)
			_, err := os.Lstat(path)
			if err == nil {
				return c, resource, root, fmt.Errorf("Sentinel skill folder %s already exists without dotfiles ownership; preserve and reconcile it manually (no adoption)", path)
			}
			if !errors.Is(err, os.ErrNotExist) {
				return c, resource, root, err
			}
		}
	}
	return c, resource, root, nil
}

func (d *SentinelSkillsDriver) Observe(ctx context.Context, r Resource, receipt Receipt) (Observation, error) {
	vanished := ""
	if c, conditional, err := d.conditional(r.ID); conditional {
		if err != nil {
			return configObservationProblem(receipt, err)
		}
		owned := sentinelSkillsOwned(receipt)
		if !c.Needed && !owned && slices.Contains(d.Selected, r.ID) {
			return d.unneeded(c)
		}
		if !c.Needed && !owned && c.Root == "" {
			return Observation{Provider: "configuration", Scope: "user"}, nil // Nothing was ever located.
		}
		if !c.Needed && owned && c.Root != "" {
			vanished = c.Name + " is no longer installed at " + c.Root + "; setup never recreates its folder. Reinstall it, or deselect Sentinel skills and apply to forget these copies"
		}
	}
	c, resource, root, err := d.guarded(r, receipt)
	if err != nil {
		return configObservationProblem(receipt, err)
	}
	o, err := c.Observe(ctx, resource, receipt)
	o.Adoptable = o.Adoptable && receipt.Ownership == "created"
	if vanished != "" {
		o.ApplyBlocked = strings.TrimSpace(o.ApplyBlocked + " " + vanished)
	}
	if err == nil {
		// Every agent's copy uses the same engine cache, so one note suffices;
		// the Codex/Copilot root is part of every Sentinel skills selection.
		if r.ID == "skills.sentinel-agents" {
			command := shellLiteral(d.Node) + " " + shellLiteral(filepath.Join(root, "impeccable", "scripts", "sentinel-engine.mjs")) + " --setup"
			if d.Target.OS == "windows" {
				command = "& " + powershellLiteral(d.Node) + " " + powershellLiteral(filepath.Join(root, "impeccable", "scripts", "sentinel-engine.mjs")) + " --setup"
			}
			o.Notes = []string{"Impeccable needs its engine once per computer: run " + command + ". Every agent's copy of the skill shares that engine cache, which stays personal and survives removal; Check does not run the engine or agent skills."}
		}
		if receipt.Ownership == "created" || receipt.Status == "in-progress" {
			names, claimErr := sentinelSkillNames()
			lock, claimed := "", []string{}
			if claimErr == nil {
				lock, claimed, claimErr = d.skillsLockClaims(names)
			}
			if claimErr != nil {
				o.Notes = append(o.Notes, "The skills CLI lock could not be checked for claims on these dotfiles-owned skills: "+claimErr.Error())
			} else if len(claimed) != 0 {
				o.Notes = append(o.Notes, "The skills CLI lock "+lock+" also claims "+strings.Join(claimed, ", ")+"; its update or remove commands would replace or delete these dotfiles-owned copies. Dotfiles keeps edited copies on removal.")
			}
		}
	}
	return o, err
}

func (d *SentinelSkillsDriver) Apply(ctx context.Context, r Resource, op Operation, receipt Receipt) (Observation, error) {
	if c, conditional, err := d.conditional(r.ID); conditional {
		if err != nil {
			return Observation{}, err
		}
		if !c.Needed {
			return Observation{}, fmt.Errorf("%s needs no separate skills copy here (not installed, or reading ~/.agents/skills); setup never creates its configuration folder", c.Name)
		}
	}
	c, resource, _, err := d.guarded(r, receipt)
	if err != nil {
		return Observation{}, err
	}
	if op.Action == "adopt" && receipt.Ownership != "created" {
		return Observation{}, errors.New("Sentinel skills cannot adopt unowned folders")
	}
	return c.Apply(ctx, resource, op, receipt)
}

func (d *SentinelSkillsDriver) Remove(ctx context.Context, r Resource, receipt Receipt) (Observation, error) {
	c, resource, _, err := d.guarded(r, receipt)
	if err != nil {
		return Observation{}, err
	}
	return c.Remove(ctx, resource, receipt)
}

func (d *SentinelSkillsDriver) ResumeResource(ctx context.Context, r Resource, op Operation, receipt Receipt) (Observation, error) {
	c, resource, _, err := d.guarded(r, receipt)
	if err != nil {
		return Observation{}, err
	}
	return c.ResumeResource(ctx, resource, op, receipt)
}

func (d *SentinelSkillsDriver) recovery(id string, receipt Receipt) (*ConfigDriver, error) {
	c := &ConfigDriver{Directory: d.Directory}
	j, err := c.readRecoveryJournal(id, receipt)
	if err != nil {
		return c, err
	}
	for _, entry := range j.Entries {
		if err := d.guardRoot(filepath.Dir(entry.State.Target.Destination)); err != nil {
			return c, err
		}
	}
	return c, nil
}

func (d *SentinelSkillsDriver) ObserveRestore(ctx context.Context, id string, receipt Receipt) (Observation, error) {
	c, err := d.recovery(id, receipt)
	if err != nil {
		return configObservationProblem(receipt, err)
	}
	return c.ObserveRestore(ctx, id, receipt)
}

func (d *SentinelSkillsDriver) RestoreResource(ctx context.Context, id string, observed Observation, receipt Receipt) error {
	c, err := d.recovery(id, receipt)
	if err != nil {
		return err
	}
	return c.RestoreResource(ctx, id, observed, receipt)
}
