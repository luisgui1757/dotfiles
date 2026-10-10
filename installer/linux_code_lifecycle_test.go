//go:build darwin || linux

package installer

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func linuxCodeFixture(t *testing.T) (*linuxAPTFixture, Resource) {
	t.Helper()
	f := newLinuxAPTFixture(t)
	pin, err := defaultLinuxCodePin("amd64")
	if err != nil {
		t.Fatal(err)
	}
	payload := []byte("checked fixed Code package input")
	hash := sha256.Sum256(payload)
	pin.Size, pin.SHA256 = int64(len(payload)), hex.EncodeToString(hash[:])
	f.driver.Code, f.version = &pin, pin.Version
	f.driver.Packages["tool.vscode"] = "code"
	f.driver.Checks = map[string][]string{"tool.vscode": {f.driver.Location.Dpkg, "--verify", "code"}}
	f.driver.Client = &http.Client{Transport: windowsVendorTransport(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, ContentLength: int64(len(payload)), Body: io.NopCloser(bytes.NewReader(payload)), Header: http.Header{}, Request: r}, nil
	})}
	f.driver.Query = func(ctx context.Context, privileged bool, program string, input []byte, args ...string) ([]byte, error) {
		if len(args) == 4 && args[0] == "--compare-versions" {
			if args[2] != "gt" {
				t.Fatal("unexpected version comparison", args)
			}
			if args[1] > args[3] {
				return nil, nil
			}
			return exec.CommandContext(ctx, "/bin/sh", "-c", "exit 1").CombinedOutput()
		}
		if program == "/bin/sh" || slices.Equal(args, []string{"--verify", "code"}) {
			return nil, nil
		}
		return f.query(ctx, privileged, program, input, args...)
	}
	t.Cleanup(func() {
		files, _ := filepath.Glob(filepath.Join(f.driver.Directory, "operations", "*.json"))
		for _, file := range files {
			var intent linuxAPTIntent
			data, err := os.ReadFile(file)
			if err == nil && Decode(data, &intent) == nil && intent.Code != nil {
				if err := intent.Code.cleanup(); err != nil {
					t.Error("fixture input cleanup", err)
				}
			}
		}
	})
	return f, Resource{ID: "tool.vscode", Name: "Code", Action: "vscode", Scope: "user"}
}

func TestLinuxCodePinnedLifecycleKeepsNativeOwnership(t *testing.T) {
	f, r := linuxCodeFixture(t)
	ctx := context.Background()
	for _, action := range []string{"install", "update", "repair", "remove"} {
		f.approve()
		receipt := aptFixtureReceipt("code-" + action)
		got, err := f.driver.Apply(ctx, r, Operation{Action: action}, receipt)
		if err != nil || got.CompletedOperation != receipt.OperationID || got.Scope != "machine" || got.Present != (action != "remove") {
			t.Fatal(action, got, err)
		}
		intent, err := f.driver.intent(receipt.OperationID)
		if err != nil || intent.Code == nil || intent.Code.Pin != *f.driver.Code {
			t.Fatal("missing exact saved input", intent, err)
		}
		if _, err := os.Lstat(intent.Code.Directory); !errors.Is(err, os.ErrNotExist) {
			t.Fatal("completed command retained native temporary input", err)
		}
		last := intent.Commands[len(intent.Commands)-1].Command
		if !strings.Contains(strings.Join(last.Arguments, " "), "DEBCONF_DB_OVERRIDE=File{") {
			t.Fatal("Code maintainer scripts lack the filtered override", last)
		}
		if action != "remove" && last.Arguments[len(last.Arguments)-1] != f.driver.codePayload(intent.Code.Pin) {
			t.Fatal("APT did not consume its saved verified DEB", last)
		}
	}
}

