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

// LinuxAPTLocation is discovered only at the process boundary. Construction of
// an application with supplied locations performs no process or package work.
type LinuxAPTLocation struct {
	APTGet, Dpkg, DpkgQuery, APTMark, Sudo string
	root                                   bool
}

func DiscoverLinuxAPT(ctx context.Context) (*LinuxAPTLocation, error) {
	location := &LinuxAPTLocation{root: os.Geteuid() == 0}
	programs := map[string]*string{"apt-get": &location.APTGet, "dpkg": &location.Dpkg, "dpkg-query": &location.DpkgQuery, "apt-mark": &location.APTMark}
	if !location.root {
		programs["sudo"] = &location.Sudo
	}
	for name, target := range programs {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		path, err := exec.LookPath(name)
		if err != nil {
			if name == "sudo" {
				return nil, fmt.Errorf("APT needs administrator access: sudo is unavailable; an administrator must provide sudo or run the installer as root: %w", err)
			}
			return nil, fmt.Errorf("APT requires %s: %w", name, err)
		}
		*target = path
	}
	return location, location.validate()
}

func (location LinuxAPTLocation) validate() error {
	paths := []string{location.APTGet, location.Dpkg, location.DpkgQuery, location.APTMark}
	if !location.root {
		paths = append(paths, location.Sudo)
	}
	for _, path := range paths {
		if !filepath.IsAbs(path) || filepath.Clean(path) != path || strings.ContainsAny(path, "\x00\r\n") {
			return errors.New("APT requires canonical absolute native executable paths")
		}
	}
	return nil
}

func (location LinuxAPTLocation) query(ctx context.Context, privileged bool, program string, input []byte, args ...string) ([]byte, error) {
	if privileged || len(input) != 0 {
		return nil, errors.New("APT inspection cannot elevate or consume process input")
	}
	switch program {
	case "dpkg-query":
		program = location.DpkgQuery
	case "apt-mark":
		program = location.APTMark
	}
	if !filepath.IsAbs(program) || filepath.Clean(program) != program {
		return nil, errors.New("APT inspection requires an absolute executable")
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, program, args...)
	command.Env = append(os.Environ(), "LC_ALL=C")
	output, diagnostic := boundedCommandOutput{limit: 8 << 20}, boundedCommandOutput{}
	command.Stdout, command.Stderr = &output, &diagnostic
	if err := command.Run(); err != nil {
		return output.data.Bytes(), fmt.Errorf("APT inspection failed: %w%s", err, nativeDiagnostic(diagnostic.data.Bytes()))
	}
	return output.data.Bytes(), nil
}

func configureLinuxAPT(location *LinuxAPTLocation, catalog *Catalog, session *nativeSession) (*LinuxAPTDriver, error) {
	if location == nil {
		return nil, nil
	}
	if err := location.validate(); err != nil {
		return nil, err
	}
	if session == nil || !filepath.IsAbs(session.Directory) {
		return nil, errors.New("APT requires its native session")
	}
	d := &LinuxAPTDriver{Directory: filepath.Join(filepath.Dir(session.Directory), "apt"), WorkerDirectory: session.Directory,
		Location: *location, Packages: map[string]string{}, Checks: map[string][]string{}, Query: location.query, Run: session.run, approval: &linuxAPTApproval{}}
	for _, resource := range catalog.Resources {
		binding := resource.Bindings["linux"]
		if binding.Provider != "apt" {
			continue
		}
		d.Packages[resource.ID] = binding.Package
		var check []string
		switch binding.Package {
		case "git":
			check = []string{"/usr/bin/git", "--version"}
		case "zsh":
			check = []string{"/usr/bin/zsh", "--version"}
		case "tmux":
			check = []string{"/usr/bin/tmux", "-V"}
		case "build-essential":
			check = []string{"/usr/bin/cc", "--version"}
		case "make":
			check = []string{"/usr/bin/make", "--version"}
		case "clangd":
			check = []string{"/usr/bin/clangd", "--version"}
		case "curl":
			check = []string{"/usr/bin/curl", "--version"}
		case "unzip":
			check = []string{"/usr/bin/unzip", "-v"}
		case "xclip":
			check = []string{"/usr/bin/xclip", "-version"}
		case "wl-clipboard":
			check = []string{"/usr/bin/wl-copy", "--version"}
		case "fontconfig":
			check = []string{"/usr/bin/fc-cache", "--version"}
		default:
			if resource.Action != "apt-library" {
				return nil, fmt.Errorf("%s lacks its APT command health check", resource.ID)
			}
			// Shared libraries have no executable entrypoint. Verify their
			// package files; native application acceptance proves loader/ABI use.
			check = []string{location.Dpkg, "--verify", binding.Package}
		}
		d.Checks[resource.ID] = check
	}
	return d, nil
}
