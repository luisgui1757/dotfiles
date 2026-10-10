package installer

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

type linuxAPTFixture struct {
	t              *testing.T
	driver         *LinuxAPTDriver
	installed      map[string]nativePackage
	configFiles    map[string]string
	version        string
	runs           int
	fail           bool
	partial        bool
	refusedRemoval bool
	lostReply      bool
}

func newLinuxAPTFixture(t *testing.T) *linuxAPTFixture {
	t.Helper()
	root, err := resolveConfigPath(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	program, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	f := &linuxAPTFixture{t: t, installed: map[string]nativePackage{}, version: "1.0"}
	d := &LinuxAPTDriver{Directory: filepath.Join(root, "apt"), WorkerDirectory: filepath.Join(root, "worker"),
		Location: LinuxAPTLocation{APTGet: program, Dpkg: program, DpkgQuery: program, APTMark: program, Sudo: program}, Packages: map[string]string{"tool.first": "first", "tool.second": "second"}, approval: &linuxAPTApproval{}}
	f.driver = d
	d.Query, d.Run = f.query, f.run
	return f
}

func (f *linuxAPTFixture) query(_ context.Context, privileged bool, program string, input []byte, args ...string) ([]byte, error) {
	if privileged || len(input) > 0 {
		return nil, errors.New("fixture received mutating inspection")
	}
	var out strings.Builder
	names := make([]string, 0, len(f.installed))
	for name := range f.installed {
		names = append(names, name)
	}
	slices.Sort(names)
	for _, name := range names {
		p := f.installed[name]
		want, status := "install", "installed"
		if p.Held {
			want = "hold"
		}
		if !p.Healthy {
			status = "unpacked"
		}
		switch program {
		case "apt-mark":
			if p.Automatic {
				fmt.Fprintln(&out, name)
			}
		case "dpkg-query":
			if len(args) == 2 && args[1] == "-f="+linuxAPTInventoryFormat {
				fmt.Fprintf(&out, "%s\t%s\t%s\t%s\tok\t%s\t\t%s\t\n", name, p.Version, want, status, p.Source, strings.Join(p.Dependencies, ", "))
			} else {
				fmt.Fprintf(&out, "%s\t%s\t%s\t%s\tok\n", name, p.Version, want, status)
			}
		default:
			return nil, errors.New("unexpected query")
		}
	}
	if program == "dpkg-query" {
		for name, version := range f.configFiles {
			if _, present := f.installed[name]; !present {
				fmt.Fprintf(&out, "%s\t%s\tdeinstall\tconfig-files\tok\tfixture\t\t\t\n", name, version)
			}
		}
	}
	return []byte(out.String()), nil
}

func (f *linuxAPTFixture) run(_ context.Context, command nativeCommand) ([]byte, error) {
	f.runs++
	code, output := 0, []byte{}
	if f.fail {
		code, output = 1, []byte("sudo: a password is required")
	} else if command.Arguments[len(command.Arguments)-1] == "update" {
		// Repository metadata refresh has no native package mutation.
	} else if slices.Contains(command.Arguments, "-W") {
		// Read-only verification completes an already absent removal root.
	} else if slices.Contains(command.Arguments, "--remove") {
		index := slices.Index(command.Arguments, "--remove")
		for _, name := range command.Arguments[index+1:] {
			if f.refusedRemoval && name == "shared:amd64" {
				f.installed["outside:amd64"] = nativePackage{Name: "outside:amd64", Source: "outside", Version: "1", Healthy: true, Dependencies: []string{name}}
				code, output = 1, []byte("dpkg: dependency problems prevent removal of shared")
				continue
			}
			if _, hasConffiles := f.configFiles[name]; hasConffiles {
				f.configFiles[name] = f.installed[name].Version
			}
			delete(f.installed, name)
		}
		f.refusedRemoval = false
	} else {
		root := command.Arguments[len(command.Arguments)-1] + ":amd64"
		added := []aptInstalledPackage{}
		log := ""
		for _, name := range []string{"shared:amd64", root} {
			old, existed := f.installed[name]
			base, _, _ := strings.Cut(name, ":")
			pkg := nativePackage{Name: name, Version: f.version, Source: base, Healthy: true, Automatic: name == "shared:amd64"}
			if name != "shared:amd64" {
				pkg.Dependencies = []string{"shared:amd64"}
			}
			if existed {
				pkg.Automatic = old.Automatic
				log += fmt.Sprintf("2026-10-10 00:00:00 upgrade %s %s %s\n", name, old.Version, pkg.Version)
			} else {
				added = append(added, aptInstalledPackage{name, pkg.Version, pkg.Automatic})
				previous := "<none>"
				if version := f.configFiles[name]; version != "" {
					previous = version
				}
				log += fmt.Sprintf("2026-10-10 00:00:00 install %s %s %s\n", name, previous, pkg.Version)
			}
			log += fmt.Sprintf("2026-10-10 00:00:00 status installed %s %s\n", name, pkg.Version)
			f.installed[name] = pkg
		}
		history := "Start-Date: 2026-10-10  00:00:00\nCommandline: apt-get -o Dotfiles::Operation=" + command.Operation + " install fixture\n"
		items := []string{}
		for _, pkg := range added {
			entry := pkg.Name + " (" + pkg.Version
			if pkg.Automatic {
				entry += ", automatic"
			}
			items = append(items, entry+")")
		}
		if len(items) > 0 {
			history += "Install: " + strings.Join(items, ", ") + "\n"
		}
		if f.partial {
			f.partial = false
			history += "Error: Sub-process dpkg returned an error code (1)\n"
			log = strings.ReplaceAll(log, "status installed "+root, "status unpacked "+root)
			pkg := f.installed[root]
			pkg.Healthy = false
			f.installed[root] = pkg
			code, output = 1, []byte("dpkg: fixture configuration failed")
		}
		history += "End-Date: 2026-10-10  00:00:01\n"
		for suffix, data := range map[string]string{".history": history, ".dpkg": log} {
			if err := os.WriteFile(filepath.Join(f.driver.Directory, "logs", command.Operation+suffix), []byte(data), 0600); err != nil {
				return nil, err
			}
		}
	}
	reply := nativeReply{ExitCode: &code, Output: output}
	if code != 0 {
		reply.Error = "exit status 1"
	}
	hash, err := digest(command)
	if err != nil {
		return nil, err
	}
	if err := saveDocument(filepath.Join(f.driver.WorkerDirectory, "commands", command.Operation+".json"), nativeCommandRecord{Schema: 1, Command: hash, Reply: &reply}); err != nil {
		return nil, err
	}
	if f.lostReply && command.Arguments[len(command.Arguments)-1] != "update" {
		f.lostReply = false
		return nil, errors.New("fixture lost reply")
	}
	if reply.Error != "" {
		return output, errors.New(reply.Error)
	}
	return output, nil
}

func (f *linuxAPTFixture) approve() {
	f.t.Helper()
	state, err := f.driver.ledger()
	if err != nil {
		f.t.Fatal(err)
	}
	installed, err := linuxAPTInventory(context.Background(), f.driver.Query)
	if err != nil {
		f.t.Fatal(err)
	}
	hash, err := f.driver.inventoryDigest(state, installed)
	if err != nil {
		f.t.Fatal(err)
	}
	f.driver.approval.expected = ""
	if err := f.driver.approveInventory(context.Background(), []string{hash}); err != nil {
		f.t.Fatal(err)
	}
}

func aptFixtureResource(name string) Resource {
	return Resource{ID: "tool." + name, Name: name, Action: "native"}
}

func aptFixtureReceipt(seed string) Receipt {
	id, _ := digest(seed)
	return Receipt{OperationID: id, Status: "in-progress"}
}

func TestLinuxAPTInventoryResolvesVirtualAndAlternativeConsumers(t *testing.T) {
	rows := "shared:amd64\t1.0\tinstall\tinstalled\tok\tshared\t\t\tvirtual (= 1.0)\n" +
		"alternative:amd64\t1.0\tinstall\tinstalled\tok\talternative\t\t\t\n" +
		"outside:amd64\t1.0\tinstall\tinstalled\tok\toutside\tvirtual (>= 1.0)\tshared:any | alternative\t\n"
	installed, err := parseLinuxAPTInventory([]byte(rows), []byte("shared\n"))
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(installed["outside:amd64"].Dependencies, []string{"alternative:amd64", "shared:amd64"}) {
		t.Fatal("virtual or alternative installed consumer lost", installed)
	}
	removal, protected, err := nativeRemovalCandidates(nil, []string{"shared:amd64"}, nil, installed)
	if err != nil || len(removal) != 0 || !slices.Equal(protected["shared:amd64"], []string{"outside:amd64"}) {
		t.Fatal("outside dependency was not protected", removal, protected, err)
	}
	for _, invalid := range []string{rows + rows, strings.Replace(rows, "shared:any", "bad;command", 1), rows + "truncated\n"} {
		if _, err := parseLinuxAPTInventory([]byte(invalid), nil); err == nil {
			t.Fatal("malformed package database accepted")
		}
	}
}

func TestLinuxAPTInstallUpdateRepairAndSharedRemoval(t *testing.T) {
	f := newLinuxAPTFixture(t)
	ctx := context.Background()
	f.installed["unrelated:amd64"] = nativePackage{Name: "unrelated:amd64", Version: "1.0", Source: "unrelated", Automatic: true, Healthy: true}
	for _, name := range []string{"first", "second"} {
		f.approve()
		receipt := aptFixtureReceipt("install-" + name)
		got, err := f.driver.Apply(ctx, aptFixtureResource(name), Operation{Action: "install"}, receipt)
		if err != nil || !got.Present || !got.Healthy || got.CompletedOperation != receipt.OperationID {
			t.Fatal("installation did not publish proved ownership", got, err)
		}
	}
	state, err := f.driver.ledger()
	if err != nil || len(state.Roots) != 2 || len(state.Pool) != 1 || state.Pool["shared:amd64"].CreatedBy != aptFixtureReceipt("install-first").OperationID {
		t.Fatal("shared pool acquired pre-existing packages", state, err)
	}
	f.version = "2.0"
	for _, action := range []string{"update", "repair"} {
		f.approve()
		got, err := f.driver.Apply(ctx, aptFixtureResource("first"), Operation{Action: action}, aptFixtureReceipt(action))
		if err != nil || !got.Healthy || f.installed["first:amd64"].Version != "2.0" {
			t.Fatal("native maintenance failed", got, err)
		}
	}
	f.approve()
	firstReceipt := aptFixtureReceipt("remove-first")
	got, err := f.driver.Remove(ctx, aptFixtureResource("first"), firstReceipt)
	if err != nil || got.Present || !slices.Equal(got.Preserved, []string{"apt:shared:amd64"}) {
		t.Fatal("shared dependency removed or omitted", got, err)
	}
	f.approve()
	got, err = f.driver.Remove(ctx, aptFixtureResource("second"), aptFixtureReceipt("remove-second"))
	if err != nil || got.Present || len(got.Preserved) != 0 || len(f.installed) != 1 || f.installed["unrelated:amd64"].Name == "" {
		t.Fatal("exact removal swept unrelated native packages", got, f.installed, err)
	}
	got, err = f.driver.Observe(ctx, aptFixtureResource("first"), firstReceipt)
	if err != nil || len(got.Preserved) != 0 {
		t.Fatal("old disclosure still reports a collected dependency", got, err)
	}
}

func TestLinuxAPTPreexistingRootCannotBeAcquiredOrMaintained(t *testing.T) {
	for _, action := range []string{"install", "update", "repair", "remove"} {
		t.Run(action, func(t *testing.T) {
			f := newLinuxAPTFixture(t)
			f.installed["first:amd64"] = nativePackage{Name: "first:amd64", Source: "first", Version: "1", Healthy: true}
			f.approve()
			if _, err := f.driver.Apply(context.Background(), aptFixtureResource("first"), Operation{Action: action}, aptFixtureReceipt(action)); err == nil || f.runs != 0 {
				t.Fatal("pre-existing native package changed", err, f.runs)
			}
			if _, err := os.Stat(f.driver.Directory); !errors.Is(err, os.ErrNotExist) {
				t.Fatal("refusal created package state", err)
			}
		})
	}
}

func TestLinuxAPTAuthenticatesOnlyAfterApprovalAndStopsOnRefusal(t *testing.T) {
	f := newLinuxAPTFixture(t)
	ctx := context.Background()
	prompts := 0
	f.driver.Authenticate = func(context.Context) error {
		prompts++
		return errors.New("authentication declined")
	}
	r, receipt := aptFixtureResource("first"), aptFixtureReceipt("authentication")
	if _, err := f.driver.Observe(ctx, r, Receipt{}); err != nil {
		t.Fatal(err)
	}
	if _, err := f.driver.Apply(ctx, r, Operation{Action: "install"}, receipt); err == nil || prompts != 0 || f.runs != 0 {
		t.Fatal("unapproved mutation reached authentication", err, prompts, f.runs)
	}
	f.approve()
	if _, err := f.driver.Apply(ctx, r, Operation{Action: "install"}, receipt); err == nil || !strings.Contains(err.Error(), "authentication declined") || prompts != 1 || f.runs != 0 {
		t.Fatal("declined authentication reached the native worker", err, prompts, f.runs)
	}
	if _, err := os.Stat(f.driver.intentPath(receipt.OperationID)); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("refused authentication recorded a command that never ran", err)
	}
}