func TestLinuxCodeExistingInstallationRemainsUnowned(t *testing.T) {
	f, r := linuxCodeFixture(t)
	f.installed["code:amd64"] = nativePackage{Name: "code:amd64", Source: "code", Version: "1.140.0-12345", Healthy: true}
	o, err := f.driver.Observe(context.Background(), r, Receipt{})
	if err != nil || !o.Present || !o.Healthy || o.CompletedOperation != "" || f.runs != 0 {
		t.Fatal("healthy existing native Code was not reused", o, err)
	}
	f.approve()
	if _, err := f.driver.Apply(context.Background(), r, Operation{Action: "install"}, aptFixtureReceipt("foreign-code")); err == nil || f.runs != 0 {
		t.Fatal("preexisting Code acquired install or removal authority", err)
	}
}

func TestLinuxCodeSavedWorkerRecoveryUsesOriginalInput(t *testing.T) {
	f, r := linuxCodeFixture(t)
	f.lostReply = true
	f.approve()
	receipt := aptFixtureReceipt("code-lost-reply")
	if _, err := f.driver.Apply(context.Background(), r, Operation{Action: "install"}, receipt); err == nil {
		t.Fatal("fixture did not lose the worker reply")
	}
	intent, err := f.driver.intent(receipt.OperationID)
	if err != nil || intent.Code == nil || intent.Code.inspect(false) != nil {
		t.Fatal("unfinished operation lost its input", intent, err)
	}
	oldPin, calls := intent.Code.Pin, f.runs
	pin := *f.driver.Code
	pin.Version = "1.142.0-12345"
	pin.URL = strings.Replace(pin.URL, oldPin.Version, pin.Version, 1)
	f.driver.Code = &pin
	f.approve()
	o, err := f.driver.ResumeResource(context.Background(), r, Operation{Action: "install"}, receipt)
	if err != nil || o.CompletedOperation != receipt.OperationID || f.runs != calls {
		t.Fatal("completed original worker was repeated or reinterpreted", o, err)
	}
	intent, err = f.driver.intent(receipt.OperationID)
	if err != nil || intent.Code.Pin != oldPin {
		t.Fatal("recovery replaced the authenticated input", intent, err)
	}
}

func TestLinuxCodeUnknownWorkerOutcomeRetainsInput(t *testing.T) {
	f, r := linuxCodeFixture(t)
	f.driver.Run = func(_ context.Context, command nativeCommand) ([]byte, error) {
		hash, err := digest(command)
		if err == nil {
			err = saveDocument(filepath.Join(f.driver.WorkerDirectory, "commands", command.Operation+".json"), nativeCommandRecord{Schema: 1, Command: hash})
		}
		return nil, errors.Join(errors.New("worker outcome unknown"), err)
	}
	f.approve()
	receipt := aptFixtureReceipt("code-unknown-worker")
	if _, err := f.driver.Apply(context.Background(), r, Operation{Action: "install"}, receipt); err == nil {
		t.Fatal("unknown worker outcome passed")
	}
	intent, err := f.driver.intent(receipt.OperationID)
	if err != nil || intent.Code == nil || intent.Code.inspect(false) != nil {
		t.Fatal("unknown operation lost its saved input", err)
	}
	f.driver.Run = func(context.Context, nativeCommand) ([]byte, error) {
		t.Fatal("unknown command was dispatched again")
		return nil, nil
	}
	if _, err := f.driver.ResumeResource(context.Background(), r, Operation{Action: "install"}, receipt); err == nil || !strings.Contains(err.Error(), "unfinished evidence") {
		t.Fatal("unknown worker did not remain blocked", err)
	}
}

func TestLinuxCodeForeignPublicationBlocksBeforeMutation(t *testing.T) {
	f, r := linuxCodeFixture(t)
	query := f.driver.Query
	f.driver.Query = func(ctx context.Context, privileged bool, program string, input []byte, args ...string) ([]byte, error) {
		if program == "/bin/sh" && len(args) == 2 && args[1] != linuxCodeHealth {
			return []byte("/usr/local/bin/code\n"), nil
		}
		return query(ctx, privileged, program, input, args...)
	}
	o, err := f.driver.Observe(context.Background(), r, Receipt{})
	if err != nil || o.ApplyBlocked == "" || f.runs != 0 {
		t.Fatal("personal Code collision was hidden from preview", o, err)
	}
	f.approve()
	if _, err := f.driver.Apply(context.Background(), r, Operation{Action: "install"}, aptFixtureReceipt("code-collision")); err == nil {
		t.Fatal("personal Code collision was overwritten")
	}
	if _, installed := f.installed["code:amd64"]; installed {
		t.Fatal("collision check happened after package mutation")
	}
}

