package installer

import (
	"bytes"
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"slices"
)

//go:embed legacy-profile-evidence.json
var legacyProfileEvidenceData []byte

type legacyEvidenceItem struct{ Tag, Commit, Kind, Source, Blob, SHA256, Content string }
type legacyEvidence struct {
	Schema    int
	Profiles  []legacyEvidenceItem
	BashHooks []legacyEvidenceItem `json:"bash_hooks"`
}

func releasedProfileEvidence() (legacyEvidence, error) {
	var evidence legacyEvidence
	if err := Decode(legacyProfileEvidenceData, &evidence); err != nil {
		return evidence, err
	}
	if evidence.Schema != 1 || len(evidence.Profiles) == 0 || len(evidence.BashHooks) == 0 {
		return evidence, errors.New("invalid released profile evidence")
	}
	for _, item := range append(slices.Clone(evidence.Profiles), evidence.BashHooks...) {
		if !operationID.MatchString(item.SHA256) || len(item.Commit) != 40 || !canonicalRelative(item.Source) {
			return evidence, errors.New("invalid released profile identity")
		}
		if item.Content != "" && legacyHash([]byte(item.Content)) != item.SHA256 {
			return evidence, errors.New("released Bash hook digest mismatch")
		}
	}
	return evidence, nil
}
func legacyHash(data []byte) string { sum := sha256.Sum256(data); return hex.EncodeToString(sum[:]) }
func (e legacyEvidence) matches(kind string, data []byte) bool {
	for _, item := range e.Profiles {
		if item.Kind == kind && item.SHA256 == legacyHash(data) {
			return true
		}
	}
	return false
}

type legacyMigrationDriver struct {
	*ConfigDriver
	profiles []legacyProfile
	evidence legacyEvidence
	checkout string
}
type legacyPreservation struct {
	Schema        int         `json:"schema"`
	Resource      string      `json:"resource"`
	Operation     string      `json:"operation"`
	Destination   string      `json:"destination"`
	Before        Observation `json:"before"`
	Readable      bool        `json:"readable"`
	SHA256        string      `json:"sha256"`
	PayloadSHA256 string      `json:"payload_sha256"`
}

func (d *legacyMigrationDriver) profile(id string) legacyProfile {
	for _, item := range d.profiles {
		if item.ID == id {
			return item
		}
	}
	return legacyProfile{}
}
func (d *legacyMigrationDriver) destination(item legacyProfile) string {
	root := d.Folders.Home
	if item.Folder == "documents" {
		root = d.Folders.Documents
	} else if item.Folder == "config" {
		root = d.Folders.Config
	} else if item.Folder == "local_app_data" {
		root = d.Folders.LocalAppData
	} else if item.Folder == "app_data" {
		root = d.Folders.AppData
	}
	return filepath.Join(root, filepath.FromSlash(item.Path))
}

// Unlike ordinary configuration adoption, migration also freezes readable link
// contents. Opening is bounded and never executes shell/profile content.
func readLegacyProfile(path string) ([]byte, bool, error) {
	before, err := snapshotTree(path, maxProfileBytes, 1)
	if err != nil || before.Kind == "absent" {
		return nil, false, err
	}
	if before.Kind != "file" && before.Kind != "link" {
		return nil, false, errors.New("legacy profile must be a regular file or a file symlink")
	}
	info, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) && before.Kind == "link" {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	if !info.Mode().IsRegular() || info.Size() > maxProfileBytes {
		return nil, false, errors.New("legacy profile referent is not a bounded regular file")
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, false, err
	}
	opened, err := f.Stat()
	if err != nil || !os.SameFile(info, opened) {
		return nil, false, errors.Join(errors.New("legacy profile changed while opening"), err, f.Close())
	}
	data, readErr := io.ReadAll(io.LimitReader(f, maxProfileBytes+1))
	err = errors.Join(readErr, f.Close())
	if err != nil || len(data) > maxProfileBytes {
		return nil, false, errors.Join(errors.New("legacy profile exceeds its read bound"), err)
	}
	if err := verifyConfigSnapshot(path, before); err != nil {
		return nil, false, err
	}
	return data, true, nil
}

