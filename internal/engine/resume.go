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
// the implement-phase execution-model guidance note, the units to execute now
// and the round they belong to (§4.1), the next action, and the blockers.
//
// Phase, NextUnits and Round are the whole machine surface: an unattended driver
// parses those three and nothing else. Everything beside them — NextAction
// included — is for a reader.
type ResumeResult struct {
	Phase       string             `json:"phase"`
	Spec        string             `json:"spec"`
	Changes     []string           `json:"changes"`
	Kept        []model.KeepEntry  `json:"kept"`
	Bookkeeping []BookkeepingEntry `json:"bookkeeping"`
	Guidance    string             `json:"guidance,omitempty"`
	NextUnits   []NextUnit         `json:"next_units"`
	Round       *int               `json:"round"`
	LastFailure *LastFailure       `json:"last_failure"`
	NextAction  NextAction         `json:"next_action"`
	Blockers    []Blocker          `json:"blockers"`
	// EpicNotice is the epic block's notice (EpicBlock.Notice) of an implement
	// phase: the file is bound to an epic the graph does not hold, or there is
	// no graph file, so it behaves as unbound. It is not part of the payload;
	// tp resume says it on stderr, as tp next, ready and claim do.
	EpicNotice string `json:"-"`
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

	loops, err := readResumeLoops(taskFilePath, specPath, tf)
	if err != nil {
		return ResumeResult{}, err
	}
	phase, wf := loops.phase, &loops.wf

	guidance := ""
	if phase == PhaseImplement {
		guidance = "run each task in a fresh subagent/context; keep the orchestrator context clean"
	}

	blockers, epicNotice, err := resumeBlockers(taskFilePath, specPath, changes, tf, &loops)
	if err != nil {
		return ResumeResult{}, err
	}

	// The units come after the blockers because an escalate blocker is what
	// empties them (§4.1), and next_action is rendered from the first unit so
	// the human summary and the machine surface describe the same work.
	nextUnits, round := BuildNextUnits(start, taskFilePath, specPath, phase, tf, loops.st, blockers)

	return ResumeResult{
		Phase:       phase,
		Spec:        specPath,
		Changes:     changes,
		Kept:        kept,
		Bookkeeping: bookkeeping,
		Guidance:    guidance,
		NextUnits:   nextUnits,
		Round:       round,
		LastFailure: ReadLastFailure(start, taskFilePath),
		// len(wf.Checks) is read, never run: the decompose action names the
		// gate that runs the registered checks (decomposeNextAction), and
		// resume itself stays the read-only oracle §4.8 requires.
		NextAction: renderNextAction(BuildNextAction(phase, specPath, tf, loops.st, len(wf.Checks) > 0), nextUnits, blockers),
		Blockers:   blockers,
		EpicNotice: epicNotice,
	}, nil
}

// resumeLoops is what tp resume reads a task file's phase from: its review
// state, effective workflow, recorded rounds and the loop verdicts on them.
type resumeLoops struct {
	st           *ReviewState
	wf           model.Workflow
	reviewRounds []ReviewRound
	auditRounds  []ReviewRound
	reviewDone   LoopDone
	auditDone    LoopDone
	reviewStale  bool
	phase        string
}

// resumeBlockers derives tp resume's blockers from the loops read for a task
// file, and the epic block's notice. Only the implement phase reads the epic
// graph, since a blocked epic stops starting a task and nothing else
// (spec/1.5.0.md §6); an unbound file reads nothing. A graph or bound file that
// cannot be read is an error.
func resumeBlockers(taskFilePath, specPath string, changes []string, tf *model.TaskFile, loops *resumeLoops) ([]Blocker, string, error) {
	var epicBlock *EpicBlock
	notice := ""
	if loops.phase == PhaseImplement {
		block, err := EpicBlockFor(tf, taskFilePath)
		if err != nil {
			return nil, "", err
		}
		epicBlock = block
		if block != nil {
			notice = block.Notice
		}
	}
	wf := &loops.wf
	return BuildBlockers(&BlockerInputs{
		Phase:           loops.phase,
		SpecPath:        specPath,
		Changes:         changes,
		TaskFile:        tf,
		ReviewRounds:    len(loops.reviewRounds),
		ReviewMaxRounds: wf.EffectiveReviewMaxRounds(),
		ReviewConverged: loops.reviewDone.Done,
		AuditRounds:     len(loops.auditRounds),
		AuditMaxRounds:  wf.EffectiveAuditMaxRounds(),
		AuditConverged:  loops.auditDone.Done,
		ReviewStale:     loops.reviewStale,
		EpicBlock:       epicBlock,

		ReviewBlockingFixedAtCap: loops.reviewDone.BlockingFixedAtCap,
		AuditBlockingFixedAtCap:  loops.auditDone.BlockingFixedAtCap,
	}), notice, nil
}

// readResumeLoops derives a task file's lifecycle phase the one way tp resume
// does, so a caller judging a file elsewhere (an epic's bound files) cannot
// disagree with the oracle about it. It writes nothing. A state index the next
// record rebuilds reads as no state; any other corrupt state is an error.
func readResumeLoops(taskFilePath, specPath string, tf *model.TaskFile) (resumeLoops, error) {
	st, err := LoadReviewState(specPath)
	if err != nil {
		// tp resume is the re-orientation oracle, and the window it has to work
		// in is exactly the gap between emitting a round and recording it —
		// where an audit emission has written a snapshot and no index yet.
		// Failing there told the orchestrator to delete the directory holding
		// its own in-flight round. A lost index still surfaces as an error.
		if !IsRebuildableStateIndex(err) {
			return resumeLoops{}, err
		}
		st = nil
	}
	l := resumeLoops{st: st, wf: EffectiveWorkflowForTaskFile(taskFilePath), reviewRounds: reviewRoundsOf(st), auditRounds: auditRoundsOf(st)}
	specHash, _ := SpecHash(specPath)
	// The same verdict import and --status --check read: converged, or the cap
	// reached with every finding of the latest round dispositioned. Reading the
	// stored flags here kept the oracle escalating on a loop import accepted.
	l.reviewDone = ReviewLoopDone(specPath, l.reviewRounds, l.wf.ReviewCleanRounds, l.wf.EffectiveReviewMaxRounds(), specHash, l.wf.ReviewConvergeOn)
	l.auditDone = AuditLoopDone(specPath, l.auditRounds, l.wf.AuditCleanRounds, l.wf.EffectiveAuditMaxRounds(), specHash)
	l.reviewStale = StateStale(l.reviewRounds, specHash)
	numDone := 0
	for i := range tf.Tasks {
		if tf.Tasks[i].Status == model.StatusDone {
			numDone++
		}
	}
	l.phase = DetectPhase(len(tf.Tasks), numDone, l.reviewDone.Done, l.auditDone.Done)
	return l, nil
}
