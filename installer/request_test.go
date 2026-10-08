package installer

import "testing"

func TestMachineApplyRequiresAnExplicitSelection(t *testing.T) {
	for _, input := range []string{
		`{"schema":1,"mode":"apply"}`,
		`{"schema":1,"mode":"apply","selected":null}`,
	} {
		var request Request
		if err := Decode([]byte(input), &request); err == nil {
			t.Errorf("missing selection authorized deselection: %s", input)
		}
	}
	for _, input := range []string{
		`{"schema":1,"mode":"apply","selected":[]}`,
		`{"schema":1,"mode":"apply","selected":["neovim"]}`,
		`{"schema":1,"mode":"apply","retry":true}`,
		`{"schema":1,"mode":"update"}`,
		`{"schema":1,"mode":"repair"}`,
		`{"schema":1,"mode":"check"}`,
		`{"schema":1,"mode":"abandon"}`,
	} {
		var request Request
		if err := Decode([]byte(input), &request); err != nil {
			t.Errorf("explicit or retained intent was rejected: %s: %v", input, err)
		}
	}
}
