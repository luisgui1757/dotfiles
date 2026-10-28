package installer

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// Sentinel (https://github.com/luisgui1757/sentinel) installs its own rules and
// skills. Setup fetches one pinned commit with git and runs the installer of
// that commit. The commit hash is the integrity check: git verifies each
// fetched object against it. Renovate updates sentinelCommit.
const (
	sentinelRepository = "https://github.com/luisgui1757/sentinel.git"
	sentinelCommit     = "b58922538fb316c33c64a62593d61d214e0122ee"
	sentinelResource   = "integration.sentinel-installer"
)

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

// SentinelDriver runs the installer of the pinned Sentinel commit. Sentinel
// decides which files it changes: its managed block in each rules file, and the
// skill folders that hold its .sentinel-owned marker. The driver also hands
// over the skill copies of dotfiles 1.0 (see sentinel_handoff.go).
type SentinelDriver struct {
	Target    Context
	Home      string
	Locations SentinelLocations
	// State is the installer state folder. The driver keeps its checkout and
	// the hosts that it installed for in State/sentinel.
	State string
	// Git is the git executable. Shell is /bin/bash, or Windows PowerShell.
	Git, Shell string
	// Run changes the system under the controller mutation guard. Query only
	// reads, and it returns a nativeCommandError with the exit code.
	Run, Query func(context.Context, nativeCommand) ([]byte, error)
	// Selected is the bound resource closure. Earlier holds the saved receipts
	// of the dotfiles 1.0 Sentinel skills. ForSelection sets both.
	Selected []string
	Earlier  map[string]Receipt
}

func (d *SentinelDriver) directory() string { return filepath.Join(d.State, "sentinel") }
func (d *SentinelDriver) checkout() string  { return filepath.Join(d.directory(), "checkout") }
func (d *SentinelDriver) record() string    { return filepath.Join(d.directory(), "hosts.json") }

