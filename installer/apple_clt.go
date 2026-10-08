package installer

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// AppleCLTDriver supplies only the hidden retained infra.apple-clt prerequisite.
// Existing healthy selected Xcode/CLT is reused without ownership. It never
// updates/removes Apple tools or reruns Homebrew over an existing prefix.
// The controller must bind and approve the observed absence before Apply, and
// may inject foreground Authenticate. Run uses the noninteractive native worker.
type AppleCLTDriver struct {
	Directory, WorkerDirectory string
	Query                      nativeCommandRunner
	Run                        func(context.Context, nativeCommand) ([]byte, error)
	Authenticate               func(context.Context) error
	clt, placeholder           string
}

func configureAppleCLT(platform NativePlatform, folders ConfigFolders, session *nativeSession) (*AppleCLTDriver, error) {
	if platform.OS != "darwin" {
		return nil, nil
	}
	if err := platform.Context.Validate(); err != nil {
		return nil, err
	}
	if session == nil || !filepath.IsAbs(session.Directory) || !filepath.IsAbs(folders.Home) {
		return nil, errors.New("Apple CLT requires explicit native session and home paths")
	}
	return &AppleCLTDriver{Directory: filepath.Join(filepath.Dir(session.Directory), "apple-clt"), WorkerDirectory: session.Directory,
		Query: macOSPrerequisiteQuery, Run: session.run, clt: "/Library/Developer/CommandLineTools",
		placeholder: "/tmp/.com.apple.dt.CommandLineTools.installondemand.in-progress"}, nil
}

type appleCLTIntent struct {
	Schema    int            `json:"schema"`
	Operation string         `json:"operation"`
	CLT       string         `json:"clt"`
	Commands  []appleCLTStep `json:"commands"`
	Completed string         `json:"completed_fingerprint,omitempty"`
	Baseline  string         `json:"receipt_baseline"`
	Selection string         `json:"selection_baseline"`
}

func (d *AppleCLTDriver) validate(r Resource) error {
	if r.ID != "infra.apple-clt" || d.Query == nil {
		return errors.New("invalid Apple CLT prerequisite boundary")
	}
	for _, path := range []string{d.Directory, d.WorkerDirectory, d.clt, d.placeholder} {
		if !filepath.IsAbs(path) || filepath.Clean(path) != path || strings.ContainsAny(path, "\x00\r\n") {
			return errors.New("Apple CLT requires canonical absolute paths")
		}
	}
	bound, err := bindConfigDestination(filepath.Join(d.Directory, "created.json"))
	if err != nil || bound != filepath.Join(d.Directory, "created.json") {
		return errors.Join(errors.New("Apple CLT state was redirected"), err)
	}
	return nil
}

func (d *AppleCLTDriver) inspect(ctx context.Context) (Observation, error) {
	desired, err := digest("apple-clt-retained-v1")
	if err != nil {
		return Observation{}, err
	}
	o := Observation{Provider: "apple-developer-infrastructure", Identity: "apple-developer-tools", Scope: "machine", Desired: desired}
	selected, err := d.selection(ctx)
	if err != nil {
		return o, err
	}
	prerequisites := MacOSPrerequisiteDriver{Query: d.Query}
	evidence := map[string]string{}
	for _, id := range []string{"tool.compiler", "tool.make"} {
		if selected == "" {
			evidence = nil
			break
		}
		check, err := prerequisites.toolchain(ctx, id)
		if err != nil {
			return o, err
		}
		if !check.Healthy {
			evidence = nil
			break
		}
		evidence[id] = check.Fingerprint
	}
	if evidence != nil {
		o.Present, o.Healthy = true, true
		o.Fingerprint, err = digest(evidence)
		return o, err
	}
	// Apple documents uninstall by removing this payload; historical receipts
	// remain for Software Update. Occupied or broken payloads are still protected.
	if _, err := os.Lstat(d.clt); err == nil {
		o.Unknown, o.Pending = true, "existing Apple Command Line Tools need native repair or selection; preserve them, complete Apple setup and check again"
		return o, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return o, err
	}
	if selected != "" && selected != d.clt {
		o.Unknown, o.Pending = true, "the selected Xcode needs license/setup or repair; preserve the chosen developer directory and complete Apple setup"
		return o, nil
	}
	baseline, err := d.nativeReceipt(ctx)
	if err != nil {
		return o, err
	}
	o.Version, _, _ = strings.Cut(baseline, "@")
	if baseline == "none" {
		o.Version = ""
	}
	o.Fingerprint, err = digest([]string{baseline, selected})
	o.HealthIssue = appleCLTDisclosure(baseline)
	return o, err
}

