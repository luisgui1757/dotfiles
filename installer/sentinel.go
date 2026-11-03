package installer

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"time"
)

// Sentinel (https://github.com/luisgui1757/sentinel) installs its own rules and
// skills. Install and update take the latest Sentinel release: GitHub must
// report it as published and immutable, and git gives the commit of its tag.
// Setup fetches that commit with git and runs the installer of that commit.
// The commit hash is the integrity check: git verifies each fetched object
// against it, and setup compares each checkout file with it. Check, repair,
// removal and resume use the commit in the record.
const (
	sentinelRepository = "https://github.com/luisgui1757/sentinel.git"
	sentinelReleases   = "https://api.github.com/repos/luisgui1757/sentinel/releases/latest"
	sentinelResource   = "integration.sentinel-installer"
)

// sentinelFloorTag and sentinelCommit are the oldest Sentinel release that setup
// installs and the commit of its tag. The tests install this release.
const (
	sentinelFloorTag = "v0.3.0"
	sentinelCommit   = "c8beb70d6e568e0e8fe7ff6ee02675585958afbf"
)

// sentinelFixedRelease makes the latest release the floor. Only the build tag
// of the native tests sets it.
var sentinelFixedRelease = false

var sentinelTag = regexp.MustCompile(`^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`)

// sentinelRelease is a release tag and the commit of that tag.
type sentinelRelease struct{ Tag, Commit string }

// sentinelOlder reports whether release tag a is older than b.
func sentinelOlder(a, b string) bool {
	x, y := sentinelTag.FindStringSubmatch(a), sentinelTag.FindStringSubmatch(b)
	for i := 1; i < 4 && x != nil && y != nil; i++ {
		p, _ := strconv.Atoi(x[i])
		q, _ := strconv.Atoi(y[i])
		if p != q {
			return p < q
		}
	}
	return false
}

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
	Tag       string        `json:"tag,omitempty"`
	Hosts     []string      `json:"hosts"`
	Homes     sentinelHomes `json:"homes"`
	Complete  bool          `json:"complete"`
}

