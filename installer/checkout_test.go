package installer

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCRLFCheckoutPreservesInstallerBytes(t *testing.T) {
	dir := t.TempDir()
	external := t.TempDir()
	index := filepath.Join(external, "protected-index")
	if err := os.WriteFile(index, []byte("preserve this index"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_DIR", filepath.Join(external, "outside.git"))
	t.Setenv("GIT_WORK_TREE", external)
	t.Setenv("GIT_INDEX_FILE", index)
	attributes, err := os.ReadFile("../.gitattributes")
	if err != nil {
		t.Fatal(err)
	}
	files := map[string][]byte{
		".gitattributes":                         attributes,
		"installer/example.go":                   []byte("package example\n\nfunc Value() int {\n\treturn 1\n}\n"),
		"installer/go.mod":                       []byte("module example\n\ngo 1.27.1\n"),
		"installer/go.sum":                       []byte("example v1.0.0 h1:fixture\n"),
		"installer/resources.json":               []byte("{\n  \"schema\": 1\n}\n"),
		".github/workflows/installer-engine.yml": []byte("name: test\n"),
		".github/workflows/nix.yml":              []byte("name: nix\n"),
	}
	for name, data := range files {
		path := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", dir, "-c", "core.autocrlf=true"}, args...)...)
		for _, variable := range os.Environ() {
			if !strings.HasPrefix(variable, "GIT_") {
				cmd.Env = append(cmd.Env, variable)
			}
		}
		cmd.Env = append(cmd.Env, "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1")
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, output)
		}
	}
	git("init", "-q")
	git("add", ".")
	for name := range files {
		if name == ".gitattributes" {
			continue
		}
		if err := os.Remove(filepath.Join(dir, filepath.FromSlash(name))); err != nil {
			t.Fatal(err)
		}
	}
	git("checkout-index", "-a")
	for name, expected := range files {
		if name == ".gitattributes" {
			continue
		}
		actual, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(name)))
		if err != nil || !bytes.Equal(actual, expected) {
			t.Fatalf("checkout changed %s bytes: %v", name, err)
		}
	}
	protected, err := os.ReadFile(index)
	if err != nil || string(protected) != "preserve this index" {
		t.Fatal("inherited Git environment affected an external index", err)
	}
}
