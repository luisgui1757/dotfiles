package installer

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

func rustOriginalCommand(name string) string {
	switch strings.ToLower(name) {
	case "rustc.exe", "rustdoc.exe", "clippy-driver.exe":
		return "dotfiles-original-" + strings.ToLower(name)
	default:
		return ""
	}
}

// Pinned stable Rust expands each top-level @file as UTF-8, one argument per
// line, without recursively expanding its contents. Inspect that same boundary
// solely to preserve an explicit caller sysroot; pass the original argv through.
func rustLauncherArguments(args []string, sysroot string) ([]string, error) {
	ended := false
	inspect := func(arg string) bool {
		if ended {
			return false
		}
		if arg == "--" {
			ended = true
			return false
		}
		return arg == "--sysroot" || strings.HasPrefix(arg, "--sysroot=")
	}
	for _, arg := range args {
		if !strings.HasPrefix(arg, "@") {
			if inspect(arg) {
				return args, nil
			}
			continue
		}
		file, err := os.Open(strings.TrimPrefix(arg, "@"))
		if err != nil {
			return nil, fmt.Errorf("read Rust argument file: %w", err)
		}
		data, readErr := io.ReadAll(io.LimitReader(file, 16<<20+1))
		if err := errors.Join(readErr, file.Close()); err != nil {
			return nil, err
		}
		if len(data) > 16<<20 || !utf8.Valid(data) {
			return nil, errors.New("Rust argument file exceeds 16 MiB or is not UTF-8")
		}
		for _, line := range strings.SplitAfter(string(data), "\n") {
			// Rust str::lines removes LF or CRLF, but keeps a bare final CR.
			if strings.HasSuffix(line, "\n") {
				line = strings.TrimSuffix(strings.TrimSuffix(line, "\n"), "\r")
			}
			if inspect(line) {
				return args, nil
			}
		}
	}
	return append([]string{"--sysroot", sysroot}, args...), nil
}

func runRustLauncher(executable, sysroot string, args []string, input io.Reader, output, diagnostic io.Writer) int {
	arguments := args
	clippy := strings.EqualFold(filepath.Base(executable), "clippy-driver.exe")
	if !clippy {
		var err error
		arguments, err = rustLauncherArguments(args, sysroot)
		if err != nil {
			fmt.Fprintln(diagnostic, "Dotfiles Rust:", err)
			return 1
		}
	}
	command := exec.Command(filepath.Join(filepath.Dir(executable), rustOriginalCommand(filepath.Base(executable))), arguments...)
	if clippy {
		// Pinned Clippy accepts SYSROOT and gives explicit CLI/argfile roots
		// precedence. Preserve its first positional Cargo wrapper argument.
		if _, supplied := os.LookupEnv("SYSROOT"); !supplied {
			command.Env = append(os.Environ(), "SYSROOT="+sysroot)
		}
	}
	command.Stdin, command.Stdout, command.Stderr = input, output, diagnostic
	// Windows sends Ctrl-C/Break to every attached process in the console group.
	// Keep this forwarding parent alive until the real compiler handles the same
	// event and exits; killing it on context cancellation would lose its status.
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)
	defer signal.Stop(interrupt)
	if err := command.Run(); err != nil {
		var exited *exec.ExitError
		if errors.As(err, &exited) {
			return exited.ExitCode()
		}
		fmt.Fprintln(diagnostic, "Dotfiles Rust:", err)
		return 1
	}
	return 0
}
