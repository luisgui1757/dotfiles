package installer

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"time"
)

// Sentinel (https://github.com/luisgui1757/sentinel) installs its own rules and
// skills. Setup fetches one pinned commit with git and runs the installer of
// that commit. The commit hash is the integrity check: git verifies each
// fetched object against it, and setup compares each checkout file with it.
const (
	sentinelRepository = "https://github.com/luisgui1757/sentinel.git"
	sentinelResource   = "integration.sentinel-installer"
)

// sentinelCommit is the only Sentinel pin. Renovate proposes a new value when
// the Sentinel main branch moves. Change only this line to move the pin.
const sentinelCommit = "b2f3cb7d44a83809f7d5d99d7d1dcea60c690593"

// sentinelHosts are the agents that the Sentinel installer supports.
var sentinelHosts = []string{"codex", "claude", "copilot", "kiro", "opencode"}

// sentinelCLIs are the catalog CLIs that make their agent a host.
var sentinelCLIs = map[string]string{"codex": "tool.codex", "claude": "tool.claude", "copilot": "tool.copilot", "kiro": "tool.kiro"}

// SentinelLocations holds the agent folders that the process environment sets.
// An empty field means the default folder under Home. KiroHome is the raw
// KIRO_HOME value. Installed names the agents whose command or application
// DiscoverSentinelAgents found.
type SentinelLocations struct {
	CodexHome, ConfigHome, ClaudeConfigDir, KiroHome, CopilotHome string
	Installed                                                     []string
}

// kiroHome resolves Kiro's global directory as Kiro documents KIRO_HOME: an
// absolute path other than a filesystem root, the home directory or one of its
// ancestors. Kiro ignores any other value and uses ~/.kiro, and so does setup.
func kiroHome(home, value string) string {
	fallback := filepath.Join(home, ".kiro")
	if value == "" || !filepath.IsAbs(value) || strings.ContainsAny(value, "\x00\r\n") {
		return fallback
	}
	value = filepath.Clean(value)
	if filepath.Dir(value) == value {
		return fallback
	}
	if relative, err := filepath.Rel(value, home); err == nil && (relative == "." || filepath.IsLocal(relative)) {
		return fallback
	}
	return value
}

// DiscoverSentinelAgents reports the agents whose command, or a Kiro macOS
// application (the IDE or the own bundle of Kiro CLI), exists. It runs nothing.
func DiscoverSentinelAgents(target Context, home string) []string {
	found := []string{}
	for _, agent := range []struct {
		name     string
		commands []string
	}{{"codex", []string{"codex"}}, {"claude", []string{"claude"}}, {"copilot", []string{"copilot"}}, {"kiro", []string{"kiro", "kiro-cli"}}, {"opencode", []string{"opencode"}}} {
		installed := false
		for _, command := range agent.commands {
			_, err := exec.LookPath(command)
			installed = installed || err == nil
		}
		if agent.name == "kiro" && target.OS == "darwin" {
			for _, name := range []string{"Kiro.app", "Kiro CLI.app"} {
				for _, app := range []string{filepath.Join("/Applications", name), filepath.Join(home, "Applications", name)} {
					info, err := os.Stat(app)
					installed = installed || err == nil && info.IsDir()
				}
			}
		}
		if installed {
			found = append(found, agent.name)
		}
	}
	return found
}

// sentinelHomes holds the five variables that move the agent folders of the
// Sentinel installer. Setup passes them to the installer unchanged. An empty
// value means that the variable is not set.
type sentinelHomes struct {
	CodexHome   string `json:"CODEX_HOME"`
	ClaudeHome  string `json:"CLAUDE_CONFIG_DIR"`
	CopilotHome string `json:"COPILOT_HOME"`
	ConfigHome  string `json:"XDG_CONFIG_HOME"`
	KiroHome    string `json:"KIRO_HOME"`
}

