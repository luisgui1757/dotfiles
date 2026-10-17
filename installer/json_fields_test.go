package installer

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestJSONFieldPublicationPreservesUnrelatedBytesAndRestoresPriorValue(t *testing.T) {
	for _, original := range []string{
		"{}", "{\r\n  \r\n}", "{ \"model\" : {\"limit\":9007199254740993}, \"items\" : [1, 2] }\r\n",
		"{\"theme\":\"old\", \"model\":\"personal\"}", "{\"model\":\"personal\", \"theme\" : \"old\"}",
		"{\"theme\":\"\\u006fld\"}",
	} {
		fields := []string{"theme"}
		installed, before, err := replaceJSONFields([]byte(original), fields, []byte(`{"theme":"rose-pine"}`))
		if err != nil {
			t.Fatal(original, err)
		}
		repeated, _, err := replaceJSONFields(installed, fields, []byte(`{"theme":"rose-pine"}`))
		if err != nil || !bytes.Equal(installed, repeated) {
			t.Fatal("repeated publication changed settings", err)
		}
		restored, _, err := replaceJSONFields(installed, fields, before)
		if err != nil || string(restored) != original {
			t.Fatalf("original bytes changed: %q -> %q: %v", original, restored, err)
		}
	}
}

func TestJSONFieldRemovalPreservesLaterPersonalFields(t *testing.T) {
	for _, current := range []string{`{"theme":"rose-pine","model":"later"}`, `{"model":"later","theme":"rose-pine"}`, `{"first":1,"theme":"rose-pine","last":2}`} {
		removed, before, err := replaceJSONFields([]byte(current), []string{"theme"}, nil)
		if err != nil || string(before) != `{"theme":"rose-pine"}` {
			t.Fatal(err, string(before))
		}
		var values map[string]json.RawMessage
		if err := json.Unmarshal(removed, &values); err != nil || values["theme"] != nil || len(values) == 0 {
			t.Fatal("removal damaged unrelated fields", string(removed), err)
		}
	}
}

func TestJSONFieldsRejectAmbiguousAndOutOfScopeInputs(t *testing.T) {
	for _, data := range []string{"[]", "null", "42", `{\"theme\":`, `{"theme":"a","theme":"b"}`, "{} {}", "{\"theme\":\"\xff\"}", "{\"theme\":\"x\",}", "{ /*comment*/ }"} {
		if _, _, err := replaceJSONFields([]byte(data), []string{"theme"}, []byte(`{"theme":"rose-pine"}`)); err == nil {
			t.Fatalf("accepted malformed settings %q", data)
		}
	}
	for _, replacement := range []string{`{"unowned":true}`, `{"theme":1,"theme":2}`, `[]`} {
		if _, _, err := replaceJSONFields([]byte("{}"), []string{"theme"}, []byte(replacement)); err == nil {
			t.Fatal("accepted invalid replacement", replacement)
		}
	}
}
