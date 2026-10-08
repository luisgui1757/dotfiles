package installer

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

type appleCLTFixture struct {
	t                    *testing.T
	d                    *AppleCLTDriver
	r                    Resource
	selected, fail, lose string
	installed, unhealthy bool
	receipt              string
	unchanged            bool
	commands             []nativeCommand
	modes                []string
}

func newAppleCLTFixture(t *testing.T) *appleCLTFixture {
	t.Helper()
	if os.PathSeparator != '/' {
		t.Skip("Apple tools use POSIX paths")
	}
	root, err := resolveConfigPath(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	session := newNativeSession(filepath.Join(root, "state", "worker"))
	d, err := configureAppleCLT(NativePlatform{Context: Context{OS: "darwin", Arch: "arm64"}}, ConfigFolders{Home: root}, session)
	if err != nil {
		t.Fatal(err)
	}
	d.clt, d.placeholder = filepath.Join(root, "CLT"), filepath.Join(root, "placeholder")
	f := &appleCLTFixture{t: t, d: d, r: Resource{ID: "infra.apple-clt", Retain: true, ReplanAfter: true}}
	d.Query = f.query
	d.Run = f.run
	return f
}

func (f *appleCLTFixture) query(_ context.Context, privileged bool, program string, input []byte, args ...string) ([]byte, error) {
	if privileged || len(input) != 0 {
		f.t.Fatal("inspection attempted mutation")
	}
	switch program {
	case "/usr/bin/xcode-select":
		if f.selected == "" {
			return nil, errAppleCLTNoSelection
		}
		return []byte(f.selected + "\n"), nil
	case "/usr/sbin/pkgutil":
		if args[0] == "--pkgs" {
			if f.installed || f.receipt != "" {
				return []byte("com.apple.pkg.CLTools_Executables\n"), nil
			}
			return []byte("unrelated.package\ncom.apple.pkg.CLTools_Executables.extra\n"), nil
		}
		if f.installed || f.receipt != "" {
			receipt := f.receipt
			if receipt == "" {
				receipt = "26.0.0@100"
			}
			version, timestamp, _ := strings.Cut(receipt, "@")
			return []byte("package-id: com.apple.pkg.CLTools_Executables\nversion: " + version + "\nvolume: /\nlocation: /\ninstall-time: " + timestamp + "\n"), nil
		}
		return nil, errors.New("no package receipt")
	case "/usr/bin/xcrun":
		if slices.Contains(args, "--show-sdk-path") {
			return []byte(filepath.Join(f.selected, "SDKs", "macos.sdk") + "\n"), nil
		}
		return []byte(filepath.Join(f.selected, "usr", "bin", args[len(args)-1]) + "\n"), nil
	default:
		if f.unhealthy {
			return nil, errors.New("broken compiler")
		}
		if filepath.Base(program) == "make" {
			return []byte("GNU Make 3.81\n"), nil
		}
		return []byte("Apple clang version 17.0.0\n"), nil
	}
}

func (f *appleCLTFixture) install(path string) {
	f.t.Helper()
	for _, directory := range []string{filepath.Join(path, "usr", "bin"), filepath.Join(path, "SDKs", "macos.sdk")} {
		if err := os.MkdirAll(directory, 0755); err != nil {
			f.t.Fatal(err)
		}
	}
	for _, name := range []string{"clang", "clang++", "make"} {
		if err := os.WriteFile(filepath.Join(path, "usr", "bin", name), []byte("fixture; never executed"), 0755); err != nil {
			f.t.Fatal(err)
		}
	}
	f.installed = true
	if !f.unchanged {
		f.receipt = "26.0.0@200"
	}
}

func (f *appleCLTFixture) run(_ context.Context, command nativeCommand) ([]byte, error) {
	mode := command.Arguments[len(command.Arguments)-6]
	operation := command.Arguments[len(command.Arguments)-7]
	f.commands, f.modes = append(f.commands, command), append(f.modes, mode)
	code, failure, output := 0, "", ""
	if f.fail != "before" {
		if mode == "install" {
			f.install(f.d.clt)
			output += "DOTFILES_CLT_RECEIPT:" + f.receipt + "\n"
			output += "DOTFILES_CLT_INSTALLED:" + operation + "\n"
		}
		if f.fail != "after-install" {
			f.selected = f.d.clt
			output += "DOTFILES_CLT_SELECTED:" + operation + "\n"
		}
	}
	if f.fail != "" {
		code, failure = 1, "fixture native failure"
		f.fail = ""
	}
	reply := nativeReply{ExitCode: &code, Error: failure, Output: []byte(output)}
	hash, err := digest(command)
	if err != nil {
		return nil, err
	}
	if err := saveDocument(filepath.Join(f.d.WorkerDirectory, "commands", command.Operation+".json"), nativeCommandRecord{Schema: 1, Command: hash, Reply: &reply}); err != nil {
		return nil, err
	}
	if f.lose != "" {
		f.lose = ""
		return nil, errors.New("lost reply")
	}
	if failure != "" {
		return nil, errors.New(failure)
	}
	return []byte(output), nil
}

func (f *appleCLTFixture) apply() (Observation, error) {
	observed, err := f.d.Observe(context.Background(), f.r, Receipt{})
	if err != nil {
		return observed, err
	}
	return f.d.Apply(context.Background(), f.r, Operation{Action: "install", Observed: observed}, bootstrapReceipt("a"))
}

func TestAppleCLTPureConstructorAndBoundary(t *testing.T) {
	root := t.TempDir()
	session := newNativeSession(filepath.Join(root, "state", "worker"))
	folders := ConfigFolders{Home: root}
	d, err := configureAppleCLT(NativePlatform{Context: Context{OS: "linux", Arch: "arm64"}}, folders, session)
	if err != nil || d != nil {
		t.Fatal(d, err)
	}
	if _, err := configureAppleCLT(NativePlatform{Context: Context{OS: "darwin", Arch: "amd64"}}, folders, session); err == nil {
		t.Fatal("Intel accepted")
	}
	d, err = configureAppleCLT(NativePlatform{Context: Context{OS: "darwin", Arch: "arm64"}}, folders, session)
	if err != nil || d.clt != "/Library/Developer/CommandLineTools" {
		t.Fatal(d, err)
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 0 {
		t.Fatal("constructor performed IO", err)
	}
}

func TestAppleCLTFreshLifecycleRetainsInfrastructure(t *testing.T) {
	f := newAppleCLTFixture(t)
	before, err := f.d.Observe(context.Background(), f.r, Receipt{})
	if err != nil || before.Present || before.Unknown || before.Pending != "" {
		t.Fatal(before, err)
	}
	if _, err := os.Stat(f.d.Directory); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("preview created state", err)
	}
	auth := 0
	f.d.Authenticate = func(context.Context) error { auth++; return nil }
	after, err := f.apply()
	if err != nil || !after.Healthy || after.CompletedOperation != bootstrapReceipt("a").OperationID || auth != 1 || len(f.commands) != 1 {
		t.Fatal(after, err, auth, f.modes)
	}
	command := f.commands[0]
	if command.Program != "/usr/bin/sudo" || !slices.Equal(command.Arguments[:6], []string{"-n", "/usr/bin/env", "-i", "PATH=/usr/bin:/bin:/usr/sbin:/sbin", "LC_ALL=C", "/bin/bash"}) || len(command.Environment) != 0 {
		t.Fatal("native action is not isolated and noninteractive", command.Program, command.Arguments)
	}
	homebrewBootstrapRetainedPlan(t, f.r, after, "update", "keep")
	homebrewBootstrapRetainedPlan(t, f.r, after, "apply", "retain")
	if _, err := f.d.Apply(context.Background(), f.r, Operation{Action: "update"}, bootstrapReceipt("b")); err == nil {
		t.Fatal("implicit developer tools update accepted")
	}
	if _, err := f.d.Remove(context.Background(), f.r, Receipt{}); err == nil {
		t.Fatal("developer tools removable")
	}
	if len(f.commands) != 1 {
		t.Fatal("retained lifecycle executed commands")
	}
}

func TestAppleCLTReusesXcodeAndCLTWithoutOwnership(t *testing.T) {
	for _, source := range []string{"xcode", "clt"} {
		t.Run(source, func(t *testing.T) {
			f := newAppleCLTFixture(t)
			f.selected = f.d.clt
			if source == "xcode" {
				f.selected = filepath.Join(filepath.Dir(f.d.clt), "Xcode.app", "Contents", "Developer")
			}
			f.install(f.selected)
			o, err := f.d.Observe(context.Background(), f.r, Receipt{})
			if err != nil || !o.Healthy || o.CompletedOperation != "" {
				t.Fatal(o, err)
			}
			if _, err := f.apply(); err == nil || len(f.commands) != 0 {
				t.Fatal("existing tools acquired", err)
			}
		})
	}
}

func TestAppleCLTPreservesPartialOrBrokenExistingTools(t *testing.T) {
	for _, state := range []string{"directory", "xcode", "broken"} {
		t.Run(state, func(t *testing.T) {
			f := newAppleCLTFixture(t)
			switch state {
			case "directory":
				if err := os.Mkdir(f.d.clt, 0755); err != nil {
					t.Fatal(err)
				}
			case "xcode":
				f.selected = filepath.Join(filepath.Dir(f.d.clt), "missing-xcode")
			case "broken":
				f.install(f.d.clt)
				f.selected, f.unhealthy = f.d.clt, true
			}
			o, err := f.d.Observe(context.Background(), f.r, Receipt{})
			if err != nil || !o.Unknown || o.Pending == "" {
				t.Fatal(o, err)
			}
			if _, err := f.apply(); err == nil || len(f.commands) != 0 {
				t.Fatal("existing tools overwritten", err)
			}
		})
	}
}

func TestAppleCLTRecoveryUsesSavedNativePhaseEvidence(t *testing.T) {
	for _, failure := range []string{"before", "after-install", "lost-success"} {
		t.Run(failure, func(t *testing.T) {
			f := newAppleCLTFixture(t)
			if failure == "lost-success" {
				f.lose = "yes"
			} else {
				f.fail = failure
			}
			if _, err := f.apply(); err == nil {
				t.Fatal("failure hidden")
			}
			o, err := f.d.Observe(context.Background(), f.r, bootstrapReceipt("a"))
			if err != nil || o.ResourceResume == nil || o.CompletedOperation != "" {
				t.Fatal(o, err)
			}
			o, err = f.d.ResumeResource(context.Background(), f.r, Operation{Action: "install"}, bootstrapReceipt("a"))
			if err != nil || !o.Healthy || o.CompletedOperation != bootstrapReceipt("a").OperationID {
				t.Fatal(o, err)
			}
			want := []string{"install", "install"}
			if failure == "after-install" {
				want = []string{"install", "finish"}
			}
			if failure == "lost-success" {
				want = []string{"install"}
			}
			if !slices.Equal(f.modes, want) {
				t.Fatal(f.modes, want)
			}
		})
	}
}

func TestAppleCLTUnknownResultAndTamperedIntentFailClosed(t *testing.T) {
	for _, bad := range []string{"unknown", "command", "marker"} {
		t.Run(bad, func(t *testing.T) {
			f := newAppleCLTFixture(t)
			f.lose = "yes"
			if _, err := f.apply(); err == nil {
				t.Fatal("failure hidden")
			}
			intent, err := f.d.intent(bootstrapReceipt("a").OperationID)
			if err != nil {
				t.Fatal(err)
			}
			command := intent.Commands[0].Command
			if bad == "command" {
				intent.Commands[0].Command.Arguments[0] = "-S"
				if err := saveDocument(f.d.intentPath(intent.Operation), intent); err != nil {
					t.Fatal(err)
				}
			} else {
				hash, err := digest(command)
				if err != nil {
					t.Fatal(err)
				}
				record := nativeCommandRecord{Schema: 1, Command: hash}
				if bad == "marker" {
					code := 0
					record.Reply = &nativeReply{ExitCode: &code, Output: []byte("DOTFILES_CLT_SELECTED:" + intent.Operation + "\n")}
				}
				if err := saveDocument(filepath.Join(f.d.WorkerDirectory, "commands", command.Operation+".json"), record); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := f.d.ResumeResource(context.Background(), f.r, Operation{Action: "install"}, bootstrapReceipt("a")); err == nil || len(f.commands) != 1 {
				t.Fatal("unknown or changed evidence authorized mutation", err, f.modes)
			}
		})
	}
}

func TestAppleCLTRequiresHealthAndNativeReceipt(t *testing.T) {
	f := newAppleCLTFixture(t)
	f.unhealthy = true
	if o, err := f.apply(); err == nil || o.CompletedOperation != "" {
		t.Fatal("broken tools completed", o, err)
	}
	f.unhealthy = false
	f.installed = false
	f.receipt = ""
	if o, err := f.d.ResumeResource(context.Background(), f.r, Operation{Action: "install"}, bootstrapReceipt("a")); err == nil || o.CompletedOperation != "" {
		t.Fatal("missing native receipt completed", o, err)
	}
	if len(f.commands) != 1 {
		t.Fatal("verification replayed successful install")
	}
}

func TestAppleCLTAuthenticationFailureCanResumeUnsentCommand(t *testing.T) {
	f := newAppleCLTFixture(t)
	f.d.Authenticate = func(context.Context) error { return errors.New("auth declined") }
	if _, err := f.apply(); err == nil || len(f.commands) != 0 {
		t.Fatal("auth failure ignored", err)
	}
	f.d.Authenticate = nil
	o, err := f.d.ResumeResource(context.Background(), f.r, Operation{Action: "install"}, bootstrapReceipt("a"))
	if err != nil || !o.Healthy || len(f.commands) != 1 {
		t.Fatal(o, err, f.modes)
	}
}

func TestAppleCLTReceiptQueryFailureIsNotAbsence(t *testing.T) {
	f := newAppleCLTFixture(t)
	failure := errors.New("native receipt enumeration failed")
	f.d.Query = func(ctx context.Context, privileged bool, program string, input []byte, args ...string) ([]byte, error) {
		if program == "/usr/sbin/pkgutil" {
			return nil, failure
		}
		return f.query(ctx, privileged, program, input, args...)
	}
	if _, err := f.d.Observe(context.Background(), f.r, Receipt{}); !errors.Is(err, failure) {
		t.Fatal("receipt query failure became absence", err)
	}
	if _, err := f.apply(); err == nil || len(f.commands) != 0 {
		t.Fatal("receipt enumeration failure reached native mutation", err)
	}
}
