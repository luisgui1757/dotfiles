package installer

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"os/exec"
	"os/user"
	"path"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"time"
	"unicode/utf8"
)

// DiscoverLegacyRetirement observes the account and user-selected tools at the
// process boundary. It runs no installer, never elevates and writes no state.
// System programs are fixed absolute paths, never resolved from PATH.
func DiscoverLegacyRetirement(normal Controller, platform NativePlatform, folders ConfigFolders, repository string) (*LegacyRetirement, error) {
	executable, err := os.Executable()
	if err != nil {
		return nil, err
	}
	account, err := user.Current()
	if err != nil {
		return nil, err
	}
	repository, err = resolveConfigPath(repository)
	if err != nil {
		return nil, err
	}
	environment := legacyRuntimeEnvironment{Executable: filepath.Clean(executable), Username: account.Username, UID: account.Uid, Programs: map[string]string{}}
	environment.Root = runtime.GOOS != "windows" && os.Geteuid() == 0
	if runtime.GOOS == "windows" {
		// Windows reports DOMAIN\user; tools and profile paths use the leaf name.
		environment.Username = account.Username[strings.LastIndex(account.Username, `\`)+1:]
	}
	environment.Cache = os.Getenv("XDG_CACHE_HOME")
	if environment.Cache == "" || !filepath.IsAbs(environment.Cache) {
		environment.Cache = filepath.Join(folders.Home, ".cache")
	}
	environment.Cache = filepath.Clean(environment.Cache)
	// GitHub CLI only evaluates gates saved by earlier development intents;
	// current inspection reads its extension manifest as data instead.
	if path, err := exec.LookPath("gh"); err == nil {
		if !filepath.IsAbs(path) {
			return nil, errors.New("gh resolved through a relative PATH entry")
		}
		environment.Programs["gh"] = filepath.Clean(path)
	} else if !errors.Is(err, exec.ErrNotFound) {
		return nil, err
	}
	if platform.OS == "linux" {
		// Linuxbrew is only observed for retirement. Normal Linux setup keeps
		// its declared APT provider and never receives this location.
		platform.Homebrew, err = discoverLegacyLinuxbrew(context.Background(), folders.Home)
		if err != nil {
			platform.HomebrewIssue = err.Error()
		}
	}
	git, err := discoverLegacyGit(normal, platform)
	if err != nil {
		return nil, err
	}
	if git != "" {
		environment.Programs["git"] = git
	}
	return newLegacyRetirement(normal, platform, folders, repository, environment)
}

func discoverLegacyLinuxbrew(ctx context.Context, home string) (*HomebrewLocation, error) {
	for _, path := range []string{"/home/linuxbrew/.linuxbrew/bin/brew", filepath.Join(home, ".linuxbrew", "bin", "brew"), "/usr/local/bin/brew", "/opt/homebrew/bin/brew"} {
		location, err := discoverHomebrew(ctx, path)
		if location != nil || err != nil {
			return location, err
		}
	}
	return nil, nil
}

// Inspection is read-only. A nonzero exit keeps its structured exit code so
// absence can be distinguished from failure without parsing prose.
func queryLegacyRuntime(ctx context.Context, command nativeCommand) ([]byte, error) {
	if err := command.validate(); err != nil || command.Operation != "" || len(command.Input) != 0 {
		return nil, errors.Join(errors.New("legacy inspection requires a bounded read-only command"), err)
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	process := exec.CommandContext(ctx, command.Program, command.Arguments...)
	process.Env = append(os.Environ(), command.Environment...)
	output, diagnostic := boundedCommandOutput{limit: 8 << 20}, boundedCommandOutput{limit: 64 << 10}
	process.Stdout, process.Stderr = &output, &diagnostic
	err := process.Run()
	if err == nil {
		return output.data.Bytes(), nil
	}
	code := -1
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		code = exit.ExitCode()
	}
	return output.data.Bytes(), &nativeCommandError{Message: fmt.Sprintf("legacy inspection %s failed: %v%s", filepath.Base(command.Program), errors.Join(err, ctx.Err()), nativeDiagnostic(diagnostic.data.Bytes())), ExitCode: &code}
}

func legacyExitCode(err error) (int, bool) {
	var native *nativeCommandError
	if errors.As(err, &native) && native.ExitCode != nil {
		return *native.ExitCode, true
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return exit.ExitCode(), true
	}
	return 0, false
}

// Privileged inspection uses existing foreground credentials only.
func (d *legacyRetirementDriver) privilegedQuery(ctx context.Context, command nativeCommand) ([]byte, error) {
	if !d.Environment.Root {
		command.Arguments = append([]string{"-n", "--", command.Program}, command.Arguments...)
		command.Program = "/usr/bin/sudo"
	}
	return d.Query(ctx, command)
}

// Read a small regular file as data. The snapshot binds its bytes and mode;
// links and special files are refused rather than followed.
func readLegacyFile(path string, limit int64) ([]byte, configSnapshot, error) {
	snapshot, err := snapshotTree(path, limit, 1)
	if err != nil || snapshot.Kind == "absent" {
		return nil, snapshot, err
	}
	if snapshot.Kind != "file" {
		return nil, snapshot, fmt.Errorf("%s is not a regular file", path)
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, snapshot, err
	}
	data, readErr := io.ReadAll(io.LimitReader(file, limit+1))
	if err := errors.Join(readErr, file.Close()); err != nil {
		return nil, snapshot, err
	}
	if int64(len(data)) > limit {
		return nil, snapshot, fmt.Errorf("%s exceeds its read bound", path)
	}
	if err := verifyConfigSnapshot(path, snapshot); err != nil {
		return nil, snapshot, err
	}
	return data, snapshot, nil
}

func legacyFileSHA256(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	hash := sha256.New()
	_, copyErr := io.Copy(hash, io.LimitReader(file, 1<<30))
	if err := errors.Join(copyErr, file.Close()); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func (d *legacyRetirementDriver) inspect(ctx context.Context, id string) (legacyRetirementScope, error) {
	var scope legacyRetirementScope
	if err := ctx.Err(); err != nil {
		return scope, err
	}
	var err error
	switch id {
	case "retire.homebrew":
		err = d.inspectNixHomebrew(ctx, &scope)
	case "retire.chezmoi":
		err = d.inspectChezmoi(ctx, &scope)
	case "retire.gh-dash":
		err = d.inspectGhDash(ctx, &scope)
	case "retire.pi":
		err = d.inspectPi(&scope)
	case "retire.sentinel-policy":
		err = d.inspectSentinelPolicy(&scope)
	case "retire.tools":
		err = d.inspectTools(ctx, &scope)
	case "retire.nix":
		err = d.inspectNix(ctx, &scope)
	default:
		if !strings.HasPrefix(id, legacyDeltaPrefix) {
			return scope, fmt.Errorf("unknown legacy retirement resource %q", id)
		}
		err = d.inspectManagerDelta(ctx, id, &scope)
	}
	if err != nil {
		return legacyRetirementScope{}, err
	}
	if scope.Blocked != "" {
		scope.Steps = nil // A hard block authorizes no partial work.
	}
	if id != "retire.homebrew" {
		scope.ReplacementState, err = d.replacementState()
		if err != nil {
			return legacyRetirementScope{}, err
		}
	}
	if len(scope.Steps) > legacyStepLimit {
		return legacyRetirementScope{}, errors.New("legacy retirement exceeds its reviewed step bound")
	}
	for _, step := range scope.Steps {
		if err := d.validateStep(step); err != nil {
			return legacyRetirementScope{}, err
		}
	}
	return scope, nil
}

func (d *legacyRetirementDriver) disclosePrerequisite(scope *legacyRetirementScope, resource, text string) error {
	selected, err := d.replacementSelected(resource)
	if err != nil {
		return err
	}
	if !selected {
		scope.Detail = append(scope.Detail, text)
	}
	return nil
}

// legacyArtifactPin is one direct-artifact pin from the pre-overhaul main or published v0.4.4.
// TestLegacyRetirementReleasedPinsMatchReleaseSource compares these with the
// immutable release blob; markers from other releases are not recognized.
type legacyArtifactPin struct{ Tool, Arch, Version, URL, SHA256, Commit string }

const legacyReleaseCommit = "ae9a6446eb0a837a144d77d2a6345db967c61e4e"
const legacyStableCommit = "05874e536372f6a73f8971c84e675e95666662d4"

var legacyReleasedArtifacts = []legacyArtifactPin{
	{"nvim", "amd64", "v0.12.5", "https://github.com/neovim/neovim/releases/download/v0.12.5/nvim-linux-x86_64.tar.gz", "bce0f56eda1f1b1db6eee8f4133d7a38813ea07933837dd1777411ca384c6875", legacyReleaseCommit},
	{"nvim", "arm64", "v0.12.5", "https://github.com/neovim/neovim/releases/download/v0.12.5/nvim-linux-arm64.tar.gz", "1aa5ca085249580ae0f91eb14f27ec0919773ff2d99a163d03f3d6c21ac29725", legacyReleaseCommit},
	{"chezmoi", "amd64", "v2.73.0", "https://github.com/twpayne/chezmoi/releases/download/v2.73.0/chezmoi_2.73.0_linux_amd64.tar.gz", "b597729b687af4488a848240134cb633de8ca0f04e0d26d48f400ee2ac338ffa", legacyReleaseCommit},
	{"chezmoi", "arm64", "v2.73.0", "https://github.com/twpayne/chezmoi/releases/download/v2.73.0/chezmoi_2.73.0_linux_arm64.tar.gz", "abcb840401d3c1f2356e0f53f5d52aa10d10f572654d9626db9ad0ca4dc03355", legacyReleaseCommit},
	{"lazygit", "amd64", "v0.66.0", "https://github.com/jesseduffield/lazygit/releases/download/v0.66.0/lazygit_0.66.0_linux_x86_64.tar.gz", "5b45541155d20bd32bf2cc5ab5b7e3d91c2eebf0fb1242281350edc27d59d2b7", legacyReleaseCommit},
	{"lazygit", "arm64", "v0.66.0", "https://github.com/jesseduffield/lazygit/releases/download/v0.66.0/lazygit_0.66.0_linux_arm64.tar.gz", "9a4fc4656897ac9f7877b835473ce1a75620cc267f554c57fc4ff266407f3257", legacyReleaseCommit},
	{"starship", "amd64", "v1.26.0", "https://github.com/starship/starship/releases/download/v1.26.0/starship-x86_64-unknown-linux-gnu.tar.gz", "321f0dd7af8340a5f2e6a8fec6538a04f617486f9ec70d878f91c09cd8deef22", legacyReleaseCommit},
	{"starship", "arm64", "v1.26.0", "https://github.com/starship/starship/releases/download/v1.26.0/starship-aarch64-unknown-linux-musl.tar.gz", "dc30189378d2f2e287384e8a692d3f95ad1df64cf0e8c36aa9201516028aed6b", legacyReleaseCommit},
	{"tree-sitter", "amd64", "v0.27.0", "https://github.com/tree-sitter/tree-sitter/releases/download/v0.27.0/tree-sitter-cli-linux-x64.zip", "e4a3826bcd0fe099ee3a5617767374939cbc23c4a35b5b53f5fc04142525a2c1", legacyReleaseCommit},
	{"tree-sitter", "arm64", "v0.27.0", "https://github.com/tree-sitter/tree-sitter/releases/download/v0.27.0/tree-sitter-cli-linux-arm64.zip", "6260b621bf5ab87027dfb463bf955504ef32cdcda62b81f28447753e48c83a62", legacyReleaseCommit},
	{"herdr", "amd64", "v0.9.3", "https://github.com/ogulcancelik/herdr/releases/download/v0.9.3/herdr-linux-x86_64", "18a8dc65f1c2fa485884344356dea1cfd911c6f06cf46fa78e193f4087f4dba7", legacyReleaseCommit},
	{"herdr", "arm64", "v0.9.3", "https://github.com/ogulcancelik/herdr/releases/download/v0.9.3/herdr-linux-aarch64", "4de7aa3e25678812e92960de64f7c2aaa1bca1f0f80a3c5e559837e231e1f5c0", legacyReleaseCommit},
	{"nvim", "amd64", "v0.12.4", "https://github.com/neovim/neovim/releases/download/v0.12.4/nvim-linux-x86_64.tar.gz", "012bf3fcac5ade43914df3f174668bf64d05e049a4f032a388c027b1ebd78628", legacyStableCommit},
	{"nvim", "arm64", "v0.12.4", "https://github.com/neovim/neovim/releases/download/v0.12.4/nvim-linux-arm64.tar.gz", "ceb7e88c6b681f0515d135dcdfad54f5eb4373b25ce6172197cd9a69c758063f", legacyStableCommit},
	{"chezmoi", "amd64", "v2.71.1", "https://github.com/twpayne/chezmoi/releases/download/v2.71.1/chezmoi_2.71.1_linux_amd64.tar.gz", "e1fb16c962644d57f4d451c324aa86163d00faf5d035500f41fb48943a66dfed", legacyStableCommit},
	{"chezmoi", "arm64", "v2.71.1", "https://github.com/twpayne/chezmoi/releases/download/v2.71.1/chezmoi_2.71.1_linux_arm64.tar.gz", "6e88c8150d3d54533ba2f335a52c2ac7b67259c525ba0f19091fc078b6852154", legacyStableCommit},
	{"lazygit", "amd64", "v0.63.1", "https://github.com/jesseduffield/lazygit/releases/download/v0.63.1/lazygit_0.63.1_linux_x86_64.tar.gz", "8e033bc78c8e192dee9510e951f6c9e154289b7198d22c924ed1d0a951b0dac1", legacyStableCommit},
	{"lazygit", "arm64", "v0.63.1", "https://github.com/jesseduffield/lazygit/releases/download/v0.63.1/lazygit_0.63.1_linux_arm64.tar.gz", "555dbc9a8efcf2e33bc24e7fbd9463e9fa375e3c5e23cc270763733c38eeae36", legacyStableCommit},
	{"starship", "amd64", "v1.26.0", "https://github.com/starship/starship/releases/download/v1.26.0/starship-x86_64-unknown-linux-gnu.tar.gz", "321f0dd7af8340a5f2e6a8fec6538a04f617486f9ec70d878f91c09cd8deef22", legacyStableCommit},
	{"starship", "arm64", "v1.26.0", "https://github.com/starship/starship/releases/download/v1.26.0/starship-aarch64-unknown-linux-musl.tar.gz", "dc30189378d2f2e287384e8a692d3f95ad1df64cf0e8c36aa9201516028aed6b", legacyStableCommit},
	{"tree-sitter", "amd64", "v0.26.11", "https://github.com/tree-sitter/tree-sitter/releases/download/v0.26.11/tree-sitter-cli-linux-x64.zip", "ff1b7f9863f2faafd78dc0e66d902ee85b37f709b314b22c009f51caf233eebd", legacyStableCommit},
	{"tree-sitter", "arm64", "v0.26.11", "https://github.com/tree-sitter/tree-sitter/releases/download/v0.26.11/tree-sitter-cli-linux-arm64.zip", "db28509fe6db8902f9d14c43c486858c7486b42c3a96b30e811e73f105762336", legacyStableCommit},
	{"herdr", "amd64", "v0.7.5", "https://github.com/ogulcancelik/herdr/releases/download/v0.7.5/herdr-linux-x86_64", "3dc83288073e4c2d3c679a30e7be97bcca9141c6fd17dbbb9219142e95c59253", legacyStableCommit},
	{"herdr", "arm64", "v0.7.5", "https://github.com/ogulcancelik/herdr/releases/download/v0.7.5/herdr-linux-aarch64", "32e763a1499a6b694b1d708e4f062b743be1da9f34fcfa4d212d6db6fe09a8b9", legacyStableCommit},
}

// Replacement resources are disclosure only: an unselected one means the
// command disappears when its earlier copy is retired.
var legacyToolReplacements = map[string]string{"nvim": "tool.nvim", "lazygit": "tool.lazygit", "starship": "tool.starship", "tree-sitter": "tool.tree-sitter", "herdr": "tool.herdr"}

var legacyMarkerKeys = []string{"binary_path", "binary_sha256", "command_path", "install_root", "schema", "sha256", "source_url", "tool", "version"}

// Provenance markers are parsed as data, never sourced. Released readers took
// the first key; duplicate or unknown keys are refused here.
func parseLegacyMarker(data []byte) (map[string]string, error) {
	values := map[string]string{}
	if len(data) == 0 || data[len(data)-1] != '\n' {
		return nil, errors.New("provenance marker is not newline-terminated")
	}
	for _, line := range strings.Split(strings.TrimSuffix(string(data), "\n"), "\n") {
		key, value, ok := strings.Cut(line, "=")
		if !ok || !slices.Contains(legacyMarkerKeys, key) || strings.ContainsAny(line, "\r\x00") {
			return nil, errors.New("invalid provenance marker line")
		}
		if _, duplicate := values[key]; duplicate {
			return nil, errors.New("duplicate provenance marker key")
		}
		values[key] = value
	}
	if len(values) != len(legacyMarkerKeys) || values["schema"] != "2" {
		return nil, errors.New("provenance marker is not the released schema 2 shape")
	}
	return values, nil
}

// legacyDirectArtifact returns the exact released shape of one marker, or a
// retained reason. Paths, pins and the current binary hash must all match.
func (d *legacyRetirementDriver) legacyDirectArtifact(tool string, marker map[string]string) (string, error) {
	home := d.Folders.Home
	pinned := slices.ContainsFunc(legacyReleasedArtifacts, func(pin legacyArtifactPin) bool {
		return pin.Tool == tool && pin.Arch == d.Platform.Arch && pin.Version == marker["version"] && pin.URL == marker["source_url"] && pin.SHA256 == marker["sha256"]
	})
	if marker["tool"] != tool || !pinned {
		return "its provenance marker is not a recognized pre-overhaul pin", nil
	}
	local := filepath.Join(home, ".local", "bin")
	arch := map[string]string{"amd64": "x86_64", "arm64": "arm64"}[d.Platform.Arch]
	shape := false
	switch tool {
	case "nvim":
		root := "/opt/nvim-linux-" + arch
		shape = marker["command_path"] == "/usr/local/bin/nvim" && marker["binary_path"] == root+"/bin/nvim" && marker["install_root"] == root
	case "lazygit", "starship":
		shape = marker["install_root"] == "/usr/local/bin" && marker["command_path"] == "/usr/local/bin/"+tool && marker["binary_path"] == "/usr/local/bin/"+tool ||
			marker["install_root"] == local && marker["command_path"] == filepath.Join(local, tool) && marker["binary_path"] == filepath.Join(local, tool)
	case "tree-sitter", "chezmoi", "herdr":
		shape = marker["install_root"] == local && marker["command_path"] == filepath.Join(local, tool) && marker["binary_path"] == filepath.Join(local, tool)
	}
	if !shape || tool == "herdr" && marker["sha256"] != marker["binary_sha256"] {
		return "its provenance marker does not describe the released install shape", nil
	}
	info, err := os.Lstat(marker["binary_path"])
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "its recorded binary is no longer a regular file", nil
	}
	actual, err := legacyFileSHA256(marker["binary_path"])
	if err != nil {
		return "", err
	}
	if actual != marker["binary_sha256"] {
		return "its binary changed after the released installer recorded it", nil
	}
	if marker["command_path"] != marker["binary_path"] {
		target, err := os.Readlink(marker["command_path"])
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
		if err == nil && target != marker["binary_path"] {
			return "its command link points elsewhere", nil
		}
	}
	return "", nil
}

// Plan removal of one marker-proved direct artifact and its marker.
func (d *legacyRetirementDriver) planDirectArtifact(scope *legacyRetirementScope, tool string) error {
	markerPath := filepath.Join(d.Folders.Home, ".local", "share", "dotfiles", "provenance", tool+".env")
	data, _, err := readLegacyFile(markerPath, 64<<10)
	if errors.Is(err, os.ErrNotExist) || err == nil && data == nil {
		return nil
	}
	if err != nil {
		scope.Retained = append(scope.Retained, tool+": provenance marker is unreadable: "+err.Error())
		return nil
	}
	marker, err := parseLegacyMarker(data)
	if err != nil {
		scope.Retained = append(scope.Retained, tool+": "+err.Error())
		return nil
	}
	reason, err := d.legacyDirectArtifact(tool, marker)
	if err != nil {
		return err
	}
	if reason != "" {
		scope.Retained = append(scope.Retained, tool+" at "+marker["binary_path"]+": "+reason+"; it was preserved")
		return nil
	}
	label := "Remove released " + tool
	if _, err := os.Lstat(marker["binary_path"]); err == nil {
		switch {
		case tool == "nvim":
			if err := addLegacyRemoval(scope, label+" command link", marker["command_path"]); err != nil {
				return err
			}
			if err := d.planNvimTree(scope, marker["install_root"]); err != nil {
				return err
			}
		case strings.HasPrefix(marker["binary_path"], "/usr/local/bin/"):
			if err := addLegacyRemoval(scope, label, marker["binary_path"]); err != nil {
				return err
			}
		default:
			if err := addLegacyDiscard(scope, label, marker["binary_path"]); err != nil {
				return err
			}
		}
	}
	if replacement := legacyToolReplacements[tool]; replacement != "" {
		if err := d.disclosePrerequisite(scope, replacement, tool+" is not selected in setup; its command disappears after retirement."); err != nil {
			return err
		}
	}
	return addLegacyDiscard(scope, "Remove the released "+tool+" provenance marker", markerPath)
}

// The release tarball unpacks only bin/nvim plus lib and share trees. The
// root-owned tree is recorded before approval and rechecked before removal.
func (d *legacyRetirementDriver) planNvimTree(scope *legacyRetirementScope, root string) error {
	entries, err := readPlainDirectory(root)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if !slices.Contains([]string{"bin", "lib", "share"}, entry.Name()) || !entry.IsDir() {
			scope.Retained = append(scope.Retained, root+" contains "+entry.Name()+", which is not part of the released Neovim tarball")
			return nil
		}
	}
	binaries, err := readPlainDirectory(filepath.Join(root, "bin"))
	if err != nil {
		return err
	}
	if len(binaries) != 1 || binaries[0].Name() != "nvim" {
		scope.Retained = append(scope.Retained, root+"/bin contains files other than the released nvim binary")
		return nil
	}
	tree, err := legacyFile(root)
	if err != nil {
		scope.Retained = append(scope.Retained, root+" could not be recorded for exact removal: "+err.Error())
		return nil
	}
	command := d.systemCommand(true, "/bin/rm", "-rf", "--one-file-system", "--", root)
	scope.Steps = append(scope.Steps, legacyRetirementStep{Kind: "command", Label: "Remove the released Neovim tree " + root, Privileged: true,
		Before: []legacyRetirementFile{tree}, Absent: []string{root}, Command: &command})
	return nil
}

type legacyCheckout struct{ Path, Origin, Commit, Entry, Replacement string }

func (d *legacyRetirementDriver) legacyCheckouts() []legacyCheckout {
	share := filepath.Join(d.Folders.Home, ".local", "share", "dotfiles")
	tmux := filepath.Join(share, "tmux-plugins")
	zsh := filepath.Join(share, "zsh-plugins")
	return []legacyCheckout{
		{filepath.Join(tmux, "tpm"), "https://github.com/tmux-plugins/tpm", "e261deb1b47614eed3400089ce7197dc68acc4eb", "tpm", "tmux.plugins"},
		{filepath.Join(tmux, "tmux-sensible"), "https://github.com/tmux-plugins/tmux-sensible", "25cb91f42d020f675bb0a2ce3fbd3a5d96119efa", "sensible.tmux", "tmux.plugins"},
		{filepath.Join(tmux, "tmux-yank"), "https://github.com/tmux-plugins/tmux-yank", "acfd36e4fcba99f8310a7dfb432111c242fe7392", "yank.tmux", "tmux.plugins"},
		{filepath.Join(tmux, "tmux-resurrect"), "https://github.com/tmux-plugins/tmux-resurrect", "cff343cf9e81983d3da0c8562b01616f12e8d548", "resurrect.tmux", "tmux.plugins"},
		{filepath.Join(tmux, "tmux-continuum"), "https://github.com/tmux-plugins/tmux-continuum", "0698e8f4b17d6454c71bf5212895ec055c578da0", "continuum.tmux", "tmux.plugins"},
		{filepath.Join(zsh, "fzf-tab"), "https://github.com/Aloxaf/fzf-tab", "d7e0234614dbe5369fdd760907d12c0e05a4dccc", "fzf-tab.plugin.zsh", "zsh.plugins"},
		{filepath.Join(zsh, "zsh-autosuggestions"), "https://github.com/zsh-users/zsh-autosuggestions", "e52ee8ca55bcc56a17c828767a3f98f22a68d4eb", "zsh-autosuggestions.zsh", "zsh.plugins"},
		{filepath.Join(share, "sentinel", "ecafffa858666343c1639f996d177f460163e93e"), "https://github.com/luisgui1757/sentinel", "ecafffa858666343c1639f996d177f460163e93e", "", ""},
	}
}

func (d *legacyRetirementDriver) git(ctx context.Context, directory string, arguments ...string) ([]byte, error) {
	program := d.Environment.Programs["git"]
	if program == "" {
		return nil, errors.New("git is required to prove a pinned checkout")
	}
	values := []string{"-i", "PATH=/usr/bin:/bin:/usr/sbin:/sbin", "LC_ALL=C", "HOME=" + d.Folders.Home, "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null",
		"GIT_OPTIONAL_LOCKS=0", "GIT_TERMINAL_PROMPT=0", program, "-c", "core.hooksPath=/dev/null", "-c", "core.fsmonitor=false",
		"-c", "core.untrackedCache=false", "-c", "credential.helper=", "-C", directory}
	return d.Query(ctx, nativeCommand{Program: "/usr/bin/env", Arguments: append(values, arguments...)})
}

// Homebrew runs with its reviewed process controls and no inherited settings.
func (d *legacyRetirementDriver) brewCommand(program string, arguments ...string) nativeCommand {
	values := []string{"-i", "PATH=/usr/bin:/bin:/usr/sbin:/sbin", "HOME=" + d.Folders.Home, "USER=" + d.Environment.Username, "LOGNAME=" + d.Environment.Username}
	values = append(append(values, brewProcessControls...), program)
	return nativeCommand{Program: "/usr/bin/env", Arguments: append(values, arguments...)}
}

// Only an exact, clean, pinned checkout with its released origin can retire.
// Git runs without optional locks so inspection never rewrites the index.
func (d *legacyRetirementDriver) planCheckout(ctx context.Context, scope *legacyRetirementScope, checkout legacyCheckout) error {
	if _, err := os.Lstat(checkout.Path); errors.Is(err, os.ErrNotExist) {
		return nil
	} else if err != nil {
		return err
	}
	resolved, err := resolveConfigPath(checkout.Path)
	if err != nil {
		return err
	}
	top, err := d.git(ctx, checkout.Path, "rev-parse", "--show-toplevel")
	if err != nil || strings.TrimSuffix(string(top), "\n") != resolved {
		scope.Retained = append(scope.Retained, legacyReason(checkout.Path+" is not its own Git checkout; it was preserved", err))
		return nil
	}
	if err := d.checkGitInspection(ctx, checkout.Path, true); err != nil {
		scope.Retained = append(scope.Retained, checkout.Path+" could not be inspected safely: "+err.Error())
		return nil
	}
	head, headErr := d.git(ctx, checkout.Path, "rev-parse", "--verify", "HEAD^{commit}")
	origin, originErr := d.git(ctx, checkout.Path, "remote", "get-url", "origin")
	if err := errors.Join(headErr, originErr); err != nil {
		scope.Retained = append(scope.Retained, checkout.Path+" could not be proved clean: "+err.Error())
		return nil
	}
	if strings.TrimSuffix(string(head), "\n") != checkout.Commit || strings.TrimSuffix(strings.TrimSuffix(string(origin), "\n"), ".git") != checkout.Origin {
		scope.Retained = append(scope.Retained, checkout.Path+" is not the exact clean released checkout; it was preserved")
		return nil
	}
	status, err := d.git(ctx, checkout.Path, "status", "--porcelain=v1", "--untracked-files=all", "--ignored")
	if err != nil || len(status) != 0 {
		scope.Retained = append(scope.Retained, legacyReason(checkout.Path+" is not the exact clean released checkout; it was preserved", err))
		return nil
	}
	if checkout.Entry != "" {
		if info, err := os.Lstat(filepath.Join(checkout.Path, checkout.Entry)); err != nil || !info.Mode().IsRegular() {
			scope.Retained = append(scope.Retained, checkout.Path+" lacks its released plugin entry; it was preserved")
			return nil
		}
	}
	if checkout.Replacement != "" {
		if err := d.disclosePrerequisite(scope, checkout.Replacement, filepath.Base(checkout.Path)+" is not replaced by a selected setup tool; configuration that loads it will stop loading it."); err != nil {
			return err
		}
	}
	return addLegacyDiscard(scope, "Remove the exact clean pinned checkout", checkout.Path)
}

func (d *legacyRetirementDriver) inspectTools(ctx context.Context, scope *legacyRetirementScope) error {
	if d.Platform.OS == "linux" {
		for _, tool := range []string{"nvim", "lazygit", "starship", "tree-sitter", "herdr"} {
			if err := d.planDirectArtifact(scope, tool); err != nil {
				return err
			}
		}
	}
	for _, checkout := range d.legacyCheckouts() {
		if err := d.planCheckout(ctx, scope, checkout); err != nil {
			return err
		}
	}
	share := filepath.Join(d.Folders.Home, ".local", "share", "dotfiles")
	if _, err := d.planPylatexencVenv(scope, filepath.Join(share, "python-tools", "pylatexenc")); err != nil {
		return err
	}
	if len(scope.Steps) == 0 {
		return nil
	}
	scope.Detail = append(scope.Detail, "Session snapshots, personal tmux/zsh files and other plugins stay.")
	for _, directory := range []string{filepath.Join(share, "tmux-plugins"), filepath.Join(share, "zsh-plugins"), filepath.Join(share, "sentinel"), filepath.Join(share, "provenance"), share} {
		if info, err := os.Lstat(directory); err == nil && info.IsDir() {
			scope.Steps = append(scope.Steps, legacyRetirementStep{Kind: "rmdir", Label: "Remove the empty released directory " + directory, Path: directory})
		}
	}
	return nil
}

// legacySentinelPolicySHA256 is the exact Sentinel 0.1.2 block, as a whole
// file, that the released setup wrote for OpenCode and Pi. Current Sentinel
// and dotfiles support neither consumer. testdata holds the bytes; the opt-in
// source test reproduces them from the released commit's renderer.
const legacySentinelPolicySHA256 = "efeb284196e99ee049718c1a4cd8a2090cdd41a7ce8be304cb158fc8a4d16801"

// Only a file holding exactly the released policy is removed, using the
// released installer's default locations. Anything else is kept and named:
// these agents are unsupported, so a kept file is never unfinished migration
// work. Personal files are not read beyond a bounded check for the marker.
func (d *legacyRetirementDriver) inspectSentinelPolicy(scope *legacyRetirementScope) error {
	config := d.Platform.Sentinel.ConfigHome
	if config == "" {
		config = filepath.Join(d.Folders.Home, ".config")
	}
	for _, path := range []string{filepath.Join(config, "opencode", "AGENTS.md"), filepath.Join(d.Folders.Home, ".pi", "agent", "AGENTS.md")} {
		entries, err := inspectTree(path, maxProfileBytes, 1)
		if err != nil {
			scope.Detail = append(scope.Detail, legacyReason(path+" could not be inspected as one policy file and stays unchanged", err))
			continue
		}
		if len(entries) == 0 {
			continue
		}
		if entries[0].Kind != "file" {
			scope.Detail = append(scope.Detail, path+" is not a plain file and stays unchanged.")
			continue
		}
		if entries[0].Content != legacySentinelPolicySHA256 {
			data, _, err := readLegacyFile(path, maxProfileBytes)
			if err != nil {
				scope.Detail = append(scope.Detail, legacyReason(path+" could not be read and stays unchanged", err))
			} else if bytes.Contains(data, []byte("<!-- AGENT-RULES:BEGIN")) {
				scope.Detail = append(scope.Detail, path+" holds other text besides the released Sentinel policy and stays unchanged; remove its AGENT-RULES block by hand if that agent no longer needs it.")
			}
			continue
		}
		scope.Steps = append(scope.Steps, legacyRetirementStep{Kind: "discard", Label: "Remove the released Sentinel policy file: " + path, Path: path, Entries: entries, Absent: []string{path}})
	}
	if len(scope.Steps) != 0 {
		scope.Detail = append(scope.Detail, "Other OpenCode and Pi files stay; uninstall those agents separately if you no longer use them.")
	}
	return nil
}

var legacyPiPackages = []string{"pi-coding-agent", "pi-agent-core", "pi-ai", "pi-tui", "pi-mcp", "pi-codemode", "chord", "pi-telemetry"}

// Pi packages are moved aside from the active npm package names intact. POSIX
// command links are removed only when they resolve into a moved package; npm's
// Windows cmd/ps1/sh shims only when every package path they name is that
// package's own command. ~/.pi is never touched.
func (d *legacyRetirementDriver) inspectPi(scope *legacyRetirementScope) error {
	if d.Platform.OS != "windows" {
		local := filepath.Join(d.Folders.Home, ".local")
		return d.planPiPackages(scope, filepath.Join(local, "lib", "node_modules", "@earendil-works"), func(directory, name, _ string) ([]string, string) {
			link := filepath.Join(local, "bin", name)
			target, err := os.Readlink(link)
			if errors.Is(err, os.ErrNotExist) {
				return nil, ""
			}
			if err != nil {
				return nil, link + " is not a Pi command link; it was preserved"
			}
			if !filepath.IsAbs(target) {
				target = filepath.Join(filepath.Dir(link), target)
			}
			if relative, err := filepath.Rel(directory, filepath.Clean(target)); err != nil || !filepath.IsLocal(relative) {
				return nil, link + " points outside the released Pi package; it was preserved"
			}
			return []string{link}, ""
		})
	}
	prefixes, note := d.windowsNpmPrefixes()
	if note != "" {
		scope.Detail = append(scope.Detail, note)
	}
	for _, prefix := range prefixes {
		before := len(scope.Steps)
		err := d.planPiPackages(scope, filepath.Join(prefix, "node_modules", "@earendil-works"), func(directory, name, target string) ([]string, string) {
			return windowsPiShims(prefix, directory, name, target)
		})
		if err != nil {
			return err
		}
		if len(scope.Steps) != before {
			scope.Detail = append(scope.Detail, "The npm global prefix "+prefix+" stays on the User PATH: Node and other global packages use it.")
		}
	}
	return nil
}

// The npm prefixes the released `npm install -g` could have used: Node's
// default %AppData%\npm, Scoop nodejs-lts's persisted prefix, and an absolute
// prefix in the user's own .npmrc, read as data.
func (d *legacyRetirementDriver) windowsNpmPrefixes() ([]string, string) {
	prefixes := []string{filepath.Join(d.scoopRoot(), "persist", "nodejs-lts", "bin")}
	if filepath.IsAbs(d.Folders.AppData) {
		prefixes = append(prefixes, filepath.Join(d.Folders.AppData, "npm"))
	}
	note := ""
	userconfig := filepath.Join(d.Folders.Home, ".npmrc")
	data, _, err := readLegacyFile(userconfig, 64<<10)
	if err != nil {
		note = legacyReason(userconfig+" could not be read as data, so a custom npm prefix it may set was not checked for Pi", err)
	}
	for _, line := range strings.Split(string(data), "\n") {
		key, value, ok := strings.Cut(strings.TrimSpace(line), "=")
		value = strings.Trim(strings.TrimSpace(value), `"'`)
		if ok && strings.EqualFold(strings.TrimSpace(key), "prefix") && filepath.IsAbs(value) {
			prefixes = append(prefixes, filepath.Clean(value))
		}
	}
	unique := []string{}
	for _, prefix := range prefixes {
		if !slices.ContainsFunc(unique, func(known string) bool { return strings.EqualFold(known, prefix) }) {
			unique = append(unique, prefix)
		}
	}
	return unique, note
}

// npm's cmd-shim writes name, name.cmd and name.ps1 beside node_modules. Each
// must be a small regular file whose every package path is this command.
func windowsPiShims(prefix, directory, name, target string) ([]string, string) {
	expected := "node_modules/@earendil-works/" + filepath.Base(directory) + "/" + strings.TrimPrefix(path.Clean(strings.ReplaceAll(target, `\`, "/")), "./")
	shims := []string{}
	for _, shim := range []string{name, name + ".cmd", name + ".ps1"} {
		file := filepath.Join(prefix, shim)
		info, err := os.Lstat(file)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil || !info.Mode().IsRegular() {
			return nil, legacyReason(file+" is not a regular npm command shim; it was preserved", err)
		}
		data, _, err := readLegacyFile(file, 64<<10)
		if err != nil {
			return nil, legacyReason(file+" could not be read as an npm command shim; it was preserved", err)
		}
		text := strings.ReplaceAll(string(data), `\`, "/")
		if count := strings.Count(text, "node_modules/"); count == 0 || strings.Count(text, expected) != count {
			return nil, file + " does not run only the released Pi package's " + name + " command; it was preserved"
		}
		shims = append(shims, file)
	}
	return shims, ""
}

// planPiPackages moves the exact released Pi packages below namespace aside.
// command reports the removable command artifacts, or a refusal, per bin entry.
func (d *legacyRetirementDriver) planPiPackages(scope *legacyRetirementScope, namespace string, command func(directory, name, target string) ([]string, string)) error {
	entries, err := readPlainDirectory(namespace)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	before := len(scope.Steps)
	moves, links, everything := []string{}, []string{}, true
	for _, entry := range entries {
		directory := filepath.Join(namespace, entry.Name())
		if !slices.Contains(legacyPiPackages, entry.Name()) {
			everything = false
			continue
		}
		data, _, err := readLegacyFile(filepath.Join(directory, "package.json"), 1<<20)
		if err != nil || data == nil {
			scope.Retained = append(scope.Retained, legacyReason(directory+" has no readable package metadata; it was preserved", err))
			everything = false
			continue
		}
		var metadata struct {
			Name    string          `json:"name"`
			Version string          `json:"version"`
			Bin     json.RawMessage `json:"bin"`
		}
		if err := json.Unmarshal(data, &metadata); err != nil || metadata.Name != "@earendil-works/"+entry.Name() || !slices.Contains([]string{"1.0.4", "0.82.1"}, metadata.Version) {
			scope.Retained = append(scope.Retained, directory+" is not a recognized released Pi package; it was preserved")
			everything = false
			continue
		}
		commands := map[string]string{}
		if len(metadata.Bin) != 0 && string(metadata.Bin) != "null" {
			var single string
			if json.Unmarshal(metadata.Bin, &single) == nil {
				commands[entry.Name()] = single
			} else if err := json.Unmarshal(metadata.Bin, &commands); err != nil {
				scope.Retained = append(scope.Retained, directory+" declares unreadable command links; it was preserved")
				everything = false
				continue
			}
		}
		for _, name := range slices.Sorted(maps.Keys(commands)) {
			if name == "" || strings.ContainsAny(name, `/\:`) || name == "." || name == ".." {
				return errors.New("invalid Pi package command name")
			}
			removable, refusal := command(directory, name, commands[name])
			if refusal != "" {
				scope.Retained = append(scope.Retained, refusal)
			}
			links = append(links, removable...)
		}
		moves = append(moves, directory)
	}
	for _, link := range sortedUnique(links) {
		if err := addLegacyDiscard(scope, "Remove the Pi command link", link); err != nil {
			return err
		}
	}
	for _, directory := range moves {
		if err := d.addUserMove(scope, "Move the released Pi package aside from the active npm package names", directory); err != nil {
			return err
		}
	}
	if len(moves) != 0 && everything {
		scope.Steps = append(scope.Steps, legacyRetirementStep{Kind: "rmdir", Label: "Remove the empty @earendil-works npm scope", Path: namespace})
	}
	if len(scope.Steps) != before {
		scope.Detail = append(scope.Detail, "Pi data in "+filepath.Join(d.Folders.Home, ".pi")+" stays. Claude Code and Codex are the supported replacements.")
	}
	return nil
}

// GitHub CLI evaluates only "gh-extension" gates saved by earlier development
// intents, whose approved value was this released `gh extension list` row.
// Current retirement reads the released extension as data below.
func (d *legacyRetirementDriver) ghCommand(arguments ...string) (nativeCommand, error) {
	program := d.Environment.Programs["gh"]
	if program == "" {
		return nativeCommand{}, errors.New("GitHub CLI is required")
	}
	return nativeCommand{Program: program, Arguments: arguments, Environment: []string{"GH_PROMPT_DISABLED=1", "GH_NO_UPDATE_NOTIFIER=1", "NO_COLOR=1", "GH_PAGER=", "PAGER=cat"}}, nil
}

func (d *legacyRetirementDriver) ghExtension(ctx context.Context, name string) (string, error) {
	command, err := d.ghCommand("extension", "list")
	if err != nil {
		return "", err
	}
	output, err := d.Query(ctx, command)
	if err != nil {
		return "", err
	}
	found := []string{}
	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		if strings.HasPrefix(line, "gh "+name+"\t") {
			found = append(found, strings.TrimRight(line, "\r"))
		}
	}
	if len(found) > 1 {
		return "", fmt.Errorf("GitHub CLI reports %d extensions named %s", len(found), name)
	}
	if len(found) == 0 {
		return "absent", nil
	}
	return found[0], nil
}

// GitHub CLI v2 keeps a binary extension in <DataDir>/extensions/gh-<name>,
// holding only its executable and the manifest.yml written by Install
// (cli/cli v2.102.0 pkg/cmd/extension/manager.go). `gh extension remove`
// deletes exactly that directory. Retirement moves it aside instead, so it
// needs no GitHub CLI and keeps working after an earlier gh is retired.
var legacyGhDashTags = []string{"v4.26.0", "v4.25.2"}

// ghDataDirectory mirrors go-gh v2.16.1 config.DataDir: XDG_DATA_HOME, then
// LocalAppData on Windows, then ~/.local/share.
func (d *legacyRetirementDriver) ghDataDirectory() (string, error) {
	if value := os.Getenv("XDG_DATA_HOME"); value != "" {
		if !filepath.IsAbs(value) {
			return "", fmt.Errorf("XDG_DATA_HOME %q is relative, so GitHub CLI's extension directory depends on its working directory", value)
		}
		return filepath.Join(filepath.Clean(value), "gh"), nil
	}
	if value := os.Getenv("LocalAppData"); d.Platform.OS == "windows" && value != "" {
		if !filepath.IsAbs(value) {
			return "", fmt.Errorf("LocalAppData %q is relative", value)
		}
		return filepath.Join(filepath.Clean(value), "GitHub CLI"), nil
	}
	return filepath.Join(d.Folders.Home, ".local", "share", "gh"), nil
}

// parseLegacyGhManifest reads the six fields of gh's binManifest as data.
func parseLegacyGhManifest(data []byte) (map[string]string, error) {
	if len(data) == 0 || !utf8.Valid(data) || strings.ContainsAny(string(data), "\x00\r") {
		return nil, errors.New("the manifest is not plain YAML text")
	}
	values := map[string]string{}
	for _, line := range strings.Split(strings.TrimSuffix(string(data), "\n"), "\n") {
		key, raw, ok := strings.Cut(line, ": ")
		if !ok || !slices.Contains([]string{"owner", "name", "host", "tag", "ispinned", "path"}, key) {
			return nil, fmt.Errorf("unexpected manifest line %q", truncateLegacyText(line, 128))
		}
		if _, duplicate := values[key]; duplicate {
			return nil, fmt.Errorf("duplicate manifest key %s", key)
		}
		value, err := legacyYAMLScalar(strings.TrimSpace(raw))
		if err != nil {
			return nil, fmt.Errorf("manifest %s: %w", key, err)
		}
		values[key] = value
	}
	if len(values) != 6 {
		return nil, errors.New("the manifest lacks a binManifest field")
	}
	return values, nil
}

func (d *legacyRetirementDriver) inspectGhDash(_ context.Context, scope *legacyRetirementScope) error {
	data, err := d.ghDataDirectory()
	if err != nil {
		scope.Retained = append(scope.Retained, legacyReason("GitHub CLI's extension directory cannot be located, so the released gh dash extension was not checked", err))
		return nil
	}
	extensions := filepath.Join(data, "extensions")
	directory := filepath.Join(extensions, "gh-dash")
	info, err := os.Lstat(directory)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if !info.IsDir() || info.Mode()&(os.ModeSymlink|os.ModeIrregular) != 0 {
		scope.Retained = append(scope.Retained, directory+" is a local or linked gh extension, not the released binary extension; it was preserved")
		return nil
	}
	binary := "gh-dash"
	if d.Platform.OS == "windows" {
		binary += ".exe"
	}
	entries, err := readPlainDirectory(directory)
	if err != nil {
		return err
	}
	names := []string{}
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	slices.Sort(names)
	if want := []string{binary, "manifest.yml"}; !slices.Equal(names, want) {
		scope.Retained = append(scope.Retained, directory+" contains "+strings.Join(names, ", ")+" rather than only the released binary and its manifest; it was preserved")
		return nil
	}
	if executable, err := os.Lstat(filepath.Join(directory, binary)); err != nil || !executable.Mode().IsRegular() {
		scope.Retained = append(scope.Retained, legacyReason(directory+" has no regular extension executable; it was preserved", err))
		return nil
	}
	manifestData, _, err := readLegacyFile(filepath.Join(directory, "manifest.yml"), 64<<10)
	if err != nil {
		scope.Retained = append(scope.Retained, legacyReason(directory+" has no readable manifest; it was preserved", err))
		return nil
	}
	manifest, err := parseLegacyGhManifest(manifestData)
	path := strings.ReplaceAll(manifest["path"], `\`, "/")
	if err != nil || manifest["owner"] != "dlvhdr" || manifest["name"] != "gh-dash" || manifest["host"] != "github.com" || manifest["ispinned"] != "true" ||
		!slices.Contains(legacyGhDashTags, manifest["tag"]) || path[strings.LastIndex(path, "/")+1:] != binary {
		scope.Retained = append(scope.Retained, legacyReason(directory+" is not a released dlvhdr/gh-dash pin ("+strings.Join(legacyGhDashTags, " or ")+"); it was preserved", err))
		return nil
	}
	tree, err := snapshotTree(directory, maxConfigBytes, maxConfigEntries)
	if err != nil {
		scope.Retained = append(scope.Retained, legacyReason(directory+" could not be recorded; it was preserved", err))
		return nil
	}
	before := len(scope.Steps)
	if err := d.addUserMove(scope, "Move the released gh dash extension "+manifest["tag"]+" aside (GitHub CLI no longer lists it)", directory); err != nil || len(scope.Steps) == before {
		return err
	}
	scope.Steps[before].Gates = append(scope.Steps[before].Gates, legacyRetirementGate{Kind: "tree", Subject: directory, Expected: tree.Hash})
	scope.Detail = append(scope.Detail, "Other GitHub CLI extensions, gh-dash configuration and GitHub CLI's update-check notes stay.")
	return d.disclosePrerequisite(scope, "tool.gh-dash", "gh-dash is not selected in setup; the dashboard command disappears after retirement.")
}

func (d *legacyRetirementDriver) chezmoiConfigDirectory() string {
	return filepath.Join(d.chezmoiBaseDirectory("XDG_CONFIG_HOME", ".config"), "chezmoi")
}

// chezmoi is retired only when its source is absent or a dotfiles checkout,
// which stays in place. Configuration and state may hold personal data and
// are moved aside.
func (d *legacyRetirementDriver) inspectChezmoi(ctx context.Context, scope *legacyRetirementScope) error {
	config := d.chezmoiConfigDirectory()
	if err := d.inspectChezmoiSource(ctx, scope, config); err != nil || scope.Blocked != "" {
		return err
	}
	installed := false
	switch d.Platform.OS {
	case "linux":
		before := len(scope.Steps)
		if err := d.planDirectArtifact(scope, "chezmoi"); err != nil {
			return err
		}
		installed = len(scope.Steps) != before
		found, err := d.planBrewPackages(ctx, scope, legacyChezmoiPackages)
		if err != nil {
			return err
		}
		installed = installed || found
	case "darwin":
		var err error
		installed, err = d.planBrewPackages(ctx, scope, legacyChezmoiPackages)
		if err != nil {
			return err
		}
	case "windows":
		var err error
		installed, err = d.planWindowsPackages(ctx, scope, legacyChezmoiPackages)
		if err != nil {
			return err
		}
	}
	if !installed {
		path, err := exec.LookPath("chezmoi")
		if err == nil {
			scope.Retained = append(scope.Retained, "chezmoi at "+path+" has no recognized released installer evidence; it was preserved")
		} else if !errors.Is(err, exec.ErrNotFound) {
			return err
		}
	}
	for _, path := range []string{config, filepath.Join(d.Environment.Cache, "chezmoi")} {
		if err := d.addUserMove(scope, "Move chezmoi configuration, state or cache aside", path); err != nil {
			return err
		}
	}
	if len(scope.Steps) != 0 {
		scope.Detail = append(scope.Detail, "chezmoi purge and destroy are never run. Managed targets keep their current contents.")
	}
	return nil
}
