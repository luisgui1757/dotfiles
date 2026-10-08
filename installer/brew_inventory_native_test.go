package installer

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"testing"
	"time"
)

// This reads the real native inventory without installing or removing packages.
// Mutation contract tests retain their separate disposable-host opt-in.
func TestNativeBrewInventoryReadOnly(t *testing.T) {
	if runtime.GOOS != "darwin" || os.Getenv("DOTFILES_TEST_NATIVE_INVENTORY") != "1" && os.Getenv("DOTFILES_TEST_NATIVE_PACKAGES") != "1" {
		t.Skip("requires explicit native inventory inspection")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	query := func(ctx context.Context, privileged bool, program string, input []byte, args ...string) ([]byte, error) {
		if privileged || program != "brew" || len(input) != 0 {
			return nil, fmt.Errorf("unexpected native inventory command")
		}
		command := exec.CommandContext(ctx, program, args...)
		command.Env = append(os.Environ(), "LC_ALL=C", "HOMEBREW_NO_AUTO_UPDATE=1", "HOMEBREW_NO_ANALYTICS=1", "HOMEBREW_NO_COLOR=1")
		return command.Output()
	}
	installed, err := brewInventory(ctx, query)
	if err != nil {
		t.Fatal(err)
	}
	// An empty attributable pool must not turn the machine's existing packages
	// into cleanup candidates, regardless of native orphan/manual classifications.
	remove, _, err := nativeRemovalCandidates(nil, nil, nil, installed)
	if err != nil || len(remove) != 0 {
		t.Fatal("existing native inventory became owned", remove, err)
	}
	t.Logf("read %d actual installed native formula/cask identities; no mutation", len(installed))
}