func TestLinuxCodeTemporaryInputPreservesUnexpectedContents(t *testing.T) {
	pin, err := defaultLinuxCodePin("amd64")
	if err != nil {
		t.Fatal(err)
	}
	input, err := newLinuxCodeInput(pin, strings.Repeat("e", 64))
	if err != nil {
		t.Fatal(err)
	}
	unexpected := filepath.Join(input.Directory, "personal.txt")
	writeConfigFixture(t, unexpected, "preserve")
	if err := input.cleanup(); err == nil {
		t.Fatal("unknown input content was deleted")
	}
	if data, err := os.ReadFile(unexpected); err != nil || string(data) != "preserve" {
		t.Fatal("unknown input changed", err)
	}
	if err := os.Remove(unexpected); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"answer.dat-new", "answer.dat-old"} {
		if err := os.WriteFile(filepath.Join(input.Directory, name), []byte(linuxCodeDebconf), 0644); err != nil {
			t.Fatal(err)
		}
	}
	if err := input.cleanup(); err != nil {
		t.Fatal("known debconf rewrite files could not be cleaned", err)
	}
}

func TestLinuxCodeInputTamperCannotReachNativeWorker(t *testing.T) {
	f, r := linuxCodeFixture(t)
	f.fail = true
	f.approve()
	receipt := aptFixtureReceipt("code-corrupt-input")
	if _, err := f.driver.Apply(context.Background(), r, Operation{Action: "install"}, receipt); err == nil {
		t.Fatal("fixture did not fail")
	}
	intent, err := f.driver.intent(receipt.OperationID)
	if err != nil {
		t.Fatal(err)
	}
	path := f.driver.codePayload(intent.Code.Pin)
	if err := os.WriteFile(path, []byte("changed"), 0600); err != nil {
		t.Fatal(err)
	}
	f.fail = false
	calls := f.runs
	if _, err := f.driver.ResumeResource(context.Background(), r, Operation{Action: "install"}, receipt); err == nil || f.runs != calls {
		t.Fatal("tampered saved package reached the worker", err)
	}
}

func TestLinuxCodeDeferredPoolCollectionRetainsMaintainerProtection(t *testing.T) {
	for _, outside := range []bool{false, true} {
		t.Run(fmt.Sprint("outside=", outside), func(t *testing.T) {
			f, code := linuxCodeFixture(t)
			ctx := context.Background()
			f.driver.Run = func(ctx context.Context, command nativeCommand) ([]byte, error) {
				out, err := f.run(ctx, command)
				if pkg, ok := f.installed["first:amd64"]; ok {
					pkg.Dependencies = []string{"code:amd64", "shared:amd64"}
					f.installed[pkg.Name] = pkg
				}
				return out, err
			}
			for _, r := range []Resource{code, aptFixtureResource("first")} {
				f.approve()
				if _, err := f.driver.Apply(ctx, r, Operation{Action: "install"}, aptFixtureReceipt("code-pool-"+r.ID)); err != nil {
					t.Fatal(err)
				}
			}
			f.approve()
			o, err := f.driver.Remove(ctx, code, aptFixtureReceipt("code-pool-defer"))
			if err != nil || !o.Present || !o.RemovalDeferred {
				t.Fatal("needed Code root was not truthfully retained", o, err)
			}
			if outside {
				f.installed["outside:amd64"] = nativePackage{Name: "outside:amd64", Source: "outside", Version: "1", Healthy: true, Dependencies: []string{"code:amd64"}}
			}
			f.approve()
			receipt := aptFixtureReceipt("code-pool-collect")
			if _, err := f.driver.Remove(ctx, aptFixtureResource("first"), receipt); err != nil {
				t.Fatal(err)
			}
			intent, err := f.driver.intent(receipt.OperationID)
			if err != nil {
				t.Fatal(err)
			}
			_, present := f.installed["code:amd64"]
			if present != outside || (intent.Code != nil) == outside {
				t.Fatal("deferred Code collection lost external-consumer or maintainer protection", intent, f.installed)
			}
			if outside {
				state, err := f.driver.ledger()
				if err != nil || state.Pool["code:amd64"].CreatedBy == "" || !slices.Contains(intent.Preserved, "code:amd64") {
					t.Fatal("outside consumer silently erased retained ownership", state, err)
				}
			}
		})
	}
}

