package installer

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// These are the released install-deps.sh bootstrap pins. The matching regression
// deliberately fails when that source changes without updating this recipe.
const homebrewBootstrapCommit = "35da6871c4be7d7fdab2fd505fb7fa667926a2a5"
const homebrewBootstrapSHA256 = "5f333bbe53bc490e51e7ccb1df8779b3dd6ee73a1a7379efda216edb08ccb148"
const homebrewBootstrapURL = "https://raw.githubusercontent.com/Homebrew/install/" + homebrewBootstrapCommit + "/install.sh"

type homebrewBootstrapRecipe struct {
	URL, SHA256 string
}

// HomebrewBootstrapDriver owns only an infrastructure prefix it reserved from
// absence. Infrastructure is retained on removal and tool updates. Its packages
// remain under the formula provider's ownership contract. Existing managers are reused.
// Authenticate belongs to the foreground controller after explicit approval;
// Run belongs to its noninteractive native worker. Constructors do neither.
// This is bootstrap-only: selected tool updates belong to the formula provider,
// never a global `brew update` that can migrate unrelated installed packages.
type HomebrewBootstrapDriver struct {
	Directory, WorkerDirectory string
	Home                       string
	Existing                   *HomebrewLocation
	Query                      nativeCommandRunner
	Run                        func(context.Context, nativeCommand) ([]byte, error)
	Authenticate               func(context.Context) error
	HTTPClient                 *http.Client
	prefix                     string
	recipe                     homebrewBootstrapRecipe
}

func configureHomebrewBootstrap(platform NativePlatform, folders ConfigFolders, session *nativeSession) (*HomebrewBootstrapDriver, error) {
	if platform.OS != "darwin" {
		return nil, nil
	}
	if err := platform.Context.Validate(); err != nil {
		return nil, err
	}
	if session == nil || !filepath.IsAbs(session.Directory) || !filepath.IsAbs(folders.Home) {
		return nil, errors.New("Homebrew bootstrap requires explicit home and native session paths")
	}
	d := &HomebrewBootstrapDriver{Directory: filepath.Join(filepath.Dir(session.Directory), "homebrew-bootstrap"), WorkerDirectory: session.Directory,
		Home: folders.Home, Existing: platform.Homebrew, Run: session.run, prefix: "/opt/homebrew", recipe: homebrewBootstrapRecipe{homebrewBootstrapURL, homebrewBootstrapSHA256}}
	if platform.Homebrew != nil {
		if err := platform.Homebrew.validate(); err != nil {
			return nil, err
		}
		d.prefix = platform.Homebrew.Prefix
	}
	d.Query = d.query
	return d, nil
}

func (d *HomebrewBootstrapDriver) query(ctx context.Context, privileged bool, program string, input []byte, args ...string) ([]byte, error) {
	location := d.location()
	if program == "brew" {
		args = append(append(d.environment(), location.Program), args...)
		program = "/usr/bin/env"
	}
	return location.query(ctx, privileged, program, input, args...)
}

func (d *HomebrewBootstrapDriver) location() HomebrewLocation {
	if d.Existing != nil {
		return *d.Existing
	}
	return HomebrewLocation{Program: filepath.Join(d.prefix, "bin", "brew"), Prefix: d.prefix, Cellar: filepath.Join(d.prefix, "Cellar")}
}

type homebrewBootstrapRecord struct {
	Schema    int              `json:"schema"`
	Location  HomebrewLocation `json:"location"`
	CreatedBy string           `json:"created_by"`
	Directory string           `json:"directory_identity"`
}

type homebrewBootstrapIntent struct {
	Schema    int                     `json:"schema"`
	Operation string                  `json:"operation"`
	Action    string                  `json:"action"`
	Location  HomebrewLocation        `json:"location"`
	Recipe    homebrewBootstrapRecipe `json:"recipe"`
	Directory string                  `json:"directory_identity,omitempty"`
	Commands  []homebrewBootstrapStep `json:"commands"`
	Complete  bool                    `json:"complete,omitempty"`
}

type homebrewBootstrapStep struct {
	Kind    string        `json:"kind"`
	Command nativeCommand `json:"command"`
}

