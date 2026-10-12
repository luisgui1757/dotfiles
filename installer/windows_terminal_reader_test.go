package installer

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func terminalReaderFixture(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "windows-terminal", "settings.fragment.jsonc"))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestWindowsTerminalReaderMatchesUncachedBytesAndErrors(t *testing.T) {
	fixture := terminalReaderFixture(t)
	projection, err := referenceTerminalExtract(fixture)
	if err != nil {
		t.Fatal(err)
	}
	cases := map[string][]byte{
		"absent":                    nil,
		"empty":                     []byte("{}"),
		"whitespace":                []byte(" \n"),
		"recipe":                    fixture,
		"saved-projection":          projection,
		"old-optional-fields":       []byte(`{"dotfiles_windows_terminal":1,"values":{"initialRows":"NTA="}}`),
		"foreign-projection":        []byte(`{"dotfiles_windows_terminal":1,"values":{"foreign":"NTA="}}`),
		"personal-jsonc":            []byte("\xef\xbb\xbf// original header\n{\"initialRows\":50,\"personal\":9007199254740993,\"profiles\":{\"defaults\":{\"font\":{\"face\":\"Personal Font\",\"features\":{\"ss01\":1},},},},}\n"),
		"duplicate-top-level":       []byte(`{"initialRows":50,"initialRows":60}`),
		"duplicate-owned-parent":    []byte(`{"profiles":{"defaults":{"font":{"face":"a","face":"b"}}}}`),
		"duplicate-personal-parent": []byte(`{"personal":{"a":1,"a":2}}`),
		"null-parent":               []byte(`{"profiles":null}`),
		"array-parent":              []byte(`{"profiles":{"defaults":[]}}`),
		"invalid-font":              []byte(`{"profiles":{"defaults":{"font":"personal"}}}`),
		"invalid-array":             []byte(`{"profiles":{"list":{}}}`),
		"invalid-entry":             []byte(`{"keybindings":[null]}`),
		"grouped-overlap":           []byte(`{"keybindings":[{"keys":["ctrl+c","alt+x"],"id":"personal"}]}`),
		"grouped-personal":          []byte(`{"keybindings":[{"keys":["ctrl+alt+p","alt+f9"],"id":"personal"}]}`),
		"invalid-key-type":          []byte(`{"keybindings":[{"keys":5}]}`),
		"duplicate-chord":           []byte(`{"keybindings":[{"keys":"ctrl+c","id":"copy"},{"keys":"C+CTRL","id":"copy"}]}`),
		"legacy-binding":            []byte(`{"actions":[{"keys":"ctrl+c","command":"copy"}]}`),
		"case-name":                 []byte(`{"schemes":[{"name":"Rose-Pine","red":"#abc"}]}`),
		"case-action":               []byte(`{"actions":[{"id":"dotfiles.CloseTab","command":"closeTab"}]}`),
		"guid-braces":               []byte(`{"profiles":{"list":[{"guid":"` + strings.Trim(windowsTerminalProfileGUID, "{}") + `"}]}}`),
		"guid-case":                 []byte(`{"profiles":{"list":[{"guid":"` + strings.ToUpper(windowsTerminalProfileGUID) + `","name":"Personal"}]}}`),
		"invalid-identity":          []byte(`{"schemes":[{"name":12}]}`),
		"raw-leaf":                  []byte(`{"actions":[{"id":"Dotfiles.CloseTab","command":{/*keep*/"action":"closeTab",}}]}`),
		"trailing-content":          []byte(`{} {}`),
		"unterminated-comment":      []byte(`{/* personal`),
		"invalid-utf8":              {0xff},
		"nul":                       []byte("{\"personal\":\"\x00\"}"),
		"oversized":                 bytes.Repeat([]byte(" "), maxProfileBytes+1),
	}
	for name, data := range cases {
		t.Run(name, func(t *testing.T) {
			original := bytes.Clone(data)
			want, wantErr := referenceTerminalExtract(data)
			got, gotErr := extractWindowsTerminal(data)
			message := func(err error) string {
				if err == nil {
					return ""
				}
				return err.Error()
			}
			if !bytes.Equal(got, want) || message(gotErr) != message(wantErr) {
				t.Fatalf("cached inspection changed bytes/errors: got %q / %v, want %q / %v", got, gotErr, want, wantErr)
			}
			if !bytes.Equal(data, original) {
				t.Fatal("inspection changed input")
			}
		})
	}
}

func TestWindowsTerminalReaderDoesNotRetainParsesAcrossInspections(t *testing.T) {
	data := []byte(`{"initialRows":50}`)
	first, err := extractWindowsTerminal(data)
	if err != nil || len(first) == 0 {
		t.Fatal("first owned setting was not discovered", err)
	}
	copy(data, []byte(`{"initialCols":50}`))
	got, err := extractWindowsTerminal(data)
	want, wantErr := referenceTerminalExtract(data)
	if err != nil || wantErr != nil || !bytes.Equal(got, want) || len(got) != 0 {
		t.Fatal("later personal key reused an earlier inspection", string(got), err, wantErr)
	}
}

func TestWindowsTerminalReaderSeparatesSliceWindows(t *testing.T) {
	data := []byte(`{"initialRows":50}`)
	reader := windowsTerminalReader{}
	if _, err := reader.propertiesOf(data); err != nil {
		t.Fatal(err)
	}
	if _, err := reader.propertiesOf(data[:len(data)-1]); err == nil {
		t.Fatal("shorter invalid input reused the full document parse")
	}
}

func TestWindowsTerminalExtractionAllocationBudget(t *testing.T) {
	data := terminalReaderFixture(t)
	// The unchanged recipe used 18,360 allocations before parse reuse and 4,512
	// afterwards on Go 1.27.1. Leave headroom without accepting the old cost.
	const maxExtractionAllocations = 10000
	allocations := testing.AllocsPerRun(3, func() {
		if _, err := extractWindowsTerminal(data); err != nil {
			t.Fatal(err)
		}
	})
	t.Logf("allocations per extraction: %.0f", allocations)
	if allocations > maxExtractionAllocations {
		t.Fatalf("settings inspection repeatedly reparses the same document: %.0f allocations exceed %d", allocations, maxExtractionAllocations)
	}
}
