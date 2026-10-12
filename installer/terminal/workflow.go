package terminal

import (
	"context"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"

	engine "github.com/luisgui1757/dotfiles/installer"
	"golang.org/x/term"
)

// Workflow connects the native terminal to the lifecycle controller. One
// full-screen session spans every menu and review; each interaction restores
// cooked input before returning. BeginApply releases the screen before any
// approved change, provider prompt or elevation, and the progress of that
// work stays in the ordinary scrollback. Close releases a remaining screen.
type Workflow struct {
	Input, Output *os.File
	screen        *Console
	progress      *progressView
}

// open reuses the screen session; an earlier apply display ends first, for
// example when a changed system brings back the review.
func (w *Workflow) open() (*Console, error) {
	if err := w.closeProgress(); err != nil {
		return nil, err
	}
	if w.screen == nil {
		c, err := Open(w.Input, w.Output)
		if err != nil {
			return nil, err
		}
		w.screen = c
	}
	return w.screen, nil
}

// Close releases the screen and ends an apply display whose result was never
// reported, for example after a failed migration phase.
func (w *Workflow) Close() error {
	return errors.Join(w.closeScreen(), w.closeProgress())
}

func (w *Workflow) closeScreen() error {
	if w.screen == nil {
		return nil
	}
	err := w.screen.Close()
	w.screen = nil
	return err
}

func (w *Workflow) closeProgress() error {
	if w.progress == nil {
		return nil
	}
	err := w.progress.Close()
	w.progress = nil
	return err
}

func engineError(err error) error {
	switch {
	case errors.Is(err, ErrBack):
		return engine.ErrBack
	case errors.Is(err, ErrCancelled):
		return engine.ErrCancelled
	}
	return err
}

func (w *Workflow) Choose(ctx context.Context, title string, choices []engine.Choice, initial []string, multiple bool) ([]string, error) {
	c, err := w.open()
	if err != nil {
		return nil, err
	}
	options := make([]Option, 0, len(choices))
	for _, choice := range choices {
		options = append(options, Option{ID: choice.ID, Label: choice.Label, Detail: choice.Detail, Category: choice.Category})
	}
	selected, err := c.Choose(ctx, title, options, initial, multiple)
	if err == nil {
		err = c.Busy("Inspecting your system...")
	}
	return selected, engineError(err)
}

func (w *Workflow) ChooseTools(ctx context.Context, menu engine.ToolMenu) ([]string, error) {
	c, err := w.open()
	if err != nil {
		return nil, err
	}
	options := make([]Option, 0, len(menu.Choices))
	for _, choice := range menu.Choices {
		options = append(options, Option{ID: choice.ID, Label: choice.Label, Detail: choice.Detail, Category: choice.Category})
	}
	screen := ToolScreen{Installed: menu.Installed}
	if menu.Status != nil {
		screen.Status = func() (string, map[string]string) {
			status := menu.Status()
			return status.Summary, status.Tools
		}
	}
	selected, err := c.ChooseTools(ctx, menu.Title, options, menu.Initial, screen)
	if err == nil {
		err = c.Busy("Inspecting your system...")
	}
	return selected, engineError(err)
}

func (w *Workflow) Review(ctx context.Context, catalog *engine.Catalog, proposal engine.Proposal, revise engine.Revise) (bool, error) {
	c, err := w.open()
	if err != nil {
		return false, err
	}
	accepted, err := c.confirm(ctx, catalog, proposal, revise)
	return accepted, engineError(err)
}

func (w *Workflow) BeginApply(ctx context.Context) (context.Context, error) {
	if err := w.Close(); err != nil {
		return ctx, err
	}
	// Progress rewrites lines with escape sequences: keep Windows virtual
	// terminal output enabled until the display ends.
	restore, err := prepareOutput(w.Output)
	if err != nil {
		return ctx, err
	}
	w.progress = &progressView{output: w.Output, restore: restore}
	return engine.WithProgress(ctx, engine.Progress{Step: w.progress.Step, Output: w.progress}), nil
}

