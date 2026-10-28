package installer

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"
)

// These tests are hermetic: native steps run fixed commands against private
// temporary files through the real worker record format. Package managers,
// Nix and system accounts are exercised only by the opted-in native lane.

func legacyStepFixture(t *testing.T) (*legacyRetirementDriver, string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("POSIX native step semantics; Windows retirement commands run in the native lane")
	}
	root, err := resolveConfigPath(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	worker := filepath.Join(root, "worker")
	d := &legacyRetirementDriver{Directory: filepath.Join(root, "retirement"), WorkerDirectory: worker, Folders: ConfigFolders{Home: root},
		Normal:      Controller{StatePath: filepath.Join(root, "normal", "state.json"), Home: root},
		Environment: legacyRuntimeEnvironment{Username: "fixture", Root: true, Programs: map[string]string{}}, Query: queryLegacyRuntime}
	// The same persisted intent/result protocol as the native worker.
	d.Run = func(_ context.Context, command nativeCommand) ([]byte, error) {
		reply, err := executeNativeCommand(worker, command, nil)
		if err != nil {
			return nil, err
		}
		if !legacyReplySucceeded(reply) {
			return reply.Output, &nativeCommandError{Message: reply.Error, ExitCode: reply.ExitCode}
		}
		return reply.Output, nil
	}
	return d, root
}

func legacyTestIntent(t *testing.T, d *legacyRetirementDriver, steps ...legacyRetirementStep) *legacyRetirementIntent {
	t.Helper()
	scope := legacyRetirementScope{Steps: steps}
	var err error
	scope.ReplacementState, err = d.replacementState()
	if err != nil {
		t.Fatal(err)
	}
	desired, err := digest(scope)
	if err != nil {
		t.Fatal(err)
	}
	operation, err := digest("legacy retirement fixture " + t.Name())
	if err != nil {
		t.Fatal(err)
	}
	intent := &legacyRetirementIntent{Schema: 1, Resource: "retire.tools", Operation: operation, Scope: scope, Attempts: make([]int, len(steps)),
		Before: Observation{Provider: "legacy-retirement", Identity: "retire.tools", Scope: "user", Desired: desired}}
	if err := saveDocument(d.intentPath(operation), intent); err != nil {
		t.Fatal(err)
	}
	return intent
}

func TestLegacyRetirementCatalogPhasesAndPlatforms(t *testing.T) {
	c := legacyRetirementCatalog()
	if err := c.Validate(); err != nil {
		t.Fatal(err)
	}
	ids := func(resources []Resource) []string {
		result := []string{}
		for _, r := range resources {
			result = append(result, r.ID)
		}
		return result
	}
	darwin, windows := Context{OS: "darwin", Arch: "arm64"}, Context{OS: "windows", Arch: "amd64"}
	if got := ids(c.Capabilities(windows)); !slices.Contains(got, "retire.chezmoi") || !slices.Contains(got, "retire.delta.psmux") || !slices.Contains(got, "retire.delta.wezterm") || slices.Contains(got, "retire.nix") || slices.Contains(got, "retire.homebrew") {
		t.Fatal("Windows retirement must not offer POSIX runtimes", got)
	}
	if got := ids(c.Capabilities(linux)); slices.Contains(got, "retire.homebrew") || !slices.Contains(got, "retire.nix") {
		t.Fatal(got)
	}
	if got := ids(legacyPhaseResources(c, darwin, LegacyRetirementPreparation)); !slices.Equal(got, []string{"retire.homebrew"}) {
		t.Fatal("Homebrew conversion is the only preparation item", got)
	}
	order, err := c.Closure([]string{"retire.nix"}, darwin)
	if err != nil || !slices.Equal(order, []string{"retire.homebrew", legacyReplacements, "retire.nix"}) {
		t.Fatal("Nix removal must follow Homebrew conversion and verified replacements", order, err)
	}
	if r, _ := c.Resource(legacyReplacements); r.Capability || !r.Retain {
		t.Fatal("replacement verification is hidden retained infrastructure")
	}
}