func TestLinuxAPTLostReplyRecoversWithoutRepeatingMutation(t *testing.T) {
	f := newLinuxAPTFixture(t)
	f.approve()
	f.lostReply = true
	r, receipt := aptFixtureResource("first"), aptFixtureReceipt("lost-reply")
	if _, err := f.driver.Apply(context.Background(), r, Operation{Action: "install"}, receipt); err == nil {
		t.Fatal("fixture did not lose native reply")
	}
	observed, err := f.driver.Observe(context.Background(), r, receipt)
	if err != nil || observed.ResourceResume == nil || observed.CompletedOperation != "" {
		t.Fatal("pending operation cannot be reviewed", observed, err)
	}
	f.approve()
	got, err := f.driver.ResumeResource(context.Background(), r, Operation{Action: "install"}, receipt)
	if err != nil || got.CompletedOperation != receipt.OperationID || f.runs != 2 {
		t.Fatal("recovery repeated the successful native command", got, err, f.runs)
	}
}

func TestLinuxAPTElevationFailureIsExplicitAndRetryable(t *testing.T) {
	f := newLinuxAPTFixture(t)
	f.approve()
	f.fail = true
	r, receipt := aptFixtureResource("first"), aptFixtureReceipt("sudo-failure")
	if _, err := f.driver.Apply(context.Background(), r, Operation{Action: "install"}, receipt); err == nil || !strings.Contains(err.Error(), "foreground") {
		t.Fatal("missing administrator authorization was hidden", err)
	}
	f.fail = false
	f.approve()
	got, err := f.driver.ResumeResource(context.Background(), r, Operation{Action: "install"}, receipt)
	if err != nil || got.CompletedOperation != receipt.OperationID || f.runs != 3 {
		t.Fatal("authorized retry did not complete saved intent", got, err, f.runs)
	}
}

