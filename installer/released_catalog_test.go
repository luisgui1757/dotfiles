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
	for _, tag := range strings.Fields(string(tags)) {
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
			if _, known := catalog.Resource(r.ID); !known && retiredCapabilities[r.ID] == "" {
				t.Errorf("%s can record %s, and this catalog does not know it", tag, r.ID)
			}
		}
	}
}