func TestLinuxCodeOldAPTIntentKeepsItsExactCommand(t *testing.T) {
	f := newLinuxAPTFixture(t)
	f.approve()
	receipt := aptFixtureReceipt("legacy-apt-without-code")
	if _, err := f.driver.Apply(context.Background(), aptFixtureResource("first"), Operation{Action: "install"}, receipt); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(f.driver.intentPath(receipt.OperationID))
	if err != nil || strings.Contains(string(data), `"code"`) {
		t.Fatal("ordinary legacy APT intent gained Code input", err)
	}
	intent, err := f.driver.intent(receipt.OperationID)
	if err != nil || intent.Code != nil {
		t.Fatal("old persisted shape no longer decodes", intent, err)
	}
	for _, saved := range intent.Commands {
		if f.driver.commandPrefixLength(intent, saved.Kind) != 4 || len(saved.Command.Environment) != 0 {
			t.Fatal("legacy command identity changed", saved)
		}
	}
}

func TestLinuxCodeChangedPublicCommandIsPreservedOnRemoval(t *testing.T) {
	f, r := linuxCodeFixture(t)
	f.approve()
	if _, err := f.driver.Apply(context.Background(), r, Operation{Action: "install"}, aptFixtureReceipt("code-link-install")); err != nil {
		t.Fatal(err)
	}
	query := f.driver.Query
	f.driver.Query = func(ctx context.Context, privileged bool, program string, input []byte, args ...string) ([]byte, error) {
		if slices.Equal(args, []string{"-c", linuxCodeRemoveLink}) {
			return nil, errors.New("link points to a personal executable")
		}
		return query(ctx, privileged, program, input, args...)
	}
	f.approve()
	calls := f.runs
	if _, err := f.driver.Remove(context.Background(), r, aptFixtureReceipt("code-link-remove")); err == nil || f.runs != calls {
		t.Fatal("changed public command reached destructive postrm", err)
	}
	if _, present := f.installed["code:amd64"]; !present {
		t.Fatal("native package was removed after its command changed")
	}
}

func TestLinuxCodeSuccessfulCompletionCleansOnlyVerifiedPackageInput(t *testing.T) {
	f, r := linuxCodeFixture(t)
	f.approve()
	if _, err := f.driver.Apply(context.Background(), r, Operation{Action: "install"}, aptFixtureReceipt("code-clean-input")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(f.driver.codePayload(*f.driver.Code)); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("completed Code operation retained its large DEB input", err)
	}
}

