package installer

import (
	"context"
	"crypto/rand"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"slices"
)

// ArchiveDriver owns a private versioned payload, not a system package or an
// executable found on PATH. Integration resources consume its stable pin paths.
type ArchiveDriver struct {
	Directory           string
	Pins                map[string]ArchivePin
	Client              *http.Client
	Python              string
	DpkgDeb             string
	WindowsRustLauncher string
	Run                 func(context.Context, nativeCommand) ([]byte, error)
}

const archiveSchema = 2

type archiveCurrent struct {
	Schema    int    `json:"schema"`
	Resource  string `json:"resource"`
	Version   string `json:"version,omitempty"`
	Operation string `json:"operation"`
}

type archiveVersion struct {
	Schema    int            `json:"schema"`
	Resource  string         `json:"resource"`
	Pin       ArchivePin     `json:"pin"`
	Operation string         `json:"operation"`
	Payload   configSnapshot `json:"payload"`
}

type archiveIntent struct {
	Schema         int         `json:"schema"`
	Resource       string      `json:"resource"`
	Operation      string      `json:"operation"`
	Action         string      `json:"action"`
	Before         Observation `json:"before"`
	Pin            ArchivePin  `json:"pin"`
	Preserved      []string    `json:"preserved,omitempty"`
	PreviousTarget string      `json:"previous_target,omitempty"`
	Generation     string      `json:"generation,omitempty"`
}

func (d *ArchiveDriver) resourceDirectory(id string) string { return filepath.Join(d.Directory, id) }
func (d *ArchiveDriver) versionDirectory(id, hash string) string {
	return filepath.Join(d.resourceDirectory(id), "versions", hash)
}
func (d *ArchiveDriver) intentPath(id, operation string) string {
	return filepath.Join(d.resourceDirectory(id), "operations", operation+".json")
}

func (d *ArchiveDriver) validate(id string) (ArchivePin, error) {
	pin, ok := d.Pins[id]
	if !ok || !resourceID.MatchString(id) || !filepath.IsAbs(d.Directory) || filepath.Clean(d.Directory) != d.Directory {
		return pin, errors.New("archive provider requires a mapped resource and absolute private directory")
	}
	path := filepath.Join(d.resourceDirectory(id), "current.json")
	bound, err := bindConfigDestination(path)
	if err != nil || bound != path {
		return pin, errors.Join(errors.New("private package parent was redirected; preserve it for inspection"), err)
	}
	return pin, pin.Validate()
}

func (d *ArchiveDriver) current(id string) (archiveCurrent, error) {
	current := archiveCurrent{Schema: archiveSchema, Resource: id}
	data, err := readDocument(filepath.Join(d.resourceDirectory(id), "current.json"))
	if errors.Is(err, os.ErrNotExist) {
		return current, nil
	}
	if err != nil {
		return current, err
	}
	if err := Decode(data, &current); err != nil {
		return current, err
	}
	if current.Schema != archiveSchema || current.Resource != id || !operationID.MatchString(current.Operation) || current.Version != "" && !operationID.MatchString(current.Version) {
		return current, errors.New("invalid private package selection")
	}
	return current, nil
}

func (d *ArchiveDriver) version(id, hash string) (archiveVersion, error) {
	var version archiveVersion
	if !operationID.MatchString(hash) {
		return version, errors.New("invalid package version identity")
	}
	path := filepath.Join(d.versionDirectory(id, hash), "version.json")
	bound, err := bindConfigDestination(path)
	if err != nil || bound != path {
		return version, errors.Join(errors.New("private version directory was redirected"), err)
	}
	data, err := readDocument(path)
	if err != nil {
		return version, err
	}
	if err := Decode(data, &version); err != nil {
		return version, err
	}
	if version.Schema != archiveSchema || version.Resource != id || version.Operation != hash || !operationID.MatchString(version.Operation) || version.Payload.Kind != "directory" {
		return version, errors.New("invalid private package provenance")
	}
	if err := version.Pin.Validate(); err != nil {
		return version, err
	}
	intent, err := d.readIntent(id, version.Operation)
	if err != nil {
		return version, err
	}
	want, err := digest(intent.Pin)
	if err != nil {
		return version, err
	}
	got, err := digest(version.Pin)
	if err != nil || want != got || intent.Action == "remove" || intent.Generation != hash {
		return version, errors.New("package provenance differs from saved publication intent")
	}
	return version, nil
}

