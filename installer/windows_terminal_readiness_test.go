package installer

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"
)

// A configured shell may start before its Terminal window becomes visible.
// query is the native process boundary; zero is pending, never successful proof.
func waitForWindowsTerminalWindow(ctx context.Context, done <-chan error, query func(context.Context) ([]byte, error)) (int64, bool, error) {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		if err := ctx.Err(); err != nil {
			return 0, false, fmt.Errorf("owned Terminal window readiness: %w", err)
		}
		select {
		case err := <-done:
			return 0, true, fmt.Errorf("owned Terminal exited before window readiness: %v", err)
		default:
		}
		output, err := query(ctx)
		if err != nil {
			if deadline := ctx.Err(); deadline != nil {
				return 0, false, fmt.Errorf("owned Terminal window readiness: %w; query failed: %w", deadline, err)
			}
			return 0, false, fmt.Errorf("cannot inspect owned Terminal window: %w", err)
		}
		if err := ctx.Err(); err != nil {
			return 0, false, fmt.Errorf("owned Terminal window readiness: %w", err)
		}
		select {
		case err := <-done:
			return 0, true, fmt.Errorf("owned Terminal exited before window readiness: %v", err)
		default:
		}
		window, err := strconv.ParseInt(strings.TrimSpace(string(output)), 10, 64)
		if err != nil {
			return 0, false, fmt.Errorf("invalid owned Terminal window handle %q: %w", output, err)
		}
		if window != 0 {
			return window, false, nil
		}
		select {
		case err := <-done:
			return 0, true, fmt.Errorf("owned Terminal exited before window readiness: %v", err)
		case <-ctx.Done():
			return 0, false, fmt.Errorf("owned Terminal window readiness: %w", ctx.Err())
		case <-ticker.C:
		}
	}
}

func TestWindowsTerminalWindowReadiness(t *testing.T) {
	t.Run("zero remains pending until the owned window appears", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
		defer cancel()
		calls := 0
		window, exited, err := waitForWindowsTerminalWindow(ctx, make(chan error), func(context.Context) ([]byte, error) {
			calls++
			if calls == 1 {
				return []byte("0\r\n"), nil
			}
			return []byte("123\r\n"), nil
		})
		if err != nil || exited || window != 123 || calls != 2 {
			t.Fatal(window, exited, err, calls)
		}
	})
	t.Run("persistent zero cannot pass the original deadline", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(t.Context(), 20*time.Millisecond)
		defer cancel()
		window, exited, err := waitForWindowsTerminalWindow(ctx, make(chan error), func(context.Context) ([]byte, error) {
			return []byte("0"), nil
		})
		if window != 0 || exited || !errors.Is(err, context.DeadlineExceeded) {
			t.Fatal(window, exited, err)
		}
	})
	for _, value := range []string{"", "not-a-handle", "0\n123"} {
		t.Run("malformed output "+value, func(t *testing.T) {
			calls := 0
			_, _, err := waitForWindowsTerminalWindow(t.Context(), make(chan error), func(context.Context) ([]byte, error) {
				calls++
				return []byte(value), nil
			})
			if err == nil || calls != 1 {
				t.Fatal("malformed inspection was retried or accepted", err, calls)
			}
		})
	}
	t.Run("query failure remains fatal", func(t *testing.T) {
		failure := errors.New("native inspection failed")
		calls := 0
		_, _, err := waitForWindowsTerminalWindow(t.Context(), make(chan error), func(context.Context) ([]byte, error) {
			calls++
			return []byte("123"), failure
		})
		if !errors.Is(err, failure) || calls != 1 {
			t.Fatal(err, calls)
		}
	})
	t.Run("owned process exit remains fatal", func(t *testing.T) {
		done := make(chan error, 1)
		window, exited, err := waitForWindowsTerminalWindow(t.Context(), done, func(context.Context) ([]byte, error) {
			done <- nil
			return []byte("0"), nil
		})
		if window != 0 || !exited || err == nil {
			t.Fatal(window, exited, err)
		}
	})
	t.Run("exit during query cannot become stale window success", func(t *testing.T) {
		done := make(chan error, 1)
		window, exited, err := waitForWindowsTerminalWindow(t.Context(), done, func(context.Context) ([]byte, error) {
			done <- nil
			return []byte("123"), nil
		})
		if window != 0 || !exited || err == nil {
			t.Fatal(window, exited, err)
		}
	})
	t.Run("query shares deadline and cannot return late success", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(t.Context(), 20*time.Millisecond)
		defer cancel()
		window, exited, err := waitForWindowsTerminalWindow(ctx, make(chan error), func(queryContext context.Context) ([]byte, error) {
			<-queryContext.Done()
			return []byte("123"), nil
		})
		if window != 0 || exited || !errors.Is(err, context.DeadlineExceeded) {
			t.Fatal(window, exited, err)
		}
	})
	t.Run("query killed at deadline preserves both errors", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(t.Context(), 20*time.Millisecond)
		defer cancel()
		failure := errors.New("query process terminated")
		window, exited, err := waitForWindowsTerminalWindow(ctx, make(chan error), func(queryContext context.Context) ([]byte, error) {
			<-queryContext.Done()
			return nil, failure
		})
		if window != 0 || exited || !errors.Is(err, context.DeadlineExceeded) || !errors.Is(err, failure) {
			t.Fatal(window, exited, err)
		}
	})
}