func TestLinuxCodeInputCleanupRecoveryPreservesUnexpectedFiles(t *testing.T) {
	for _, change := range []string{"extra", "payload", "empty"} {
		t.Run(change, func(t *testing.T) {
			f, r := linuxCodeFixture(t)
			f.lostReply = true
			f.approve()
			receipt := aptFixtureReceipt("cleanup-" + change)
			if _, err := f.driver.Apply(context.Background(), r, Operation{Action: "install"}, receipt); err == nil {
				t.Fatal("missing interrupted fixture")
			}
			file := f.driver.codePayload(*f.driver.Code)
			if err := f.driver.verifyCodePayload(*f.driver.Code); err != nil {
				t.Fatal("unknown outcome discarded input", err)
			}
			original, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			calls := f.runs
			switch change {
			case "extra":
				writeConfigFixture(t, filepath.Join(filepath.Dir(file), "personal"), "preserve me")
			case "payload":
				writeConfigFixture(t, file, "changed input")
			case "empty":
				if err := os.Remove(file); err != nil {
					t.Fatal(err)
				}
			}
			f.approve()
			got, err := f.driver.ResumeResource(context.Background(), r, Operation{Action: "install"}, receipt)
			if change != "empty" {
				if err == nil || f.runs != calls {
					t.Fatal("changed input was deleted or worker repeated", got, err)
				}
				if change == "extra" {
					extra := filepath.Join(filepath.Dir(file), "personal")
					if b, err := os.ReadFile(extra); err != nil || string(b) != "preserve me" {
						t.Fatal(b, err)
					}
					if err := os.Remove(extra); err != nil {
						t.Fatal(err)
					}
				} else {
					if b, err := os.ReadFile(file); err != nil || string(b) != "changed input" {
						t.Fatal(b, err)
					}
					if err := os.WriteFile(file, original, 0600); err != nil {
						t.Fatal(err)
					}
				}
				f.approve()
				got, err = f.driver.ResumeResource(context.Background(), r, Operation{Action: "install"}, receipt)
			}
			if err != nil || got.CompletedOperation != receipt.OperationID || f.runs != calls {
				t.Fatal("completed worker could not recover cleanup", got, err)
			}
			if _, err := os.Lstat(filepath.Dir(file)); !errors.Is(err, os.ErrNotExist) {
				t.Fatal("verified cache was retained", err)
			}
		})
	}
}

func TestLinuxCodeOldSavedCommandsRetainDowngradePolicy(t *testing.T) {
	f, r := linuxCodeFixture(t)
	f.approve()
	if _, err := f.driver.Apply(context.Background(), r, Operation{Action: "install"}, aptFixtureReceipt("before-old-update")); err != nil {
		t.Fatal(err)
	}
	f.lostReply = true
	f.approve()
	receipt := aptFixtureReceipt("old-update")
	if _, err := f.driver.Apply(context.Background(), r, Operation{Action: "update"}, receipt); err == nil {
		t.Fatal("missing lost reply")
	}
	intent, err := f.driver.intent(receipt.OperationID)
	if err != nil {
		t.Fatal(err)
	}
	// Reproduce the old saved shape and its actual recorded command identity.
	intent.Code.AllowDowngrade = false
	last := &intent.Commands[len(intent.Commands)-1]
	last.Command.Arguments = slices.DeleteFunc(last.Command.Arguments, func(s string) bool { return s == "--allow-downgrades" })
	hash, err := digest(last.Command)
	if err != nil {
		t.Fatal(err)
	}
	code := 0
	if err := saveDocument(filepath.Join(f.driver.WorkerDirectory, "commands", last.Command.Operation+".json"), nativeCommandRecord{Schema: 1, Command: hash, Reply: &nativeReply{ExitCode: &code}}); err != nil {
		t.Fatal(err)
	}
	if err := saveDocument(f.driver.intentPath(intent.Operation), intent); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(f.driver.intentPath(intent.Operation))
	if err != nil || bytes.Contains(data, []byte("allow_downgrade")) {
		t.Fatal("not an old serialized input", err)
	}
	calls := f.runs
	f.approve()
	got, err := f.driver.ResumeResource(context.Background(), r, Operation{Action: "update"}, receipt)
	if err != nil || got.CompletedOperation != receipt.OperationID || f.runs != calls {
		t.Fatal("old saved command changed or repeated", got, err)
	}
}

func TestLinuxCodeVersionDisclosurePropagatesNativeComparisonFailure(t *testing.T) {
	f, _ := linuxCodeFixture(t)
	f.driver.Query = func(context.Context, bool, string, []byte, ...string) ([]byte, error) {
		return nil, context.DeadlineExceeded
	}
	if _, err := f.driver.codeApplyDetail(context.Background(), "1:1.0-2"); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal("comparison timeout was treated as upgrade permission", err)
	}
}

