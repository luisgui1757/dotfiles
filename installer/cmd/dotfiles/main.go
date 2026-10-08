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
	platform.Sentinel = engine.SentinelLocations{CodexHome: os.Getenv("CODEX_HOME"), ConfigHome: os.Getenv("XDG_CONFIG_HOME"), PiAgentDir: os.Getenv("PI_CODING_AGENT_DIR")}
	if platform.OS == "darwin" {
		platform.Homebrew, err = engine.DiscoverHomebrew(ctx)
		if err != nil {
			// A broken optional provider must be visible to its resources,
			// without preventing independent archive-only selections.
			platform.HomebrewIssue = err.Error()
		}
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
	c, err := engine.NewNativeController(absolute, source, state, platform, folders)
	if err != nil {
		return err
	}
	if !machine && platform.OS == "darwin" {
		authenticate := func(ctx context.Context) error {
			command := exec.CommandContext(ctx, "/usr/bin/sudo", "-v")
			command.Stdin, command.Stdout, command.Stderr = os.Stdin, os.Stdout, os.Stderr
			if err := command.Run(); err != nil {
				return fmt.Errorf("administrator authentication failed: %w", err)
			}
			return nil
		}
		c.Driver.(*engine.NativeDriver).Homebrew.Authenticate = authenticate
		c.Driver.(*engine.NativeDriver).AppleCLT.Authenticate = authenticate
	}
	if !machine && platform.APT != nil && platform.APT.Sudo != "" {
		c.Driver.(*engine.NativeDriver).APT.Authenticate = func(ctx context.Context) error {
			command := exec.CommandContext(ctx, platform.APT.Sudo, "-v")
			command.Stdin, command.Stdout, command.Stderr = os.Stdin, os.Stdout, os.Stderr
			if err := command.Run(); err != nil {
				return fmt.Errorf("APT administrator authentication failed: %w", err)
			}
			return nil
		}
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
		if migration != nil {
			result, err = migration.Dispatch(ctx, request)
		} else {
			result, err = c.Dispatch(ctx, request)
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
			result, err = migration.Run(ctx, ui)
			if err != nil {
				return err
			}
		}
		if migration == nil || result.Status == "ready" {
			result, err = c.Run(ctx, ui)
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
