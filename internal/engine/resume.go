package engine

import (
	"sort"

	"github.com/deligoez/tp/internal/model"
)

// BookkeepingEntry is one tp-owned dirty file in tp resume's bookkeeping array
// (§5.2): uncommitted state tp itself wrote (the task file, .tp-review/, .tp/),
// reported separately from changes and never counted as an unexplained-changes
// blocker. Path is repo-root-relative; Kind is closure/round/config; Ref is a
// task id (closure), a round number (round), or a path basename (config).
type BookkeepingEntry struct {
	Path string `json:"path"`
	Kind string `json:"kind"`
	Ref  string `json:"ref"`
}

// ResumeResult is the resume oracle's output object (§4.2): the lifecycle phase,
// the resolved spec path, the byte-sorted uncommitted changes not on the
// keep-list, the byte-sorted keep-list matches, the tp-owned bookkeeping (§5.2),
// the implement-phase execution-model guidance note, the next action, and the
// blockers.
type ResumeResult struct {
	Phase       string             `json:"phase"`
	Spec        string             `json:"spec"`
	Changes     []string           `json:"changes"`
	Kept        []model.KeepEntry  `json:"kept"`
	Bookkeeping []BookkeepingEntry `json:"bookkeeping"`
	Guidance    string             `json:"guidance,omitempty"`
	NextAction  NextAction         `json:"next_action"`
	Blockers    []Blocker          `json:"blockers"`
}

// AssembleResume builds the read-only resume result from durable state alone:
// the task file (tf, empty when its file is absent) and its resolved spec, the
// review/audit rounds in .tp-review/, the keep-list in .tp/local.json, and git —
// all discovered from start for the working-tree classification. It performs no
// write (§4.8). A corrupt review-state directory is surfaced as an error for the
// caller to abort on; a missing spec or non-git tree degrades gracefully.
func AssembleResume(start, taskFilePath, specPath string, tf *model.TaskFile) (ResumeResult, error) {
	// Working-tree classification against the keep-list (§4.5). A malformed keep
	// pattern leaves every change unexplained rather than silently swallowing it.
	raw := WorktreeChanges(start)
	changes := raw
	kept := []model.KeepEntry{}
	if classified, err := ClassifyPaths(LoadKeepList(start), raw); err == nil {
		changes = classified.Changes
		kept = classified.Kept
	}
	// §5.2: tp-owned dirty state (the task file, .tp-review/, .tp/) is
	// bookkeeping — reported separately from changes and never counted as an
	// unexplained-changes blocker. Classified after the keep-list so an
	// explicitly keep-listed tp-owned file stays in kept and is not
	// double-reported.
	bookkeeping, remaining := DeriveBookkeeping(start, taskFilePath, specPath, changes, tf)
	changes = remaining
	sort.Strings(changes)
	sort.Slice(kept, func(i, j int) bool { return kept[i].Path < kept[j].Path })

	// Convergence and staleness from the review state and the effective workflow.
	st, err := LoadReviewState(specPath)
	if err != nil {
		return ResumeResult{}, err
	}
	wf := EffectiveWorkflowForTaskFile(taskFilePath)
	specHash, _ := SpecHash(specPath)
	reviewRounds := reviewRoundsOf(st)
	auditRounds := auditRoundsOf(st)
	reviewConverged := Converged(reviewRounds, wf.ReviewCleanRounds, specHash)
	auditConverged := Converged(auditRounds, wf.AuditCleanRounds, specHash)
	reviewStale := StateStale(reviewRounds, specHash)

	numDone := 0
	for i := range tf.Tasks {
		if tf.Tasks[i].Status == model.StatusDone {
			numDone++
		}
	}
	phase := DetectPhase(len(tf.Tasks), numDone, reviewConverged, auditConverged)

	guidance := ""
	if phase == PhaseImplement {
		guidance = "run each task in a fresh subagent/context; keep the orchestrator context clean"
	}

	blockers := BuildBlockers(&BlockerInputs{
		Phase:           phase,
		SpecPath:        specPath,
		Changes:         changes,
		TaskFile:        tf,
		ReviewRounds:    len(reviewRounds),
		ReviewMaxRounds: wf.EffectiveReviewMaxRounds(),
		ReviewConverged: reviewConverged,
		AuditRounds:     len(auditRounds),
		AuditMaxRounds:  wf.EffectiveAuditMaxRounds(),
		AuditConverged:  auditConverged,
		ReviewStale:     reviewStale,
	})

	return ResumeResult{
		Phase:       phase,
		Spec:        specPath,
		Changes:     changes,
		Kept:        kept,
		Bookkeeping: bookkeeping,
		Guidance:    guidance,
		NextAction:  BuildNextAction(phase, specPath, tf, st),
		Blockers:    blockers,
	}, nil
}