func (d *legacyMigrationDriver) replacement(item legacyProfile, data []byte) ([]byte, bool, error) {
	if item.Kind == "config" {
		relevant, err := d.releasedLink(item)
		if err != nil || !relevant {
			return data, relevant, err
		}
		if _, err := os.Stat(d.destination(item)); err != nil {
			return nil, true, errors.New("released configuration link is unreadable; restore its source before migration")
		}
		_, _, err = profileEncoding(data)
		return bytes.Clone(data), true, err
	}
	if item.Kind == "git" {
		info, err := os.Lstat(d.destination(item))
		if errors.Is(err, os.ErrNotExist) {
			return nil, false, nil
		}
		if err != nil {
			return nil, true, err
		}
		if info.Mode()&os.ModeSymlink == 0 {
			return data, false, nil
		}
		if relevant, err := d.releasedLink(item); err != nil || !relevant {
			return data, relevant, err
		}
		if _, err := os.Stat(d.destination(item)); err != nil {
			return nil, true, errors.New("Git configuration link is unreadable; preserve the link and restore its source before migration")
		}
		return bytes.Clone(data), true, validateGitEncoding(data)
	}
	if item.Kind != "bash" {
		encode, _, err := profileEncoding(data)
		if err == nil {
			// New scoped profile blocks have no space after the colon. A
			// first migration must not compete with an existing new owner.
			stripped := bytes.ReplaceAll(data, encode("# >>> dotfiles: "), nil)
			if bytes.Contains(stripped, encode("# >>> dotfiles:")) {
				return nil, true, errors.New("profile contains new installer blocks; preserve it and use setup recovery instead of legacy whole-file replacement")
			}
		}
		return []byte("# Legacy dotfiles profile preserved by migrate. Add personal settings after reviewing the saved original.\n"), true, nil
	}
	for _, evidence := range d.evidence.BashHooks {
		hook := []byte(evidence.Content)
		if bytes.Contains(data, hook) {
			if bytes.Count(data, hook) != 1 {
				return nil, true, errors.New("multiple legacy Bash hooks require manual review; no bytes were removed")
			}
			return bytes.Replace(data, hook, nil, 1), true, nil
		}
	}
	if bytes.Contains(data, []byte("# >>> dotfiles: exec zsh")) || bytes.Contains(data, []byte("# <<< dotfiles: exec zsh")) {
		return nil, true, errors.New("edited or unknown legacy Bash hook: preserve the file and review the marked block manually; migration only removes exact released bytes")
	}
	return data, false, nil
}

