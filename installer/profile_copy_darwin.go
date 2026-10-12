package installer

import (
	"context"
	"fmt"
	"os"
	"os/exec"

	"golang.org/x/sys/unix"
)

func copyNativeProfile(ctx context.Context, from, to string, _ *os.File, _ unix.Stat_t) error {
	// macOS cp preserves its native ACLs, flags and resource metadata. The
	// destination is our exclusively reserved inode; no shell parses paths.
	output, err := exec.CommandContext(ctx, "/bin/cp", "-p", from, to).CombinedOutput()
	if err != nil {
		return fmt.Errorf("preserve profile metadata: %w: %s", err, output)
	}
	return nil
}
