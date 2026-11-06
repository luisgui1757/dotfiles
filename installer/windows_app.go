package installer

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

//go:embed windows-app-pins.json
var windowsAppPinData []byte

// WindowsAppPin identifies a reviewed per-user NSIS installer and the
// application that it registers. The application updates itself.
type WindowsAppPin struct {
	Version     string `json:"version"`
	URL         string `json:"url"`
	SHA256      string `json:"sha256"`
	Size        int64  `json:"size"`
	ProductCode string `json:"product_code"`
	Publisher   string `json:"publisher"`
	Folder      string `json:"folder"`
	Executable  string `json:"executable"`
	Uninstaller string `json:"uninstaller"`
}

var windowsAppGUID = regexp.MustCompile(`^[a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12}$`)
var windowsAppVersion = regexp.MustCompile(`^[0-9]+(\.[0-9]+){1,3}$`)

func windowsAppName(name string) bool {
	return name != "" && len(name) <= 128 && name == strings.TrimSpace(name) && !strings.ContainsAny(name, "\x00\r\n\"/\\:*?<>|") && name != "." && name != ".."
}

func (pin WindowsAppPin) Validate() error {
	u, err := url.Parse(pin.URL)
	if err != nil || u.Scheme != "https" || u.Host != "github.com" || u.User != nil || u.Fragment != "" || u.RawQuery != "" ||
		!strings.HasPrefix(u.Path, "/pingdotgg/t3code/releases/download/v"+pin.Version+"/") || !strings.HasSuffix(u.Path, "-x64.exe") ||
		!operationID.MatchString(pin.SHA256) || !windowsAppVersion.MatchString(pin.Version) || pin.Size <= 0 || pin.Size > 512<<20 ||
		!windowsAppGUID.MatchString(pin.ProductCode) || !windowsAppName(pin.Publisher) || !windowsAppName(pin.Folder) ||
		!windowsAppName(pin.Executable) || !strings.HasSuffix(pin.Executable, ".exe") || !windowsAppName(pin.Uninstaller) || !strings.HasSuffix(pin.Uninstaller, ".exe") {
		return errors.New("Windows application requires an exact official x64 installer, version, size, SHA-256, publisher and layout")
	}
	return nil
}

func (pin WindowsAppPin) name() string {
	return strings.TrimSuffix(pin.Executable, ".exe")
}

func DefaultWindowsAppPins() (map[string]WindowsAppPin, error) {
	pins := map[string]WindowsAppPin{}
	if err := Decode(windowsAppPinData, &pins); err != nil {
		return nil, err
	}
	for id, pin := range pins {
		if id != "app.t3-code-windows" {
			return nil, fmt.Errorf("unsupported Windows application %s", id)
		}
		if err := pin.Validate(); err != nil {
			return nil, err
		}
	}
	return pins, nil
}

// WindowsAppDriver runs a pinned per-user installer through the locked native
// session. It never elevates. Query is read-only PowerShell.
type WindowsAppDriver struct {
	Directory, WorkerDirectory, PowerShell, Programs string
	Pins                                             map[string]WindowsAppPin
	Query                                            nativeCommandRunner
	Run                                              func(context.Context, nativeCommand) ([]byte, error)
	Client                                           *http.Client
}

type windowsAppSnapshot struct {
	Present     bool   `json:"present"`
	Healthy     bool   `json:"healthy"`
	Version     string `json:"version"`
	Location    string `json:"location"`
	Scope       string `json:"scope"`
	Running     bool   `json:"running"`
	HealthIssue string `json:"health_issue"`
}

type windowsAppIntent struct {
	Schema    int                `json:"schema"`
	Resource  string             `json:"resource"`
	Operation string             `json:"operation"`
	Action    string             `json:"action"`
	Pin       WindowsAppPin      `json:"pin"`
	Before    windowsAppSnapshot `json:"before"`
	After     windowsAppSnapshot `json:"after"`
	Command   nativeCommand      `json:"command"`
	Complete  bool               `json:"complete"`
}