func (w *Workflow) Report(result engine.Result) error {
	if result.Plan.Mode == "check" {
		c, err := w.open()
		if err != nil {
			return err
		}
		lines := append([]string{reportHeadline(result.Status), ""}, strings.Split(result.Message, "\n")...)
		return engineError(c.Read(context.Background(), "Installation check", lines))
	}
	closeErr := w.Close()
	_, err := fmt.Fprint(w.Output, reportText(result))
	return errors.Join(closeErr, err)
}

func reportHeadline(status string) string {
	switch status {
	case "ready":
		return "Done: everything selected is ready."
	case "needs-action":
		return "Finished, but these items need your action:"
	case "failed":
		return "Failed."
	case "cancelled":
		return "Cancelled; nothing was changed."
	}
	return status
}

// Disclosures follow the actionable reasons under their own heading, so a
// note about an unexercised application never reads as the failure. A failure
// is one message whose continuation lines, such as native output, stay
// together beneath it.
func reportText(result engine.Result) string {
	var actions, notes []string
	for _, line := range strings.Split(result.Message, "\n") {
		line = plain(strings.TrimRight(line, " \t\r"))
		switch {
		case strings.TrimSpace(line) == "":
		case strings.HasPrefix(line, "Preserved files for inspection: "), strings.HasPrefix(line, "External application not exercised: "), strings.HasPrefix(line, "Note: "), strings.HasPrefix(line, "Apply log "):
			notes = append(notes, "  - "+line)
		case result.Status == "failed" && len(actions) > 0:
			actions = append(actions, "    "+line)
		default:
			actions = append(actions, "  - "+strings.TrimSpace(line))
		}
	}
	text := "\n==> " + reportHeadline(result.Status) + "\n"
	if len(actions) > 0 {
		text += strings.Join(actions, "\n") + "\n"
	}
	if len(notes) > 0 {
		text += "Notes (informational, not failures):\n" + strings.Join(notes, "\n") + "\n"
	}
	if result.Log != "" {
		text += "Full log: " + plain(result.Log) + "\n"
	}
	return text
}

// Retirement records are installed as durable completion evidence in the
// engine. Present their actual effect without changing any approval fields.
func displayAction(catalog *engine.Catalog, op engine.Operation) string {
	r, _ := catalog.Resource(op.Resource)
	if r.Action == "retire" && op.Action == "install" {
		if r.ID == "retire.homebrew" {
			return "change"
		}
		return "remove"
	}
	return op.Action
}

func planSummary(catalog *engine.Catalog, plan engine.Plan) []string {
	counts := map[string]int{}
	admin := 0
	for _, op := range plan.Operations {
		counts[displayAction(catalog, op)]++
		if op.Privileged && (op.Action == "install" || op.Action == "update" || op.Action == "repair" || op.Action == "adopt" || op.Action == "remove") {
			admin++
		}
	}
	return []string{
		fmt.Sprintf("Add: %d  Change: %d", counts["install"], counts["repair"]+counts["update"]+counts["adopt"]+counts["change"]),
		fmt.Sprintf("Remove: %d  Keep: %d", counts["remove"], counts["keep"]+counts["retain"]),
		fmt.Sprintf("Needs action: %d  Admin: %d", counts["pending"], admin),
	}
}

// planHighlights names removals, replacements, administrator changes and
// items that need action, so the counts are explained without the details.
func planHighlights(catalog *engine.Catalog, plan engine.Plan) []string {
	lines := []string{}
	for _, op := range plan.Operations {
		action := displayAction(catalog, op)
		r, _ := catalog.Resource(op.Resource)
		switch {
		case action == "remove" || action == "adopt" || action == "change" || op.Privileged && (op.Action == "install" || op.Action == "repair" || op.Action == "update"):
			lines = append(lines, strings.ToUpper(action)+": "+r.Name)
		case op.Action == "pending":
			lines = append(lines, "NEEDS ACTION: "+r.Name+" - "+op.Reason)
		}
	}
	return lines
}

