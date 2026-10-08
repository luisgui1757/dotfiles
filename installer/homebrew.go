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

// HomebrewLocation is observed at the process boundary, never inferred from
// architecture or the user's home. It carries no inherited environment.
type HomebrewLocation struct {
	Program, Prefix, Cellar string
}

func (location HomebrewLocation) validate() error {
	for _, path := range []string{location.Program, location.Prefix, location.Cellar} {
		if !filepath.IsAbs(path) || filepath.Clean(path) != path || strings.ContainsAny(path, "\x00\r\n:") {
			return errors.New("Homebrew paths must be canonical absolute POSIX paths")
		}
	}
	return nil
}

func DiscoverHomebrew(ctx context.Context) (*HomebrewLocation, error) {
	// The supported macOS bootstrap has one declared prefix. Inspect it when
	// shell initialization has not added brew to PATH yet, then read its actual
	// prefix and Cellar just as for a PATH-discovered installation.
	return discoverHomebrew(ctx, "/opt/homebrew/bin/brew")
}

func discoverHomebrew(ctx context.Context, bootstrapProgram string) (*HomebrewLocation, error) {
	program, err := exec.LookPath("brew")
	if errors.Is(err, exec.ErrNotFound) {
		if _, err := os.Lstat(bootstrapProgram); errors.Is(err, os.ErrNotExist) {
			return nil, nil
		} else if err != nil {
			return nil, err
		}
		return inspectHomebrew(ctx, bootstrapProgram)
	}
	if err != nil {
		return nil, err
	}
	return inspectHomebrew(ctx, program)
}

func inspectHomebrew(ctx context.Context, program string) (*HomebrewLocation, error) {
	location := &HomebrewLocation{Program: program}
	for option, destination := range map[string]*string{"--prefix": &location.Prefix, "--cellar": &location.Cellar} {
		data, err := location.query(ctx, false, "brew", nil, option)
		if err != nil {
			return nil, fmt.Errorf("discover Homebrew %s: %w", option, err)
		}
		*destination = strings.TrimSuffix(string(data), "\n")
	}
	if err := location.validate(); err != nil {
		return nil, err
	}
	return location, nil
}

func (location HomebrewLocation) query(ctx context.Context, privileged bool, program string, input []byte, args ...string) ([]byte, error) {
	if privileged || len(input) != 0 {
		return nil, errors.New("Homebrew inspection cannot elevate or consume process input")
	}
	brew := program == "brew"
	if brew {
		program = location.Program
	}
	if !filepath.IsAbs(program) || filepath.Clean(program) != program {
		return nil, errors.New("native inspection requires an absolute executable")
	}
	limit := 30 * time.Second
	if brew && len(args) > 0 && args[0] == "linkage" {
		// One affected-set inspection may traverse many existing consumers.
		// The caller's earlier deadline or cancellation still wins.
		limit = 5 * time.Minute
	}
	ctx, cancel := context.WithTimeout(ctx, limit)
	defer cancel()
	command := exec.CommandContext(ctx, program, args...)
	command.Env = append(os.Environ(), brewProcessControls...)
	if brew && len(args) > 0 && args[0] == "linkage" {
		command.Env = append(command.Env, "HOMEBREW_DEV_CMD_RUN=1")
	}
	output, diagnostic := boundedCommandOutput{limit: 8 << 20}, boundedCommandOutput{}
	command.Stdout, command.Stderr = &output, &diagnostic
	if err := command.Run(); err != nil {
		// exec reports a killed process as ExitError; retain the deadline cause
		// so incomplete inspection cannot be mistaken for broken linkage.
		return output.data.Bytes(), fmt.Errorf("native inspection failed: %w%s", errors.Join(err, ctx.Err()), nativeDiagnostic(diagnostic.data.Bytes()))
	}
	return output.data.Bytes(), nil
}

func configureHomebrew(platform NativePlatform, catalog *Catalog, session *nativeSession) (*BrewDriver, error) {
	location := platform.Homebrew
	if location == nil {
		return nil, nil
	}
	if platform.OS != "darwin" {
		return nil, errors.New("Homebrew is not the declared native provider on this platform")
	}
	if err := location.validate(); err != nil {
		return nil, err
	}
	d := &BrewDriver{Directory: filepath.Dir(session.Directory), Program: location.Program, Cellar: location.Cellar,
		Packages: map[string]string{}, Checks: map[string][]string{}, Environment: slices.Clone(brewProcessControls), Query: location.query, Run: session.run, approval: &brewApproval{}}
	for _, resource := range catalog.Resources {
		binding := resource.Bindings[platform.OS]
		if binding.Provider != "homebrew-formula" {
			continue
		}
		d.Packages[resource.ID] = binding.Package
		switch resource.ID {
		case "tool.git":
			d.Checks[resource.ID] = []string{filepath.Join(location.Prefix, "bin", "git"), "--version"}
		case "tool.tmux":
			d.Checks[resource.ID] = []string{filepath.Join(location.Prefix, "bin", "tmux"), "-V"}
		default:
			return nil, fmt.Errorf("%s lacks its native command health check", resource.ID)
		}
	}
	return d, nil
}

func homebrewCommandDirectories(platform NativePlatform, catalog *Catalog, selected []string) []string {
	if platform.Homebrew != nil {
		for _, id := range selected {
			if resource, ok := catalog.Resource(id); ok && resource.Bindings[platform.OS].Provider == "homebrew-formula" {
				return []string{filepath.Join(platform.Homebrew.Prefix, "bin")}
			}
		}
	}
	return nil
}

// Existing package infrastructure is reused and never acquired or removed by
// selecting a tool. Fresh infrastructure creation has a separate bootstrap path.
type existingHomebrewDriver struct{ HomebrewLocation }

func (d existingHomebrewDriver) Observe(ctx context.Context, _ Resource, _ Receipt) (Observation, error) {
	o := Observation{Provider: "homebrew-infrastructure", Identity: d.Prefix, Scope: "machine"}
	if err := d.validate(); err != nil {
		return o, err
	}
	info, err := os.Stat(d.Program)
	if os.IsNotExist(err) {
		o.ApplyBlocked = "the observed Homebrew executable disappeared; rediscover the native provider"
		return o, nil
	}
	if err != nil {
		return o, err
	}
	o.Present = true
	o.Fingerprint, err = digest(d.HomebrewLocation)
	if err != nil {
		return o, err
	}
	output, checkErr := d.query(ctx, false, "brew", nil, "--version")
	o.Healthy = info.Mode().IsRegular() && info.Mode().Perm()&0111 != 0 && checkErr == nil && strings.HasPrefix(string(output), "Homebrew ")
	if !o.Healthy {
		o.ApplyBlocked = "the existing Homebrew installation needs repair"
		if checkErr != nil {
			o.HealthIssue = checkErr.Error()
		}
	}
	return o, nil
}

func (existingHomebrewDriver) Apply(context.Context, Resource, Operation, Receipt) (Observation, error) {
	return Observation{}, errors.New("existing Homebrew infrastructure is reused, not overwritten")
}

func (existingHomebrewDriver) Remove(context.Context, Resource, Receipt) (Observation, error) {
	return Observation{}, errors.New("package removal cannot remove Homebrew infrastructure")
}