func (d *legacyMigrationDriver) configured(id string, payload []byte) *ConfigDriver {
	copy := *d.ConfigDriver
	copy.Manifest.Targets = slices.Clone(d.Manifest.Targets)
	for i := range copy.Manifest.Targets {
		if copy.Manifest.Targets[i].Resource == id {
			copy.Manifest.Targets[i].Source = legacyHash(payload) + ".profile"
		}
	}
	return &copy
}
func (d *legacyMigrationDriver) preservationPath(receipt Receipt) string {
	return filepath.Join(d.Directory, receipt.Recovery, "migration.json")
}
func (d *legacyMigrationDriver) saved(r Resource, receipt Receipt) (legacyPreservation, []byte, error) {
	var saved legacyPreservation
	if receipt.OperationID == "" {
		return saved, nil, os.ErrNotExist
	}
	if err := d.ConfigDriver.validate(r, receipt); err != nil {
		return saved, nil, err
	}
	data, err := readDocument(d.preservationPath(receipt))
	if err != nil {
		return saved, nil, err
	}
	if err := Decode(data, &saved); err != nil {
		return saved, nil, err
	}
	destination, err := bindConfigDestination(d.destination(d.profile(r.ID)))
	if err != nil {
		return saved, nil, err
	}
	if saved.Schema != 1 || saved.Resource != r.ID || saved.Operation != receipt.OperationID || saved.Destination != destination || !sameArtifact(saved.Before, receipt.Before) || saved.Before.Inventory != receipt.Before.Inventory || !operationID.MatchString(saved.PayloadSHA256) || !operationID.MatchString(saved.SHA256) {
		return saved, nil, errors.New("saved migration preservation differs from approved engine intent")
	}
	payload, payloadSnapshot, err := readProfile(filepath.Join(d.Repository, saved.PayloadSHA256+".profile"))
	readable := payloadSnapshot.Kind == "file"
	if err != nil || !readable || legacyHash(payload) != saved.PayloadSHA256 {
		return saved, nil, errors.Join(errors.New("saved migration payload changed; preserve recovery data"), err)
	}
	if saved.Readable {
		original, originalSnapshot, err := readProfile(filepath.Join(d.Directory, receipt.Recovery, "readable-profile"))
		readable := originalSnapshot.Kind == "file"
		if err != nil || !readable || legacyHash(original) != saved.SHA256 {
			return saved, nil, errors.Join(errors.New("saved readable legacy profile changed; preserve recovery data"), err)
		}
	}
	return saved, payload, nil
}

