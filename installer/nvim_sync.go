package installer

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// All paths and environment are supplied by the process boundary. Construction
// is pure; the native session owns the lifetime of each mutating headless phase.
type NvimSyncOptions struct {
	Repository, Directory, RuntimeDirectory, Executable string
	Environment                                         []string
	Run                                                 func(context.Context, nativeCommand) ([]byte, error)
}

type NvimSyncDriver struct {
	NvimSyncOptions
	query func(context.Context, nativeCommand) ([]byte, error)
}

type nvimSyncIntent struct {
	Schema                                     int `json:"schema"`
	Operation, Action, Desired, PreviousTarget string
	Reserved                                   bool
	Step                                       int
	Attempt                                    int
	Failed                                     bool
	Complete                                   bool
	Manifest                                   []string `json:"manifest,omitempty"`
	Preserved                                  []string `json:"preserved,omitempty"`
}

const maxNvimEntries = maxPackageEntries

func NewNvimSyncDriver(options NvimSyncOptions) (*NvimSyncDriver, error) {
	d := &NvimSyncDriver{NvimSyncOptions: options, query: queryNvimSync}
	for _, path := range []string{d.Repository, d.Directory, d.RuntimeDirectory, d.Executable} {
		if !filepath.IsAbs(path) || filepath.Clean(path) != path || strings.ContainsAny(path, "\x00\r\n") {
			return nil, errors.New("Neovim sync requires explicit absolute canonical paths")
		}
	}
	for _, env := range d.Environment {
		key, value, ok := strings.Cut(env, "=")
		if !ok || !slices.Contains([]string{"HOME", "USERPROFILE", "LOCALAPPDATA", "APPDATA", "PATH", "XDG_CONFIG_HOME", "XDG_DATA_HOME", "XDG_STATE_HOME", "XDG_CACHE_HOME", "TEMP", "TMP", "INCLUDE", "LIB", "LIBPATH", "VCINSTALLDIR", "VCToolsInstallDir", "WindowsSdkDir", "WindowsSDKVersion", "RUSTUP_HOME"}, key) || strings.ContainsRune(env, 0) {
			return nil, errors.New("unexpected Neovim sync environment control")
		}
		if key == "RUSTUP_HOME" && (!filepath.IsAbs(value) || filepath.Clean(value) != value || strings.ContainsAny(value, "\r\n")) {
			return nil, errors.New("Neovim sync requires an explicit absolute managed Rust home")
		}
	}
	return d, nil
}

func (d *NvimSyncDriver) links() *ArchiveDriver { return &ArchiveDriver{Directory: d.RuntimeDirectory} }
func (d *NvimSyncDriver) current() string       { return d.links().currentLink("nvim.sync") }
func (d *NvimSyncDriver) payload(operation string) string {
	return filepath.Join(d.links().versionDirectory("nvim.sync", operation), "payload")
}
func (d *NvimSyncDriver) intentPath(operation string) string {
	return filepath.Join(d.Directory, "operations", operation+".json")
}
func (d *NvimSyncDriver) source() (string, error) {
	snapshot, err := snapshotConfig(filepath.Join(d.Repository, "nvim"))
	if err != nil || snapshot.Kind != "directory" {
		return "", errors.Join(errors.New("Neovim configuration source is unavailable"), err)
	}
	return snapshot.Hash, nil
}
func (d *NvimSyncDriver) validate(r Resource, receipt Receipt) error {
	if r.ID != "nvim.sync" || r.Action != "nvim-sync" || receipt.OperationID != "" && !operationID.MatchString(receipt.OperationID) {
		return errors.New("invalid Neovim resource identity")
	}
	for _, path := range []string{d.current(), d.intentPath(strings.Repeat("a", 64))} {
		bound, err := bindConfigDestination(path)
		if err != nil || bound != path {
			return errors.Join(errors.New("Neovim runtime parent was redirected"), err)
		}
	}
	return nil
}
func (d *NvimSyncDriver) intent(operation string) (nvimSyncIntent, error) {
	var intent nvimSyncIntent
	if !operationID.MatchString(operation) {
		return intent, errors.New("invalid Neovim operation")
	}
	data, err := readDocument(d.intentPath(operation))
	if err != nil {
		return intent, err
	}
	if err := Decode(data, &intent); err != nil {
		return intent, err
	}
	if intent.Schema != 1 || intent.Operation != operation || !slices.Contains([]string{"install", "update", "repair", "remove"}, intent.Action) || intent.Step < 0 || intent.Step > 3 || intent.Attempt < 0 || !operationID.MatchString(intent.Desired) {
		return intent, errors.New("invalid Neovim saved intent")
	}
	if intent.PreviousTarget != "" && !d.ownsPath(intent.PreviousTarget) {
		return intent, errors.New("Neovim intent names an unrelated runtime")
	}
	if len(intent.Manifest) > 0 {
		if _, err := d.readManifest(intent); err != nil {
			return intent, err
		}
	}
	return intent, nil
}
func (d *NvimSyncDriver) ownsPath(path string) bool {
	return filepath.Base(path) == "payload" && operationID.MatchString(filepath.Base(filepath.Dir(path))) && path == d.payload(filepath.Base(filepath.Dir(path)))
}
func (d *NvimSyncDriver) save(intent nvimSyncIntent) error {
	return saveDocument(d.intentPath(intent.Operation), intent)
}

