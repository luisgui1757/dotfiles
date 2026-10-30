package installer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

type caskBoundaryFixture struct {
	d              *BrewDriver
	r              Resource
	receipt        Receipt
	spec           brewCaskSpec
	installed      map[string]nativePackage
	commands       []nativeCommand
	running        bool
	signatureError error
}

func caskMetadataFixture(s brewCaskSpec) map[string]any {
	return map[string]any{"token": "zen", "full_token": "zen", "tap": "homebrew/cask", "sha256": strings.Repeat("a", 64), "auto_updates": true, "depends_on": map[string]any{}, "artifacts": []any{
		map[string]any{"app": []string{filepath.Base(s.Application)}, "target": s.Application},
		map[string]any{"command_wrapper": []any{"zen", map[string]string{"executable": "$APPDIR/Zen.app/Contents/MacOS/zen"}}, "target": "$HOMEBREW_PREFIX/bin/zen"},
		map[string]any{"uninstall": []map[string]string{{"quit": s.Bundle}}},
		map[string]any{"zap": []map[string]any{{"trash": []string{"~/Library/Application Support/Zen"}}}},
	}}
}

func newCaskBoundaryFixture(t *testing.T) *caskBoundaryFixture {
	t.Helper()
	installed := map[string]nativePackage{"outside": {Name: "outside", Source: "outside", Version: "1", Healthy: true}}
	d, _, receipt := brewMutationInventoryFixture(t, installed, nil)
	root := filepath.Dir(d.Directory)
	spec := zenCaskSpec(filepath.Join(root, "home"), root)
	spec.Application = filepath.Join(root, "Applications", "Zen.app")
	f := &caskBoundaryFixture{d: d, r: Resource{ID: "app.zen", Name: "Zen", Action: "package", Scope: "machine"}, receipt: receipt, spec: spec, installed: installed}
	d.Packages = map[string]string{f.r.ID: "zen"}
	d.Casks = map[string]brewCaskSpec{f.r.ID: spec}
	query := d.Query
	d.Query = func(ctx context.Context, privileged bool, program string, input []byte, args ...string) ([]byte, error) {
		switch program {
		case "/usr/bin/codesign":
			if !slices.Equal(args[:len(args)-1], []string{"--verify", "--deep", "--strict=symlinks", "-R", "=" + spec.Requirement}) {
				t.Fatal(args)
			}
			return nil, f.signatureError
		case "/usr/bin/osascript":
			if f.running {
				return []byte("running\n"), nil
			}
			return []byte("not-running\n"), nil
		case "brew":
			if slices.Equal(args, []string{"info", "--json=v2", "--cask", "zen"}) {
				return json.Marshal(map[string]any{"casks": []any{caskMetadataFixture(spec)}})
			}
		}
		return query(ctx, privileged, program, input, args...)
	}
	d.Run = func(_ context.Context, command nativeCommand) ([]byte, error) {
		f.commands = append(f.commands, command)
		if !slices.Contains(command.Environment, "HOMEBREW_CASK_OPTS=") || !slices.Contains(command.Environment, "HOMEBREW_NO_SUDO=1") {
			t.Fatal("missing native cask isolation", command)
		}
		var output []byte
		if command.Arguments[0] == "uninstall" {
			if !slices.Equal(command.Arguments, []string{"uninstall", "--cask", "zen"}) {
				t.Fatal(command)
			}
			if err := os.RemoveAll(spec.Application); err != nil {
				t.Fatal(err)
			}
			delete(installed, "cask/zen")
		} else {
			if !slices.Contains(command.Arguments, "--no-binaries") || !slices.Contains(command.Arguments, "--require-sha") {
				t.Fatal(command)
			}
			f.publish(t)
			installed["cask/zen"] = nativePackage{Name: "cask/zen", Source: "zen", Version: "1", Healthy: true}
			output = []byte(fmt.Sprintf("dotfiles-operation-%s  zen was successfully installed!\n", command.Operation))
		}
		hash, err := digest(command)
		if err != nil {
			return nil, err
		}
		code := 0
		return output, saveDocument(filepath.Join(d.Directory, "worker", "commands", command.Operation+".json"), nativeCommandRecord{Schema: 1, Command: hash, Reply: &nativeReply{Output: output, ExitCode: &code}})
	}
	return f
}

