package terminal

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	engine "github.com/luisgui1757/dotfiles/installer"
	"golang.org/x/term"
)

// Workflow connects the native terminal to the lifecycle controller. Each
// interaction closes its console before returning, leaving no raw mode or
// reader active when an adapter needs the foreground terminal for elevation.
type Workflow struct{ Input, Output *os.File }

func (w Workflow) Choose(ctx context.Context, title string, choices []engine.Choice, initial []string, multiple bool) (selected []string, resultErr error) {
	c, err := Open(w.Input, w.Output)
	if err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, c.Close()) }()
	options := make([]Option, 0, len(choices))
	for _, choice := range choices {
		options = append(options, Option{ID: choice.ID, Label: choice.Label, Detail: choice.Detail})
	}
	selected, err = c.Choose(ctx, title, options, initial, multiple)
	if errors.Is(err, ErrCancelled) {
		err = engine.ErrCancelled
	}
	return selected, err
}

func (w Workflow) Review(ctx context.Context, catalog *engine.Catalog, plan engine.Plan) (accepted bool, resultErr error) {
	c, err := Open(w.Input, w.Output)
	if err != nil {
		return false, err
	}
	defer func() { resultErr = errors.Join(resultErr, c.Close()) }()
	if err := c.Read(ctx, "Review every change", planText(catalog, plan)); err != nil {
		if errors.Is(err, ErrCancelled) {
			err = engine.ErrCancelled
		}
		return false, err
	}
	accepted, err = c.confirm(ctx, plan)
	if errors.Is(err, ErrCancelled) {
		return false, engine.ErrCancelled
	}
	return accepted, err
}

func (w Workflow) Report(result engine.Result) (resultErr error) {
	lines := append([]string{result.Status, ""}, strings.Split(result.Message, "\n")...)
	if result.Plan.Mode == "check" {
		c, err := Open(w.Input, w.Output)
		if err != nil {
			return err
		}
		defer func() { resultErr = errors.Join(resultErr, c.Close()) }()
		err = c.Read(context.Background(), "Installation check", lines)
		if errors.Is(err, ErrCancelled) {
			return nil
		}
		return err
	}
	for i := range lines {
		lines[i] = plain(lines[i])
	}
	_, err := fmt.Fprintln(w.Output, strings.Join(lines, "\n"))
	return err
}

func planSummary(plan engine.Plan) []string {
	counts := map[string]int{}
	admin := 0
	for _, op := range plan.Operations {
		counts[op.Action]++
		if op.Privileged && (op.Action == "install" || op.Action == "update" || op.Action == "repair" || op.Action == "adopt" || op.Action == "remove") {
			admin++
		}
	}
	return []string{
		fmt.Sprintf("Add: %d  Change: %d", counts["install"], counts["repair"]+counts["update"]+counts["adopt"]),
		fmt.Sprintf("Remove: %d  Keep: %d", counts["remove"], counts["keep"]+counts["retain"]),
		fmt.Sprintf("Needs action: %d  Admin: %d", counts["pending"], admin),
	}
}

func planText(catalog *engine.Catalog, plan engine.Plan) []string {
	lines := planSummary(plan)
	for _, op := range plan.Operations {
		if op.Action == "remove" || op.Action == "adopt" || op.Privileged && (op.Action == "install" || op.Action == "repair" || op.Action == "update") {
			r, _ := catalog.Resource(op.Resource)
			lines = append(lines, strings.ToUpper(op.Action)+": "+r.Name)
		}
	}
	lines = append(lines, "", "Operation: "+plan.Mode, "Target: "+plan.Target, "Source: "+plan.Source, "Plan: "+plan.ID,
		"Selected: "+strings.Join(plan.Selected, ", "), "Keep: "+strings.Join(plan.Keep, ", "), "")
	if plan.Mode == "abandon" {
		lines = append(lines, "Archive the interrupted operation. Keep every resource and ownership receipt.", "")
	}
	if plan.Mode == "restore" {
		lines = append(lines, "Restore only the interrupted configuration from its saved journal. Keep conflicting files and archive the transaction.", "")
	} else if plan.ResourceResume != nil {
		lines = append(lines, "Resume only the original immutable provider target. Review the remaining work afterwards.", "")
	}
	if len(plan.Operations) == 0 {
		lines = append(lines, "No resource changes.")
	}
	for _, op := range plan.Operations {
		r, _ := catalog.Resource(op.Resource)
		provider := op.Observed.Provider
		if provider == "" {
			provider = r.Bindings[plan.Context.OS].Provider
		}
		lines = append(lines, strings.ToUpper(op.Action)+": "+r.Name+" ("+r.ID+")", "Why: "+op.Reason)
		if len(op.RequiredBy) > 0 {
			lines = append(lines, "Required by: "+strings.Join(op.RequiredBy, ", "))
		}
		lines = append(lines, "Ownership: "+op.Ownership)
		if provider != "" {
			lines = append(lines, "Provider: "+provider)
		}
		if op.Observed.Version != "" {
			lines = append(lines, "Installed version: "+op.Observed.Version)
		}
		for _, path := range op.Observed.Preserved {
			lines = append(lines, "Preserved files for inspection: "+path)
		}
		for _, name := range op.Observed.UnverifiedApplications {
			lines = append(lines, "External application not exercised: "+name)
		}
		if op.Privileged {
			lines = append(lines, "Requires administrator privileges.")
		}
		lines = append(lines, "")
	}
	return lines
}