func TestLinuxAPTFailedPartialInstallationNeedsSuccessfulRecordedRecovery(t *testing.T) {
	f := newLinuxAPTFixture(t)
	f.approve()
	f.partial = true
	r, receipt := aptFixtureResource("first"), aptFixtureReceipt("partial-install")
	if _, err := f.driver.Apply(context.Background(), r, Operation{Action: "install"}, receipt); err == nil {
		t.Fatal("fixture should fail while configuring the root")
	}
	state, err := f.driver.ledger()
	if err != nil || len(state.Roots) != 0 || len(state.Pool) != 0 {
		t.Fatal("failed transaction acquired ownership", state, err)
	}
	f.approve()
	got, err := f.driver.ResumeResource(context.Background(), r, Operation{Action: "install"}, receipt)
	if err != nil || !got.Healthy || got.CompletedOperation != receipt.OperationID || f.runs != 4 {
		t.Fatal("saved failed installation could not be completed safely", got, err, f.runs)
	}
	state, err = f.driver.ledger()
	if err != nil || state.Roots[r.ID].CreatedBy != receipt.OperationID || state.Pool["shared:amd64"].CreatedBy != receipt.OperationID {
		t.Fatal("recovery lost original operation attribution", state, err)
	}
	f.approve()
	if got, err := f.driver.Remove(context.Background(), r, aptFixtureReceipt("remove-recovered")); err != nil || got.Present || len(f.installed) != 0 {
		t.Fatal("recovered ownership could not be removed", got, err)
	}
}

