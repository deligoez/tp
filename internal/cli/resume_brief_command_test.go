package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// briefCommandOf pulls next_action.brief_command from a resume result.
func briefCommandOf(t *testing.T, res map[string]any) any {
	t.Helper()
	na := res["next_action"].(map[string]any)
	return na["brief_command"]
}

// TestResume_BriefCommandImplement: §9.3 — the implement phase's brief_command
// is tp next --brief (claims the task and delivers the brief), not the read-only
// tp brief <id>.
func TestResume_BriefCommandImplement(t *testing.T) {
	dir := newPayloadRepo(t, `[{"id":"t1","title":"T","status":"open","depends_on":[],"estimate_minutes":5,"acceptance":"a","source_sections":["x"]}]`)
	assert.Equal(t, "tp next --brief", briefCommandOf(t, resumeResult(t, dir)))
}

// TestResume_BriefCommandReview: §9.3 — the review brief_command names the spec
// and the next round (tp review <spec> --round N).
func TestResume_BriefCommandReview(t *testing.T) {
	dir := newPayloadRepo(t, `[]`)
	assert.Equal(t, "tp review spec.md --round 1", briefCommandOf(t, resumeResult(t, dir)))
}

// TestResume_BriefCommandAudit: §9.3 — the audit brief_command is tp audit <spec>.
func TestResume_BriefCommandAudit(t *testing.T) {
	dir := newPayloadRepo(t, `[{"id":"t1","title":"T","status":"done","depends_on":[],"estimate_minutes":5,"acceptance":"a","source_sections":["x"]}]`)
	assert.Equal(t, "tp audit spec.md", briefCommandOf(t, resumeResult(t, dir)))
}

// TestResume_BriefCommandNullForDecompose: decompose is agent work with no tp
// command, so it carries a null brief_command.
func TestResume_BriefCommandNullForDecompose(t *testing.T) {
	dir := newPayloadRepo(t, `[]`)
	writeConvergedRounds(t, dir, 2, 0)
	assert.Nil(t, briefCommandOf(t, resumeResult(t, dir)))
}

// TestResume_BriefCommandNullForRelease mirrors decompose for the release phase.
func TestResume_BriefCommandNullForRelease(t *testing.T) {
	dir := newPayloadRepo(t, `[{"id":"t1","title":"T","status":"done","depends_on":[],"estimate_minutes":5,"acceptance":"a","source_sections":["x"]}]`)
	writeConvergedRounds(t, dir, 0, 2)
	assert.Nil(t, briefCommandOf(t, resumeResult(t, dir)))
}

// TestResume_BriefCommandSurvivesCompact: §9.3 — brief_command is
// decision-critical, so it survives --compact.
func TestResume_BriefCommandSurvivesCompact(t *testing.T) {
	dir := newPayloadRepo(t, `[]`)
	out, stderr, code := runTP(t, dir, "resume", "--compact")
	require.Equal(t, 0, code, "resume --compact: %s", stderr)
	var res map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &res))
	assert.Equal(t, "tp review spec.md --round 1", briefCommandOf(t, res))
}

// TestResume_BriefCommandReviewInFlight: §10.2 — when a review round is in
// flight, the brief_command still points at that round's prompt emission
// (tp review <spec> --round N), even though the action is to record it.
func TestResume_BriefCommandReviewInFlight(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, ".git"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# Spec\n## 1. A\na\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.tasks.json"),
		[]byte(`{"spec":"spec.md","tasks":[]}`), 0o600))

	_, _, code := runTP(t, dir, "review", "spec.md")
	require.Equal(t, 0, code)

	assert.Equal(t, "tp review spec.md --round 1", briefCommandOf(t, resumeResult(t, dir)))
}
