package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"

	engine "github.com/luisgui1757/dotfiles/installer"
	"github.com/luisgui1757/dotfiles/installer/terminal"
)

func main() {
	if handled, code := engine.RunWindowsRustLauncher(); handled {
		os.Exit(code)
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	if err := run(ctx); err != nil {
		fmt.Fprintln(os.Stderr, "Dotfiles:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	if len(os.Args) == 3 && os.Args[1] == "internal-legacy-system-text" {
		return engine.InspectLegacySystemText(os.Args[2], os.Stdout)
	}
	if len(os.Args) == 3 && os.Args[1] == "internal-legacy-root-status" {
		return engine.InspectLegacyRootStatus(os.Args[2], os.Stdout)
	}
	if len(os.Args) == 3 && os.Args[1] == "internal-native-worker" {
		return engine.RunNativeWorker(os.Args[2], os.Stdin, os.Stdout)
	}
	machine := len(os.Args) == 2 && os.Args[1] == "machine"
	if len(os.Args) != 1 && !machine {
		return errors.New("run dotfiles without arguments for installation, update, check or removal; automation uses 'dotfiles machine' with an explicit JSON request")
	}
	entrypoint := os.Getenv("DOTFILES_ENTRYPOINT")
	if entrypoint != "" && entrypoint != "setup" && entrypoint != "migrate" {
		return errors.New("unknown installer entrypoint; use setup or migrate from the checkout")
	}
	// Machine migration names its phase explicitly. An unset phase keeps the
	// released profile protocol; runtime retirement is a separate request.
	phase := os.Getenv("DOTFILES_MIGRATION_PHASE")
	if phase != "" && (entrypoint != "migrate" || !machine || phase != "profiles" && phase != "runtime") {
		return errors.New("DOTFILES_MIGRATION_PHASE is profiles or runtime and applies only to 'migrate machine'")
	}
	repository := os.Getenv("DOTFILES_CHECKOUT")
	if repository == "" {
		return errors.New("launch through the checkout's installer entrypoint; DOTFILES_CHECKOUT is missing")
	}
	absolute, err := filepath.Abs(repository)
	if err != nil {
		return err
	}
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	file, err := os.Open(executable)
	if err != nil {
		return err
	}
	hash := sha256.New()
	count, readErr := io.Copy(hash, io.LimitReader(file, 64<<20+1))
	if err := errors.Join(readErr, file.Close()); err != nil {
		return err
	}
	if count > 64<<20 {
		return errors.New("installer executable exceeds its identity bound")
	}
	source := hex.EncodeToString(hash.Sum(nil))
	platform, err := engine.DiscoverPlatform()
	if err != nil {
		return err
	}
	platform.SystemCommandDirectories, err = engine.DiscoverSystemCommandDirectories()
	if err != nil {
		return err
	}
	platform.Sentinel = engine.SentinelLocations{CodexHome: os.Getenv("CODEX_HOME"), ConfigHome: os.Getenv("XDG_CONFIG_HOME"), ClaudeConfigDir: os.Getenv("CLAUDE_CONFIG_DIR")}
	if platform.OS == "darwin" {
		discoverHomebrew(ctx, &platform)
	}
	if platform.OS == "linux" {
		platform.LinuxClipboard = engine.DiscoverLinuxClipboardCapability()
		platform.APT, err = engine.DiscoverLinuxAPT(ctx)
		if err != nil {
			platform.APTIssue = err.Error()
		}
		platform.DpkgDeb, err = exec.LookPath("dpkg-deb")
		if err != nil {
			return fmt.Errorf("Ubuntu/Debian requires its native archive decoder dpkg-deb: %w", err)
		}
	}
	if platform.OS == "windows" {
		platform.WindowsRustLauncher, platform.WindowsRustLauncherSHA256 = executable, source
		platform.WindowsPowerShell, err = engine.DiscoverWindowsPowerShell()
		if err != nil {
			platform.WindowsVendorIssue = err.Error()
		}
		platform.WindowsBuildToolsDirectory, err = engine.DiscoverWindowsBuildToolsDirectory()
		if err != nil {
			platform.WindowsVendorIssue = err.Error()
		}
	}
	folders, err := engine.DiscoverConfigFolders()
	if err != nil {
		return err
	}
	state, err := engine.DiscoverStateDirectory(platform.Context, folders)
	if err != nil {
		return err
	}
	build := func() (engine.Controller, *engine.LegacyRetirement, error) {
		c, err := engine.NewNativeController(absolute, source, state, platform, folders)
		if err != nil {
			return c, nil, err
		}
		configureAuthentication(c, platform, machine)
		// The released machine profile protocol never constructs retirement.
		if entrypoint != "migrate" || machine && phase != "runtime" {
			return c, nil, nil
		}
		retirement, err := engine.DiscoverLegacyRetirement(c, platform, folders, absolute)
		if err != nil {
			return c, nil, err
		}
		if !machine {
			retirement.SetAuthentication(foregroundSudo("administrator authentication failed"))
		}
		return c, retirement, nil
	}
	c, retirement, err := build()
	if err != nil {
		return err
	}
	var result engine.Result
	var migration *engine.LegacyMigration
	if entrypoint == "migrate" {
		migration, err = engine.NewLegacyMigration(absolute, source, state, platform.Context, folders)
		if err != nil {
			return err
		}
	}
	if machine {
		data, readErr := io.ReadAll(io.LimitReader(os.Stdin, 8<<20+1))
		if readErr != nil {
			return readErr
		}
		var request engine.Request
		if err := engine.Decode(data, &request); err != nil {
			return err
		}
		switch {
		case migration != nil && phase == "runtime":
			result, err = retirement.Dispatch(ctx, request)
		case migration != nil:
			result, err = migration.Dispatch(ctx, request)
		default:
			result, err = c.Dispatch(ctx, request)
			if err == nil {
				result.Message = strings.TrimSpace(result.Message + "\n" + legacyDisclosure(platform, folders))
			}
		}
		if err != nil {
			return err
		}
		if err := json.NewEncoder(os.Stdout).Encode(result); err != nil {
			return err
		}
	} else {
		ui := terminal.Workflow{Input: os.Stdin, Output: os.Stdout}
		if migration != nil {
			result, err = migrate(ctx, ui, migration, c, retirement, platform.OS == "darwin", func() (engine.Controller, *engine.LegacyRetirement, error) {
				// Setup must never inherit a Homebrew location from before conversion.
				discoverHomebrew(ctx, &platform)
				return build()
			})
		} else {
			result, err = c.Run(ctx, ui)
			if disclosure := legacyDisclosure(platform, folders); err == nil && disclosure != "" {
				_, err = fmt.Fprintln(os.Stdout, disclosure)
			}
		}
		if err != nil {
			return err
		}
	}
	if result.Status == "failed" || result.Status == "needs-action" {
		return errors.New(result.Message)
	}
	return nil
}

// A Homebrew prefix still run from the Nix store is visible to its resources
// as an issue, so setup never reuses it as native infrastructure.
func discoverHomebrew(ctx context.Context, platform *engine.NativePlatform) {
	location, err := engine.DiscoverHomebrew(ctx)
	platform.Homebrew, platform.HomebrewIssue = location, ""
	if err != nil {
		// A broken optional provider must be visible to its resources,
		// without preventing independent archive-only selections.
		platform.HomebrewIssue = err.Error()
	} else if location != nil {
		platform.HomebrewIssue = engine.LegacyNixHomebrewIssue(*location)
	}
}

func foregroundSudo(failure string) func(context.Context) error {
	return func(ctx context.Context) error {
		command := exec.CommandContext(ctx, "/usr/bin/sudo", "-v")
		command.Stdin, command.Stdout, command.Stderr = os.Stdin, os.Stdout, os.Stderr
		if err := command.Run(); err != nil {
			return fmt.Errorf("%s: %w", failure, err)
		}
		return nil
	}
}

func configureAuthentication(c engine.Controller, platform engine.NativePlatform, machine bool) {
	if machine {
		return
	}
	if platform.OS == "darwin" {
		authenticate := foregroundSudo("administrator authentication failed")
		c.Driver.(*engine.NativeDriver).Homebrew.Authenticate = authenticate
		c.Driver.(*engine.NativeDriver).AppleCLT.Authenticate = authenticate
	}
	if platform.APT != nil && platform.APT.Sudo != "" {
		c.Driver.(*engine.NativeDriver).APT.Authenticate = func(ctx context.Context) error {
			command := exec.CommandContext(ctx, platform.APT.Sudo, "-v")
			command.Stdin, command.Stdout, command.Stderr = os.Stdin, os.Stdout, os.Stderr
			if err := command.Run(); err != nil {
				return fmt.Errorf("APT administrator authentication failed: %w", err)
			}
			return nil
		}
	}
}

func legacyDisclosure(platform engine.NativePlatform, folders engine.ConfigFolders) string {
	hints := engine.LegacyInstallationHints(platform, folders)
	if len(hints) == 0 {
		return ""
	}
	return "Earlier dotfiles installation detected; run migrate to review its retirement:\n- " + strings.Join(hints, "\n- ")
}

// Interactive migration runs its phases in order and stops at the first one
// that is not ready: profiles and configuration links, Homebrew conversion on
// macOS, normal setup, a read-only check, runtime retirement, final check.
func migrate(ctx context.Context, ui terminal.Workflow, migration *engine.LegacyMigration, c engine.Controller, retirement *engine.LegacyRetirement, darwin bool,
	rebuild func() (engine.Controller, *engine.LegacyRetirement, error)) (engine.Result, error) {
	stopped := func(phase string, result engine.Result) engine.Result {
		if result.Status == "cancelled" {
			return result
		}
		return engine.Result{Status: "needs-action", Plan: result.Plan, Message: "Migration stopped after the " + phase + " phase; run migrate again to continue.\n" + result.Message}
	}
	result, err := migration.Run(ctx, ui)
	if err != nil || result.Status != "ready" {
		return stopped("profile and configuration preparation", result), err
	}
	if darwin {
		result, err = retirement.Run(ctx, ui, engine.LegacyRetirementPreparation)
		if err != nil || result.Status != "ready" {
			return stopped("Homebrew preparation", result), err
		}
		if c, retirement, err = rebuild(); err != nil {
			return engine.Result{Status: "failed"}, err
		}
	}
	result, err = c.Run(ctx, ui)
	if err != nil || result.Status != "ready" {
		return stopped("setup", result), err
	}
	for _, phase := range []string{"setup check", "runtime retirement", "final check"} {
		if phase == "runtime retirement" {
			result, err = retirement.Run(ctx, ui, engine.LegacyRetirementRuntime)
		} else {
			result, err = c.Dispatch(ctx, engine.Request{Schema: 1, Mode: "check"})
			if err == nil && result.Status != "ready" {
				err = ui.Report(result)
			}
		}
		if err != nil || result.Status != "ready" {
			return stopped(phase, result), err
		}
	}
	result = engine.Result{Status: "ready", Message: "Migration complete: earlier profiles, runtimes and tools are retired, and the replacement setup passed its final check."}
	return result, ui.Report(result)
}