func TestLegacyRetirementReleasedPinsMatchReleaseSource(t *testing.T) {
	repository, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	git := func(arguments ...string) string {
		t.Helper()
		output, err := exec.Command("git", append([]string{"-C", repository}, arguments...)...).Output()
		if err != nil {
			t.Fatalf("released evidence requires complete release history: git %v: %v", arguments, err)
		}
		return string(output)
	}
	if strings.TrimSpace(git("rev-parse", legacyReleaseCommit+"^{commit}")) != legacyReleaseCommit {
		t.Fatal("the pre-overhaul main differs from the reviewed commit")
	}
	values := func(commit, path string) (map[string]string, string) {
		source := git("show", commit+":"+path)
		result := map[string]string{}
		for _, line := range strings.Split(source, "\n") {
			key, value, ok := strings.Cut(line, "=")
			if !ok || key == "" || strings.ContainsAny(key, " \t$") || !strings.HasPrefix(value, `"`) {
				continue
			}
			value = strings.TrimPrefix(value, `"`)
			if end := strings.Index(value, `"`); end >= 0 {
				if _, seen := result[key]; !seen {
					result[key] = value[:end]
				}
			}
		}
		return result, source
	}
	deps, source := values(legacyReleaseCommit, "install-deps.sh")
	names := map[string][2]string{"nvim": {"NVIM_LINUX_VERSION", "NVIM_LINUX"}, "chezmoi": {"CHEZMOI_VERSION", "CHEZMOI_LINUX"}, "lazygit": {"LAZYGIT_LINUX_VERSION", "LAZYGIT_LINUX"},
		"starship": {"STARSHIP_VERSION", "STARSHIP_LINUX"}, "tree-sitter": {"TREE_SITTER_CLI_LINUX_VERSION", "TREE_SITTER_CLI_LINUX"}, "herdr": {"HERDR_VERSION", "HERDR_LINUX"}}
	assets := map[string]map[string]string{
		"nvim":        {"amd64": "neovim/neovim/releases/download/%[1]s/nvim-linux-x86_64.tar.gz", "arm64": "neovim/neovim/releases/download/%[1]s/nvim-linux-arm64.tar.gz"},
		"chezmoi":     {"amd64": "twpayne/chezmoi/releases/download/%[1]s/chezmoi_%[2]s_linux_amd64.tar.gz", "arm64": "twpayne/chezmoi/releases/download/%[1]s/chezmoi_%[2]s_linux_arm64.tar.gz"},
		"lazygit":     {"amd64": "jesseduffield/lazygit/releases/download/%[1]s/lazygit_%[2]s_linux_x86_64.tar.gz", "arm64": "jesseduffield/lazygit/releases/download/%[1]s/lazygit_%[2]s_linux_arm64.tar.gz"},
		"starship":    {"amd64": "starship/starship/releases/download/%[1]s/starship-x86_64-unknown-linux-gnu.tar.gz", "arm64": "starship/starship/releases/download/%[1]s/starship-aarch64-unknown-linux-musl.tar.gz"},
		"tree-sitter": {"amd64": "tree-sitter/tree-sitter/releases/download/%[1]s/tree-sitter-cli-linux-x64.zip", "arm64": "tree-sitter/tree-sitter/releases/download/%[1]s/tree-sitter-cli-linux-arm64.zip"},
		"herdr":       {"amd64": "ogulcancelik/herdr/releases/download/%[1]s/herdr-linux-x86_64", "arm64": "ogulcancelik/herdr/releases/download/%[1]s/herdr-linux-aarch64"},
	}
	seen := map[string]bool{}
	for _, pin := range legacyReleasedArtifacts {
		deps, _ := values(pin.Commit, "install-deps.sh")
		name := names[pin.Tool]
		arch := map[string]string{"amd64": "X86_64", "arm64": "ARM64"}[pin.Arch]
		url := "https://github.com/" + fmt.Sprintf(assets[pin.Tool][pin.Arch], pin.Version, strings.TrimPrefix(pin.Version, "v"))
		if deps[name[0]] != pin.Version || deps[name[1]+"_"+arch+"_SHA256"] != pin.SHA256 || pin.URL != url {
			t.Errorf("%s/%s differs from the released install-deps.sh pin", pin.Tool, pin.Arch)
		}
		seen[pin.Commit+"/"+pin.Tool+"/"+pin.Arch] = true
	}
	if len(seen) != 24 {
		t.Fatal("every released direct artifact and architecture needs exactly one pin", seen)
	}
	for _, template := range []string{`asset="nvim-linux-${arch}.tar.gz"`, `asset="chezmoi_${version_no_v}_linux_${arch}.tar.gz"`, `printf 'schema=2\n'`, `printf '%s/%s.env\n' "$(direct_artifact_provenance_dir)" "$1"`} {
		if !strings.Contains(source, template) {
			t.Error("released artifact template changed:", template)
		}
	}
	commits := map[string]string{}
	for _, checkout := range (&legacyRetirementDriver{Folders: ConfigFolders{Home: "/home/fixture"}}).legacyCheckouts() {
		commits[filepath.Base(checkout.Path)] = checkout.Commit
	}
	setup, _ := values(legacyReleaseCommit, "setup.sh")
	for name, key := range map[string]string{"tpm": "TPM_COMMIT", "tmux-sensible": "TMUX_SENSIBLE_COMMIT", "tmux-yank": "TMUX_YANK_COMMIT", "tmux-resurrect": "TMUX_RESURRECT_COMMIT",
		"tmux-continuum": "TMUX_CONTINUUM_COMMIT", "fzf-tab": "FZF_TAB_COMMIT", "zsh-autosuggestions": "ZSH_AUTOSUGGESTIONS_COMMIT"} {
		if commits[name] == "" || commits[name] != deps[key] {
			t.Errorf("%s differs from released %s", name, key)
		}
	}
	if commits[setup["SENTINEL_REF"]] != setup["SENTINEL_REF"] {
		t.Error("Sentinel cache pin differs from the released setup")
	}
	if deps["GH_DASH_VERSION"] != "v4.26.0" || !slices.Contains(legacyGhDashTags, deps["GH_DASH_VERSION"]) || deps["PI_CLI_VERSION"] != "1.0.4" || deps["PI_CLI_PACKAGE"] != "@earendil-works/pi-coding-agent" {
		t.Error("released gh-dash or Pi identity changed")
	}
	nix, _ := values(legacyReleaseCommit, "scripts/install-nix-prerequisite.sh")
	if "nix (Nix) "+nix["nix_version"] != legacyNixVersion {
		t.Error("released Nix prerequisite version changed")
	}
}

func TestLegacyProvenanceMarkerIsParsedAsExactData(t *testing.T) {
	valid := "schema=2\ntool=herdr\nversion=v0.9.3\nsource_url=u\nsha256=s\nbinary_sha256=s\ncommand_path=/c\nbinary_path=/c\ninstall_root=/\n"
	if values, err := parseLegacyMarker([]byte(valid)); err != nil || values["tool"] != "herdr" || values["install_root"] != "/" {
		t.Fatal(values, err)
	}
	for name, data := range map[string]string{
		"duplicate":    valid + "tool=lazygit\n",
		"unknown":      valid + "PATH=$(rm -rf ~)\n",
		"schema":       strings.Replace(valid, "schema=2", "schema=1", 1),
		"unterminated": strings.TrimSuffix(valid, "\n"),
		"missing":      strings.Replace(valid, "install_root=/\n", "", 1),
	} {
		if _, err := parseLegacyMarker([]byte(data)); err == nil {
			t.Error("accepted", name)
		}
	}
}

