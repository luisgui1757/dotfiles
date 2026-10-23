// Package terminal implements the installer's keyboard-only selection surface.
// It returns choices; it never probes or mutates packages, configuration or state.
package terminal

import (
	"context"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"

	"golang.org/x/term"
)

// ErrBack leaves the current screen; ErrCancelled quits the installer.
var (
	ErrBack      = errors.New("back")
	ErrCancelled = errors.New("cancelled")
)

type Option struct {
	ID, Label, Detail, Category string
}

// Category rows exist only in the selector. Results always contain leaf IDs.
type selectionRow struct {
	Option
	members []string
}

func selectionRows(options []Option, multiple bool) []selectionRow {
	rows := []selectionRow{}
	seen := map[string]bool{}
	for _, option := range options {
		if !multiple || option.Category == "" {
			rows = append(rows, selectionRow{Option: option, members: []string{option.ID}})
			continue
		}
		if seen[option.Category] {
			continue
		}
		seen[option.Category] = true
		header := selectionRow{Option: Option{Label: option.Category}}
		children := []selectionRow{}
		for _, child := range options {
			if child.Category == option.Category {
				header.members = append(header.members, child.ID)
				children = append(children, selectionRow{Option: child, members: []string{child.ID}})
			}
		}
		rows = append(rows, header)
		rows = append(rows, children...)
	}
	return rows
}

func (row selectionRow) selectedCount(selected map[string]bool) int {
	count := 0
	for _, id := range row.members {
		if selected[id] {
			count++
		}
	}
	return count
}

type key int

const (
	keyNone key = iota
	keyUp
	keyDown
	keyToggle
	keyEnter
	keyBack
	keyQuit
	keyDetails
	keyAll
	keyNoneSelected
	keyResize
)

// Console is one full-screen session. It stays on the alternate screen across
// screens, so the shell beneath never flashes between choices. Raw input exists
// only while a screen reads keys: between screens, Ctrl-C interrupts normally.
type Console struct {
	input, output *os.File
	restore       func() error
	restoreInput  func() error
	closed        bool
}

// Open refuses redirected input/output before changing terminal state. Machine
// requests have a separate explicit interface; EOF must never imply "all".
func Open(input, output *os.File) (*Console, error) {
	if !term.IsTerminal(int(input.Fd())) || !term.IsTerminal(int(output.Fd())) {
		return nil, errors.New("interactive setup needs a terminal; use an explicit machine request for automation")
	}
	restore, err := prepareOutput(output)
	if err != nil {
		return nil, err
	}
	c := &Console{input: input, output: output, restore: restore}
	if _, err := fmt.Fprint(output, "\x1b[?1049h\x1b[?25l"); err != nil {
		return nil, errors.Join(err, c.Close())
	}
	return c, nil
}

func (c *Console) Close() error {
	if c.closed {
		return nil
	}
	c.closed = true
	_, err := fmt.Fprint(c.output, "\x1b[?25h\x1b[?1049l")
	return errors.Join(err, c.restore())
}

// interact reads keys in raw mode and always restores cooked input afterwards,
// so no raw mode or reader survives into elevation or provider prompts.
func (c *Console) interact(read func() error) error {
	if c.closed {
		return errors.New("terminal is closed")
	}
	if err := c.enterRaw(); err != nil {
		return err
	}
	return errors.Join(read(), c.leaveRaw())
}

func (c *Console) enterRaw() error {
	restore, err := prepareInput(c.input)
	if err != nil {
		return err
	}
	c.restoreInput = restore
	return nil
}

func (c *Console) leaveRaw() error {
	if c.restoreInput == nil {
		return nil
	}
	err := c.restoreInput()
	c.restoreInput = nil
	return err
}

// cooked runs slow work inside a screen with ordinary input, so Ctrl-C
// interrupts it like any other inspection. Keys typed meanwhile are discarded
// on return: they answered nothing that was on the screen.
func (c *Console) cooked(ctx context.Context, work func() error) error {
	if err := c.leaveRaw(); err != nil {
		return err
	}
	workErr := work()
	if err := c.enterRaw(); err != nil {
		return errors.Join(workErr, err)
	}
	return errors.Join(workErr, discardPending(ctx, c.input))
}