// sentinelRecord is the durable intent and completion of the last Sentinel
// operation. Apply and Remove write it before they change anything, and
// Complete after the installer succeeds. Hosts and Homes are the hosts and
// folders that setup installed for. Check, update and removal use them.
type sentinelRecord struct {
	Schema    int           `json:"schema"`
	Operation string        `json:"operation"`
	Action    string        `json:"action"`
	Commit    string        `json:"commit"`
	Hosts     []string      `json:"hosts"`
	Homes     sentinelHomes `json:"homes"`
	Complete  bool          `json:"complete"`
}

// SentinelDriver runs the installer of the pinned Sentinel commit. Sentinel
// decides which files it changes: its managed block in each rules file, and the
// skill folders that hold its .sentinel-owned marker. The driver also hands
// over the files of dotfiles 1.0 (see sentinel_handoff.go).
type SentinelDriver struct {
	Target    Context
	Home      string
	Locations SentinelLocations
	// State is the installer state folder. The driver keeps its checkout and
	// its record in State/sentinel.
	State string
	// Repository and Commit are the Sentinel source: sentinelRepository and
	// sentinelCommit, except in tests.
	Repository, Commit string
	// Git is the git executable. Shell is /bin/bash, or Windows PowerShell.
	Git, Shell string
	// Run changes the system under the controller mutation guard. Query only
	// reads, and it returns a nativeCommandError with the exit code.
	Run, Query func(context.Context, nativeCommand) ([]byte, error)
	// Selected is the bound resource closure. Earlier holds the saved receipts
	// of the dotfiles 1.0 Sentinel resources. ForSelection sets both.
	Selected []string
	Earlier  map[string]Receipt
}

func (d *SentinelDriver) directory() string { return filepath.Join(d.State, "sentinel") }
func (d *SentinelDriver) checkout() string  { return filepath.Join(d.directory(), "checkout") }
func (d *SentinelDriver) record() string    { return filepath.Join(d.directory(), "record.json") }

// readRecord returns the saved record, or nil when there is none.
func (d *SentinelDriver) readRecord() (*sentinelRecord, error) {
	data, err := readDocument(d.record())
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	var record sentinelRecord
	if err == nil {
		err = Decode(data, &record)
	}
	if err == nil && (record.Schema != 1 || !operationID.MatchString(record.Operation) || !gitCommit(record.Commit) ||
		!slices.Contains([]string{"install", "update", "repair", "remove"}, record.Action)) {
		err = errors.New("invalid identity")
	}
	for _, host := range record.Hosts {
		if !slices.Contains(sentinelHosts, host) {
			err = errors.Join(err, fmt.Errorf("unknown host %q", host))
		}
	}
	if err != nil {
		return nil, fmt.Errorf("cannot read the Sentinel record %s: %w", d.record(), err)
	}
	return &record, nil
}

// homes returns the recorded folders while a record exists, so that check,
// update and removal reach the files that setup wrote. Otherwise it returns
// the current folders.
func (d *SentinelDriver) homes(record *sentinelRecord) sentinelHomes {
	if record != nil {
		return record.Homes
	}
	return sentinelHomes{d.Locations.CodexHome, d.Locations.ClaudeConfigDir, d.Locations.CopilotHome, d.Locations.ConfigHome, d.Locations.KiroHome}
}

// hosts returns the agents that Sentinel installs for now. An agent counts when
// its command or application was found, when its configuration folder exists,
// or when this selection installs its CLI.
func (d *SentinelDriver) hosts(homes sentinelHomes) ([]string, error) {
	folder := func(value string, parts ...string) string {
		if value != "" {
			return value
		}
		return filepath.Join(append([]string{d.Home}, parts...)...)
	}
	roots := map[string][]string{
		"codex":    {folder(homes.CodexHome, ".codex")},
		"claude":   {folder(homes.ClaudeHome, ".claude")},
		"copilot":  {folder(homes.CopilotHome, ".copilot")},
		"kiro":     {kiroHome(d.Home, homes.KiroHome)},
		"opencode": {filepath.Join(folder(homes.ConfigHome, ".config"), "opencode")},
	}
	found := []string{}
	for _, host := range sentinelHosts {
		present := slices.Contains(d.Locations.Installed, host) || slices.Contains(d.Selected, sentinelCLIs[host])
		for _, root := range roots[host] {
			_, err := os.Lstat(root)
			if err != nil && !errors.Is(err, os.ErrNotExist) {
				return nil, fmt.Errorf("cannot check for %s at %s: %w", host, root, err)
			}
			present = present || err == nil
		}
		if present {
			found = append(found, host)
		}
	}
	return found, nil
}