func TestLegacyDirectArtifactRequiresReleasedPinShapeAndBinary(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("released direct artifacts are Linux-only")
	}
	home, err := resolveConfigPath(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	catalog, err := DefaultCatalog()
	if err != nil {
		t.Fatal(err)
	}
	target := Context{OS: "linux", Arch: "amd64"}
	d := &legacyRetirementDriver{Directory: filepath.Join(home, "state", "dotfiles", "migration", "retirement"), Platform: NativePlatform{Context: target}, Folders: ConfigFolders{Home: home},
		Normal: Controller{Catalog: catalog, Context: target, Home: home, StatePath: filepath.Join(home, "state", "dotfiles", "state.json")}}
	binary := filepath.Join(home, ".local", "bin", "tree-sitter")
	marker := filepath.Join(home, ".local", "share", "dotfiles", "provenance", "tree-sitter.env")
	pin := legacyReleasedArtifacts[slices.IndexFunc(legacyReleasedArtifacts, func(pin legacyArtifactPin) bool { return pin.Tool == "tree-sitter" && pin.Arch == "amd64" })]
	writeMarker := func(version, binarySHA string) {
		writeConfigFixture(t, marker, fmt.Sprintf("schema=2\ntool=tree-sitter\nversion=%s\nsource_url=%s\nsha256=%s\nbinary_sha256=%s\ncommand_path=%s\nbinary_path=%s\ninstall_root=%s\n",
			version, pin.URL, pin.SHA256, binarySHA, binary, binary, filepath.Dir(binary)))
	}
	writeConfigFixture(t, binary, "released tree-sitter\n")
	writeMarker(pin.Version, legacyHash([]byte("released tree-sitter\n")))
	var scope legacyRetirementScope
	if err := d.planDirectArtifact(&scope, "tree-sitter"); err != nil || len(scope.Steps) != 2 || len(scope.Retained) != 0 || scope.Steps[0].Path != binary || scope.Steps[1].Path != marker {
		t.Fatal(scope, err)
	}
	if !slices.ContainsFunc(scope.Detail, func(line string) bool { return strings.Contains(line, "not selected") }) {
		t.Fatal("an unselected replacement must be disclosed", scope.Detail)
	}
	writeConfigFixture(t, binary, "personal rebuild\n")
	scope = legacyRetirementScope{}
	if err := d.planDirectArtifact(&scope, "tree-sitter"); err != nil || len(scope.Steps) != 0 || len(scope.Retained) != 1 {
		t.Fatal("a changed binary must be preserved", scope, err)
	}
	writeMarker("v0.26.0", legacyHash([]byte("personal rebuild\n")))
	scope = legacyRetirementScope{}
	if err := d.planDirectArtifact(&scope, "tree-sitter"); err != nil || len(scope.Steps) != 0 || !strings.Contains(strings.Join(scope.Retained, ""), "pre-overhaul pin") {
		t.Fatal("an unreviewed release pin must be preserved", scope, err)
	}
}

func legacyGitFixture(t *testing.T, repository string, arguments ...string) string {
	t.Helper()
	environment := []string{"GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null", "GIT_AUTHOR_NAME=fixture", "GIT_AUTHOR_EMAIL=fixture@example.invalid",
		"GIT_COMMITTER_NAME=fixture", "GIT_COMMITTER_EMAIL=fixture@example.invalid"}
	for _, value := range os.Environ() {
		if !strings.HasPrefix(value, "GIT_") {
			environment = append(environment, value)
		}
	}
	command := exec.Command("git", append([]string{"-C", repository}, arguments...)...)
	command.Env = environment
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v %s", arguments, err, output)
	}
	return string(output)
}

func TestLegacyPinnedCheckoutRequiresExactCleanPin(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("released pinned checkouts are POSIX-only")
	}
	git, err := exec.LookPath("git")
	if err != nil {
		t.Fatal("the contributor gate requires Git:", err)
	}
	home, err := resolveConfigPath(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	repository := filepath.Join(home, ".local", "share", "dotfiles", "zsh-plugins", "fixture")
	if err := os.MkdirAll(repository, 0700); err != nil {
		t.Fatal(err)
	}
	legacyGitFixture(t, repository, "init", "-q")
	writeConfigFixture(t, filepath.Join(repository, "fixture.plugin.zsh"), "# plugin\n")
	legacyGitFixture(t, repository, "add", "fixture.plugin.zsh")
	legacyGitFixture(t, repository, "commit", "-q", "-m", "fixture")
	legacyGitFixture(t, repository, "remote", "add", "origin", "https://example.invalid/fixture.git")
	head := strings.TrimSpace(legacyGitFixture(t, repository, "rev-parse", "HEAD"))
	d := &legacyRetirementDriver{Folders: ConfigFolders{Home: home}, Environment: legacyRuntimeEnvironment{Username: "fixture", Programs: map[string]string{"git": git}}, Query: queryLegacyRuntime}
	checkout := legacyCheckout{Path: repository, Origin: "https://example.invalid/fixture", Commit: head, Entry: "fixture.plugin.zsh"}
	ctx := context.Background()
	var scope legacyRetirementScope
	if err := d.planCheckout(ctx, &scope, checkout); err != nil || len(scope.Steps) != 1 || scope.Steps[0].Kind != "discard" || len(scope.Retained) != 0 {
		t.Fatal(scope, err)
	}
	index, err := legacyFile(filepath.Join(repository, ".git", "index"))
	if err != nil {
		t.Fatal(err)
	}
	if err := d.planCheckout(ctx, &legacyRetirementScope{}, checkout); err != nil {
		t.Fatal(err)
	}
	if err := verifyConfigSnapshot(index.Path, index.Snapshot); err != nil {
		t.Fatal("inspection rewrote the Git index and would invalidate approval", err)
	}
	writeConfigFixture(t, filepath.Join(repository, "personal-notes.txt"), "keep me\n")
	scope = legacyRetirementScope{}
	if err := d.planCheckout(ctx, &scope, checkout); err != nil || len(scope.Steps) != 0 || len(scope.Retained) != 1 {
		t.Fatal("an untracked personal file must keep the checkout", scope, err)
	}
	if err := os.Remove(filepath.Join(repository, "personal-notes.txt")); err != nil {
		t.Fatal(err)
	}
	checkout.Commit = strings.Repeat("0", 40)
	scope = legacyRetirementScope{}
	if err := d.planCheckout(ctx, &scope, checkout); err != nil || len(scope.Steps) != 0 || len(scope.Retained) != 1 {
		t.Fatal("a different commit must keep the checkout", scope, err)
	}
}

