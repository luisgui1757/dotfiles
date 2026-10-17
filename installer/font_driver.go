package installer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// FontDriver adds native font activation to the existing configuration lifecycle.
// It owns only twelve fixed faces and their exact per-user registrations. The
// private font archive remains a separate prerequisite owned by ArchiveDriver.
type FontDriver struct {
	Target                           Context
	Folders                          ConfigFolders
	Directory                        string
	Archives                         *ArchiveDriver
	PowerShell, FontCache, FontMatch string
	Run                              func(context.Context, nativeCommand) ([]byte, error)
	Query                            func(context.Context, nativeCommand) ([]byte, error)
}

func (d *FontDriver) configuration(r Resource) (*ConfigDriver, Resource, []string, string, error) {
	if r.ID != "tool.font" || d.Archives == nil {
		return nil, r, nil, "", errors.New("unknown fixed font integration")
	}
	pin, err := d.Archives.validate("font.hack")
	if err != nil {
		return nil, r, nil, "", err
	}
	targets := []ConfigTarget{}
	paths := []string{}
	folders := d.Folders
	switch d.Target.OS {
	case "darwin":
		folders.Home = filepath.Join(d.Folders.Home, "Library", "Fonts")
	case "linux":
		folders.Home = d.Folders.Data
		if folders.Home == "" {
			folders.Home = filepath.Join(d.Folders.Home, ".local", "share")
		}
		folders.Home = filepath.Join(folders.Home, "fonts")
	case "windows":
		if !filepath.IsAbs(d.Folders.LocalAppData) {
			return nil, r, nil, "", errors.New("font integration requires actual LocalAppData")
		}
		folders.Home = filepath.Join(d.Folders.LocalAppData, "Microsoft", "Windows", "Fonts")
	default:
		return nil, r, nil, "", errors.New("unsupported font platform")
	}
	for _, face := range hackFontFaces() {
		source, err := d.Archives.RequiredFilePath("font.hack", face.File)
		if err != nil {
			return nil, r, nil, "", err
		}
		relative, err := integrationRelative(d.Directory, source)
		if err != nil {
			return nil, r, nil, "", err
		}
		mode := ""
		if d.Target.OS == "windows" {
			mode = "copy"
		}
		name := "Dotfiles-" + face.File
		if d.Target.OS == "linux" {
			name = "Dotfiles-HackNerdFont/" + face.File
		}
		targets = append(targets, ConfigTarget{Source: relative, Folder: "home", Path: name, Mode: mode})
		paths = append(paths, filepath.Join(folders.Home, filepath.FromSlash(name)))
	}
	c, cr, err := integrationConfiguration(d.Directory, d.Target, folders, r, targets)
	if err != nil {
		return nil, r, nil, "", err
	}
	desired, err := digest(struct {
		Pin     ArchivePin
		Targets []ConfigTarget
		Folder  string
	}{pin, targets, folders.Home})
	return c, cr, paths, desired, err
}
func fontRegistrations(probe []fontNativeFace) []string {
	values := make([]string, len(probe))
	for i := range probe {
		values[i] = probe[i].Registration
	}
	return values
}
func emptyFontInventory() string {
	hash, _ := digest(make([]string, len(hackFontFaces())))
	return hash
}
func fontNativeHealthy(probe []fontNativeFace) bool {
	faces := hackFontFaces()
	if len(probe) != len(faces) {
		return false
	}
	for i, face := range faces {
		if probe[i].Name != face.PostScript || !probe[i].Glyph || !operationID.MatchString(probe[i].Hash) {
			return false
		}
	}
	return true
}
func (d *FontDriver) nativeOwned(probe []fontNativeFace, paths []string) bool {
	if !fontNativeHealthy(probe) {
		return false
	}
	for i, path := range paths {
		if d.Target.OS == "windows" && probe[i].Registration != path {
			return false
		}
		hash, err := fontFileHash(path)
		if err != nil || hash != probe[i].Hash {
			return false
		}
	}
	return true
}
func (d *FontDriver) Observe(ctx context.Context, r Resource, receipt Receipt) (Observation, error) {
	c, cr, paths, desired, err := d.configuration(r)
	if err != nil {
		return Observation{}, err
	}
	o, err := c.Observe(ctx, cr, receipt)
	if err != nil || o.Unknown {
		return o, err
	}
	o.Desired, o.ApplyBlocked = desired, ""
	j, jerr := c.readJournal(cr, receipt)
	if jerr != nil && !errors.Is(jerr, errConfigJournalAbsent) {
		return configObservationProblem(receipt, jerr)
	}
	removed := jerr == nil && j.Complete && (j.Action == "remove" || j.Cancelled)

	if removed && d.Target.OS == "linux" && !o.Present {
		o.Inventory = emptyFontInventory()
		return o, nil
	}
	probe, err := d.query(ctx, paths)
	if err != nil {
		return configObservationProblem(receipt, err)
	}
	o.Inventory, err = digest(fontRegistrations(probe))
	if err != nil {
		return o, err
	}
	if receipt.Ownership != "created" && receipt.Status != "in-progress" && fontNativeHealthy(probe) {
		identity, _ := digest(probe)
		return Observation{Present: true, Healthy: true, Provider: "native-font", Identity: "Hack Nerd Font family", Fingerprint: identity, Scope: "user", Inventory: o.Inventory, Desired: desired}, nil
	}
	o.Adoptable = false
	if o.Present && receipt.Ownership != "created" && receipt.Status != "in-progress" {
		o.ApplyBlocked = "preexisting font destination is retained; choose another font installation or resolve this exact collision"
		return o, nil
	}
	for i, value := range fontRegistrations(probe) {
		if value != "" && (value != paths[i] || receipt.Before.Inventory != emptyFontInventory()) {
			o.Healthy, o.Adoptable, o.CompletedOperation = false, false, ""
			o.ApplyBlocked = "a preexisting or changed font registration is retained; preserve it and resolve that exact value before installing"
			o.Consumers = []string{"preexisting or changed current-user font registration: " + hackFontFaces()[i].File}
			return o, nil
		}
	}
	if removed {
		return o, nil
	}
	o.Healthy = o.Healthy && d.nativeOwned(probe, paths)
	if !o.Healthy {
		o.CompletedOperation = ""
	}
	if receipt.Status == "in-progress" && jerr == nil && !j.Restoring && !j.Cancelled || receipt.Status == "in-progress" && receipt.Ownership == "created" && jerr != nil {
		// File publication and native activation are separately observable. Its
		// durable file intent remains the only ownership/recovery authority.
		if !o.Healthy || j.Action == "remove" && !j.Complete {
			token, e := digest(struct {
				Files  Observation
				Native []fontNativeFace
			}{o, probe})
			if e != nil {
				return o, e
			}
			o.ResourceResume = &ResourceResume{Operation: receipt.OperationID, Token: token}
			o.Pending = "font publication or native activation is unfinished; resume the saved operation"
		}
	}
	if o.Present && !o.Healthy && o.Pending == "" {
		o.HealthIssue = "the native font engine did not select every managed face and Nerd Font glyph"
	}
	return o, nil
}
func (d *FontDriver) Apply(ctx context.Context, r Resource, op Operation, receipt Receipt) (Observation, error) {
	c, cr, paths, desired, err := d.configuration(r)
	if err != nil {
		return Observation{}, err
	}
	before, err := d.Observe(ctx, r, receipt)
	if err != nil || !sameArtifact(before, op.Observed) || before.Inventory != op.Observed.Inventory || desired != op.Observed.Desired || before.ApplyBlocked != "" {
		return before, errors.Join(errors.New("font integration changed after approval or has a protected registration"), err)
	}
	archive, err := d.Archives.Observe(ctx, Resource{ID: "font.hack"}, Receipt{})
	if err != nil || !archive.Healthy {
		return before, errors.Join(errors.New("font integration requires its verified archive"), err)
	}
	if receipt.Before.Present || receipt.Before.Inventory != emptyFontInventory() {
		return before, errors.New("font creation requires originally absent exact registration values")
	}
	_, actual, err := inspectConfiguration(c.Catalog, c.Manifest, c.Target, c.Folders, c.Repository, cr.ID)
	if err != nil {
		return before, err
	}
	op.Observed.Desired = actual.Desired
	if receipt.Ownership == "created" && d.Target.OS == "windows" {
		if err = d.activate(ctx, c, cr, paths, receipt, false); err != nil {
			return before, err
		}
	}
	if _, err = c.Apply(ctx, cr, op, receipt); err != nil {
		return before, err
	}
	if err = d.activate(ctx, c, cr, paths, receipt, true); err != nil {
		return before, err
	}
	after, err := d.Observe(ctx, r, receipt)
	if err != nil || !after.Healthy {
		return after, errors.Join(errors.New("fonts were published but native consumption is not ready; resume after resolving font activation"), err)
	}
	return after, nil
}
func (d *FontDriver) Remove(ctx context.Context, r Resource, receipt Receipt) (Observation, error) {
	c, cr, paths, _, err := d.configuration(r)
	if err != nil {
		return Observation{}, err
	}
	before, err := d.Observe(ctx, r, receipt)
	if err != nil || !sameArtifact(before, receipt.After) || before.ApplyBlocked != "" {
		return before, errors.Join(errors.New("changed font integration is retained"), err)
	}
	if err = d.activate(ctx, c, cr, paths, receipt, false); err != nil {
		return before, err
	}
	if _, err = c.Remove(ctx, cr, receipt); err != nil {
		return before, err
	}
	if d.Target.OS == "linux" {
		if err = d.refreshLinux(ctx, paths, receipt, "removed"); err != nil {
			return before, err
		}
	}
	return d.Observe(ctx, r, receipt)
}
func (d *FontDriver) ResumeResource(ctx context.Context, r Resource, op Operation, receipt Receipt) (Observation, error) {
	c, cr, paths, _, err := d.configuration(r)
	if err != nil {
		return Observation{}, err
	}
	current, err := d.Observe(ctx, r, receipt)
	if err != nil || current.ResourceResume == nil || op.Observed.ResourceResume == nil || *current.ResourceResume != *op.Observed.ResourceResume || current.Desired != op.Observed.Desired {
		return current, errors.Join(errors.New("font recovery evidence changed after approval"), err)
	}
	j, err := c.readJournal(cr, receipt)
	if errors.Is(err, errConfigJournalAbsent) {
		if op.Action == "remove" {
			return d.Remove(ctx, r, receipt)
		}
		return d.Apply(ctx, r, op, receipt)
	}
	if err != nil || j.Action != op.Action {
		return current, errors.Join(errors.New("font recovery differs from saved file intent"), err)
	}
	if !j.Complete {
		files, err := c.Observe(ctx, cr, receipt)
		if err != nil {
			return current, err
		}
		fileOp := op
		fileOp.Observed = files
		if _, err = c.ResumeResource(ctx, cr, fileOp, receipt); err != nil {
			return current, err
		}
	}
	if j.Action != "remove" {
		if err = d.activate(ctx, c, cr, paths, receipt, true); err != nil {
			return current, err
		}
	} else if d.Target.OS == "linux" {
		if err = d.refreshLinux(ctx, paths, receipt, "removed"); err != nil {
			return current, err
		}
	}
	return d.Observe(ctx, r, receipt)
}

