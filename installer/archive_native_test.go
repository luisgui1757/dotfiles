package installer

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestNativeArchiveInstallsExecutesChecksAndRemovesRealStarship(t *testing.T) {
	if os.Getenv("DOTFILES_TEST_ARCHIVES") != "1" {
		t.Skip("set DOTFILES_TEST_ARCHIVES=1 for verified upstream downloads and real execution")
	}
	pins, err := DefaultArchivePins(nativePlatform(t))
	if err != nil {
		t.Fatal(err)
	}
	pin, ok := pins["tool.starship"]
	if !ok {
		t.Fatal("native runner has no supported Starship artifact")
	}
	c, d, _ := archiveController(t)
	d.Pins["tool.shared"], d.Client = pin, nil
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"first"}})
	payload, err := d.PayloadPath("tool.shared")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	entrypoint, err := d.CommandPath("tool.shared", "starship")
	if err != nil {
		t.Fatal(err)
	}
	command := exec.CommandContext(ctx, entrypoint, "--version")
	command.Env = append(os.Environ(), "HOME="+c.Home)
	output, err := command.CombinedOutput()
	if err != nil || !strings.Contains(string(output), "starship "+pin.Version) {
		t.Fatalf("native package execution: %s %v", output, err)
	}
	t.Logf("native %s/%s: %s", runtime.GOOS, runtime.GOARCH, output)
	dispatchApproved(t, c, Request{Schema: 1, Mode: "update"})
	check, err := c.Dispatch(ctx, Request{Schema: 1, Mode: "check"})
	if err != nil || check.Status != "ready" {
		t.Fatal("native check", check, err)
	}
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{}})
	if _, err := os.Lstat(payload); !os.IsNotExist(err) {
		t.Fatal("native removal left payload", err)
	}
}

// Exercise each real upstream layout through the same provider and receipt
// lifecycle. Full capability tests separately cover system prerequisites and
// tool configuration; an executable on a provisioned runner cannot prove those.
func TestNativeArchivePinnedCommandLifecycle(t *testing.T) {
	if os.Getenv("DOTFILES_TEST_ARCHIVES") != "1" {
		t.Skip("set DOTFILES_TEST_ARCHIVES=1 for verified upstream downloads and real execution")
	}
	pins, err := DefaultArchivePins(nativePlatform(t))
	if err != nil {
		t.Fatal(err)
	}
	for _, tool := range []string{"nvim", "rg", "gh", "lazygit", "jq", "tree-sitter", "hyperfine", "node", "shellcheck", "taplo", "gh-dash", "powershell", "cmake", "python", "herdr", "editorconfig", "clipboard", "pi"} {
		t.Run(tool, func(t *testing.T) {
			pin, ok := pins["tool."+tool]
			if !ok && (tool == "powershell" || tool == "clipboard") && runtime.GOOS != "windows" {
				t.Skip("this tool uses a native provider on POSIX")
			}
			if !ok {
				t.Fatal("native runner has no supported artifact")
			}
			if tool == "clipboard" && (os.Getenv("GITHUB_ACTIONS") != "true" || os.Getenv("DOTFILES_TEST_PUBLIC_WINDOWS") != "1") {
				t.Skip("clipboard writes require the explicitly enabled disposable Windows runner")
			}
			c, d, _ := archiveController(t)
			d.Pins["tool.shared"], d.Client = pin, nil
			dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"first"}})
			commandName := tool
			switch tool {
			case "powershell":
				commandName = "pwsh"
			case "python":
				if runtime.GOOS != "windows" {
					commandName = "python3"
				}
			case "editorconfig":
				commandName = "editorconfig-checker"
			case "clipboard":
				commandName = "win32yank"
			}
			command, err := d.CommandPath("tool.shared", commandName)
			if err != nil {
				t.Fatal(err)
			}
			execute := func(args ...string) string {
				t.Helper()
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()
				cmd := exec.CommandContext(ctx, command, args...)
				cmd.Dir = c.Home
				cmd.Env = append(os.Environ(), "HOME="+c.Home, "XDG_CONFIG_HOME="+filepath.Join(c.Home, "config"), "XDG_DATA_HOME="+filepath.Join(c.Home, "data"), "XDG_STATE_HOME="+filepath.Join(c.Home, "state"), "npm_config_userconfig="+filepath.Join(c.Home, "npmrc"), "npm_config_globalconfig="+filepath.Join(c.Home, "global-npmrc"), "npm_config_cache="+filepath.Join(c.Home, "npm-cache"))
				if tool == "gh-dash" {
					cmd.Env = append(cmd.Env, "GH_CONFIG_DIR="+filepath.Join(c.Home, "gh"), "GH_TOKEN=", "GITHUB_TOKEN=", "GH_ENTERPRISE_TOKEN=", "GITHUB_ENTERPRISE_TOKEN=")
				}
				if tool == "pi" {
					cmd.Env = append(cmd.Env, "PI_CODING_AGENT_DIR="+filepath.Join(c.Home, "pi"), "PI_PACKAGE_DIR=", "PI_OFFLINE=1")
				}
				output, err := cmd.CombinedOutput()
				if err != nil {
					t.Fatalf("%s %v: %v\n%s", tool, args, err, output)
				}
				return string(output)
			}
			if tool == "clipboard" {
				// win32yank has no version flag. Its verified archive pins the
				// version; this checks its real purpose on a disposable host.
				value := "dotfiles clipboard ✓\nsecond line\n"
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()
				copy := exec.CommandContext(ctx, command, "-i", "--crlf")
				copy.Stdin = strings.NewReader(value)
				if output, err := copy.CombinedOutput(); err != nil {
					t.Fatalf("write fixture clipboard: %v\n%s", err, output)
				}
				if output := execute("-o", "--lf"); output != value {
					t.Fatal("native clipboard did not round-trip Unicode and newlines")
				}
			} else if output := execute("--version"); !strings.Contains(output, pin.Version) {
				t.Fatal("version differs from pin", output, pin.Version)
			}
			if tool == "gh-dash" {
				if output := execute("--help"); !strings.Contains(output, "--config") {
					t.Fatal("standalone dashboard help is unavailable without authentication", output)
				}
			}
			if tool == "pi" {
				if output := execute("--help"); !strings.Contains(output, "--mode") || !strings.Contains(output, "rpc") {
					t.Fatal("standalone Pi help is unavailable without authentication", output)
				}
				exercisePiOfflineExtension(t, command, c.Home)
			}
			if tool == "python" {
				for _, optimization := range []string{"-O", "-OO"} {
					execute("-I", optimization, "-c", "import ssl, sqlite3, venv, ensurepip; assert ssl.OPENSSL_VERSION; assert sqlite3.sqlite_version")
				}
				execute("-I", "-c", "import ssl, sqlite3, venv, ensurepip; assert ssl.OPENSSL_VERSION; assert sqlite3.sqlite_version")
				execute("-I", "-m", "venv", filepath.Join(c.Home, "python-venv"))
			}
			if tool == "cmake" {
				if err := os.WriteFile(filepath.Join(c.Home, "CMakeLists.txt"), []byte("cmake_minimum_required(VERSION 3.20)\nproject(DotfilesRuntime NONE)\n"), 0600); err != nil {
					t.Fatal(err)
				}
				execute("-S", c.Home, "-B", filepath.Join(c.Home, "cmake-build"))
			}
			if tool == "nvim" {
				execute("--clean", "--headless", "+lua assert(vim.fn.filereadable(vim.env.VIMRUNTIME .. '/filetype.lua') == 1)", "+qa")
			}
			if tool == "node" {
				payload, err := d.PayloadPath("tool.shared")
				if err != nil {
					t.Fatal(err)
				}
				npm := filepath.Join(payload, filepath.FromSlash(pin.RequiredFiles[0]))
				if output := execute(npm, "--version"); strings.TrimSpace(output) == "" {
					t.Fatal("bundled npm is not executable through the private Node runtime")
				}
			}
			dispatchApproved(t, c, Request{Schema: 1, Mode: "update"})
			check, err := c.Dispatch(context.Background(), Request{Schema: 1, Mode: "check"})
			if err != nil || check.Status != "ready" {
				t.Fatal("native check", check, err)
			}
			if tool == "python" {
				payload, err := d.PayloadPath("tool.shared")
				if err != nil {
					t.Fatal(err)
				}
				cache := filepath.Join(payload, pin.PythonStdlib, "__pycache__", "ssl.cpython-313.pyc")
				original, err := os.ReadFile(cache)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(cache, append(slices.Clone(original), 0), 0644); err != nil {
					t.Fatal(err)
				}
				changed, err := c.Dispatch(context.Background(), Request{Schema: 1, Mode: "check"})
				if err != nil || changed.Status == "ready" {
					t.Fatal("Python preparation concealed a subsequent bytecode edit", changed, err)
				}
				if err := os.WriteFile(cache, original, 0644); err != nil {
					t.Fatal(err)
				}
			}
			dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{}})
			if _, err := os.Stat(command); !os.IsNotExist(err) {
				t.Fatal("removal left the private command", err)
			}
		})
	}
}