func (d *HomebrewBootstrapDriver) validate(r Resource) error {
	if r.ID != "infra.homebrew" || d.Query == nil || !operationID.MatchString(d.recipe.SHA256) || d.recipe.URL == "" {
		return errors.New("invalid Homebrew bootstrap resource boundary")
	}
	if err := d.location().validate(); err != nil {
		return err
	}
	for _, directory := range []string{d.Directory, d.WorkerDirectory, d.Home} {
		if !filepath.IsAbs(directory) || filepath.Clean(directory) != directory || strings.ContainsAny(directory, "\x00\r\n") {
			return errors.New("Homebrew bootstrap requires canonical absolute paths")
		}
	}
	bound, err := bindConfigDestination(filepath.Join(d.Directory, "created.json"))
	if err != nil || bound != filepath.Join(d.Directory, "created.json") {
		return errors.Join(errors.New("Homebrew bootstrap state directory was redirected"), err)
	}
	return nil
}

func (d *HomebrewBootstrapDriver) record() (homebrewBootstrapRecord, error) {
	var record homebrewBootstrapRecord
	data, err := readDocument(filepath.Join(d.Directory, "created.json"))
	if err != nil {
		return record, err
	}
	if err := Decode(data, &record); err != nil {
		return record, err
	}
	if record.Schema != 1 || record.Location != d.location() || record.Directory == "" || !operationID.MatchString(record.CreatedBy) {
		return record, errors.New("invalid Homebrew infrastructure ownership record")
	}
	return record, nil
}

func (d *HomebrewBootstrapDriver) Observe(ctx context.Context, r Resource, receipt Receipt) (Observation, error) {
	if err := d.validate(r); err != nil {
		return Observation{}, err
	}
	location := d.location()
	o, identity, err := d.inspect(ctx)
	if err != nil {
		return o, err
	}
	record, recordErr := d.record()
	if recordErr != nil && !errors.Is(recordErr, os.ErrNotExist) {
		return o, recordErr
	}
	if recordErr == nil && identity != record.Directory {
		o.ApplyBlocked = "the retained Homebrew prefix changed identity; inspect it before reusing it"
	}
	if receipt.OperationID != "" {
		intent, err := d.intent(receipt.OperationID)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return o, err
		}
		if err == nil {
			if intent.Complete {
				if err := d.verifyCompletion(intent); err != nil {
					return o, err
				}
				if o.Healthy && recordErr == nil && identity == record.Directory && intent.Directory == identity && record.CreatedBy == intent.Operation {
					o.CompletedOperation = intent.Operation
				}
			} else if receipt.Status == "in-progress" {
				o.Pending = "resume the saved Homebrew infrastructure operation; Homebrew and any installed Command Line Tools are retained"
				token, err := digest(struct {
					Intent      homebrewBootstrapIntent
					Observation Observation
					Directory   string
				}{intent, o, identity})
				if err != nil {
					return o, err
				}
				o.ResourceResume = &ResourceResume{Operation: intent.Operation, Token: token}
			}
		}
	}
	if identity != "" && !o.Present && o.Pending == "" {
		o.Unknown = true
		o.Pending = "the Homebrew prefix already exists without a working manager; preserve it and repair it explicitly"
	}
	if o.Present && !o.Healthy && o.Pending == "" {
		o.ApplyBlocked = "the existing Homebrew installation needs native repair before it can be reused"
	}
	// Keep the same artifact identity as the existing-infrastructure adapter,
	// so discovery after the bootstrap boundary cannot manufacture new ownership.
	if o.Present {
		o.Fingerprint, err = digest(location)
	}
	return o, err
}

func (d *HomebrewBootstrapDriver) inspect(ctx context.Context) (Observation, string, error) {
	location := d.location()
	// This infrastructure is installed once and then retained. Keeping this
	// policy identity independent of installer releases lets ordinary tool
	// updates reuse the manager without invoking Homebrew's global migrations.
	desired, err := digest("homebrew-bootstrap-retained-v1")
	if err != nil {
		return Observation{}, "", err
	}
	o := Observation{Provider: "homebrew-infrastructure", Identity: location.Prefix, Scope: "machine", Desired: desired}
	identity, err := homebrewBootstrapDirectory(location.Prefix)
	if errors.Is(err, os.ErrNotExist) {
		return o, "", nil
	}
	if err != nil {
		return o, "", err
	}
	info, err := os.Stat(location.Program)
	if errors.Is(err, os.ErrNotExist) {
		return o, identity, nil
	}
	if err != nil {
		return o, identity, err
	}
	o.Present = true
	o.Fingerprint, err = digest(location)
	if err != nil {
		return o, identity, err
	}
	output, checkErr := d.Query(ctx, false, "brew", nil, "--version")
	o.Healthy = info.Mode().IsRegular() && info.Mode().Perm()&0111 != 0 && checkErr == nil && strings.HasPrefix(string(output), "Homebrew ")
	if !o.Healthy {
		o.HealthIssue = "the retained Homebrew executable needs native repair"
		if checkErr != nil {
			o.HealthIssue = checkErr.Error()
		}
		return o, identity, ctx.Err()
	}
	for option, expected := range map[string]string{"--prefix": location.Prefix, "--cellar": location.Cellar} {
		data, err := d.Query(ctx, false, "brew", nil, option)
		if err != nil || strings.TrimSuffix(string(data), "\n") != expected {
			return o, identity, errors.Join(errors.New("Homebrew rediscovery differs from its declared infrastructure"), err)
		}
	}
	return o, identity, nil
}