func (f *caskBoundaryFixture) publish(t *testing.T) {
	t.Helper()
	if err := os.MkdirAll(f.spec.Application, 0755); err != nil {
		t.Fatal(err)
	}
	if err := saveDocument(filepath.Join(f.spec.Caskroom, ".metadata", "config.json"), map[string]any{"default": map[string]string{"appdir": filepath.Dir(f.spec.Application)}, "env": map[string]string{}, "explicit": map[string]string{"appdir": filepath.Dir(f.spec.Application)}}); err != nil {
		t.Fatal(err)
	}
	if err := saveDocument(filepath.Join(f.spec.Caskroom, ".metadata", "1", "timestamp", "Casks", "zen.json"), caskMetadataFixture(f.spec)); err != nil {
		t.Fatal(err)
	}
}

func (f *caskBoundaryFixture) controller() Controller {
	home := filepath.Dir(f.d.Directory)
	return Controller{Catalog: &Catalog{Schema: 1, Resources: []Resource{{ID: "zen", Name: "Zen", Capability: true, Requires: []string{f.r.ID}}, f.r}}, Context: Context{OS: "darwin", Arch: "arm64"}, Source: "cask-fixture", Home: home, StatePath: filepath.Join(home, "state.json"), Driver: f.d}
}

func TestNativeAppLifecyclePreservesVendorUpdatesAndPersonalProfiles(t *testing.T) {
	f := newCaskBoundaryFixture(t)
	c := f.controller()
	personal := filepath.Join(c.Home, "personal-profile")
	if err := os.WriteFile(personal, []byte("personal"), 0600); err != nil {
		t.Fatal(err)
	}
	personalBefore, err := os.Stat(personal)
	if err != nil {
		t.Fatal(err)
	}
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"zen"}})
	// A normal signed update may replace the complete bundle directory and bytes.
	if err := os.RemoveAll(f.spec.Application); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(f.spec.Application, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(f.spec.Application, "new-version"), []byte("2"), 0644); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(c.StatePath)
	if err != nil {
		t.Fatal(err)
	}
	result, err := c.Dispatch(context.Background(), Request{Schema: 1, Mode: "check"})
	if err != nil || result.Status != "ready" {
		t.Fatal(result, err)
	}
	after, err := os.ReadFile(c.StatePath)
	if err != nil || string(before) != string(after) {
		t.Fatal("Check wrote state", err)
	}
	dispatchApproved(t, c, Request{Schema: 1, Mode: "update"})
	if len(f.commands) != 1 {
		t.Fatal("Update invoked brew on a self-updating browser", f.commands)
	}
	preview, err := c.Dispatch(context.Background(), Request{Schema: 1, Mode: "apply", Selected: []string{}})
	if err != nil || operation(t, preview.Plan, "app.zen").Action != "retain" {
		t.Fatal(preview, err)
	}
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{}, RemoveShared: []string{"app.zen"}})
	if _, err := os.Stat(f.spec.Application); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("app remains", err)
	}
	data, err := os.ReadFile(personal)
	if err != nil || string(data) != "personal" {
		t.Fatal("personal data changed", err)
	}
	info, err := os.Stat(personal)
	if err != nil || info.Mode().Perm() != personalBefore.Mode().Perm() {
		t.Fatal("personal permissions changed", err)
	}
	if len(f.installed) != 1 || f.installed["outside"].Version != "1" {
		t.Fatal("unrelated packages changed", f.installed)
	}
}