func (d *AppleCLTDriver) Observe(ctx context.Context, r Resource, receipt Receipt) (Observation, error) {
	if err := d.validate(r); err != nil {
		return Observation{}, err
	}
	o, err := d.inspect(ctx)
	if err != nil || receipt.OperationID == "" {
		return o, err
	}
	intent, err := d.intent(receipt.OperationID)
	if errors.Is(err, os.ErrNotExist) {
		return o, nil
	}
	if err != nil {
		return o, err
	}
	if intent.Completed != "" {
		if err := d.verifyEvidence(intent); err != nil {
			return o, err
		}
		if o.Healthy && o.Fingerprint == intent.Completed {
			if err := d.installedIdentity(ctx, intent); err != nil {
				return o, err
			}
			o.CompletedOperation = intent.Operation

		}
	} else if receipt.Status == "in-progress" {
		o.Pending = "resume the saved Apple CLT prerequisite action; installed developer tools are retained"
		token, err := digest(struct {
			Intent appleCLTIntent
			Now    Observation
		}{intent, o})
		if err != nil {
			return o, err
		}
		o.ResourceResume = &ResourceResume{Operation: intent.Operation, Token: token}
	}
	return o, nil
}

func (d *AppleCLTDriver) intentPath(operation string) string {
	return filepath.Join(d.Directory, "operations", operation+".json")
}

func (d *AppleCLTDriver) intent(operation string) (appleCLTIntent, error) {
	var intent appleCLTIntent
	if !operationID.MatchString(operation) {
		return intent, errors.New("invalid Apple CLT operation")
	}
	data, err := readDocument(d.intentPath(operation))
	if err != nil {
		return intent, err
	}
	if err := Decode(data, &intent); err != nil {
		return intent, err
	}
	if intent.Schema == 1 {
		return intent, errors.New("saved Apple CLT schema 1 has no approved receipt baseline; preserve the journal and use its original installer to inspect or abandon it")
	}
	if intent.Schema != 2 || !validAppleCLTReceipt(intent.Baseline) || intent.Selection != "" && intent.Selection != d.clt || intent.Operation != operation || intent.CLT != d.clt || len(intent.Commands) > 8 || intent.Completed != "" && !operationID.MatchString(intent.Completed) {
		return intent, errors.New("invalid saved Apple CLT intent")
	}
	for i, step := range intent.Commands {
		command, err := d.command(operation, i, step.Mode, intent.Baseline, intent.Selection, step.InstalledReceipt)
		if err != nil {
			return intent, err
		}
		want, err := digest(command)
		if err != nil {
			return intent, err
		}
		got, err := digest(step.Command)
		if err != nil || want != got {
			return intent, errors.Join(errors.New("saved Apple CLT command changed its reviewed recipe"), err)
		}
	}
	return intent, nil
}

func (d *AppleCLTDriver) Apply(ctx context.Context, r Resource, op Operation, receipt Receipt) (Observation, error) {
	return d.change(ctx, r, op, receipt, false)
}

func (d *AppleCLTDriver) ResumeResource(ctx context.Context, r Resource, op Operation, receipt Receipt) (Observation, error) {
	return d.change(ctx, r, op, receipt, true)
}

func (d *AppleCLTDriver) Remove(context.Context, Resource, Receipt) (Observation, error) {
	return Observation{}, errors.New("Apple developer infrastructure is retained and cannot be removed by dotfiles")
}