func TestLinuxAPTRecoveryPlannedPackagesAndForeignLogsDoNotAuthorizeRetry(t *testing.T) {
	for _, change := range []string{"planned-only", "foreign-operation", "wrong-version", "manually-promoted"} {
		t.Run(change, func(t *testing.T) {
			f := newLinuxAPTFixture(t)
			f.approve()
			f.partial = true
			r, receipt := aptFixtureResource("first"), aptFixtureReceipt("partial-install")
			if _, err := f.driver.Apply(context.Background(), r, Operation{Action: "install"}, receipt); err == nil {
				t.Fatal("fixture should fail")
			}
			intent, err := f.driver.intent(receipt.OperationID)
			if err != nil {
				t.Fatal(err)
			}
			base := filepath.Join(f.driver.Directory, "logs", intent.Commands[1].Command.Operation)
			switch change {
			case "planned-only":
				if err := os.WriteFile(base+".dpkg", nil, 0600); err != nil {
					t.Fatal(err)
				}
			case "foreign-operation":
				data, err := os.ReadFile(base + ".history")
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(base+".history", []byte(strings.ReplaceAll(string(data), intent.Commands[1].Command.Operation, strings.Repeat("b", 64))), 0600); err != nil {
					t.Fatal(err)
				}
			case "wrong-version", "manually-promoted":
				pkg := f.installed["first:amd64"]
				if change == "wrong-version" {
					pkg.Version = "9.0"
				} else {
					pkg.Automatic = true
				}
				f.installed[pkg.Name] = pkg
			}
			f.approve()
			if _, err := f.driver.ResumeResource(context.Background(), r, Operation{Action: "install"}, receipt); err == nil || f.runs != 2 {
				t.Fatal("unattributed package authorized recovery mutation", err, f.runs)
			}
		})
	}
}