func TestNativeAppReusesExistingBundlesWithoutAcquiringThem(t *testing.T) {
	for _, where := range []string{"system", "user", "cask", "cask-user"} {
		t.Run(where, func(t *testing.T) {
			f := newCaskBoundaryFixture(t)
			c := f.controller()
			path := f.spec.Application
			if where == "user" {
				path = f.spec.UserApplication
			}
			if err := os.MkdirAll(path, 0755); err != nil {
				t.Fatal(err)
			}
			if where == "cask" || where == "cask-user" {
				f.publish(t)
				if where == "cask-user" {
					if err := os.MkdirAll(filepath.Dir(f.spec.UserApplication), 0755); err != nil {
						t.Fatal(err)
					}
					if err := os.Rename(f.spec.Application, f.spec.UserApplication); err != nil {
						t.Fatal(err)
					}
					path = f.spec.UserApplication
				}
				f.installed["cask/zen"] = nativePackage{Name: "cask/zen", Source: "zen", Version: "1", Healthy: true}
				if err := os.MkdirAll(filepath.Dir(f.spec.Binary), 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(f.spec.Binary, []byte("outside command"), 0755); err != nil {
					t.Fatal(err)
				}
			}
			observed, err := f.d.Observe(context.Background(), f.r, Receipt{})
			if err != nil || !observed.Present || !observed.Healthy || observed.Pending != "" || observed.Identity != path {
				t.Fatal("authentic existing application was not reusable", observed, err)
			}
			dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"zen"}})
			dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{}, RemoveShared: []string{"app.zen"}})
			if len(f.commands) != 0 {
				t.Fatal("existing app was mutated", f.commands)
			}
			if _, err := os.Stat(path); err != nil {
				t.Fatal("existing app disappeared", err)
			}
			if _, err := c.Dispatch(context.Background(), Request{Schema: 1, Mode: "apply", Selected: []string{"zen"}, Adopt: []string{"app.zen"}}); err == nil {
				t.Fatal("native app was adoptable")
			}
		})
	}
}

func TestNativeAppMissingBundleRepairAndForeignReplacementProtection(t *testing.T) {
	for _, kind := range []string{"missing", "file", "link", "unknown-signature", "binary", "running"} {
		t.Run(kind, func(t *testing.T) {
			f := newCaskBoundaryFixture(t)
			c := f.controller()
			dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"zen"}})
			switch kind {
			case "missing", "file", "link":
				if err := os.RemoveAll(f.spec.Application); err != nil {
					t.Fatal(err)
				}
				if kind == "file" {
					if err := os.WriteFile(f.spec.Application, []byte("personal"), 0600); err != nil {
						t.Fatal(err)
					}
				}
				if kind == "link" {
					if err := os.Symlink(c.Home, f.spec.Application); err != nil {
						t.Fatal(err)
					}
				}
			case "unknown-signature":
				f.signatureError = context.DeadlineExceeded
			case "binary":
				if err := os.MkdirAll(filepath.Dir(f.spec.Binary), 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(f.spec.Binary, []byte("personal"), 0600); err != nil {
					t.Fatal(err)
				}
			case "running":
				f.running = true
			}
			if kind == "missing" {
				dispatchApproved(t, c, Request{Schema: 1, Mode: "repair"})
				if len(f.commands) != 2 || f.commands[1].Arguments[0] != "reinstall" {
					t.Fatal(f.commands)
				}
				return
			}
			request := Request{Schema: 1, Mode: "apply", Selected: []string{}, RemoveShared: []string{"app.zen"}}
			preview, err := c.Dispatch(context.Background(), request)
			if kind == "unknown-signature" {
				if err != nil || operation(t, preview.Plan, "app.zen").Action != "retain" || !operation(t, preview.Plan, "app.zen").Observed.Unknown {
					t.Fatal("signature inspection failure escaped its resource", preview, err)
				}
			}
			if kind == "running" {
				request.ExpectedPlan = preview.Plan.ID
				_, err = c.Dispatch(context.Background(), request)
				if err == nil {
					t.Fatal("running application was removed")
				}
			} else if err == nil && operation(t, preview.Plan, "app.zen").Action != "retain" {
				t.Fatal("foreign app not retained", preview)
			}
			if len(f.commands) != 1 {
				t.Fatal("protected app was mutated", f.commands)
			}
		})
	}
}