func (d *AppleCLTDriver) change(ctx context.Context, r Resource, op Operation, receipt Receipt, resume bool) (Observation, error) {
	if err := d.validate(r); err != nil {
		return Observation{}, err
	}
	if op.Action != "install" || !operationID.MatchString(receipt.OperationID) || d.Run == nil {
		return Observation{}, errors.New("Apple CLT requires an approved missing-prerequisite install; updates and removal are not supported")
	}
	intent, err := d.intent(receipt.OperationID)
	if errors.Is(err, os.ErrNotExist) {
		current, err := d.inspect(ctx)
		if err != nil || current.Present || current.Unknown || current.Pending != "" {
			return Observation{}, errors.Join(errors.New("Apple developer tools are not proven absent; preserve them and review a new plan"), err)
		}
		if !sameArtifact(current, op.Observed) {
			return Observation{}, errors.New("Apple CLT receipt or selection changed after approval; review a new plan")
		}
		selected, err := d.selection(ctx)
		if err != nil {
			return Observation{}, err
		}
		baseline, err := d.nativeReceipt(ctx)
		if err != nil {
			return Observation{}, err
		}
		fingerprint, err := digest([]string{baseline, selected})
		if err != nil || fingerprint != current.Fingerprint {
			return Observation{}, errors.Join(errors.New("Apple CLT receipt or selection changed before saving intent"), err)
		}
		intent = appleCLTIntent{Schema: 2, Operation: receipt.OperationID, CLT: d.clt, Baseline: baseline, Selection: selected}
		if err := saveDocument(d.intentPath(intent.Operation), intent); err != nil {
			return Observation{}, err
		}
	} else if err != nil {
		return Observation{}, err
	}
	if intent.Completed != "" {
		return d.Observe(ctx, r, receipt)
	}
	mode, installedReceipt := "install", ""
	for i, step := range intent.Commands {
		reply, err := d.result(step.Command)
		if errors.Is(err, os.ErrNotExist) && i == len(intent.Commands)-1 {
			if step.Mode == "install" {
				if err := d.unchangedAbsence(ctx, intent); err != nil {
					return Observation{}, err
				}
			} else if err := d.unchangedInstalled(ctx, step.InstalledReceipt); err != nil {
				return Observation{}, err
			}
			if err := d.dispatch(ctx, step.Command); err != nil {
				return Observation{}, err
			}
			reply, err = d.result(step.Command)
		}
		if err != nil {
			return Observation{}, err
		}
		if step.Mode == "install" && appleCLTMarker(reply, "INSTALLED", intent.Operation) {
			installedReceipt, err = appleCLTInstalledReceipt(reply)
			if err != nil || !appleCLTReceiptChanged(intent.Baseline, installedReceipt) {
				return Observation{}, errors.Join(errors.New("CLT installed evidence lacks a changed native receipt"), err)
			}
			mode = "finish"
		}
		if i == len(intent.Commands)-1 {
			if appleCLTSuccess(reply) && appleCLTMarker(reply, "SELECTED", intent.Operation) {
				return d.finish(ctx, r, receipt, intent)
			}
			if !resume {
				return Observation{}, fmt.Errorf("Apple CLT command needs explicit retry after its native failure: %s%s", reply.Error, nativeDiagnostic(reply.Output))
			}
		}
	}
	if mode == "install" {
		if err := d.unchangedAbsence(ctx, intent); err != nil {
			return Observation{}, err
		}
	}
	if mode == "finish" {
		if err := d.unchangedInstalled(ctx, installedReceipt); err != nil {
			return Observation{}, err
		}
	}
	if len(intent.Commands) >= 8 {
		return Observation{}, errors.New("Apple CLT exceeded its bounded recovery attempts")
	}
	command, err := d.command(intent.Operation, len(intent.Commands), mode, intent.Baseline, intent.Selection, installedReceipt)
	if err != nil {
		return Observation{}, err
	}
	intent.Commands = append(intent.Commands, appleCLTStep{Mode: mode, Command: command, InstalledReceipt: installedReceipt})
	if err := saveDocument(d.intentPath(intent.Operation), intent); err != nil {
		return Observation{}, err
	}
	if err := d.dispatch(ctx, command); err != nil {
		return Observation{}, err
	}
	return d.finish(ctx, r, receipt, intent)
}

