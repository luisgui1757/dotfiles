package installer

import "os"

// LinuxClipboardCapability describes this session, not a target identity or a
// reason to omit desktop helpers from installation. Both APT helpers remain
// prerequisites so a headless install also works in a later graphical session.
// Presence advertises a potential transport; no clipboard contents are probed.
// There is deliberately no kernel/WSL detection or Windows host provisioning.
type LinuxClipboardCapability struct {
	Wayland bool
	X11     bool
	Tmux    bool
}

// DiscoverLinuxClipboardCapability belongs at the process boundary. Store no
// display addresses, runtime-directory paths or other inherited environment.
func DiscoverLinuxClipboardCapability() LinuxClipboardCapability {
	return LinuxClipboardCapability{Wayland: os.Getenv("WAYLAND_DISPLAY") != "", X11: os.Getenv("DISPLAY") != "", Tmux: os.Getenv("TMUX") != ""}
}

// Disclosures accompany successful native helper health checks. Lack of a
// graphical session does not make the installed command-line tools unhealthy;
// actual clipboard transport is chosen and verified by its runtime consumer.
func (c LinuxClipboardCapability) Disclosures() []string {
	switch {
	case c.Wayland:
		return []string{"Wayland clipboard round-trip in the runtime session (not exercised by installer checks)"}
	case c.X11:
		return []string{"X11 clipboard round-trip in the runtime session (not exercised by installer checks)"}
	case c.Tmux:
		return []string{"headless tmux/terminal clipboard transport (no graphical display advertised; verify OSC 52 in the actual terminal)"}
	default:
		return []string{"headless terminal clipboard transport (no graphical display advertised; verify OSC 52 or a runtime bridge in the actual terminal)"}
	}
}
