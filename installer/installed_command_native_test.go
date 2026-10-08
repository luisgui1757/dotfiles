package installer

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

// Both archive and native-package acceptance use the installed executable and
// real machine protocol. No controller or process boundary is substituted.
func installedMachineFixture(t *testing.T, repository, home string, timeout time.Duration) func(Request) Result {
	t.Helper()
	binary := filepath.Join(home, "dotfiles")
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}
	build := exec.Command("go", "build", "-trimpath", "-o", binary, "./cmd/dotfiles")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build installed entrypoint: %s %v", output, err)
	}
	return func(request Request) Result {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		result, err := installedMachineRequest(ctx, binary, repository, home, request)
		if err != nil {
			t.Fatal(err)
		}
		return result
	}
}

func installedMachineRequest(ctx context.Context, binary, repository, home string, request Request) (Result, error) {
	var result Result
	data, err := json.Marshal(request)
	if err != nil {
		return result, err
	}
	cmd := exec.CommandContext(ctx, binary, "machine")
	cmd.Env = append(os.Environ(), "HOME="+home, "XDG_CONFIG_HOME="+filepath.Join(home, ".config"), "XDG_DATA_HOME="+filepath.Join(home, ".local", "share"), "XDG_STATE_HOME="+filepath.Join(home, ".local", "state"), "CODEX_HOME="+filepath.Join(home, ".codex"), "PI_CODING_AGENT_DIR="+filepath.Join(home, ".pi", "agent"), "DOTFILES_CHECKOUT="+repository, "ZDOTDIR="+home, "TERM=xterm-256color")
	cmd.Stdin = bytes.NewReader(data)
	cmd.WaitDelay = 2 * time.Second
	output, diagnostic := boundedCommandOutput{limit: 8 << 20}, boundedCommandOutput{limit: 64 << 10}
	cmd.Stdout, cmd.Stderr = &output, &diagnostic
	if err := cmd.Run(); err != nil {
		return result, fmt.Errorf("installed entrypoint: %w%s", err, nativeDiagnostic(append(output.data.Bytes(), diagnostic.data.Bytes()...)))
	}
	if err := Decode(output.data.Bytes(), &result); err != nil {
		return result, err
	}
	return result, nil
}
