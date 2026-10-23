package installer

import (
	"crypto/sha256"
	"fmt"
	"testing"
)

// These script hashes come from immutable Git blob fcaa28be53b689b01b792ca80c56a97a99c5b252 at
// 5d6518b. New exact-entry approval must not reinterpret saved whole-PATH steps.
func TestLegacyUserPathHistoricalRecipesKeepTheirBytes(t *testing.T) {
	for _, recipe := range []struct{ script, sha256 string }{
		{legacyUserPathRead, "e81469e44a2343b9c206d42d772a5a31384a23c1757110bc48f89ed8f864005e"},
		{legacyUserPathQuery, "b0e263b3005cfca151df96bb693907a8d85f0fd33fb54c1b52e270e2d0a3416c"},
		{legacyUserPathEdit, "5ed28fdb35b88100f0a948cb788f0cd35660f24253cf0a461cda89bfddf96d1b"},
	} {
		if fmt.Sprintf("%x", sha256.Sum256([]byte(recipe.script))) != recipe.sha256 {
			t.Fatal("historical User PATH recipe changed")
		}
	}
}