// hosts returns the agents that Sentinel installs for now. An agent counts when
// its command or application was found, when its configuration folder exists,
// or when this selection installs its CLI.
func (d *SentinelDriver) hosts() ([]string, error) {
	folder := func(value string, parts ...string) string {
		if value != "" {
			return value
		}
		return filepath.Join(append([]string{d.Home}, parts...)...)
	}
	roots := map[string][]string{
		"codex":    {folder(d.Locations.CodexHome, ".codex")},
		"claude":   {folder(d.Locations.ClaudeConfigDir, ".claude")},
		"copilot":  {folder(d.Locations.CopilotHome, ".copilot")},
		"kiro":     {filepath.Join(d.Home, ".kiro"), kiroHome(d.Home, d.Locations.KiroHome)},
		"opencode": {filepath.Join(folder(d.Locations.ConfigHome, ".config"), "opencode")},
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

// recordedHosts returns the hosts that an earlier apply installed for.
func (d *SentinelDriver) recordedHosts() ([]string, error) {
	data, err := readDocument(d.record())
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	var hosts []string
	if err == nil {
		err = Decode(data, &hosts)
	}
	for _, host := range hosts {
		if !slices.Contains(sentinelHosts, host) {
			err = errors.Join(err, fmt.Errorf("unknown host %q", host))
		}
	}
	if err != nil {
		return nil, fmt.Errorf("cannot read the Sentinel host record %s: %w", d.record(), err)
	}
	return hosts, nil
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
// variables such as HTTPS_PROXY stay in effect.
func (d *SentinelDriver) git(directory string, arguments ...string) nativeCommand {
	values := []string{"-c", "core.hooksPath=/dev/null", "-c", "core.fsmonitor=false", "-c", "credential.helper=", "-c", "init.defaultBranch=main",
		"-c", "advice.detachedHead=false", "-C", directory}
	return nativeCommand{Program: d.Git, Arguments: append(values, arguments...), Environment: []string{
		"GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null", "GIT_OPTIONAL_LOCKS=0", "GIT_TERMINAL_PROMPT=0", "LC_ALL=C",
		"GIT_DIR=" + filepath.Join(directory, ".git"), "GIT_WORK_TREE=" + directory, "GIT_INDEX_FILE=" + filepath.Join(directory, ".git", "index")}}
}

// verify returns the commit of a checkout. An empty commit with no error means
// that there is no checkout. A checkout with local changes is an error.
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
	status, err := run(ctx, d.git(directory, "status", "--porcelain=v1", "--untracked-files=all", "--ignored"))
	if err != nil || len(strings.TrimSpace(string(status))) != 0 {
		return commit, errors.Join(fmt.Errorf("%s has local changes; setup fetches the pinned commit again", directory), err)
	}
	return commit, nil
}

func gitCommit(value string) bool {
	return len(value) == 40 && strings.Trim(value, "0123456789abcdef") == ""
}

// prepare returns a checkout of the pinned commit. It fetches only that commit
// into a new folder, and refuses any other commit.
func (d *SentinelDriver) prepare(ctx context.Context) (_ string, err error) {
	if commit, err := d.verify(ctx, d.Run, d.checkout()); err == nil && commit == sentinelCommit {
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
		{"fetch", "--quiet", "--depth", "1", "--no-tags", sentinelRepository, sentinelCommit},
		{"checkout", "--quiet", "--detach", "FETCH_HEAD"},
	} {
		if _, err := d.Run(ctx, d.git(stage, arguments...)); err != nil {
			return "", fmt.Errorf("cannot fetch Sentinel %s from %s. Check the network and the proxy settings (git uses HTTPS_PROXY), then try again: %w", sentinelCommit, sentinelRepository, err)
		}
	}
	commit, err := d.verify(ctx, d.Run, stage)
	if err != nil {
		return "", err
	}
	if commit != sentinelCommit {
		return "", fmt.Errorf("the fetched Sentinel commit %s is not the pinned commit %s; setup refuses it", commit, sentinelCommit)
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
func (d *SentinelDriver) installer(checkout string, hosts []string, mode string) (nativeCommand, error) {
	if d.Target.OS == "windows" {
		if d.Shell == "" {
			return nativeCommand{}, errors.New("Sentinel needs Windows PowerShell, which setup did not find")
		}
		arguments := []string{"-NoLogo", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-File", filepath.Join(checkout, "tools", "install.ps1"),
			"-Global", "-Hosts", strings.Join(hosts, ","), "-Skills", "core,ui"}
		if mode != "" {
			arguments = append(arguments, "-"+strings.ToUpper(mode[:1])+mode[1:])
		}
		return nativeCommand{Program: d.Shell, Arguments: arguments, Environment: d.environment()}, nil
	}
	arguments := []string{filepath.Join(checkout, "tools", "install"), "--global", "--hosts", strings.Join(hosts, ","), "--skills", "core,ui"}
	if mode != "" {
		arguments = append(arguments, "--"+mode)
	}
	return nativeCommand{Program: d.Shell, Arguments: arguments, Environment: d.environment()}, nil
}

// The installer finds every agent folder through these variables. Setup sets
// each one, so a caller variable cannot move a target that setup checked.
func (d *SentinelDriver) environment() []string {
	return []string{"HOME=" + d.Home, "CODEX_HOME=" + d.Locations.CodexHome, "CLAUDE_CONFIG_DIR=" + d.Locations.ClaudeConfigDir,
		"COPILOT_HOME=" + d.Locations.CopilotHome, "XDG_CONFIG_HOME=" + d.Locations.ConfigHome}
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

// Observe reports the cached checkout and the result of the installer check:
// exit 0 is healthy, exit 1 means that installed files differ, and any other
// exit is an error. Desired is the pin, so Update applies a new pin.
func (d *SentinelDriver) Observe(ctx context.Context, r Resource, receipt Receipt) (Observation, error) {
	if r.ID != sentinelResource {
		return d.observeEarlier(r, receipt), nil
	}
	o := Observation{Provider: "sentinel", Identity: d.checkout(), Desired: sentinelCommit, Scope: "user"}
	hosts, err := d.hosts()
	if err != nil {
		return o, err
	}
	if _, err := d.handoff(); err != nil {
		o.ApplyBlocked = err.Error()
	}
	if len(hosts) == 0 {
		o.ApplyBlocked = strings.TrimSpace(o.ApplyBlocked + " Sentinel found no coding agent. Install Codex, Claude Code, Copilot CLI, Kiro or OpenCode, then apply again.")
	}
	if _, err := d.installer(d.checkout(), hosts, ""); err != nil {
		o.ApplyBlocked = strings.TrimSpace(o.ApplyBlocked + " " + err.Error() + ".")
	}
	if slices.Contains(hosts, "kiro") && kiroHome(d.Home, d.Locations.KiroHome) != filepath.Join(d.Home, ".kiro") {
		o.Notes = append(o.Notes, "Kiro reads "+kiroHome(d.Home, d.Locations.KiroHome)+", but Sentinel writes the Kiro rules and skills to "+filepath.Join(d.Home, ".kiro")+".")
	}
	commit, verifyErr := d.verify(ctx, d.Query, d.checkout())
	if commit == "" && verifyErr == nil {
		return o, nil
	}
	o.Present, o.Version = true, commit
	if o.Fingerprint, err = digest(commit); err != nil {
		return o, err
	}
	if verifyErr != nil {
		o.HealthIssue = verifyErr.Error()
		return o, nil
	}
	if len(hosts) == 0 {
		o.HealthIssue = "no coding agent to check"
		return o, nil
	}
	command, err := d.installer(d.checkout(), hosts, "check")
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
	if code, ok := legacyExitCode(err); ok && code == 1 {
		o.HealthIssue = sentinelFindings(output)
		return o, nil
	}
	return o, fmt.Errorf("Sentinel check failed: %w", err)
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

// Apply fetches the pinned commit, hands over the dotfiles 1.0 skill copies and
// runs the installer for the current hosts. A failed fetch deletes nothing. The
// host record comes first, so a later removal also reaches an interrupted run.
func (d *SentinelDriver) Apply(ctx context.Context, r Resource, op Operation, receipt Receipt) (Observation, error) {
	if r.ID != sentinelResource {
		return Observation{}, fmt.Errorf("%s only hands over earlier files; it cannot be installed", r.Name)
	}
	hosts, err := d.hosts()
	if err != nil {
		return Observation{}, err
	}
	if len(hosts) == 0 {
		return Observation{}, errors.New("Sentinel found no coding agent")
	}
	if _, err := d.handoff(); err != nil {
		return Observation{}, err
	}
	checkout, err := d.prepare(ctx)
	if err != nil {
		return Observation{}, err
	}
	if err := d.handOver(); err != nil {
		return Observation{}, err
	}
	recorded, err := d.recordedHosts()
	if err != nil {
		return Observation{}, err
	}
	if err := saveDocument(d.record(), sentinelUnion(recorded, hosts)); err != nil {
		return Observation{}, err
	}
	command, err := d.installer(checkout, hosts, "")
	if err != nil {
		return Observation{}, err
	}
	if _, err := d.Run(ctx, command); err != nil {
		return Observation{}, fmt.Errorf("the Sentinel installer failed: %w", err)
	}
	o, err := d.Observe(ctx, r, receipt)
	o.CompletedOperation = receipt.OperationID
	return o, err
}

// Remove runs the installer removal for each host that setup installed for or
// finds now. Then it deletes the checkout and the host record.
func (d *SentinelDriver) Remove(ctx context.Context, r Resource, receipt Receipt) (Observation, error) {
	if r.ID != sentinelResource {
		return Observation{}, fmt.Errorf("%s has nothing to remove", r.Name)
	}
	recorded, err := d.recordedHosts()
	if err != nil {
		return Observation{}, err
	}
	current, err := d.hosts()
	if err != nil {
		return Observation{}, err
	}
	if hosts := sentinelUnion(recorded, current); len(hosts) != 0 {
		checkout := d.checkout()
		if commit, err := d.verify(ctx, d.Run, checkout); err != nil || commit == "" {
			if checkout, err = d.prepare(ctx); err != nil {
				return Observation{}, err
			}
		}
		command, err := d.installer(checkout, hosts, "remove")
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
