package installer

import (
	"crypto/sha256"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNativeBrewCaskDependencyUpdatePreservesExternalApplication(t *testing.T) {
	testNativeBrewSharedLibrary(t, false, true)
}

// The cask's binary invokes an actual compiled external formula consumer. The
// installer may maintain that formula but cannot acquire or rewrite the cask.
func (f *nativeBrewFixture) installConsumerCask(name, formula, program string) string {
	f.t.Helper()
	payload := archiveTar(f.t, []archiveFixtureEntry{{Name: "fixture/application", Text: "#!/bin/sh\nexec " + shellLiteral(program) + " \"$@\"\n", Mode: 0755}})
	archive := filepath.Join(f.root, name+".tar.gz")
	if err := os.WriteFile(archive, payload, 0600); err != nil {
		f.t.Fatal(err)
	}
	archiveURL := url.URL{Scheme: "file", Path: archive}
	definition := fmt.Sprintf(`cask %q do
	version "1.0"
	sha256 %q
	url %q
	name "Dotfiles native contract application"
	desc "Isolated native dependency consumer"
	homepage "https://example.invalid/"
	depends_on formula: %q
	binary "fixture/application", target: %q
end
`, name, fmt.Sprintf("%x", sha256.Sum256(payload)), archiveURL.String(), formula, name)
	writeConfigFixture(f.t, filepath.Join(f.tapRoot, "Casks", name+".rb"), definition)
	f.casks = append(f.casks, f.tap+"/"+name)
	f.require("", "install", "--cask", f.tap+"/"+name)
	prefix := strings.TrimSpace(string(f.require("", "--prefix")))
	if !filepath.IsAbs(prefix) {
		f.t.Fatal("Homebrew prefix must be absolute", prefix)
	}
	return filepath.Join(prefix, "bin", name)
}
