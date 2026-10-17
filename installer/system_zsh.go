package installer

import (
	"context"
	"errors"
	"os"
)

// macOS ships /bin/zsh as an operating-system component. It is always reused,
// never claimed as an installer-owned package or removed with the profile.
type macOSZshDriver struct{}

func (macOSZshDriver) Observe(ctx context.Context, _ Resource, _ Receipt) (Observation, error) {
	o := Observation{Provider: "operating-system", Identity: "/bin/zsh", Scope: "machine"}
	if err := ctx.Err(); err != nil {
		return o, err
	}
	snapshot, err := snapshotTree("/bin/zsh", 32<<20, 1)
	if err != nil {
		return o, err
	}
	info, err := os.Stat("/bin/zsh")
	if snapshot.Kind != "file" || err != nil || info.Mode().Perm()&0111 == 0 {
		o.ApplyBlocked = "macOS's /bin/zsh is missing or damaged; repair the operating-system component"
		return o, nil
	}
	o.Present, o.Healthy, o.Fingerprint = true, true, snapshot.Hash
	return o, nil
}
func (macOSZshDriver) Apply(context.Context, Resource, Operation, Receipt) (Observation, error) {
	return Observation{}, errors.New("the macOS system shell is not an installer-owned package")
}
func (macOSZshDriver) Remove(context.Context, Resource, Receipt) (Observation, error) {
	return Observation{}, errors.New("the macOS system shell cannot be removed by dotfiles")
}
