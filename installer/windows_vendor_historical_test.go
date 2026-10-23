package installer

import (
	"bytes"
	"crypto/sha1"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func windowsVendorHistoricalScript(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "windows-vendor-process-v0.go"))
	if err != nil {
		t.Fatal(err)
	}
	// Git's immutable blob identity binds the fixture to the actual old source,
	// rather than reconstructing a supposed old command from current constants.
	object := append(fmt.Appendf(nil, "blob %d\x00", len(data)), data...)
	if fmt.Sprintf("%x", sha1.Sum(object)) != "1cf1a58f06749d227ae95b564598e06cf69f4715" {
		t.Fatal("historical vendor script no longer matches its Git object")
	}
	file, err := parser.ParseFile(token.NewFileSet(), "historical.go", data, 0)
	if err != nil {
		t.Fatal(err)
	}
	values := map[string]string{}
	var evaluate func(ast.Expr) string
	evaluate = func(expression ast.Expr) string {
		switch value := expression.(type) {
		case *ast.BasicLit:
			text, err := strconv.Unquote(value.Value)
			if err != nil || value.Kind != token.STRING {
				t.Fatal("invalid historical constant", err)
			}
			return text
		case *ast.Ident:
			text, exists := values[value.Name]
			if !exists {
				t.Fatal("historical constant dependency missing", value.Name)
			}
			return text
		case *ast.BinaryExpr:
			if value.Op != token.ADD {
				t.Fatal("unsupported historical constant expression")
			}
			return evaluate(value.X) + evaluate(value.Y)
		default:
			t.Fatal("unsupported historical constant")
			return ""
		}
	}
	for _, declaration := range file.Decls {
		group, ok := declaration.(*ast.GenDecl)
		if !ok || group.Tok != token.CONST {
			continue
		}
		for _, spec := range group.Specs {
			value := spec.(*ast.ValueSpec)
			if len(value.Names) != 1 || len(value.Values) != 1 {
				t.Fatal("unexpected historical constant declaration")
			}
			values[value.Names[0].Name] = evaluate(value.Values[0])
		}
	}
	script := values["windowsVendorInstallScript"]
	if script == "" {
		t.Fatal("historical install recipe is missing")
	}
	return script
}

func TestWindowsVendorHistoricalArguments(t *testing.T) {
	script := windowsVendorHistoricalScript(t)
	for _, arguments := range [][]string{windowsVendorArguments(script), legacyWindowsPowerShellArguments(script)} {
		if !windowsVendorHistoricalArguments(arguments) {
			t.Fatal("actual historical command encoding is no longer readable")
		}
		arguments[0] = "-NoExit"
		if windowsVendorHistoricalArguments(arguments) {
			t.Fatal("changed historical command was accepted")
		}
	}
}

func TestWindowsVendorHistoricalCompletedAndPending(t *testing.T) {
	for _, prefixed := range []bool{false, true} {
		for _, complete := range []bool{false, true} {
			t.Run(fmt.Sprintf("modules=%t/complete=%t", prefixed, complete), func(t *testing.T) {
				f := newWindowsVendorFixture(t)
				f.interrupt = !complete
				if _, err := f.apply(); (err == nil) != complete {
					t.Fatal("unexpected fixture completion", err)
				}
				intent, err := f.driver.readIntent(f.receipt.OperationID)
				if err != nil {
					t.Fatal(err)
				}
				script := windowsVendorHistoricalScript(t)
				if prefixed {
					intent.Commands[0].Arguments = windowsVendorArguments(script)
				} else {
					intent.Commands[0].Arguments = legacyWindowsPowerShellArguments(script)
				}
				path := f.driver.intentPath(intent.Operation)
				if err := saveDocument(path, intent); err != nil {
					t.Fatal(err)
				}
				if complete {
					command := intent.Commands[0]
					hash, err := digest(command)
					if err != nil {
						t.Fatal(err)
					}
					code := 0
					if err := saveDocument(filepath.Join(f.driver.WorkerDirectory, "commands", command.Operation+".json"), nativeCommandRecord{Schema: 1, Command: hash, Reply: &nativeReply{ExitCode: &code}}); err != nil {
						t.Fatal(err)
					}
				}
				before, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				observed, err := f.apply()
				if complete && (err != nil || !observed.Healthy || observed.CompletedOperation != intent.Operation) || !complete && (err == nil || !strings.Contains(err.Error(), "saved Windows vendor command differs")) {
					t.Fatal("historical completion/dispatch boundary failed", observed, err)
				}
				after, err := os.ReadFile(path)
				if err != nil || !bytes.Equal(before, after) || f.runs != 1 {
					t.Fatal("historical journal rewritten or native command replayed", err)
				}
			})
		}
	}
}