// sentinelUnion merges host lists in the order of sentinelHosts.
func sentinelUnion(lists ...[]string) []string {
	result := []string{}
	for _, host := range sentinelHosts {
		if slices.ContainsFunc(lists, func(list []string) bool { return slices.Contains(list, host) }) {
			result = append(result, host)
		}
	}
	return result
}

// git runs inside directory without system or global Git configuration. Proxy
// variables such as HTTPS_PROXY stay in effect. A checkout keeps the bytes of
// the commit: no line-ending conversion and no replacement objects.
func (d *SentinelDriver) git(directory string, arguments ...string) nativeCommand {
	values := []string{"-c", "core.hooksPath=/dev/null", "-c", "core.fsmonitor=false", "-c", "credential.helper=", "-c", "init.defaultBranch=main",
		"-c", "advice.detachedHead=false", "-c", "core.autocrlf=false", "-c", "core.eol=lf", "-C", directory}
	return nativeCommand{Program: d.Git, Arguments: append(values, arguments...), Environment: []string{
		"GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null", "GIT_OPTIONAL_LOCKS=0", "GIT_TERMINAL_PROMPT=0", "GIT_NO_REPLACE_OBJECTS=1", "LC_ALL=C",
		"GIT_DIR=" + filepath.Join(directory, ".git"), "GIT_WORK_TREE=" + directory, "GIT_INDEX_FILE=" + filepath.Join(directory, ".git", "index")}}
}

// verify returns the commit of a checkout, or "" when there is none. The local
// configuration and the index of a checkout can hide a change, so verify does
// not trust them. Fsck proves each object against its hash. Each file is then
// hashed as raw bytes, without clean filters, and compared with the tree of
// the commit. Replacement objects are off. An extra file, a missing file or a
// file with other content or another executable bit is an error.
func (d *SentinelDriver) verify(ctx context.Context, run func(context.Context, nativeCommand) ([]byte, error), directory string) (string, error) {
	info, err := os.Lstat(directory)
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return "", fmt.Errorf("%s is not a plain directory", directory)
	}
	head, err := run(ctx, d.git(directory, "rev-parse", "--verify", "HEAD^{commit}"))
	commit := strings.TrimSpace(string(head))
	if err != nil || !gitCommit(commit) {
		return "", errors.Join(fmt.Errorf("cannot read the commit of %s", directory), err)
	}
	changed := func(err error) (string, error) {
		return commit, errors.Join(fmt.Errorf("%s differs from commit %s; setup fetches the pinned commit again", directory, commit), err)
	}
	if _, err := run(ctx, d.git(directory, "fsck", "--full", "--no-dangling", "--no-progress")); err != nil {
		return changed(err)
	}
	listing, err := run(ctx, d.git(directory, "ls-tree", "-r", "-z", "--full-tree", commit))
	if err != nil {
		return changed(err)
	}
	type entry struct{ mode, object string }
	expected := map[string]entry{}
	for _, line := range strings.Split(strings.TrimSuffix(string(listing), "\x00"), "\x00") {
		meta, path, ok := strings.Cut(line, "\t")
		fields := strings.Fields(meta)
		if !ok || len(fields) != 3 || fields[1] != "blob" || fields[0] != "100644" && fields[0] != "100755" || strings.Contains(path, "\n") {
			return changed(fmt.Errorf("unexpected tree entry %q", line))
		}
		expected[path] = entry{fields[0], fields[2]}
	}
	paths := []string{}
	err = filepath.WalkDir(directory, func(path string, entry os.DirEntry, err error) error {
		if err != nil || path == directory {
			return err
		}
		relative, err := filepath.Rel(directory, path)
		if err != nil {
			return err
		}
		relative = filepath.ToSlash(relative)
		if relative == ".git" && entry.IsDir() {
			return filepath.SkipDir
		}
		if entry.IsDir() {
			return nil
		}
		want, ok := expected[relative]
		if !ok || !entry.Type().IsRegular() {
			return fmt.Errorf("%s is not a file of the commit", relative)
		}
		if runtime.GOOS != "windows" {
			info, err := entry.Info()
			if err != nil {
				return err
			}
			if (info.Mode().Perm()&0111 != 0) != (want.mode == "100755") {
				return fmt.Errorf("%s has another executable bit", relative)
			}
		}
		paths = append(paths, relative)
		return nil
	})
	if err != nil {
		return changed(err)
	}
	if len(paths) != len(expected) {
		return changed(errors.New("a file of the commit is missing"))
	}
	command := d.git(directory, "hash-object", "--no-filters", "--stdin-paths")
	command.Input = []byte(strings.Join(paths, "\n") + "\n")
	hashes, err := run(ctx, command)
	objects := strings.Fields(string(hashes))
	if err != nil || len(objects) != len(paths) {
		return changed(err)
	}
	for i, path := range paths {
		if objects[i] != expected[path].object {
			return changed(fmt.Errorf("%s has other content", path))
		}
	}
	return commit, nil
}