func (d *ArchiveDriver) readIntent(id, operation string) (archiveIntent, error) {
	var intent archiveIntent
	if !operationID.MatchString(operation) {
		return intent, errors.New("invalid archive operation identity")
	}
	data, err := readDocument(d.intentPath(id, operation))
	if err != nil {
		return intent, err
	}
	if err := Decode(data, &intent); err != nil {
		return intent, err
	}
	if intent.Schema != archiveSchema || intent.Resource != id || intent.Operation != operation || !slices.Contains([]string{"install", "repair", "update", "adopt", "remove"}, intent.Action) {
		return intent, errors.New("invalid private package operation")
	}
	if intent.Action != "remove" && !operationID.MatchString(intent.Generation) {
		return intent, errors.New("invalid private package generation")
	}
	return intent, intent.Pin.Validate()
}

func (d *ArchiveDriver) observe(id string) (Observation, error) {
	pin, err := d.validate(id)
	if err != nil {
		return Observation{}, err
	}
	directory, err := bindConfigDestination(filepath.Join(d.resourceDirectory(id), "current.json"))
	if err != nil {
		return Observation{}, err
	}
	o := Observation{Provider: "archive", Identity: directory, Scope: "user"}
	o.Desired = archiveDesiredID(pin)
	current, err := d.current(id)
	if err != nil {
		return o, err
	}
	if current.Version == "" {
		link, err := archiveLinkTarget(d.currentLink(id))
		if err != nil {
			return o, err
		}
		if link != "" {
			o.Pending = "unrecorded private command entrypoint; preserve it for inspection"
			return o, nil
		}
		if current.Operation != "" {
			intent, err := d.readIntent(id, current.Operation)
			if err != nil || intent.Action != "remove" {
				return o, errors.Join(errors.New("absent package lacks removal intent"), err)
			}
			o.CompletedOperation = current.Operation
		}
		return o, nil
	}
	o.Present = true
	version, err := d.version(id, current.Version)
	if err != nil {
		return o, err
	}
	actual, err := snapshotTree(filepath.Join(d.versionDirectory(id, current.Version), "payload"), maxPackageBytes, maxPackageEntries)
	if err != nil {
		return o, err
	}
	o.Version = version.Pin.Version
	o.Fingerprint, err = digest(struct {
		Current archiveCurrent
		Actual  configSnapshot
	}{current, actual})
	if err != nil {
		return o, err
	}
	installed := archiveDesiredID(version.Pin)
	activeIntent, err := d.readIntent(id, current.Operation)
	if err != nil || activeIntent.Action == "remove" || activeIntent.Generation != current.Version {
		return o, errors.Join(errors.New("current package selection differs from its publication"), err)
	}
	link, err := archiveLinkTarget(d.currentLink(id))
	if err != nil {
		return o, err
	}
	expected := filepath.Join(d.versionDirectory(id, current.Version), "payload")
	o.Healthy = installed == o.Desired && link == expected
	if link != "" && link != expected {
		o.Pending = "command entrypoint changed or its publication is unfinished"
	}
	if actual != version.Payload {
		o.Healthy = false
		o.Adoptable = o.Pending == ""
		return o, nil
	}
	if link == expected {
		o.CompletedOperation = current.Operation
	}
	return o, nil
}