func planText(catalog *engine.Catalog, plan engine.Plan, details string) []string {
	lines := append(planSummary(catalog, plan), planHighlights(catalog, plan)...)
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
		lines = append(lines, strings.ToUpper(displayAction(catalog, op))+": "+r.Name+" ("+r.ID+")", "Why: "+op.Reason)
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
		for _, path := range engine.ExistingPreservedPaths(op.Observed.Preserved) {
			lines = append(lines, "Preserved files for inspection: "+path)
		}
		for _, name := range op.Observed.UnverifiedApplications {
			lines = append(lines, "External application not exercised: "+name)
		}
		for _, note := range op.Observed.Notes {
			lines = append(lines, "Note: "+note)
		}
		for _, consumer := range op.Observed.Consumers {
			lines = append(lines, "Outside consumer: "+consumer)
		}
		if op.Privileged {
			lines = append(lines, "Requires administrator privileges.")
		}
		lines = append(lines, "")
	}
	if details != "" {
		lines = append(lines, "Exact reviewed scope:")
		lines = append(lines, strings.Split(details, "\n")...)
	}
	return lines
}

// Read is a scrollable document, not a clipped option detail. Every approval
// field and explanation remains reachable in narrow or resized terminals.
// Enter and Esc both close it; Ctrl-C quits.
func (c *Console) Read(ctx context.Context, title string, lines []string) error {
	return c.interact(func() error { return c.read(ctx, title, lines) })
}

func (c *Console) read(ctx context.Context, title string, lines []string) error {
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
		screen.WriteString("\r\n" + fit(fmt.Sprintf("Lines %d-%d of %d | j/k/Space", offset+1, min(len(wrapped), offset+rows), len(wrapped)), width-1) + "\r\n" + fit("Enter/Esc: back | Ctrl+C: quit", width-1))
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
		case keyBack, keyEnter:
			return nil
		case keyQuit:
			return ErrCancelled
		case keyUp:
			offset = max(0, offset-1)
		case keyDown:
			offset++
		case keyToggle:
			offset += rows
		}
	}
}

// confirmRow is one line of the confirmation's scrolling region. Choice rows,
// Apply and Back are selectable; plan highlights and group names are not.
type confirmRow struct {
	text   string
	choice int // index into the proposal's choices, or -1
	action string
}

const maxHighlights = 8

func confirmRows(catalog *engine.Catalog, proposal engine.Proposal) []confirmRow {
	rows := []confirmRow{}
	highlights := planHighlights(catalog, proposal.Plan)
	if len(highlights) > maxHighlights {
		highlights = append(highlights[:maxHighlights-1], fmt.Sprintf("+%d more | d: details", len(highlights)-maxHighlights+1))
	}
	for _, text := range highlights {
		rows = append(rows, confirmRow{text: text, choice: -1})
	}
	group := ""
	for i, choice := range proposal.Choices {
		if choice.Group != group {
			group = choice.Group
			rows = append(rows, confirmRow{choice: -1}, confirmRow{text: group + ":", choice: -1})
		}
		mark := "[ ] "
		if choice.Checked {
			mark = "[x] "
		}
		rows = append(rows, confirmRow{text: mark + choice.Label, choice: i})
	}
	return append(rows, confirmRow{choice: -1}, confirmRow{text: "Apply changes", choice: -1, action: "apply"}, confirmRow{text: "Back to main menu", choice: -1, action: "back"})
}

func (row confirmRow) selectable() bool { return row.choice >= 0 || row.action != "" }

