package terminal

import (
	"errors"
	"io"
	"os"
	"strings"
	"sync"
	"unicode/utf8"

	"golang.org/x/term"
)

const progressIndent = "    "

// progressView prints approved work on the ordinary screen, where it stays in
// the scrollback. Installer steps stand out; native output follows indented.
// Native output is untrusted text, so complete escape sequences and controls
// are removed. A carriage return rewrites the current line, as package
// managers expect. Writes never fail: a display failure must not interrupt a
// provider command, so the first one is kept and returned with the report.
type progressView struct {
	mutex   sync.Mutex
	output  *os.File
	restore func() error
	pending []byte
	open    bool
	err     error
	// clixml is set inside a PowerShell CLIXML document; skip marks hidden
	// text ended by a carriage return, whose line feed then adds no blank line.
	clixml, skip bool
}

func (v *progressView) Step(text string) {
	v.mutex.Lock()
	defer v.mutex.Unlock()
	v.finish()
	v.clixml, v.skip = false, false
	v.write("\x1b[1m==> " + plain(text) + "\x1b[0m\n")
}

func (v *progressView) Write(data []byte) (int, error) {
	v.mutex.Lock()
	defer v.mutex.Unlock()
	for _, b := range data {
		switch b {
		case '\n':
			v.emit(true)
		case '\r':
			v.emit(false)
		default:
			v.pending = append(v.pending, b)
			if len(v.pending) >= 4096 {
				// Wrap an endless line at a character boundary.
				cut := len(v.pending) - 1
				for cut > 0 && !utf8.RuneStart(v.pending[cut]) {
					cut--
				}
				rest := append([]byte(nil), v.pending[cut:]...)
				v.pending = v.pending[:cut]
				v.emit(true)
				v.pending = append(v.pending, rest...)
			}
		}
	}
	return len(data), nil
}

// Close ends a partial line before the result report, restores the output
// mode and returns the first display failure.
func (v *progressView) Close() error {
	v.mutex.Lock()
	defer v.mutex.Unlock()
	v.finish()
	var err error
	if v.restore != nil {
		err = v.restore()
	}
	return errors.Join(v.err, err)
}

func (v *progressView) finish() {
	if len(v.pending) > 0 || v.open {
		v.emit(true)
	}
}

// emit displays the pending segment: final segments end their line, others
// stay rewritable until the next carriage return or newline.
func (v *progressView) emit(final bool) {
	text := cleanOutput(string(v.pending))
	v.pending = v.pending[:0]
	if v.hidden(text) {
		if final && v.open {
			v.write("\n") // Keep the line a carriage return displayed.
			v.open = false
		}
		v.skip = !final
		return
	}
	skip := v.skip
	v.skip = false
	if skip && final && text == "" {
		return
	}
	rewind := ""
	if v.open {
		rewind = "\r\x1b[K"
	}
	if !final {
		if text != "" {
			if width, _, err := term.GetSize(int(v.output.Fd())); err == nil {
				text = fit(text, width-1-len(progressIndent))
			}
			v.write(rewind + progressIndent + text)
			v.open = true
		}
		return
	}
	if text != "" {
		v.write(rewind + progressIndent + text + "\n")
	} else {
		v.write("\n") // After CRLF, keep the line the carriage return displayed.
	}
	v.open = false
}

// Windows PowerShell children serialize their progress and information
// records as CLIXML on stderr: a "#< CLIXML" marker, then an <Objs> document
// that the 4096-byte wrap can split. The plain text appears as ordinary lines
// too, so the display hides the markup; captured evidence and the apply log
// keep every byte, and protocol parsers still refuse CLIXML.
func (v *progressView) hidden(text string) bool {
	trimmed := strings.TrimSpace(text)
	switch {
	case v.clixml:
	case trimmed == "#< CLIXML":
		return true
	case strings.HasPrefix(trimmed, "<Objs Version="):
	default:
		return false
	}
	v.clixml = !strings.HasSuffix(trimmed, "</Objs>")
	return true
}

func (v *progressView) write(text string) {
	if v.err == nil {
		_, v.err = io.WriteString(v.output, text)
	}
}

// cleanOutput removes whole escape sequences, not only their ESC byte, so
// colored output does not leave parameter debris, and drops other controls.
func cleanOutput(text string) string {
	var b strings.Builder
	for i := 0; i < len(text); {
		r, size := utf8.DecodeRuneInString(text[i:])
		i += size
		switch {
		case r == 0x1b:
			i = skipEscape(text, i)
		case r == '\t' || r >= 32 && r != 127 && (r < 0x80 || r >= 0xa0):
			b.WriteRune(r)
		}
	}
	return b.String()
}

// skipEscape returns the index after the sequence introduced at text[i-1].
func skipEscape(text string, i int) int {
	if i >= len(text) {
		return i
	}
	switch text[i] {
	case '[':
		for i++; i < len(text); i++ {
			if text[i] >= 0x40 && text[i] <= 0x7e {
				return i + 1
			}
		}
	case ']', 'P', 'X', '^', '_':
		for i++; i < len(text); i++ {
			if text[i] == 0x07 {
				return i + 1
			}
			if text[i] == 0x1b && i+1 < len(text) && text[i+1] == '\\' {
				return i + 2
			}
		}
	default:
		for i < len(text) && text[i] >= 0x20 && text[i] <= 0x2f {
			i++
		}
		if i < len(text) {
			i++
		}
	}
	return i
}
