package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// tp renders the category enum into every auditor prompt in words that promise
// "unknown values are rejected". For as long as the enum existed, nothing
// checked what came back — a row could carry any string and be written into the
// permanent round file that convergence is computed from. These pin the sink.

func TestParseAuditRows_RejectsCategoryOutsideTheEnum(t *testing.T) {
	t.Parallel()
	data := []byte(`{"role":"go-safety","item_id":"a","status":"FAIL","category":"banana"}
{"role":"go-safety","item_id":"b","status":"FAIL","category":"security"}
{"role":"go-safety","item_id":"c","status":"FAIL","category":"misc"}
`)

	_, _, err := parseAuditRows("r.ndjson", data)
	require.Error(t, err, "a category outside the enum must not enter the record")

	// Every offending line at once: an operator fixing one and re-running only
	// to meet the next is the round-trip this shape exists to avoid.
	assert.Contains(t, err.Error(), `line 1: "banana"`)
	assert.Contains(t, err.Error(), `line 3: "misc"`)

	// The message names what tp will take, so the fix needs no second lookup.
	assert.Contains(t, err.Error(), "security, concurrency, error-handling, correctness, contract")
}

func TestParseAuditRows_AcceptsTheEnumAndAbsentCategory(t *testing.T) {
	t.Parallel()
	// PASS rows carry an explicit null, which unmarshals to no string at all;
	// treating that as invalid would reject every clean round.
	data := []byte(`{"role":"go-safety","item_id":"a","status":"FAIL","category":"security"}
{"role":"go-safety","item_id":"b","status":"PASS","category":null}
{"role":"go-safety","item_id":"c","status":"PARTIAL","category":"contract"}
`)

	_, findings, err := parseAuditRows("r.ndjson", data)
	require.NoError(t, err)
	assert.Equal(t, 2, findings, "PASS does not count; FAIL and PARTIAL do")
}