func (d *legacyMigrationDriver) Observe(ctx context.Context, r Resource, receipt Receipt) (Observation, error) {
	if problem := d.profile(r.ID).Problem; problem != "" {
		return configObservationProblem(receipt, errors.New(problem))
	}
	if err := errors.Join(ctx.Err(), d.ConfigDriver.validate(r, receipt)); err != nil {
		return Observation{}, err
	}
	_, payload, savedErr := d.saved(r, receipt)
	if savedErr == nil {
		config := d.configured(r.ID, payload)
		journal, err := config.readJournal(r, receipt)
		if err == nil && journal.Complete && !journal.Cancelled && receipt.Ownership == "created" && receipt.Status == "ready" {
			// New setup may already have added personal/managed blocks. The
			// historical migration verifies its preserved original, not those
			// newer bytes, and never acquires fresh mutation authority.
			baseline, err := config.readBaseline(r, receipt)
			if err != nil {
				return Observation{}, err
			}
			for _, target := range baseline.Targets {
				if err := verifyConfigSnapshot(target.Backup, target.Original); err != nil {
					return Observation{}, err
				}
			}
			return receipt.After, nil
		}
		if err == nil && journal.Cancelled && receipt.Status != "in-progress" {
			savedErr, payload = os.ErrNotExist, nil
		} else if err == nil {
			o, observeErr := config.Observe(ctx, r, receipt)
			return d.withPreserved(o, observeErr, receipt)
		}
		if err != nil && !errors.Is(err, errConfigJournalAbsent) {
			return Observation{}, err
		}
	} else if !errors.Is(savedErr, os.ErrNotExist) {
		return Observation{}, savedErr
	}
	// First inspection of a personal location is item-local: an unavailable
	// path is reported for that item without blocking unrelated migrations.
	// Saved preservation evidence above still fails closed.
	item := d.profile(r.ID)
	if item.Kind == "config" {
		relevant, err := d.releasedLink(item)
		if err != nil {
			return configObservationProblem(receipt, err)
		}
		if !relevant {
			return Observation{Healthy: true, Provider: "configuration", Scope: "user"}, nil
		}
	} else if item.Kind == "git" {
		info, err := os.Lstat(d.destination(item))
		if errors.Is(err, os.ErrNotExist) || err == nil && info.Mode()&os.ModeSymlink == 0 {
			return Observation{Healthy: true, Provider: "configuration", Scope: "user"}, nil
		}
		if err != nil {
			return configObservationProblem(receipt, err)
		}
		relevant, err := d.releasedLink(item)
		if err != nil {
			return configObservationProblem(receipt, err)
		}
		if !relevant {
			return Observation{Healthy: true, Provider: "configuration", Scope: "user"}, nil
		}
	}
	data, readable, err := readLegacyProfile(d.destination(item))
	if err != nil {
		return configObservationProblem(receipt, err)
	}
	desired, relevant, replacementErr := d.replacement(item, data)
	if savedErr == nil && !bytes.Equal(payload, desired) {
		return Observation{}, errors.New("legacy profile changed after preservation; restore the interrupted operation before reviewing a replacement")
	}
	config := d.configured(r.ID, desired)
	states, o, err := inspectConfiguration(config.Catalog, config.Manifest, config.Target, config.Folders, config.Repository, r.ID)
	if err != nil {
		return configObservationProblem(receipt, err)
	}
	if len(states) != 1 {
		return o, errors.New("migration requires one explicit profile per item")
	}
	mode := uint32(0600)
	if runtime.GOOS == "windows" {
		mode = 0666
	} // Native Go reports Windows writable files as 0666.
	expected, err := snapshotEntries([]configEntry{{Path: ".", Kind: "file", Content: legacyHash(desired), Mode: mode}})
	if err != nil {
		return o, err
	}
	if states[0].Input.Kind != "absent" && states[0].Input != expected {
		return o, errors.New("private migration payload changed; preserve it for inspection")
	}
	states[0].Input, states[0].Desired, states[0].Current = expected, expected, configSnapshot{}
	o.Desired, err = digest(states)
	if err != nil {
		return o, err
	}
	o.Inventory, err = digest(struct {
		Readable bool
		SHA256   string
	}{readable, legacyHash(data)})
	if err != nil {
		return o, err
	}
	o.Healthy, o.ApplyBlocked = false, ""
	if !relevant {
		o.Present, o.Adoptable, o.Healthy = false, false, true
	}
	if replacementErr != nil {
		o.ApplyBlocked = replacementErr.Error()
	}
	if item.Kind == "git" && relevant {
		normal, err := LoadState(filepath.Join(filepath.Dir(d.Directory), "state.json"), d.Folders.Home)
		if err != nil {
			// Setup ownership cannot be excluded; only this link waits.
			return configObservationProblem(receipt, err)
		}
		for _, id := range []string{"config.git", gitSettingsResource} {
			if receipt := normal.Receipts[id]; receipt.Ownership == "created" || receipt.Ownership == "uncertain" {
				o.ApplyBlocked = "Git is recorded by setup. Remove that Git selection through setup to restore its original baseline and preserve edits before migrating this link."
			}
		}
	}
	return o, nil
}

func (d *legacyMigrationDriver) withPreserved(o Observation, err error, receipt Receipt) (Observation, error) {
	if err != nil {
		return o, err
	}
	saved, _, err := d.saved(Resource{ID: d.profileResource(receipt), Action: "config"}, receipt)
	if err != nil {
		return o, err
	}
	if saved.Readable {
		o.Preserved = append(o.Preserved, filepath.Join(d.Directory, receipt.Recovery, "readable-profile"))
	}
	if baseline, err := d.ConfigDriver.readBaseline(Resource{ID: saved.Resource, Action: "config"}, receipt); err == nil {
		for _, target := range baseline.Targets {
			if target.Original.Kind != "absent" {
				o.Preserved = append(o.Preserved, target.Backup)
			}
		}
	} else {
		return o, err
	}
	o.Preserved = sortedUnique(o.Preserved)
	return o, nil
}
func (d *legacyMigrationDriver) profileResource(receipt Receipt) string {
	return filepath.Base(receipt.Recovery)
}

