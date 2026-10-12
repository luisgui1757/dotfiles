package installer

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

// This classifies only the unmodified upstream baseline. Every corrected
// process/bootstrap boundary remains mandatory in the native Windows fixture.
func windowsPowerShellBaselineDiagnostic(output []byte, err error, want string) (string, error) {
	if err == nil {
		if strings.TrimSpace(string(output)) == want {
			return "unmodified PS7 -> Go -> PS5.1 already returns the correct hash", nil
		}
	} else if strings.Contains(string(output), "CommandNotFoundException") && strings.Contains(string(output), "Get-FileHash") {
		return "unmodified PS7 -> Go -> PS5.1 cannot autoload Get-FileHash", nil
	}
	return "", fmt.Errorf("unexpected raw native hash result: %v %s", err, output)
}

func TestWindowsPowerShellBaselineDiagnostic(t *testing.T) {
	want := strings.Repeat("a", 64)
	for _, tc := range []struct {
		name, output string
		err          error
		valid        bool
	}{
		{"already-correct", want + "\r\n", nil, true},
		{"known-autoload-failure", "Get-FileHash: CommandNotFoundException", errors.New("exit status 1"), true},
		{"wrong-hash", strings.Repeat("b", 64), nil, false},
		{"empty-success", "", nil, false},
		{"error-looking-success", "Get-FileHash: CommandNotFoundException", nil, false},
		{"different-failure", "Get-FileHash: UnauthorizedAccessException", errors.New("exit status 1"), false},
		{"unrelated-command", "Other-Command: CommandNotFoundException", errors.New("exit status 1"), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			diagnostic, err := windowsPowerShellBaselineDiagnostic([]byte(tc.output), tc.err, want)
			if (err == nil) != tc.valid || tc.valid && diagnostic == "" {
				t.Fatal("raw baseline classification differs", diagnostic, err)
			}
		})
	}
}
