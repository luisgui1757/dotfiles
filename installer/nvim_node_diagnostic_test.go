package installer

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestNativeNodeArchiveDiagnosticResolvesTemporaryDirectoryAlias(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX temporary-directory aliases use symbolic links")
	}
	home, err := resolveConfigPath(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	pin, client, _ := archivePinFor(t, archiveTar(t, []archiveFixtureEntry{
		{Name: "tool", Text: "original", Mode: 0755},
		{Name: "tool-link", Link: "tool"},
	}), "tar.gz")
	d := &ArchiveDriver{Directory: filepath.Join(home, "packages"), Pins: map[string]ArchivePin{"tool.node": pin}, Client: client}
	r := Resource{ID: "tool.node"}
	o, err := d.Observe(context.Background(), r, Receipt{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.Apply(context.Background(), r, Operation{Action: "install", Observed: o}, Receipt{OperationID: strings.Repeat("a", 64)}); err != nil {
		t.Fatal(err)
	}
	payload, err := d.PayloadPath(r.ID)
	if err != nil {
		t.Fatal(err)
	}
	writeConfigFixture(t, filepath.Join(payload, "tool"), "private-user-bytes")
	before, err := snapshotTree(d.Directory, maxPackageBytes, maxPackageEntries)
	if err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(home, "temporary-alias")
	if err := os.Symlink(home, alias); err != nil {
		t.Fatal(err)
	}
	output, err := nativeNodeArchiveDiagnostic(context.Background(), d, filepath.Join(alias, "reference"))
	if err != nil || !strings.Contains(output, "entry differences=1") || strings.Contains(output, "private-user-bytes") {
		t.Fatal("temporary-directory alias hid the payload difference", output, err)
	}
	after, err := snapshotTree(d.Directory, maxPackageBytes, maxPackageEntries)
	if err != nil || after != before {
		t.Fatal("diagnostic changed installed evidence", err)
	}
	if target, err := os.Readlink(alias); err != nil || target != home {
		t.Fatal("diagnostic changed temporary-directory alias", target, err)
	}
}

func TestNativeNodeArchiveDiagnosticPreservesEvidenceAndReconstructsOnlyPayloadDrift(t *testing.T) {
	for _, change := range []string{"healthy", "content", "added", "missing", "pointer", "desired", "manifest"} {
		t.Run(change, func(t *testing.T) {
			home, err := resolveConfigPath(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			pin, client, requests := archivePinFor(t, archiveTar(t, []archiveFixtureEntry{{Name: "tool", Text: "original", Mode: 0755}}), "tar.gz")
			d := &ArchiveDriver{Directory: filepath.Join(home, "packages"), Pins: map[string]ArchivePin{"tool.node": pin}, Client: client}
			r := Resource{ID: "tool.node"}
			o, err := d.Observe(context.Background(), r, Receipt{})
			if err != nil {
				t.Fatal(err)
			}
			receipt := Receipt{OperationID: strings.Repeat("a", 64)}
			if _, err := d.Apply(context.Background(), r, Operation{Action: "install", Observed: o}, receipt); err != nil {
				t.Fatal(err)
			}
			payload, err := d.PayloadPath(r.ID)
			if err != nil {
				t.Fatal(err)
			}
			switch change {
			case "content":
				writeConfigFixture(t, filepath.Join(payload, "tool"), "private-user-bytes")
			case "added":
				writeConfigFixture(t, filepath.Join(payload, "extra"), "private-user-bytes")
			case "missing":
				if err := os.Remove(filepath.Join(payload, "tool")); err != nil {
					t.Fatal(err)
				}
			case "pointer":
				if err := os.Remove(d.currentLink(r.ID)); err != nil {
					t.Fatal(err)
				}
			case "desired":
				pin.Version = "next"
				d.Pins[r.ID] = pin
			case "manifest":
				version, err := d.version(r.ID, receipt.OperationID)
				if err != nil {
					t.Fatal(err)
				}
				version.Payload.Hash = strings.Repeat("0", 64)
				if err := saveDocument(filepath.Join(d.versionDirectory(r.ID, receipt.OperationID), "version.json"), version); err != nil {
					t.Fatal(err)
				}
			}
			before, err := snapshotTree(d.Directory, maxPackageBytes, maxPackageEntries)
			if err != nil {
				t.Fatal(err)
			}
			output, diagnosticErr := nativeNodeArchiveDiagnostic(context.Background(), d, filepath.Join(home, "reference"))
			after, err := snapshotTree(d.Directory, maxPackageBytes, maxPackageEntries)
			if err != nil || before != after || strings.Contains(output, "private-user-bytes") {
				t.Fatal("diagnostic modified evidence or revealed file contents", diagnosticErr, err)
			}
			payloadDrift := change == "content" || change == "added" || change == "missing"
			if (diagnosticErr != nil) != (change == "manifest") || strings.Contains(output, "entry differences=") != payloadDrift {
				t.Fatal("diagnostic misclassified archive state", output, diagnosticErr)
			}
			wantRequests := int32(1)
			if payloadDrift || change == "manifest" {
				wantRequests++
			}
			if requests.Load() != wantRequests {
				t.Fatal("reconstructed archive without proved payload mismatch", requests.Load())
			}
			o, err = d.Observe(context.Background(), r, receipt)
			if err != nil || o.Healthy != (change == "healthy") {
				t.Fatal("diagnostic changed the integrity verdict", o, err)
			}
		})
	}
}

func TestNativeNodeArchiveEntryDiffBoundsReportedPaths(t *testing.T) {
	actual := []configEntry{}
	for i := 0; i < 100; i++ {
		actual = append(actual, configEntry{Path: fmt.Sprintf("%03d-%s", i, strings.Repeat("x", 5000)), Kind: "file", Content: strings.Repeat("a", 64)})
	}
	output := nativeNodeEntryDiff(nil, actual)
	if strings.Count(output, ": expected kind=") != 40 || !strings.Contains(output, "differences=100") || len(output) > 20<<10 {
		t.Fatal("entry difference output exceeded its bounds", len(output))
	}
}