func (d *AppleCLTDriver) verifyEvidence(intent appleCLTIntent) error {
	installed, selected := false, false
	for i, step := range intent.Commands {
		reply, err := d.result(step.Command)
		if err != nil {
			return err
		}
		installed = installed || step.Mode == "install" && appleCLTMarker(reply, "INSTALLED", intent.Operation)
		if i == len(intent.Commands)-1 {
			selected = appleCLTSuccess(reply) && appleCLTMarker(reply, "SELECTED", intent.Operation)
		}
	}
	if !installed || !selected {
		return errors.New("Apple CLT lacks successful operation-bound installation and final selection evidence")
	}
	return nil
}

func (d *AppleCLTDriver) unchangedInstalled(ctx context.Context, expected string) error {
	actual, err := d.nativeReceipt(ctx)
	if err != nil || actual != expected {
		return errors.Join(errors.New("Apple CLT receipt changed after its saved installation; preserve it"), err)
	}
	selected, err := d.selection(ctx)
	if err != nil || selected != "" && selected != d.clt {
		return errors.Join(errors.New("Apple developer selection changed after installation; preserve it"), err)
	}
	return nil
}

func (d *AppleCLTDriver) unchangedAbsence(ctx context.Context, intent appleCLTIntent) error {
	current, err := d.inspect(ctx)
	wanted, hashErr := digest([]string{intent.Baseline, intent.Selection})
	if err != nil || hashErr != nil || current.Present || current.Unknown || current.Pending != "" || current.Fingerprint != wanted {
		return errors.Join(errors.New("Apple CLT payload, receipt or selection changed from the saved absent baseline; preserve it and replan"), err, hashErr)
	}
	return nil
}

func appleCLTInstalledReceipt(reply nativeReply) (string, error) {
	found := ""
	for _, line := range strings.Split(string(reply.Output), "\n") {
		if value, ok := strings.CutPrefix(line, "DOTFILES_CLT_RECEIPT:"); ok {
			if found != "" || value == "none" || !validAppleCLTReceipt(value) {
				return "", errors.New("invalid Apple CLT installed receipt marker")
			}
			found = value
		}
	}
	if found == "" {
		return "", errors.New("missing Apple CLT installed receipt marker")
	}
	return found, nil
}

func (d *AppleCLTDriver) installedIdentity(ctx context.Context, intent appleCLTIntent) error {
	expected := ""
	for _, step := range intent.Commands {
		reply, err := d.result(step.Command)
		if err != nil {
			return err
		}
		if step.Mode == "install" && appleCLTMarker(reply, "INSTALLED", intent.Operation) {
			expected, err = appleCLTInstalledReceipt(reply)
			if err != nil {
				return err
			}
		}
		if step.Mode == "finish" && step.InstalledReceipt != expected {
			return errors.New("CLT finish phase differs from installed receipt evidence")
		}
	}
	actual, err := d.nativeReceipt(ctx)
	if err != nil || expected == "" || !appleCLTReceiptChanged(intent.Baseline, expected) || actual != expected {
		return errors.Join(errors.New("Apple CLT lacks its operation-attributed changed native receipt"), err)
	}
	selected, err := d.selection(ctx)
	if err != nil || selected != d.clt {
		return errors.Join(errors.New("Apple CLT is not the selected developer directory; preserve the current selection"), err)
	}
	return nil
}

func (d *AppleCLTDriver) finish(ctx context.Context, r Resource, receipt Receipt, intent appleCLTIntent) (Observation, error) {
	if err := d.verifyEvidence(intent); err != nil {
		return Observation{}, err
	}
	if err := d.installedIdentity(ctx, intent); err != nil {
		return Observation{}, err
	}
	o, err := d.inspect(ctx)
	if err != nil || !o.Healthy {
		return Observation{}, errors.Join(errors.New("Apple CLT has not passed compiler, GNU Make and SDK health checks"), err)
	}
	intent.Completed = o.Fingerprint
	if err := saveDocument(d.intentPath(intent.Operation), intent); err != nil {
		return Observation{}, err
	}
	return d.Observe(ctx, r, receipt)
}
