package installer

import (
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"
)

// Only called after native acceptance fails. Never dump the settings document:
// personal profiles and unrelated fields are outside this fixed projection.
func windowsTerminalManagedDiff(before, after []byte) ([]string, error) {
	values := []map[string]windowsTerminalCanonicalValue{}
	for _, data := range [][]byte{before, after} {
		block, err := extractWindowsTerminal(data)
		if err != nil {
			return nil, err
		}
		owned, err := windowsTerminalOwned(block)
		if err != nil {
			return nil, err
		}
		values = append(values, owned)
	}
	fields := []string{}
	for _, owned := range values {
		for field := range owned {
			fields = append(fields, field)
		}
	}
	slices.Sort(fields)
	fields = slices.Compact(fields)
	format := func(value windowsTerminalCanonicalValue, present bool) string {
		if !present {
			return "<absent>"
		}
		text := string(value.JSON)
		if value.Number != "" {
			text = value.Number
		}
		if len(text) > 256 {
			text = strings.ToValidUTF8(text[:256], "?") + " (truncated)"
		}
		return text
	}
	result, changed := []string{}, 0
	for _, field := range fields {
		a, aPresent := values[0][field]
		b, bPresent := values[1][field]
		if aPresent == bPresent && reflect.DeepEqual(a, b) {
			continue
		}
		changed++
		if changed <= 16 {
			result = append(result, fmt.Sprintf("%s: before=%s after=%s", field, format(a, aPresent), format(b, bPresent)))
		}
	}
	if changed > 16 {
		result = append(result, fmt.Sprintf("%d additional changed owned fields", changed-16))
	}
	return result, nil
}

func TestWindowsTerminalManagedDiffExcludesPersonalAndEquivalentValues(t *testing.T) {
	before := []byte(`{"personal":"private-before","actions":[{"id":"Dotfiles.CloseTab","command":"closeTab"}],"profiles":{"defaults":{"font":{"size":12}}}}`)
	after := []byte(`{"personal":"private-after","actions":[{"id":"Dotfiles.CloseTab","command":{"action":"closeTab"}}],"profiles":{"defaults":{"font":{"size":12.0}}}}`)
	diff, err := windowsTerminalManagedDiff(before, after)
	if err != nil || len(diff) != 0 {
		t.Fatal("equivalent or unrelated values appeared in owned diff", diff, err)
	}
	after = []byte(`{"personal":"private-after","profiles":{"defaults":{"font":{"size":14}}}}`)
	diff, err = windowsTerminalManagedDiff(before, after)
	if err != nil || len(diff) != 2 || !strings.Contains(diff[0], "<absent>") || diff[1] != "profiles/defaults/font/size: before=12 after=14" {
		t.Fatal("owned change diagnostic", diff, err)
	}
}

func TestWindowsTerminalManagedDiffBoundsChangedValuesAndCount(t *testing.T) {
	p := windowsTerminalProjection{Values: map[string][]byte{}}
	for _, field := range windowsTerminalScalars {
		p.Values[field] = []byte(`"` + strings.Repeat("x", 500) + `"`)
	}
	after, err := windowsTerminalProjectionData(p)
	if err != nil {
		t.Fatal(err)
	}
	diff, err := windowsTerminalManagedDiff(nil, after)
	if err != nil || len(diff) != 17 || !strings.Contains(diff[0], "(truncated)") || !strings.Contains(diff[16], "additional changed owned fields") || len(strings.Join(diff, "\n")) > 6000 {
		t.Fatal("unbounded owned diagnostic", diff, err)
	}
}
