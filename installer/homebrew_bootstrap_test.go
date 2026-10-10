package installer

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
)

type bootstrapTransport func(*http.Request) (*http.Response, error)

func (f bootstrapTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

type homebrewBootstrapFixture struct {
	t                 *testing.T
	d                 *HomebrewBootstrapDriver
	r                 Resource
	commands          []nativeCommand
	kinds             []string
	inventory, source string
	fail, lose        string
	partial           bool
	beforeRun         func(string)
}

func newHomebrewBootstrapFixture(t *testing.T) *homebrewBootstrapFixture {
	t.Helper()
	if os.PathSeparator != '/' {
		t.Skip("Homebrew uses POSIX paths")
	}
	root, err := resolveConfigPath(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	session := newNativeSession(filepath.Join(root, "native", "worker"))
	d, err := configureHomebrewBootstrap(NativePlatform{Context: Context{OS: "darwin", Arch: "arm64"}}, ConfigFolders{Home: root}, session)
	if err != nil {
		t.Fatal(err)
	}
	d.prefix = filepath.Join(root, "prefix")
	payload := "#!/bin/bash\n# public fixture payload; never executed\n"
	sum := sha256.Sum256([]byte(payload))
	d.recipe.SHA256 = hex.EncodeToString(sum[:])
	d.HTTPClient = &http.Client{Transport: bootstrapTransport(func(req *http.Request) (*http.Response, error) {
		if req.URL.String() != homebrewBootstrapURL {
			t.Fatal("unexpected payload source", req.URL)
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(payload)), Header: make(http.Header), Request: req}, nil
	})}
	f := &homebrewBootstrapFixture{t: t, d: d, r: Resource{ID: "infra.homebrew", Retain: true, ReplanAfter: true}, inventory: `{"formulae":[],"casks":[]}`, source: "https://github.com/Homebrew/brew"}
	d.Query = func(_ context.Context, privileged bool, program string, input []byte, args ...string) ([]byte, error) {
		if privileged || len(input) != 0 {
			t.Fatal("query attempted native mutation")
		}
		if program == "/usr/bin/env" && slices.Contains(args, "remote.origin.url") {
			return []byte(f.source + "\n"), nil
		}
		if program != "brew" {
			t.Fatal("unexpected query", program, args)
		}
		switch strings.Join(args, " ") {
		case "--version":
			return []byte("Homebrew 4.fixture\n"), nil
		case "--prefix":
			return []byte(d.location().Prefix + "\n"), nil
		case "--cellar":
			return []byte(d.location().Cellar + "\n"), nil
		case "info --json=v2 --installed":
			return []byte(f.inventory), nil
		default:
			t.Fatal("unexpected brew query", args)
			return nil, errors.New("unsupported query")
		}
	}
	d.Run = f.run
	return f
}

func (f *homebrewBootstrapFixture) installManager() {
	f.t.Helper()
	if err := os.MkdirAll(filepath.Dir(f.d.location().Program), 0755); err != nil {
		f.t.Fatal(err)
	}
	if err := os.WriteFile(f.d.location().Program, []byte("#!/bin/sh\nprintf 'Homebrew 4.fixture\\n'\n"), 0755); err != nil {
		f.t.Fatal(err)
	}
}

func (f *homebrewBootstrapFixture) run(_ context.Context, command nativeCommand) ([]byte, error) {
	kind := "verify"
	if command.Program == "/usr/bin/sudo" {
		kind = "reserve"
	} else if len(command.Input) != 0 {
		kind = "install"
	}
	f.commands = append(f.commands, command)
	f.kinds = append(f.kinds, kind)
	if f.beforeRun != nil {
		f.beforeRun(kind)
	}
	code := 0
	failure := ""
	failed := f.fail == kind
	if failed {
		f.fail = ""
		code = 1
		failure = "fixture native prerequisite"
	}
	if !failed || f.partial {
		switch kind {
		case "reserve":
			if err := os.Mkdir(f.d.location().Prefix, 0755); err != nil {
				code = 1
				failure = err.Error()
			}
		case "install":
			f.installManager()
		}
	}
	reply := nativeReply{ExitCode: &code, Error: failure}
	hash, err := digest(command)
	if err != nil {
		return nil, err
	}
	if err := saveDocument(filepath.Join(f.d.WorkerDirectory, "commands", command.Operation+".json"), nativeCommandRecord{Schema: 1, Command: hash, Reply: &reply}); err != nil {
		return nil, err
	}
	if f.lose == kind {
		f.lose = ""
		return nil, errors.New("fixture lost worker transport")
	}
	if failure != "" {
		return nil, errors.New(failure)
	}
	return nil, nil
}

func bootstrapReceipt(letter string) Receipt {
	return Receipt{OperationID: strings.Repeat(letter, 64), Status: "in-progress"}
}

func (f *homebrewBootstrapFixture) apply(action, letter string) (Observation, error) {
	return f.d.Apply(context.Background(), f.r, Operation{Action: action}, bootstrapReceipt(letter))
}

func TestHomebrewBootstrapConstructorAndReleasedPins(t *testing.T) {
	root := t.TempDir()
	session := newNativeSession(filepath.Join(root, "missing", "worker"))
	folders := ConfigFolders{Home: root}
	for _, platform := range []Context{{OS: "darwin", Arch: "amd64"}, {OS: "darwin", Arch: "bad"}} {
		if _, err := configureHomebrewBootstrap(NativePlatform{Context: platform}, folders, session); err == nil {
			t.Fatal("unsupported bootstrap target accepted", platform)
		}
	}
	d, err := configureHomebrewBootstrap(NativePlatform{Context: Context{OS: "darwin", Arch: "arm64"}}, folders, session)
	if err != nil || d.location().Prefix != "/opt/homebrew" || d.Existing != nil {
		t.Fatal(d, err)
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 0 {
		t.Fatal("constructor performed IO", entries, err)
	}
	// These reviewed pins are independent of the retired monolithic installer.
	// Updating the bootstrap requires a new verified upstream revision and hash.
	if homebrewBootstrapCommit != "35da6871c4be7d7fdab2fd505fb7fa667926a2a5" ||
		homebrewBootstrapSHA256 != "5f333bbe53bc490e51e7ccb1df8779b3dd6ee73a1a7379efda216edb08ccb148" {
		t.Fatal("Homebrew bootstrap pins changed without updating reviewed evidence")
	}
}

func TestHomebrewBootstrapFreshLifecycleAndRetention(t *testing.T) {
	f := newHomebrewBootstrapFixture(t)
	ctx := context.Background()
	before, err := f.d.Observe(ctx, f.r, Receipt{})
	if err != nil || before.Present || before.Unknown || before.ApplyBlocked != "" {
		t.Fatal(before, err)
	}
	if _, err := os.Stat(f.d.Directory); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("observation created state", err)
	}
	o, err := f.apply("install", "a")
	if err != nil || !o.Healthy || o.CompletedOperation != bootstrapReceipt("a").OperationID || !slices.Equal(f.kinds, []string{"reserve", "install"}) {
		t.Fatal(o, err, f.kinds)
	}
	location := f.d.location()
	f.d.Existing = &location // Real rediscovery after the bootstrap boundary.
	if _, err := f.apply("update", "b"); err == nil || len(f.commands) != 2 {
		t.Fatal("tool updates invoked manager maintenance", err, f.kinds)
	}
	homebrewBootstrapRetainedPlan(t, f.r, o, "update", "keep")
	homebrewBootstrapRetainedPlan(t, f.r, o, "apply", "retain")
	if _, err := f.d.Remove(ctx, f.r, bootstrapReceipt("a")); err == nil {
		t.Fatal("retained infrastructure was removable")
	}
	if _, err := os.Stat(location.Program); err != nil {
		t.Fatal("manager was removed", err)
	}
	record, err := f.d.record()
	if err != nil || record.CreatedBy != bootstrapReceipt("a").OperationID {
		t.Fatal("maintenance rewrote creation provenance", record, err)
	}
}

func TestHomebrewBootstrapReusesExistingWithoutAcquiring(t *testing.T) {
	f := newHomebrewBootstrapFixture(t)
	f.installManager()
	location := f.d.location()
	f.d.Existing = &location
	o, err := f.d.Observe(context.Background(), f.r, Receipt{})
	if err != nil || !o.Present || !o.Healthy || o.CompletedOperation != "" {
		t.Fatal(o, err)
	}
	existing, err := (existingHomebrewDriver{location}).Observe(context.Background(), f.r, Receipt{})
	if err != nil || o.Identity != existing.Identity || o.Provider != existing.Provider || o.Fingerprint != existing.Fingerprint {
		t.Fatal("rediscovery changed existing identity", o, existing, err)
	}
	for _, action := range []string{"install", "update", "repair"} {
		if _, err := f.apply(action, "a"); err == nil {
			t.Fatal("acquired pre-existing manager", action)
		}
	}
	if len(f.commands) != 0 {
		t.Fatal("pre-existing manager mutated", f.kinds)
	}
	if _, err := f.d.record(); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("existing manager acquired record", err)
	}
}