func (d *ArchiveDriver) Observe(ctx context.Context, r Resource, receipt Receipt) (Observation, error) {
	if err := ctx.Err(); err != nil {
		return Observation{}, err
	}
	o, err := d.observe(r.ID)
	if err != nil {
		return Observation{Provider: "archive", Scope: "user", Unknown: true, Pending: err.Error()}, nil
	}
	o.Adoptable = o.Adoptable && receipt.Ownership == "created" && receipt.After.Identity == o.Identity
	o.Preserved = slices.Clone(receipt.After.Preserved)
	if o.CompletedOperation != "" {
		intent, err := d.readIntent(r.ID, o.CompletedOperation)
		if err != nil {
			return Observation{}, err
		}
		o.Preserved = sortedUnique(append(o.Preserved, intent.Preserved...))
	}
	if receipt.Status == "in-progress" && receipt.OperationID != "" && o.CompletedOperation != receipt.OperationID {
		intent, err := d.readIntent(r.ID, receipt.OperationID)
		if errors.Is(err, os.ErrNotExist) {
			return o, nil
		}
		if err != nil {
			return Observation{Provider: "archive", Scope: "user", Unknown: true, Pending: err.Error(), Preserved: o.Preserved}, nil
		}
		token, err := digest(struct {
			Intent archiveIntent
			Actual Observation
		}{intent, o})
		if err != nil {
			return o, err
		}
		o.ResourceResume = &ResourceResume{Operation: receipt.OperationID, Token: token}
		o.Pending = "resume the verified private package operation"
	}
	return o, nil
}

func (d *ArchiveDriver) Apply(ctx context.Context, r Resource, op Operation, receipt Receipt) (Observation, error) {
	pin, err := d.validate(r.ID)
	if err != nil {
		return Observation{}, err
	}
	if !operationID.MatchString(receipt.OperationID) || !slices.Contains([]string{"install", "repair", "update", "adopt"}, op.Action) {
		return Observation{}, errors.New("invalid package apply intent")
	}
	before, err := d.observe(r.ID)
	if err != nil || !sameArtifact(before, op.Observed) || before.Desired != op.Observed.Desired || before.Pending != "" {
		return Observation{}, errors.Join(errors.New("private package changed after approval"), err)
	}
	intent := archiveIntent{Schema: archiveSchema, Resource: r.ID, Operation: receipt.OperationID, Action: op.Action, Before: before, Pin: pin}
	if _, err := os.Lstat(d.intentPath(r.ID, receipt.OperationID)); !errors.Is(err, os.ErrNotExist) {
		return Observation{}, errors.Join(errors.New("package operation already exists; resume it"), err)
	}
	current, err := d.current(r.ID)
	if err != nil {
		return Observation{}, err
	}
	intent.PreviousTarget, err = archiveLinkTarget(d.currentLink(r.ID))
	if err != nil {
		return Observation{}, err
	}
	intent.Generation = receipt.OperationID
	intent.Preserved = sortedUnique(append(slices.Clone(receipt.After.Preserved), op.Observed.Preserved...))
	if op.Action == "adopt" {
		if !before.Adoptable || receipt.Ownership != "created" || receipt.After.Identity != before.Identity {
			return Observation{}, errors.New("replacement requires an owned damaged package and explicit approval")
		}
		intent.Preserved = sortedUnique(append(intent.Preserved, d.versionDirectory(r.ID, current.Version)))
	} else if before.Adoptable {
		return Observation{}, errors.New("damaged package requires explicit replacement approval")
	}
	// A missing pointer can reuse the exact unchanged generation. Every real
	// package replacement gets the operation's fresh immutable directory.
	if op.Action == "repair" && current.Version != "" && receipt.Ownership == "created" && sameArtifact(before, receipt.After) {
		version, err := d.version(r.ID, current.Version)
		if err != nil {
			return Observation{}, err
		}
		if archiveDesiredID(version.Pin) == archiveDesiredID(pin) {
			intent.Generation = current.Version
		}
	}
	if err := saveDocument(d.intentPath(r.ID, receipt.OperationID), intent); err != nil {
		return Observation{}, err
	}
	return d.publish(ctx, intent)
}

