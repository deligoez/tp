package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// assertRefusedLiteral asserts one write sink's refusal of an illegal
// audit_converge_on literal. It reads the parsed error envelope rather than raw
// stderr for the same reason its consume-side sibling does: output.Error
// JSON-encodes the message, so a raw substring assertion for a quoted literal
// never matches and reddens on a refusal that is in fact correct.
//
// The code is asserted twice — once as the process exit status and once as the
// envelope's own field — because v0.37.0 §2 makes the two error classes carry
// information: a write sink reports ExitUsage (2) because the offending value is
// on the command line, while a consuming sink reports ExitValidation (1) because
// the offending value is in the tree. A sink that exits 2 while reporting 1 in
// its envelope tells an agent parsing the JSON the wrong story.
func assertRefusedLiteral(t *testing.T, stderr string, code int) {
	t.Helper()
	e := errJSON(t, stderr)
	assert.Equal(t, 2, code, "an illegal literal argument is a usage error, not a validation error")
	assert.Equal(t, float64(2), e["code"], "and the envelope reports the same code")
	assert.Equal(t, `invalid audit_converge_on value "nope"`, e["error"],
		"the refusal names the field and quotes the value it refused")
	assert.Equal(t, "must be one of: blocking, all", e["hint"],
		"and names both legal literals, byte-identical to the review twin's hint")
}

// TestAuditConvergeOn_WriteSinksRefuseIllegalLiteral covers the write half of
// v0.37.0 §7 row 3: an illegal literal at either write sink — `tp set
// --workflow` and its `--project` form — exits ExitUsage with a hint naming the
// field and its two legal values. The mutant row 3 names is mapping both sinks
// to one code, which this test catches at the project sink, where the shipped
// unknown-field reply is ExitValidation.
//
// The exit code is deliberately not the only observable. A refusal that lands
// after the write would exit 2 and still have persisted the illegal value, so
// both sinks are also asserted to have left the tree exactly as they found it:
// the task file byte-identical, and no .tp/config.json created at all. That
// second observable is what separates a sink that validates from one that
// merely complains.
func TestAuditConvergeOn_WriteSinksRefuseIllegalLiteral(t *testing.T) {
	dir := writeStrategyProject(t, "{}")
	taskFile := filepath.Join(dir, "s.tasks.json")
	before, err := os.ReadFile(taskFile)
	require.NoError(t, err)

	_, stderr, code := runTP(t, dir, "set", "--workflow", "audit_converge_on=nope")
	assertRefusedLiteral(t, stderr, code)

	after, err := os.ReadFile(taskFile)
	require.NoError(t, err)
	assert.Equal(t, string(before), string(after),
		"a refused task-level write left the task file byte-identical")

	_, stderr, code = runTP(t, dir, "set", "--workflow", "--project", "audit_converge_on=nope")
	assertRefusedLiteral(t, stderr, code)

	_, statErr := os.Stat(filepath.Join(dir, ".tp", "config.json"))
	assert.True(t, os.IsNotExist(statErr),
		"a refused project-level write created no project config")

	after, err = os.ReadFile(taskFile)
	require.NoError(t, err)
	assert.Equal(t, string(before), string(after),
		"and did not reach the task file either")
}

