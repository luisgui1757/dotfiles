package installer

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestFinalRemovalRefreshesReportsAfterSharedDependencyCollection(t *testing.T) {
	for _, split := range []bool{false, true} {
		for _, personal := range []bool{false, true} {
			c, d := engineFixture(t)
			dependency := filepath.Join(d.dir, "node")
			d.preserved = map[string][]string{"plugins": {dependency}}
			var want []string
			if personal {
				path := filepath.Join(d.dir, "personal")
				if err := os.WriteFile(path, []byte("keep"), 0600); err != nil {
					t.Fatal(err)
				}
				d.preserved["plugins"] = append(d.preserved["plugins"], path)
				want = []string{path}
			}
			request := Request{Schema: 1, Mode: "apply", Selected: []string{"editor", "agent"}}
			if _, err := applyFixture(c, d, approve(t, c, d, request)); err != nil {
				t.Fatal(err)
			}
			if split {
				request.Selected = []string{"agent"}
				if result, err := applyFixture(c, d, approve(t, c, d, request)); err != nil || !strings.Contains(result.Message, dependency) {
					t.Fatal("intermediate removal did not disclose its surviving dependency", result, err)
				}
			}
			request.Selected, request.RemoveShared = []string{}, []string{"node"}
			result, err := applyFixture(c, d, approve(t, c, d, request))
			if err != nil || strings.Contains(result.Message, dependency) {
				t.Fatal("completed removal reported a now-absent shared dependency", split, personal, result, err)
			}
			state, err := LoadState(d.statePath, d.home)
			if err != nil || state.Transaction != nil || !slices.Equal(preservedPaths(state), want) {
				t.Fatal("final reports lost personal data or retained obsolete package references", state, err)
			}
			if !personal && len(state.Receipts) != 0 {
				t.Fatal("empty removal receipts were not retired", state.Receipts)
			}
		}
	}
}
