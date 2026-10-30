package installer

import (
	"context"
	"os/exec"
	"time"
)

// ElevationDriver authenticates once for approved work that needs foreground
// sudo, after Execute reproduces the approved plan and before any mutation.
// The returned release ends credential refresh. Providers keep authenticating
// before each privileged command, which costs no prompt while the credential
// is fresh and still asks if refresh stopped.
type ElevationDriver interface {
	Elevate(context.Context, Plan) (func() error, error)
}

func noElevation() error { return nil }

// KeepSudoAlive runs the entrypoint's foreground authentication once, then
// refreshes the sudo timestamp with `sudo -n -v` until release, so a long
// approved apply is not interrupted by an expired credential. Refresh never
// prompts or reads a password, and stops with the apply.
func KeepSudoAlive(ctx context.Context, sudo string, authenticate func(context.Context) error) (func() error, error) {
	return keepSudoAlive(ctx, sudo, authenticate, time.Minute)
}

func keepSudoAlive(ctx context.Context, sudo string, authenticate func(context.Context) error, interval time.Duration) (func() error, error) {
	if err := authenticate(ctx); err != nil {
		return nil, err
	}
	refresh, cancel := context.WithCancel(ctx)
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-refresh.Done():
				return
			case <-ticker.C:
			}
			output, err := exec.CommandContext(refresh, sudo, "-n", "-v").CombinedOutput()
			if err != nil && refresh.Err() == nil {
				reportDetail(ctx, "Administrator credential refresh stopped (%v%s); a later privileged step asks again if needed.", err, nativeDiagnostic(output))
				return
			}
		}
	}()
	return func() error {
		cancel()
		<-stopped
		return nil
	}, nil
}

// Only these providers authenticate in the foreground. Machine scope alone
// does not mean sudo: Homebrew formulae, for example, never use it.
func (d *NativeDriver) Elevate(ctx context.Context, plan Plan) (func() error, error) {
	if d.Elevation == nil {
		return noElevation, nil
	}
	for _, op := range plan.Operations {
		if !mutating(op.Action) && plan.ResourceResume == nil {
			continue
		}
		r, ok := d.Catalog.Resource(op.Resource)
		if !ok {
			continue
		}
		provider, err := d.provider(r)
		if err != nil {
			continue // Routing errors are reported by the operation itself.
		}
		needed := false
		switch p := provider.(type) {
		case *LinuxAPTDriver:
			needed = p.Authenticate != nil
		case *HomebrewBootstrapDriver:
			needed = p.Authenticate != nil
		case *AppleCLTDriver:
			needed = p.Authenticate != nil
		case *KeyboardAccessDriver:
			needed = p.Authenticate != nil
		}
		if needed {
			return d.Elevation(ctx)
		}
	}
	return noElevation, nil
}
