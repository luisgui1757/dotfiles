package installer

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

func TestXZArchiveRetainsTarSafetyAndDecoderIntegrity(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Zen's XZ archive is Linux-only")
	}
	if _, err := os.Stat("/usr/bin/xz"); err != nil {
		t.Fatal("Linux extraction fixture requires xz", err)
	}
	for _, name := range []string{"tool", "../outside"} {
		plain := debTarFixture(t, []archiveFixtureEntry{{Name: name, Text: "fixture", Mode: 0755}})
		cmd := exec.Command("/usr/bin/xz", "--compress", "--stdout")
		cmd.Stdin = bytes.NewReader(plain)
		encoded, err := cmd.Output()
		if err != nil {
			t.Fatal(err)
		}
		pin, client, _ := archivePinFor(t, encoded, "tar.xz")
		err = downloadArchive(context.Background(), client, pin, filepath.Join(t.TempDir(), "payload"))
		if (err == nil) != (name == "tool") {
			t.Fatal("XZ bypassed checked tar extraction", name, err)
		}
		if name == "tool" {
			// Keep the outer SHA valid while damaging the decoder's own trailer.
			encoded[len(encoded)-3] ^= 0xff
			pin, client, _ = archivePinFor(t, encoded, "tar.xz")
			if err := downloadArchive(context.Background(), client, pin, filepath.Join(t.TempDir(), "payload")); err == nil {
				t.Fatal("decoder checksum failure was ignored")
			}
		}
	}
}