func TestHomebrewBootstrapPreservesUnknownPrefixAndReservationRace(t *testing.T) {
	for _, race := range []bool{false, true} {
		t.Run(map[bool]string{false: "existing", true: "racing"}[race], func(t *testing.T) {
			f := newHomebrewBootstrapFixture(t)
			create := func() {
				if err := os.Mkdir(f.d.prefix, 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(f.d.prefix, "outside"), []byte("preserve"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			if race {
				f.d.Authenticate = func(context.Context) error { create(); return nil }
			} else {
				create()
			}
			if _, err := f.apply("install", "a"); err == nil {
				t.Fatal("existing prefix overwritten")
			}
			data, err := os.ReadFile(filepath.Join(f.d.prefix, "outside"))
			if err != nil || string(data) != "preserve" {
				t.Fatal("outside prefix changed", err)
			}
			if slices.Contains(f.kinds, "install") {
				t.Fatal("bootstrap ran over outside prefix", f.kinds)
			}
		})
	}
}

func TestHomebrewBootstrapRecoversLostSuccessfulWorkerReplies(t *testing.T) {
	for _, kind := range []string{"reserve", "install", "verify"} {
		t.Run(kind, func(t *testing.T) {
			f := newHomebrewBootstrapFixture(t)
			if kind == "verify" {
				f.fail, f.partial = "install", true
				if _, err := f.apply("install", "a"); err == nil {
					t.Fatal("expected installer failure")
				}
			}
			f.lose = kind
			var err error
			if kind == "verify" {
				_, err = f.d.ResumeResource(context.Background(), f.r, Operation{Action: "install"}, bootstrapReceipt("a"))
			} else {
				_, err = f.apply("install", "a")
			}
			if err == nil {
				t.Fatal("lost reply was hidden")
			}
			o, err := f.d.Observe(context.Background(), f.r, bootstrapReceipt("a"))
			if err != nil || o.ResourceResume == nil {
				t.Fatal("missing recovery preview", o, err)
			}
			o, err = f.d.ResumeResource(context.Background(), f.r, Operation{Action: "install"}, bootstrapReceipt("a"))
			if err != nil || !o.Healthy || o.CompletedOperation != bootstrapReceipt("a").OperationID {
				t.Fatal(o, err)
			}
			if count := len(slices.DeleteFunc(slices.Clone(f.kinds), func(s string) bool { return s != kind })); count != 1 {
				t.Fatal("successful command replayed", f.kinds)
			}
		})
	}
}

func TestHomebrewBootstrapFailedNativeCommandRequiresExplicitResume(t *testing.T) {
	f := newHomebrewBootstrapFixture(t)
	f.fail = "install"
	if _, err := f.apply("install", "a"); err == nil || !strings.Contains(err.Error(), "foreground") {
		t.Fatal("missing concrete native prerequisite diagnostic", err)
	}
	if _, err := f.apply("install", "a"); err == nil {
		t.Fatal("failed bootstrap retried without recovery")
	}
	o, err := f.d.ResumeResource(context.Background(), f.r, Operation{Action: "install"}, bootstrapReceipt("a"))
	if err != nil || !o.Healthy || !slices.Equal(f.kinds, []string{"reserve", "install", "install"}) {
		t.Fatal(o, err, f.kinds)
	}
}

const bootstrapOutsideInventory = `{"formulae":[{"name":"outside","full_name":"outside","pinned":true,"linked_keg":"1","installed":[{"version":"1","installed_on_request":true,"runtime_dependencies":[]}]}],"casks":[]}`

func TestHomebrewBootstrapPartialInstallRecoversWithExistingPackages(t *testing.T) {
	f := newHomebrewBootstrapFixture(t)
	f.fail, f.partial = "install", true
	if _, err := f.apply("install", "a"); err == nil {
		t.Fatal("partial install reported success")
	}
	f.inventory = bootstrapOutsideInventory
	o, err := f.d.ResumeResource(context.Background(), f.r, Operation{Action: "install"}, bootstrapReceipt("a"))
	if err != nil || !o.Healthy || !slices.Equal(f.kinds, []string{"reserve", "install", "verify"}) {
		t.Fatal("working manager was recursively bootstrapped", o, err, f.kinds)
	}
	if f.inventory != bootstrapOutsideInventory {
		t.Fatal("outside packages changed")
	}
	if _, err := f.apply("update", "b"); err == nil {
		t.Fatal("retained manager attempted global update")
	}
}

func TestHomebrewBootstrapRejectsChangedSourceAndPrefixDuringRecovery(t *testing.T) {
	for _, change := range []string{"source", "prefix"} {
		t.Run(change, func(t *testing.T) {
			f := newHomebrewBootstrapFixture(t)
			f.fail, f.partial = "install", true
			if _, err := f.apply("install", "a"); err == nil {
				t.Fatal("expected installer failure")
			}
			if change == "source" {
				f.source = "https://example.invalid/outside"
			} else {
				if err := os.Rename(f.d.prefix, f.d.prefix+".original"); err != nil {
					t.Fatal(err)
				}
				f.installManager()
			}
			if _, err := f.d.ResumeResource(context.Background(), f.r, Operation{Action: "install"}, bootstrapReceipt("a")); err == nil {
				t.Fatal("changed infrastructure completed implicitly")
			}
			if len(f.commands) != 2 {
				t.Fatal("changed infrastructure mutated", f.kinds)
			}
		})
	}
}

func TestHomebrewBootstrapChecksDownloadBeforeElevationAndSanitizesEnvironment(t *testing.T) {
	f := newHomebrewBootstrapFixture(t)
	calls := 0
	f.d.Authenticate = func(context.Context) error { calls++; return nil }
	original := f.d.recipe
	f.d.recipe.SHA256 = strings.Repeat("0", 64)
	if _, err := f.apply("install", "a"); err == nil {
		t.Fatal("wrong checksum accepted")
	}
	if calls != 0 || len(f.commands) != 0 {
		t.Fatal("bad payload reached elevation")
	}
	f.d.recipe = original
	if _, err := f.apply("install", "b"); err != nil {
		t.Fatal(err)
	}
	command := f.commands[1]
	for _, required := range []string{"-i", "NONINTERACTIVE=1", "SUDO_ASKPASS=/usr/bin/false", "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1", "HOMEBREW_BREW_GIT_REMOTE=https://github.com/Homebrew/brew"} {
		if !slices.Contains(command.Arguments, required) {
			t.Fatal("missing bootstrap environment boundary", required)
		}
	}
	if len(command.Environment) != 0 || command.Program != "/usr/bin/env" || !reflect.DeepEqual(f.commands[0].Arguments, []string{"-n", "/bin/mkdir", f.d.prefix}) {
		t.Fatal("unsafe native bootstrap command", command.Program)
	}
}

func TestHomebrewBootstrapRequiresSuccessHealthAndUnchangedPackages(t *testing.T) {
	for _, fault := range []string{"health", "inventory", "unfinished"} {
		t.Run(fault, func(t *testing.T) {
			f := newHomebrewBootstrapFixture(t)
			originalRun := f.d.Run
			f.d.Run = func(ctx context.Context, command nativeCommand) ([]byte, error) {
				data, err := originalRun(ctx, command)
				if len(command.Input) == 0 || err != nil {
					return data, err
				}
				switch fault {
				case "health":
					if err := os.Remove(f.d.location().Program); err != nil {
						t.Fatal(err)
					}
				case "inventory":
					f.inventory = bootstrapOutsideInventory
				case "unfinished":
					hash, err := digest(command)
					if err != nil {
						t.Fatal(err)
					}
					if err := saveDocument(filepath.Join(f.d.WorkerDirectory, "commands", command.Operation+".json"), nativeCommandRecord{Schema: 1, Command: hash}); err != nil {
						t.Fatal(err)
					}
				}
				return data, nil
			}
			if _, err := f.apply("install", "a"); err == nil {
				t.Fatal("unproved bootstrap completed", fault)
			}
			if _, err := f.d.record(); !errors.Is(err, os.ErrNotExist) {
				t.Fatal("unproved bootstrap acquired ownership", err)
			}
		})
	}
}

func TestHomebrewBootstrapRejectsTamperedSavedCommandOrPayload(t *testing.T) {
	for _, tamper := range []string{"command", "payload"} {
		t.Run(tamper, func(t *testing.T) {
			f := newHomebrewBootstrapFixture(t)
			f.fail = "install"
			if _, err := f.apply("install", "a"); err == nil {
				t.Fatal("expected failure")
			}
			intent, err := f.d.intent(bootstrapReceipt("a").OperationID)
			if err != nil {
				t.Fatal(err)
			}
			if tamper == "command" {
				intent.Commands[1].Command.Arguments = append(intent.Commands[1].Command.Arguments, "unreviewed")
				if err := saveDocument(f.d.intentPath(intent.Operation), intent); err != nil {
					t.Fatal(err)
				}
			} else if err := os.WriteFile(filepath.Join(f.d.Directory, "payloads", intent.Operation+".sh"), []byte("changed"), 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := f.d.ResumeResource(context.Background(), f.r, Operation{Action: "install"}, bootstrapReceipt("a")); err == nil {
				t.Fatal("changed saved recipe accepted")
			}
			if len(f.commands) != 2 {
				t.Fatal("tampering reached native worker")
			}
		})
	}
}

func TestHomebrewBootstrapWorkerRunsOnlyVerifiedPayloadWithCleanEnvironment(t *testing.T) {
	if os.PathSeparator != '/' {
		t.Skip("POSIX bootstrap process boundary")
	}
	f := newHomebrewBootstrapFixture(t)
	ctx := context.Background()
	payload := `#!/bin/bash
set -eu
[ "$NONINTERACTIVE" = 1 ]
[ "$SUDO_ASKPASS" = /usr/bin/false ]
[ "$GIT_CONFIG_GLOBAL" = /dev/null ]
[ "${DOTFILES_BOOTSTRAP_PRIVATE_FIXTURE-unset}" = unset ]
[ "${BASH_ENV-unset}" = unset ]
printf 'verified noninteractive payload'
`
	sum := sha256.Sum256([]byte(payload))
	f.d.recipe.SHA256 = hex.EncodeToString(sum[:])
	f.d.HTTPClient.Transport = bootstrapTransport(func(req *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(payload)), Header: make(http.Header), Request: req}, nil
	})
	operation := bootstrapReceipt("a").OperationID
	if err := f.d.preparePayload(ctx, operation, f.d.recipe); err != nil {
		t.Fatal(err)
	}
	intent := homebrewBootstrapIntent{Operation: operation, Action: "install", Location: f.d.location(), Recipe: f.d.recipe}
	command, err := f.d.command(intent, 1, "install")
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("DOTFILES_BOOTSTRAP_PRIVATE_FIXTURE", "fixture-only-value")
	envFile := filepath.Join(f.d.Home, "unapproved-bash-env")
	if err := os.WriteFile(envFile, []byte("exit 77\n"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("BASH_ENV", envFile)
	client, err := workerFixture(f.d.WorkerDirectory)
	if err != nil {
		t.Fatal(err)
	}
	output, runErr := client.run(ctx, command)
	if err := errors.Join(runErr, client.close()); err != nil || string(output) != "verified noninteractive payload" {
		t.Fatal("verified command inherited unapproved process inputs", string(output), err)
	}
	reply, err := f.d.result(command)
	if err != nil || reply.Error != "" || reply.ExitCode == nil || *reply.ExitCode != 0 {
		t.Fatal("worker did not persist payload success", reply, err)
	}
}

func TestHomebrewBootstrapPartialRepositorySourceIsPreserved(t *testing.T) {
	f := newHomebrewBootstrapFixture(t)
	f.fail = "install"
	if _, err := f.apply("install", "a"); err == nil {
		t.Fatal("expected install failure")
	}
	if err := os.Mkdir(filepath.Join(f.d.prefix, ".git"), 0755); err != nil {
		t.Fatal(err)
	}
	f.source = "https://example.invalid/outside"
	if _, err := f.d.ResumeResource(context.Background(), f.r, Operation{Action: "install"}, bootstrapReceipt("a")); err == nil {
		t.Fatal("changed partial repository source accepted")
	}
	if len(f.commands) != 2 {
		t.Fatal("changed partial repository reached native worker", f.kinds)
	}
}

func homebrewBootstrapRetainedPlan(t *testing.T, r Resource, o Observation, mode, want string) {
	t.Helper()
	r.Name, r.Action = "Homebrew", "native"
	platform := Context{OS: "darwin", Arch: "arm64"}
	catalog := &Catalog{Schema: 1, Resources: []Resource{r, {ID: "fixture", Name: "Selected fixture", Capability: true, Requires: []string{r.ID}}}}
	state := State{Schema: 1, Context: platform, Target: "bootstrap-fixture", Selected: []string{"fixture"}, Receipts: map[string]Receipt{r.ID: {Ownership: "created", Status: "installed", After: o}}}
	request := Request{Schema: 1, Mode: mode}
	if mode == "apply" {
		request.Selected = []string{}
	}
	plan, err := PlanChanges(catalog, platform, state, request, map[string]Observation{r.ID: o}, "bootstrap-fixture")
	if err != nil || len(plan.Operations) != 1 || plan.Operations[0].Action != want {
		t.Fatal("retained bootstrap policy was not planned", mode, plan, err)
	}
}

func TestHomebrewBootstrapRecoveryUsesOriginalVerifiedPin(t *testing.T) {
	f := newHomebrewBootstrapFixture(t)
	f.lose = "reserve"
	if _, err := f.apply("install", "a"); err == nil {
		t.Fatal("expected lost reservation reply")
	}
	f.d.recipe.SHA256 = strings.Repeat("0", 64)
	o, err := f.d.ResumeResource(context.Background(), f.r, Operation{Action: "install"}, bootstrapReceipt("a"))
	if err != nil || !o.Healthy {
		t.Fatal("installer revision replaced the pending operation pin", o, err)
	}
}

func TestHomebrewBootstrapResumedVerificationLeavesNewPackagesUnowned(t *testing.T) {
	f := newHomebrewBootstrapFixture(t)
	f.beforeRun = func(kind string) {
		if kind == "install" {
			f.inventory = bootstrapOutsideInventory
		}
	}
	if _, err := f.apply("install", "a"); err == nil {
		t.Fatal("unexpected concurrent package inventory was not disclosed")
	}
	o, err := f.d.ResumeResource(context.Background(), f.r, Operation{Action: "install"}, bootstrapReceipt("a"))
	if err != nil || !o.Healthy || !slices.Equal(f.kinds, []string{"reserve", "install", "verify"}) {
		t.Fatal("read-only recovery did not preserve outside packages", o, err, f.kinds)
	}
	if f.inventory != bootstrapOutsideInventory {
		t.Fatal("outside package changed")
	}
}

func TestHomebrewBootstrapCompletionBindsCurrentCreationRecord(t *testing.T) {
	f := newHomebrewBootstrapFixture(t)
	if _, err := f.apply("install", "a"); err != nil {
		t.Fatal(err)
	}
	record, err := f.d.record()
	if err != nil {
		t.Fatal(err)
	}
	record.CreatedBy = bootstrapReceipt("b").OperationID
	if err := saveDocument(filepath.Join(f.d.Directory, "created.json"), record); err != nil {
		t.Fatal(err)
	}
	o, err := f.d.Observe(context.Background(), f.r, bootstrapReceipt("a"))
	if err != nil || o.CompletedOperation != "" {
		t.Fatal("historical operation claimed a different creation record", o, err)
	}
}
