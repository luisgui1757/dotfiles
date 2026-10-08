package installer

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

// WindowsVendorPin identifies a reviewed Microsoft VC++ x64 runtime bundle.
// BundleID is the exact registered Burn bundle, not its broad upgrade family.
type WindowsVendorPin struct {
	Version  string `json:"version"`
	URL      string `json:"url"`
	SHA256   string `json:"sha256"`
	Size     int64  `json:"size"`
	BundleID string `json:"bundle_id"`
}

var windowsVendorVersion = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$`)
var windowsVendorGUID = regexp.MustCompile(`^\{[a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12}\}$`)

func (pin WindowsVendorPin) Validate() error {
	u, err := url.Parse(pin.URL)
	if err != nil || u.Scheme != "https" || u.Host != "download.visualstudio.microsoft.com" || u.User != nil || u.Fragment != "" || u.RawQuery != "" ||
		!strings.HasSuffix(strings.ToLower(u.Path), "/vc_redist.x64.exe") || !operationID.MatchString(pin.SHA256) ||
		!windowsVendorVersion.MatchString(pin.Version) || !strings.HasPrefix(pin.Version, "14.") || pin.Size <= 0 || pin.Size > 128<<20 || !windowsVendorGUID.MatchString(pin.BundleID) {
		return errors.New("VC++ runtime requires an exact official x64 bundle, version, size and SHA-256")
	}
	return nil
}

// Query is read-only PowerShell; Run must be the existing locked native session.
// No package-manager cascade or executable from PATH participates in installation.
type WindowsVendorDriver struct {
	Directory, WorkerDirectory, PowerShell string
	Pins                                   map[string]WindowsVendorPin
	Query                                  nativeCommandRunner
	Run                                    func(context.Context, nativeCommand) ([]byte, error)
	Client                                 *http.Client
}

type windowsVendorSnapshot struct {
	Present     bool              `json:"present"`
	Healthy     bool              `json:"healthy"`
	Version     string            `json:"version"`
	BundleID    string            `json:"bundle_id"`
	Packages    map[string]string `json:"packages"`
	Files       map[string]string `json:"files"`
	Consumers   []string          `json:"consumers"`
	Boot        string            `json:"boot"`
	HealthIssue string            `json:"health_issue"`
}

type windowsVendorIntent struct {
	Schema    int                   `json:"schema"`
	Resource  string                `json:"resource"`
	Operation string                `json:"operation"`
	Action    string                `json:"action"`
	Pin       WindowsVendorPin      `json:"pin"`
	Before    windowsVendorSnapshot `json:"before"`
	After     windowsVendorSnapshot `json:"after"`
	Commands  []nativeCommand       `json:"commands"`
	Complete  bool                  `json:"complete"`
}

func (d *WindowsVendorDriver) validate(r Resource) (WindowsVendorPin, error) {
	pin, found := d.Pins[r.ID]
	if !found || r.ID != "tool.vcredist" || d.Query == nil || !filepath.IsAbs(d.PowerShell) || filepath.Clean(d.PowerShell) != d.PowerShell ||
		!filepath.IsAbs(d.Directory) || filepath.Clean(d.Directory) != d.Directory || !filepath.IsAbs(d.WorkerDirectory) || filepath.Clean(d.WorkerDirectory) != d.WorkerDirectory {
		return pin, errors.New("invalid Windows VC++ vendor provider boundary")
	}
	for _, directory := range []string{d.Directory, d.WorkerDirectory} {
		path := filepath.Join(directory, "boundary.json")
		bound, err := bindConfigDestination(path)
		if err != nil || bound != path {
			return pin, errors.Join(errors.New("Windows vendor state directory was redirected"), err)
		}
	}
	return pin, pin.Validate()
}

func (d *WindowsVendorDriver) query(ctx context.Context) (windowsVendorSnapshot, error) {
	var snapshot windowsVendorSnapshot
	output, err := d.Query(ctx, false, d.PowerShell, nil, windowsVendorArguments(windowsVendorObserveScript)...)
	if err != nil {
		return snapshot, fmt.Errorf("inspect registered Microsoft VC++ runtime: %w", err)
	}
	if len(output) > 1<<20 {
		return snapshot, errors.New("Windows vendor inspection exceeded its bound")
	}
	if err := Decode(output, &snapshot); err != nil {
		return snapshot, err
	}
	if snapshot.Boot == "" || len(snapshot.Boot) > 128 || len(snapshot.Packages) > 2 || len(snapshot.Files) > 3 || len(snapshot.Consumers) > 1024 ||
		snapshot.Present && !windowsVendorVersion.MatchString(snapshot.Version) || snapshot.BundleID != "" && !windowsVendorGUID.MatchString(snapshot.BundleID) || snapshot.Healthy && (!snapshot.Present || len(snapshot.Files) != 3) {
		return snapshot, errors.New("invalid registered VC++ runtime inspection")
	}
	for name, id := range snapshot.Packages {
		if !slices.Contains([]string{"minimum", "additional"}, name) || !windowsVendorGUID.MatchString(id) {
			return snapshot, errors.New("invalid registered VC++ runtime package identity")
		}
	}
	for name, hash := range snapshot.Files {
		if !slices.Contains([]string{"vcruntime140.dll", "vcruntime140_1.dll", "msvcp140.dll"}, name) || !operationID.MatchString(hash) {
			return snapshot, errors.New("invalid VC++ runtime file identity")
		}
	}
	for _, consumer := range snapshot.Consumers {
		if consumer == "" || len(consumer) > 512 || strings.ContainsAny(consumer, "\x00\r\n") {
			return snapshot, errors.New("invalid registered VC++ runtime dependent")
		}
	}
	snapshot.Consumers = sortedUnique(snapshot.Consumers)
	return snapshot, nil
}

func windowsVendorFingerprint(snapshot windowsVendorSnapshot) (string, error) {
	return digest(struct {
		Version, Bundle string
		Packages, Files map[string]string
	}{snapshot.Version, snapshot.BundleID, snapshot.Packages, snapshot.Files})
}

func (d *WindowsVendorDriver) intentPath(operation string) string {
	return filepath.Join(d.Directory, "operations", operation+".json")
}

func (d *WindowsVendorDriver) readIntent(operation string) (windowsVendorIntent, error) {
	var intent windowsVendorIntent
	if !operationID.MatchString(operation) {
		return intent, errors.New("invalid Windows vendor operation identity")
	}
	data, err := readDocument(d.intentPath(operation))
	if err != nil {
		return intent, err
	}
	if err := Decode(data, &intent); err != nil {
		return intent, err
	}
	if intent.Schema != 1 || intent.Operation != operation || intent.Resource != "tool.vcredist" || !slices.Contains([]string{"install", "update", "repair", "remove"}, intent.Action) || len(intent.Commands) == 0 || len(intent.Commands) > 16 {
		return intent, errors.New("invalid saved Windows vendor intent")
	}
	if err := intent.Pin.Validate(); err != nil {
		return intent, err
	}
	for attempt, command := range intent.Commands {
		want, err := d.command(intent, attempt)
		a, hashErr := digest(command)
		b, expectedErr := digest(want)
		if err != nil || hashErr != nil || expectedErr != nil || a != b {
			return intent, errors.New("saved Windows vendor command differs from its trusted adapter")
		}
	}
	return intent, nil
}

func (d *WindowsVendorDriver) result(command nativeCommand) (nativeReply, error) {
	var record nativeCommandRecord
	data, err := readDocument(filepath.Join(d.WorkerDirectory, "commands", command.Operation+".json"))
	if err != nil {
		return nativeReply{}, err
	}
	if err := Decode(data, &record); err != nil {
		return nativeReply{}, err
	}
	hash, err := digest(command)
	if err != nil || record.Schema != 1 || record.Command != hash || record.Reply == nil || record.Reply.Ready || record.Reply.ExitCode == nil {
		return nativeReply{}, errors.New("vendor command completion is unproved; preserve its native operation record")
	}
	return *record.Reply, nil
}

func (d *WindowsVendorDriver) Observe(ctx context.Context, r Resource, receipt Receipt) (Observation, error) {
	pin, err := d.validate(r)
	if err != nil {
		return Observation{}, err
	}
	snapshot, err := d.query(ctx)
	if err != nil {
		return Observation{}, err
	}
	o := Observation{Provider: "windows-vendor", Identity: "microsoft-vc-runtime-x64", Scope: "machine", Privileged: true,
		Present: snapshot.Present, Healthy: snapshot.Healthy, Version: snapshot.Version, HealthIssue: snapshot.HealthIssue, Consumers: snapshot.Consumers}
	o.Desired, err = digest(pin)
	if err != nil {
		return o, err
	}
	if snapshot.Present {
		o.Fingerprint, err = windowsVendorFingerprint(snapshot)
		if err != nil {
			return o, err
		}
		o.UnverifiedApplications = []string{"Manual and other-user applications using the shared Microsoft VC++ runtime"}
	}
	if receipt.OperationID == "" {
		return o, nil
	}
	intent, err := d.readIntent(receipt.OperationID)
	if errors.Is(err, os.ErrNotExist) {
		return o, nil
	}
	if err != nil {
		return o, err
	}
	if intent.Complete {
		reply, err := d.result(intent.Commands[len(intent.Commands)-1])
		if err != nil || !windowsVendorSucceeded(reply) {
			return o, errors.Join(errors.New("VC++ completion lacks successful native evidence"), err)
		}
		if err := d.verifyRuntimeLog(intent, len(intent.Commands)-1); err != nil {
			return o, err
		}
		if intent.Action != "remove" && (intent.After.BundleID != intent.Pin.BundleID || intent.After.Version != intent.Pin.Version || len(intent.After.Packages) != 2) {
			return o, errors.New("saved VC++ completion differs from its pinned registered bundle")
		}
		fingerprint, err := windowsVendorFingerprint(intent.After)
		if err != nil {
			return o, err
		}
		// A completed reboot-required installation may publish its final DLLs at
		// the next boot. The registered bundle/packages must remain exactly the
		// same and the native probe must establish genuine, loadable DLLs. Core
		// recovery updates the pending receipt only after an explicit retry.
		postRestart := (receipt.Status == "needs-action" || receipt.Status == "in-progress") && *reply.ExitCode != 0 && snapshot.Boot != intent.Before.Boot && snapshot.Healthy &&
			snapshot.BundleID == intent.After.BundleID && snapshot.Version == intent.After.Version && maps.Equal(snapshot.Packages, intent.After.Packages)
		if intent.Action == "remove" && !o.Present || intent.Action != "remove" && o.Present && (fingerprint == o.Fingerprint || postRestart) {
			o.CompletedOperation = intent.Operation
		}
		if *reply.ExitCode != 0 && snapshot.Boot == intent.Before.Boot {
			o.Pending = "Restart Windows, then check the Microsoft VC++ runtime again."
		}
	} else if receipt.Status == "in-progress" {
		o.Pending = "Resume the saved Microsoft VC++ runtime operation."
		token, err := digest(struct {
			Intent windowsVendorIntent
			State  windowsVendorSnapshot
		}{intent, snapshot})
		if err != nil {
			return o, err
		}
		o.ResourceResume = &ResourceResume{Operation: intent.Operation, Token: token}
	}
	return o, nil
}

func windowsVendorSucceeded(reply nativeReply) bool {
	return reply.ExitCode != nil && slices.Contains([]int{0, 3010, 1641}, *reply.ExitCode) && (*reply.ExitCode != 0 || reply.Error == "")
}

func (d *WindowsVendorDriver) Apply(ctx context.Context, r Resource, op Operation, receipt Receipt) (Observation, error) {
	return d.change(ctx, r, op.Action, receipt, false)
}
func (d *WindowsVendorDriver) Remove(ctx context.Context, r Resource, receipt Receipt) (Observation, error) {
	return d.change(ctx, r, "remove", receipt, false)
}
func (d *WindowsVendorDriver) ResumeResource(ctx context.Context, r Resource, op Operation, receipt Receipt) (Observation, error) {
	return d.change(ctx, r, op.Action, receipt, true)
}

func (d *WindowsVendorDriver) change(ctx context.Context, r Resource, action string, receipt Receipt, resume bool) (Observation, error) {
	pin, err := d.validate(r)
	if err != nil {
		return Observation{}, err
	}
	if d.Run == nil || !operationID.MatchString(receipt.OperationID) || !slices.Contains([]string{"install", "update", "repair", "remove"}, action) {
		return Observation{}, errors.New("Windows vendor mutation requires its locked native session and saved operation")
	}
	before, err := d.query(ctx)
	if err != nil {
		return Observation{}, err
	}
	intent, err := d.readIntent(receipt.OperationID)
	if errors.Is(err, os.ErrNotExist) && !resume {
		fingerprint, fingerprintErr := windowsVendorFingerprint(before)
		if fingerprintErr != nil {
			return Observation{}, fingerprintErr
		}
		if action == "install" && before.Present || action != "install" && (receipt.Ownership != "created" || receipt.Before.Present || !before.Present || fingerprint != receipt.After.Fingerprint) {
			return Observation{}, errors.New("pre-existing or changed VC++ runtime is not owned by this operation")
		}
		if action == "remove" && (before.BundleID != pin.BundleID || before.Version != pin.Version || len(before.Packages) != 2 || len(before.Consumers) != 0) {
			return Observation{}, errors.New("retain VC++ runtime: exact owned bundle removal is blocked by changed identity or registered dependents")
		}
		intent = windowsVendorIntent{Schema: 1, Resource: r.ID, Operation: receipt.OperationID, Action: action, Pin: pin, Before: before}
		command, err := d.command(intent, 0)
		if err != nil {
			return Observation{}, err
		}
		intent.Commands = []nativeCommand{command}
		if err := saveDocument(d.intentPath(intent.Operation), intent); err != nil {
			return Observation{}, err
		}
	} else if err != nil {
		return Observation{}, err
	}
	if intent.Action != action || intent.Pin != pin {
		return Observation{}, errors.New("vendor retry differs from its saved action or reviewed installer")
	}
	if intent.Complete {
		return d.Observe(ctx, r, receipt)
	}
	if err := d.prepare(ctx, intent); err != nil {
		return Observation{}, err
	}
	command := intent.Commands[len(intent.Commands)-1]
	if prior, priorErr := d.result(command); priorErr == nil && prior.ExitCode != nil && *prior.ExitCode == 1223 && resume {
		current, err := windowsVendorFingerprint(before)
		original, originalErr := windowsVendorFingerprint(intent.Before)
		if err != nil || originalErr != nil || before.Present != intent.Before.Present || current != original || !slices.Equal(before.Consumers, intent.Before.Consumers) {
			return Observation{}, errors.New("runtime changed after UAC denial; preserve it and preview a new operation")
		}
		if len(intent.Commands) >= 16 {
			return Observation{}, errors.New("UAC retry limit reached; abandon the unstarted operation and preview again")
		}
		command, err = d.command(intent, len(intent.Commands))
		if err != nil {
			return Observation{}, err
		}
		intent.Commands = append(intent.Commands, command)
		if err := saveDocument(d.intentPath(intent.Operation), intent); err != nil {
			return Observation{}, err
		}
	}
	_, runErr := d.Run(ctx, command)
	reply, err := d.result(command)
	if err != nil {
		return Observation{}, errors.Join(runErr, err)
	}
	if !windowsVendorSucceeded(reply) {
		return Observation{}, &nativeCommandError{Message: windowsVendorExitMessage(*reply.ExitCode), ExitCode: reply.ExitCode}
	}
	after, err := d.query(ctx)
	if err != nil {
		return Observation{}, err
	}
	if action == "remove" {
		if after.Present {
			return Observation{}, errors.New("VC++ runtime remains registered or present; its installer did not establish removal")
		}
	} else if !after.Present || !after.Healthy && *reply.ExitCode == 0 || after.BundleID != pin.BundleID || after.Version != pin.Version || len(after.Packages) != 2 {
		return Observation{}, errors.New("vendor installer did not leave the exact registered, usable VC++ runtime; preserve its logs for recovery")
	}
	if err := d.verifyRuntimeLog(intent, len(intent.Commands)-1); err != nil {
		return Observation{}, err
	}
	intent.After, intent.Complete = after, true
	if err := saveDocument(d.intentPath(intent.Operation), intent); err != nil {
		return Observation{}, err
	}
	return d.Observe(ctx, r, receipt)
}

func (d *WindowsVendorDriver) prepare(ctx context.Context, intent windowsVendorIntent) (result error) {
	directory := filepath.Join(d.Directory, "payloads", intent.Pin.SHA256)
	file := filepath.Join(directory, "installer.exe")
	bound, err := bindConfigDestination(file)
	if err != nil || bound != file {
		return errors.Join(errors.New("vendor payload parent was redirected"), err)
	}
	if _, err := os.Lstat(file); errors.Is(err, os.ErrNotExist) {
		if err := os.MkdirAll(filepath.Dir(directory), 0700); err != nil {
			return err
		}
		stage, err := os.MkdirTemp(filepath.Dir(directory), ".vendor-download-")
		if err != nil {
			return err
		}
		defer func() { result = errors.Join(result, os.RemoveAll(stage)) }()
		pin := ArchivePin{Version: intent.Pin.Version, URL: intent.Pin.URL, SHA256: intent.Pin.SHA256, Format: "file", File: "installer.exe", RequiredFiles: []string{"installer.exe"}}
		if err := downloadArchive(ctx, d.Client, pin, filepath.Join(stage, "payload")); err != nil {
			return err
		}
		if err := os.Rename(filepath.Join(stage, "payload"), directory); err != nil {
			return err
		}
	}
	info, err := os.Lstat(file)
	if err != nil || !info.Mode().IsRegular() || info.Size() != intent.Pin.Size {
		return errors.Join(errors.New("vendor installer payload is missing, redirected or damaged"), err)
	}
	handle, err := os.Open(file)
	if err != nil {
		return err
	}
	hash := sha256.New()
	_, readErr := io.Copy(hash, io.LimitReader(handle, intent.Pin.Size+1))
	if err := errors.Join(readErr, handle.Close()); err != nil {
		return err
	}
	if hex.EncodeToString(hash.Sum(nil)) != intent.Pin.SHA256 {
		return errors.New("vendor installer payload differs from its reviewed SHA-256")
	}
	return nil
}

func windowsVendorExitMessage(code int) string {
	switch code {
	case 1223, 1602:
		return "Microsoft VC++ setup was cancelled or UAC permission was denied; no completion was claimed"
	case 740:
		return "Microsoft VC++ setup requires administrator approval"
	case 1618, 1001, 1003:
		return "another Windows installer is running; let it finish before retrying"
	default:
		return fmt.Sprintf("Microsoft VC++ setup failed with exit code %d; inspect the saved operation log", code)
	}
}

func (d *WindowsVendorDriver) command(intent windowsVendorIntent, attempt int) (nativeCommand, error) {
	operation, err := digest(struct {
		Operation string
		Attempt   int
	}{intent.Operation, attempt})
	if err != nil {
		return nativeCommand{}, err
	}
	input, err := json.Marshal(struct {
		Action, File, SHA256, Version, BundleID, Log string
		Before                                       windowsVendorSnapshot
	}{intent.Action, filepath.Join(d.Directory, "payloads", intent.Pin.SHA256, "installer.exe"), intent.Pin.SHA256, intent.Pin.Version, intent.Pin.BundleID,
		filepath.Join(d.Directory, "operations", operation+".log"), intent.Before})
	if err != nil {
		return nativeCommand{}, err
	}
	command := nativeCommand{Operation: operation, Program: d.PowerShell, Arguments: windowsVendorArguments(windowsVendorInstallScript), Input: input}
	return command, command.validate()
}

func (d *WindowsVendorDriver) verifyRuntimeLog(intent windowsVendorIntent, attempt int) error {
	command := intent.Commands[attempt]
	path := filepath.Join(d.Directory, "operations", command.Operation+".log")
	bound, err := bindConfigDestination(path)
	if err != nil || bound != path {
		return errors.Join(errors.New("vendor operation log was redirected"), err)
	}
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() > 4<<20 {
		return errors.Join(errors.New("vendor completion lacks its bounded operation-specific log"), err)
	}
	data, err := os.ReadFile(path)
	if err != nil || len(data) > 4<<20 {
		return errors.Join(errors.New("vendor completion lacks its bounded operation-specific log"), err)
	}
	encode, _, err := profileEncoding(data)
	if err != nil {
		return fmt.Errorf("vendor log encoding: %w", err)
	}
	action := map[string]string{"install": "Install", "update": "Install", "repair": "Repair", "remove": "Uninstall"}[intent.Action]
	for _, pkg := range []string{"vcRuntimeMinimum_x64", "vcRuntimeAdditional_x64"} {
		if !bytes.Contains(data, encode("Applying execute package: "+pkg+", action: "+action+",")) ||
			!bytes.Contains(data, encode("Applied execute package: "+pkg+", result: 0x0,")) {
			return errors.New("VC++ log does not prove both exact runtime packages completed the recorded operation")
		}
	}
	return nil
}
