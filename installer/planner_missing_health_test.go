package installer

import (
	"strings"
	"testing"
)

func TestMissingPrerequisiteReasonDisclosesHealthEvidence(t *testing.T) {
	c := testCatalog(t)
	observed := map[string]Observation{"node": {HealthIssue: "reinstall missing payload; historical receipt 1, install-time 100"}, "plugins": {}}
	plan, err := PlanChanges(c, linux, State{Schema: 1, Context: linux, Receipts: map[string]Receipt{}}, Request{Schema: 1, Mode: "apply", Selected: []string{"editor"}}, observed, "source")
	if err != nil {
		t.Fatal(err)
	}
	op := operation(t, plan, "node")
	if op.Action != "install" || !strings.Contains(op.Reason, observed["node"].HealthIssue) {
		t.Fatal(op)
	}
	if operation(t, plan, "plugins").Reason != "required by selection" {
		t.Fatal("ordinary absence reason changed")
	}
}