func (d *ArchiveDriver) publish(ctx context.Context, intent archiveIntent) (Observation, error) {
	current, err := d.observe(intent.Resource)
	if err != nil {
		return current, err
	}
	if current.CompletedOperation == intent.Operation {
		return current, nil
	}
	if !sameArtifact(current, intent.Before) {
		return current, errors.New("package changed since interrupted operation")
	}
	directory := d.versionDirectory(intent.Resource, intent.Generation)
	if info, err := os.Lstat(directory); err == nil {
		version, readErr := d.version(intent.Resource, intent.Generation)
		if readErr == nil && (version.Operation == intent.Operation || intent.Generation == version.Operation && intent.Before.Present) {
			actual, err := snapshotTree(filepath.Join(directory, "payload"), maxPackageBytes, maxPackageEntries)
			if err != nil || actual != version.Payload {
				return current, errors.Join(errors.New("staged package changed; preserve it for recovery"), err)
			}
		} else {
			if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
				return current, errors.New("interrupted package stage is not a directory")
			}
			// Preserve incomplete bytes before retrying. They may include user edits;
			// a missing manifest does not grant recursive deletion authority.
			backup := directory + ".interrupted-" + rand.Text()
			intent.Preserved = sortedUnique(append(intent.Preserved, backup))
			if err := saveDocument(d.intentPath(intent.Resource, intent.Operation), intent); err != nil {
				return current, err
			}
			if err := moveConfigExclusive(directory, backup); err != nil {
				return current, err
			}
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return current, err
	}
	if _, err := os.Lstat(directory); errors.Is(err, os.ErrNotExist) {
		if err := prepareStateDirectory(filepath.Dir(directory)); err != nil {
			return current, err
		}
		if err := os.Mkdir(directory, 0700); err != nil {
			return current, err
		}
		if err := d.preparePayload(ctx, intent, filepath.Join(directory, "payload")); err != nil {
			return current, err
		}
		payload, err := snapshotTree(filepath.Join(directory, "payload"), maxPackageBytes, maxPackageEntries)
		if err != nil {
			return current, err
		}
		version := archiveVersion{Schema: archiveSchema, Resource: intent.Resource, Pin: intent.Pin, Operation: intent.Operation, Payload: payload}
		if err := saveDocument(filepath.Join(directory, "version.json"), version); err != nil {
			return current, err
		}
	} else if err != nil {
		return current, err
	}
	if err := d.switchLink(intent, false); err != nil {
		return current, err
	}
	if err := saveDocument(filepath.Join(d.resourceDirectory(intent.Resource), "current.json"), archiveCurrent{archiveSchema, intent.Resource, intent.Generation, intent.Operation}); err != nil {
		return current, err
	}
	after, err := d.observe(intent.Resource)
	after.Preserved = intent.Preserved
	return after, err
}

func (d *ArchiveDriver) ResumeResource(ctx context.Context, r Resource, op Operation, receipt Receipt) (Observation, error) {
	actual, err := d.Observe(ctx, r, receipt)
	if err != nil || actual.ResourceResume == nil || op.Observed.ResourceResume == nil || *actual.ResourceResume != *op.Observed.ResourceResume {
		return Observation{}, errors.Join(errors.New("private package recovery changed after approval"), err)
	}
	intent, err := d.readIntent(r.ID, receipt.OperationID)
	if err != nil {
		return Observation{}, err
	}
	if intent.Action == "remove" {
		return d.remove(ctx, intent)
	}
	return d.publish(ctx, intent)
}

func (d *ArchiveDriver) Remove(ctx context.Context, r Resource, receipt Receipt) (Observation, error) {
	before, err := d.observe(r.ID)
	if err != nil || !sameArtifact(before, receipt.After) || before.Pending != "" || receipt.Ownership != "created" || !operationID.MatchString(receipt.OperationID) {
		return Observation{}, errors.Join(errors.New("private package removal lacks unchanged ownership evidence"), err)
	}
	pin, err := d.validate(r.ID)
	if err != nil {
		return Observation{}, err
	}
	intent := archiveIntent{Schema: archiveSchema, Resource: r.ID, Operation: receipt.OperationID, Action: "remove", Before: before, Pin: pin, Preserved: slices.Clone(receipt.After.Preserved)}
	intent.PreviousTarget, err = archiveLinkTarget(d.currentLink(r.ID))
	if err != nil {
		return Observation{}, err
	}
	if err := saveDocument(d.intentPath(r.ID, receipt.OperationID), intent); err != nil {
		return Observation{}, err
	}
	return d.remove(ctx, intent)
}

