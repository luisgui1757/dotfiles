package installer

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestLinuxClipboardCapabilityObservesSessionWithoutTargetOrPrivateValues(t *testing.T) {
	for _, test := range []struct{ name, wayland, x11, tmux, want string }{
		{"headless", "", "", "", "headless terminal"},
		{"multiplexer", "", "", "private-tmux-value", "headless tmux"},
		{"x11", "", "private-display-value", "", "X11"},
		{"wayland", "private-wayland-value", "private-display-value", "", "Wayland"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("WAYLAND_DISPLAY", test.wayland)
			t.Setenv("DISPLAY", test.x11)
			t.Setenv("TMUX", test.tmux)
			capability := DiscoverLinuxClipboardCapability()
			if capability.Wayland != (test.wayland != "") || capability.X11 != (test.x11 != "") || capability.Tmux != (test.tmux != "") {
				t.Fatal("wrong observed session", capability)
			}
			if got := capability.Disclosures(); len(got) != 1 || !strings.Contains(got[0], test.want) {
				t.Fatal("unsupported session disclosure", got)
			}
			data, err := json.Marshal(capability)
			if err != nil || strings.Contains(string(data), "private-") {
				t.Fatal("session captured environment values", string(data), err)
			}
		})
	}
}
