package installer

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestGitDanglingParentIsResourceLocal(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX dangling directory link")
	}
	for _, recorded := range []bool{false, true} {
		t.Run(map[bool]string{false: "new-selection", true: "recorded-receipt"}[recorded], func(t *testing.T) {
			c, source, target := gitSettingsFixture(t)
			if recorded {
				dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"git"}})
				if err := os.Rename(filepath.Dir(target), filepath.Dir(target)+"-saved"); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.MkdirAll(filepath.Dir(filepath.Dir(target)), 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(filepath.Join(c.Home, "missing-personal-config"), filepath.Dir(target)); err != nil {
				t.Fatal(err)
			}
			folders := c.Driver.(*NativeDriver).Configurations.Folders
			rebuilt, err := NewNativeController(filepath.Dir(filepath.Dir(source)), c.Source, filepath.Dir(c.StatePath), nativePlatform(t), folders)
			if err != nil {
				t.Fatal("bad Git path prevented controller construction", err)
			}
			rebuilt.Catalog = c.Catalog
			selected := []string{"git"}
			if recorded {
				selected = []string{}
			}
			preview, err := rebuilt.Preview(context.Background(), Request{Schema: 1, Mode: "apply", Selected: selected})
			if err != nil {
				t.Fatal("bad Git path prevented an independent preview", err)
			}
			op := operation(t, preview, gitSettingsResource)
			if !op.Observed.Unknown || !strings.Contains(op.Observed.Pending, "missing-personal-config") {
				t.Fatal("the real path error was not preserved locally", op)
			}
		})
	}
}
