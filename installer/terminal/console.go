// Package terminal implements the installer's keyboard-only selection surface.
// It returns choices; it never probes or mutates packages, configuration or state.
package terminal

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"golang.org/x/term"
)

var ErrCancelled = errors.New("cancelled")

type Option struct {
	ID, Label, Detail string
}

type key int

const (
	keyNone key = iota
	keyUp
	keyDown
	keyToggle
	keyEnter
	keyBack
	keyAll
	keyNoneSelected
	keyResize
)

type Console struct {
	input, output *os.File
	restore       func() error
	closed        bool
}

// Open refuses redirected input/output before changing terminal state. Machine
// requests have a separate explicit interface; EOF must never imply "all".
func Open(input, output *os.File) (*Console, error) {
	if !term.IsTerminal(int(input.Fd())) || !term.IsTerminal(int(output.Fd())) {
		return nil, errors.New("interactive setup needs a terminal; use an explicit machine request for automation")
	}
	restore, err := prepare(input, output)
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

// Choose supports both a single-action menu and multiple tool checkboxes.
// Every call reads synchronously: no background reader may steal input from
// sudo, a package manager, or another prompt after the selector returns.
func (c *Console) Choose(ctx context.Context, title string, options []Option, initial []string, multiple bool) ([]string, error) {
	if c.closed {
		return nil, errors.New("terminal is closed")
	}
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
	cursor := 0
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
		screen := render(title, options, selected, cursor, multiple, width, height)
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
			return nil, ErrCancelled
		case keyUp:
			cursor = (cursor + len(options) - 1) % len(options)
		case keyDown:
			cursor = (cursor + 1) % len(options)
		case keyToggle:
			if multiple {
				selected[options[cursor].ID] = !selected[options[cursor].ID]
			}
		case keyAll, keyNoneSelected:
			if multiple {
				for _, option := range options {
					selected[option.ID] = pressed == keyAll
				}
			}
		case keyEnter:
			if !multiple {
				return []string{options[cursor].ID}, nil
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

func render(title string, options []Option, selected map[string]bool, cursor int, multiple bool, width, height int) string {
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
	line("")
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
			if selected[options[i].ID] {
				mark = "[x] "
			}
		}
		line(pointer + mark + options[i].Label)
	}
	line("")
	line(options[cursor].Detail)
	line(fmt.Sprintf("%d/%d | Esc/q: back", cursor+1, len(options)))
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
	case 3, 4, 27, 'q':
		return keyBack
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