func configureWindowsApps(platform NativePlatform, folders ConfigFolders, session *nativeSession) (*WindowsAppDriver, error) {
	if platform.OS != "windows" || platform.WindowsPowerShell == "" {
		return nil, nil
	}
	if session == nil || !filepath.IsAbs(platform.WindowsPowerShell) || !filepath.IsAbs(folders.LocalAppData) || filepath.Clean(folders.LocalAppData) != folders.LocalAppData {
		return nil, errors.New("Windows applications need observed system PowerShell, LocalAppData and native session")
	}
	pins, err := DefaultWindowsAppPins()
	if err != nil {
		return nil, err
	}
	return &WindowsAppDriver{
		Directory: filepath.Join(filepath.Dir(session.Directory), "windows-app"), WorkerDirectory: session.Directory,
		PowerShell: platform.WindowsPowerShell, Programs: filepath.Join(folders.LocalAppData, "Programs"),
		Pins: pins, Query: queryWindowsVendor, Run: session.run,
	}, nil
}

func (d *WindowsAppDriver) validate(r Resource) (WindowsAppPin, error) {
	pin, found := d.Pins[r.ID]
	if !found || d.Query == nil {
		return pin, errors.New("unsupported Windows application")
	}
	for _, path := range []string{d.PowerShell, d.Directory, d.WorkerDirectory, d.Programs} {
		if !filepath.IsAbs(path) || filepath.Clean(path) != path || strings.ContainsAny(path, "\x00\r\n") {
			return pin, errors.New("invalid Windows application provider boundary")
		}
	}
	for _, directory := range []string{d.Directory, d.WorkerDirectory} {
		path := filepath.Join(directory, "boundary.json")
		bound, err := bindConfigDestination(path)
		if err != nil || bound != path {
			return pin, errors.Join(errors.New("Windows application state directory was redirected"), err)
		}
	}
	return pin, pin.Validate()
}

func (d *WindowsAppDriver) location(pin WindowsAppPin) string {
	return filepath.Join(d.Programs, pin.Folder)
}

func (d *WindowsAppDriver) input(pin WindowsAppPin, fields map[string]string) ([]byte, error) {
	values := map[string]string{"ProductCode": pin.ProductCode, "Publisher": pin.Publisher, "Programs": d.Programs, "Folder": pin.Folder, "Executable": pin.Executable, "Uninstaller": pin.Uninstaller}
	for key, value := range fields {
		values[key] = value
	}
	return json.Marshal(values)
}

func (d *WindowsAppDriver) query(ctx context.Context, pin WindowsAppPin) (windowsAppSnapshot, error) {
	var snapshot windowsAppSnapshot
	input, err := d.input(pin, nil)
	if err != nil {
		return snapshot, err
	}
	output, err := d.Query(ctx, false, d.PowerShell, input, windowsVendorArguments(windowsAppObserveScript)...)
	if err != nil {
		return snapshot, fmt.Errorf("inspect the registered %s application: %w", pin.name(), err)
	}
	if err := Decode(output, &snapshot); err != nil {
		return snapshot, err
	}
	if !slices.Contains([]string{"", "user", "machine"}, snapshot.Scope) || snapshot.Version != "" && !windowsAppVersion.MatchString(snapshot.Version) || len(snapshot.HealthIssue) > 512 ||
		snapshot.Location != "" && (!filepath.IsAbs(snapshot.Location) || filepath.Clean(snapshot.Location) != snapshot.Location || strings.ContainsAny(snapshot.Location, "\x00\r\n")) ||
		snapshot.Healthy && (!snapshot.Present || snapshot.Scope == "" || snapshot.Location == "") {
		return snapshot, errors.New("invalid Windows application inspection")
	}
	return snapshot, nil
}

func windowsAppFingerprint(pin WindowsAppPin, snapshot windowsAppSnapshot) (string, error) {
	return digest(struct{ ProductCode, Publisher, Scope, Location string }{pin.ProductCode, pin.Publisher, snapshot.Scope, strings.ToLower(snapshot.Location)})
}

func (d *WindowsAppDriver) intentPath(operation string) string {
	return filepath.Join(d.Directory, "operations", operation+".json")
}

