package cli

import (
	"bytes"
	"encoding/json"
)

// importedHasWorkflowKey reports whether the raw imported document is a JSON
// object with a top-level "workflow" key, checked on the raw bytes before
// struct defaulting can invent one.
func importedHasWorkflowKey(raw []byte) bool {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return false
	}
	var keys map[string]json.RawMessage
	if err := json.Unmarshal(trimmed, &keys); err != nil {
		return false
	}
	_, ok := keys["workflow"]
	return ok
}
