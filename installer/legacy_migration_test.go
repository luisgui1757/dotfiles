package installer

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

func legacyMigrationFixture(t *testing.T, windows bool) *LegacyMigration {
	t.Helper()
	root, err := resolveConfigPath(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	home := filepath.Join(root, "home pessoal ü")
	repo := filepath.Join(root, "trusted clone ü")
	if err := os.MkdirAll(home, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(repo, 0700); err != nil {
		t.Fatal(err)
	}
	target := Context{OS: "linux", Arch: "amd64"}
	folders := ConfigFolders{Home: home, Documents: filepath.Join(root, "redirected Documents ü")}
	if windows {
		target.OS = "windows"
	}
	m, err := NewLegacyMigration(repo, "trusted-source", filepath.Join(root, "state ü"), target, folders)
	if err != nil {
		t.Fatal(err)
	}
	return m
}
func approveLegacy(t *testing.T, m *LegacyMigration, request Request) Result {
	t.Helper()
	preview, err := m.Dispatch(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	request.ExpectedPlan = preview.Plan.ID
	result, err := m.Dispatch(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	return result
}
func legacyRequest(ids ...string) Request {
	return Request{Schema: 1, Mode: "apply", Selected: ids, Adopt: ids}
}
func legacyState(t *testing.T, m *LegacyMigration) State {
	t.Helper()
	state, err := LoadState(m.Controller.StatePath, m.Controller.Home)
	if err != nil {
		t.Fatal(err)
	}
	return state
}

func TestLegacyMigrationPreviewAndCancellationNeverCreateState(t *testing.T) {
	m := legacyMigrationFixture(t, false)
	path := filepath.Join(m.Controller.Home, ".zshrc")
	writeConfigFixture(t, path, "personal additions\n")
	request := legacyRequest("legacy.zshrc")
	if _, err := m.Dispatch(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	ui := &scriptedInteraction{t: t, answers: [][]string{{"legacy.zshrc"}}, accept: false}
	result, err := m.Run(context.Background(), ui)
	if err != nil || result.Status != "cancelled" {
		t.Fatal(result, err)
	}
	if _, err := os.Lstat(filepath.Dir(m.Controller.StatePath)); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("preview/cancel created migration state", err)
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "personal additions\n" {
		t.Fatal(string(data), err)
	}
}

func TestLegacyMigrationPreservesUnknownProfileAndDoesNotReplayOverNewBlocks(t *testing.T) {
	m := legacyMigrationFixture(t, false)
	path := filepath.Join(m.Controller.Home, ".zshrc")
	original := "export PERSONAL_VALUE='kept privately'\n"
	writeConfigFixture(t, path, original)
	withoutAdopt := Request{Schema: 1, Mode: "apply", Selected: []string{"legacy.zshrc"}}
	preview, err := m.Dispatch(context.Background(), withoutAdopt)
	if err != nil || preview.Plan.Operations[0].Action != "pending" {
		t.Fatal("unknown profile silently adopted", preview, err)
	}
	result := approveLegacy(t, m, legacyRequest("legacy.zshrc"))
	if result.Status != "ready" {
		t.Fatal(result)
	}
	receipt := legacyState(t, m).Receipts["legacy.zshrc"]
	if receipt.Ownership != "created" || len(receipt.After.Preserved) != 2 {
		t.Fatal(receipt)
	}
	for _, path := range receipt.After.Preserved {
		data, err := os.ReadFile(path)
		if err != nil || string(data) != original {
			t.Fatal(path, string(data), err)
		}
	}
	for _, path := range []string{m.Controller.StatePath, m.driver.preservationPath(receipt), m.driver.journalPath(receipt.OperationID)} {
		data, err := os.ReadFile(path)
		if err != nil || bytes.Contains(data, []byte("kept privately")) {
			t.Fatal("personal bytes copied to JSON", path, err)
		}
	}
	writeConfigFixture(t, path, "# >>> dotfiles:profile.zsh >>>\nnew setup\n# <<< dotfiles:profile.zsh <<<\n")
	check, err := m.Dispatch(context.Background(), Request{Schema: 1, Mode: "check"})
	if err != nil || check.Status != "ready" {
		t.Fatal(check, err)
	}
	if _, err := m.Dispatch(context.Background(), legacyRequest("legacy.zshrc")); err == nil {
		t.Fatal("completed migration allowed replacement again")
	}
	if _, err := m.Dispatch(context.Background(), Request{Schema: 1, Mode: "apply", Selected: []string{}}); err == nil {
		t.Fatal("completed migration allowed implicit rollback")
	}
	data, err := os.ReadFile(path)
	if err != nil || !bytes.Contains(data, []byte("new setup")) {
		t.Fatal(string(data), err)
	}
}

func TestLegacyMigrationPreservesLiveLinkAndReadableEdits(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX released profiles are source links")
	}
	m := legacyMigrationFixture(t, false)
	source := filepath.Join(filepath.Dir(m.Controller.Home), "old checkout ü", "home", "dot_zshrc")
	writeConfigFixture(t, source, "legacy shell plus personal edits\n")
	path := filepath.Join(m.Controller.Home, ".zshrc")
	if err := os.Symlink(source, path); err != nil {
		t.Fatal(err)
	}
	request := legacyRequest("legacy.zshrc")
	preview, err := m.Dispatch(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	writeConfigFixture(t, source, "changed through the source link\n")
	request.ExpectedPlan = preview.Plan.ID
	if _, err := m.Dispatch(context.Background(), request); err == nil {
		t.Fatal("referent edit did not invalidate approval")
	}
	request.ExpectedPlan = ""
	approveLegacy(t, m, request)
	receipt := legacyState(t, m).Receipts["legacy.zshrc"]
	if err := os.Remove(source); err != nil {
		t.Fatal(err)
	}
	readable := filepath.Join(m.driver.Directory, receipt.Recovery, "readable-profile")
	data, err := os.ReadFile(readable)
	if err != nil || string(data) != "changed through the source link\n" {
		t.Fatal(string(data), err)
	}
	var link string
	for _, saved := range receipt.After.Preserved {
		if target, err := os.Readlink(saved); err == nil {
			link = target
		}
	}
	if link != source {
		t.Fatal("original link was not retained", link)
	}
	check, err := m.Dispatch(context.Background(), Request{Schema: 1, Mode: "check"})
	if err != nil || check.Status != "ready" {
		t.Fatal(check, err)
	}
}

func TestLegacyMigrationRemovesOnlyExactBashHookBytes(t *testing.T) {
	m := legacyMigrationFixture(t, false)
	hook := m.driver.evidence.BashHooks[0].Content
	path := filepath.Join(m.Controller.Home, ".bashrc")
	original := "# personal before\n" + hook + "export AFTER=unchanged\n"
	writeConfigFixture(t, path, original)
	approveLegacy(t, m, legacyRequest("legacy.bash-hook"))
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "# personal before\nexport AFTER=unchanged\n" {
		t.Fatal(string(data), err)
	}
	receipt := legacyState(t, m).Receipts["legacy.bash-hook"]
	data, err = os.ReadFile(filepath.Join(m.driver.Directory, receipt.Recovery, "readable-profile"))
	if err != nil || string(data) != original {
		t.Fatal(string(data), err)
	}
}

func TestLegacyMigrationEditedBashMarkerNeverGrantsWholeFileAuthority(t *testing.T) {
	m := legacyMigrationFixture(t, false)
	path := filepath.Join(m.Controller.Home, ".bashrc")
	original := "# >>> dotfiles: exec zsh (interactive bash fallback) >>>\npersonal custom block\n"
	writeConfigFixture(t, path, original)
	preview, err := m.Dispatch(context.Background(), legacyRequest("legacy.bash-hook"))
	if err != nil || preview.Plan.Operations[0].Observed.ApplyBlocked == "" {
		t.Fatal(preview, err)
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != original {
		t.Fatal(string(data), err)
	}
}

func TestLegacyMigrationWindowsKnownDocumentsDoNotReplaceConventionalPath(t *testing.T) {
	m := legacyMigrationFixture(t, true)
	actual := filepath.Join(m.driver.Folders.Documents, "PowerShell", "Microsoft.PowerShell_profile.ps1")
	conventional := filepath.Join(m.driver.Folders.Home, "Documents", "PowerShell", "Microsoft.PowerShell_profile.ps1")
	writeConfigFixture(t, actual, "actual known folder personal bytes\r\n")
	writeConfigFixture(t, conventional, "released conventional bytes\r\n")
	result := approveLegacy(t, m, legacyRequest("legacy.powershell"))
	if result.Status != "needs-action" {
		t.Fatal("conventional legacy profile silently ignored", result)
	}
	data, err := os.ReadFile(conventional)
	if err != nil || string(data) != "released conventional bytes\r\n" {
		t.Fatal(string(data), err)
	}
	request := legacyRequest("legacy.conventional-powershell")
	request.Selected = append(request.Selected, "legacy.powershell")
	result = approveLegacy(t, m, request)
	if result.Status != "ready" {
		t.Fatal(result)
	}
}

func TestLegacyMigrationRecognizesReleasedPowerShellAfterCheckoutPull(t *testing.T) {
	m := legacyMigrationFixture(t, true)
	original, err := os.ReadFile(filepath.Join("..", "shells", "powershell_profile.ps1"))
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(m.driver.Folders.Documents, "PowerShell", "Microsoft.PowerShell_profile.ps1")
	writeConfigFixture(t, path, string(original))
	choices, err := m.choices(context.Background(), State{})
	if err != nil || len(choices) != 1 || !strings.HasPrefix(choices[0].Detail, "Exact released profile bytes recognized.") {
		t.Fatal("pulled passive profile lost release recognition", choices, err)
	}
	edited := append(bytes.Clone(original), []byte("\n# personal addition\n")...)
	writeConfigFixture(t, path, string(edited))
	choices, err = m.choices(context.Background(), State{})
	if err != nil || len(choices) != 1 || !strings.HasPrefix(choices[0].Detail, "Unknown or edited profile:") {
		t.Fatal("personal edits incorrectly recognized as released", choices, err)
	}
	result := approveLegacy(t, m, legacyRequest("legacy.powershell"))
	if result.Status != "ready" {
		t.Fatal(result)
	}
	receipt := legacyState(t, m).Receipts["legacy.powershell"]
	readable, err := os.ReadFile(filepath.Join(m.driver.Directory, receipt.Recovery, "readable-profile"))
	if err != nil || !bytes.Equal(readable, edited) {
		t.Fatal("migration did not preserve the actual edited legacy bytes", err)
	}
}

type interruptLegacyDriver struct{ *legacyMigrationDriver }

func (d interruptLegacyDriver) Apply(ctx context.Context, r Resource, op Operation, receipt Receipt) (Observation, error) {
	after, err := d.legacyMigrationDriver.Apply(ctx, r, op, receipt)
	return after, errors.Join(err, errors.New("test crash before engine completion"))
}

func TestLegacyMigrationInterruptedPublicationResumesOrRestores(t *testing.T) {
	for _, restore := range []bool{false, true} {
		t.Run(map[bool]string{false: "resume", true: "restore"}[restore], func(t *testing.T) {
			m := legacyMigrationFixture(t, false)
			path := filepath.Join(m.Controller.Home, ".zshrc")
			writeConfigFixture(t, path, "personal baseline\n")
			m.Controller.Driver = interruptLegacyDriver{m.driver}
			request := legacyRequest("legacy.zshrc")
			preview, err := m.Dispatch(context.Background(), request)
			if err != nil {
				t.Fatal(err)
			}
			request.ExpectedPlan = preview.Plan.ID
			if _, err := m.Dispatch(context.Background(), request); err == nil || !strings.Contains(err.Error(), "test crash") {
				t.Fatal(err)
			}
			m.Controller.Driver = m.driver
			state := legacyState(t, m)
			receipt := state.Receipts["legacy.zshrc"]
			resource, _ := m.Controller.Catalog.Resource("legacy.zshrc")
			j, err := m.driver.readJournal(resource, receipt)
			if err != nil {
				t.Fatal(err)
			}
			if err := moveConfigExclusive(path, filepath.Join(j.Entries[0].Workspace, "next")); err != nil {
				t.Fatal(err)
			}
			j.Complete, j.Entries[0].Phase = false, "moving"
			if err := saveDocument(m.driver.journalPath(j.Operation), j); err != nil {
				t.Fatal(err)
			}
			if restore {
				request = Request{Schema: 1, Mode: "restore"}
			} else {
				request = Request{Schema: 1, Mode: "apply", Retry: true}
			}
			result := approveLegacy(t, m, request)
			state = legacyState(t, m)
			if !restore && state.Transaction != nil {
				// Native publication recovery and the remaining engine plan have separate approvals.
				request.ExpectedPlan = ""
				result = approveLegacy(t, m, request)
				state = legacyState(t, m)
			}
			if state.Transaction != nil {
				t.Fatal("transaction not resolved", state)
			}
			if restore {
				data, err := os.ReadFile(path)
				if err != nil || string(data) != "personal baseline\n" {
					t.Fatal(string(data), err)
				}
				if result.Status != "needs-action" {
					t.Fatal("restoration falsely claimed migrated", result)
				}
				result = approveLegacy(t, m, legacyRequest("legacy.zshrc"))
				if result.Status != "ready" {
					t.Fatal("restored migration could not be reviewed again", result)
				}
			} else if result.Status != "ready" {
				t.Fatal(result)
			}
		})
	}
}

func TestLegacyMigrationRejectsChangedPreservationAndUnfinishedReleasedUpgrade(t *testing.T) {
	for _, fixture := range []string{"readable", "journal"} {
		t.Run(fixture, func(t *testing.T) {
			m := legacyMigrationFixture(t, false)
			writeConfigFixture(t, filepath.Join(m.Controller.Home, ".zshrc"), "original\n")
			approveLegacy(t, m, legacyRequest("legacy.zshrc"))
			receipt := legacyState(t, m).Receipts["legacy.zshrc"]
			if fixture == "readable" {
				writeConfigFixture(t, filepath.Join(m.driver.Directory, receipt.Recovery, "readable-profile"), "edited recovery\n")
			} else {
				root := filepath.Join(filepath.Dir(m.driver.Directory), "migrations", "v0.1.0-to-v0.4.4.fixture")
				for name, value := range map[string]string{"stage": "applying\n", "old-checkout": "/old\n", "new-checkout": "/new\n"} {
					writeConfigFixture(t, filepath.Join(root, name), value)
				}
			}
			if _, err := m.Dispatch(context.Background(), Request{Schema: 1, Mode: "check"}); err == nil {
				t.Fatal("changed/unfinished evidence trusted")
			}
		})
	}
}

func TestLegacyEvidenceIncludesAllPublishedProfileVersions(t *testing.T) {
	evidence, err := releasedProfileEvidence()
	if err != nil {
		t.Fatal(err)
	}
	tags := []string{}
	hashes := []string{}
	for _, item := range evidence.Profiles {
		tags = append(tags, item.Tag)
		if item.Kind == "zshrc" {
			hashes = append(hashes, item.SHA256)
		}
	}
	if len(sortedUnique(tags)) != 8 || !slices.Contains(hashes, "a42ac956beb9bc2979f48fc0df386494924cc4fee64a119ff679d7f47c7d465f") {
		t.Fatal("distinct intermediate release evidence missing", tags, hashes)
	}
}

func TestLegacyMigrationNeverTakesOverNewScopedProfileBlocks(t *testing.T) {
	m := legacyMigrationFixture(t, false)
	path := filepath.Join(m.Controller.Home, ".zshrc")
	original := "# >>> dotfiles:integration.shells >>>\nnew managed block\n# <<< dotfiles:integration.shells <<<\n"
	writeConfigFixture(t, path, original)
	preview, err := m.Dispatch(context.Background(), legacyRequest("legacy.zshrc"))
	if err != nil || !strings.Contains(preview.Plan.Operations[0].Observed.ApplyBlocked, "new installer blocks") {
		t.Fatal(preview, err)
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != original {
		t.Fatal(string(data), err)
	}
}

func TestLegacyMigrationResolvesConventionalDocumentsAliasBeforeMapping(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("native Windows junction coverage belongs to known-folder discovery; this fixture uses POSIX symlinks")
	}
	m := legacyMigrationFixture(t, true)
	folders := m.driver.Folders
	if err := os.MkdirAll(folders.Documents, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(folders.Documents, filepath.Join(folders.Home, "Documents")); err != nil {
		t.Fatal(err)
	}
	m, err := NewLegacyMigration(filepath.Dir(folders.Home), "trusted-source", filepath.Dir(m.driver.Directory), m.Controller.Context, folders)
	if err != nil {
		t.Fatal(err)
	}
	if _, duplicate := m.Controller.Catalog.Resource("legacy.conventional-powershell"); duplicate {
		t.Fatal("same physical profile received two migration owners")
	}
}
