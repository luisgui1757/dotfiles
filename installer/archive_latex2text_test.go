package installer

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestArchiveLatexConsoleRevisionUpgradesReadableLegacyGeneration(t *testing.T) {
	d, commands := latexPreparationFixture(t)
	r := Resource{ID: "tool.shared"}
	receipt := Receipt{OperationID: strings.Repeat("a", 64), Status: "in-progress"}
	before, err := d.Observe(context.Background(), r, receipt)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.Apply(context.Background(), r, Operation{Action: "install", Observed: before}, receipt); err != nil {
		t.Fatal(err)
	}
	// Model the actual old persisted shape: no console_revision in either the
	// published version or its matching intent. Payload ownership is unchanged.
	version, err := d.version(r.ID, receipt.OperationID)
	if err != nil {
		t.Fatal(err)
	}
	intent, err := d.readIntent(r.ID, receipt.OperationID)
	if err != nil {
		t.Fatal(err)
	}
	version.Pin.Latex2text.ConsoleRevision = 0
	intent.Pin.Latex2text.ConsoleRevision = 0
	oldPin, err := json.Marshal(version.Pin)
	if err != nil || strings.Contains(string(oldPin), "console_revision") {
		t.Fatal("legacy shape changed", err)
	}
	var restored ArchivePin
	if err := Decode(oldPin, &restored); err != nil || restored.Validate() != nil || archivePinID(restored) != archivePinID(version.Pin) {
		t.Fatal("legacy identity changed during reading", err)
	}
	versionPath := filepath.Join(d.versionDirectory(r.ID, receipt.OperationID), "version.json")
	if err := saveDocument(versionPath, version); err != nil {
		t.Fatal(err)
	}
	if err := saveDocument(d.intentPath(r.ID, receipt.OperationID), intent); err != nil {
		t.Fatal(err)
	}
	oldDocument, err := os.ReadFile(versionPath)
	if err != nil {
		t.Fatal(err)
	}
	observed, err := d.Observe(context.Background(), r, receipt)
	if err != nil || !observed.Present || observed.Healthy || observed.Adoptable || observed.Pending != "" {
		t.Fatal("old complete generation is not available for an ordinary update", observed, err)
	}
	if len(*commands) != 6 {
		t.Fatal("read-only check executed preparation")
	}
	receipt.After, receipt.Ownership, receipt.OperationID = observed, "created", strings.Repeat("b", 64)
	after, err := d.Apply(context.Background(), r, Operation{Action: "update", Observed: observed}, receipt)
	if err != nil || !after.Healthy || after.CompletedOperation != receipt.OperationID {
		t.Fatal("console recipe change did not produce a healthy replacement", after, err)
	}
	if data, err := os.ReadFile(versionPath); err != nil || string(data) != string(oldDocument) {
		t.Fatal("update rewrote old provenance", err)
	}
}

func TestArchiveLatexConsolePreparationRefusesOldOrUnknownRecipes(t *testing.T) {
	for _, revision := range []int{-1, 0, 2} {
		d, commands := latexPreparationFixture(t)
		pin := d.Pins["tool.shared"]
		pin.Latex2text.ConsoleRevision = revision
		payload := t.TempDir()
		if err := d.prepareLatex2text(context.Background(), archiveIntent{Pin: pin}, payload); err == nil || revision == 0 && !strings.Contains(err.Error(), "preserve the original operation and payload") {
			t.Fatal("unreviewed saved preparation was executed", revision, err)
		}
		if len(*commands) != 0 {
			t.Fatal("invalid recipe reached native Python")
		}
		entries, err := os.ReadDir(payload)
		if err != nil || len(entries) != 0 {
			t.Fatal("old saved preparation changed its payload", entries, err)
		}
	}
}

func TestArchiveLatexConsoleFailureCannotPublish(t *testing.T) {
	d, _ := latexPreparationFixture(t)
	run := d.Run
	d.Run = func(ctx context.Context, command nativeCommand) ([]byte, error) {
		if slices.Contains(command.Arguments, latex2textConsole) {
			return nil, os.ErrPermission
		}
		return run(ctx, command)
	}
	r := Resource{ID: "tool.shared"}
	receipt := Receipt{OperationID: strings.Repeat("a", 64)}
	before, err := d.Observe(context.Background(), r, receipt)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.Apply(context.Background(), r, Operation{Action: "install", Observed: before}, receipt); err == nil {
		t.Fatal("failed console preparation published")
	}
	if target, err := archiveLinkTarget(d.currentLink(r.ID)); err != nil || target != "" {
		t.Fatal("failed console preparation exposed a command", target, err)
	}
}

func TestArchiveLatexConsoleRevisionIsBoundOnEveryTarget(t *testing.T) {
	for _, target := range []Context{{OS: "darwin", Arch: "arm64"}, {OS: "linux", Arch: "amd64"}, {OS: "linux", Arch: "arm64"}, {OS: "windows", Arch: "amd64"}} {
		pins, err := DefaultArchivePins(NativePlatform{Context: target, Libc: "glibc"})
		if err != nil {
			t.Fatal(err)
		}
		pin := pins["tool.latex2text"]
		if pin.Latex2text.ConsoleRevision != 1 {
			t.Fatal("default pin did not request UTF-8 console", target)
		}
		current := archivePinID(pin)
		pin.Latex2text.ConsoleRevision = 0
		if archivePinID(pin) == current {
			t.Fatal("console preparation is absent from desired identity", target)
		}
	}
}