func (d *NvimSyncDriver) command(intent nvimSyncIntent, step int, check bool) nativeCommand {
	payload := d.payload(intent.Operation)
	rustupHome := filepath.Join(payload, ".rustup")
	for _, value := range d.Environment {
		if explicit, ok := strings.CutPrefix(value, "RUSTUP_HOME="); ok {
			rustupHome = explicit
		}
	}
	args := []string{"--headless", "-i", "NONE", "--cmd", "lua vim.opt.rtp:prepend(vim.env.DOTFILES_NVIM_CONFIG)"}
	env := append(slices.Clone(d.Environment), "DOTFILES_NVIM_CONFIG="+filepath.Join(d.Repository, "nvim"), "DOTFILES_NVIM_RUNTIME="+payload, "DOTFILES_NVIM_LOCKFILE="+filepath.Join(payload, fmt.Sprintf(".lazy-lock-%d.json", intent.Attempt)), "DOTFILES_TREESITTER_SYNC_INSTALL=", "NVIM_APPNAME=nvim", "NVIM_LOG_FILE="+os.DevNull, "VIMINIT=", "EXINIT=", "NVIM=", "HOME="+filepath.Join(payload, ".home"), "USERPROFILE="+filepath.Join(payload, ".home"), "LOCALAPPDATA="+filepath.Join(payload, ".local"), "APPDATA="+filepath.Join(payload, ".roaming"), "XDG_CONFIG_HOME="+d.Repository, "XDG_DATA_HOME="+filepath.Join(payload, ".data"), "XDG_STATE_HOME="+filepath.Join(payload, ".state"), "XDG_CACHE_HOME="+filepath.Join(payload, ".cache"), "NPM_CONFIG_CACHE="+filepath.Join(payload, ".cache", "npm"), "NPM_CONFIG_USERCONFIG="+filepath.Join(payload, ".home", ".npmrc"), "CARGO_HOME="+filepath.Join(payload, ".cargo"), "RUSTUP_HOME="+rustupHome, "PIP_CONFIG_FILE="+os.DevNull, "PIP_CACHE_DIR="+filepath.Join(payload, ".cache", "pip"), "PIP_TARGET=", "PIP_PREFIX=")
	command := nativeCommand{Program: d.Executable, Environment: env}
	if check {
		command.Arguments = append(args, "--clean", "+lua require('util.sync_check').run()", "+cquit 1")
		return command
	}
	args = append(args, "-u", filepath.Join(d.Repository, "nvim", "init.lua"))
	switch step {
	case 0:
		args = append(args, "+Lazy! restore", "+lua if vim.v.errmsg ~= '' then vim.cmd('cquit 1') end", "+qa")
	case 1:
		env = append(env, "DOTFILES_TREESITTER_SYNC_INSTALL=1")
		args = append(args, "+lua require('lazy').load({ plugins = { 'nvim-treesitter' } })", "+lua if vim.v.errmsg ~= '' then vim.cmd('cquit 1') end", "+qa")
	case 2:
		args = append(args, "+lua require('util.mason_tools').run_checked('MasonToolsInstallSync')", "+cquit 1")
	}
	command.Arguments, command.Environment = args, env
	command.Operation, _ = digest(struct {
		Operation     string
		Step, Attempt int
	}{intent.Operation, step, intent.Attempt})
	return command
}

func queryNvimSync(ctx context.Context, command nativeCommand) ([]byte, error) {
	if err := command.validate(); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, command.Program, command.Arguments...)
	cmd.Env = append(os.Environ(), command.Environment...)
	var output nativeOutput
	cmd.Stdout, cmd.Stderr = &output, &output
	err := cmd.Run()
	if output.exceeded {
		err = errors.Join(err, errors.New("Neovim verification output exceeds bound"))
	}
	return output.data.Bytes(), err
}

