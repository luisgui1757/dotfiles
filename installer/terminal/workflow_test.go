package terminal

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	engine "github.com/luisgui1757/dotfiles/installer"
	"golang.org/x/term"
)

func TestReviewContainsDependencyOwnershipPrivilegeAndCompleteLongReason(t *testing.T) {
	catalog := &engine.Catalog{Schema: 1, Resources: []engine.Resource{{ID: "compiler", Name: "Compiler", Action: "compiler"}}}
	if err := catalog.Validate(); err != nil {
		t.Fatal(err)
	}
	reason := strings.Repeat("compilers are needed for parser builds ", 10)
	kept, deleted := filepath.Join(t.TempDir(), "saved edits"), filepath.Join(t.TempDir(), "deleted copy")
	if err := os.WriteFile(kept, []byte("edits"), 0600); err != nil {
		t.Fatal(err)
	}
	plan := engine.Plan{Mode: "apply", ID: strings.Repeat("a", 64), Source: "source", Target: "/fixture", Operations: []engine.Operation{
		{Resource: "compiler", Action: "install", Reason: reason, RequiredBy: []string{"neovim"}, Ownership: "unrecorded", Privileged: true,
			Observed: engine.Observation{UnverifiedApplications: []string{"external-app"}, Notes: []string{"run the engine setup"}, Preserved: []string{kept, deleted}}},
	}}
	lines := planText(catalog, plan, "")
	for _, expected := range []string{"Why: " + reason, "Required by: neovim", "Ownership: unrecorded", "Requires administrator privileges.", "External application not exercised: external-app",
		"Note: run the engine setup", "Preserved files for inspection: " + kept} {
		if !slices.Contains(lines, expected) {
			t.Fatal("approval omitted consequential detail", expected)
		}
	}
	if slices.Contains(lines, "Preserved files for inspection: "+deleted) {
		t.Fatal("approval listed a preserved copy that no longer exists")
	}
	report := reportText(engine.Result{Status: "ready", Message: "Note: run the engine setup\nExternal application not exercised: external-app"})
	if !strings.Contains(report, "Notes (informational, not failures):\n  - Note: run the engine setup\n") || strings.Contains(report, "need your action") {
		t.Fatal("a note was reported as an action", report)
	}
	if strings.Join(wrapLines([]string{reason}, 31), "") != reason {
		t.Fatal("narrow preview lost approval text")
	}
}

func TestReviewDisclosesOutsideRegistrationPreventingRemoval(t *testing.T) {
	catalog := &engine.Catalog{Schema: 1, Resources: []engine.Resource{{ID: "runtime", Name: "Shared runtime"}}}
	consumer := "Older VC++ x64 runtime registration in Registry32: 14.44.35211.0"
	for _, action := range []string{"keep", "retain", "install", "pending"} {
		plan := engine.Plan{Operations: []engine.Operation{{Resource: "runtime", Action: action, Observed: engine.Observation{Consumers: []string{consumer}}}}}
		lines := planText(catalog, plan, "")
		if !slices.Contains(lines, "Outside consumer: "+consumer) {
			t.Fatal("review hides the outside registration protecting the runtime", action, lines)
		}
	}
}

func TestRetirementReviewNamesRemovalsAndShowsCompleteScope(t *testing.T) {
	catalog := &engine.Catalog{Schema: 1, Resources: []engine.Resource{
		{ID: "retire.delta.fzf", Name: "Remove earlier fzf", Action: "retire"},
		{ID: "retire.homebrew", Name: "Convert Nix-managed Homebrew", Action: "retire"},
		{ID: "retire.replacements", Name: "Verified replacements", Action: "retire"},
	}}
	plan := engine.Plan{Operations: []engine.Operation{
		{Resource: "retire.delta.fzf", Action: "install"},
		{Resource: "retire.homebrew", Action: "install", Privileged: true},
		{Resource: "retire.replacements", Action: "keep"},
	}}
	details := strings.Repeat("preserve this exact path and native package identity ", 30) + "\nRetained: personal configuration"
	lines := planText(catalog, plan, details)
	for _, want := range append([]string{"Add: 0  Change: 1", "Remove: 1  Keep: 1", "Needs action: 0  Admin: 1", "REMOVE: Remove earlier fzf (retire.delta.fzf)", "CHANGE: Convert Nix-managed Homebrew (retire.homebrew)"}, strings.Split(details, "\n")...) {
		if !slices.Contains(lines, want) {
			t.Errorf("retirement review omitted or misrepresented %q", want)
		}
	}
	if strings.Contains(strings.Join(lines, "\n"), "INSTALL:") || plan.Operations[0].Action != "install" {
		t.Fatal("retirement display changed engine actions or called removal an installation")
	}
	if !strings.Contains(strings.Join(wrapLines(lines, 31), ""), strings.ReplaceAll(details, "\n", "")) {
		t.Fatal("narrow review lost exact retirement scope")
	}
}

