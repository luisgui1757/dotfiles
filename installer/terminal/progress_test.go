package terminal

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	engine "github.com/luisgui1757/dotfiles/installer"
)

func TestProgressOutputRemovesWholeEscapeSequences(t *testing.T) {
	input := "\x1b[1;31mred\x1b[0m plain\x1b]0;title\x07 end\x1b(B.\x1b]8;;https://x\x1b\\link\ttab\u009b2J\x00"
	if got := cleanOutput(input); got != "red plain end.link\ttab2J" {
		t.Fatalf("%q", got)
	}
}

func progressFixture(t *testing.T) (*progressView, func() string) {
	t.Helper()
	file, err := os.Create(filepath.Join(t.TempDir(), "screen"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { file.Close() })
	return &progressView{output: file}, func() string {
		data, err := os.ReadFile(file.Name())
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}
}

func TestProgressRewritesCarriageReturnLinesAndIndentsOutput(t *testing.T) {
	view, screen := progressFixture(t)
	view.Step("[1/2] Installing Neovim")
	for _, chunk := range []string{"Get:1 pack", "age\n10%\r50", "%\r100%\n", "windows line\r\n", "partial"} {
		if n, err := view.Write([]byte(chunk)); err != nil || n != len(chunk) {
			t.Fatal(n, err)
		}
	}
	view.Step("[2/2] Installing Git")
	if err := view.Close(); err != nil {
		t.Fatal(err)
	}
	want := "\x1b[1m==> [1/2] Installing Neovim\x1b[0m\n" +
		"    Get:1 package\n" +
		"    10%\r\x1b[K    50%\r\x1b[K    100%\n" +
		"    windows line\n" +
		"    partial\n" +
		"\x1b[1m==> [2/2] Installing Git\x1b[0m\n"
	if got := screen(); got != want {
		t.Fatalf("got  %q\nwant %q", got, want)
	}
}

// Windows PowerShell children serialize progress and information records as
// CLIXML on stderr. The live display hides that markup, including a document
// the 4096-byte wrap splits, and keeps the plain lines around it.
func TestProgressHidesPowerShellCLIXMLRecords(t *testing.T) {
	view, screen := progressFixture(t)
	view.Step("[1/2] Remove the earlier fd")
	document := `<Objs Version="1.1.0.1" xmlns="http://schemas.microsoft.com/powershell/2004/04">` +
		strings.Repeat(`<Obj S="progress" RefId="0"><AV>Preparing modules for first use.</AV></Obj>`, 80) + "</Objs>"
	for _, chunk := range []string{"#< CLIXML\r\n", "Uninstalling 'fd' (10.4.2).\n", document[:100], document[100:] + "\n", "'fd' was uninstalled.\n",
		"#< CLIXML\n", `<Objs Version="1.1.0.1"><Obj S="progress"`} {
		if n, err := view.Write([]byte(chunk)); err != nil || n != len(chunk) {
			t.Fatal(n, err)
		}
	}
	view.Step("[2/2] Remove the earlier fzf")
	if _, err := view.Write([]byte("after an unterminated document\n")); err != nil {
		t.Fatal(err)
	}
	if err := view.Close(); err != nil {
		t.Fatal(err)
	}
	want := "\x1b[1m==> [1/2] Remove the earlier fd\x1b[0m\n" +
		"    Uninstalling 'fd' (10.4.2).\n" +
		"    'fd' was uninstalled.\n" +
		"\x1b[1m==> [2/2] Remove the earlier fzf\x1b[0m\n" +
		"    after an unterminated document\n"
	if got := screen(); got != want {
		t.Fatalf("got  %q\nwant %q", got, want)
	}
}

func TestProgressWrapsEndlessLinesAtCharacterBoundaries(t *testing.T) {
	view, screen := progressFixture(t)
	line := strings.Repeat("é", 3000)
	if _, err := view.Write([]byte(line + "\n")); err != nil {
		t.Fatal(err)
	}
	if err := view.Close(); err != nil {
		t.Fatal(err)
	}
	if got := strings.ReplaceAll(strings.ReplaceAll(screen(), progressIndent, ""), "\n", ""); got != line {
		t.Fatal("wrapping lost or split characters")
	}
}

// A display failure must not fail the provider's pipe; the report returns it.
func TestProgressKeepsDisplayFailureForTheReport(t *testing.T) {
	view, _ := progressFixture(t)
	if err := view.output.Close(); err != nil {
		t.Fatal(err)
	}
	if n, err := view.Write([]byte("native output\n")); err != nil || n != len("native output\n") {
		t.Fatal("display failure reached the native command", n, err)
	}
	view.Step("next")
	if err := view.Close(); err == nil {
		t.Fatal("display failure was lost")
	}
}

func TestReportSeparatesNotesFromActions(t *testing.T) {
	text := reportText(engine.Result{Status: "needs-action", Message: "nvim.sync: waiting for git\nExternal application not exercised: X11 clipboard\nPreserved files for inspection: /saved"})
	actions := strings.Index(text, "  - nvim.sync: waiting for git")
	notes := strings.Index(text, "Notes (informational, not failures):")
	if actions < 0 || notes < actions || !strings.Contains(text[notes:], "  - External application not exercised: X11 clipboard") || !strings.Contains(text[notes:], "  - Preserved files for inspection: /saved") {
		t.Fatalf("%q", text)
	}
}
