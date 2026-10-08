package installer

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func interruptedScopedRestore(t *testing.T, cut string) (Controller, *ProfileDriver, profileJournal, []byte) {
	t.Helper()
	c, d, j, original := interruptedProfileController(t, "update", "published")
	path := j.Entries[0].Path
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	edit := []byte("\n# personal settings added after publication\n")
	writeConfigFixture(t, path, string(append(data, edit...)))
	state, err := LoadState(c.StatePath, c.Home)
	if err != nil {
		t.Fatal(err)
	}
	receipt := state.Receipts["profile.test"]
	observed, err := d.ObserveRestore(context.Background(), "profile.test", receipt)
	if err != nil {
		t.Fatal(err)
	}
	if err := d.RestoreResource(context.Background(), "profile.test", observed, receipt); err != nil {
		t.Fatal(err)
	}
	j, err = d.readRecoveryJournal("profile.test", receipt)
	if err != nil || len(j.Restoration) != 1 {
		t.Fatal("fixture did not publish a scoped inverse", err)
	}
	j.Complete, j.Sealed, j.Cancelled = false, false, false
	e := &j.Restoration[0]
	e.Published = false
	if cut != "published" {
		if err := moveConfigExclusive(path, filepath.Join(e.Workspace, "next")); err != nil {
			t.Fatal(err)
		}
		e.Moved = false
	}
	if cut == "staged" || cut == "intent" {
		if err := moveConfigExclusive(filepath.Join(e.Workspace, "previous"), path); err != nil {
			t.Fatal(err)
		}
	}
	if cut == "intent" {
		e.Content, err = os.ReadFile(filepath.Join(e.Workspace, "next"))
		if err != nil {
			t.Fatal(err)
		}
		e.Staged = false
	}
	if err := saveDocument(d.journalPath(j.Operation), j); err != nil {
		t.Fatal(err)
	}
	return c, d, j, append(bytes.Clone(original), edit...)
}

func TestScopedRestoreAcceptsFreshEditsBetweenAttempts(t *testing.T) {
	for _, cut := range []string{"intent", "staged", "moved", "published"} {
		t.Run(cut, func(t *testing.T) {
			c, _, j, expected := interruptedScopedRestore(t, cut)
			path := j.Entries[0].Path
			edit := []byte("\n# more personal settings between restoration attempts\n")
			if cut == "moved" {
				writeConfigFixture(t, path, string(edit))
			} else {
				data, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				writeConfigFixture(t, path, string(append(data, edit...)))
				expected = append(expected, edit...)
			}
			result := dispatchApproved(t, c, Request{Schema: 1, Mode: "restore"})
			actual, err := os.ReadFile(path)
			if err != nil || !bytes.Equal(actual, expected) || result.Status != "ready" {
				t.Fatalf("restoration did not preserve the active personal settings: %q, %s, %v", actual, result.Status, err)
			}
			if cut == "moved" {
				preserved := filepath.Join(j.Restoration[0].Workspace, "displaced")
				data, err := os.ReadFile(preserved)
				if err != nil || !bytes.Equal(data, edit) || !strings.Contains(result.Message, preserved) {
					t.Fatal("recreated profile was lost or not disclosed", err, result.Message)
				}
			}
		})
	}
}

func TestProfileRestoreKeepsUnparseablePersonalEditsWithoutStrandingTransaction(t *testing.T) {
	for _, damage := range []string{"utf8", "nul", "marker", "duplicate-marker", "oversized"} {
		t.Run(damage, func(t *testing.T) {
			c, _, j, _ := interruptedProfileController(t, "update", "published")
			path := j.Entries[0].Path
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			switch damage {
			case "utf8":
				data = append(data, 0xff)
			case "nul":
				data = append(data, 0)
			case "marker":
				data = bytes.Replace(data, []byte("# >>> dotfiles:"), []byte("# >>> changed:"), 1)
			case "duplicate-marker":
				data = append(data, j.Entries[0].Block...)
			case "oversized":
				data = append(data, bytes.Repeat([]byte("#"), maxProfileBytes)...)
			}
			writeConfigFixture(t, path, string(data))
			request := Request{Schema: 1, Mode: "restore"}
			preview, err := c.Preview(context.Background(), request)
			if err != nil {
				t.Fatal(err)
			}
			request.ExpectedPlan = preview.ID
			result, err := c.Dispatch(context.Background(), request)
			if err != nil {
				t.Fatal("could not preserve the edited profile and finish recovery", err)
			}
			actual, err := os.ReadFile(path)
			if err != nil || !bytes.Equal(actual, data) || result.Status != "needs-action" || !strings.Contains(result.Message, path) {
				t.Fatal("unparseable personal edits were changed or not disclosed", err, result)
			}
			state, err := LoadState(c.StatePath, c.Home)
			if err != nil || state.Transaction != nil {
				t.Fatal("preserved personal edits stranded the whole installer", err)
			}
		})
	}
}

