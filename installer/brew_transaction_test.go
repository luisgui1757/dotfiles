package installer

import (
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestBrewEvidenceIdentifiesTheInstallingProcess(t *testing.T) {
	op := strings.Repeat("b", 64)
	marker := "dotfiles-operation-" + op + "  "
	cellar := "/opt/homebrew/Cellar"
	output := "Already installed: reused\n" + marker + cellar + "/first/1.0: 4 files, 20KB\n" + marker + cellar + "/shared/2.0_1: 3 files, 10KB\n"
	got, err := brewInstallEvidence([]byte(output), cellar, op)
	want := []brewInstalledKeg{{"first", "1.0", cellar + "/first/1.0"}, {"shared", "2.0_1", cellar + "/shared/2.0_1"}}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatal(got, err)
	}
	if got, err := brewInstallEvidence([]byte(output), cellar, strings.Repeat("c", 64)); err != nil || len(got) != 0 {
		t.Fatal("another operation acquired this process's evidence", got, err)
	}
	for _, invalid := range []string{
		marker + "/outside/first/1.0: 4 files",
		marker + cellar + "/first/../other/1.0: 4 files",
		marker + cellar + "/first: 4 files",
		marker + cellar + "/first/1.0:",
		output + marker + cellar + "/first/1.0: 4 files",
	} {
		if _, err := brewInstallEvidence([]byte(invalid), cellar, op); err == nil {
			t.Fatal("ambiguous completion record accepted", invalid)
		}
	}
}

func TestBrewEvidenceRequiresExactConfiguredNativeCellar(t *testing.T) {
	cellar := filepath.ToSlash(t.TempDir())
	operation := strings.Repeat("a", 64)
	output := []byte(fmt.Sprintf("dotfiles-operation-%s  %s/first/1.0: fixture\n", operation, cellar))
	kegs, err := brewInstallEvidence(output, cellar, operation)
	if err != nil || len(kegs) != 1 || kegs[0].Name != "first" {
		t.Fatal("canonical native filesystem path was rejected", kegs, err)
	}
	if _, err := brewInstallEvidence(output, cellar+"/other", operation); err == nil {
		t.Fatal("a different cellar acquired this command's packages")
	}
}