func homebrewBootstrapDirectory(prefix string) (identity string, result error) {
	file, err := openConfigParent(filepath.Join(prefix, ".dotfiles-bootstrap-check"))
	if err != nil {
		return "", err
	}
	defer func() { result = errors.Join(result, file.Close()) }()
	return configDirectoryIdentity(file)
}

func (d *HomebrewBootstrapDriver) Apply(ctx context.Context, r Resource, op Operation, receipt Receipt) (Observation, error) {
	return d.change(ctx, r, op.Action, receipt, false)
}

func (d *HomebrewBootstrapDriver) ResumeResource(ctx context.Context, r Resource, op Operation, receipt Receipt) (Observation, error) {
	return d.change(ctx, r, op.Action, receipt, true)
}

func (d *HomebrewBootstrapDriver) Remove(context.Context, Resource, Receipt) (Observation, error) {
	return Observation{}, errors.New("Homebrew infrastructure and Command Line Tools are retained; package removal cannot uninstall them")
}

func (d *HomebrewBootstrapDriver) source(ctx context.Context) error {
	data, err := d.Query(ctx, false, "/usr/bin/env", nil, "-i", "PATH=/usr/bin:/bin", "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null", "/usr/bin/git", "-C", d.location().Prefix, "config", "--local", "--get", "remote.origin.url")
	source := strings.TrimSpace(string(data))
	if err != nil || source != "https://github.com/Homebrew/brew" && source != "https://github.com/Homebrew/brew.git" {
		return errors.Join(errors.New("retained Homebrew source is not its original official repository; preserve it"), err)
	}
	return nil
}

func (d *HomebrewBootstrapDriver) change(ctx context.Context, r Resource, action string, receipt Receipt, resume bool) (Observation, error) {
	if err := d.validate(r); err != nil {
		return Observation{}, err
	}
	if d.Run == nil || !operationID.MatchString(receipt.OperationID) || action != "install" {
		return Observation{}, errors.New("Homebrew bootstrap requires an approved install operation; retained manager updates and repairs are not implicit")
	}
	_, identity, err := d.inspect(ctx)
	if err != nil {
		return Observation{}, err
	}
	intent, err := d.intent(receipt.OperationID)
	if errors.Is(err, os.ErrNotExist) {
		if identity != "" {
			return Observation{}, errors.New("Homebrew prefix appeared before bootstrap; preserve it and review a new plan")
		}
		intent = homebrewBootstrapIntent{Schema: 1, Operation: receipt.OperationID, Action: action, Location: d.location(), Recipe: d.recipe}
		if err := saveDocument(d.intentPath(intent.Operation), intent); err != nil {
			return Observation{}, err
		}
	} else if err != nil {
		return Observation{}, err
	} else if intent.Action != action {
		return Observation{}, errors.New("Homebrew infrastructure recovery changed its saved action")
	}
	if intent.Complete {
		return d.Observe(ctx, r, receipt)
	}
	if len(intent.Commands) > 0 {
		last := intent.Commands[len(intent.Commands)-1]
		reply, err := d.result(last.Command)
		if errors.Is(err, os.ErrNotExist) {
			if err := d.dispatch(ctx, intent, last); err != nil {
				return Observation{}, err
			}
			reply, err = d.result(last.Command)
		}
		if err != nil {
			return Observation{}, err
		}
		if reply.Error == "" && reply.ExitCode != nil && *reply.ExitCode == 0 && last.Kind != "reserve" && (!resume || last.Kind == "verify") {
			return d.finish(ctx, r, receipt, intent)
		}
		if (reply.Error != "" || reply.ExitCode == nil || *reply.ExitCode != 0) && !resume {
			return Observation{}, errors.New("failed Homebrew infrastructure command needs explicit recovery approval")
		}
	}
	if intent.Action == "install" && intent.Directory == "" {
		if len(intent.Commands) == 0 || intent.Commands[len(intent.Commands)-1].Kind != "reserve" {
			if err := d.preparePayload(ctx, intent.Operation, intent.Recipe); err != nil {
				return Observation{}, err
			}
			if err := d.runStep(ctx, &intent, "reserve"); err != nil {
				return Observation{}, err
			}
		} else {
			last, err := d.result(intent.Commands[len(intent.Commands)-1].Command)
			if err != nil {
				return Observation{}, err
			}
			if last.Error != "" || last.ExitCode == nil || *last.ExitCode != 0 {
				if err := d.runStep(ctx, &intent, "reserve"); err != nil {
					return Observation{}, err
				}
			}
		}
		intent.Directory, err = homebrewBootstrapDirectory(intent.Location.Prefix)
		if err != nil {
			return Observation{}, err
		}
		if err := saveDocument(d.intentPath(intent.Operation), intent); err != nil {
			return Observation{}, err
		}
	}
	kind := "install"
	current, _, err := d.inspect(ctx)
	if err != nil {
		return Observation{}, err
	}
	if current.Healthy && resume {
		// The original failed installer can leave a usable manager. Confirm it
		// through a saved read-only worker command; never rerun its recursive
		// bootstrap changes or a global package-manager update over new users.
		kind = "verify"
	}
	if err := d.runStep(ctx, &intent, kind); err != nil {
		return Observation{}, err
	}
	return d.finish(ctx, r, receipt, intent)
}