func gitCommit(value string) bool {
	return len(value) == 40 && strings.Trim(value, "0123456789abcdef") == ""
}

// prepare returns a checkout of the pinned commit. It keeps a verified checkout
// of the pin. Otherwise it fetches only the pin into a new folder, verifies it
// and replaces the old checkout. It refuses any other commit.
func (d *SentinelDriver) prepare(ctx context.Context) (_ string, err error) {
	if commit, err := d.verify(ctx, d.Run, d.checkout()); err == nil && commit == d.Commit {
		return d.checkout(), nil
	}
	if err := os.MkdirAll(d.directory(), 0700); err != nil {
		return "", err
	}
	stage, err := os.MkdirTemp(d.directory(), ".fetch-")
	if err != nil {
		return "", err
	}
	defer func() {
		if err != nil {
			err = errors.Join(err, os.RemoveAll(stage))
		}
	}()
	for _, arguments := range [][]string{
		{"init", "--quiet"},
		{"fetch", "--quiet", "--depth", "1", "--no-tags", d.Repository, d.Commit},
		{"checkout", "--quiet", "--detach", "FETCH_HEAD"},
	} {
		if _, err := d.Run(ctx, d.git(stage, arguments...)); err != nil {
			return "", fmt.Errorf("cannot fetch Sentinel %s from %s. Check the network and the proxy settings (git uses HTTPS_PROXY), then try again: %w", d.Commit, d.Repository, err)
		}
	}
	commit, err := d.verify(ctx, d.Run, stage)
	if err != nil {
		return "", err
	}
	if commit != d.Commit {
		return "", fmt.Errorf("the fetched Sentinel commit %s is not the pinned commit %s; setup refuses it", commit, d.Commit)
	}
	if err := os.RemoveAll(d.checkout()); err != nil {
		return "", err
	}
	if err := os.Rename(stage, d.checkout()); err != nil {
		return "", err
	}
	return d.checkout(), nil
}

// installer is the command line of the Sentinel installer in a checkout. Mode
// is "", "check" or "remove".
func (d *SentinelDriver) installer(checkout string, hosts []string, homes sentinelHomes, mode string) (nativeCommand, error) {
	environment := []string{"HOME=" + d.Home, "CODEX_HOME=" + homes.CodexHome, "CLAUDE_CONFIG_DIR=" + homes.ClaudeHome,
		"COPILOT_HOME=" + homes.CopilotHome, "XDG_CONFIG_HOME=" + homes.ConfigHome, "KIRO_HOME=" + homes.KiroHome}
	if d.Target.OS == "windows" {
		if d.Shell == "" {
			return nativeCommand{}, errors.New("Sentinel needs Windows PowerShell, which setup did not find")
		}
		arguments := []string{"-NoLogo", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-File", filepath.Join(checkout, "tools", "install.ps1"),
			"-Global", "-Hosts", strings.Join(hosts, ","), "-Skills", "core,ui"}
		if mode != "" {
			arguments = append(arguments, "-"+strings.ToUpper(mode[:1])+mode[1:])
		}
		return nativeCommand{Program: d.Shell, Arguments: arguments, Environment: environment}, nil
	}
	arguments := []string{filepath.Join(checkout, "tools", "install"), "--global", "--hosts", strings.Join(hosts, ","), "--skills", "core,ui"}
	if mode != "" {
		arguments = append(arguments, "--"+mode)
	}
	return nativeCommand{Program: d.Shell, Arguments: arguments, Environment: environment}, nil
}