func TestNativeArchiveChangesTheInstalledVersion(t *testing.T) {
	if os.Getenv("DOTFILES_TEST_ARCHIVES") != "1" {
		t.Skip("enable real upstream native lifecycle")
	}
	platform := nativePlatform(t)
	pins, err := DefaultArchivePins(platform)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile("testdata/starship-update-baseline.json")
	if err != nil {
		t.Fatal(err)
	}
	var baselines map[string]ArchivePin
	if err := Decode(data, &baselines); err != nil {
		t.Fatal(err)
	}
	before, after := baselines[platform.OS+"/"+platform.Arch], pins["tool.starship"]
	if before.Version == "" || before.Version == after.Version || before.SHA256 == after.SHA256 {
		t.Fatal("native version-change fixture lacks distinct real artifacts")
	}
	c, d, _ := archiveController(t)
	d.Pins["tool.shared"], d.Client = before, nil
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"first"}})
	previous, err := d.PayloadPath("tool.shared")
	if err != nil {
		t.Fatal(err)
	}
	command, err := d.CommandPath("tool.shared", "starship")
	if err != nil {
		t.Fatal(err)
	}
	verify := func(version string) {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		output, err := exec.CommandContext(ctx, command, "--version").CombinedOutput()
		if err != nil || !strings.Contains(string(output), "starship "+version) {
			t.Fatalf("real version transition: %s %v", output, err)
		}
	}
	verify(before.Version)
	d.Pins["tool.shared"] = after
	dispatchApproved(t, c, Request{Schema: 1, Mode: "update"})
	verify(after.Version)
	if _, err := os.Stat(previous); !os.IsNotExist(err) {
		t.Fatal("unchanged retired generation remains after successful version update", err)
	}
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{}})
	if _, err := os.Stat(command); !os.IsNotExist(err) {
		t.Fatal("updated private command remains after removal", err)
	}
}