func (d *HomebrewBootstrapDriver) finish(ctx context.Context, r Resource, receipt Receipt, intent homebrewBootstrapIntent) (Observation, error) {
	if err := d.verifyCompletion(intent); err != nil {
		return Observation{}, err
	}
	observed, identity, err := d.inspect(ctx)
	if err != nil || !observed.Healthy || identity != intent.Directory {
		return Observation{}, errors.Join(errors.New("Homebrew bootstrap has not passed native rediscovery and health checks"), err)
	}
	if err := d.source(ctx); err != nil {
		return Observation{}, err
	}
	installed, err := brewInventory(ctx, d.Query)
	if err != nil {
		return Observation{}, err
	}
	if intent.Commands[len(intent.Commands)-1].Kind == "install" && len(installed) != 0 {
		return Observation{}, errors.New("unexpected packages appeared during bootstrap; review them before resuming infrastructure verification")
	}
	if intent.Action == "install" {
		record := homebrewBootstrapRecord{Schema: 1, Location: intent.Location, CreatedBy: intent.Operation, Directory: identity}
		if err := saveDocument(filepath.Join(d.Directory, "created.json"), record); err != nil {
			return Observation{}, err
		}
	}
	intent.Complete = true
	if err := saveDocument(d.intentPath(intent.Operation), intent); err != nil {
		return Observation{}, err
	}
	return d.Observe(ctx, r, receipt)
}

func (d *HomebrewBootstrapDriver) verifyCompletion(intent homebrewBootstrapIntent) error {
	if len(intent.Commands) == 0 || intent.Directory == "" {
		return errors.New("Homebrew bootstrap lacks completed native command evidence")
	}
	last := intent.Commands[len(intent.Commands)-1]
	reply, err := d.result(last.Command)
	if err != nil || last.Kind == "reserve" || reply.Error != "" || reply.ExitCode == nil || *reply.ExitCode != 0 {
		return errors.Join(errors.New("Homebrew bootstrap has no successful final native command"), err)
	}
	if intent.Action == "install" {
		reserved, attempted := false, false
		for _, step := range intent.Commands {
			if step.Kind != "reserve" && step.Kind != "install" {
				continue
			}
			reply, err := d.result(step.Command)
			if err != nil {
				return err
			}
			reserved = reserved || step.Kind == "reserve" && reply.Error == "" && reply.ExitCode != nil && *reply.ExitCode == 0
			attempted = attempted || step.Kind == "install" && reply.ExitCode != nil
		}
		if !reserved || !attempted {
			return errors.New("Homebrew prefix lacks its exclusive reservation and attempted installer evidence")
		}
	}
	return nil
}

func homebrewBootstrapFailure(reply nativeReply) error {
	return fmt.Errorf("Homebrew infrastructure command failed; authorize administrator access in the foreground or resolve the reported native prerequisite, then retry: %s%s", reply.Error, nativeDiagnostic(reply.Output))
}