func TestNativeAppDoesNotPublishAFormulaCommandDirectory(t *testing.T) {
	catalog, err := DefaultCatalog()
	if err != nil {
		t.Fatal(err)
	}
	platform := NativePlatform{Context: Context{OS: "darwin", Arch: "arm64"}, Homebrew: &HomebrewLocation{Prefix: "/opt/homebrew"}}
	closure, err := catalog.Closure([]string{"zen"}, platform.Context)
	if err != nil {
		t.Fatal(err)
	}
	if paths := homebrewCommandDirectories(platform, catalog, closure); len(paths) != 0 {
		t.Fatal("GUI-only selection changed command discovery", paths)
	}
}

func TestNativeAppCommandInspectionErrorStaysLocal(t *testing.T) {
	f := newCaskBoundaryFixture(t)
	// A non-directory parent reports missing on Windows. An overlong component
	// produces a genuine inspection error on every supported filesystem instead.
	f.spec.Binary = filepath.Join(filepath.Dir(f.d.Directory), strings.Repeat("z", 256))
	f.d.Casks[f.r.ID] = f.spec
	if _, err := os.Lstat(f.spec.Binary); err == nil || errors.Is(err, os.ErrNotExist) {
		t.Fatal("fixture did not produce an inspection error", err)
	}
	preview, err := f.controller().Dispatch(context.Background(), Request{Schema: 1, Mode: "apply", Selected: []string{"zen"}})
	if err != nil {
		t.Fatal("command-path inspection failure aborted the plan", err)
	}
	op := operation(t, preview.Plan, "app.zen")
	if op.Action != "pending" || !op.Observed.Unknown || len(f.commands) != 0 {
		t.Fatal("uncertain native command path was not protected locally", op, f.commands)
	}
}

func TestNativeAppMissingUnownedBundleExplainsNativeRecovery(t *testing.T) {
	f := newCaskBoundaryFixture(t)
	f.publish(t)
	f.installed["cask/zen"] = nativePackage{Name: "cask/zen", Source: "zen", Version: "1", Healthy: true}
	if err := os.RemoveAll(f.spec.Application); err != nil {
		t.Fatal(err)
	}
	preview, err := f.controller().Dispatch(context.Background(), Request{Schema: 1, Mode: "apply", Selected: []string{"zen"}})
	if err != nil {
		t.Fatal(err)
	}
	op := operation(t, preview.Plan, "app.zen")
	if op.Action != "pending" || op.Observed.Adoptable || !strings.Contains(op.Reason, "HOMEBREW_NO_AUTOREMOVE=1 HOMEBREW_NO_INSTALL_CLEANUP=1 brew reinstall --cask zen") || !strings.Contains(op.Reason, "HOMEBREW_NO_AUTOREMOVE=1 brew uninstall --cask zen") || len(f.commands) != 0 {
		t.Fatal("missing personal app recovery must protect unrelated Homebrew dependencies", op, f.commands)
	}
}

func TestNativeAppRemovalProtectsPromotedFormulaPool(t *testing.T) {
	f := newCaskBoundaryFixture(t)
	c := f.controller()
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"zen"}})
	ledger, err := f.d.ledger()
	if err != nil {
		t.Fatal(err)
	}
	// A formerly automatic formula remains recorded after the owner promotes it.
	// Cask removal has no authority to update or relinquish this pool member.
	ledger.Pool["outside"] = brewOwnership{Name: "outside", Source: "outside", CreatedBy: f.receipt.OperationID}
	if err := saveDocument(filepath.Join(f.d.Directory, "packages.json"), ledger); err != nil {
		t.Fatal(err)
	}
	run := f.d.Run
	f.d.Run = func(ctx context.Context, command nativeCommand) ([]byte, error) {
		output, err := run(ctx, command)
		outside := f.installed["outside"]
		outside.Version = "unapproved-version"
		f.installed["outside"] = outside
		return output, err
	}
	request := Request{Schema: 1, Mode: "apply", Selected: []string{}, RemoveShared: []string{"app.zen"}}
	preview, err := c.Dispatch(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	request.ExpectedPlan = preview.Plan.ID
	if _, err := c.Dispatch(context.Background(), request); err == nil || !strings.Contains(err.Error(), "outside its recorded command") {
		t.Fatal("cask removal accepted an unrelated formula change", err)
	}
}

