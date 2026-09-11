package engine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// loopRounds writes one round file per element of files into the spec's state
// dir and returns the rounds referencing them, each stamped with spec hash "h".
func loopRounds(t *testing.T, specPath string, files ...[]string) []ReviewRound {
	t.Helper()
	stateDir := ReviewStateDir(specPath)
	require.NoError(t, os.MkdirAll(stateDir, 0o755))
	rounds := make([]ReviewRound, 0, len(files))
	for i, lines := range files {
		name := "round-" + string(rune('1'+i)) + ".ndjson"
		content := strings.Join(lines, "\n")
		if content != "" {
			content += "\n"
		}
		require.NoError(t, os.WriteFile(filepath.Join(stateDir, name), []byte(content), 0o600))
		rounds = append(rounds, ReviewRound{Round: i + 1, File: name, SpecHash: "h"})
	}
	return rounds
}

const (
	highOpen      = `{"severity":"high","finding":"h"}`
	mediumOpen    = `{"severity":"medium","finding":"m"}`
	highFixed     = `{"severity":"high","finding":"h","resolved":{"status":"fixed","evidence":"spec edited"}}`
	highWontfix   = `{"severity":"high","finding":"h","resolved":{"status":"wontfix","evidence":"accepted by the operator"}}`
	mediumFixed   = `{"severity":"medium","finding":"m","resolved":{"status":"fixed","evidence":"spec edited"}}`
	mediumWontfix = `{"severity":"medium","finding":"m","resolved":{"status":"wontfix","evidence":"out of scope"}}`
	mediumBareWF  = `{"severity":"medium","finding":"m","resolved":{"status":"wontfix","evidence":"  "}}`
)

func TestReviewLoopDone_ConvergedWins(t *testing.T) {
	t.Parallel()
	spec := filepath.Join(t.TempDir(), "spec.md")
	rounds := loopRounds(t, spec, []string{highOpen}, []string{mediumOpen}, []string{})

	d := ReviewLoopDone(spec, rounds, 2, 3, "h", ReviewConvergeOnBlocking)
	assert.True(t, d.Done)
	assert.Equal(t, DoneByConverged, d.By)
}

func TestReviewLoopDone_AtTheCapEveryFindingDispositioned(t *testing.T) {
	t.Parallel()
	spec := filepath.Join(t.TempDir(), "spec.md")
	rounds := loopRounds(t, spec, []string{highOpen}, []string{highOpen}, []string{highWontfix, mediumFixed, mediumWontfix})

	d := ReviewLoopDone(spec, rounds, 2, 3, "changed", ReviewConvergeOnBlocking)
	assert.True(t, d.Done, "at the cap, a fully dispositioned latest round ends the loop")
	assert.Equal(t, DoneByCap, d.By)
	assert.True(t, d.CapReached)
	assert.Equal(t, 1, d.FixedAtCap, "the non-blocking fixed row no round re-read is counted")
	assert.True(t, d.StaleWaived, "the spec changed after the last round and that is reported, not hidden")
	assert.Equal(t, 0, d.Undisposed)
}

// TestReviewLoopDone_ABlockingFixedAtTheCapKeepsItOpen: fixed says the spec
// changed, but at the cap no round will re-read it, so for a blocking finding
// it would be an acceptance nobody made — and a unit may write fixed under
// TP_UNATTENDED. Only a blocking finding accepted with evidence ends the loop.
func TestReviewLoopDone_ABlockingFixedAtTheCapKeepsItOpen(t *testing.T) {
	t.Parallel()
	spec := filepath.Join(t.TempDir(), "spec.md")
	rounds := loopRounds(t, spec, []string{highOpen}, []string{highOpen}, []string{highFixed, mediumWontfix})

	d := ReviewLoopDone(spec, rounds, 2, 3, "h", ReviewConvergeOnBlocking)
	assert.False(t, d.Done, "a blocking finding marked fixed at the cap is not an acceptance")
	assert.True(t, d.CapReached)
	assert.Equal(t, 1, d.BlockingFixedAtCap)
	assert.Equal(t, 0, d.FixedAtCap, "the blocking fixed row is not counted as a waived fixed row")
}

func TestReviewLoopDone_AtTheCapAnOpenFindingKeepsItOpen(t *testing.T) {
	t.Parallel()
	spec := filepath.Join(t.TempDir(), "spec.md")
	rounds := loopRounds(t, spec, []string{highOpen}, []string{highOpen}, []string{highWontfix, mediumOpen})

	d := ReviewLoopDone(spec, rounds, 2, 3, "h", ReviewConvergeOnBlocking)
	assert.False(t, d.Done)
	assert.True(t, d.CapReached)
	assert.Equal(t, 1, d.Undisposed)
}

func TestReviewLoopDone_EmptyEvidenceIsNotADisposition(t *testing.T) {
	t.Parallel()
	spec := filepath.Join(t.TempDir(), "spec.md")
	rounds := loopRounds(t, spec, []string{highOpen}, []string{highOpen}, []string{mediumBareWF})

	d := ReviewLoopDone(spec, rounds, 2, 3, "h", ReviewConvergeOnAll)
	assert.False(t, d.Done, "a wontfix whose evidence is blank clears nothing, so it cannot end the loop")
	assert.Equal(t, 1, d.Undisposed)
}

func TestReviewLoopDone_BelowTheCapAndUncapped(t *testing.T) {
	t.Parallel()
	spec := filepath.Join(t.TempDir(), "spec.md")
	rounds := loopRounds(t, spec, []string{highOpen}, []string{mediumWontfix})

	below := ReviewLoopDone(spec, rounds, 2, 3, "h", ReviewConvergeOnAll)
	assert.False(t, below.Done)
	assert.False(t, below.CapReached)

	uncapped := ReviewLoopDone(spec, rounds, 2, 0, "h", ReviewConvergeOnAll)
	assert.False(t, uncapped.Done)
	assert.False(t, uncapped.CapReached, "a cap of 0 is uncapped and never ends a loop")
}