func (d *FontDriver) activate(ctx context.Context, c *ConfigDriver, r Resource, paths []string, receipt Receipt, install bool) error {
	baseline, err := c.readBaseline(r, receipt)
	if err != nil {
		return err
	}
	if baseline.Before.Present || baseline.Before.Inventory != emptyFontInventory() {
		return errors.New("native font mutation lacks originally absent registration proof")
	}
	probe, err := d.query(ctx, paths)
	if err != nil {
		return err
	}
	values := fontRegistrations(probe)
	hashes := make([]string, len(paths))
	for i, path := range paths {
		if values[i] != "" && values[i] != path {
			return errors.New("changed native font registration is retained")
		}
		if !install && d.Target.OS == "windows" && values[i] == "" {
			continue
		}
		hashes[i], err = fontFileHash(path)
		if !install && errors.Is(err, os.ErrNotExist) {
			hashes[i] = "absent"
			continue
		}
		if err != nil {
			return err
		}
	}
	if err := d.verifyFontFiles(c, r, paths, receipt, install); err != nil {
		return err
	}
	if d.Run == nil {
		return errors.New("font activation requires the locked native session")
	}
	switch d.Target.OS {
	case "linux":
		if !install {
			return nil
		}
		return d.refreshLinux(ctx, paths, receipt, "installed")
	case "darwin":
		// Registration is scoped to these owned file URLs. CoreText also discovers
		// ~/Library/Fonts; a duplicate registration is accepted only if the native
		// postcondition later selects the exact published bytes.
		data, _ := json.Marshal(paths)
		verb := "Register"
		if !install {
			verb = "Unregister"
		}
		script := `ObjC.import('CoreText'); var paths=` + string(data) + `; paths.forEach(function(p){ var e=Ref(); var ok=$.CTFontManager` + verb + `FontsForURL($.NSURL.fileURLWithPath($(p)),$.kCTFontManagerScopeUser,e); if(!ok&&e[0]){var code=Number($.CFErrorGetCode(e[0]));if(code!==105&&code!==201)throw Error('CoreText registration needs action: '+code);}});`
		op, _ := digest(struct {
			Operation, Action string
			Hashes            []string
		}{receipt.OperationID, verb, hashes})
		output, err := d.Run(ctx, nativeCommand{Operation: op, Program: "/usr/bin/osascript", Arguments: []string{"-l", "JavaScript", "-e", script}})
		if err != nil {
			return fmt.Errorf("native font activation: %w: %s", err, nativeErrorDetail(err, output))
		}
		return nil
	case "windows":
		data, err := json.Marshal(struct {
			Faces                 []fontFace `json:"faces"`
			Paths, Before, Hashes []string
			Install               bool `json:"install"`
		}{hackFontFaces(), paths, values, hashes, install})
		if err != nil {
			return err
		}
		op, _ := digest(struct {
			Operation string
			Payload   string
		}{receipt.OperationID, string(data)})
		output, err := d.Run(ctx, nativeCommand{Operation: op, Program: d.PowerShell, Arguments: windowsVendorArguments(windowsFontPrelude + windowsFontChange), Input: data})
		if err != nil {
			return fmt.Errorf("native font activation needs action; preserve saved registration/file evidence: %w: %s", err, nativeErrorDetail(err, output))
		}
		return nil
	}
	return errors.New("unsupported font activation platform")
}
func (d *FontDriver) refreshLinux(ctx context.Context, paths []string, receipt Receipt, phase string) error {
	if d.Run == nil || !filepath.IsAbs(d.FontCache) {
		return errors.New("fontconfig fc-cache and native session are required")
	}
	op, _ := digest(struct{ Operation, Phase string }{receipt.OperationID, "font-cache-" + phase})
	// Refresh the conventional parent after both publication and removal.
	dir := filepath.Dir(filepath.Dir(paths[0]))
	output, err := d.Run(ctx, nativeCommand{Operation: op, Program: d.FontCache, Arguments: []string{"-f", dir}})
	if err != nil {
		return fmt.Errorf("refresh current-user font cache: %w: %s", err, nativeErrorDetail(err, output))
	}
	return nil
}
func (d *FontDriver) ObserveRestore(ctx context.Context, id string, receipt Receipt) (Observation, error) {
	if id != "tool.font" {
		return Observation{}, ErrNoResourceRestore
	}
	c := &ConfigDriver{Directory: d.Directory, allowCompleteRestore: receipt.Status == "in-progress"}
	o, err := c.ObserveRestore(ctx, id, receipt)
	if err != nil {
		return o, err
	}
	j, err := c.readRecoveryJournal(id, receipt)
	if err != nil {
		return o, err
	}
	if len(j.Entries) != len(hackFontFaces()) {
		return Observation{}, errors.New("font recovery requires exactly twelve saved faces")
	}
	paths := []string{}
	for _, entry := range j.Entries {
		paths = append(paths, entry.State.Target.Destination)
	}
	probe, err := d.query(ctx, paths)
	if err != nil {
		return o, err
	}
	o.Inventory, err = digest(fontRegistrations(probe))
	return o, err
}
func (d *FontDriver) RestoreResource(ctx context.Context, id string, approved Observation, receipt Receipt) error {
	if id != "tool.font" {
		return ErrNoResourceRestore
	}
	current, err := d.ObserveRestore(ctx, id, receipt)
	if err != nil || current.Inventory != approved.Inventory || !sameArtifact(current, approved) {
		return errors.Join(errors.New("font restoration evidence changed after approval"), err)
	}
	c := &ConfigDriver{Directory: d.Directory, allowCompleteRestore: receipt.Status == "in-progress"}
	j, err := c.readRecoveryJournal(id, receipt)
	if err != nil {
		return err
	}
	if len(j.Entries) != len(hackFontFaces()) {
		return errors.New("font recovery requires exactly twelve saved faces")
	}
	paths := []string{}
	for _, entry := range j.Entries {
		paths = append(paths, entry.State.Target.Destination)
	}
	// Windows unregistration only removes values proved absent in the original
	// baseline and still referring to exact owned paths. A foreign value blocks.
	if d.Target.OS != "linux" {
		if err = d.activate(ctx, c, Resource{ID: id}, paths, receipt, false); err != nil {
			return err
		}
	}
	base, err := c.ObserveRestore(ctx, id, receipt)
	if err != nil {
		return err
	}
	if err = c.RestoreResource(ctx, id, base, receipt); err != nil {
		return err
	}
	if d.Target.OS == "linux" {
		return d.refreshLinux(ctx, paths, receipt, "restored")
	}
	if receipt.Ownership == "created" {
		return d.activate(ctx, c, Resource{ID: id}, paths, receipt, true)
	}
	return nil
}
func (d *FontDriver) FinishTransaction(ctx context.Context, plan Plan, receipts map[string]Receipt) (map[string][]string, error) {
	return (&ConfigDriver{Directory: d.Directory}).FinishTransaction(ctx, plan, receipts)
}