func TestProfileRestoreKeepsEditsToAnAlreadyReturnedOriginal(t *testing.T) {
	for _, cut := range []string{"moved", "published"} {
		t.Run(cut, func(t *testing.T) {
			c, d, j, original := interruptedProfileController(t, "update", cut)
			if err := d.restoreProfileEntry(context.Background(), &j, j.Entries[0]); err != nil {
				t.Fatal(err)
			}
			j.Restoring = true
			if err := saveDocument(d.journalPath(j.Operation), j); err != nil {
				t.Fatal(err)
			}
			want := append(bytes.Clone(original), []byte("\n# edited after the original returned\n")...)
			writeConfigFixture(t, j.Entries[0].Path, string(want))
			result := dispatchApproved(t, c, Request{Schema: 1, Mode: "restore"})
			actual, err := os.ReadFile(j.Entries[0].Path)
			if err != nil || !bytes.Equal(actual, want) || result.Status != "ready" {
				t.Fatal("retry displaced or refused edits to the returned original", err, result)
			}
		})
	}
}

func TestProfileRestoreReturnsOriginalWhenAnUnpublishedTargetIsRecreated(t *testing.T) {
	for _, action := range []string{"install", "update", "remove"} {
		t.Run(action, func(t *testing.T) {
			c, _, j, original := interruptedProfileController(t, action, "moved")
			e := j.Entries[0]
			recreated := "# independently created while the profile was absent\n"
			writeConfigFixture(t, e.Path, recreated)
			result := dispatchApproved(t, c, Request{Schema: 1, Mode: "restore"})
			actual, err := os.ReadFile(e.Path)
			if err != nil || !bytes.Equal(actual, original) || result.Status != "ready" {
				t.Fatal("restoration left the original profile inactive", string(actual), err, result.Status)
			}
			preserved := filepath.Join(e.Workspace, "displaced")
			data, err := os.ReadFile(preserved)
			if err != nil || string(data) != recreated || !strings.Contains(result.Message, preserved) {
				t.Fatal("recreated profile was lost or not disclosed", err, result.Message)
			}
		})
	}
}

func TestProfileRestoreInspectsFilesBeforeChangingDirection(t *testing.T) {
	c, d, j, _ := interruptedProfileController(t, "update", "moved")
	workspace := j.Entries[0].Workspace
	if err := moveConfigExclusive(workspace, workspace+"-saved"); err != nil {
		t.Fatal(err)
	}
	if err := createDirectoryLink(workspace+"-saved", workspace); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(d.journalPath(j.Operation))
	if err != nil {
		t.Fatal(err)
	}
	request := Request{Schema: 1, Mode: "restore"}
	preview, err := c.Preview(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	request.ExpectedPlan = preview.ID
	if _, err := c.Dispatch(context.Background(), request); err == nil {
		t.Fatal("accepted an uninspectable recovery path")
	}
	after, err := os.ReadFile(d.journalPath(j.Operation))
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("failed inspection changed the saved recovery direction", err)
	}
}

func TestProfileRestoreNamesTheActiveFileWhenItsSavedOriginalIsMissing(t *testing.T) {
	c, _, j, _ := interruptedProfileController(t, "update", "published")
	e := j.Entries[0]
	data, err := os.ReadFile(e.Path)
	if err != nil {
		t.Fatal(err)
	}
	data = append(data, []byte("\n# personal edit after publication\n")...)
	writeConfigFixture(t, e.Path, string(data))
	if err := os.Remove(filepath.Join(e.Workspace, "previous")); err != nil {
		t.Fatal(err)
	}
	request := Request{Schema: 1, Mode: "restore"}
	preview, err := c.Preview(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	request.ExpectedPlan = preview.ID
	result, err := c.Dispatch(context.Background(), request)
	if err != nil || result.Status != "needs-action" || !strings.Contains(result.Message, e.Path) {
		t.Fatal("missing original left no actionable recovery location", result, err)
	}
	actual, err := os.ReadFile(e.Path)
	if err != nil || !bytes.Equal(data, actual) {
		t.Fatal("missing-original restoration changed the active profile", err)
	}
	state, err := LoadState(c.StatePath, c.Home)
	if err != nil || state.Transaction != nil {
		t.Fatal("missing original stranded the transaction", err)
	}
}

func TestScopedRestorePreservesEditsWhenItsStagedCopyDisappears(t *testing.T) {
	for _, cut := range []string{"moved", "published"} {
		t.Run(cut, func(t *testing.T) {
			c, _, j, expected := interruptedScopedRestore(t, cut)
			e := j.Restoration[0]
			if cut == "moved" {
				if err := os.Remove(filepath.Join(e.Workspace, "next")); err != nil {
					t.Fatal(err)
				}
			} else {
				edit := []byte("\n# edit after inverse publication\n")
				data, err := os.ReadFile(e.Path)
				if err != nil {
					t.Fatal(err)
				}
				writeConfigFixture(t, e.Path, string(append(data, edit...)))
				expected = append(expected, edit...)
			}
			result := dispatchApproved(t, c, Request{Schema: 1, Mode: "restore"})
			actual, err := os.ReadFile(e.Path)
			if err != nil || !bytes.Equal(actual, expected) || result.Status != "ready" {
				t.Fatal("missing inverse stage lost personal settings", string(actual), result, err)
			}
		})
	}
}
