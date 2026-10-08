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

	engine "github.com/luisgui1757/dotfiles/installer"
	"golang.org/x/term"
)

func TestReviewContainsDependencyOwnershipPrivilegeAndCompleteLongReason(t *testing.T) {
	catalog := &engine.Catalog{Schema: 1, Resources: []engine.Resource{{ID: "compiler", Name: "Compiler", Action: "compiler"}}}
	if err := catalog.Validate(); err != nil {
		t.Fatal(err)
	}
	reason := strings.Repeat("compilers are needed for parser builds ", 10)
	plan := engine.Plan{Mode: "apply", ID: strings.Repeat("a", 64), Source: "source", Target: "/fixture", Operations: []engine.Operation{
		{Resource: "compiler", Action: "install", Reason: reason, RequiredBy: []string{"neovim"}, Ownership: "unrecorded", Privileged: true, Observed: engine.Observation{UnverifiedApplications: []string{"external-app"}}},
	}}
	lines := planText(catalog, plan)
	for _, expected := range []string{"Why: " + reason, "Required by: neovim", "Ownership: unrecorded", "Requires administrator privileges.", "External application not exercised: external-app"} {
		if !slices.Contains(lines, expected) {
			t.Fatal("approval omitted consequential detail", expected)
		}
	}
	if strings.Join(wrapLines([]string{reason}, 31), "") != reason {
		t.Fatal("narrow preview lost approval text")
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
		{ID: "agent", Name: "Coding agent", Capability: true, Requires: []string{"runtime"}},
		{ID: "runtime", Name: "Shared runtime", Action: "runtime", Shared: true},
	}}
	c := engine.Controller{Catalog: catalog, Context: engine.Context{OS: runtime.GOOS, Arch: runtime.GOARCH}, Home: home,
		StatePath: filepath.Join(home, "ledger", "state.json"), Source: "native-ui-fixture", Driver: workflowFiles{home}}
	result, err := c.Run(context.Background(), Workflow{Input: os.Stdin, Output: os.Stdout})
	if err != nil {
		t.Fatal(err)
	}
	expected := "cancelled"
	if mode == "workflow-apply" {
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

func TestNativeControllerWorkflowAppliesOnlyAfterExplicitConfirmation(t *testing.T) {
	for _, mode := range []string{"apply", "back", "cancel"} {
		t.Run(mode, func(t *testing.T) {
			s := startNativeSession(t, "workflow-"+mode)
			s.waitFor(t, "Choose the tools you want installed")
			if mode == "cancel" {
				s.send(t, "q")
			} else {
				s.send(t, " \r")
				s.waitFor(t, "Review every change")
				s.waitFor(t, "Required by: agent")
				s.send(t, "\r")
				s.waitFor(t, "Apply this exact plan?")
				if mode == "apply" {
					s.send(t, "j\r")
				} else {
					s.send(t, "\r")
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

func TestReviewNeutralizesTerminalControlsWithoutDroppingVisibleText(t *testing.T) {
	if got := strings.Join(wrapLines([]string{"ab\x1b[2Jcd\x00ef\u0085g"}, 4), ""); got != "ab[2Jcdefg" {
		t.Fatal(got)
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
	lines := planText(catalog, plan)
	beforeDetails := strings.Join(lines[:min(7, len(lines))], "\n")
	for _, expected := range []string{"Remove: 1", "Admin: 1", "Removed tool", "New tool"} {
		if !strings.Contains(beforeDetails, expected) {
			t.Fatal("approval buried consequential changes", expected, beforeDetails)
		}
	}
}
