package engine

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestTaskExport_EachReviewRoundCarriesItsFindings is spec/1.8.0.md §13's
// History at the export: each recorded review round carries its findings as
// its round file holds them, each with its severity, role, class, location,
// text and disposition, so the Spec tab draws them beside the round's
// snapshot difference. A round whose file is gone carries none; an audit
// round carries none.
func TestTaskExport_EachReviewRoundCarriesItsFindings(t *testing.T) {
	dir, specPath, tfp := setupResumeProject(t, exportTasksJSON)
	require.NoError(t, os.WriteFile(specPath, []byte(exportSpec), 0o600))
	exportLoopsFixture(t, specPath)
	st, err := LoadReviewState(specPath)
	require.NoError(t, err)
	st.ReviewRounds[0].File = "review-round-1.ndjson"
	st.ReviewRounds[1].File = "review-round-2.ndjson"
	require.NoError(t, SaveReviewState(specPath, st))
	writeStateFile(t, specPath, "review-round-1.ndjson",
		`{"severity":"high","role":"tester","class":"untestable","location":"§2","finding":"Two readings pass.","evidence":"x","resolved":{"status":"fixed","evidence":"narrowed"}}`+"\n"+
			`{"severity":"low","role":"architect","location":"§1","finding":"A word is loose.","evidence":"y"}`+"\n")

	exp, err := TaskExportOf(dir, tfp)
	require.NoError(t, err)
	rounds := exp.Loops.Review.Rounds
	require.Len(t, rounds, 2)
	assert.Equal(t, []ExportFinding{
		{Severity: "high", Role: "tester", Class: "untestable", Location: "§2", Finding: "Two readings pass.", Disposition: "fixed"},
		{Severity: "low", Role: "architect", Location: "§1", Finding: "A word is loose."},
	}, rounds[0].FindingRows)
	assert.Nil(t, rounds[1].FindingRows, "a round whose file is gone carries no findings")
	assert.Nil(t, exp.Loops.Audit.Rounds[0].FindingRows, "an audit round carries none")
}