func TestLegacyNativeStepConsultsSavedResultBeforePreconditions(t *testing.T) {
	d, root := legacyStepFixture(t)
	target, counter := filepath.Join(root, "artifact"), filepath.Join(root, "runs")
	writeConfigFixture(t, target, "released bytes\n")
	before, err := legacyFile(target)
	if err != nil {
		t.Fatal(err)
	}
	command := nativeCommand{Program: "/bin/sh", Arguments: []string{"-c", `printf x >> "$1" && rm -f -- "$2"`, "sh", counter, target}}
	intent := legacyTestIntent(t, d, legacyRetirementStep{Kind: "command", Label: "remove the fixture artifact", Command: &command, Before: []legacyRetirementFile{before}, Absent: []string{target}})
	ctx := context.Background()
	if err := d.executeStep(ctx, intent, 0, false); err != nil {
		t.Fatal(err)
	}
	// A controller that dies after the worker persisted success leaves Next
	// unchanged. The original precondition no longer holds; the saved result wins.
	if err := d.executeStep(ctx, intent, 0, false); err != nil {
		t.Fatal("saved success was not consulted before preconditions:", err)
	}
	if data, err := os.ReadFile(counter); err != nil || string(data) != "x" {
		t.Fatal("a completed native step was repeated", string(data), err)
	}
	if outcome, err := d.stepOutcome(*intent, 0); err != nil || outcome.Status != legacyOutcomeSucceeded {
		t.Fatal(outcome, err)
	}
}

func TestLegacyNativeStepNeverReplaysUnknownOutcome(t *testing.T) {
	d, root := legacyStepFixture(t)
	counter := filepath.Join(root, "runs")
	command := nativeCommand{Program: "/bin/sh", Arguments: []string{"-c", `printf x >> "$1"`, "sh", counter}}
	intent := legacyTestIntent(t, d, legacyRetirementStep{Kind: "command", Label: "fixture", Command: &command})
	saved, err := d.stepCommand(*intent, 0)
	if err != nil {
		t.Fatal(err)
	}
	hash, err := digest(saved)
	if err != nil {
		t.Fatal(err)
	}
	// The worker persists intent before starting; no reply means unknown.
	if err := saveDocument(filepath.Join(d.WorkerDirectory, "commands", saved.Operation+".json"), nativeCommandRecord{Schema: 1, Command: hash}); err != nil {
		t.Fatal(err)
	}
	for _, recovery := range []bool{false, true} {
		if err := d.executeStep(context.Background(), intent, 0, recovery); !errors.Is(err, errLegacyOutcomeUnknown) {
			t.Fatal(recovery, err)
		}
	}
	if _, err := os.Stat(counter); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("an unknown native outcome was replayed", err)
	}
	o, err := d.observeInterrupted(context.Background(), Observation{}, *intent)
	if err != nil || o.ResourceResume != nil || !strings.Contains(o.Pending, "unknown") {
		t.Fatal("unknown outcomes must not receive recovery approval", o, err)
	}
}

func TestLegacyNativeStepKnownFailureNeedsExplicitNewAttempt(t *testing.T) {
	d, root := legacyStepFixture(t)
	counter := filepath.Join(root, "runs")
	command := nativeCommand{Program: "/bin/sh", Arguments: []string{"-c", `printf x >> "$1"; exit 3`, "sh", counter}}
	intent := legacyTestIntent(t, d, legacyRetirementStep{Kind: "command", Label: "fixture", Command: &command})
	ctx := context.Background()
	for _, recovery := range []bool{false, false} {
		if err := d.executeStep(ctx, intent, 0, recovery); err == nil {
			t.Fatal("failed native command reported success")
		}
	}
	if data, err := os.ReadFile(counter); err != nil || string(data) != "x" {
		t.Fatal("a known failure was retried without recovery approval", string(data), err)
	}
	o, err := d.observeInterrupted(ctx, Observation{}, *intent)
	if err != nil || o.ResourceResume == nil || !strings.Contains(o.Pending, "failed") {
		t.Fatal(o, err)
	}
	if err := d.executeStep(ctx, intent, 0, true); err == nil {
		t.Fatal("the new attempt also fails")
	}
	saved, err := readDocument(d.intentPath(intent.Operation))
	if err != nil {
		t.Fatal(err)
	}
	var durable legacyRetirementIntent
	if err := Decode(saved, &durable); err != nil || durable.Attempts[0] != 1 {
		t.Fatal("the new attempt must be durable before dispatch", durable.Attempts, err)
	}
	if data, err := os.ReadFile(counter); err != nil || string(data) != "xx" {
		t.Fatal(string(data), err)
	}
}

func TestLegacyInterruptedObservationCountsOnlyRemainingAdministratorSteps(t *testing.T) {
	d, _ := legacyStepFixture(t)
	command := nativeCommand{Program: "/bin/sh", Arguments: []string{"-c", "true"}}
	intent := legacyTestIntent(t, d,
		legacyRetirementStep{Kind: "command", Label: "completed administrator step", Command: &command, Privileged: true},
		legacyRetirementStep{Kind: "command", Label: "remaining user step", Command: &command})
	ctx := context.Background()
	if err := d.executeStep(ctx, intent, 0, false); err != nil {
		t.Fatal(err)
	}
	intent.Next = 1
	if err := saveDocument(d.intentPath(intent.Operation), intent); err != nil {
		t.Fatal(err)
	}
	observed, err := d.observeInterrupted(ctx, Observation{}, *intent)
	if err != nil || observed.Privileged || observed.ResourceResume == nil || !strings.Contains(observed.Pending, "remaining user step") {
		t.Fatal("completed administrator step leaked into remaining approval", observed, err)
	}
	scope := intent.Scope
	scope.Steps = scope.Steps[intent.Next:]
	if !strings.Contains(legacyScopeSummary(scope), "0 administrator") {
		t.Fatal("remaining summary disagrees with observation", legacyScopeSummary(scope))
	}
}

