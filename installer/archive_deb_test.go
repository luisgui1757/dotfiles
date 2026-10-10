package installer

import (
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func debDecoderFixture(t *testing.T, script string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the Debian decoder boundary is POSIX")
	}
	path := filepath.Join(t.TempDir(), "dpkg-deb")
	contents := "#!/bin/sh\nset -eu\n[ \"$1\" = --fsys-tarfile ] && [ \"$2\" = - ]\nprintf '%s' \"$$\" > \"${0%/*}/pid\"\n" + script + "\n"
	if err := os.WriteFile(path, []byte(contents), 0755); err != nil {
		t.Fatal(err)
	}
	return path
}

func debTarFixture(t *testing.T, entries []archiveFixtureEntry) []byte {
	t.Helper()
	reader, err := gzip.NewReader(bytes.NewReader(archiveTar(t, entries)))
	if err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(reader)
	if err := errors.Join(err, reader.Close()); err != nil {
		t.Fatal(err)
	}
	return data
}

func TestDebArchiveVerifiesBytesBeforeStartingDecoder(t *testing.T) {
	decoder := debDecoderFixture(t, "exec cat")
	pin, client, _ := archivePinFor(t, []byte("corrupt package"), "deb")
	pin.SHA256 = strings.Repeat("0", 64)
	if err := downloadArchiveWithDeb(context.Background(), client, pin, filepath.Join(t.TempDir(), "payload"), decoder); err == nil {
		t.Fatal("corrupt package reached extraction")
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(decoder), "pid")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("decoder ran before SHA verification", err)
	}
}

func TestDebArchiveUsesExistingTarSafetyAndRequiredFiles(t *testing.T) {
	decoder := debDecoderFixture(t, "exec cat")
	for _, name := range []string{"tool", "../personal", "/absolute", "NUL"} {
		t.Run(name, func(t *testing.T) {
			data := debTarFixture(t, []archiveFixtureEntry{{Name: name, Text: "program", Mode: 0755}})
			pin, client, _ := archivePinFor(t, data, "deb")
			destination := filepath.Join(t.TempDir(), "payload")
			err := downloadArchiveWithDeb(context.Background(), client, pin, destination, decoder)
			if (err == nil) != (name == "tool") {
				t.Fatal("Debian stream bypassed tar validation", err)
			}
		})
	}
}

func TestDebArchiveCancelsAndReapsMalformedDecoder(t *testing.T) {
	decoder := debDecoderFixture(t, "printf '%0512d' 0\nexec sleep 30")
	pin, client, _ := archivePinFor(t, []byte("verified input"), "deb")
	started := time.Now()
	err := downloadArchiveWithDeb(context.Background(), client, pin, filepath.Join(t.TempDir(), "payload"), decoder)
	if err == nil || time.Since(started) > 5*time.Second {
		t.Fatal("malformed decoder was not cancelled promptly", err)
	}
	data, err := os.ReadFile(filepath.Join(filepath.Dir(decoder), "pid"))
	if err != nil {
		t.Fatal(err)
	}
	pid, err := strconv.Atoi(string(data))
	if err != nil {
		t.Fatal(err)
	}
	process, err := os.FindProcess(pid)
	if err != nil {
		t.Fatal(err)
	}
	if err := process.Signal(syscall.Signal(0)); err == nil {
		t.Fatal("decoder remains alive after failure")
	}
}

func TestDebArchiveBoundsDecoderDiagnostics(t *testing.T) {
	decoder := debDecoderFixture(t, "head -c 2097152 /dev/zero >&2\nexec cat")
	pin, client, _ := archivePinFor(t, debTarFixture(t, []archiveFixtureEntry{{Name: "tool", Text: "program", Mode: 0755}}), "deb")
	err := downloadArchiveWithDeb(context.Background(), client, pin, filepath.Join(t.TempDir(), "payload"), decoder)
	if err == nil || len(err.Error()) > (1<<20)+1024 {
		t.Fatal("decoder diagnostic exceeded its bound")
	}
}