func (d *ArchiveDriver) remove(ctx context.Context, intent archiveIntent) (Observation, error) {
	if err := ctx.Err(); err != nil {
		return Observation{}, err
	}
	current, err := d.current(intent.Resource)
	if err != nil {
		return Observation{}, err
	}
	if current.Version != "" {
		before, err := d.observe(intent.Resource)
		if err != nil || !sameArtifact(before, intent.Before) {
			return Observation{}, errors.Join(errors.New("package changed during removal"), err)
		}
		if err := d.switchLink(intent, true); err != nil {
			return Observation{}, err
		}
		if err := saveDocument(filepath.Join(d.resourceDirectory(intent.Resource), "current.json"), archiveCurrent{Schema: archiveSchema, Resource: intent.Resource, Operation: intent.Operation}); err != nil {
			return Observation{}, err
		}
	} else if current.Operation != intent.Operation {
		return Observation{}, errors.New("package removal selection changed")
	}
	after, err := d.observe(intent.Resource)
	after.Preserved = intent.Preserved
	return after, err
}

// Cleanup runs after consumers have switched or been removed. It only visits
// versions with complete, verified publication provenance; foreign data survives.
func (d *ArchiveDriver) FinishTransaction(ctx context.Context, plan Plan, receipts map[string]Receipt) (map[string][]string, error) {
	preserved := map[string][]string{}
	for id := range d.Pins {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		current, err := d.current(id)
		if err != nil {
			preserved[id] = append(preserved[id], d.resourceDirectory(id))
			continue
		}
		preserved[id] = slices.Clone(receipts[id].After.Preserved)
		if current.Operation != "" {
			intent, err := d.readIntent(id, current.Operation)
			if err != nil {
				preserved[id] = append(preserved[id], d.resourceDirectory(id))
				continue
			}
			preserved[id] = sortedUnique(append(preserved[id], intent.Preserved...))
		}
		link, linkErr := archiveLinkTarget(d.currentLink(id))
		expected := ""
		if current.Version != "" {
			expected = filepath.Join(d.versionDirectory(id, current.Version), "payload")
		}
		if linkErr != nil || link != expected {
			if plan.Mode == "abandon" && receipts[id].Status == "in-progress" {
				return nil, errors.Join(errors.New("command entrypoint publication must be recovered before abandonment"), linkErr)
			}
			preserved[id] = append(preserved[id], d.resourceDirectory(id))
			continue
		}
		links, err := d.cleanLinks(id)
		if err != nil {
			preserved[id] = append(preserved[id], d.resourceDirectory(id))
			continue
		}
		preserved[id] = append(preserved[id], links...)
		directory := filepath.Join(d.resourceDirectory(id), "versions")
		entries, err := readPlainDirectory(directory)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			preserved[id] = append(preserved[id], directory)
			continue
		}
		for _, entry := range entries {
			if entry.Name() == current.Version {
				continue
			}
			name := filepath.Join(directory, entry.Name())
			if cleanupPreserved(name, preserved[id]) {
				continue
			}
			kept, err := d.cleanGeneration(id, entry.Name())
			preserved[id] = sortedUnique(append(preserved[id], kept...))
			if err != nil {
				return preserved, err
			}

		}
	}
	return preserved, nil
}

// PayloadPath resolves the installed immutable generation. Integrations use
// CommandPath/BinaryDirectories instead, which are stable before installation.
func (d *ArchiveDriver) PayloadPath(id string) (string, error) {
	if _, err := d.validate(id); err != nil {
		return "", err
	}
	current, err := d.current(id)
	if err != nil {
		return "", err
	}
	if current.Version == "" {
		return "", errors.New("package has no installed generation")
	}
	return filepath.Join(d.versionDirectory(id, current.Version), "payload"), nil
}
