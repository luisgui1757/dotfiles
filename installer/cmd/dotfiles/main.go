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
		if ctx.Err() != nil && errors.Is(err, context.Canceled) {
			fmt.Fprintln(os.Stderr, "Dotfiles: interrupted")
			os.Exit(130)
		}
		fmt.Fprintln(os.Stderr, "Dotfiles:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context) (resultErr error) {
	if len(os.Args) == 3 && os.Args[1] == "internal-native-worker" {
		return engine.RunNativeWorker(os.Args[2], os.Stdin, os.Stdout)
	}
	machine := len(os.Args) == 2 && os.Args[1] == "machine"
	if len(os.Args) != 1 && !machine {
		return errors.New("run dotfiles without arguments for installation, update, check or removal; automation uses 'dotfiles machine' with an explicit JSON request")
	}
	if entrypoint := os.Getenv("DOTFILES_ENTRYPOINT"); entrypoint != "" && entrypoint != "setup" {
		return errors.New("unknown installer entrypoint; use setup from the checkout. To migrate a v0.4.x installation, run migrate from a " + engine.MigrationRelease + " checkout first")
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
	platform.Sentinel = engine.SentinelLocations{CodexHome: os.Getenv("CODEX_HOME"), ConfigHome: os.Getenv("XDG_CONFIG_HOME"), ClaudeConfigDir: os.Getenv("CLAUDE_CONFIG_DIR"), KiroHome: os.Getenv("KIRO_HOME"), CopilotHome: os.Getenv("COPILOT_HOME")}
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
	platform.Sentinel.Installed = engine.DiscoverSentinelAgents(platform.Context, folders.Home)
	state, err := engine.DiscoverStateDirectory(platform.Context, folders)
	if err != nil {
		return err
	}
	c, err := engine.NewNativeController(absolute, source, state, platform, folders)
	if err != nil {
		return err
	}
	configureAuthentication(c, platform, machine)
	var result engine.Result
	if machine {
		data, readErr := io.ReadAll(io.LimitReader(os.Stdin, 8<<20+1))
		if readErr != nil {
			return readErr
		}
		var request engine.Request
		if err := engine.Decode(data, &request); err != nil {
			return err
		}
		result, err = c.Dispatch(ctx, request)
		if err == nil {
			result.Message = strings.TrimSpace(result.Message + "\n" + earlierInstallationDisclosure(platform, folders))
		}
		if err != nil {
			return err
		}
		if err := json.NewEncoder(os.Stdout).Encode(result); err != nil {
			return err
		}
	} else {
		ui := &terminal.Workflow{Input: os.Stdin, Output: os.Stdout}
		defer func() { resultErr = errors.Join(resultErr, ui.Close()) }()
		result, err = c.Run(ctx, ui)
		if disclosure := earlierInstallationDisclosure(platform, folders); err == nil && disclosure != "" {
			_, err = fmt.Fprintln(os.Stdout, disclosure)
		}
		// Setup has already shown this result; keep the failing exit status.
		if err == nil && (result.Status == "failed" || result.Status == "needs-action") {
			return errors.New("setup did not finish; the report above lists what needs your action")
		}
		var reported *engine.ReportedError
		if errors.As(err, &reported) {
			return errors.New("setup did not finish; the report above shows the cause and its log")
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
		platform.HomebrewIssue = engine.NixHomebrewIssue(*location)
	}
}

// The prompt names the requester; sudo prints nothing while credentials are cached.
const sudoPrompt = "Dotfiles needs administrator access for the approved changes. [sudo] password for %u: "

func foregroundSudo(program, failure string) func(context.Context) error {
	return func(ctx context.Context) error {
		command := exec.CommandContext(ctx, program, "-v", "-p", sudoPrompt)
		command.Stdin, command.Stdout, command.Stderr = os.Stdin, os.Stdout, os.Stderr
		if err := command.Run(); err != nil {
			return fmt.Errorf("%s: %w", failure, err)
		}
		return nil
	}
}

// sudoSession asks once after an approved plan is reproduced and keeps the
// credential fresh until that apply ends; see engine.KeepSudoAlive.
func sudoSession(program string, authenticate func(context.Context) error) func(context.Context) (func() error, error) {
	return func(ctx context.Context) (func() error, error) {
		return engine.KeepSudoAlive(ctx, program, authenticate)
	}
}

func configureAuthentication(c engine.Controller, platform engine.NativePlatform, machine bool) {
	if machine {
		return
	}
	driver := c.Driver.(*engine.NativeDriver)
	if platform.OS == "darwin" {
		authenticate := foregroundSudo("/usr/bin/sudo", "administrator authentication failed")
		driver.Homebrew.Authenticate = authenticate
		driver.AppleCLT.Authenticate = authenticate
		if os.Geteuid() != 0 {
			driver.Elevation = sudoSession("/usr/bin/sudo", authenticate)
		}
	}
	if platform.APT != nil && platform.APT.Sudo != "" {
		authenticate := foregroundSudo(platform.APT.Sudo, "APT administrator authentication failed")
		driver.APT.Authenticate = authenticate
		if driver.KeyboardAccess != nil {
			driver.KeyboardAccess.Authenticate = authenticate
		}
		driver.Elevation = sudoSession(platform.APT.Sudo, authenticate)
	}
}

func earlierInstallationDisclosure(platform engine.NativePlatform, folders engine.ConfigFolders) string {
	hints := engine.EarlierInstallationHints(platform, folders)
	if len(hints) == 0 {
		return ""
	}
	return "Earlier dotfiles installation detected. To review its retirement, run migrate from a " + engine.MigrationRelease + " checkout, then run setup here again:\n- " + strings.Join(hints, "\n- ")
}
