package installer

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCorruptHistoricalJournalDoesNotBlockAnotherInstallation(t *testing.T) {
	for _, provider := range []string{"configuration", "profiles"} {
		t.Run(provider, func(t *testing.T) {
			var c Controller
			if provider == "profiles" {
				c, _, _ = profileController(t)
			} else {
				c, _, _ = nativeConfigFixture(t, "copy")
			}
			dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"test"}})
			dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{}})
			directory := filepath.Join(filepath.Dir(c.StatePath), provider, "operations")
			files, err := os.ReadDir(directory)
			if err != nil || len(files) < 2 {
				t.Fatal("fixture did not retain completed history", files, err)
			}
			path := filepath.Join(directory, files[0].Name())
			writeConfigFixture(t, path, "{damaged historical metadata")
			result := dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"test"}})
			if !strings.Contains(result.Message, path) {
				t.Fatal("damaged history was not disclosed", result)
			}
			data, err := os.ReadFile(path)
			if err != nil || string(data) != "{damaged historical metadata" {
				t.Fatal("damaged history was destroyed", string(data), err)
			}
			dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{}})
		})
	}
}

func TestProfilePublishedJournalsDoNotRetainPersonalContent(t *testing.T) {
	c, d, path := profileController(t)
	writeConfigFixture(t, path, "# unique personal content outside the owned integration")
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"test"}})
	d.Targets["profile.test"][0].Script = "# updated integration"
	dispatchApproved(t, c, Request{Schema: 1, Mode: "update"})
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{}})
	files, err := os.ReadDir(filepath.Join(d.Directory, "profiles", "operations"))
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		data, err := readDocument(filepath.Join(d.Directory, "profiles", "operations", file.Name()))
		if err != nil {
			t.Fatal(err)
		}
		var j profileJournal
		if err := Decode(data, &j); err != nil {
			t.Fatal(err)
		}
		for _, e := range j.Entries {
			if len(e.Content) != 0 {
				t.Fatal("published operation retains the user's complete profile")
			}
		}
	}
}

func TestArchiveDamagedHistoryWithUnknownWorkspaceDoesNotBlockRemoval(t *testing.T) {
	c, d, _ := archiveController(t)
	dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{"first"}})
	old, err := d.current("tool.shared")
	if err != nil {
		t.Fatal(err)
	}
	pin, client, _ := archivePinFor(t, archiveTar(t, []archiveFixtureEntry{{Name: "tool", Text: "version two", Mode: 0755}}), "tar.gz")
	pin.Version = "2.0"
	d.Pins["tool.shared"], d.Client = pin, client
	dispatchApproved(t, c, Request{Schema: 1, Mode: "update"})
	path := d.intentPath("tool.shared", old.Operation)
	writeConfigFixture(t, path, "{damaged historical metadata")
	workspace := filepath.Join(d.resourceDirectory("tool.shared"), ".previous-"+old.Operation)
	writeConfigFixture(t, filepath.Join(workspace, "personal-file"), "keep these bytes")
	result := dispatchApproved(t, c, Request{Schema: 1, Mode: "apply", Selected: []string{}})
	if !strings.Contains(result.Message, workspace) || !strings.Contains(result.Message, path) {
		t.Fatal("unproved workspace and its metadata were not disclosed", result)
	}
	data, err := os.ReadFile(filepath.Join(workspace, "personal-file"))
	if err != nil || string(data) != "keep these bytes" {
		t.Fatal("unproved historical workspace was removed", string(data), err)
	}
}

func TestInvalidRestoreEvidenceDoesNotHideRecoveryMenu(t *testing.T) {
	c, d, _ := profileController(t)
	c.Driver = profileUnrecordedPublication{d}
	request := Request{Schema: 1, Mode: "apply", Selected: []string{"test"}}
	preview, err := c.Preview(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	request.ExpectedPlan = preview.ID
	if _, err := c.Dispatch(context.Background(), request); err == nil {
		t.Fatal("fixture did not interrupt after publication")
	}
	c.Driver = d
	state, err := LoadState(c.StatePath, c.Home)
	if err != nil {
		t.Fatal(err)
	}
	path := d.journalPath(state.Receipts["profile.test"].OperationID)
	writeConfigFixture(t, path, "{damaged active metadata")
	ui := &scriptedInteraction{t: t, answers: [][]string{{"exit"}}}
	if _, err := c.Run(context.Background(), ui); err != nil {
		t.Fatal("invalid restoration evidence hid the other recovery choices", err)
	}
	if len(ui.menus) != 1 || len(ui.reports) == 0 || !strings.Contains(ui.reports[0].Message, "restoration unavailable") {
		t.Fatal("restoration error was not disclosed beside the recovery menu", ui)
	}
	request = Request{Schema: 1, Mode: "abandon"}
	preview, err = c.Preview(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	request.ExpectedPlan = preview.ID
	if _, err := c.Dispatch(context.Background(), request); err == nil {
		t.Fatal("abandon discarded an active operation with damaged evidence")
	}
	state, err = LoadState(c.StatePath, c.Home)
	if err != nil || state.Transaction == nil {
		t.Fatal("failed abandonment lost recovery intent", err)
	}
}
