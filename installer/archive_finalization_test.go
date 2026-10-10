package installer

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestArchiveUnrelatedDamageCannotBlockApplyOrAbandon(t *testing.T) {
	for _, action := range []string{"apply", "abandon"} {
		for _, damage := range []string{"current", "entrypoint", "versions"} {
			t.Run(action+"/"+damage, func(t *testing.T) {
				c, d, _ := archiveController(t)
				request := Request{Schema: 1, Mode: "apply", Selected: []string{"first"}}
				if action == "abandon" {
					c.Driver = profileUnrecordedPublication{d}
					plan, err := c.Preview(context.Background(), request)
					if err != nil {
						t.Fatal(err)
					}
					request.ExpectedPlan = plan.ID
					if _, err := c.Dispatch(context.Background(), request); err == nil || !strings.Contains(err.Error(), "fixture stopped") {
						t.Fatal("fixture did not interrupt", err)
					}
					c.Driver = d
					request = Request{Schema: 1, Mode: "abandon"}
				}
				d.Pins["tool.unrelated"] = d.Pins["tool.shared"]
				directory := d.resourceDirectory("tool.unrelated")
				if err := os.MkdirAll(directory, 0700); err != nil {
					t.Fatal(err)
				}
				switch damage {
				case "current":
					writeConfigFixture(t, filepath.Join(directory, "current.json"), "{damaged selection")
				case "entrypoint":
					if err := createDirectoryLink(t.TempDir(), d.currentLink("tool.unrelated")); err != nil {
						t.Fatal(err)
					}
				case "versions":
					writeConfigFixture(t, filepath.Join(directory, "versions"), "preserve this unexpected file")
				}
				before, err := snapshotTree(directory, maxPackageBytes, maxPackageEntries)
				if err != nil {
					t.Fatal(err)
				}
				result := dispatchApproved(t, c, request)
				if !strings.Contains(result.Message, directory) {
					t.Fatal("unrelated damage was not disclosed", result)
				}
				if err := verifyConfigSnapshot(directory, before); err != nil {
					t.Fatal("unrelated damage was mutated", err)
				}
			})
		}
	}
}
