package installer

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func nativePreparedCommand(t *testing.T, home, program string, input string, args ...string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	command := exec.CommandContext(ctx, program, args...)
	command.Dir = home
	command.Env = append(os.Environ(), "HOME="+home, "USERPROFILE="+home, "CARGO_HOME="+filepath.Join(home, "cargo"), "RUSTUP_HOME="+filepath.Join(home, "rustup"), "PYTHONPATH=", "PYTHONHOME=")
	command.Stdin = strings.NewReader(input)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("private command %s: %s %v", filepath.Base(program), output, err)
	}
	return string(output)
}

func TestNativeLatexPreparationLifecycleAndStablePython(t *testing.T) {
	if os.Getenv("DOTFILES_TEST_LANGUAGE_PREPARATION") != "1" {
		t.Skip("set DOTFILES_TEST_LANGUAGE_PREPARATION=1 for private pinned language artifacts")
	}
	pins, err := DefaultArchivePins(nativePlatform(t))
	if err != nil {
		t.Fatal(err)
	}
	pin, ok := pins["tool.latex2text"]
	if !ok {
		t.Fatal("latex2text pin is missing")
	}
	c, d, _ := archiveController(t)
	home := c.Home
	pythonPayload := filepath.Join(home, "python-v1")
	pythonPin := pins["tool.python"]
	if err := downloadArchive(context.Background(), nil, pythonPin, pythonPayload); err != nil {
		t.Fatal(err)
	}
	if err := preparePythonArchive(context.Background(), pythonPin, pythonPayload); err != nil {
		t.Fatal(err)
	}
	stable := filepath.Join(home, "python-current")
	if err := createDirectoryLink(pythonPayload, stable); err != nil {
		t.Fatal(err)
	}
	name := "python3"
	if runtime.GOOS == "windows" {
		name = "python"
	}
	d.Python = filepath.Join(stable, filepath.FromSlash(pythonPin.Commands[name]))
	worker, err := workerFixture(filepath.Join(home, "worker"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := worker.close(); err != nil {
			t.Error(err)
		}
	})
	d.Run, d.Pins["tool.shared"], d.Client = worker.run, pin, nil
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"first"}})
	converter, err := d.CommandPath("tool.shared", "latex2text")
	if err != nil {
		t.Fatal(err)
	}
	payload, err := d.PayloadPath("tool.shared")
	if err != nil {
		t.Fatal(err)
	}
	// Reproduce Windows pipe encoding on every host; the managed command must
	// establish UTF-8 itself, independent of the parent interpreter locale.
	t.Setenv("PYTHONIOENCODING", "cp1252")
	t.Setenv("PYTHONUTF8", "0")
	if output := nativePreparedCommand(t, home, converter, `\alpha`); strings.TrimSpace(output) != "α" {
		t.Fatal(output)
	}
	check, err := c.Dispatch(context.Background(), Request{Schema: 1, Mode: "check"})
	if err != nil || check.Status != "ready" {
		t.Fatal("ordinary converter use changed its archive", check, err)
	}
	// Remove the old base generation entirely and repoint only its stable link.
	// A venv tied to the resolved old generation cannot survive this probe.
	moved := filepath.Join(home, "python-v2")
	if err := os.Rename(pythonPayload, moved); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(stable); err != nil {
		t.Fatal(err)
	}
	if err := createDirectoryLink(moved, stable); err != nil {
		t.Fatal(err)
	}
	if output := nativePreparedCommand(t, home, converter, `α + \beta`); strings.TrimSpace(output) != "α + β" {
		t.Fatal(output)
	}
	check, err = c.Dispatch(context.Background(), Request{Schema: 1, Mode: "check"})
	if err != nil || check.Status != "ready" {
		t.Fatal("base switch changed the venv", check, err)
	}
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{}})
	if _, err := os.Lstat(payload); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("converter payload survived removal", err)
	}
	t.Log("private pinned converter installed, used UTF-8 streams under cp1252, survived stable Python switch, checked unchanged, and removed")
}

