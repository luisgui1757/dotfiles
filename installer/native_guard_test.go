package installer

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNativeControllerObservationsCannotBypassWorker(t *testing.T) {
	home, err := resolveConfigPath(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	repository, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	folders := ConfigFolders{Home: home, Config: filepath.Join(home, ".config"), Documents: filepath.Join(home, "documents"), LocalAppData: filepath.Join(home, "local"), AppData: filepath.Join(home, "roaming")}
	state := filepath.Join(home, "state")
	c, err := NewNativeController(repository, "native-guard", state, nativePlatform(t), folders)
	if err != nil {
		t.Fatal(err)
	}
	worker, err := workerFixture(filepath.Join(state, "native", "worker"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := worker.close(); err != nil {
			t.Error(err)
		}
	})
	request := Request{Schema: 1, Mode: "apply", Selected: []string{}}
	if _, err := c.Preview(context.Background(), request); err == nil || !strings.Contains(err.Error(), "mutation lock") {
		t.Fatal("preview bypassed a live native worker", err)
	}
	// Provider exclusion must happen before loading or changing core intent,
	// including abandonment. An invalid state makes the ordering observable.
	if err := os.WriteFile(c.StatePath, []byte("invalid-state"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"apply", "abandon"} {
		request.Mode, request.ExpectedPlan = mode, strings.Repeat("a", 64)
		if _, err := c.Dispatch(context.Background(), request); err == nil || !strings.Contains(err.Error(), "mutation lock") {
			t.Fatal("mutation bypassed provider exclusion before loading state", mode, err)
		}
	}
	data, err := os.ReadFile(c.StatePath)
	if err != nil || string(data) != "invalid-state" {
		t.Fatal("blocked controller changed state", err)
	}
}