// Recheck native file ownership after hashing and before native activation. The
// native Windows boundary repeats the content hash check while changing keys.
func (d *FontDriver) verifyFontFiles(c *ConfigDriver, r Resource, paths []string, receipt Receipt, install bool) error {
	j, err := c.readRecoveryJournal(r.ID, receipt)
	if errors.Is(err, ErrNoResourceRestore) && !install && c.Catalog != nil {
		_, current, e := inspectConfiguration(c.Catalog, c.Manifest, c.Target, c.Folders, c.Repository, r.ID)
		if e != nil || !sameArtifact(current, receipt.After) {
			return errors.Join(errors.New("changed font files are retained"), e)
		}
		return nil
	}
	if err != nil {
		return err
	}
	if len(j.Entries) != len(paths) {
		return errors.New("font recovery target shape changed")
	}
	for i, entry := range j.Entries {
		if entry.State.Target.Destination != paths[i] {
			return errors.New("font recovery target changed")
		}
		actual, e := snapshotConfig(paths[i])
		if e != nil {
			return e
		}
		expected := entry.State.Desired
		if j.Cancelled {
			expected = entry.State.Current
		}
		if actual != expected && !(receipt.Ownership == "created" && actual == entry.State.Current) && !(!install && actual.Kind == "absent") {
			return errors.New("font file changed or is not yet published; preserve it and resume file recovery before native activation")
		}
	}
	return nil
}