// The native PTY/ConPTY journey publishes a real private file. This is proof of
// the connected controller/terminal boundary, not native package installation.
type workflowFiles struct{ directory string }

func (d workflowFiles) Observe(_ context.Context, r engine.Resource, receipt engine.Receipt) (engine.Observation, error) {
	path := filepath.Join(d.directory, r.ID)
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return engine.Observation{}, nil
	}
	if err != nil {
		return engine.Observation{}, err
	}
	var stored struct{ Operation string }
	if err := json.Unmarshal(data, &stored); err != nil {
		return engine.Observation{}, err
	}
	o := engine.Observation{Present: true, Healthy: true, Provider: "private-files", Identity: path, Fingerprint: fmt.Sprintf("%x", sha256.Sum256(data))}
	if stored.Operation == receipt.OperationID {
		o.CompletedOperation = stored.Operation
	}
	return o, nil
}
func (d workflowFiles) Apply(ctx context.Context, r engine.Resource, _ engine.Operation, receipt engine.Receipt) (engine.Observation, error) {
	data, err := json.Marshal(struct{ Operation string }{receipt.OperationID})
	if err != nil {
		return engine.Observation{}, err
	}
	if err := os.WriteFile(filepath.Join(d.directory, r.ID), data, 0600); err != nil {
		return engine.Observation{}, err
	}
	return d.Observe(ctx, r, receipt)
}
func (d workflowFiles) Remove(ctx context.Context, r engine.Resource, receipt engine.Receipt) (engine.Observation, error) {
	if err := os.Remove(filepath.Join(d.directory, r.ID)); err != nil {
		return engine.Observation{}, err
	}
	return d.Observe(ctx, r, receipt)
}

func testWorkflowChild(t *testing.T, mode string, before *term.State) {
	home := t.TempDir()
	catalog := &engine.Catalog{Schema: 1, Resources: []engine.Resource{
		{ID: "agent", Name: "Coding agent", Category: "AI coding", Capability: true, Requires: []string{"runtime"}},
		{ID: "runtime", Name: "Shared runtime", Action: "runtime", Shared: true},
	}}
	c := engine.Controller{Catalog: catalog, Context: engine.Context{OS: runtime.GOOS, Arch: runtime.GOARCH}, Home: home,
		StatePath: filepath.Join(home, "ledger", "state.json"), Source: "native-ui-fixture", Driver: workflowFiles{home}}
	if mode == "workflow-shared" {
		// Start from an installed tool whose runtime only it needs.
		request := engine.Request{Schema: 1, Mode: "apply", Selected: []string{"agent"}}
		preview, err := c.Dispatch(context.Background(), request)
		if err != nil {
			t.Fatal(err)
		}
		request.ExpectedPlan = preview.Plan.ID
		if installed, err := c.Dispatch(context.Background(), request); err != nil || installed.Status != "ready" {
			t.Fatal(installed, err)
		}
	}
	w := &Workflow{Input: os.Stdin, Output: os.Stdout}
	result, err := c.Run(context.Background(), w)
	if err := errors.Join(err, w.Close()); err != nil {
		t.Fatal(err)
	}
	expected := "cancelled"
	if mode == "workflow-shared" {
		expected = "ready"
		state, err := engine.LoadState(c.StatePath, home)
		if _, statErr := os.Stat(filepath.Join(home, "runtime")); err != nil || len(state.Selected) != 0 || !errors.Is(statErr, os.ErrNotExist) {
			t.Fatal("approved shared removal did not run", state, err, statErr)
		}
	} else if mode == "workflow-apply" {
		expected = "ready"
		state, err := engine.LoadState(c.StatePath, home)
		if err != nil || len(state.Selected) != 1 || state.Selected[0] != "agent" || state.Receipts["runtime"].Ownership != "created" {
			t.Fatal("connected installation lost selection or dependency ownership", state, err)
		}
	} else if _, err := os.Stat(filepath.Dir(c.StatePath)); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("cancelled workflow created state", err)
	}
	if result.Status != expected {
		t.Fatal(result)
	}
	fmt.Println("Ready for ordinary input")
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil || strings.TrimSpace(line) != "ordinary input" {
		t.Fatal("workflow stole subsequent prompt input", err)
	}
	after, err := term.GetState(int(os.Stdin.Fd()))
	if err != nil || !reflect.DeepEqual(before, after) {
		t.Fatal("workflow failed to restore terminal state", err)
	}
	fmt.Printf("workflow=%s\nrestored=true\n", result.Status)
}

