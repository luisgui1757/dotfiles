package installer

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// RunWindowsRustLauncher recognizes only the three fixed prepared entrypoints.
// Normal installer invocation never takes this path. No shell or worker runs.
func RunWindowsRustLauncher() (handled bool, exitCode int) {
	executable, err := os.Executable()
	if err != nil || rustOriginalCommand(filepath.Base(executable)) == "" {
		return false, 0
	}
	if !strings.EqualFold(filepath.Base(filepath.Dir(executable)), "bin") {
		fmt.Fprintln(os.Stderr, "Dotfiles Rust: launcher is outside the managed toolchain bin directory")
		return true, 1
	}
	sysroot := windowsExtendedPath(filepath.Dir(filepath.Dir(executable)))
	return true, runRustLauncher(executable, sysroot, os.Args[1:], os.Stdin, os.Stdout, os.Stderr)
}
