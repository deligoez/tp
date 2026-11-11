package engine

import (
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ExportLoops is a task file's three loops as tp export reports them.
type ExportLoops struct {
	Ground ExportLoop `json:"ground"`
	Review ExportLoop `json:"review"`
	Audit  ExportLoop `json:"audit"`
}

// ExportLoop is one loop: its round cap (null when uncapped; ground has none),
// the clean rounds it needs, its trailing run of clean rounds, and every
// recorded round in order. Ground needs one clean round: its --check reads
// the latest round alone.
type ExportLoop struct {
	Cap               *int          `json:"cap"`
	CleanRoundsNeeded int           `json:"clean_rounds_needed"`
	Streak            int           `json:"streak"`
	Rounds            []ExportRound `json:"rounds"`
}

// ExportRound is one recorded round: when it was recorded, its finding count
// (review, audit) or FAIL count (ground), whether it is clean as the loop
// grades it now, and the difference of its spec snapshot from the round
// before's, null for round 1 or when either snapshot is absent. A ground round
// records no time, so its RecordedAt is its round file's modification time.
// A review round also carries its findings as its round file holds them
// (spec/1.8.0.md §13), absent when the file cannot be read.
type ExportRound struct {
	Round        int                 `json:"round"`
	RecordedAt   string              `json:"recorded_at"`
	Findings     *int                `json:"findings,omitempty"`
	Fails        *int                `json:"fails,omitempty"`
	Clean        bool                `json:"clean"`
	SnapshotDiff *ExportSnapshotDiff `json:"snapshot_diff"`
	FindingRows  []ExportFinding     `json:"finding_rows,omitempty"`
}

// ExportFinding is one finding of a recorded review round: its severity,
// role, class, location and text, and its disposition when it carries one.
type ExportFinding struct {
	Severity    string `json:"severity"`
	Role        string `json:"role,omitempty"`
	Class       string `json:"class,omitempty"`
	Location    string `json:"location"`
	Finding     string `json:"finding"`
	Disposition string `json:"disposition,omitempty"`
}

// ExportSnapshotDiff is the section-level difference of two spec snapshots:
// the sections changed or added, with their text, and those removed.
type ExportSnapshotDiff struct {
	Changed []DiffSection `json:"changed"`
	Removed []DiffSection `json:"removed"`
}

// exportLoops reads the three loops of the task file's spec, graded under
// the file's effective workflow as tp resume grades them.
func exportLoops(taskFilePath, specPath string) (ExportLoops, error) {
	st, err := LoadReviewState(specPath)
	if err != nil {
		if !IsRebuildableStateIndex(err) {
			return ExportLoops{}, err
		}
		st = nil
	}
	wf := EffectiveWorkflowForTaskFile(taskFilePath)
	ground, err := exportGroundLoop(specPath)
	if err != nil {
		return ExportLoops{}, err
	}
	review := reviewRoundsOf(st)
	audit := LiveAuditRounds(specPath, auditRoundsOf(st))
	reviewClean := make([]bool, len(review))
	for i := range review {
		reviewClean[i] = ReviewRoundClean(specPath, &review[i], wf.ReviewConvergeOn)
	}
	auditClean := make([]bool, len(audit))
	for i := range audit {
		auditClean[i] = audit[i].Clean
	}
	return ExportLoops{
		Ground: ground,
		Review: specExportLoop(specPath, PhaseReview, review, reviewClean, wf.EffectiveReviewMaxRounds(), wf.ReviewCleanRounds),
		Audit:  specExportLoop(specPath, PhaseAudit, audit, auditClean, wf.EffectiveAuditMaxRounds(), wf.AuditCleanRounds),
	}, nil
}

// specExportLoop is a review or audit loop from its recorded rounds and each
// round's clean verdict.
func specExportLoop(specPath, phase string, rounds []ReviewRound, clean []bool, maxRounds, cleanNeeded int) ExportLoop {
	l := ExportLoop{CleanRoundsNeeded: cleanNeeded, Streak: trailingTrue(clean), Rounds: make([]ExportRound, 0, len(rounds))}
	if maxRounds > 0 {
		l.Cap = &maxRounds
	}
	for i := range rounds {
		findings := rounds[i].Findings
		round := ExportRound{Round: rounds[i].Round, RecordedAt: rounds[i].RecordedAt, Findings: &findings,
			Clean: clean[i], SnapshotDiff: exportSnapshotDiff(specPath, phase, rounds[i].Round)}
		if phase == PhaseReview {
			round.FindingRows = exportFindingRows(specPath, rounds[i].File)
		}
		l.Rounds = append(l.Rounds, round)
	}
	return l
}

// exportFindingRows reads a review round file's findings, nil when it has
// no file or the file cannot be read.
func exportFindingRows(specPath, file string) []ExportFinding {
	if file == "" {
		return nil
	}
	rows, ok := readNDJSONRows(filepath.Join(ReviewStateDir(specPath), file))
	if !ok {
		return nil
	}
	str := func(row map[string]any, key string) string { s, _ := row[key].(string); return s }
	out := make([]ExportFinding, 0, len(rows))
	for _, row := range rows {
		f := ExportFinding{Severity: str(row, "severity"), Role: str(row, "role"), Class: str(row, "class"),
			Location: str(row, "location"), Finding: str(row, "finding")}
		if resolved, isMap := row["resolved"].(map[string]any); isMap {
			f.Disposition = str(resolved, "status")
		}
		out = append(out, f)
	}
	return out
}

// exportGroundLoop reads every recorded ground round: its FAIL rows, and
// clean when it holds none and every emitted unit is dispositioned.
func exportGroundLoop(specPath string) (ExportLoop, error) {
	l := ExportLoop{CleanRoundsNeeded: 1, Rounds: []ExportRound{}}
	next, err := NextGroundRound(specPath)
	if err != nil {
		return ExportLoop{}, err
	}
	clean := []bool{}
	for r := 1; r < next; r++ {
		info, statErr := os.Stat(GroundRoundPath(specPath, r))
		if statErr != nil {
			continue
		}
		round, err := exportGroundRound(specPath, r)
		if err != nil {
			return ExportLoop{}, err
		}
		round.RecordedAt = info.ModTime().UTC().Format(time.RFC3339)
		l.Rounds = append(l.Rounds, round)
		clean = append(clean, round.Clean)
	}
	l.Streak = trailingTrue(clean)
	return l, nil
}

// exportGroundRound reads one recorded ground round over the floor its
// emission froze.
func exportGroundRound(specPath string, round int) (ExportRound, error) {
	floor, err := readGroundFloor(specPath, round)
	if err != nil {
		return ExportRound{}, err
	}
	rows, err := readGroundRoundRows(specPath, round)
	if err != nil {
		return ExportRound{}, err
	}
	fails := groundVerdictCounts(rows)[VerdictFail]
	cov := GroundCoverageOf(floor, rows)
	return ExportRound{Round: round, Fails: &fails, Clean: fails == 0 && cov.Dispositioned == cov.Emitted,
		SnapshotDiff: exportSnapshotDiff(specPath, groundSnapshotPhase, round)}, nil
}

// exportSnapshotDiff is the difference of round's snapshot of phase from the
// round before's, nil for round 1 or when either snapshot cannot be read.
func exportSnapshotDiff(specPath, phase string, round int) *ExportSnapshotDiff {
	if round < 2 {
		return nil
	}
	dir := ReviewStateDir(specPath)
	before, err := os.ReadFile(filepath.Join(dir, snapshotFilename(phase, round-1))) //nolint:gosec // the spec's own state dir
	if err != nil {
		return nil
	}
	after, err := os.ReadFile(filepath.Join(dir, snapshotFilename(phase, round))) //nolint:gosec // the spec's own state dir
	if err != nil {
		return nil
	}
	d := DiffSections(strings.Split(string(before), "\n"), strings.Split(string(after), "\n"))
	out := &ExportSnapshotDiff{Changed: d.Changed, Removed: d.Removed}
	if out.Changed == nil {
		out.Changed = []DiffSection{}
	}
	if out.Removed == nil {
		out.Removed = []DiffSection{}
	}
	return out
}

// trailingTrue is the length of the trailing run of true values.
func trailingTrue(vs []bool) int {
	n := 0
	for i := len(vs) - 1; i >= 0 && vs[i]; i-- {
		n++
	}
	return n
}