func TestNativeRustPreparationLifecycleAndBundledTools(t *testing.T) {
	if os.Getenv("DOTFILES_TEST_LANGUAGE_PREPARATION") != "1" {
		t.Skip("set DOTFILES_TEST_LANGUAGE_PREPARATION=1 for private pinned language artifacts")
	}
	pins, err := DefaultArchivePins(nativePlatform(t))
	if err != nil {
		t.Fatal(err)
	}
	pin, ok := pins["tool.rust"]
	if !ok {
		t.Fatal("native Rust pin is missing")
	}
	c, d, _ := archiveController(t)
	if runtime.GOOS == "windows" {
		platform := nativePlatform(t)
		platform.WindowsPowerShell, err = DiscoverWindowsPowerShell()
		if err != nil {
			t.Fatal(err)
		}
		platform.WindowsBuildToolsDirectory, err = DiscoverWindowsBuildToolsDirectory()
		if err != nil {
			t.Fatal(err)
		}
		compiler, err := configureWindowsBuildTools(platform, newNativeSession(filepath.Join(c.Home, "compiler-worker")))
		if err != nil || compiler == nil {
			t.Fatal("native Windows compiler is unavailable", err)
		}
		environment, err := compiler.CompilerEnvironment(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		directories, err := DiscoverSystemCommandDirectories()
		if err != nil {
			t.Fatal(err)
		}
		// The real installed binary is also the fixed native Rust launcher.
		// Build it before replacing PATH with the reviewed compiler environment.
		d.WindowsRustLauncher = filepath.Join(c.Home, "installer-rust-launcher.exe")
		build := exec.Command("go", "build", "-trimpath", "-o", d.WindowsRustLauncher, "./cmd/dotfiles")
		if output, err := build.CombinedOutput(); err != nil {
			t.Fatalf("build Rust launcher: %s %v", output, err)
		}
		launcher, err := os.ReadFile(d.WindowsRustLauncher)
		if err != nil {
			t.Fatal(err)
		}
		hash := sha256.Sum256(launcher)
		pin.WindowsRustLauncher.SHA256 = hex.EncodeToString(hash[:])
		for name, value := range environment {
			if name == "PATH" {
				value += string(os.PathListSeparator) + strings.Join(directories, string(os.PathListSeparator))
			}
			t.Setenv(name, value)
		}
	}
	d.Pins["tool.shared"], d.Client = pin, nil
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"first"}})
	paths := map[string]string{}
	for _, name := range []string{"rustc", "cargo", "rustfmt", "cargo-clippy", "clippy-driver"} {
		paths[name], err = d.CommandPath("tool.shared", name)
		if err != nil {
			t.Fatal(err)
		}
		output := nativePreparedCommand(t, c.Home, paths[name], "", "--version")
		if !strings.Contains(output, pin.Version) && name != "rustfmt" && name != "cargo-clippy" && name != "clippy-driver" {
			t.Fatal(name, output)
		}
	}
	payload, err := d.PayloadPath("tool.shared")
	if err != nil {
		t.Fatal(err)
	}
	for _, component := range []string{"libcore-*.rlib", "libstd-*.rlib"} {
		files, err := filepath.Glob(filepath.Join(payload, "lib", "rustlib", pin.RustTarget, "lib", component))
		if err != nil || len(files) != 1 {
			t.Fatal("missing target library", component, files, err)
		}
	}
	if _, err := os.Stat(filepath.Join(payload, "lib", "rustlib", "src", "rust", "library", "core", "src", "lib.rs")); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(c.Home, "private-rust-smoke.rs")
	if err := os.WriteFile(source, []byte("fn main() { println!(\"private Rust ready\"); }\n"), 0600); err != nil {
		t.Fatal(err)
	}
	executable := filepath.Join(c.Home, "private-rust-smoke")
	if runtime.GOOS == "windows" {
		executable += ".exe"
	}
	if runtime.GOOS == "windows" {
		// Keep the real long fixture path: the original compiler must expose
		// its default physical sysroot, while the managed launcher keeps an
		// extended-length root that MSVC can consume without MAX_PATH loss.
		original := filepath.Join(filepath.Dir(paths["rustc"]), rustOriginalCommand("rustc.exe"))
		physical := strings.TrimSpace(nativePreparedCommand(t, c.Home, original, "", "--print", "sysroot"))
		managed := strings.TrimSpace(nativePreparedCommand(t, c.Home, paths["rustc"], "", "--print", "sysroot"))
		if !strings.HasPrefix(managed, `\\?\`) || strings.HasPrefix(physical, `\\?\`) {
			t.Fatal("Rust launcher did not preserve an explicit extended-length sysroot", physical, managed)
		}
		libraries, err := filepath.Glob(filepath.Join(physical, "lib", "rustlib", pin.RustTarget, "lib", "libstd-*.rlib"))
		if err != nil || len(libraries) != 1 || len(libraries[0]) <= 260 {
			t.Fatal("native fixture no longer exercises the observed linker long-path boundary", libraries, err)
		}
		for _, arguments := range [][]string{{"--sysroot", managed, "--print", "sysroot"}, {"--sysroot=" + managed, "--print", "sysroot"}} {
			if value := strings.TrimSpace(nativePreparedCommand(t, c.Home, paths["rustc"], "", arguments...)); value != managed {
				t.Fatal("caller sysroot changed", value, managed)
			}
		}
		argfile := filepath.Join(c.Home, "private rust args.txt")
		if err := os.WriteFile(argfile, []byte("--sysroot\r\n"+managed+"\r\n--print\r\nsysroot\r\n"), 0600); err != nil {
			t.Fatal(err)
		}
		if value := strings.TrimSpace(nativePreparedCommand(t, c.Home, paths["rustc"], "", "@"+argfile)); value != managed {
			t.Fatal("caller argfile sysroot changed", value, managed)
		}
		t.Logf("native Rust default std library path exceeds MAX_PATH: %d characters; managed sysroot=%s", len(libraries[0]), managed)
	}
	nativePreparedCommand(t, c.Home, paths["rustc"], "", source, "-o", executable)
	if output := nativePreparedCommand(t, c.Home, executable, ""); strings.TrimSpace(output) != "private Rust ready" {
		t.Fatal(output)
	}
	formatted := nativePreparedCommand(t, c.Home, paths["rustfmt"], "fn main(){println!(\"private\");}\n", "--emit", "stdout")
	if !strings.Contains(formatted, "fn main() {") {
		t.Fatal(formatted)
	}
	project := filepath.Join(c.Home, "clippy-project")
	if err := os.MkdirAll(filepath.Join(project, "src"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, "Cargo.toml"), []byte("[package]\nname=\"private-smoke\"\nversion=\"0.1.0\"\nedition=\"2021\"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, "src", "main.rs"), []byte("fn main() {}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", filepath.Dir(paths["rustc"])+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("RUSTC", paths["rustc"])
	nativePreparedCommand(t, c.Home, paths["cargo"], "", "build", "--offline", "--manifest-path", filepath.Join(project, "Cargo.toml"), "--target-dir", filepath.Join(project, "target"))
	nativePreparedCommand(t, c.Home, paths["cargo"], "", "clippy", "--offline", "--manifest-path", filepath.Join(project, "Cargo.toml"), "--target-dir", filepath.Join(project, "target"), "--", "-D", "warnings")
	check, err := c.Dispatch(context.Background(), Request{Schema: 1, Mode: "check"})
	if err != nil || check.Status != "ready" {
		t.Fatal("Rust use changed the immutable toolchain", check, err)
	}
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{}})
	if _, err := os.Lstat(payload); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("toolchain survived removal", err)
	}
	t.Log("private Rust compiled and ran a program, formatted stdin, built through offline Cargo, ran offline Clippy, checked unchanged, and removed")
}
