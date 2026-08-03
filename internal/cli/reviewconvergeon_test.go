package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestReviewConvergeOnTaskLevelSetAndResolved covers §3.3: review_converge_on is
// settable per task and flows into tp config --resolved attributed to override.
func TestReviewConvergeOnTaskLevelSetAndResolved(t *testing.T) {
	dir := writeStrategyProject(t, "{}")

	out, stderr, code := runTP(t, dir, "set", "--workflow", "review_converge_on=all")
	require.Equal(t, 0, code, "review_converge_on is settable per task: %s", stderr)
	var res map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &res))
	assert.Equal(t, "all", res["updated"].(map[string]any)["review_converge_on"])

	out, _, code = runTP(t, dir, "config", "--resolved")
	require.Equal(t, 0, code)
	require.NoError(t, json.Unmarshal([]byte(out), &res))
	field := res["workflow"].(map[string]any)["review_converge_on"].(map[string]any)
	assert.Equal(t, "all", field["value"])
	assert.Equal(t, "override", field["source"])
}

// TestReviewConvergeOnProjectLevelSet covers §3.3: the project default is
// settable via --project into .tp/config.json and resolves at the project layer.
func TestReviewConvergeOnProjectLevelSet(t *testing.T) {
	dir := writeStrategyProject(t, "{}")

	out, stderr, code := runTP(t, dir, "set", "--workflow", "--project", "review_converge_on=all")
	require.Equal(t, 0, code, "project review_converge_on is settable: %s", stderr)
	var res map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &res))
	assert.Equal(t, "all", res["updated"].(map[string]any)["review_converge_on"])

	out, _, code = runTP(t, dir, "config", "--resolved")
	require.Equal(t, 0, code)
	require.NoError(t, json.Unmarshal([]byte(out), &res))
	field := res["workflow"].(map[string]any)["review_converge_on"].(map[string]any)
	assert.Equal(t, "all", field["value"], "the project default flows into resolution")
	assert.Equal(t, "project", field["source"], "the project layer is named")
}

// TestReviewConvergeOnDefaultBlocking covers §3.3: the built-in default is
// blocking, reported with source default when no layer sets it.
func TestReviewConvergeOnDefaultBlocking(t *testing.T) {
	dir := writeStrategyProject(t, "{}")

	out, _, code := runTP(t, dir, "config", "--resolved")
	require.Equal(t, 0, code)
	var res map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &res))
	field := res["workflow"].(map[string]any)["review_converge_on"].(map[string]any)
	assert.Equal(t, "blocking", field["value"], "the built-in default is blocking")
	assert.Equal(t, "default", field["source"])
}

// TestReviewConvergeOnRejectsInvalidLiteral covers §3.3: both set commands
// validate the literal argument and reject a bad value as a usage error (exit 2)
// with a hint naming the legal values.
func TestReviewConvergeOnRejectsInvalidLiteral(t *testing.T) {
	dir := writeStrategyProject(t, "{}")

	_, stderr, code := runTP(t, dir, "set", "--workflow", "review_converge_on=bogus")
	require.Equal(t, 2, code, "an invalid task-level literal is a usage error")
	assert.Contains(t, stderr, "must be one of: blocking, all")

	_, stderr, code = runTP(t, dir, "set", "--workflow", "--project", "review_converge_on=bogus")
	require.Equal(t, 2, code, "an invalid project-level literal is a usage error")
	assert.Contains(t, stderr, "must be one of: blocking, all")
}

// TestReviewConvergeOnConfigResolvedShowsRawInvalidStored covers §3.3: a
// hand-edited stored value that is invalid is surfaced raw by tp config
// --resolved without erroring, so an operator can locate the offending layer.
func TestReviewConvergeOnConfigResolvedShowsRawInvalidStored(t *testing.T) {
	dir := writeStrategyProject(t, `{"review_converge_on":"bogus"}`)

	out, _, code := runTP(t, dir, "config", "--resolved")
	require.Equal(t, 0, code, "config --resolved does not validate the field")
	var res map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &res))
	field := res["workflow"].(map[string]any)["review_converge_on"].(map[string]any)
	assert.Equal(t, "bogus", field["value"], "the raw stored value is surfaced")
	assert.Equal(t, "override", field["source"])
}

// TestReviewConvergeOnValidWrittenToDisk asserts the set write persists the
// legal value into the task file's workflow block (belt-and-suspenders on the
// resolution test above, independent of config --resolved).
func TestReviewConvergeOnValidWrittenToDisk(t *testing.T) {
	dir := writeStrategyProject(t, "{}")

	_, stderr, code := runTP(t, dir, "set", "--workflow", "review_converge_on=blocking")
	require.Equal(t, 0, code, "blocking is a legal value: %s", stderr)

	data, err := os.ReadFile(filepath.Join(dir, "s.tasks.json"))
	require.NoError(t, err)
	assert.Contains(t, string(data), `"review_converge_on": "blocking"`)
}