func (d *NvimSyncDriver) Observe(ctx context.Context, r Resource, receipt Receipt) (Observation, error) {
	o := Observation{Provider: "nvim-sync", Identity: d.current(), Scope: "user", Preserved: slices.Clone(receipt.After.Preserved)}
	if err := d.validate(r, receipt); err != nil {
		return o, err
	}
	desired, sourceErr := d.source()
	o.Desired = desired
	if sourceErr != nil {
		o.ApplyBlocked = sourceErr.Error()
	}
	current, err := archiveLinkTarget(d.current())
	if err != nil {
		o.Unknown, o.Pending = true, err.Error()
		return o, nil
	}
	if current != "" {
		o.Present = true
		o.Fingerprint, _ = digest(current)
		if !d.ownsPath(current) {
			o.HealthIssue = "Neovim runtime points outside its owned generations"
			return o, nil
		}
		generation, err := d.intent(filepath.Base(filepath.Dir(current)))
		if err != nil || generation.Action == "remove" {
			o.HealthIssue = "Neovim runtime lacks provider evidence"
			return o, nil
		}
		o.Version = generation.Desired
		if sourceErr == nil && generation.Complete && len(generation.Manifest) > 0 {
			output, healthErr := d.query(ctx, d.command(generation, 0, true))
			o.Healthy = healthErr == nil
			if healthErr != nil {
				o.HealthIssue = nativeErrorDetail(healthErr, output)
			}
		}
		if generation.Desired != desired && sourceErr == nil {
			o.Healthy = false
			o.HealthIssue = "Neovim runtime does not match the current configuration"
		}
	}
	if receipt.OperationID != "" {
		intent, err := d.intent(receipt.OperationID)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return o, err
		}
		if err == nil {
			o.Preserved = sortedUnique(append(o.Preserved, intent.Preserved...))
			if intent.Complete && (intent.Action == "remove" && current == "" || intent.Action != "remove" && current == d.payload(intent.Operation) && o.Healthy) {
				o.CompletedOperation = intent.Operation
			}
			if !intent.Complete {
				o.Pending = "Neovim synchronization has a saved incomplete operation"
				partial, err := snapshotTree(d.payload(intent.Operation), maxPackageBytes, maxNvimEntries)
				if err != nil {
					return o, err
				}
				token, err := digest(struct {
					Intent  nvimSyncIntent
					Current string
					Partial configSnapshot
				}{intent, current, partial})
				if err != nil {
					return o, err
				}
				o.ResourceResume = &ResourceResume{Operation: intent.Operation, Token: token}
			}
		}
	}
	return o, ctx.Err()
}

func nativeErrorDetail(err error, output []byte) string {
	text := strings.TrimSpace(string(output))
	if len(text) > 4096 {
		text = text[len(text)-4096:]
	}
	text = strings.Map(func(r rune) rune {
		if r < 32 && r != '\n' && r != '\t' || r == 127 {
			return -1
		}
		return r
	}, text)
	if text == "" {
		return err.Error()
	}
	return err.Error() + ": " + text
}

func (d *NvimSyncDriver) Apply(ctx context.Context, r Resource, op Operation, receipt Receipt) (Observation, error) {
	if err := d.validate(r, receipt); err != nil {
		return Observation{}, err
	}
	if !operationID.MatchString(receipt.OperationID) || !slices.Contains([]string{"install", "update", "repair"}, op.Action) {
		return Observation{}, errors.New("Neovim sync requires durable engine intent")
	}
	intent, err := d.intent(receipt.OperationID)
	if errors.Is(err, os.ErrNotExist) {
		desired, err := d.source()
		if err != nil {
			return Observation{}, err
		}
		previous, err := archiveLinkTarget(d.current())
		if err != nil {
			return Observation{}, err
		}
		if previous != "" && (receipt.Ownership != "created" || !d.ownsPath(previous)) {
			return Observation{}, errors.New("refusing to acquire a pre-existing Neovim runtime")
		}
		if op.Observed.Desired != "" && op.Observed.Desired != desired {
			return Observation{}, errors.New("Neovim source changed after approval")
		}
		intent = nvimSyncIntent{Schema: 1, Operation: receipt.OperationID, Action: op.Action, Desired: desired, PreviousTarget: previous}
		if err := d.save(intent); err != nil {
			return Observation{}, err
		}
	} else if err != nil {
		return Observation{}, err
	}
	if intent.Action != op.Action {
		return Observation{}, errors.New("Neovim saved action differs")
	}
	return d.apply(ctx, r, receipt, intent)
}

