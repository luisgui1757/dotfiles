package installer

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"os"
	"strings"
	"testing"
	"unicode/utf16"
)

func legacyWindowsPowerShellArguments(script string) []string {
	units := utf16.Encode([]rune(script))
	data := make([]byte, len(units)*2)
	for i, unit := range units {
		binary.LittleEndian.PutUint16(data[i*2:], unit)
	}
	return []string{"-NoLogo", "-NoProfile", "-NonInteractive", "-EncodedCommand", base64.StdEncoding.EncodeToString(data)}
}

func TestWindowsPowerShellLegacyPendingCommandsStayUnchanged(t *testing.T) {
	t.Run("runtime", func(t *testing.T) {
		f := newWindowsVendorFixture(t)
		f.interrupt = true
		if _, err := f.apply(); err == nil {
			t.Fatal("expected pending operation")
		}
		intent, err := f.driver.readIntent(f.receipt.OperationID)
		if err != nil {
			t.Fatal(err)
		}
		intent.Commands[0].Arguments = legacyWindowsPowerShellArguments(windowsVendorInstallScript)
		path := f.driver.intentPath(intent.Operation)
		if err := saveDocument(path, intent); err != nil {
			t.Fatal(err)
		}
		before, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		runs := f.runs
		if _, err := f.apply(); err == nil || !strings.Contains(err.Error(), "saved Windows vendor command differs") {
			t.Fatal("legacy command was reinterpreted", err)
		}
		after, err := os.ReadFile(path)
		if err != nil || !bytes.Equal(before, after) || f.runs != runs {
			t.Fatal("legacy pending command was changed or executed", err)
		}
	})
	t.Run("build-tools", func(t *testing.T) {
		f := newWindowsBuildToolsFixture(t)
		f.code = 1223
		if _, err := f.apply(); err == nil {
			t.Fatal("expected pending operation")
		}
		intent, err := f.driver.readIntent(f.receipt.OperationID)
		if err != nil {
			t.Fatal(err)
		}
		intent.Commands[0].Arguments = legacyWindowsPowerShellArguments(windowsBuildToolsInstallScript)
		path := f.driver.intentPath(intent.Operation)
		if err := saveDocument(path, intent); err != nil {
			t.Fatal(err)
		}
		before, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		runs := f.runs
		if _, err := f.apply(); err == nil || !strings.Contains(err.Error(), "saved Build Tools command differs") {
			t.Fatal("legacy command was reinterpreted", err)
		}
		after, err := os.ReadFile(path)
		if err != nil || !bytes.Equal(before, after) || f.runs != runs {
			t.Fatal("legacy pending command was changed or executed", err)
		}
	})
}