func TestLegacyLineRemovalKeepsPersonalBytesAndOriginal(t *testing.T) {
	d, root := legacyStepFixture(t)
	ctx := context.Background()
	entry := "UUID=11111111-2222-3333-4444-555555555555 /nix apfs rw,noauto,nobrowse,nosuid,noatime,owners"
	original := "# personal comment\n" + entry + "\nLABEL=personal /Volumes/data apfs rw\n"
	fstab := filepath.Join(root, "fstab")
	writeConfigFixture(t, fstab, original)
	var scope legacyRetirementScope
	if err := d.addLineRemoval(context.Background(), &scope, "Stop mounting the fixture store", fstab, entry); err != nil || scope.Blocked != "" || len(scope.Steps) != 1 {
		t.Fatal(scope, err)
	}
	intent := legacyTestIntent(t, d, scope.Steps...)
	if err := d.executeStep(ctx, intent, 0, false); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(fstab); err != nil || string(data) != "# personal comment\nLABEL=personal /Volumes/data apfs rw\n" {
		t.Fatal("personal fstab bytes changed", string(data), err)
	}
	if data, err := os.ReadFile(scope.Steps[0].Backup); err != nil || string(data) != original {
		t.Fatal("the original system file was not kept", err)
	}
	if info, err := os.Stat(fstab); err != nil || info.Mode().Perm() != 0600 {
		t.Fatal("system file mode changed", err)
	}
	synthetic := filepath.Join(root, "synthetic.conf")
	writeConfigFixture(t, synthetic, "nix\n")
	scope = legacyRetirementScope{}
	if err := d.addLineRemoval(context.Background(), &scope, "Stop creating the fixture mount point", synthetic, "nix"); err != nil || len(scope.Steps) != 1 || !slices.Contains(scope.Steps[0].Absent, synthetic) {
		t.Fatal(scope, err)
	}
	intent = legacyTestIntent(t, d, scope.Steps...)
	intent.Operation = strings.Repeat("a", 64)
	if err := saveDocument(d.intentPath(intent.Operation), intent); err != nil {
		t.Fatal(err)
	}
	if err := d.executeStep(ctx, intent, 0, false); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(synthetic); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("an emptied system file must be removed", err)
	}
	duplicate := filepath.Join(root, "shells")
	writeConfigFixture(t, duplicate, "/nix/store/fixture/bin/zsh\n/nix/store/fixture/bin/zsh\n")
	scope = legacyRetirementScope{}
	if err := d.addLineRemoval(context.Background(), &scope, "Unregister", duplicate, "/nix/store/fixture/bin/zsh"); err != nil || scope.Blocked == "" || len(scope.Steps) != 0 {
		t.Fatal("a repeated line must be reviewed, never guessed", scope, err)
	}
}

func TestLegacyRootFilesAreMovedAsideNotDeleted(t *testing.T) {
	d, root := legacyStepFixture(t)
	home := filepath.Join(root, "root-home")
	writeConfigFixture(t, filepath.Join(home, ".cache", "nix", "fetcher-cache-v1.sqlite"), "cache\n")
	writeConfigFixture(t, filepath.Join(home, ".bashrc"), "# personal\n")
	if err := os.Symlink("/nix/var/nix/profiles/default", filepath.Join(home, ".nix-profile")); err != nil {
		t.Fatal(err)
	}
	backup := filepath.Join(home, ".dotfiles-retired-profile")
	cacheBackup := filepath.Join(home, ".cache", ".dotfiles-retired-nix")
	steps := []legacyRetirementStep{}
	for _, pair := range [][2]string{{filepath.Join(home, ".nix-profile"), backup}, {filepath.Join(home, ".cache", "nix"), cacheBackup}} {
		command := d.systemCommand(true, "/bin/sh", "-c", legacyRootFilesScript, "sh", pair[0], pair[1])
		steps = append(steps, legacyRetirementStep{Kind: "command", Label: "Preserve fixed root fixture", Privileged: true, Backup: pair[1], Command: &command})
	}
	intent := legacyTestIntent(t, d, steps...)
	for i := range steps {
		if err := d.executeStep(context.Background(), intent, i, false); err != nil {
			t.Fatal(err)
		}
	}
	if target, err := os.Readlink(backup); err != nil || target != "/nix/var/nix/profiles/default" {
		t.Fatal("profile link was not preserved intact", target, err)
	}
	if data, err := os.ReadFile(filepath.Join(cacheBackup, "fetcher-cache-v1.sqlite")); err != nil || string(data) != "cache\n" {
		t.Fatal(err)
	}
	if !slices.Contains(intent.Preserved, backup) || !slices.Contains(intent.Preserved, cacheBackup) {
		t.Fatal("root recovery locations were not recorded", intent.Preserved)
	}
	if data, err := os.ReadFile(filepath.Join(home, ".bashrc")); err != nil || string(data) != "# personal\n" {
		t.Fatal("unrelated root files changed", err)
	}
}

// The former path-only validator accepted this incomplete, non-vendor shape.
// It must remain rejected alongside the official-receipt tests.
func TestLegacyDeterminateReceiptScopeIsValidatedAsData(t *testing.T) {
	valid := `{"version":"3.11.3","actions":[{"action":{"action":"create_directory","path":"/nix","user":null},"state":"Completed"},` +
		`{"action":{"action":"create_or_insert_into_file","path":"/etc/zshrc","buf":"\n# Nix\nif [ -e '/nix/var/nix/profiles/default/etc/profile.d/nix-daemon.sh' ]; then\n"},"state":"Completed"}],` +
		`"planner":{"planner":"macos","settings":{}}}`
	for _, document := range []string{valid, strings.Replace(valid, `"path":"/etc/zshrc"`, `"path":"/Users/fixture/.zshrc"`, 1),
		strings.Replace(valid, `"planner"`, `"hooks"`, 1), `{"version":"3","actions":[],"planner":{}}`, `not json`} {
		if _, err := parseLegacyDeterminateReceipt([]byte(document)); err == nil {
			t.Error("accepted an unreviewed or incomplete vendor receipt", document)
		}
	}
}

func TestLegacyPlistDirectoryServiceAndMountParsing(t *testing.T) {
	plist := `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>org.nixos.darwin-store</string>
	<key>ProgramArguments</key>
	<array>
		<string>/usr/sbin/diskutil</string>
		<string>mount</string>
		<string>-mountPoint</string>
		<string>/nix</string>
		<string>11111111-2222-3333-4444-555555555555</string>
	</array>
	<key>RunAtLoad</key>
	<true/>
</dict>
</plist>`
	values, err := legacyPlist([]byte(plist))
	if err != nil || legacyPlistString(values, "Label") != "org.nixos.darwin-store" || values["RunAtLoad"].XMLName.Local != "true" ||
		!slices.Equal(legacyPlistStrings(values, "ProgramArguments"), []string{"/usr/sbin/diskutil", "mount", "-mountPoint", "/nix", "11111111-2222-3333-4444-555555555555"}) {
		t.Fatal(values, err)
	}
	if _, err := legacyPlist([]byte(strings.Replace(plist, "<key>RunAtLoad</key>", "<key>Label</key>", 1))); err == nil {
		t.Fatal("duplicate property list keys were accepted")
	}
	attributes := legacyDsclAttributes([]byte("IsHidden: 1\nNFSHomeDirectory: /var/empty\nPrimaryGroupID: 350\nRealName:\n Nix build user 1\nUniqueID: 351\nUserShell: /sbin/nologin\n"))
	if attributes["RealName"] != "Nix build user 1" || attributes["UniqueID"] != "351" || attributes["IsHidden"] != "1" {
		t.Fatal(attributes)
	}
	if legacyMountPath(`/nix\040store`) != "/nix store" || legacyMountPath(`/nix`) != "/nix" {
		t.Fatal("kernel mount paths must be unescaped")
	}
}