func (d *NvimSyncDriver) apply(ctx context.Context, r Resource, receipt Receipt, intent nvimSyncIntent) (Observation, error) {
	if intent.Complete {
		return d.Observe(ctx, r, receipt)
	}
	desired, err := d.source()
	if err != nil || desired != intent.Desired {
		return Observation{}, errors.Join(errors.New("Neovim source changed since saved intent"), err)
	}
	if d.Run == nil {
		return Observation{}, errors.New("Neovim sync requires its native command session")
	}
	payload := d.payload(intent.Operation)
	if !intent.Reserved {
		if err := prepareStateDirectory(filepath.Dir(payload)); err != nil {
			return Observation{}, err
		}
		if _, err := os.Lstat(payload); err == nil {
			backup := payload + ".interrupted-" + rand.Text()
			intent.Preserved = sortedUnique(append(intent.Preserved, backup))
			if err := d.save(intent); err != nil {
				return Observation{}, err
			}
			if err := moveConfigExclusive(payload, backup); err != nil {
				return Observation{}, err
			}
		} else if !errors.Is(err, os.ErrNotExist) {
			return Observation{}, err
		}
		if err := os.Mkdir(payload, 0700); err != nil {
			return Observation{}, fmt.Errorf("cannot reserve fresh Neovim runtime: %w", err)
		}
		intent.Reserved = true
		if err := d.save(intent); err != nil {
			return Observation{}, err
		}
	}
	if intent.Failed {
		return Observation{}, errors.New("Neovim phase failed; explicitly resume its saved operation")
	}
	bound, err := bindConfigDestination(filepath.Join(payload, "lazy"))
	if err != nil || bound != filepath.Join(payload, "lazy") {
		return Observation{}, errors.Join(errors.New("Neovim stage was redirected"), err)
	}
	lockfile := filepath.Join(payload, fmt.Sprintf(".lazy-lock-%d.json", intent.Attempt))
	if _, err := os.Lstat(lockfile); errors.Is(err, os.ErrNotExist) {
		if err := copyConfigPayload(filepath.Join(d.Repository, "nvim", "lazy-lock.json"), lockfile); err != nil {
			return Observation{}, err
		}
	} else if err != nil {
		return Observation{}, err
	}
	locked, err := snapshotConfig(lockfile)
	sourceLock, sourceErr := snapshotConfig(filepath.Join(d.Repository, "nvim", "lazy-lock.json"))
	if err != nil || sourceErr != nil || locked != sourceLock || locked.Kind != "file" {
		return Observation{}, errors.Join(errors.New("Neovim private lock differs from the reviewed source"), err, sourceErr)
	}
	for intent.Step < 3 {
		output, err := d.Run(ctx, d.command(intent, intent.Step, false))
		if err != nil {
			intent.Failed = true
			return Observation{}, errors.Join(errors.New(nativeErrorDetail(err, output)), d.save(intent))
		}
		intent.Step++
		if err := d.save(intent); err != nil {
			return Observation{}, err
		}
	}
	output, err := d.query(ctx, d.command(intent, 0, true))
	if err != nil {
		// Neovim commands can report an error while exiting zero. A failed
		// postcondition must permit a separately approved rerun of the recipe.
		intent.Failed, intent.Step = true, 0
		return Observation{}, errors.Join(errors.New(nativeErrorDetail(err, output)), d.save(intent))
	}
	desired, err = d.source()
	if err != nil || desired != intent.Desired {
		return Observation{}, errors.Join(errors.New("Neovim source changed during synchronization"), err)
	}
	if len(intent.Manifest) == 0 {
		entries, err := inspectTree(payload, maxPackageBytes, maxNvimEntries)
		if err != nil {
			return Observation{}, err
		}
		intent.Manifest, err = d.writeManifest(intent.Operation, entries)
		if err != nil {
			return Observation{}, err
		}
		if err := d.save(intent); err != nil {
			return Observation{}, err
		}
	}
	publication := archiveIntent{Resource: "nvim.sync", Operation: intent.Operation, Generation: intent.Operation, PreviousTarget: intent.PreviousTarget}
	if err := d.links().switchLink(publication, false); err != nil {
		return Observation{}, err
	}
	intent.Complete = true
	if err := d.save(intent); err != nil {
		return Observation{}, err
	}
	return d.Observe(ctx, r, receipt)
}

func (d *NvimSyncDriver) ResumeResource(ctx context.Context, r Resource, op Operation, receipt Receipt) (Observation, error) {
	if err := d.validate(r, receipt); err != nil {
		return Observation{}, err
	}
	actual, err := d.Observe(ctx, r, receipt)
	if err != nil || actual.ResourceResume == nil || op.Observed.ResourceResume == nil || *actual.ResourceResume != *op.Observed.ResourceResume {
		return Observation{}, errors.Join(errors.New("Neovim recovery changed after approval"), err)
	}
	intent, err := d.intent(receipt.OperationID)
	if err != nil {
		return Observation{}, err
	}
	if intent.Action != op.Action {
		return Observation{}, errors.New("Neovim recovery action differs")
	}
	if intent.Failed {
		intent.Attempt++
		intent.Failed = false
		if err := d.save(intent); err != nil {
			return Observation{}, err
		}
	}
	if intent.Action == "remove" {
		return d.remove(ctx, r, receipt, intent)
	}
	return d.apply(ctx, r, receipt, intent)
}