// querySentinel runs a read-only command and keeps its exit code.
func querySentinel(ctx context.Context, command nativeCommand) ([]byte, error) {
	if err := command.validate(); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	process := exec.CommandContext(ctx, command.Program, command.Arguments...)
	process.Env = append(os.Environ(), command.Environment...)
	process.Stdin = bytes.NewReader(command.Input)
	output := boundedCommandOutput{limit: 1 << 20}
	process.Stdout, process.Stderr = &output, &output
	err := process.Run()
	if err == nil {
		return output.data.Bytes(), nil
	}
	code := -1
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		code = exit.ExitCode()
	}
	return output.data.Bytes(), &nativeCommandError{Message: fmt.Sprintf("%s failed: %v%s", filepath.Base(command.Program), errors.Join(err, ctx.Err()), nativeDiagnostic(output.data.Bytes())), ExitCode: &code}
}

// Observe reports the installation that the record describes. Present means
// that a record exists, and CompletedOperation is its finished operation. The
// check runs only from a checkout that verify proves to be the pin: exit 0 is
// healthy, exit 1 means that installed files differ, and any other exit is an
// error. An interrupted operation of this receipt offers a resume.
func (d *SentinelDriver) Observe(ctx context.Context, r Resource, receipt Receipt) (Observation, error) {
	if r.ID != sentinelResource {
		return d.observeEarlier(r, receipt), nil
	}
	o := Observation{Provider: "sentinel", Identity: d.record(), Desired: d.Commit, Scope: "user"}
	record, err := d.readRecord()
	if err != nil {
		return Observation{Unknown: true, Provider: "sentinel", Scope: "user", Pending: err.Error()}, nil
	}
	homes := d.homes(record)
	hosts, err := d.hosts(homes)
	if err != nil {
		return o, err
	}
	if _, err := d.handoff(); err != nil {
		o.ApplyBlocked = err.Error()
	}
	if len(hosts) == 0 {
		o.ApplyBlocked = strings.TrimSpace(o.ApplyBlocked + " Sentinel found no coding agent. Install Codex, Claude Code, Copilot CLI, Kiro or OpenCode, then apply again.")
	}
	if _, err := d.installer(d.checkout(), hosts, homes, ""); err != nil {
		o.ApplyBlocked = strings.TrimSpace(o.ApplyBlocked + " " + err.Error() + ".")
	}
	if current := d.homes(nil); current != homes {
		o.Notes = append(o.Notes, "A Sentinel folder variable changed after the install. Setup keeps the recorded values "+sentinelHomesText(homes)+". To move Sentinel, untick it and apply, then select it again.")
	}
	if err := sentinelKiroHome(d.Home, homes, hosts); err != nil {
		o.ApplyBlocked = strings.TrimSpace(o.ApplyBlocked + " " + err.Error())
	}
	if record == nil {
		return o, nil
	}
	if o.Fingerprint, err = digest("sentinel-installation"); err != nil {
		return o, err
	}
	if !record.Complete {
		switch {
		case receipt.Status == "in-progress" && receipt.OperationID == record.Operation:
			token, err := digest(record)
			o.Present, o.Pending = true, "the Sentinel "+record.Action+" was interrupted; resume runs the Sentinel installer again"
			o.ResourceResume = &ResourceResume{Operation: record.Operation, Token: token}
			return o, err
		case record.Action != "remove" && receipt.Ownership != "created":
			// An abandoned first install never finished: apply installs again.
			return Observation{Provider: "sentinel", Identity: d.record(), Desired: d.Commit, Scope: "user", ApplyBlocked: o.ApplyBlocked, Notes: o.Notes}, nil
		}
		o.Present, o.HealthIssue = true, "an earlier Sentinel "+record.Action+" did not finish; apply runs the Sentinel installer again"
		return o, nil
	}
	o.Present, o.CompletedOperation = true, record.Operation
	commit, err := d.verify(ctx, d.Query, d.checkout())
	switch {
	case err != nil:
		o.HealthIssue = err.Error()
		return o, nil
	case commit == "":
		o.HealthIssue = "the Sentinel checkout is missing; repair fetches the pinned commit again"
		return o, nil
	case commit != d.Commit || record.Commit != d.Commit:
		o.HealthIssue = "Sentinel " + record.Commit + " is installed, and the pin is " + d.Commit + "; update installs the pinned commit"
		return o, nil
	case len(hosts) == 0:
		o.HealthIssue = "no coding agent to check"
		return o, nil
	}
	command, err := d.installer(d.checkout(), hosts, homes, "check")
	if err != nil {
		o.HealthIssue = err.Error()
		return o, nil
	}
	output, err := d.Query(ctx, command)
	for _, line := range strings.Split(string(output), "\n") {
		if strings.HasPrefix(line, "warning: ") {
			o.Notes = append(o.Notes, "Sentinel "+strings.TrimSpace(line))
		}
	}
	if err == nil {
		o.Healthy = true
		return o, nil
	}
	if code, ok := commandExitCode(err); ok && code == 1 {
		o.HealthIssue = sentinelFindings(output)
		return o, nil
	}
	return o, fmt.Errorf("Sentinel check failed: %w", err)
}

