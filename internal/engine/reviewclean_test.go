package engine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeRoundForClean writes a round findings file into the state dir and
// returns a ReviewRound referencing it (stored Clean left false).
func writeRoundForClean(t *testing.T, specPath, name string, lines ...string) *ReviewRound {
	t.Helper()
	stateDir := ReviewStateDir(specPath)
	require.NoError(t, os.MkdirAll(stateDir, 0o755))
	content := strings.Join(lines, "\n")
	if content != "" {
		content += "\n"
	}
	require.NoError(t, os.WriteFile(filepath.Join(stateDir, name), []byte(content), 0o600))
	return &ReviewRound{Round: 1, File: name}
}

func TestReviewRoundClean_BlockingVsAll(t *testing.T) {
	dir := t.TempDir()
	specPath := filepath.Join(dir, "spec.md")

	// Only medium + low survive.
	r := writeRoundForClean(t, specPath, "r1.ndjson",
		`{"severity":"medium","finding":"m"}`,
		`{"severity":"low","finding":"l"}`)

	assert.True(t, ReviewRoundClean(specPath, r, ReviewConvergeOnBlocking),
		"medium/low survivors are clean under blocking")
	assert.False(t, ReviewRoundClean(specPath, r, ReviewConvergeOnAll),
		"any survivor is unclean under all")

	// A surviving high finding is unclean under both.
	r2 := writeRoundForClean(t, specPath, "r2.ndjson",
		`{"severity":"medium","finding":"m"}`,
		`{"severity":"high","finding":"h"}`)
	assert.False(t, ReviewRoundClean(specPath, r2, ReviewConvergeOnBlocking),
		"a surviving high finding blocks under blocking")
	assert.False(t, ReviewRoundClean(specPath, r2, ReviewConvergeOnAll))

	// A surviving critical finding likewise blocks under blocking.
	r3 := writeRoundForClean(t, specPath, "r3.ndjson",
		`{"severity":"critical","finding":"c"}`)
	assert.False(t, ReviewRoundClean(specPath, r3, ReviewConvergeOnBlocking))
}

func TestReviewRoundClean_CaseInsensitiveSeverity(t *testing.T) {
	dir := t.TempDir()
	specPath := filepath.Join(dir, "spec.md")

	// Upper/mixed-case blocking severities still block.
	r := writeRoundForClean(t, specPath, "r.ndjson",
		`{"severity":"HIGH","finding":"h"}`)
	assert.False(t, ReviewRoundClean(specPath, r, ReviewConvergeOnBlocking),
		"HIGH normalizes to high (blocking)")

	// Upper/mixed-case non-blocking severities stay non-blocking.
	r2 := writeRoundForClean(t, specPath, "r2.ndjson",
		`{"severity":"Medium","finding":"m"}`,
		`{"severity":" LOW ","finding":"l"}`)
	assert.True(t, ReviewRoundClean(specPath, r2, ReviewConvergeOnBlocking),
		"Medium/LOW normalize to non-blocking")
}

func TestReviewRoundClean_MissingOrUnknownSeverityBlocks(t *testing.T) {
	dir := t.TempDir()
	specPath := filepath.Join(dir, "spec.md")

	missing := writeRoundForClean(t, specPath, "missing.ndjson",
		`{"finding":"no severity field"}`)
	assert.False(t, ReviewRoundClean(specPath, missing, ReviewConvergeOnBlocking),
		"a missing severity is treated as blocking")

	unknown := writeRoundForClean(t, specPath, "unknown.ndjson",
		`{"severity":"bogus","finding":"out of vocab"}`)
	assert.False(t, ReviewRoundClean(specPath, unknown, ReviewConvergeOnBlocking),
		"an out-of-vocabulary severity is treated as blocking")
}

func TestReviewRoundClean_SeverityLessRoundCleanIffZeroSurvive(t *testing.T) {
	dir := t.TempDir()
	specPath := filepath.Join(dir, "spec.md")

	// A round whose only rows are resolved (wontfix/duplicate with evidence),
	// none carrying a severity: zero survive -> clean under both policies.
	zero := writeRoundForClean(t, specPath, "zero.ndjson",
		`{"finding":"a","resolved":{"status":"wontfix","evidence":"verifier: false positive"}}`,
		`{"finding":"b","resolved":{"status":"duplicate","evidence":"dup of a"}}`)
	assert.True(t, ReviewRoundClean(specPath, zero, ReviewConvergeOnBlocking))
	assert.True(t, ReviewRoundClean(specPath, zero, ReviewConvergeOnAll))

	// One severity-less survivor -> blocking, so unclean under both.
	one := writeRoundForClean(t, specPath, "one.ndjson",
		`{"finding":"survivor with no severity"}`)
	assert.False(t, ReviewRoundClean(specPath, one, ReviewConvergeOnBlocking))
	assert.False(t, ReviewRoundClean(specPath, one, ReviewConvergeOnAll))
}

