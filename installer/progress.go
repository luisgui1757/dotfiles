package installer

import (
	"context"
	"fmt"
	"io"
	"path"
	"strings"
	"time"
)

// Progress displays approved work while it runs. Step receives installer
// status, Output receives live native command output. Both are display only:
// they must not fail, and provider evidence keeps its own bounded capture.
// Previews and machine requests carry no progress.
type Progress struct {
	Step   func(string)
	Output io.Writer
}

type progressKey struct{}

func WithProgress(ctx context.Context, progress Progress) context.Context {
	return context.WithValue(ctx, progressKey{}, progress)
}

func reportStep(ctx context.Context, format string, arguments ...any) {
	if progress, ok := ctx.Value(progressKey{}).(Progress); ok && progress.Step != nil {
		progress.Step(fmt.Sprintf(format, arguments...))
	}
}

func progressOutput(ctx context.Context) io.Writer {
	progress, _ := ctx.Value(progressKey{}).(Progress)
	return progress.Output
}

// reportDetail writes one line beneath the current step. The Output contract
// excludes write failures, so there is no error to return.
func reportDetail(ctx context.Context, format string, arguments ...any) {
	if output := progressOutput(ctx); output != nil {
		_, _ = fmt.Fprintf(output, format+"\n", arguments...)
	}
}

func progressVerb(r Resource, action string) string {
	if r.Action == "retire" {
		return "" // Retirement names already describe their effect.
	}
	return map[string]string{"install": "Installing ", "update": "Updating ", "repair": "Repairing ", "adopt": "Replacing ", "remove": "Removing "}[action]
}

// Show which native program runs, not its environment. Long recipes, such as
// encoded PowerShell, are shortened because the display is not evidence.
func displayCommand(command nativeCommand) string {
	words := []string{path.Base(strings.ReplaceAll(command.Program, `\`, "/"))}
	for _, argument := range command.Arguments {
		if strings.ContainsAny(argument, " \t\n") {
			argument = fmt.Sprintf("%q", argument)
		}
		words = append(words, argument)
	}
	line := strings.Join(words, " ")
	if runes := []rune(line); len(runes) > 240 {
		line = string(runes[:240]) + "…"
	}
	return line
}

func formatBytes(count int64) string {
	if count < 1<<20 {
		return fmt.Sprintf("%.1f KiB", float64(count)/(1<<10))
	}
	return fmt.Sprintf("%.1f MiB", float64(count)/(1<<20))
}

func formatElapsed(elapsed time.Duration) string {
	if elapsed < time.Minute {
		return elapsed.Round(100 * time.Millisecond).String()
	}
	return elapsed.Round(time.Second).String()
}

// downloadProgress rewrites one display line at most twice per second.
type downloadProgress struct {
	output      io.Writer
	name        string
	total, done int64
	shown       time.Time
}

func (p *downloadProgress) Write(data []byte) (int, error) {
	p.done += int64(len(data))
	if now := time.Now(); now.Sub(p.shown) >= 500*time.Millisecond {
		p.shown = now
		if p.total > 0 {
			_, _ = fmt.Fprintf(p.output, "\rDownloading %s: %d%% of %s", p.name, p.done*100/p.total, formatBytes(p.total))
		} else {
			_, _ = fmt.Fprintf(p.output, "\rDownloading %s: %s", p.name, formatBytes(p.done))
		}
	}
	return len(data), nil
}
