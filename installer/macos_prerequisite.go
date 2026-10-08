package installer

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// MacOSPrerequisiteDriver reuses Apple's selected developer tools and clipboard
// commands. It never owns, updates, switches or removes Xcode, CLT or OS files.
// Fresh Homebrew bootstrap may provision CLT; observations must then run again.
// Missing CLT behind an existing manager requires its separate prerequisite
// provisioning action, not rerunning Homebrew over an existing prefix.
type MacOSPrerequisiteDriver struct {
	Query             nativeCommandRunner
	clipboardCommands []string
}

func configureMacOSPrerequisites(platform NativePlatform) (*MacOSPrerequisiteDriver, error) {
	if platform.OS != "darwin" {
		return nil, nil
	}
	if err := platform.Context.Validate(); err != nil {
		return nil, err
	}
	return &MacOSPrerequisiteDriver{Query: macOSPrerequisiteQuery, clipboardCommands: []string{"/usr/bin/pbcopy", "/usr/bin/pbpaste"}}, nil
}

// These are inspection commands only. In particular, never invoke Apple's
// /usr/bin compiler shims while no developer directory is selected: that can
// display an installation dialog. xcrun resolves the selected real executable.
func macOSPrerequisiteQuery(ctx context.Context, privileged bool, program string, input []byte, args ...string) ([]byte, error) {
	if privileged || len(input) != 0 || !filepath.IsAbs(program) || filepath.Clean(program) != program {
		return nil, errors.New("Apple prerequisite inspection cannot elevate, run shell input or use relative executables")
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, program, args...)
	// Inspect the system-selected toolchain consistently. Caller SDK/toolchain
	// overrides must not redirect xcrun away from xcode-select's directory.
	for _, value := range os.Environ() {
		key, _, _ := strings.Cut(value, "=")
		if key != "DEVELOPER_DIR" && key != "SDKROOT" && key != "TOOLCHAINS" && key != "LC_ALL" {
			command.Env = append(command.Env, value)
		}
	}
	command.Env = append(command.Env, "LC_ALL=C")
	output, diagnostic := boundedCommandOutput{limit: 64 << 10}, boundedCommandOutput{}
	command.Stdout, command.Stderr = &output, &diagnostic
	if err := command.Run(); err != nil {
		if program == "/usr/bin/xcode-select" && len(args) == 1 && args[0] == "--print-path" && appleCLTSelectionMissing(err, output.data.Bytes(), diagnostic.data.Bytes()) {
			return nil, fmt.Errorf("%w: %w", errAppleCLTNoSelection, err)
		}
		return nil, fmt.Errorf("Apple prerequisite inspection failed: %w%s", err, nativeDiagnostic(diagnostic.data.Bytes()))
	}
	return output.data.Bytes(), nil
}

func (d *MacOSPrerequisiteDriver) Observe(ctx context.Context, r Resource, _ Receipt) (Observation, error) {
	if err := ctx.Err(); err != nil {
		return Observation{}, err
	}
	if d.Query == nil {
		return Observation{}, errors.New("Apple prerequisite driver requires a read-only query boundary")
	}
	switch r.ID {
	case "tool.compiler", "tool.make":
		return d.toolchain(ctx, r.ID)
	case "tool.clipboard":
		return d.clipboard(ctx)
	default:
		return Observation{}, errors.New("resource is not an Apple prerequisite")
	}
}