// sentinelKiroHome refuses a KIRO_HOME that Kiro ignores. The Sentinel
// installer gets KIRO_HOME unchanged and writes there, but Kiro then reads
// ~/.kiro (see kiroHome).
func sentinelKiroHome(home string, homes sentinelHomes, hosts []string) error {
	if !slices.Contains(hosts, "kiro") || homes.KiroHome == "" || kiroHome(home, homes.KiroHome) == filepath.Clean(homes.KiroHome) {
		return nil
	}
	return fmt.Errorf("Kiro ignores KIRO_HOME=%s and reads %s. Unset KIRO_HOME or set it to an absolute folder outside your home folder path, then apply again.", homes.KiroHome, kiroHome(home, homes.KiroHome))
}

func sentinelHomesText(homes sentinelHomes) string {
	values := []string{}
	for _, item := range []struct{ name, value string }{{"CODEX_HOME", homes.CodexHome}, {"CLAUDE_CONFIG_DIR", homes.ClaudeHome},
		{"COPILOT_HOME", homes.CopilotHome}, {"XDG_CONFIG_HOME", homes.ConfigHome}, {"KIRO_HOME", homes.KiroHome}} {
		if item.value == "" {
			item.value = "(not set)"
		}
		values = append(values, item.name+"="+item.value)
	}
	return strings.Join(values, ", ")
}

// sentinelFindings keeps the check lines that report a problem.
func sentinelFindings(output []byte) string {
	findings := []string{}
	for _, line := range strings.Split(string(output), "\n") {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "ok:") && !strings.HasPrefix(line, "warning: ") && len(findings) < 12 {
			findings = append(findings, line)
		}
	}
	if len(findings) == 0 {
		return "Sentinel check found differences"
	}
	return "Sentinel check: " + strings.Join(findings, "; ")
}

