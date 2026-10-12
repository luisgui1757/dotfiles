package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	engine "github.com/luisgui1757/dotfiles/installer"
	"github.com/luisgui1757/dotfiles/installer/terminal"
)

// screenFixture is an ordinary-file terminal surface; its reports are kept for
// inspection and the alternate screen is unavailable.
func screenFixture(t *testing.T) (*migrationReports, func() string) {
	t.Helper()
	input, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "screen")
	output, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { input.Close(); output.Close() })
	return &migrationReports{Workflow: &terminal.Workflow{Input: input, Output: output}}, func() string {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}
}

// The migrate entrypoint prints the setup report, then exits with the stop
// message on stderr. Together they must show each actionable reason once.
func TestMigrationStopDoesNotRepeatPrintedReport(t *testing.T) {
	reports, screen := screenFixture(t)
	setup := engine.Result{Status: "needs-action", Plan: engine.Plan{Mode: "apply"}, Message: "skills.sentinel-agents: another installer owns these skills\nPreserved files for inspection: /home/user/.claude/previous"}
	if err := reports.Report(setup); err != nil {
		t.Fatal(err)
	}
	stopped := reports.stopped("setup", setup)
	shown := screen() + "Dotfiles: " + stopped.Message + "\n"
	for _, line := range strings.Split(setup.Message, "\n") {
		if strings.Count(shown, line) != 1 {
			t.Fatalf("report line shown %d times: %q\n%s", strings.Count(shown, line), line, shown)
		}
	}
	if stopped.Status != "needs-action" || !strings.Contains(stopped.Message, "after the setup phase") || !strings.Contains(stopped.Message, "Run migrate again") {
		t.Fatal("stop message lost its phase or next step", stopped)
	}
}

func TestMigrationStopKeepsReportsThatAreNotOnScreen(t *testing.T) {
	printed := engine.Result{Status: "needs-action", Plan: engine.Plan{Mode: "apply"}, Message: "earlier phase report"}
	check := engine.Result{Status: "needs-action", Plan: engine.Plan{Mode: "check"}, Message: "check report on the alternate screen"}
	unreported := engine.Result{Status: "needs-action", Message: "profile preparation stopped before its report"}
	for name, report := range map[string]*engine.Result{"unreported": nil, "different": &printed, "check": &check} {
		t.Run(name, func(t *testing.T) {
			reports, _ := screenFixture(t)
			want := unreported
			if report != nil {
				// A check report opens the alternate screen, which this
				// non-terminal fixture refuses; it was never left on screen.
				_ = reports.Report(*report)
				if report == &check {
					want = check
				}
			}
			stopped := reports.stopped("setup check", want)
			if !strings.Contains(stopped.Message, want.Message) {
				t.Fatal("stop message dropped a report the user cannot see", stopped.Message)
			}
		})
	}
	cancelled := engine.Result{Status: "cancelled", Message: "nothing changed"}
	if got := (&migrationReports{}).stopped("setup", cancelled); got.Status != "cancelled" || got.Message != cancelled.Message {
		t.Fatal("cancellation must pass through unchanged", got)
	}
}