func TestLinuxAPTPoolRetentionAndRelinquishment(t *testing.T) {
	for _, mode := range []string{"outside", "manual", "source", "held"} {
		t.Run(mode, func(t *testing.T) {
			f := newLinuxAPTFixture(t)
			f.approve()
			if _, err := f.driver.Apply(context.Background(), aptFixtureResource("first"), Operation{Action: "install"}, aptFixtureReceipt("initial")); err != nil {
				t.Fatal(err)
			}
			shared := f.installed["shared:amd64"]
			switch mode {
			case "outside":
				f.installed["outside:amd64"] = nativePackage{Name: "outside:amd64", Source: "outside", Version: "1", Healthy: true, Dependencies: []string{"shared:amd64"}}
			case "manual":
				shared.Automatic = false
			case "source":
				shared.Source = "replacement"
			case "held":
				shared.Held = true
			}
			f.installed[shared.Name] = shared
			f.approve()
			got, err := f.driver.Remove(context.Background(), aptFixtureResource("first"), aptFixtureReceipt("remove"))
			if err != nil || got.Present || !slices.Equal(got.Preserved, []string{"apt:shared:amd64"}) {
				t.Fatal("retained native package was not disclosed", got, err)
			}
			state, err := f.driver.ledger()
			_, owned := state.Pool[shared.Name]
			if err != nil || owned != (mode == "outside" || mode == "held") {
				t.Fatal("native ownership was not retained/relinquished correctly", state, err)
			}
		})
	}
}

