package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/deligoez/tp/internal/engine"
)

// TestIsAuditableType_ReviewStateIsNotCode: a spec's .tp-review/ directory
// holds round files, snapshots, floors and state.json, which tp writes and
// commits beside the spec. They are not code a spec is audited against, yet
// on tp's own history they took 99 of 1,360 top-ten slots of the code roles'
// lists (spec/1.21.0-measurements.md), as .tp/ would have before it was
// filtered.
func TestIsAuditableType_ReviewStateIsNotCode(t *testing.T) {
	t.Parallel()
	for _, p := range []string{
		"spec/.tp-review/1.0.0/state.json",
		"spec/.tp-review/1.0.0/review-round-1.ndjson",
		".tp-review/spec/floor-ground-round-1.txt",
	} {
		assert.False(t, engine.IsAuditableType(p), "%s is tp's review state", p)
	}
	for _, p := range []string{"internal/tp-review/x.go", "a.go", "cmd/.tp-reviewer/main.go"} {
		assert.True(t, engine.IsAuditableType(p), "%s is code", p)
	}
}