// Back returns to the main menu, whose last entry exits; Ctrl-C quits.
func TestNativeControllerWorkflowAppliesOnlyAfterExplicitConfirmation(t *testing.T) {
	for _, mode := range []string{"apply", "back", "declined", "tools-back", "type-ahead", "double-escape", "quit"} {
		t.Run(mode, func(t *testing.T) {
			s := startNativeSession(t, "workflow-"+mode)
			s.waitFor(t, "Choose the tools you want installed")
			s.waitFor(t, "> [ ] AI coding")
			switch mode {
			case "quit":
				s.send(t, "\x03")
			case "tools-back":
				s.send(t, "\x1b")
				s.waitFor(t, "What would you like to do?")
				s.send(t, "\r")
				s.waitForCount(t, "Choose the tools you want installed", 2)
				s.send(t, "\x03")
			case "double-escape":
				// Two quick Escapes close the details and then leave the
				// confirmation; neither press may be lost.
				s.send(t, " \r")
				s.waitFor(t, "Apply these changes?")
				s.send(t, "d")
				s.waitFor(t, "Required by: agent")
				s.send(t, "\x1b\x1b")
				s.waitFor(t, "What would you like to do?")
				s.send(t, "k\r")
			case "type-ahead":
				// The extra Enter arrives before the confirmation exists.
				s.send(t, " \r\r")
				s.waitFor(t, "Apply these changes?")
				s.send(t, "\x1b")
				s.waitFor(t, "What would you like to do?")
				s.send(t, "k\r")
			default:
				s.send(t, " \r")
				s.waitFor(t, "Apply these changes?")
				s.waitFor(t, "Add: 1  Change: 0")
				s.waitFor(t, "> Apply changes")
				s.send(t, "d")
				s.waitFor(t, "Required by: agent")
				s.send(t, "\x1b")
				s.waitForCount(t, "Apply these changes?", 2)
				switch mode {
				case "apply":
					s.send(t, "\r")
					s.waitFor(t, "==> [1/1] Installing Shared runtime")
					s.waitFor(t, "==> Done: everything selected is ready.")
				case "back":
					s.send(t, "\x1b")
				case "declined":
					s.send(t, "j\r")
				}
				if mode != "apply" {
					s.waitFor(t, "What would you like to do?")
					s.send(t, "k\r")
				}
			}
			status := "cancelled"
			if mode == "apply" {
				status = "ready"
			}
			s.waitFor(t, "workflow="+status)
			s.waitFor(t, "restored=true")
		})
	}
}

// One review shows the shared dependency as a checkbox; toggling it revises
// the counts in place before the user applies.
func TestNativeWorkflowRevisesSharedRemovalInOneReview(t *testing.T) {
	s := startNativeSession(t, "workflow-shared")
	// The hosted public journey navigates with these same screen helpers.
	s.chooseMenu(t, 0, "Choose tools")
	s.waitFor(t, "All 1 installed tools are healthy and current.")
	s.send(t, " ")
	s.waitFor(t, "Add 0, remove 1")
	s.send(t, "\r")
	confirmation := func(screen string) bool {
		return strings.Contains(screen, "Apply these changes?") && !strings.Contains(screen, "Updating the plan...")
	}
	screen, seen := s.nextScreen(t, 0, confirmation, 15*time.Second)
	if !strings.Contains(screen, "Also remove unused shared dependencies:") || !strings.Contains(screen, "> Apply changes") || !strings.Contains(screen, "Remove: 0  Keep: 1") {
		t.Fatalf("confirmation lacks the shared decision: %q", screen)
	}
	s.send(t, "k ")
	s.waitFor(t, "Updating the plan...")
	screen, seen = s.nextScreen(t, seen, func(screen string) bool {
		return confirmation(screen) && strings.Contains(screen, "> [x] Shared runtime")
	}, 15*time.Second)
	if !strings.Contains(screen, "Remove: 1  Keep: 0") {
		t.Fatalf("revision did not update the counts: %q", screen)
	}
	s.send(t, "j")
	s.nextScreen(t, seen, func(screen string) bool { return confirmation(screen) && strings.Contains(screen, "> Apply changes") }, 15*time.Second)
	s.send(t, "\r")
	s.waitFor(t, "==> [1/1] Removing Shared runtime")
	s.waitFor(t, "Full log: ")
	s.waitFor(t, "workflow=ready")
	s.waitFor(t, "restored=true")
}