func TestLinuxAPTStaleApprovalAndTamperedRecoveryCannotMutate(t *testing.T) {
	f := newLinuxAPTFixture(t)
	f.approve()
	f.installed["outside:amd64"] = nativePackage{Name: "outside:amd64", Source: "outside", Version: "1", Healthy: true}
	if _, err := f.driver.Apply(context.Background(), aptFixtureResource("first"), Operation{Action: "install"}, aptFixtureReceipt("stale")); err == nil || f.runs != 0 {
		t.Fatal("stale native approval executed", err)
	}
	f.approve()
	f.lostReply = true
	receipt := aptFixtureReceipt("tamper")
	if _, err := f.driver.Apply(context.Background(), aptFixtureResource("first"), Operation{Action: "install"}, receipt); err == nil {
		t.Fatal("fixture should lose reply")
	}
	intent, err := f.driver.intent(receipt.OperationID)
	if err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"program", "environment", "arguments", "operation", "input"} {
		t.Run(mode, func(t *testing.T) {
			changed := intent
			changed.Commands = slices.Clone(intent.Commands)
			command := &changed.Commands[0].Command
			switch mode {
			case "program":
				command.Program = filepath.Join(f.driver.Directory, "untrusted")
			case "environment":
				command.Environment = []string{"UNTRUSTED=1"}
			case "arguments":
				command.Arguments = append(slices.Clone(command.Arguments), "another")
			case "operation":
				command.Operation = strings.Repeat("b", 64)
			case "input":
				command.Input = []byte("unapproved")
			}
			if err := saveDocument(f.driver.intentPath(receipt.OperationID), changed); err != nil {
				t.Fatal(err)
			}
			if _, err := f.driver.intent(receipt.OperationID); err == nil {
				t.Fatal("saved intent changed execution authority")
			}
		})
	}
}

func TestLinuxAPTSavedRemovalRechecksManualPromotionBeforeDispatch(t *testing.T) {
	f := newLinuxAPTFixture(t)
	f.approve()
	r := aptFixtureResource("first")
	if _, err := f.driver.Apply(context.Background(), r, Operation{Action: "install"}, aptFixtureReceipt("initial")); err != nil {
		t.Fatal(err)
	}
	receipt := aptFixtureReceipt("saved-remove")
	intent := linuxAPTIntent{Schema: 1, Resource: r.ID, Operation: receipt.OperationID, Package: "first", Action: "remove", Before: f.installed, Remove: []string{"first:amd64", "shared:amd64"}}
	command, err := f.driver.command(intent, 0, "remove", nil, []string{"--remove", "first:amd64", "shared:amd64"})
	if err != nil {
		t.Fatal(err)
	}
	intent.Commands = []linuxAPTCommand{{Kind: "remove", Command: command}}
	if err := saveDocument(f.driver.intentPath(receipt.OperationID), intent); err != nil {
		t.Fatal(err)
	}
	pkg := f.installed["shared:amd64"]
	pkg.Automatic = false
	f.installed[pkg.Name] = pkg
	f.approve()
	if _, err := f.driver.ResumeResource(context.Background(), r, Operation{Action: "remove"}, receipt); err == nil || f.runs != 2 {
		t.Fatal("saved removal ignored a manual promotion before dispatch", err, f.runs)
	}
}