// Apply installs the pinned commit for the current hosts. It saves its intent
// first, so an interruption at any later step can resume. Then it fetches the
// pin, hands over the dotfiles 1.0 files and runs the installer. A failed fetch
// deletes nothing.
func (d *SentinelDriver) Apply(ctx context.Context, r Resource, op Operation, receipt Receipt) (Observation, error) {
	if r.ID != sentinelResource {
		return Observation{}, fmt.Errorf("%s only hands over earlier files; it cannot be installed", r.Name)
	}
	record, err := d.readRecord()
	if err != nil {
		return Observation{}, err
	}
	homes := d.homes(record)
	hosts, err := d.hosts(homes)
	if err != nil {
		return Observation{}, err
	}
	if len(hosts) == 0 {
		return Observation{}, errors.New("Sentinel found no coding agent")
	}
	if err := sentinelKiroHome(d.Home, homes, hosts); err != nil {
		return Observation{}, err
	}
	if _, err := d.handoff(); err != nil {
		return Observation{}, err
	}
	action := op.Action
	if action == "adopt" || action == "" {
		action = "install"
	}
	intent := sentinelRecord{Schema: 1, Operation: receipt.OperationID, Action: action, Commit: d.Commit, Hosts: hosts, Homes: homes}
	if record != nil {
		intent.Hosts = sentinelUnion(record.Hosts, hosts)
	}
	if err := saveDocument(d.record(), intent); err != nil {
		return Observation{}, err
	}
	checkout, err := d.prepare(ctx)
	if err != nil {
		return Observation{}, err
	}
	if err := d.handOver(); err != nil {
		return Observation{}, err
	}
	command, err := d.installer(checkout, hosts, homes, "")
	if err != nil {
		return Observation{}, err
	}
	if _, err := d.Run(ctx, command); err != nil {
		return Observation{}, fmt.Errorf("the Sentinel installer failed: %w", err)
	}
	intent.Complete = true
	if err := saveDocument(d.record(), intent); err != nil {
		return Observation{}, err
	}
	return d.Observe(ctx, r, receipt)
}

// Remove saves its intent, then runs the installer removal from a verified
// checkout of the pin for each recorded or current host. It uses the recorded
// folders. Then it deletes the checkout and the record.
func (d *SentinelDriver) Remove(ctx context.Context, r Resource, receipt Receipt) (Observation, error) {
	if r.ID != sentinelResource {
		return Observation{}, fmt.Errorf("%s has nothing to remove", r.Name)
	}
	record, err := d.readRecord()
	if err != nil {
		return Observation{}, err
	}
	homes := d.homes(record)
	current, err := d.hosts(homes)
	if err != nil {
		return Observation{}, err
	}
	intent := sentinelRecord{Schema: 1, Operation: receipt.OperationID, Action: "remove", Commit: d.Commit, Hosts: current, Homes: homes}
	if record != nil {
		intent.Hosts = sentinelUnion(record.Hosts, current)
	}
	if err := saveDocument(d.record(), intent); err != nil {
		return Observation{}, err
	}
	if len(intent.Hosts) != 0 {
		checkout, err := d.prepare(ctx)
		if err != nil {
			return Observation{}, err
		}
		command, err := d.installer(checkout, intent.Hosts, homes, "remove")
		if err != nil {
			return Observation{}, err
		}
		if _, err := d.Run(ctx, command); err != nil {
			return Observation{}, fmt.Errorf("the Sentinel installer could not remove its files: %w", err)
		}
	}
	if err := os.RemoveAll(d.directory()); err != nil {
		return Observation{}, err
	}
	return d.Observe(ctx, r, receipt)
}

// ResumeResource runs the saved operation again. The Sentinel installer is
// idempotent, so a second run finishes a partial install or removal.
func (d *SentinelDriver) ResumeResource(ctx context.Context, r Resource, op Operation, receipt Receipt) (Observation, error) {
	record, err := d.readRecord()
	if err != nil {
		return Observation{}, err
	}
	if record == nil || record.Complete || record.Operation != receipt.OperationID {
		return Observation{}, errors.New("the saved Sentinel operation differs from the approved recovery")
	}
	if record.Action == "remove" {
		return d.Remove(ctx, r, receipt)
	}
	return d.Apply(ctx, r, Operation{Action: record.Action}, receipt)
}

// sentinelGit is the git of the tool.git provider on each platform.
func sentinelGit(platform NativePlatform, archives *ArchiveDriver) (string, error) {
	switch platform.OS {
	case "windows":
		return archives.CommandPath("tool.git", "git")
	case "darwin":
		if platform.Homebrew != nil {
			return filepath.Join(platform.Homebrew.Prefix, "bin", "git"), nil
		}
		return "/opt/homebrew/bin/git", nil
	}
	return "/usr/bin/git", nil
}
