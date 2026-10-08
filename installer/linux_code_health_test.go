//go:build darwin || linux

package installer

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestLinuxCodeHealthReportsExactNativeMismatch(t *testing.T) {
	for _, mismatch := range []string{"mode", "parent", "owner", "link", "healthy"} {
		t.Run(mismatch, func(t *testing.T) {
			bin := t.TempDir()
			for name, text := range map[string]string{
				"readlink": "#!/bin/sh\nif [ \"$MISMATCH\" = link ]; then printf '/personal/code\\n'; else printf '/usr/share/code/bin/code\\n'; fi\n",
				"stat":     "#!/bin/sh\ncase \"$2\" in\n%u) if [ \"$MISMATCH:$3\" = owner:/usr/share/code ]; then printf '1001\\n'; else printf '0\\n'; fi;;\n%a) if [ \"$MISMATCH:$3\" = mode:/usr/share/code ]; then printf '775\\n'; elif [ \"$MISMATCH:$3\" = parent:/usr/share ]; then printf '777\\n'; else printf '755\\n'; fi;;\nesac\n",
			} {
				if err := os.WriteFile(filepath.Join(bin, name), []byte(text), 0755); err != nil {
					t.Fatal(err)
				}
			}
			cmd := exec.Command("/bin/sh", "-c", linuxCodeHealth)
			cmd.Env = []string{"PATH=" + bin, "MISMATCH=" + mismatch}
			out, err := cmd.CombinedOutput()
			if mismatch == "healthy" {
				if err != nil || len(out) != 0 {
					t.Fatal("healthy native query changed behavior", string(out), err)
				}
				return
			}
			want := "/usr/share/code"
			if mismatch == "link" {
				want = "/usr/bin/code target=/personal/code"
			} else if mismatch == "parent" {
				want = "/usr/share owner=0 mode=777"
			} else if mismatch == "mode" {
				want += " owner=0 mode=775"
			} else {
				want += " owner=1001 mode=755"
			}
			if err == nil || !strings.Contains(string(out), want) || len(out) > 1024 {
				t.Fatal("native mismatch lost its bounded path/metadata diagnostic", string(out), err)
			}
		})
	}
}
