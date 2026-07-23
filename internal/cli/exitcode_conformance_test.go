package cli_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// §13.1 row 1: invalid-JSON tp add is a usage error (exit 2); the decoder
// detail lives in hint, never in error. (Decode happens before task-file
// discovery, so no task file is required.)
func TestExitCode_AddInvalidJSON_Exit2_DecoderInHint(t *testing.T) {
	dir := t.TempDir()
	_, stderr, code := runTP(t, dir, "add", `{not valid json`)
	e := errJSON(t, stderr)
	assert.Equal(t, 2, code)
	assert.Equal(t, float64(2), e["code"], "invalid JSON must be usage (code 2)")
	assert.Equal(t, "invalid task JSON", e["error"], "decoder detail must not leak into error")
	require.NotEmpty(t, e["hint"], "decoder detail goes in hint")
}

// §13.1 row 2: any cobra flag-parse failure exits 2 as the tp error object
// {error, code, hint}, not exit 1 with bare cobra text.
func TestExitCode_FlagParseFailure_Exit2_TPErrorObject(t *testing.T) {
	dir := t.TempDir()
	_, stderr, code := runTP(t, dir, "--nope")
	e := errJSON(t, stderr)
	assert.Equal(t, 2, code)
	assert.Equal(t, float64(2), e["code"], "flag-parse failure must be usage (code 2)")
	assert.NotEmpty(t, e["error"], "error object must carry a message")
	assert.NotEmpty(t, e["hint"], "error object must carry a hint")
}

// §13.1 row 2 (subcommand): an unknown flag on a subcommand also exits 2 with a
// tp error object.
func TestExitCode_SubcommandFlagParseFailure_Exit2(t *testing.T) {
	dir := t.TempDir()
	_, stderr, code := runTP(t, dir, "list", "--bogus")
	e := errJSON(t, stderr)
	assert.Equal(t, 2, code)
	assert.Equal(t, float64(2), e["code"])
	assert.NotEmpty(t, e["hint"])
}

// §13.1 row 3: a done reason starting with '-' is misread as a flag; the error
// must exit 2 with a hint naming the '--' separator.
func TestExitCode_DoneDashReason_Exit2_HintNamesSeparator(t *testing.T) {
	dir := t.TempDir()
	// Flag parsing fails before runDone runs, so no task file is required.
	_, stderr, code := runTP(t, dir, "done", "some-task", "- evidence here")
	e := errJSON(t, stderr)
	assert.Equal(t, 2, code)
	assert.Equal(t, float64(2), e["code"])
	hint, ok := e["hint"].(string)
	require.True(t, ok, "hint must be present")
	assert.Contains(t, hint, "--", "hint must name the '--' separator")
}

// §13.1 row 3 with a leading flag: even when a real flag precedes it, the
// dash-leading reason is detected and the hint still names '--'.
func TestExitCode_DoneDashReason_AfterCommitFlag(t *testing.T) {
	dir := t.TempDir()
	_, stderr, code := runTP(t, dir, "done", "some-task", "--commit", "abc123", "- dash reason")
	e := errJSON(t, stderr)
	assert.Equal(t, 2, code)
	hint, ok := e["hint"].(string)
	require.True(t, ok)
	assert.Contains(t, hint, "--")
}

// §13.1 row 3 (commit variant): tp commit also takes a trailing reason.
func TestExitCode_CommitDashReason_Exit2_HintNamesSeparator(t *testing.T) {
	dir := t.TempDir()
	_, stderr, code := runTP(t, dir, "commit", "some-task", "- leading dash reason")
	e := errJSON(t, stderr)
	assert.Equal(t, 2, code)
	hint, ok := e["hint"].(string)
	require.True(t, ok)
	assert.Contains(t, hint, "--")
}

// §13.2: every non-zero error object carries a hint naming a next command. A
// state error that historically emitted no hint (tp show on a missing id) now
// carries one via the centralized default.
func TestExitCode_StateErrorCarriesHint_ShowMissingID(t *testing.T) {
	dir := initEntryProject(t)
	_, stderr, code := runTP(t, dir, "show", "does-not-exist")
	e := errJSON(t, stderr)
	assert.Equal(t, 4, code)
	assert.Equal(t, float64(4), e["code"])
	assert.NotEmpty(t, e["hint"], "non-zero error object must carry a hint (§13.2)")
}

// §13.2: a usage error with no explicit hint still carries a default hint.
func TestExitCode_UsageErrorCarriesHint_DoneNoArgs(t *testing.T) {
	dir := t.TempDir()
	_, stderr, code := runTP(t, dir, "done")
	e := errJSON(t, stderr)
	assert.Equal(t, 2, code)
	assert.NotEmpty(t, e["hint"], "usage error must carry a hint (§13.2)")
}