// Confirmation leads with the consequential counts, then names removals,
// replacements and administrator changes, then the optional decisions as
// checkboxes. Toggling one recomputes the whole plan; d opens every planned
// change. By the owner's decision Apply is the initial choice. Keys typed
// before this screen appeared, or while a revision was computed, are
// discarded, so type-ahead can never approve a plan, and EOF never consents.
func (c *Console) confirm(ctx context.Context, catalog *engine.Catalog, proposal engine.Proposal, revise engine.Revise) (accepted bool, resultErr error) {
	resultErr = c.interact(func() error {
		if err := discardPending(ctx, c.input); err != nil {
			return err
		}
		rows := confirmRows(catalog, proposal)
		cursor := slices.IndexFunc(rows, func(row confirmRow) bool { return row.action == "apply" })
		offset, previous, working := 0, "", ""
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
			top := append([]string{"Apply these changes?"}, planSummary(catalog, proposal.Plan)...)
			if proposal.Notice != "" {
				top = append(top, proposal.Notice)
			}
			hint := "j/k: move  Enter: choose"
			if len(proposal.Choices) != 0 {
				hint = "j/k: move  Space: toggle  Enter: choose"
			}
			detail := working
			if detail == "" && rows[cursor].choice >= 0 {
				detail = proposal.Choices[rows[cursor].choice].Detail
			}
			bottom := []string{detail, hint, "d: details  Esc: back  ^C: quit"}
			region := max(1, height-len(top)-len(bottom))
			if cursor < offset {
				offset = cursor
			}
			if cursor >= offset+region {
				offset = cursor - region + 1
			}
			// Reveal the plan highlights above the first decision when they fit.
			if first := slices.IndexFunc(rows, confirmRow.selectable); cursor == first && cursor < region {
				offset = 0
			}
			lines := slices.Clone(top)
			for i := offset; i < min(len(rows), offset+region); i++ {
				pointer := "  "
				if i == cursor {
					pointer = "> "
				}
				if !rows[i].selectable() {
					pointer = ""
				}
				lines = append(lines, pointer+rows[i].text)
			}
			for len(lines) < len(top)+region {
				lines = append(lines, "")
			}
			lines = append(lines, bottom...)
			for i := range lines {
				lines[i] = fit(lines[i], width-1)
			}
			screen := "\x1b[H\x1b[2J" + strings.Join(lines, "\r\n")
			if screen != previous {
				if _, err := fmt.Fprint(c.output, screen); err != nil {
					return err
				}
				previous = screen
			}
			if working != "" {
				// Recompute with ordinary input, then show the revised plan.
				choices := slices.Clone(proposal.Choices)
				index := rows[cursor].choice
				choices[index].Checked = !choices[index].Checked
				var revised engine.Proposal
				if err := c.cooked(ctx, func() (err error) {
					revised, err = revise(ctx, choices)
					return err
				}); err != nil {
					return err
				}
				proposal, working = revised, ""
				focus := choices[index]
				rows = confirmRows(catalog, proposal)
				cursor = slices.IndexFunc(rows, func(row confirmRow) bool {
					return row.choice >= 0 && proposal.Choices[row.choice].Group == focus.Group && proposal.Choices[row.choice].ID == focus.ID
				})
				if cursor < 0 {
					cursor = slices.IndexFunc(rows, func(row confirmRow) bool { return row.action == "apply" })
				}
				continue
			}
			pressed, err := readKey(ctx, c.input)
			if err != nil {
				return err
			}
			move := func(step int) {
				for next := cursor + step; next >= 0 && next < len(rows); next += step {
					if rows[next].selectable() {
						cursor = next
						return
					}
				}
			}
			switch pressed {
			case keyBack:
				return ErrBack
			case keyQuit:
				return ErrCancelled
			case keyUp:
				move(-1)
			case keyDown:
				move(1)
			case keyDetails:
				if err := c.read(ctx, "Review every change", planText(catalog, proposal.Plan, proposal.Details)); err != nil {
					return err
				}
				previous = ""
			case keyToggle, keyEnter:
				switch {
				case rows[cursor].choice >= 0 && revise != nil:
					working = "Updating the plan..."
				case pressed == keyEnter && rows[cursor].action == "apply":
					accepted = true
					return nil
				case pressed == keyEnter && rows[cursor].action == "back":
					return ErrBack
				}
			}
		}
	})
	return accepted, resultErr
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
