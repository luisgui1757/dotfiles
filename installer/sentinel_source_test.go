package installer

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// This opt-in source check renders the exact pinned upstream tree in a private
// directory. It never runs the upstream installer or writes a consumer policy.
func TestSentinelPinnedUpstreamRendererMatchesBundledPolicy(t *testing.T) {
	if os.Getenv("DOTFILES_TEST_SENTINEL_SOURCE") != "1" {
		t.Skip("set DOTFILES_TEST_SENTINEL_SOURCE=1 for the pinned upstream render proof")
	}
	data, err := os.ReadFile("sentinel/provenance.json")
	if err != nil {
		t.Fatal(err)
	}
	var source struct {
		Version       string            `json:"version"`
		ArchiveURL    string            `json:"archive_url"`
		ArchiveSHA256 string            `json:"archive_sha256"`
		Files         map[string]string `json:"files"`
	}
	if err := json.Unmarshal(data, &source); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	root := t.TempDir()
	payload := filepath.Join(root, "upstream")
	pin := ArchivePin{Version: source.Version, URL: source.ArchiveURL, SHA256: source.ArchiveSHA256, Format: "tar.gz", StripComponents: 1, RequiredFiles: []string{"tools/sentinel-lib.sh"}}
	if err := downloadArchive(ctx, nil, pin, payload); err != nil {
		t.Fatal(err)
	}
	for path, want := range source.Files {
		got, err := os.ReadFile(filepath.Join(payload, filepath.FromSlash(path)))
		if err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(got)
		if hex.EncodeToString(sum[:]) != want {
			t.Fatal("upstream source hash changed", path)
		}
	}
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Fatal("source render proof requires Bash", err)
	}
	cmd := exec.CommandContext(ctx, bash, "--noprofile", "--norc", "-c", `set -eu; source "$1/tools/sentinel-lib.sh"; sentinel_render_managed_block "$1/core" "$1/MANIFEST.json" "$1/VERSION"`, "sentinel-render", filepath.ToSlash(payload))
	cmd.Dir = root
	cmd.Env = []string{"PATH=/usr/bin:/bin", "HOME=" + filepath.ToSlash(root), "TMPDIR=" + filepath.ToSlash(root), "LC_ALL=C"}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	got, err := cmd.Output()
	if err != nil || string(got) != sentinelPolicy {
		t.Fatalf("pinned upstream renderer differs from bundled bytes: %v %s", err, stderr.String())
	}
}