func TestLinuxCodeDowngradeEvidenceIsExactAndOperationBound(t *testing.T) {
	f, r := linuxCodeFixture(t)
	f.approve()
	if _, err := f.driver.Apply(context.Background(), r, Operation{Action: "install"}, aptFixtureReceipt("downgrade-base")); err != nil {
		t.Fatal(err)
	}
	pkg := f.installed["code:amd64"]
	pkg.Version = "1:1.141.0-1"
	f.installed[pkg.Name] = pkg
	f.approve()
	f.lostReply = true
	receipt := aptFixtureReceipt("downgrade-native-history")
	if _, err := f.driver.Apply(context.Background(), r, Operation{Action: "update"}, receipt); err == nil {
		t.Fatal("missing lost reply")
	}
	intent, err := f.driver.intent(receipt.OperationID)
	if err != nil {
		t.Fatal(err)
	}
	last := intent.Commands[len(intent.Commands)-1]
	path := filepath.Join(f.driver.Directory, "logs", last.Command.Operation+".history")
	history, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	approved := linuxCodeApprovedDowngrade(intent)
	for _, field := range []string{approved, "other:amd64 (2, 1)", approved + ", other:amd64 (2, 1)", "code:amd64 (wrong, " + f.driver.Code.Version + ")"} {
		data := strings.Replace(string(history), "End-Date:", "Downgrade: "+field+"\nEnd-Date:", 1)
		if err := os.WriteFile(path, []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
		_, _, err := f.driver.evidence(intent)
		if (err == nil) != (field == approved) {
			t.Fatal("downgrade evidence exceeded saved exact permission", field, err)
		}
	}
	data := strings.Replace(string(history), "End-Date:", "Downgrade: "+approved+"\nEnd-Date:", 1)
	if err := os.WriteFile(path, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	intent.Code.AllowDowngrade = false
	if _, _, err := f.driver.evidence(intent); err == nil {
		t.Fatal("old command acquired new downgrade authority")
	}
	calls := f.runs
	f.approve()
	if got, err := f.driver.ResumeResource(context.Background(), r, Operation{Action: "update"}, receipt); err != nil || got.CompletedOperation != receipt.OperationID || f.runs != calls {
		t.Fatal("approved completed downgrade could not resume", got, err)
	}
}

func TestLinuxCodeInterruptedDowngradeResumesExactPinnedInput(t *testing.T) {
	f, r := linuxCodeFixture(t)
	f.approve()
	if _, err := f.driver.Apply(context.Background(), r, Operation{Action: "install"}, aptFixtureReceipt("partial-downgrade-base")); err != nil {
		t.Fatal(err)
	}
	pkg := f.installed["code:amd64"]
	pkg.Version = "1:1.141.0-1"
	f.installed[pkg.Name] = pkg
	f.partial = true
	f.approve()
	receipt := aptFixtureReceipt("partial-downgrade")
	if _, err := f.driver.Apply(context.Background(), r, Operation{Action: "update"}, receipt); err == nil {
		t.Fatal("configuration failure was hidden")
	}
	intent, err := f.driver.intent(receipt.OperationID)
	if err != nil {
		t.Fatal(err)
	}
	last := intent.Commands[len(intent.Commands)-1]
	path := filepath.Join(f.driver.Directory, "logs", last.Command.Operation+".history")
	history, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	data := strings.Replace(string(history), "End-Date:", "Downgrade: "+linuxCodeApprovedDowngrade(intent)+"\nEnd-Date:", 1)
	if err := os.WriteFile(path, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	if err := f.driver.verifyCodePayload(intent.Code.Pin); err != nil {
		t.Fatal("failed worker lost required input", err)
	}
	f.approve()
	got, err := f.driver.ResumeResource(context.Background(), r, Operation{Action: "update"}, receipt)
	if err != nil || !got.Healthy || got.CompletedOperation != receipt.OperationID || f.installed[pkg.Name].Version != intent.Code.Pin.Version {
		t.Fatal("partial downgrade could not finish its exact operation", got, err)
	}
	if _, err := os.Lstat(f.driver.codePayload(intent.Code.Pin)); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("successful recovery retained payload", err)
	}
}