func (d *legacyMigrationDriver) Apply(ctx context.Context, r Resource, op Operation, receipt Receipt) (Observation, error) {
	if problem := d.profile(r.ID).Problem; problem != "" {
		return Observation{}, errors.New(problem)
	}
	current, err := d.Observe(ctx, r, receipt)
	if err != nil || !sameArtifact(current, op.Observed) || current.Desired != op.Observed.Desired || current.Inventory != op.Observed.Inventory {
		return current, errors.Join(errors.New("legacy profile or readable symlink contents changed after approval"), err)
	}
	if op.Action != "adopt" || !current.Present || !receipt.Adopted || current.ApplyBlocked != "" {
		return current, errors.New("legacy replacement requires explicit per-item adoption of a present, reviewable profile")
	}
	item := d.profile(r.ID)
	data, readable, err := readLegacyProfile(d.destination(item))
	if err != nil {
		return current, err
	}
	inventory, err := digest(struct {
		Readable bool
		SHA256   string
	}{readable, legacyHash(data)})
	if err != nil || inventory != op.Observed.Inventory {
		return current, errors.Join(errors.New("legacy profile contents changed before preservation"), err)
	}
	payload, _, err := d.replacement(item, data)
	if err != nil {
		return current, err
	}
	destination, err := bindConfigDestination(d.destination(item))
	if err != nil {
		return current, err
	}
	config := d.configured(r.ID, payload)
	if readable {
		if err := saveLegacyBytes(filepath.Join(d.Directory, receipt.Recovery, "readable-profile"), data); err != nil {
			return current, err
		}
	}
	if err := saveLegacyBytes(filepath.Join(config.Repository, legacyHash(payload)+".profile"), payload); err != nil {
		return current, err
	}
	saved := legacyPreservation{Schema: 1, Resource: r.ID, Operation: receipt.OperationID, Destination: destination, Before: receipt.Before, Readable: readable, SHA256: legacyHash(data), PayloadSHA256: legacyHash(payload)}
	if err := saveDocument(d.preservationPath(receipt), saved); err != nil {
		return current, err
	}
	after, err := config.Apply(ctx, r, op, receipt)
	return d.withPreserved(after, err, receipt)
}
func (d *legacyMigrationDriver) Remove(context.Context, Resource, Receipt) (Observation, error) {
	return Observation{}, errors.New("completed migration is historical; normal setup owns subsequent configuration removal")
}
func (d *legacyMigrationDriver) ResumeResource(ctx context.Context, r Resource, op Operation, receipt Receipt) (Observation, error) {
	_, payload, err := d.saved(r, receipt)
	if err != nil {
		return Observation{}, err
	}
	after, err := d.configured(r.ID, payload).ResumeResource(ctx, r, op, receipt)
	return d.withPreserved(after, err, receipt)
}

// Payloads are immutable, content-addressed private files. A partial attempt is
// never trusted or overwritten; only this invocation's unpublished temp is removed.
func saveLegacyBytes(path string, data []byte) (result error) {
	if err := prepareStateDirectory(filepath.Dir(path)); err != nil {
		return err
	}
	existing, err := os.Lstat(path)
	if err == nil {
		if !existing.Mode().IsRegular() {
			return errors.New("migration preservation path is redirected")
		}
		saved, err := os.ReadFile(path)
		if err != nil || !bytes.Equal(saved, data) {
			return errors.Join(errors.New("migration preservation already exists with different bytes"), err)
		}
		return nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".migration-*")
	if err != nil {
		return err
	}
	defer func() {
		if err := os.Remove(f.Name()); err != nil && !errors.Is(err, os.ErrNotExist) {
			result = errors.Join(result, err)
		}
	}()
	if _, err := f.Write(data); err != nil {
		return errors.Join(err, f.Close())
	}
	if err := f.Sync(); err != nil {
		return errors.Join(err, f.Close())
	}
	if err := f.Close(); err != nil {
		return err
	}
	return moveConfigExclusive(f.Name(), path)
}