func TestAuditRoundClean_AnAcceptedFindingClearsTheRound(t *testing.T) {
	t.Parallel()
	spec := filepath.Join(t.TempDir(), "spec.md")
	accepted := loopRounds(t, spec, []string{
		`{"role":"go-safety","item_id":"a","status":"PASS","severity":null}`,
		`{"role":"go-safety","item_id":"b","status":"FAIL","severity":"error","resolved":{"status":"wontfix","evidence":"measured: not reachable"}}`,
	})
	assert.True(t, AuditRoundClean(spec, &accepted[0]),
		"a FAIL accepted wontfix with evidence no longer holds the round open")

	for name, row := range map[string]string{
		"blank evidence": `{"role":"go-safety","item_id":"b","status":"FAIL","severity":"error","resolved":{"status":"wontfix","evidence":""}}`,
		"fixed":          `{"role":"go-safety","item_id":"b","status":"FAIL","severity":"error","resolved":{"status":"fixed","evidence":"code changed"}}`,
		"open":           `{"role":"go-safety","item_id":"b","status":"FAIL","severity":"error"}`,
	} {
		sub := filepath.Join(t.TempDir(), "spec.md")
		r := loopRounds(t, sub, []string{row})
		assert.False(t, AuditRoundClean(sub, &r[0]), name)
	}
}

func TestAuditLoopDone_PassRowsNeedNoDisposition(t *testing.T) {
	t.Parallel()
	spec := filepath.Join(t.TempDir(), "spec.md")
	open := `{"role":"r","item_id":"x","status":"FAIL","severity":"error"}`
	rounds := loopRounds(t, spec, []string{open}, []string{open}, []string{
		`{"role":"r","item_id":"p","status":"PASS","severity":null}`,
		`{"role":"r","item_id":"w","status":"FAIL","severity":"warning","resolved":{"status":"fixed","evidence":"code changed"}}`,
	})
	rounds[2].ConvergeOn = AuditConvergeOnBlocking // a warning does not block under blocking

	d := AuditLoopDone(spec, rounds, 2, 3, "h")
	assert.True(t, d.Done)
	assert.Equal(t, DoneByCap, d.By)
	assert.Equal(t, 1, d.FixedAtCap)
}

// TestAuditLoopDone_ABlockingFixedAtTheCapKeepsItOpen: the audit twin of the
// review rule, graded under the latest round's recorded policy — under `all`
// every non-PASS row blocks, so an error FAIL marked fixed at the cap keeps
// the loop open until the operator accepts it.
func TestAuditLoopDone_ABlockingFixedAtTheCapKeepsItOpen(t *testing.T) {
	t.Parallel()
	spec := filepath.Join(t.TempDir(), "spec.md")
	open := `{"role":"r","item_id":"x","status":"FAIL","severity":"error"}`
	rounds := loopRounds(t, spec, []string{open}, []string{open}, []string{
		`{"role":"r","item_id":"x","status":"FAIL","severity":"error","resolved":{"status":"fixed","evidence":"code changed"}}`,
	})

	d := AuditLoopDone(spec, rounds, 2, 3, "h")
	assert.False(t, d.Done)
	assert.Equal(t, 1, d.BlockingFixedAtCap)
}

func TestLiveAuditRounds_RecomputesTheStoredFlag(t *testing.T) {
	t.Parallel()
	spec := filepath.Join(t.TempDir(), "spec.md")
	rounds := loopRounds(t, spec,
		[]string{`{"role":"r","item_id":"x","status":"FAIL","severity":"error","resolved":{"status":"duplicate","evidence":"same as y"}}`},
		[]string{`{"role":"r","item_id":"y","status":"PASS","severity":null}`},
	)
	// Stored flags say unclean, as they were stamped before the disposition.
	live := LiveAuditRounds(spec, rounds)
	require.Len(t, live, 2)
	assert.True(t, live[0].Clean)
	assert.True(t, live[1].Clean)
	assert.False(t, rounds[0].Clean, "the caller's slice is not written through")
	assert.True(t, Converged(live, 2, "h"))
}

// TestAuditRoundClean_GradedUnderItsOwnPolicy: a round is re-graded under the
// policy it was recorded with, so relaxing audit_converge_on later cannot clean
// a round recorded under `all`, and a disposition in a `blocking` round only has
// to take out the blocking rows.
func TestAuditRoundClean_GradedUnderItsOwnPolicy(t *testing.T) {
	t.Parallel()
	warning := `{"role":"r","item_id":"w","status":"FAIL","severity":"warning"}`
	acceptedError := `{"role":"r","item_id":"e","status":"FAIL","severity":"error","resolved":{"status":"wontfix","evidence":"measured"}}`

	spec := filepath.Join(t.TempDir(), "spec.md")
	rounds := loopRounds(t, spec, []string{warning, acceptedError})

	rounds[0].ConvergeOn = AuditConvergeOnAll
	assert.False(t, AuditRoundClean(spec, &rounds[0]),
		"under `all` the advisory FAIL still holds the round open")

	rounds[0].ConvergeOn = AuditConvergeOnBlocking
	assert.True(t, AuditRoundClean(spec, &rounds[0]),
		"under `blocking` accepting the error row is enough")

	rounds[0].ConvergeOn = ""
	assert.False(t, AuditRoundClean(spec, &rounds[0]),
		"a round recorded before the policy was stored is graded under `all`")

	rounds[0].Clean = true
	assert.True(t, AuditRoundClean(spec, &rounds[0]), "a round stamped clean stays clean")
}