func TestReviewRoundClean_SurvivingSetExemptsResolved(t *testing.T) {
	dir := t.TempDir()
	specPath := filepath.Join(dir, "spec.md")

	// wontfix / duplicate with evidence are out of the surviving set even when
	// carrying a blocking severity; the round is clean under blocking.
	exempt := writeRoundForClean(t, specPath, "exempt.ndjson",
		`{"severity":"high","finding":"a","resolved":{"status":"wontfix","evidence":"e"}}`,
		`{"severity":"critical","finding":"b","resolved":{"status":"duplicate","evidence":"e"}}`)
	assert.True(t, ReviewRoundClean(specPath, exempt, ReviewConvergeOnBlocking),
		"resolved wontfix/duplicate (with evidence) leave the surviving set")

	// A resolution without evidence does NOT exempt the finding.
	noEvidence := writeRoundForClean(t, specPath, "noev.ndjson",
		`{"severity":"high","finding":"a","resolved":{"status":"wontfix","evidence":"  "}}`)
	assert.False(t, ReviewRoundClean(specPath, noEvidence, ReviewConvergeOnBlocking),
		"a resolution without evidence keeps the finding in the surviving set")

	// A "fixed" resolution is not an exemption either.
	fixed := writeRoundForClean(t, specPath, "fixed.ndjson",
		`{"severity":"high","finding":"a","resolved":{"status":"fixed","evidence":"e"}}`)
	assert.False(t, ReviewRoundClean(specPath, fixed, ReviewConvergeOnBlocking))
}

func TestReviewRoundClean_MissingFileFallsBackToStoredFlag(t *testing.T) {
	dir := t.TempDir()
	specPath := filepath.Join(dir, "spec.md")

	clean := &ReviewRound{Round: 1, File: "gone.ndjson", Clean: true}
	assert.True(t, ReviewRoundClean(specPath, clean, ReviewConvergeOnBlocking),
		"a missing round file falls back to the stored clean flag")
	dirty := &ReviewRound{Round: 2, File: "gone2.ndjson", Clean: false}
	assert.False(t, ReviewRoundClean(specPath, dirty, ReviewConvergeOnBlocking))
}

func TestReviewRoundNonBlockingOpen(t *testing.T) {
	dir := t.TempDir()
	specPath := filepath.Join(dir, "spec.md")

	// Clean-under-blocking round with two medium/low survivors -> count 2.
	acceptedOpen := writeRoundForClean(t, specPath, "accepted.ndjson",
		`{"severity":"medium","finding":"m"}`,
		`{"severity":"low","finding":"l"}`)
	assert.Equal(t, 2, ReviewRoundNonBlockingOpen(specPath, acceptedOpen, ReviewConvergeOnBlocking),
		"both medium and low survivors are counted under blocking")
	assert.Equal(t, 0, ReviewRoundNonBlockingOpen(specPath, acceptedOpen, ReviewConvergeOnAll),
		"nothing is non-blocking under all")

	// A blocking survivor is not counted; only the medium survivor is.
	mixed := writeRoundForClean(t, specPath, "mixed.ndjson",
		`{"severity":"high","finding":"h"}`,
		`{"severity":"medium","finding":"m"}`)
	assert.Equal(t, 1, ReviewRoundNonBlockingOpen(specPath, mixed, ReviewConvergeOnBlocking),
		"only the medium survivor counts; the high one does not")

	// Missing / out-of-vocabulary severities are blocking, never counted.
	badSev := writeRoundForClean(t, specPath, "badsev.ndjson",
		`{"finding":"no severity"}`,
		`{"severity":"bogus","finding":"out of vocab"}`)
	assert.Equal(t, 0, ReviewRoundNonBlockingOpen(specPath, badSev, ReviewConvergeOnBlocking),
		"missing/out-of-vocab severities are blocking, not counted as non-blocking")

	// Resolved wontfix/duplicate (with evidence) leave the surviving set even
	// when medium/low, so they are not counted.
	resolved := writeRoundForClean(t, specPath, "resolved.ndjson",
		`{"severity":"medium","finding":"a","resolved":{"status":"wontfix","evidence":"e"}}`,
		`{"severity":"low","finding":"b","resolved":{"status":"duplicate","evidence":"e"}}`)
	assert.Equal(t, 0, ReviewRoundNonBlockingOpen(specPath, resolved, ReviewConvergeOnBlocking),
		"resolved-away non-blocking findings are not counted")

	// A missing round file yields 0.
	missing := &ReviewRound{Round: 1, File: "gone.ndjson"}
	assert.Equal(t, 0, ReviewRoundNonBlockingOpen(specPath, missing, ReviewConvergeOnBlocking),
		"a missing round file yields 0")
}

func TestReviewConsecutiveCleanAndConverged_SeverityAware(t *testing.T) {
	dir := t.TempDir()
	specPath := filepath.Join(dir, "spec.md")

	// Round 1: surviving high (blocking); rounds 2 and 3: only medium/low.
	writeRoundForClean(t, specPath, "review-round-1.ndjson", `{"severity":"high","finding":"h"}`)
	writeRoundForClean(t, specPath, "review-round-2.ndjson", `{"severity":"medium","finding":"m"}`)
	writeRoundForClean(t, specPath, "review-round-3.ndjson", `{"severity":"low","finding":"l"}`)
	rounds := []ReviewRound{
		{Round: 1, File: "review-round-1.ndjson", SpecHash: "sha256:z"},
		{Round: 2, File: "review-round-2.ndjson", SpecHash: "sha256:z"},
		{Round: 3, File: "review-round-3.ndjson", SpecHash: "sha256:z"},
	}

	assert.Equal(t, 2, ReviewConsecutiveClean(specPath, rounds, ReviewConvergeOnBlocking),
		"rounds 2 and 3 are clean under blocking; round 1 blocks")
	assert.Equal(t, 0, ReviewConsecutiveClean(specPath, rounds, ReviewConvergeOnAll),
		"every round has a survivor, so none are clean under all")

	assert.True(t, ReviewConverged(specPath, rounds, 2, "sha256:z", ReviewConvergeOnBlocking))
	assert.False(t, ReviewConverged(specPath, rounds, 2, "sha256:z", ReviewConvergeOnAll))
	assert.False(t, ReviewConverged(specPath, rounds, 2, "sha256:edited", ReviewConvergeOnBlocking),
		"a stale spec unconverges regardless of severity")
}