func (d *WindowsAppDriver) readIntent(r Resource, operation string) (windowsAppIntent, error) {
	var intent windowsAppIntent
	if !operationID.MatchString(operation) {
		return intent, errors.New("invalid Windows application operation identity")
	}
	data, err := readDocument(d.intentPath(operation))
	if err != nil {
		return intent, err
	}
	if err := Decode(data, &intent); err != nil {
		return intent, err
	}
	if intent.Schema != 1 || intent.Operation != operation || intent.Resource != r.ID || !slices.Contains([]string{"install", "repair", "remove"}, intent.Action) {
		return intent, errors.New("invalid saved Windows application intent")
	}
	if err := intent.Pin.Validate(); err != nil {
		return intent, err
	}
	want, err := d.command(intent)
	a, hashErr := digest(intent.Command)
	b, expectedErr := digest(want)
	if err != nil || hashErr != nil || expectedErr != nil || a != b {
		return intent, errors.New("saved Windows application command differs from its trusted adapter")
	}
	return intent, nil
}

func (d *WindowsAppDriver) command(intent windowsAppIntent) (nativeCommand, error) {
	input, err := d.input(intent.Pin, map[string]string{"Action": intent.Action, "File": filepath.Join(d.Directory, "payloads", intent.Pin.SHA256, "installer.exe"), "SHA256": intent.Pin.SHA256})
	if err != nil {
		return nativeCommand{}, err
	}
	command := nativeCommand{Operation: intent.Operation, Program: d.PowerShell, Arguments: windowsVendorArguments(windowsAppChangeScript), Input: input}
	return command, command.validate()
}

func (d *WindowsAppDriver) result(command nativeCommand) (nativeReply, error) {
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
		return nativeReply{}, errors.New("application command completion is unproved; preserve its native operation record")
	}
	return *record.Reply, nil
}

func (d *WindowsAppDriver) Observe(ctx context.Context, r Resource, receipt Receipt) (Observation, error) {
	pin, err := d.validate(r)
	if err != nil {
		return Observation{}, err
	}
	o := Observation{Provider: "windows-app", Identity: d.location(pin), Scope: "user", Desired: "windows-auto-updating-app-v1",
		ApplyDetail:   "run the " + pin.Publisher + " signed installer silently for the current user; it adds Start menu and desktop shortcuts; the app updates itself; personal data stays untouched",
		RemovalDetail: "close " + pin.name() + " and run its signed uninstaller; personal data and a t3 command that the app installed remain; a vendor update and an edit of unsigned app files are indistinguishable"}
	snapshot, err := d.query(ctx, pin)
	if err != nil {
		if ctx.Err() != nil {
			return o, ctx.Err()
		}
		o.Unknown, o.Pending = true, "the Windows application could not be inspected: "+err.Error()
		return o, nil
	}
	o.Present, o.Healthy, o.HealthIssue = snapshot.Present, snapshot.Healthy, snapshot.HealthIssue
	if snapshot.Location != "" {
		o.Identity = snapshot.Location
	}
	if snapshot.Scope == "machine" {
		o.Scope = "machine"
	}
	if snapshot.Present {
		o.Fingerprint, err = windowsAppFingerprint(pin, snapshot)
		if err != nil {
			return o, err
		}
	}
	if snapshot.Present && !snapshot.Healthy && receipt.Ownership != "created" {
		o.ApplyBlocked = "preserve the existing " + pin.name() + " installation: " + snapshot.HealthIssue
	}
	if receipt.OperationID == "" {
		return o, nil
	}
	intent, err := d.readIntent(r, receipt.OperationID)
	if errors.Is(err, os.ErrNotExist) {
		return o, nil
	}
	if err != nil {
		return o, err
	}
	if intent.Complete {
		reply, err := d.result(intent.Command)
		if err != nil || *reply.ExitCode != 0 {
			return o, errors.Join(errors.New("Windows application completion lacks successful native evidence"), err)
		}
		fingerprint, err := windowsAppFingerprint(intent.Pin, intent.After)
		if err != nil {
			return o, err
		}
		if intent.Action == "remove" && !o.Present || intent.Action != "remove" && o.Present && fingerprint == o.Fingerprint {
			o.CompletedOperation = intent.Operation
		}
	} else if receipt.Status == "in-progress" {
		o.Pending = "Resume the saved " + pin.name() + " operation."
		token, err := digest(struct {
			Intent windowsAppIntent
			State  windowsAppSnapshot
		}{intent, snapshot})
		if err != nil {
			return o, err
		}
		o.ResourceResume = &ResourceResume{Operation: intent.Operation, Token: token}
	}
	return o, nil
}