func TestNativeAppSchema3CompletionRemainsReadable(t *testing.T) {
	f := newCaskBoundaryFixture(t)
	if _, err := f.d.Apply(context.Background(), f.r, Operation{Action: "install"}, f.receipt); err != nil {
		t.Fatal(err)
	}
	intent, err := f.d.intent(f.receipt.OperationID)
	if err != nil || intent.Schema != 4 || !slices.Contains(intent.Commands[0].Environment, "HOMEBREW_NO_EMOJI=") {
		t.Fatal("new cask intent did not bind its output controls", intent, err)
	}
	// Serialize the earlier schema exactly, including the worker's command hash.
	intent.Schema = 3
	intent.Commands[0].Environment = slices.DeleteFunc(intent.Commands[0].Environment, func(v string) bool { return v == "HOMEBREW_NO_EMOJI=" })
	command := intent.Commands[0]
	reply, err := f.d.commandResult(f.commands[0])
	if err != nil {
		t.Fatal(err)
	}
	hash, err := digest(command)
	if err != nil {
		t.Fatal(err)
	}
	if err := saveDocument(filepath.Join(f.d.Directory, "worker", "commands", command.Operation+".json"), nativeCommandRecord{Schema: 1, Command: hash, Reply: &reply}); err != nil {
		t.Fatal(err)
	}
	if err := saveDocument(f.d.intentPath(intent.Operation), intent); err != nil {
		t.Fatal(err)
	}
	observed, err := f.d.Observe(context.Background(), f.r, f.receipt)
	if err != nil || !observed.Healthy || observed.CompletedOperation != intent.Operation {
		t.Fatal("historical cask completion was invalidated", observed, err)
	}
}

func TestNativeAppIntentRejectsRecipeChanges(t *testing.T) {
	for _, change := range []string{"force", "zap", "adopt", "appdir", "environment", "upgrade"} {
		t.Run(change, func(t *testing.T) {
			f := newCaskBoundaryFixture(t)
			if _, err := f.d.Apply(context.Background(), f.r, Operation{Action: "install"}, f.receipt); err != nil {
				t.Fatal(err)
			}
			intent, err := f.d.intent(f.receipt.OperationID)
			if err != nil {
				t.Fatal(err)
			}
			switch change {
			case "force", "zap", "adopt":
				intent.Commands[0].Arguments = append(intent.Commands[0].Arguments, "--"+change)
			case "appdir":
				intent.Commands[0].Arguments[2] = "--appdir=/outside"
			case "environment":
				intent.Commands[0].Environment = append(intent.Commands[0].Environment, "HOMEBREW_CASK_OPTS=--no-quarantine")
			case "upgrade":
				intent.Action = "update"
				intent.Commands[0].Arguments[0] = "upgrade"
			}
			if err := saveDocument(f.d.intentPath(f.receipt.OperationID), intent); err != nil {
				t.Fatal(err)
			}
			if _, err := f.d.intent(f.receipt.OperationID); err == nil {
				t.Fatal("saved data broadened cask authority", change)
			}
		})
	}
}

func TestNativeAppRecipeRejectsScriptsDependenciesAndRedirectedArtifacts(t *testing.T) {
	s := zenCaskSpec(t.TempDir(), t.TempDir())
	for _, change := range []string{"valid", "dependency", "no-check", "updater", "script", "app", "wrapper", "tap"} {
		t.Run(change, func(t *testing.T) {
			doc := caskMetadataFixture(s)
			switch change {
			case "dependency":
				doc["depends_on"] = map[string]any{"formula": []string{"other"}}
			case "no-check":
				doc["sha256"] = "no_check"
			case "updater":
				doc["auto_updates"] = false
			case "script":
				doc["artifacts"] = append(doc["artifacts"].([]any), map[string]any{"preflight": []string{"arbitrary"}})
			case "app":
				doc["artifacts"].([]any)[0].(map[string]any)["target"] = "/outside/Zen.app"
			case "wrapper":
				doc["artifacts"].([]any)[1].(map[string]any)["command_wrapper"] = []any{"zen", map[string]string{"content": "arbitrary"}}
			case "tap":
				doc["tap"] = "outside/tap"
			}
			data, err := json.Marshal(doc)
			if err != nil {
				t.Fatal(err)
			}
			err = validateCaskMetadata(data, s, "zen", false)
			if (err == nil) != (change == "valid") {
				t.Fatal(change, err)
			}
		})
	}
}

