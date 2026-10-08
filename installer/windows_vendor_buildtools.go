package installer

import (
	"context"
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

// The bootstrapper is pinned; Microsoft's signed installer resolves its own
// servicing packages, as the released installer recipe permits. The resulting
// registered instance and complete package inventory are recorded separately.
type WindowsBuildToolsPin struct {
	Version, URL, SHA256 string
	Size                 int64
	Components           []string
}

var windowsBuildToolsComponents = []string{"Microsoft.VisualStudio.Workload.VCTools", "Microsoft.VisualStudio.Component.VC.Tools.x86.x64", "Microsoft.VisualStudio.Component.Windows11SDK.26100"}
var windowsBuildToolsInstanceID = regexp.MustCompile(`^[a-f0-9]{8}$`)

func (pin WindowsBuildToolsPin) Validate() error {
	u, err := url.Parse(pin.URL)
	if err != nil || u.Scheme != "https" || u.Host != "download.visualstudio.microsoft.com" || u.User != nil || u.Fragment != "" || u.RawQuery != "" ||
		!strings.HasSuffix(strings.ToLower(u.Path), "/vs_buildtools.exe") || !operationID.MatchString(pin.SHA256) || !windowsVendorVersion.MatchString(pin.Version) || pin.Size <= 0 || pin.Size > 128<<20 || !slices.Equal(pin.Components, windowsBuildToolsComponents) {
		return errors.New("Build Tools requires its reviewed official bootstrapper and exact C++/Windows SDK components")
	}
	return nil
}

type WindowsBuildToolsDriver struct {
	Directory, WorkerDirectory, PowerShell, InstallDirectory string
	Pin                                                      WindowsBuildToolsPin
	Query                                                    nativeCommandRunner
	Run                                                      func(context.Context, nativeCommand) ([]byte, error)
	Client                                                   *http.Client
}

// The locator returns an array, not a set. Count keeps every occurrence while
// the seven exported fields distinguish all of vswhere's package properties.
type windowsBuildToolsPackage struct {
	ID        string `json:"id"`
	Version   string `json:"version"`
	Chip      string `json:"chip"`
	Language  string `json:"language"`
	Branch    string `json:"branch"`
	Type      string `json:"type"`
	Extension bool   `json:"extension"`
	Count     int    `json:"count"`
}

func compareBuildToolsPackages(a, b windowsBuildToolsPackage) int {
	left := [...]string{a.ID, a.Version, a.Chip, a.Language, a.Branch, a.Type}
	right := [...]string{b.ID, b.Version, b.Chip, b.Language, b.Branch, b.Type}
	for i, value := range left {
		if order := strings.Compare(value, right[i]); order != 0 {
			return order
		}
	}
	if a.Extension != b.Extension {
		if a.Extension {
			return 1
		}
		return -1
	}
	return 0
}

type windowsBuildToolsInstance struct {
	ID       string                     `json:"id"`
	Path     string                     `json:"path"`
	Version  string                     `json:"version"`
	Product  string                     `json:"product"`
	Complete bool                       `json:"complete"`
	Packages []windowsBuildToolsPackage `json:"packages"`
	Files    map[string]string          `json:"files"`
}

type windowsBuildToolsSnapshot struct {
	Instances   []windowsBuildToolsInstance `json:"instances"`
	Selected    string                      `json:"selected"`
	Healthy     bool                        `json:"healthy"`
	Occupied    bool                        `json:"occupied"`
	Boot        string                      `json:"boot"`
	HealthIssue string                      `json:"health_issue"`
	Consumers   []string                    `json:"consumers"`
}

type windowsBuildToolsIntent struct {
	Schema                              int
	Operation, Action, InstallDirectory string
	Pin                                 WindowsBuildToolsPin
	Before, After                       windowsBuildToolsSnapshot
	Commands                            []nativeCommand
	Complete                            bool
}

func (d *WindowsBuildToolsDriver) validate(r Resource) error {
	if r.ID != "tool.compiler" || d.Query == nil {
		return errors.New("invalid Windows Build Tools resource boundary")
	}
	for _, path := range []string{d.Directory, d.WorkerDirectory, d.PowerShell, d.InstallDirectory} {
		if !filepath.IsAbs(path) || filepath.Clean(path) != path || path == filepath.Dir(path) {
			return errors.New("Build Tools requires canonical absolute paths")
		}
	}
	for _, directory := range []string{d.Directory, d.WorkerDirectory, d.InstallDirectory} {
		path := filepath.Join(directory, "boundary.json")
		bound, err := bindConfigDestination(path)
		if err != nil || bound != path {
			return errors.Join(errors.New("Build Tools directory was redirected"), err)
		}
	}
	return d.Pin.Validate()
}

func (d *WindowsBuildToolsDriver) query(ctx context.Context) (windowsBuildToolsSnapshot, error) {
	var s windowsBuildToolsSnapshot
	input, err := json.Marshal(struct{ InstallDirectory string }{d.InstallDirectory})
	if err != nil {
		return s, err
	}
	data, err := d.Query(ctx, false, d.PowerShell, input, windowsVendorArguments(windowsBuildToolsObserveScript)...)
	if err != nil {
		return s, fmt.Errorf("inspect registered Microsoft Build Tools: %w", err)
	}
	if len(data) > 1<<20 {
		return s, errors.New("Build Tools inspection exceeded its bound")
	}
	if err := Decode(data, &s); err != nil {
		return s, err
	}
	if len(s.Instances) > 128 || s.Boot == "" || len(s.Boot) > 128 || len(s.Consumers) > 1024 {
		return s, errors.New("invalid Build Tools inspection")
	}
	seen := map[string]bool{}
	for index := range s.Instances {
		instance := &s.Instances[index]
		if !windowsBuildToolsInstanceID.MatchString(instance.ID) || seen[instance.ID] || !filepath.IsAbs(instance.Path) || filepath.Clean(instance.Path) != instance.Path || !windowsVendorVersion.MatchString(instance.Version) || !strings.HasPrefix(instance.Product, "Microsoft.VisualStudio.Product.") || len(instance.Packages) > 10000 || len(instance.Files) > 4 {
			return s, errors.New("invalid registered Visual Studio instance")
		}
		seen[instance.ID] = true
		slices.SortFunc(instance.Packages, compareBuildToolsPackages)
		total := 0
		for i, entry := range instance.Packages {
			tokens := entry.ID + entry.Version + entry.Chip + entry.Language + entry.Branch + entry.Type
			if entry.ID == "" || len(tokens) > 512 || strings.ContainsAny(tokens, "\x00\r\n") || entry.Count < 1 || entry.Count > 10000 || i > 0 && compareBuildToolsPackages(instance.Packages[i-1], entry) == 0 {
				return s, errors.New("invalid Visual Studio package inventory")
			}
			total += entry.Count
			if total > 10000 {
				return s, errors.New("Visual Studio package inventory exceeds its occurrence bound")
			}
		}
		for key, value := range instance.Files {
			if !slices.Contains([]string{"cl.exe", "link.exe", "VsDevCmd.bat", "Microsoft.VCToolsVersion.default.txt"}, key) || !operationID.MatchString(value) {
				return s, errors.New("invalid Visual Studio tool identity")
			}
		}
	}
	if s.Selected != "" && !seen[s.Selected] || s.Healthy && (s.Selected == "" || len(s.selected().Files) != 4 || !s.selected().Complete) {
		return s, errors.New("invalid selected Visual Studio instance")
	}
	for _, consumer := range s.Consumers {
		if consumer == "" || len(consumer) > 512 || strings.ContainsAny(consumer, "\x00\r\n") {
			return s, errors.New("invalid Build Tools consumer")
		}
	}
	s.Consumers = sortedUnique(s.Consumers)
	return s, nil
}

func (s windowsBuildToolsSnapshot) selected() windowsBuildToolsInstance {
	for _, instance := range s.Instances {
		if instance.ID == s.Selected {
			return instance
		}
	}
	return windowsBuildToolsInstance{}
}

func windowsBuildToolsFingerprint(s windowsBuildToolsSnapshot) (string, error) {
	return digest(s.selected())
}
func (d *WindowsBuildToolsDriver) intentPath(operation string) string {
	return filepath.Join(d.Directory, "operations", operation+".json")
}

func (d *WindowsBuildToolsDriver) readIntent(operation string) (windowsBuildToolsIntent, error) {
	var intent windowsBuildToolsIntent
	if !operationID.MatchString(operation) {
		return intent, errors.New("invalid Build Tools operation identity")
	}
	data, err := readDocument(d.intentPath(operation))
	if err != nil {
		return intent, err
	}
	var header struct{ Schema int }
	if err := json.Unmarshal(data, &header); err != nil {
		return intent, err
	}
	if header.Schema != 2 {
		return intent, fmt.Errorf("saved Build Tools operation uses inventory schema %d; preserve the instance and original journal for explicit recovery", header.Schema)
	}
	if err := Decode(data, &intent); err != nil {
		return intent, err
	}
	if intent.Schema != 2 || intent.Operation != operation || intent.InstallDirectory != d.InstallDirectory || !slices.Contains([]string{"install", "update", "repair", "remove"}, intent.Action) || len(intent.Commands) == 0 || len(intent.Commands) > 16 {
		return intent, errors.New("invalid saved Build Tools intent")
	}
	if err := intent.Pin.Validate(); err != nil {
		return intent, err
	}
	for index, command := range intent.Commands {
		want, err := d.command(intent, index)
		a, aerr := digest(want)
		b, berr := digest(command)
		if err != nil || aerr != nil || berr != nil || a != b {
			return intent, errors.New("saved Build Tools command differs from its trusted adapter")
		}
	}
	return intent, nil
}

func (d *WindowsBuildToolsDriver) result(command nativeCommand) (nativeReply, error) {
	return (&WindowsVendorDriver{WorkerDirectory: d.WorkerDirectory}).result(command)
}

func (d *WindowsBuildToolsDriver) Observe(ctx context.Context, r Resource, receipt Receipt) (Observation, error) {
	if err := d.validate(r); err != nil {
		return Observation{}, err
	}
	s, err := d.query(ctx)
	if err != nil {
		return Observation{}, err
	}
	instance := s.selected()
	o := Observation{Provider: "windows-buildtools", Identity: instance.ID, Version: instance.Version, Present: s.Selected != "", Healthy: s.Healthy, HealthIssue: s.HealthIssue, Scope: "machine", Privileged: true, Consumers: s.Consumers}
	o.Desired, err = digest(d.Pin)
	if err != nil {
		return o, err
	}
	if o.Present {
		o.Fingerprint, err = windowsBuildToolsFingerprint(s)
		if err != nil {
			return o, err
		}
		o.UnverifiedApplications = []string{"Manual and other-user projects using this Microsoft C++ toolchain"}
	}
	if s.Occupied && !o.Present {
		o.ApplyBlocked = "The dedicated Build Tools directory already exists without a registered instance; preserve it and inspect the saved operation."
		o.Preserved = []string{d.InstallDirectory}
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
			return o, errors.Join(errors.New("Build Tools completion lacks successful native evidence"), err)
		}
		fingerprint, err := windowsBuildToolsFingerprint(intent.After)
		if err != nil {
			return o, err
		}
		saved := intent.After.selected()
		postRestart := (receipt.Status == "needs-action" || receipt.Status == "in-progress") && *reply.ExitCode != 0 && s.Boot != intent.Before.Boot && s.Healthy &&
			instance.ID == saved.ID && instance.Path == saved.Path && instance.Product == saved.Product && instance.Version == saved.Version && slices.Equal(instance.Packages, saved.Packages)
		if intent.Action == "remove" && !o.Present || intent.Action != "remove" && o.Present && (fingerprint == o.Fingerprint || postRestart) {
			o.CompletedOperation = intent.Operation
		}
		if *reply.ExitCode != 0 && s.Boot == intent.Before.Boot {
			o.Pending = "Restart Windows, then verify the Microsoft C++ compiler again."
		}
		if intent.Action == "remove" {
			o.Preserved = append(o.Preserved, "Microsoft Visual Studio Installer and vendor-retained shared SDK/runtime components")
		}
	} else if receipt.Status == "in-progress" {
		o.Pending = "Resume the saved Microsoft Build Tools operation."
		token, err := digest(struct {
			Intent windowsBuildToolsIntent
			State  windowsBuildToolsSnapshot
		}{intent, s})
		if err != nil {
			return o, err
		}
		o.ResourceResume = &ResourceResume{Operation: intent.Operation, Token: token}
	}
	return o, nil
}