func (d *MacOSPrerequisiteDriver) toolchain(ctx context.Context, id string) (Observation, error) {
	o := Observation{Provider: "operating-system", Identity: "apple-developer-tools/" + strings.TrimPrefix(id, "tool."), Scope: "machine"}
	data, err := d.Query(ctx, false, "/usr/bin/xcode-select", nil, "--print-path")
	if err != nil {
		if ctx.Err() != nil {
			return o, ctx.Err()
		}
		o.Unknown, o.Pending = true, "Apple developer tools are not selected; install Command Line Tools with xcode-select --install, or select your intended Xcode with xcode-select --switch, then check again"
		return o, nil
	}
	selected, err := macOSDeveloperPath(data)
	if err != nil {
		return o, err
	}
	root, err := resolveConfigPath(selected)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return o, err
		}
		o.Unknown, o.Pending = true, "the selected Apple developer directory is missing; install Command Line Tools or select your existing Xcode, then check again"
		return o, nil
	}
	info, err := os.Stat(root)
	if err != nil || !info.IsDir() {
		return o, errors.Join(errors.New("selected Apple developer path is not a directory"), err)
	}
	tools := []string{"make"}
	if id == "tool.compiler" {
		tools = []string{"clang", "clang++"}
	}
	evidence := map[string]string{"developer_directory": root}
	o.Fingerprint, err = digest(evidence)
	if err != nil {
		return o, err
	}
	for _, name := range tools {
		data, err := d.Query(ctx, false, "/usr/bin/xcrun", nil, "--no-cache", "--find", name)
		if err != nil {
			if ctx.Err() != nil {
				return o, ctx.Err()
			}
			o.Unknown, o.Pending = true, "the selected Apple developer tools cannot resolve "+name+"; complete any Xcode license/setup action or repair the selected tools, then check again"
			return o, nil
		}
		path, err := macOSDeveloperPath(data)
		if err != nil {
			return o, err
		}
		path, err = resolveConfigPath(path)
		if err != nil {
			return o, err
		}
		relative, err := filepath.Rel(root, path)
		if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.IsAbs(relative) {
			return o, errors.Join(errors.New("xcrun resolved a command outside its selected developer directory"), err)
		}
		info, err := os.Stat(path)
		if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0111 == 0 {
			return o, errors.Join(errors.New("selected Apple developer command is missing or not executable"), err)
		}
		data, err = d.Query(ctx, false, path, nil, "--version")
		if ctx.Err() != nil {
			return o, ctx.Err()
		}
		prefix := "Apple clang version "
		if name == "make" {
			prefix = "GNU Make "
		}
		if err != nil || !strings.HasPrefix(string(data), prefix) {
			o.Present, o.ApplyBlocked = true, "the selected Apple "+name+" command failed its native health check; repair the selected developer tools"
			if err != nil {
				o.HealthIssue = err.Error()
			}
			return o, nil
		}
		version, _, _ := strings.Cut(string(data), "\n")
		evidence[name], evidence[name+"_version"] = path, version
	}
	if id == "tool.compiler" {
		data, err := d.Query(ctx, false, "/usr/bin/xcrun", nil, "--no-cache", "--sdk", "macosx", "--show-sdk-path")
		if err != nil {
			if ctx.Err() != nil {
				return o, ctx.Err()
			}
			o.Present, o.ApplyBlocked = true, "the selected Apple compiler has no usable macOS SDK; complete or repair its developer-tools installation"
			return o, nil
		}
		sdk, err := macOSDeveloperPath(data)
		if err != nil {
			return o, err
		}
		info, err := os.Stat(sdk)
		if err != nil || !info.IsDir() {
			o.Present, o.ApplyBlocked = true, "the selected Apple macOS SDK directory is missing; repair the selected developer tools"
			return o, nil
		}
		evidence["sdk"] = sdk
	}
	o.Fingerprint, err = digest(evidence)
	o.Present, o.Healthy = true, true
	return o, err
}

func macOSDeveloperPath(data []byte) (string, error) {
	path := strings.TrimSuffix(string(data), "\n")
	if len(path) > 4096 || path == string(filepath.Separator) || !filepath.IsAbs(path) || filepath.Clean(path) != path || strings.ContainsAny(path, "\x00\r\n") {
		return "", errors.New("Apple developer tools returned an invalid absolute path")
	}
	return path, nil
}

func (d *MacOSPrerequisiteDriver) clipboard(ctx context.Context) (Observation, error) {
	o := Observation{Provider: "operating-system", Identity: "macos-pbcopy-pbpaste", Scope: "machine"}
	if len(d.clipboardCommands) != 2 {
		return o, errors.New("macOS clipboard requires both OS commands")
	}
	evidence := map[string]string{}
	for _, path := range d.clipboardCommands {
		if err := ctx.Err(); err != nil {
			return o, err
		}
		snapshot, err := snapshotTree(path, 8<<20, 1)
		if err != nil {
			return o, err
		}
		info, err := os.Stat(path)
		if snapshot.Kind != "file" || err != nil || info.Mode().Perm()&0111 == 0 {
			o.ApplyBlocked = "macOS pbcopy/pbpaste is missing or damaged; repair the operating-system component"
			return o, nil
		}
		evidence[path] = snapshot.Hash
	}
	var err error
	o.Fingerprint, err = digest(evidence)
	o.Present, o.Healthy = true, true
	o.UnverifiedApplications = []string{"macOS graphical-session clipboard round-trip (clipboard contents were not read or changed)"}
	return o, err
}

func (*MacOSPrerequisiteDriver) Apply(context.Context, Resource, Operation, Receipt) (Observation, error) {
	return Observation{}, errors.New("Apple developer tools and clipboard commands are reused infrastructure; install missing CLT through its prerequisite action")
}
func (*MacOSPrerequisiteDriver) Remove(context.Context, Resource, Receipt) (Observation, error) {
	return Observation{}, errors.New("dotfiles cannot remove Apple's developer tools or clipboard commands")
}