func legacyEngineFixture(t *testing.T) (*LegacyRetirement, *fileDriver, string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("hermetic engine fixture uses POSIX paths; Windows retirement runs in the native lane")
	}
	home, err := resolveConfigPath(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	normalDriver := &fileDriver{dir: home, home: home, statePath: filepath.Join(home, "state", "dotfiles", "state.json")}
	normal := Controller{Catalog: testCatalog(t), Context: linux, Source: "source", Home: home, StatePath: normalDriver.statePath, Driver: normalDriver}
	retirement, err := newLegacyRetirement(normal, NativePlatform{Context: linux}, ConfigFolders{Home: home}, home,
		legacyRuntimeEnvironment{Executable: "/usr/bin/env", Username: "fixture", Programs: map[string]string{}})
	if err != nil {
		t.Fatal(err)
	}
	return retirement, normalDriver, home
}

func TestLegacyReplacementsBindNormalSetupState(t *testing.T) {
	retirement, normalDriver, _ := legacyEngineFixture(t)
	ctx := context.Background()
	resource, _ := retirement.Controller.Catalog.Resource(legacyReplacements)
	o, err := retirement.driver.Observe(ctx, resource, Receipt{})
	if err != nil || o.Healthy || !strings.Contains(o.ApplyBlocked, "setup first") {
		t.Fatal("retirement must wait for an applied replacement selection", o, err)
	}
	c := retirement.driver.Normal.Catalog
	if _, err := Execute(ctx, c, linux, "source", normalDriver.home, normalDriver.statePath, approve(t, c, normalDriver, Request{Schema: 1, Mode: "apply", Selected: []string{"agent"}}), normalDriver); err != nil {
		t.Fatal(err)
	}
	ready, err := retirement.driver.Observe(ctx, resource, Receipt{})
	if err != nil || !ready.Healthy || ready.ApplyBlocked != "" || !operationID.MatchString(ready.Fingerprint) {
		t.Fatal(ready, err)
	}
	if value, err := retirement.driver.gateValue(ctx, legacyRetirementGate{Kind: "replacements"}); err != nil || value != "" {
		t.Fatal(value, err)
	}
	if err := os.WriteFile(filepath.Join(normalDriver.dir, "node"), []byte("changed outside setup"), 0600); err != nil {
		t.Fatal(err)
	}
	changed, err := retirement.driver.Observe(ctx, resource, Receipt{})
	if err != nil || changed.Fingerprint == ready.Fingerprint {
		t.Fatal("a changed replacement must invalidate retirement approval", changed, err)
	}
}

// APT's global database changes when migration removes a replaced DEB. The
// selected tools' own identities and health can remain completely unchanged.
type legacyReplacementInventoryDriver struct {
	*fileDriver
	inventory string
}

func (d *legacyReplacementInventoryDriver) Observe(ctx context.Context, r Resource, receipt Receipt) (Observation, error) {
	o, err := d.fileDriver.Observe(ctx, r, receipt)
	if err != nil {
		return o, err
	}
	data, err := os.ReadFile(d.inventory)
	if err != nil {
		return o, err
	}
	o.Inventory = legacyHash(data)
	return o, nil
}

func TestLegacyReplacementPreservationAllowsRetiredPackageInventory(t *testing.T) {
	retirement, normalDriver, home := legacyEngineFixture(t)
	ctx := context.Background()
	c := retirement.driver.Normal.Catalog
	if _, err := Execute(ctx, c, linux, "source", home, normalDriver.statePath, approve(t, c, normalDriver, Request{Schema: 1, Mode: "apply", Selected: []string{"agent"}}), normalDriver); err != nil {
		t.Fatal(err)
	}
	inventory := filepath.Join(home, "native-package-database")
	writeConfigFixture(t, inventory, "selected tools and old WezTerm\n")
	retirement.driver.Normal.Driver = &legacyReplacementInventoryDriver{fileDriver: normalDriver, inventory: inventory}
	resource, _ := retirement.Controller.Catalog.Resource(legacyReplacements)
	before, err := retirement.driver.Observe(ctx, resource, Receipt{})
	if err != nil || !before.Healthy {
		t.Fatal(before, err)
	}
	writeConfigFixture(t, inventory, "selected tools\n")
	after, err := retirement.driver.Observe(ctx, resource, Receipt{})
	if err != nil || !after.Healthy || !sameArtifact(before, after) {
		t.Fatal("retired global inventory invalidated unchanged replacement artifacts", before, after, err)
	}
	if before.Desired == after.Desired {
		t.Fatal("fresh approval no longer binds the full native inventory")
	}
}

func TestLegacyRetirementIntentMustMatchApprovedScope(t *testing.T) {
	d, _ := legacyStepFixture(t)
	command := nativeCommand{Program: "/usr/bin/env"}
	intent := legacyTestIntent(t, d, legacyRetirementStep{Kind: "command", Label: "fixture", Command: &command})
	receipt := Receipt{Ownership: "uncertain", Before: intent.Before, OperationID: intent.Operation, Status: "in-progress"}
	if _, err := d.readIntent("retire.tools", receipt); err != nil {
		t.Fatal(err)
	}
	intent.Scope.Detail = []string{"widened after approval"}
	if err := saveDocument(d.intentPath(intent.Operation), intent); err != nil {
		t.Fatal(err)
	}
	if _, err := d.readIntent("retire.tools", receipt); err == nil {
		t.Fatal("a saved scope that differs from approval was accepted")
	}
	o, err := d.Observe(context.Background(), Resource{ID: "retire.tools", Scope: "user"}, receipt)
	if err != nil || !o.Unknown || o.ResourceResume != nil || !strings.Contains(o.Pending, "unreadable") {
		t.Fatal(o, err)
	}
}