// Busy replaces a finished screen while the installer inspects the system, so
// a slow preview never looks like a frozen menu.
func (c *Console) Busy(message string) error {
	if c.closed {
		return errors.New("terminal is closed")
	}
	width, _, err := term.GetSize(int(c.output.Fd()))
	if err != nil {
		return err
	}
	_, err = fmt.Fprint(c.output, "\x1b[H\x1b[2J"+fit("Dotfiles", width-1)+"\r\n\r\n"+fit(message, width-1)+"\r\n\r\n"+fit("Ctrl+C: quit", width-1))
	return err
}

// Choose supports both a single-action menu and multiple tool checkboxes.
// Every call reads synchronously: no background reader may steal input from
// sudo, a package manager, or another prompt after the selector returns.
func (c *Console) Choose(ctx context.Context, title string, options []Option, initial []string, multiple bool) (result []string, resultErr error) {
	resultErr = c.interact(func() (err error) {
		result, err = c.choose(ctx, title, options, initial, multiple, nil)
		return err
	})
	return result, resultErr
}

// ToolScreen adds the recorded selection and live status to tool selection.
// Status is polled while the screen waits for keys and may become ready late.
type ToolScreen struct {
	Installed []string
	Status    func() (summary string, labels map[string]string)
}

func (c *Console) ChooseTools(ctx context.Context, title string, options []Option, initial []string, tools ToolScreen) (result []string, resultErr error) {
	resultErr = c.interact(func() (err error) {
		result, err = c.choose(ctx, title, options, initial, true, &tools)
		return err
	})
	return result, resultErr
}

func (c *Console) choose(ctx context.Context, title string, options []Option, initial []string, multiple bool, tools *ToolScreen) ([]string, error) {
	if len(options) == 0 {
		return nil, errors.New("there are no available choices")
	}
	known, selected := map[string]bool{}, map[string]bool{}
	for _, option := range options {
		if option.ID == "" || known[option.ID] {
			return nil, errors.New("selection has an empty or duplicate choice")
		}
		known[option.ID] = true
	}
	for _, id := range initial {
		if !known[id] {
			return nil, fmt.Errorf("selection contains unavailable choice %q", id)
		}
		selected[id] = true
	}
	rows := selectionRows(options, multiple)
	cursor := 0
	if !multiple && len(initial) == 1 {
		cursor = max(0, slices.IndexFunc(rows, func(row selectionRow) bool { return row.ID == initial[0] }))
	}
	previous := ""
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		width, height, err := term.GetSize(int(c.output.Fd()))
		if err != nil {
			return nil, fmt.Errorf("read terminal dimensions: %w", err)
		}
		if width < 32 || height < 10 {
			return nil, errors.New("enlarge the terminal to at least 32 columns and 10 rows, then try again")
		}
		summary, labels := "", map[string]string(nil)
		if tools != nil {
			summary, labels = tools.summary(options, selected)
		}
		screen := render(title, rows, selected, cursor, multiple, width, height, summary, labels)
		if screen != previous {
			if _, err := fmt.Fprint(c.output, screen); err != nil {
				return nil, err
			}
			previous = screen
		}
		pressed, err := readKey(ctx, c.input)
		if err != nil {
			return nil, err
		}
		switch pressed {
		case keyBack:
			return nil, ErrBack
		case keyQuit:
			return nil, ErrCancelled
		case keyUp:
			cursor = (cursor + len(rows) - 1) % len(rows)
		case keyDown:
			cursor = (cursor + 1) % len(rows)
		case keyToggle:
			if multiple {
				row := rows[cursor]
				check := row.selectedCount(selected) != len(row.members)
				for _, id := range row.members {
					selected[id] = check
				}
			}
		case keyAll, keyNoneSelected:
			if multiple {
				for _, option := range options {
					selected[option.ID] = pressed == keyAll
				}
			}
		case keyEnter:
			if !multiple {
				return []string{rows[cursor].ID}, nil
			}
			result := []string{}
			for _, option := range options {
				if selected[option.ID] {
					result = append(result, option.ID)
				}
			}
			return result, nil
		}
	}
}

