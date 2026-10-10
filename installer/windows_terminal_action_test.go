package installer

import (
	"bytes"
	"context"
	"os"
	"testing"
)

func TestWindowsTerminalCloseTabEquivalentCommandFingerprint(t *testing.T) {
	projection := func(command string) []byte {
		t.Helper()
		data, err := windowsTerminalProjectionData(windowsTerminalProjection{Values: map[string][]byte{"close-tab/command": []byte(command)}})
		if err != nil {
			t.Fatal(err)
		}
		return data
	}
	original := projection(`"closeTab"`)
	want, err := profileBlockFingerprint(windowsTerminalResource, [][]byte{original})
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		command string
		equal   bool
	}{
		{`{"action":"closeTab"}`, true},
		{`{ "action": /* native equivalent */ "closeTab" }`, true},
		{`{"action":"closeTab","index":0}`, false},
		{`{"action":"closeTab","extra":true}`, false},
		{`{"action":"closePane"}`, false},
		{`"closePane"`, false},
	} {
		t.Run(test.command, func(t *testing.T) {
			got, err := profileBlockFingerprint(windowsTerminalResource, [][]byte{projection(test.command)})
			if err != nil || (got == want) != test.equal {
				t.Fatal("close-tab semantic equality", got == want, test.equal, err)
			}
		})
	}
	p, err := decodeWindowsTerminalProjection(original)
	if err != nil || !bytes.Equal(p.Values["close-tab/command"], []byte(`"closeTab"`)) {
		t.Fatal("fingerprinting changed raw restoration value", p, err)
	}
}

func TestWindowsTerminalNativeCloseTabSaveKeepsOwnershipAndRawBaseline(t *testing.T) {
	c, _, path := windowsTerminalController(t)
	original := `{"actions":[{"id":"Dotfiles.CloseTab","command":"close\u0054ab"}]}`
	writeConfigFixture(t, path, original)
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"test"}, Adopt: []string{windowsTerminalResource}})
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	data = bytes.ReplaceAll(data, []byte(`"command":"closeTab"`), []byte(`"command":{"action":"closeTab"}`))
	if !bytes.Contains(data, []byte(`"command":{"action":"closeTab"}`)) {
		t.Fatal("fixture did not change the native command spelling", string(data))
	}
	writeConfigFixture(t, path, string(data))
	check, err := c.Dispatch(context.Background(), Request{Schema: 1, Mode: "check"})
	if err != nil || check.Status != "ready" {
		t.Fatal("native close-tab serialization lost ownership", check, err)
	}
	dispatchApproved(t, c, Request{Schema: 1, Mode: "update"})
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{}})
	data, err = os.ReadFile(path)
	if err != nil || !bytes.Contains(data, []byte(`"command":"close\u0054ab"`)) {
		t.Fatal("removal did not restore original raw action", string(data), err)
	}
}