// The whole engine path: approval, durable intent, an interrupted step,
// recovery bound to the exact saved operation, and preserved personal data.
func TestLegacyPiRetirementThroughEngineWithResume(t *testing.T) {
	retirement, normalDriver, home := legacyEngineFixture(t)
	ctx := context.Background()
	c := retirement.driver.Normal.Catalog
	if _, err := Execute(ctx, c, linux, "source", home, normalDriver.statePath, approve(t, c, normalDriver, Request{Schema: 1, Mode: "apply", Selected: []string{}}), normalDriver); err != nil {
		t.Fatal(err)
	}
	pkg := filepath.Join(home, ".local", "lib", "node_modules", "@earendil-works", "pi-coding-agent")
	writeConfigFixture(t, filepath.Join(pkg, "package.json"), `{"name":"@earendil-works/pi-coding-agent","version":"1.0.4","bin":{"pi":"dist/cli.js"}}`)
	writeConfigFixture(t, filepath.Join(pkg, "dist", "cli.js"), "#!/usr/bin/env node\n")
	link := filepath.Join(home, ".local", "bin", "pi")
	if err := os.MkdirAll(filepath.Dir(link), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("../lib/node_modules/@earendil-works/pi-coding-agent/dist/cli.js", link); err != nil {
		t.Fatal(err)
	}
	personal := filepath.Join(home, ".pi", "agent", "settings.json")
	writeConfigFixture(t, personal, "{\"personal\":true}\n")
	scope, err := retirement.driver.inspect(ctx, "retire.pi")
	if err != nil || len(scope.Steps) != 3 || scope.Steps[0].Kind != "discard" || scope.Steps[1].Kind != "move" || scope.Steps[2].Kind != "rmdir" {
		t.Fatal(scope, err)
	}
	backup := scope.Steps[1].Backup
	writeConfigFixture(t, backup, "occupied")
	request := Request{Schema: 1, Mode: "apply", Selected: []string{"retire.pi"}}
	preview, err := retirement.Controller.Dispatch(ctx, request)
	if err != nil || preview.Status != "preview" || operation(t, preview.Plan, "retire.pi").Action != "install" || operation(t, preview.Plan, legacyReplacements).Action != "keep" {
		t.Fatal(preview, err)
	}
	if _, err := os.Lstat(link); err != nil {
		t.Fatal("preview changed the installation", err)
	}
	request.ExpectedPlan = preview.Plan.ID
	if result, err := retirement.Controller.Dispatch(ctx, request); err == nil || result.Status != "failed" {
		t.Fatal("an occupied preservation path must stop retirement", result, err)
	}
	if _, err := os.Lstat(link); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("the completed first step was not durable", err)
	}
	if _, err := os.Stat(filepath.Join(pkg, "package.json")); err != nil {
		t.Fatal("the blocked step moved the package", err)
	}
	if err := os.Remove(backup); err != nil {
		t.Fatal(err)
	}
	retry := Request{Schema: 1, Mode: "apply", Retry: true}
	ui := &scriptedInteraction{t: t, answers: [][]string{{"retry"}}}
	if result, err := retirement.Run(ctx, ui, LegacyRetirementRuntime); err != nil || result.Status != "cancelled" || len(ui.plans) != 1 || ui.plans[0].ResourceResume == nil {
		t.Fatal("interactive recovery did not review only the original retirement", result, err)
	}
	if len(ui.details) != 1 || !strings.Contains(ui.details[0], pkg+" -> "+backup) || strings.Contains(ui.details[0], scope.Steps[0].Label) {
		t.Fatal("recovery review omitted remaining scope or repeated a completed step", ui.details)
	}
	for round := 0; round < 2; round++ {
		retry.ExpectedPlan = ""
		preview, err := retirement.Controller.Dispatch(ctx, retry)
		if err != nil || round == 0 && preview.Plan.ResourceResume == nil {
			t.Fatal("recovery must resume the exact saved operation", preview, err)
		}
		retry.ExpectedPlan = preview.Plan.ID
		result, err := retirement.Controller.Dispatch(ctx, retry)
		if err != nil || round == 0 && result.Status != "needs-action" || round == 1 && result.Status != "ready" {
			t.Fatal(round, result, err)
		}
	}
	if data, err := os.ReadFile(filepath.Join(backup, "package.json")); err != nil || !strings.Contains(string(data), `"version":"1.0.4"`) {
		t.Fatal("the Pi package was not preserved intact", err)
	}
	if entries, err := os.ReadDir(filepath.Dir(pkg)); err != nil || len(entries) != 1 || entries[0].Name() != filepath.Base(backup) {
		t.Fatal("the npm scope must contain only the adjacent preserved package", entries, err)
	}
	if data, err := os.ReadFile(personal); err != nil || string(data) != "{\"personal\":true}\n" {
		t.Fatal("Pi personal data changed", err)
	}
	state, err := LoadState(retirement.Controller.StatePath, home)
	if err != nil || state.Transaction != nil || state.Receipts["retire.pi"].Ownership != "created" || state.Receipts["retire.pi"].Status != "ready" {
		t.Fatal(state, err)
	}
	check, err := retirement.Controller.Dispatch(ctx, Request{Schema: 1, Mode: "check"})
	if err != nil || check.Status != "ready" || operation(t, check.Plan, "retire.pi").Action != "keep" {
		t.Fatal("completed retirement must stay verified history", check, err)
	}
}