func TestNativeAppCatalogRetainsOldRecoveryWithoutSelectingIt(t *testing.T) {
	c, err := DefaultCatalog()
	if err != nil {
		t.Fatal(err)
	}
	for _, osname := range []string{"darwin", "linux"} {
		closure, err := c.Closure([]string{"zen"}, Context{OS: osname, Arch: "arm64"})
		if err != nil {
			t.Fatal(err)
		}
		if osname == "darwin" {
			if !slices.Contains(closure, "app.zen") || !slices.Contains(closure, "infra.homebrew") || slices.Contains(closure, "tool.zen") || slices.Contains(closure, "desktop.zen") {
				t.Fatal(closure)
			}
		} else if slices.Contains(closure, "app.zen") || !slices.Contains(closure, "tool.zen") || !slices.Contains(closure, "desktop.zen") {
			t.Fatal(closure)
		}
	}
}

func TestNativeAppRemovalDisclosureKeepsLegacyObservationCompatible(t *testing.T) {
	var old Observation
	if err := Decode([]byte(`{"present":true,"healthy":true,"provider":"homebrew-cask","identity":"/Applications/Zen.app","fingerprint":"native"}`), &old); err != nil {
		t.Fatal(err)
	}
	current := old
	current.RemovalDetail = "same developer at the same path"
	if old.RemovalDetail != "" || !sameArtifact(old, current) {
		t.Fatal("disclosure changed ownership")
	}
	f := newCaskBoundaryFixture(t)
	c := f.controller()
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"zen"}})
	p, err := c.Dispatch(context.Background(), Request{Schema: 1, Mode: "apply", Selected: []string{}, RemoveShared: []string{"app.zen"}})
	if err != nil || !strings.Contains(operation(t, p.Plan, "app.zen").Reason, "indistinguishable") {
		t.Fatal("removal omitted its limit", p, err)
	}
}

func TestNativeAppAcceptsHomebrewExpandedMetadataAndCompactReceipt(t *testing.T) {
	f := newCaskBoundaryFixture(t)
	doc := caskMetadataFixture(f.spec)
	wrapper := doc["artifacts"].([]any)[1].(map[string]any)
	wrapper["command_wrapper"] = []any{"zen", map[string]string{"executable": filepath.Join(f.spec.Application, "Contents", "MacOS", "zen")}}
	wrapper["target"] = f.spec.Binary
	data, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	t.Run("expanded-query", func(t *testing.T) {
		if err := validateCaskMetadata(data, f.spec, "zen", false); err != nil {
			t.Fatal("real brew info expansion rejected", err)
		}
	})
	f.publish(t)
	path := filepath.Join(f.spec.Caskroom, ".metadata", "1", "timestamp", "Casks", "zen.json")
	if err := os.WriteFile(path, []byte("{}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	receipt := map[string]any{"homebrew_version": "7.0.9", "uninstall_flight_blocks": false, "source": map[string]string{"tap": "homebrew/cask", "version": "1"}, "uninstall_artifacts": doc["artifacts"]}
	if err := saveDocument(filepath.Join(f.spec.Caskroom, ".metadata", "INSTALL_RECEIPT.json"), receipt); err != nil {
		t.Fatal(err)
	}
	t.Run("compact-installed-receipt", func(t *testing.T) {
		if err := f.d.caskInstalledRecipe(f.spec, "zen"); err != nil {
			t.Fatal("real compact cask receipt rejected", err)
		}
	})
}