// SentinelDriver runs the installer of a Sentinel release. Sentinel decides
// which files it changes: its managed block in each rules file, the skill
// folders that hold its .sentinel-owned marker, and the Claude Code output
// style output-styles/sentinel.md.
type SentinelDriver struct {
	Target    Context
	Home      string
	Locations SentinelLocations
	// State is the installer state folder. The driver keeps its checkout and
	// its record in State/sentinel.
	State string
	// Repository, Releases and Floor are the Sentinel source, the latest-release
	// API and the oldest release that setup installs.
	Repository, Releases string
	Floor                sentinelRelease
	// Latest returns the latest release. Nil asks Releases.
	Latest func(context.Context) (sentinelRelease, error)
	// Git is the git executable. Shell is /bin/bash, or Windows PowerShell.
	Git, Shell string
	// Run changes the system under the controller mutation guard. Query only
	// reads, and it returns a nativeCommandError with the exit code.
	Run, Query func(context.Context, nativeCommand) ([]byte, error)
	// Selected is the bound resource closure. ForSelection sets it.
	Selected []string

	// looked, latest and latestFailure keep the one lookup of a setup run.
	looked        bool
	latest        sentinelRelease
	latestFailure error
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
	if err == nil && (record.Schema != 1 || !operationID.MatchString(record.Operation) || !gitCommit(record.Commit) || record.Tag != "" && !sentinelTag.MatchString(record.Tag) ||
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

// resolve returns the latest release, at most once for each driver, so that
// approval and apply see the same release. It refuses a release that is older
// than the floor.
func (d *SentinelDriver) resolve(ctx context.Context) (sentinelRelease, error) {
	if !d.looked {
		d.looked = true
		switch {
		case d.Latest != nil:
			d.latest, d.latestFailure = d.Latest(ctx)
		case sentinelFixedRelease:
			d.latest = d.Floor
		default:
			d.latest, d.latestFailure = d.githubRelease(ctx)
		}
		if d.latestFailure == nil && (!sentinelTag.MatchString(d.latest.Tag) || !gitCommit(d.latest.Commit)) {
			d.latestFailure = fmt.Errorf("GitHub gave the Sentinel release %q with the commit %q", d.latest.Tag, d.latest.Commit)
		}
		if d.latestFailure == nil && sentinelOlder(d.latest.Tag, d.Floor.Tag) {
			d.latestFailure = fmt.Errorf("the latest Sentinel release %s is older than %s, the oldest release that setup installs", d.latest.Tag, d.Floor.Tag)
		}
	}
	return d.latest, d.latestFailure
}

// githubRelease asks GitHub for the latest release, which must be published,
// immutable and named vX.Y.Z, and then for the commit of its tag.
func (d *SentinelDriver) githubRelease(ctx context.Context) (sentinelRelease, error) {
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	client := &http.Client{}
	get := func(url, accept string) ([]byte, error) {
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, err
		}
		request.Header.Set("Accept", accept)
		request.Header.Set("X-GitHub-Api-Version", "2022-11-28")
		request.Header.Set("User-Agent", "dotfiles-installer")
		response, err := client.Do(request)
		if err != nil {
			return nil, err
		}
		defer response.Body.Close()
		body, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
		if err == nil && response.StatusCode != http.StatusOK {
			err = fmt.Errorf("%s answered %s", url, response.Status)
		}
		return body, err
	}
	var release struct {
		Tag                          string `json:"tag_name"`
		Draft, Prerelease, Immutable bool
	}
	data, err := get(d.Releases, "application/vnd.github+json")
	if err == nil {
		err = json.Unmarshal(data, &release)
	}
	if err == nil && (release.Draft || release.Prerelease || !release.Immutable || !sentinelTag.MatchString(release.Tag)) {
		err = fmt.Errorf("the latest Sentinel release %q is not a published and immutable vX.Y.Z release", release.Tag)
	}
	if err != nil {
		return sentinelRelease{}, err
	}
	data, err = get(strings.TrimSuffix(d.Releases, "/releases/latest")+"/commits/refs/tags/"+release.Tag, "application/vnd.github.sha")
	return sentinelRelease{release.Tag, strings.TrimSpace(string(data))}, err
}

// installed returns the release in a record. A record from before setup kept
// the tag names only the commit, and the floor commit is the floor release.
func (d *SentinelDriver) installed(record *sentinelRecord) sentinelRelease {
	if record.Tag == "" && record.Commit == d.Floor.Commit {
		return d.Floor
	}
	return sentinelRelease{record.Tag, record.Commit}
}

func sentinelName(release sentinelRelease) string {
	if release.Tag != "" {
		return release.Tag
	}
	return "commit " + release.Commit
}

// target returns the release that install and update fetch, a note, and the
// reason that blocks an install. Without a record, a failed lookup blocks the
// install. With a record, a failed lookup, a downgrade or a tag that names
// another commit keeps the installed release and gives a note.
func (d *SentinelDriver) target(ctx context.Context, record *sentinelRecord) (sentinelRelease, string, string) {
	latest, err := d.resolve(ctx)
	if record == nil {
		if err != nil {
			return sentinelRelease{}, "", "Setup cannot find the latest Sentinel release: " + err.Error() + ". Check the network and the proxy settings (HTTPS_PROXY), then apply again."
		}
		return latest, "", ""
	}
	installed := d.installed(record)
	switch {
	case err != nil:
	case installed.Tag != "" && latest.Tag == installed.Tag && latest.Commit != installed.Commit:
		err = fmt.Errorf("the tag %s now names the commit %s, and setup installed %s; setup refuses it", latest.Tag, latest.Commit, installed.Commit)
	case installed.Tag != "" && sentinelOlder(latest.Tag, installed.Tag):
		err = fmt.Errorf("the latest release %s is older than the installed release %s; setup refuses a downgrade", latest.Tag, installed.Tag)
	case latest.Commit != installed.Commit:
		return latest, "Sentinel " + latest.Tag + " is available, and " + sentinelName(installed) + " is installed. Update tools installs it.", ""
	default:
		return latest, "", ""
	}
	return installed, "Setup could not check for a newer Sentinel release: " + err.Error() + ". It keeps " + sentinelName(installed) + ".", ""
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
		return commit, errors.Join(fmt.Errorf("%s differs from commit %s; setup fetches the commit again", directory, commit), err)
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

// prepare returns a checkout of commit. It keeps a verified checkout of that
// commit. Otherwise it fetches only that commit into a new folder, verifies it
// and replaces the old checkout. It refuses any other commit.
func (d *SentinelDriver) prepare(ctx context.Context, want string) (_ string, err error) {
	if commit, err := d.verify(ctx, d.Run, d.checkout()); err == nil && commit == want {
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
		{"fetch", "--quiet", "--depth", "1", "--no-tags", d.Repository, want},
		{"checkout", "--quiet", "--detach", "FETCH_HEAD"},
	} {
		if _, err := d.Run(ctx, d.git(stage, arguments...)); err != nil {
			return "", fmt.Errorf("cannot fetch Sentinel %s from %s. Check the network and the proxy settings (git uses HTTPS_PROXY), then try again: %w", want, d.Repository, err)
		}
	}
	commit, err := d.verify(ctx, d.Run, stage)
	if err != nil {
		return "", err
	}
	if commit != want {
		return "", fmt.Errorf("the fetched Sentinel commit %s is not the expected commit %s; setup refuses it", commit, want)
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
			"-Global", "-Hosts", strings.Join(hosts, ","), "-Skills", "core,ui,formal,ops"}
		if mode != "" {
			arguments = append(arguments, "-"+strings.ToUpper(mode[:1])+mode[1:])
		}
		return nativeCommand{Program: d.Shell, Arguments: arguments, Environment: environment}, nil
	}
	arguments := []string{filepath.Join(checkout, "tools", "install"), "--global", "--hosts", strings.Join(hosts, ","), "--skills", "core,ui,formal,ops"}
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
// that a record exists, and CompletedOperation is its finished operation.
// Desired is the commit that install and update fetch. A newer release, or a
// failed lookup, is only a note. The check runs only from a checkout that
// verify proves to be the recorded commit: exit 0 is healthy, exit 1 means
// that installed files differ, and any other exit is an error. An interrupted
// operation of this receipt offers a resume.
func (d *SentinelDriver) Observe(ctx context.Context, r Resource, receipt Receipt) (Observation, error) {
	o := Observation{Provider: "sentinel", Identity: d.record(), Scope: "user"}
	record, err := d.readRecord()
	if err != nil {
		return Observation{Unknown: true, Provider: "sentinel", Scope: "user", Pending: err.Error()}, nil
	}
	release, note, blocked := d.target(ctx, record)
	o.Desired, o.ApplyBlocked = release.Commit, blocked
	if note != "" {
		o.Notes = append(o.Notes, note)
	}
	homes := d.homes(record)
	hosts, err := d.hosts(homes)
	if err != nil {
		return o, err
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
			return Observation{Provider: "sentinel", Identity: d.record(), Desired: o.Desired, Scope: "user", ApplyBlocked: o.ApplyBlocked, Notes: o.Notes}, nil
		}
		o.Present, o.HealthIssue = true, "an earlier Sentinel "+record.Action+" did not finish; apply runs the Sentinel installer again"
		return o, nil
	}
	o.Present, o.CompletedOperation = true, record.Operation
	// After a resume or a recovered completion, the controller saves this
	// observation in the receipt. It names the installed commit, so that a later
	// update still sees a newer release.
	if receipt.OperationID == record.Operation && receipt.Status != "ready" {
		o.Desired = record.Commit
	}
	commit, err := d.verify(ctx, d.Query, d.checkout())
	switch {
	case err != nil:
		o.HealthIssue = err.Error()
		return o, nil
	case commit == "":
		o.HealthIssue = "the Sentinel checkout is missing; repair fetches the installed commit again"
		return o, nil
	case commit != record.Commit:
		o.HealthIssue = "the Sentinel checkout holds " + commit + ", and setup installed " + record.Commit + "; repair fetches the installed commit again"
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

// Apply installs a release for the current hosts. Install and update fetch the
// approved commit, op.Observed.Desired. Repair and resume fetch the recorded
// commit. Apply saves its intent first, so an interruption at any later step
// can resume. Then it fetches the commit and runs the installer. A failed fetch
// deletes nothing. The result names the installed commit as Desired, so that a
// later update sees a newer release.
func (d *SentinelDriver) Apply(ctx context.Context, r Resource, op Operation, receipt Receipt) (Observation, error) {
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
	action := op.Action
	if action == "adopt" || action == "" {
		action = "install"
	}
	var release sentinelRelease
	switch {
	case op.Action != "repair" && op.Observed.Desired != "":
		release.Commit = op.Observed.Desired
		if latest, err := d.resolve(ctx); err == nil && latest.Commit == release.Commit {
			release = latest
		} else if record != nil && record.Commit == release.Commit {
			release = d.installed(record)
		}
	case record != nil:
		release = d.installed(record)
	default:
		return Observation{}, errors.New("Sentinel has no approved release to install")
	}
	if !gitCommit(release.Commit) {
		return Observation{}, fmt.Errorf("the approved Sentinel commit %q is not a commit", release.Commit)
	}
	intent := sentinelRecord{Schema: 1, Operation: receipt.OperationID, Action: action, Commit: release.Commit, Tag: release.Tag, Hosts: hosts, Homes: homes}
	if record != nil {
		intent.Hosts = sentinelUnion(record.Hosts, hosts)
	}
	if err := saveDocument(d.record(), intent); err != nil {
		return Observation{}, err
	}
	checkout, err := d.prepare(ctx, release.Commit)
	if err != nil {
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
	o, err := d.Observe(ctx, r, receipt)
	o.Desired = release.Commit
	return o, err
}

// Remove saves its intent, then runs the installer removal from a verified
// checkout of the recorded commit, or of the floor without a record, for each
// recorded or current host. It uses the recorded folders. Then it deletes the
// checkout and the record.
func (d *SentinelDriver) Remove(ctx context.Context, r Resource, receipt Receipt) (Observation, error) {
	record, err := d.readRecord()
	if err != nil {
		return Observation{}, err
	}
	homes := d.homes(record)
	current, err := d.hosts(homes)
	if err != nil {
		return Observation{}, err
	}
	release := d.Floor
	if record != nil {
		release = d.installed(record)
	}
	intent := sentinelRecord{Schema: 1, Operation: receipt.OperationID, Action: "remove", Commit: release.Commit, Tag: release.Tag, Hosts: current, Homes: homes}
	if record != nil {
		intent.Hosts = sentinelUnion(record.Hosts, current)
	}
	if err := saveDocument(d.record(), intent); err != nil {
		return Observation{}, err
	}
	if len(intent.Hosts) != 0 {
		checkout, err := d.prepare(ctx, release.Commit)
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