// summary counts the pending change against the recorded selection, followed
// by the status of installed tools once it is known.
func (tools *ToolScreen) summary(options []Option, selected map[string]bool) (string, map[string]string) {
	added, removed := 0, 0
	for _, option := range options {
		installed := slices.Contains(tools.Installed, option.ID)
		if selected[option.ID] && !installed {
			added++
		} else if !selected[option.ID] && installed {
			removed++
		}
	}
	parts := []string{}
	if added+removed != 0 {
		parts = append(parts, fmt.Sprintf("Add %d, remove %d", added, removed))
	}
	var labels map[string]string
	if tools.Status != nil {
		status := ""
		status, labels = tools.Status()
		if status != "" {
			parts = append(parts, status)
		}
	}
	return strings.Join(parts, " | "), labels
}

func render(title string, options []selectionRow, selected map[string]bool, cursor int, multiple bool, width, height int, summary string, labels map[string]string) string {
	var b strings.Builder
	b.WriteString("\x1b[H\x1b[2J")
	line := func(s string) { b.WriteString(fit(s, width-1)); b.WriteString("\r\n") }
	line("Dotfiles")
	line(title)
	if multiple {
		line("j/k: move | Space: toggle")
		line("a: all | n: none | Enter: next")
		// The footer includes the back key even on narrow terminals.
	} else {
		line("j/k: move | Enter: choose")
		line("")
	}
	line(summary)
	rows := height - 9
	start := cursor - rows/2
	if start < 0 {
		start = 0
	}
	if start+rows > len(options) {
		start = max(0, len(options)-rows)
	}
	end := min(len(options), start+rows)
	for i := start; i < end; i++ {
		pointer, mark := "  ", ""
		if i == cursor {
			pointer = "> "
		}
		if multiple {
			mark = "[ ] "
			count := options[i].selectedCount(selected)
			if count == len(options[i].members) {
				mark = "[x] "
			} else if count > 0 {
				mark = "[-] "
			}
			if options[i].Category != "" {
				mark = "  " + mark
			}
		}
		label := options[i].Label
		if status := labels[options[i].ID]; status != "" && options[i].ID != "" {
			label += " (" + status + ")"
		}
		line(pointer + mark + label)
	}
	line("")
	detail := options[cursor].Detail
	if multiple && options[cursor].ID == "" {
		detail = fmt.Sprintf("%d of %d selected | Space toggles", options[cursor].selectedCount(selected), len(options[cursor].members))
	} else if multiple && options[cursor].Category != "" {
		detail = options[cursor].Category + ": " + detail
	}
	line(detail)
	line(fmt.Sprintf("%d/%d | Esc: back | Ctrl+C: quit", cursor+1, len(options)))
	return b.String()
}

// Terminal labels are plain text. Strip control characters so even future
// externally supplied descriptions cannot inject terminal escape sequences.
// Non-ASCII runes conservatively reserve two cells to avoid wrapped rows.
func fit(s string, cells int) string {
	var b strings.Builder
	for _, r := range s {
		if r < 32 || r == 127 || r >= 0x80 && r < 0xa0 {
			continue
		}
		cost := 1
		if r > 127 {
			cost = 2
		}
		if cells < cost {
			break
		}
		b.WriteRune(r)
		cells -= cost
	}
	return b.String()
}

func characterKey(r rune) key {
	switch r {
	case 3, 4:
		return keyQuit
	case 27, 'q':
		return keyBack
	case 'd':
		return keyDetails
	case '\r', '\n':
		return keyEnter
	case ' ', 'x':
		return keyToggle
	case 'k':
		return keyUp
	case 'j':
		return keyDown
	case 'a':
		return keyAll
	case 'n':
		return keyNoneSelected
	default:
		return keyNone
	}
}