// Read is a scrollable document, not a clipped option detail. Every approval
// field and explanation remains reachable in narrow or resized terminals.
func (c *Console) Read(ctx context.Context, title string, lines []string) error {
	if c.closed {
		return errors.New("terminal is closed")
	}
	offset, previous := 0, ""
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		width, height, err := term.GetSize(int(c.output.Fd()))
		if err != nil {
			return err
		}
		if width < 32 || height < 10 {
			return errors.New("enlarge the terminal to at least 32 columns and 10 rows, then try again")
		}
		wrapped := wrapLines(lines, width-1)
		rows := height - 5
		offset = min(offset, max(0, len(wrapped)-rows))
		var screen strings.Builder
		screen.WriteString("\x1b[H\x1b[2J" + fit(title, width-1) + "\r\n\r\n")
		for _, line := range wrapped[offset:min(len(wrapped), offset+rows)] {
			screen.WriteString(line + "\r\n")
		}
		screen.WriteString("\r\n" + fit(fmt.Sprintf("Lines %d-%d of %d | j/k/Space", offset+1, min(len(wrapped), offset+rows), len(wrapped)), width-1) + "\r\n" + fit("Enter: continue | Esc/q: back", width-1))
		if screen.String() != previous {
			if _, err := fmt.Fprint(c.output, screen.String()); err != nil {
				return err
			}
			previous = screen.String()
		}
		pressed, err := readKey(ctx, c.input)
		if err != nil {
			return err
		}
		switch pressed {
		case keyBack:
			return ErrCancelled
		case keyEnter:
			return nil
		case keyUp:
			offset = max(0, offset-1)
		case keyDown:
			offset++
		case keyToggle:
			offset += rows
		}
	}
}

// Confirmation repeats the consequential counts even in a 32x10 terminal.
// Back is always the initial choice; Enter or EOF can never imply consent.
func (c *Console) confirm(ctx context.Context, plan engine.Plan) (bool, error) {
	apply, previous := false, ""
	for {
		if err := ctx.Err(); err != nil {
			return false, err
		}
		width, height, err := term.GetSize(int(c.output.Fd()))
		if err != nil {
			return false, err
		}
		if width < 32 || height < 10 {
			return false, errors.New("enlarge the terminal to at least 32 columns and 10 rows, then try again")
		}
		lines := append([]string{"Apply this exact plan?"}, planSummary(plan)...)
		backMark, applyMark := "> ", "  "
		if apply {
			backMark, applyMark = "  ", "> "
		}
		lines = append(lines, "", backMark+"Back without applying", applyMark+"Apply reviewed changes", "", "j/k: move | Enter: choose")
		for i := range lines {
			lines[i] = fit(lines[i], width-1)
		}
		screen := "\x1b[H\x1b[2J" + strings.Join(lines, "\r\n")
		if screen != previous {
			if _, err := fmt.Fprint(c.output, screen); err != nil {
				return false, err
			}
			previous = screen
		}
		pressed, err := readKey(ctx, c.input)
		if err != nil {
			return false, err
		}
		switch pressed {
		case keyBack:
			return false, ErrCancelled
		case keyUp, keyDown:
			apply = !apply
		case keyEnter:
			return apply, nil
		}
	}
}

func plain(text string) string {
	return strings.Map(func(r rune) rune {
		if r < 32 || r == 127 || r >= 0x80 && r < 0xa0 {
			return -1
		}
		return r
	}, text)
}

func wrapLines(lines []string, cells int) []string {
	result := []string{}
	for _, line := range lines {
		clean := plain(line)
		for len(clean) > 0 {
			part := fit(clean, cells)
			result = append(result, part)
			clean = clean[len(part):]
		}
		if line == "" {
			result = append(result, "")
		}
	}
	return result
}