func TestInstalledMigrationPhaseIsExplicit(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("native Windows migration runs in its separately gated hosted fixture")
	}
	repository, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	home, err := resolveConfigPath(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	invoke := installedMachineFixture(t, repository, home, time.Minute, "DOTFILES_ENTRYPOINT=migrate", "DOTFILES_MIGRATION_PHASE=profiles")
	if result := invoke(Request{Schema: 1, Mode: "check"}); result.Status != "ready" {
		t.Fatal("the explicit profile phase keeps the released protocol", result)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	for _, environment := range [][]string{{"DOTFILES_ENTRYPOINT=setup", "DOTFILES_MIGRATION_PHASE=runtime"}, {"DOTFILES_ENTRYPOINT=migrate", "DOTFILES_MIGRATION_PHASE=everything"}} {
		if _, err := installedMachineRequest(ctx, filepath.Join(home, "dotfiles"), repository, home, Request{Schema: 1, Mode: "check"}, environment...); err == nil || !strings.Contains(err.Error(), "DOTFILES_MIGRATION_PHASE") {
			t.Fatal("an implicit or unknown migration phase was accepted", environment, err)
		}
	}
}

func TestLegacySentinelPolicyFixtureMatchesPin(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "released-sentinel-0.1.2-policy.md"))
	if err != nil || legacyHash(data) != legacySentinelPolicySHA256 || !strings.HasPrefix(string(data), "<!-- AGENT-RULES:BEGIN do-not-edit-inside-this-block -->\n<!-- version: 0.1.2 ") {
		t.Fatal("released Sentinel policy fixture differs from its pin", err)
	}
}

// Only exact released bytes are removed; edited files stay intact. OpenCode is
// a Sentinel host now, so its policy file stays even with the released bytes.
func TestLegacySentinelPolicyRetirementRemovesOnlyExactReleasedFiles(t *testing.T) {
	released, err := os.ReadFile(filepath.Join("testdata", "released-sentinel-0.1.2-policy.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, edited := range []bool{false, true} {
		t.Run(map[bool]string{false: "exact", true: "edited-after-preview"}[edited], func(t *testing.T) {
			retirement, normalDriver, home := legacyEngineFixture(t)
			ctx := context.Background()
			c := retirement.driver.Normal.Catalog
			if _, err := Execute(ctx, c, linux, "source", home, normalDriver.statePath, approve(t, c, normalDriver, Request{Schema: 1, Mode: "apply", Selected: []string{}}), normalDriver); err != nil {
				t.Fatal(err)
			}
			pi := filepath.Join(home, ".pi", "agent", "AGENTS.md")
			opencode := filepath.Join(home, ".config", "opencode", "AGENTS.md")
			writeConfigFixture(t, pi, string(released))
			writeConfigFixture(t, opencode, string(released))
			scope, err := retirement.driver.inspect(ctx, "retire.sentinel-policy")
			if err != nil || len(scope.Steps) != 1 || scope.Steps[0].Kind != "discard" || scope.Steps[0].Path != pi || len(scope.Retained) != 0 ||
				slices.ContainsFunc(scope.Detail, func(detail string) bool { return strings.Contains(detail, opencode) }) {
				t.Fatal("retirement did not select only the exact Pi policy file", scope, err)
			}
			request := Request{Schema: 1, Mode: "apply", Selected: []string{"retire.sentinel-policy"}}
			preview, err := retirement.Controller.Dispatch(ctx, request)
			if err != nil || preview.Status != "preview" || operation(t, preview.Plan, "retire.sentinel-policy").Action != "install" {
				t.Fatal(preview, err)
			}
			if edited {
				writeConfigFixture(t, pi, string(released)+"Personal addition.\n")
			}
			request.ExpectedPlan = preview.Plan.ID
			result, err := retirement.Controller.Dispatch(ctx, request)
			_, statErr := os.Stat(pi)
			if edited {
				if result.Status == "ready" || statErr != nil {
					t.Fatal("an edited policy file was removed", result, err, statErr)
				}
			} else if err != nil || result.Status != "ready" || !errors.Is(statErr, os.ErrNotExist) {
				t.Fatal("the exact released policy file was not removed", result, err, statErr)
			}
			if data, err := os.ReadFile(opencode); err != nil || string(data) != string(released) {
				t.Fatal("retirement changed the OpenCode policy file", err)
			}
		})
	}
}

func TestLegacySentinelPolicyRetirementKeepsMixedPiPolicy(t *testing.T) {
	released, err := os.ReadFile(filepath.Join("testdata", "released-sentinel-0.1.2-policy.md"))
	if err != nil {
		t.Fatal(err)
	}
	retirement, _, home := legacyEngineFixture(t)
	mixed := filepath.Join(home, ".pi", "agent", "AGENTS.md")
	writeConfigFixture(t, mixed, "# Personal Pi rules\n"+string(released))
	scope, err := retirement.driver.inspect(context.Background(), "retire.sentinel-policy")
	if err != nil || len(scope.Steps) != 0 || len(scope.Retained) != 0 ||
		!slices.ContainsFunc(scope.Detail, func(detail string) bool {
			return strings.Contains(detail, mixed) && strings.Contains(detail, "other text")
		}) {
		t.Fatal("retirement did not keep and name the mixed policy file", scope, err)
	}
	// A kept file for an unsupported agent is not unfinished migration.
	resource, _ := retirement.Controller.Catalog.Resource("retire.sentinel-policy")
	if o, err := retirement.driver.Observe(context.Background(), resource, Receipt{}); err != nil || !o.Healthy || o.ApplyBlocked != "" {
		t.Fatal("a kept mixed policy file blocked migration", o, err)
	}
	if data, err := os.ReadFile(mixed); err != nil || string(data) != "# Personal Pi rules\n"+string(released) {
		t.Fatal("a mixed policy file changed", err)
	}
}

func TestLegacySentinelPolicyRetirementKeepsLinksWithoutMislabelling(t *testing.T) {
	retirement, _, home := legacyEngineFixture(t)
	personal := filepath.Join(home, "rules.md")
	writeConfigFixture(t, personal, "# Personal rules without Sentinel\n")
	link := filepath.Join(home, ".pi", "agent", "AGENTS.md")
	if err := os.MkdirAll(filepath.Dir(link), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(personal, link); err != nil {
		t.Fatal(err)
	}
	scope, err := retirement.driver.inspect(context.Background(), "retire.sentinel-policy")
	if err != nil || len(scope.Steps) != 0 || len(scope.Retained) != 0 || len(scope.Detail) != 1 || !strings.Contains(scope.Detail[0], "not a plain file") {
		t.Fatal("a linked personal policy file was mislabelled or blocked", scope, err)
	}
}
