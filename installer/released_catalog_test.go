package installer

import (
	"encoding/json"
	"os/exec"
	"strings"
	"testing"
)

func TestEachResourceOfAReleaseStaysInTheCatalog(t *testing.T) {
	tags, err := exec.Command("git", "tag", "--list", "v[1-9]*").Output()
	if err != nil || len(strings.Fields(string(tags))) == 0 {
		t.Fatal("the release tags are required: run git fetch --tags", err)
	}
	catalog, err := DefaultCatalog()
	if err != nil {
		t.Fatal(err)
	}
	floor := false
	for _, tag := range strings.Fields(string(tags)) {
		floor = floor || tag == supportFloor
		if sentinelOlder(tag, supportFloor) {
			continue
		}
		data, err := exec.Command("git", "show", tag+":installer/resources.json").Output()
		if err != nil {
			t.Fatal(tag, err)
		}
		var released struct {
			Resources []struct {
				ID string `json:"id"`
			} `json:"resources"`
		}
		if err := json.Unmarshal(data, &released); err != nil {
			t.Fatal(tag, err)
		}
		for _, r := range released.Resources {
			if _, retired := retiredResources[r.ID]; !retired && retiredCapabilities[r.ID] == "" {
				if _, known := catalog.Resource(r.ID); !known {
					t.Errorf("%s can record %s, and this catalog does not know it", tag, r.ID)
				}
			}
		}
	}
	if !floor {
		t.Fatal("the release tag", supportFloor, "is required: run git fetch --tags")
	}
}