func TestToolScreenSummarizesChangesAndLabelsStatus(t *testing.T) {
	tools := ToolScreen{Installed: []string{"git", "zsh"}, Status: func() (string, map[string]string) {
		return "1 of 2 installed tools need attention.", map[string]string{"zsh": "needs repair"}
	}}
	options := []Option{{ID: "git", Label: "Git"}, {ID: "zsh", Label: "zsh"}, {ID: "fzf", Label: "fzf"}}
	summary, labels := tools.summary(options, map[string]bool{"zsh": true, "fzf": true})
	if summary != "Add 1, remove 1 | 1 of 2 installed tools need attention." {
		t.Fatal(summary)
	}
	screen := render("Choose tools", selectionRows(options, true), map[string]bool{"zsh": true}, 0, true, 80, 20, summary, labels)
	if !strings.Contains(screen, "[x] zsh (needs repair)") || !strings.Contains(screen, summary) {
		t.Fatal("status was not shown", screen)
	}
}

func TestConfirmRowsBoundHighlightsAndEndWithActions(t *testing.T) {
	resources := []engine.Resource{}
	operations := []engine.Operation{}
	for i := range 12 {
		id := fmt.Sprintf("tool%d", i)
		resources = append(resources, engine.Resource{ID: id, Name: id, Action: "native"})
		operations = append(operations, engine.Operation{Resource: id, Action: "remove"})
	}
	catalog := &engine.Catalog{Schema: 1, Resources: resources}
	proposal := engine.Proposal{Plan: engine.Plan{Operations: operations}, Choices: []engine.ReviewChoice{{Group: engine.ReviewShared, ID: "tool0", Label: "tool0", Checked: true}}}
	rows := confirmRows(catalog, proposal)
	texts := []string{}
	for _, row := range rows {
		texts = append(texts, row.text)
	}
	if len(rows) != maxHighlights+6 || texts[maxHighlights-1] != "+5 more | d: details" || texts[maxHighlights+1] != engine.ReviewShared+":" ||
		texts[maxHighlights+2] != "[x] tool0" || rows[len(rows)-2].action != "apply" || rows[len(rows)-1].action != "back" {
		t.Fatal("confirmation rows", texts)
	}
}

func TestReviewNeutralizesTerminalControlsWithoutDroppingVisibleText(t *testing.T) {
	if got := strings.Join(wrapLines([]string{"ab\x1b[2Jcd\x00ef\u0085g"}, 4), ""); got != "ab[2Jcdefg" {
		t.Fatal(got)
	}
}

func TestConfirmationNamesWhatNeedsAction(t *testing.T) {
	catalog := &engine.Catalog{Schema: 1, Resources: []engine.Resource{{ID: "rg", Name: "ripgrep", Action: "archive"}}}
	plan := engine.Plan{Operations: []engine.Operation{{Resource: "rg", Action: "pending", Reason: "pre-existing resource requires explicit adoption"}}}
	if !slices.Contains(planHighlights(catalog, plan), "NEEDS ACTION: ripgrep - pre-existing resource requires explicit adoption") {
		t.Fatal(planHighlights(catalog, plan))
	}
}

func TestReviewLeadsWithRemovalAndPrivilegeSummary(t *testing.T) {
	catalog := &engine.Catalog{Schema: 1, Resources: []engine.Resource{
		{ID: "add", Name: "New tool", Action: "add"},
		{ID: "remove", Name: "Removed tool", Action: "remove"},
	}}
	if err := catalog.Validate(); err != nil {
		t.Fatal(err)
	}
	plan := engine.Plan{Operations: []engine.Operation{
		{Resource: "add", Action: "install", Privileged: true},
		{Resource: "remove", Action: "remove"},
	}}
	lines := planText(catalog, plan, "")
	beforeDetails := strings.Join(lines[:min(7, len(lines))], "\n")
	for _, expected := range []string{"Remove: 1", "Admin: 1", "Removed tool", "New tool"} {
		if !strings.Contains(beforeDetails, expected) {
			t.Fatal("approval buried consequential changes", expected, beforeDetails)
		}
	}
}