func (d *WindowsAppDriver) Apply(ctx context.Context, r Resource, op Operation, receipt Receipt) (Observation, error) {
	return d.change(ctx, r, op.Action, receipt, false)
}

func (d *WindowsAppDriver) Remove(ctx context.Context, r Resource, receipt Receipt) (Observation, error) {
	return d.change(ctx, r, "remove", receipt, false)
}

func (d *WindowsAppDriver) ResumeResource(ctx context.Context, r Resource, op Operation, receipt Receipt) (Observation, error) {
	return d.change(ctx, r, op.Action, receipt, true)
}

func (d *WindowsAppDriver) change(ctx context.Context, r Resource, action string, receipt Receipt, resume bool) (Observation, error) {
	pin, err := d.validate(r)
	if err != nil {
		return Observation{}, err
	}
	if d.Run == nil || !operationID.MatchString(receipt.OperationID) || !slices.Contains([]string{"install", "repair", "remove"}, action) {
		return Observation{}, errors.New("Windows application mutation requires its locked native session and saved operation")
	}
	before, err := d.query(ctx, pin)
	if err != nil {
		return Observation{}, err
	}
	intent, err := d.readIntent(r, receipt.OperationID)
	if errors.Is(err, os.ErrNotExist) && !resume {
		fingerprint, err := windowsAppFingerprint(pin, before)
		if err != nil {
			return Observation{}, err
		}
		if action == "install" && before.Present || action != "install" && (receipt.Ownership != "created" || receipt.Before.Present || !before.Present || fingerprint != receipt.After.Fingerprint) {
			return Observation{}, errors.New("pre-existing or changed " + pin.name() + " installation is not owned by this operation")
		}
		if action == "remove" && (!before.Healthy || before.Scope != "user" || !strings.EqualFold(before.Location, d.location(pin))) {
			return Observation{}, errors.New("retain " + pin.name() + ": its signed per-user installation could not be verified")
		}
		if before.Running {
			return Observation{}, errors.New("quit " + pin.name() + " before setup changes its installation")
		}
		intent = windowsAppIntent{Schema: 1, Resource: r.ID, Operation: receipt.OperationID, Action: action, Pin: pin, Before: before}
		intent.Command, err = d.command(intent)
		if err != nil {
			return Observation{}, err
		}
		if err := saveDocument(d.intentPath(intent.Operation), intent); err != nil {
			return Observation{}, err
		}
	} else if err != nil {
		return Observation{}, err
	}
	if intent.Action != action || intent.Pin != pin {
		return Observation{}, errors.New("Windows application retry differs from its saved action or reviewed installer")
	}
	if intent.Complete {
		return d.Observe(ctx, r, receipt)
	}
	if action != "remove" {
		if err := prepareWindowsInstaller(ctx, d.Client, d.Directory, pin.Version, pin.URL, pin.SHA256, pin.Size); err != nil {
			return Observation{}, err
		}
	}
	_, runErr := d.Run(ctx, intent.Command)
	reply, err := d.result(intent.Command)
	if err != nil {
		return Observation{}, errors.Join(runErr, err)
	}
	if *reply.ExitCode != 0 {
		return Observation{}, &nativeCommandError{Message: fmt.Sprintf("%s setup failed with exit code %d", pin.name(), *reply.ExitCode), ExitCode: reply.ExitCode}
	}
	after, err := d.query(ctx, pin)
	if err != nil {
		return Observation{}, err
	}
	if action == "remove" {
		if after.Present {
			return Observation{}, errors.New(pin.name() + " remains registered or present; its uninstaller did not establish removal")
		}
	} else if !after.Present || !after.Healthy || after.Scope != "user" || !strings.EqualFold(after.Location, d.location(pin)) || after.Version != pin.Version {
		return Observation{}, fmt.Errorf("the vendor installer did not leave the signed per-user %s %s: %+v", pin.name(), pin.Version, after)
	}
	intent.After, intent.Complete = after, true
	if err := saveDocument(d.intentPath(intent.Operation), intent); err != nil {
		return Observation{}, err
	}
	return d.Observe(ctx, r, receipt)
}