func (d *WindowsBuildToolsDriver) Apply(ctx context.Context, r Resource, op Operation, receipt Receipt) (Observation, error) {
	return d.change(ctx, r, op.Action, receipt, false)
}
func (d *WindowsBuildToolsDriver) Remove(ctx context.Context, r Resource, receipt Receipt) (Observation, error) {
	return d.change(ctx, r, "remove", receipt, false)
}
func (d *WindowsBuildToolsDriver) ResumeResource(ctx context.Context, r Resource, op Operation, receipt Receipt) (Observation, error) {
	return d.change(ctx, r, op.Action, receipt, true)
}

func (d *WindowsBuildToolsDriver) change(ctx context.Context, r Resource, action string, receipt Receipt, resume bool) (Observation, error) {
	if err := d.validate(r); err != nil {
		return Observation{}, err
	}
	if d.Run == nil || !operationID.MatchString(receipt.OperationID) || !slices.Contains([]string{"install", "update", "repair", "remove"}, action) {
		return Observation{}, errors.New("Build Tools mutation requires its locked native session and saved operation")
	}
	before, err := d.query(ctx)
	if err != nil {
		return Observation{}, err
	}
	intent, err := d.readIntent(receipt.OperationID)
	if errors.Is(err, os.ErrNotExist) && !resume {
		fingerprint, err := windowsBuildToolsFingerprint(before)
		if err != nil {
			return Observation{}, err
		}
		if action == "install" && (before.Selected != "" || before.Occupied) || action != "install" && (receipt.Ownership != "created" || receipt.Before.Present || before.Selected == "" || fingerprint != receipt.After.Fingerprint || before.selected().Path != d.InstallDirectory || before.selected().Product != "Microsoft.VisualStudio.Product.BuildTools") {
			return Observation{}, errors.New("pre-existing or changed Visual Studio instance is not owned by this operation")
		}
		if action == "remove" && len(before.Consumers) != 0 {
			return Observation{}, errors.New("registered consumers block Build Tools removal")
		}
		intent = windowsBuildToolsIntent{Schema: 2, Operation: receipt.OperationID, Action: action, InstallDirectory: d.InstallDirectory, Pin: d.Pin, Before: before}
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
	want, err := digest(d.Pin)
	saved, savedErr := digest(intent.Pin)
	if err != nil || savedErr != nil || intent.Action != action || want != saved {
		return Observation{}, errors.New("Build Tools retry differs from its saved action or reviewed bootstrapper")
	}
	if intent.Complete {
		return d.Observe(ctx, r, receipt)
	}
	pin := WindowsVendorPin{Version: intent.Pin.Version, URL: intent.Pin.URL, SHA256: intent.Pin.SHA256, Size: intent.Pin.Size}
	if err := (&WindowsVendorDriver{Directory: d.Directory, Client: d.Client}).prepare(ctx, windowsVendorIntent{Pin: pin}); err != nil {
		return Observation{}, err
	}
	command := intent.Commands[len(intent.Commands)-1]
	if reply, resultErr := d.result(command); resultErr == nil && reply.ExitCode != nil && *reply.ExitCode == 1223 && resume {
		a, aerr := windowsBuildToolsFingerprint(before)
		b, berr := windowsBuildToolsFingerprint(intent.Before)
		if aerr != nil || berr != nil || a != b || before.Occupied != intent.Before.Occupied || !slices.Equal(before.Consumers, intent.Before.Consumers) || len(intent.Commands) >= 16 {
			return Observation{}, errors.New("Build Tools changed after denied elevation or retry limit reached; preserve it and preview again")
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
		return Observation{}, &nativeCommandError{Message: strings.ReplaceAll(windowsVendorExitMessage(*reply.ExitCode), "VC++", "Build Tools"), ExitCode: reply.ExitCode}
	}
	after, err := d.query(ctx)
	if err != nil {
		return Observation{}, err
	}
	if action == "remove" {
		for _, instance := range after.Instances {
			if instance.ID == intent.Before.Selected || instance.Path == d.InstallDirectory {
				return Observation{}, errors.New("the owned Build Tools instance remains registered")
			}
		}
		if after.Selected != "" {
			return Observation{}, errors.New("another compiler appeared; preserve it and reconcile the removal")
		}
	} else {
		instance := after.selected()
		if instance.ID == "" || instance.Path != d.InstallDirectory || instance.Product != "Microsoft.VisualStudio.Product.BuildTools" || !instance.Complete || !after.Healthy && *reply.ExitCode == 0 {
			return Observation{}, errors.New("vendor setup did not establish a complete registered and usable dedicated C++ instance")
		}
		for _, component := range d.Pin.Components {
			found := false
			for _, entry := range instance.Packages {
				if entry.ID == component {
					found = true
				}
			}
			if !found {
				return Observation{}, fmt.Errorf("Build Tools is missing the requested component %s", component)
			}
		}
		if action == "install" {
			for _, old := range intent.Before.Instances {
				if old.ID == instance.ID {
					return Observation{}, errors.New("vendor setup reused a pre-existing Visual Studio instance")
				}
			}
		} else if instance.ID != intent.Before.Selected {
			return Observation{}, errors.New("vendor setup replaced the owned instance identity")
		}
	}
	intent.After, intent.Complete = after, true
	if err := saveDocument(d.intentPath(intent.Operation), intent); err != nil {
		return Observation{}, err
	}
	return d.Observe(ctx, r, receipt)
}

func (d *WindowsBuildToolsDriver) command(intent windowsBuildToolsIntent, attempt int) (nativeCommand, error) {
	operation, err := digest(struct {
		Operation string
		Attempt   int
	}{intent.Operation, attempt})
	if err != nil {
		return nativeCommand{}, err
	}
	input, err := json.Marshal(struct {
		Action, File, SHA256, InstallDirectory string
		Components                             []string
		Before                                 windowsBuildToolsSnapshot
	}{intent.Action, filepath.Join(d.Directory, "payloads", intent.Pin.SHA256, "installer.exe"), intent.Pin.SHA256, d.InstallDirectory, intent.Pin.Components, intent.Before})
	if err != nil {
		return nativeCommand{}, err
	}
	command := nativeCommand{Operation: operation, Program: d.PowerShell, Arguments: windowsVendorArguments(windowsBuildToolsInstallScript), Input: input}
	return command, command.validate()
}

// CompilerEnvironment resolves SDK variables after the selected toolchain is
// installed. Only vendor-directory PATH entries and explicit SDK variables are
// returned; callers prepend PATH to their existing approved command environment.
func (d *WindowsBuildToolsDriver) CompilerEnvironment(ctx context.Context) (map[string]string, error) {
	if err := d.validate(Resource{ID: "tool.compiler"}); err != nil {
		return nil, err
	}
	snapshot, err := d.query(ctx)
	if err != nil {
		return nil, err
	}
	if !snapshot.Healthy {
		return nil, errors.New("a usable Microsoft C++ toolchain is required before resolving its SDK environment")
	}
	input, err := json.Marshal(struct{ InstallDirectory, Instance, Probe string }{d.InstallDirectory, snapshot.Selected, windowsVendorArguments(windowsBuildToolsEnvironmentProbe)[4]})
	if err != nil {
		return nil, err
	}
	data, err := d.Query(ctx, false, d.PowerShell, input, windowsVendorArguments(windowsBuildToolsEnvironmentScript)...)
	if err != nil {
		return nil, fmt.Errorf("resolve Microsoft C++ environment: %w", err)
	}
	var environment map[string]string
	if len(data) > 256<<10 {
		return nil, errors.New("compiler environment exceeded its bound")
	}
	if err := Decode(data, &environment); err != nil {
		return nil, err
	}
	allowed := []string{"PATH", "INCLUDE", "LIB", "LIBPATH", "VCINSTALLDIR", "VCToolsInstallDir", "WindowsSdkDir", "WindowsSDKVersion"}
	if len(environment) != len(allowed) {
		return nil, errors.New("compiler environment is incomplete")
	}
	for name, value := range environment {
		if !slices.Contains(allowed, name) || value == "" || len(value) > 32767 || strings.ContainsAny(value, "\x00\r\n") {
			return nil, errors.New("compiler environment contains an unsupported or malformed value")
		}
	}
	return environment, nil
}