func TestLinuxAPTPartialRemovalCanCompleteWithNewlyProtectedDependency(t *testing.T) {
	f := newLinuxAPTFixture(t)
	f.approve()
	r := aptFixtureResource("first")
	if _, err := f.driver.Apply(context.Background(), r, Operation{Action: "install"}, aptFixtureReceipt("initial")); err != nil {
		t.Fatal(err)
	}
	f.approve()
	f.refusedRemoval = true
	receipt := aptFixtureReceipt("partial-removal")
	if _, err := f.driver.Remove(context.Background(), r, receipt); err == nil {
		t.Fatal("fixture must simulate a late outside consumer refusing dependency removal")
	}
	if _, present := f.installed["first:amd64"]; present {
		t.Fatal("fixture root was not removed before the refused dependency")
	}
	f.approve()
	got, err := f.driver.ResumeResource(context.Background(), r, Operation{Action: "remove"}, receipt)
	if err != nil || got.Present || got.CompletedOperation != receipt.OperationID || !slices.Equal(got.Preserved, []string{"apt:shared:amd64"}) {
		t.Fatal("saved partial removal could not retain its newly protected dependency", got, err)
	}
	state, err := f.driver.ledger()
	if err != nil || len(state.Roots) != 0 || state.Pool["shared:amd64"].Name == "" {
		t.Fatal("partial removal lost the preserved shared pool", state, err)
	}
}

func TestLinuxAPTConfigurationIsPureAndRootNeedsNoSudo(t *testing.T) {
	root := t.TempDir()
	program, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	location := &LinuxAPTLocation{APTGet: program, Dpkg: program, DpkgQuery: program, APTMark: program, root: true}
	catalog := &Catalog{Resources: []Resource{{ID: "tool.git", Bindings: map[string]Binding{"linux": {Provider: "apt", Package: "git"}}}}}
	d, err := configureLinuxAPT(location, catalog, &nativeSession{Directory: filepath.Join(root, "worker")})
	if err != nil || d.Packages["tool.git"] != "git" || !slices.Equal(d.Checks["tool.git"], []string{"/usr/bin/git", "--version"}) {
		t.Fatal("pure APT recipe configuration failed", d, err)
	}
	if files, err := os.ReadDir(root); err != nil || len(files) != 0 {
		t.Fatal("constructor initialized native state", files, err)
	}
	intent := linuxAPTIntent{Operation: aptFixtureReceipt("root").OperationID, Action: "install", Package: "git"}
	command, err := d.command(intent, 0, "install", nil, nil)
	if err != nil || command.Program != program || slices.Contains(command.Arguments, "-n") || !slices.Equal(command.Environment, []string{"LC_ALL=C", "DEBIAN_FRONTEND=noninteractive"}) {
		t.Fatal("root command tried sudo or inherited an environment", command, err)
	}
}

func TestLinuxAPTLibraryHealthRejectsChangedFilesDespiteSuccessfulDpkgExit(t *testing.T) {
	f := newLinuxAPTFixture(t)
	f.installed["first:amd64"] = nativePackage{Name: "first:amd64", Source: "first", Version: "1", Healthy: true}
	f.driver.Checks = map[string][]string{"tool.first": {f.driver.Location.Dpkg, "--verify", "first"}}
	for _, diagnostic := range []string{"", "??5?????? /usr/lib/libexample.so\n", "missing /usr/lib/libexample.so\n"} {
		f.driver.Query = func(ctx context.Context, privileged bool, program string, input []byte, args ...string) ([]byte, error) {
			if program == f.driver.Location.Dpkg {
				if privileged || len(input) != 0 || !slices.Equal(args, []string{"--verify", "first"}) {
					t.Fatal("library verification changed its read-only boundary", privileged, args)
				}
				return []byte(diagnostic), nil
			}
			return f.query(ctx, privileged, program, input, args...)
		}
		o, err := f.driver.Observe(context.Background(), Resource{ID: "tool.first", Name: "Fixture library", Action: "apt-library"}, Receipt{})
		if err != nil || !o.Present || o.Healthy != (diagnostic == "") || diagnostic != "" && !strings.Contains(o.HealthIssue, "changed package files") {
			t.Fatal("package database registration hid changed runtime files", o, err)
		}
	}
}
