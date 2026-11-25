package engine

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

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
	// PendingCloses names the tasks whose pooled unit asked to close and
	// no run has integrated yet: a crash leaves them, and the next tp run
	// integrates them first (spec/backlog/parallel-implement.md §2.6).
	PendingCloses []string `json:"pending_closes,omitempty"`
	// UnitBranches names the branches an unfinished pooled unit left: its
	// work, kept until its task is done (spec/backlog/parallel-implement.md).
	UnitBranches []string `json:"unit_branches,omitempty"`
	// RunningUnits names the implement units a live run's rolling pool runs
	// now, left out of NextUnits (spec/backlog/rolling-implement.md §2).
	RunningUnits []string `json:"running_units,omitempty"`
	// FoundQueue counts the spec's open queue entries and the tasks they
	// hold (spec/1.31.0.md §6); nil when none is open. It moves no phase.
	FoundQueue *FoundQueueCount `json:"found_queue,omitempty"`
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
	return AssembleResumeRunning(start, taskFilePath, specPath, tf, nil)
}

// AssembleResumeLive is AssembleResume for a reader outside any run: tp
// resume, tp export and the dashboard. Beside a live run's rolling pool it
// lists no unit, even for a free slot, and its next action takes no task: the
// pool starts what comes next itself, and a reader that took a unit would run
// a task the pool runs, or is about to (spec/backlog/rolling-implement.md
// §2). That answer outranks a pending close's tp run, whose lock the live run
// holds. The driver never reads its cycle through this: it holds the run lock
// and would see its own pool as live.
func AssembleResumeLive(start, taskFilePath, specPath string, tf *model.TaskFile) (ResumeResult, error) {
	return assembleResumeLive(start, taskFilePath, specPath, tf, resumeReads{})
}

// assembleResumeLive is AssembleResumeLive over what reads already holds.
func assembleResumeLive(start, taskFilePath, specPath string, tf *model.TaskFile, reads resumeReads) (ResumeResult, error) {
	res, err := assembleResume(start, taskFilePath, specPath, tf, nil, reads)
	if err != nil {
		return res, err
	}
	running := LiveRunningUnits(CanonicalProjectRoot(filepath.Dir(taskFilePath)), taskFilePath)
	if len(running) == 0 {
		return res, nil
	}
	res.RunningUnits = running
	res.NextUnits = []NextUnit{}
	res.NextAction = livePoolAction(taskFilePath, running)
	return res, nil
}

// livePoolAction is the next action beside a live pool: nothing to run.
func livePoolAction(taskFilePath string, running []string) NextAction {
	return NextAction{
		Summary: fmt.Sprintf("units of a tp run over %s are in progress (%s); a live run starts the next units itself, and a new tp run waits for these first",
			RunBase(taskFilePath), strings.Join(running, ", ")),
	}
}

// AssembleResumeRunning is AssembleResume with the implement tasks a rolling
// pool runs now left out of next_units (spec/backlog/rolling-implement.md
// §2); running is nil outside a pool.
func AssembleResumeRunning(start, taskFilePath, specPath string, tf *model.TaskFile, running []string) (ResumeResult, error) {
	return assembleResume(start, taskFilePath, specPath, tf, running, resumeReads{})
}

// resumeReads is what a reader assembling many resumes over one tree has
// read once for all of them, and what it does not print.
type resumeReads struct {
	// changes is WorktreeChanges(start), read by the caller; nil reads it.
	// Each resume classifies its own copy.
	changes []string
	// noUnitBranches leaves UnitBranches unread, one git call a reader that
	// prints no unit branch does not need.
	noUnitBranches bool
	// loops is readResumeLoops' answer for the one task file assembled, when
	// the caller has read it; nil reads it.
	loops *resumeLoops
}

// resumeTree classifies the working tree's changes, read already when
// shared is not nil, into the unexplained changes, the kept entries and tp's
// own bookkeeping, each sorted.
func resumeTree(start, taskFilePath, specPath string, tf *model.TaskFile, shared []string) (changes []string, kept []model.KeepEntry, bookkeeping []BookkeepingEntry) {
	// Working-tree classification against the keep-list (§4.5). A malformed keep
	// pattern leaves every change unexplained rather than silently swallowing it.
	raw := slices.Clone(shared)
	if shared == nil {
		raw = WorktreeChanges(start)
	}
	changes = raw
	kept = []model.KeepEntry{}
	if classified, err := ClassifyPaths(LoadKeepList(start), raw); err == nil {
		changes = classified.Changes
		kept = classified.Kept
	}
	// §5.2: tp-owned dirty state (the task file, .tp-review/, .tp/) is
	// bookkeeping — reported separately from changes and never counted as an
	// unexplained-changes blocker. Classified after the keep-list so an
	// explicitly keep-listed tp-owned file stays in kept and is not
	// double-reported.
	bookkeeping, changes = DeriveBookkeeping(start, taskFilePath, specPath, changes, tf)
	sort.Strings(changes)
	sort.Slice(kept, func(i, j int) bool { return kept[i].Path < kept[j].Path })
	return changes, kept, bookkeeping
}

// assembleResume is AssembleResumeRunning over what reads already holds.
func assembleResume(start, taskFilePath, specPath string, tf *model.TaskFile, running []string, reads resumeReads) (ResumeResult, error) {
	changes, kept, bookkeeping := resumeTree(start, taskFilePath, specPath, tf, reads.changes)

	loops := reads.loops
	if loops == nil {
		read, err := readResumeLoops(taskFilePath, specPath, tf)
		if err != nil {
			return ResumeResult{}, err
		}
		loops = &read
	}
	phase, wf := loops.phase, &loops.wf

	guidance := ""
	if phase == PhaseImplement {
		guidance = "run each task in a fresh subagent/context; keep the orchestrator context clean"
	}

	// Every path the result prints names the spec relative to the repository
	// root; specPath itself stays what the reads below resolve.
	shown := repoRelative(start, specPath)
	blockers, epicNotice, err := resumeBlockers(taskFilePath, shown, changes, tf, loops)
	if err != nil {
		return ResumeResult{}, err
	}

	// The units come after the blockers because an escalate blocker is what
	// empties them (§4.1), and next_action is rendered from the first unit so
	// the human summary and the machine surface describe the same work.
	nextUnits, round := BuildNextUnitsRunning(start, taskFilePath, specPath, phase, tf, loops.st, blockers, running)
	for i := range nextUnits {
		nextUnits[i].BriefCommand = resumeBriefCommand(start, specPath, shown, &nextUnits[i])
	}
	poolBriefs(nextUnits, len(running) > 0)
	if phase == PhaseImplement {
		guidance = implementGuidance(guidance, tf, wf.RunMaxParallel, nextUnits)
	}

	res := ResumeResult{
		Phase:       phase,
		Spec:        shown,
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
		NextAction:    renderNextAction(BuildNextAction(phase, specPath, shown, tf, loops.st, len(wf.Checks) > 0), nextUnits, blockers),
		Blockers:      blockers,
		PendingCloses: pendingCloseIDs(taskFilePath),
		RunningUnits:  running,
		FoundQueue:    FoundQueueCounts(taskFilePath, tf),
		EpicNotice:    epicNotice,
	}
	if !reads.noUnitBranches {
		res.UnitBranches = CycleUnitBranches(CanonicalProjectRoot(filepath.Dir(taskFilePath)), taskFilePath)
	}
	if len(res.PendingCloses) > 0 {
		res.NextAction = pendingCloseAction(res.PendingCloses)
	}
	return res, nil
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
	// reviewInFlight is the review round emitted and not recorded (0 when
	// none), and reviewInFlightOld whether its snapshot is not the spec.
	reviewInFlight    int
	reviewInFlightOld bool
}

// resumeBlockers derives tp resume's blockers from the loops read for a task
// file, and the epic block's notice. Only the implement phase reads the epic
// graph, since a blocked epic stops starting a task and nothing else
// (spec/1.5.0.md §6); an unbound file reads nothing. A graph or bound file that
// cannot be read is an error.
func resumeBlockers(taskFilePath, shownSpec string, changes []string, tf *model.TaskFile, loops *resumeLoops) ([]Blocker, string, error) {
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
		SpecPath:        shownSpec,
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

		UndecidedRequests: loops.phase == PhaseImplement && len(UndecidedRequestIDs(taskFilePath, 1)) > 0,

		ReviewInFlightRound:       loops.reviewInFlight,
		ReviewInFlightOverOldText: loops.reviewInFlightOld,

		ReviewBlockingFixedAtCap: loops.reviewDone.BlockingFixedAtCap,
		AuditBlockingFixedAtCap:  loops.auditDone.BlockingFixedAtCap,

		ReviewCarriedBlocking: loops.reviewDone.CarriedBlocking,
		ReviewLatestRound:     latestRoundNumber(loops.reviewRounds),
		ReviewUndisposedAtCap: loops.reviewDone.Undisposed,
		AuditCarriedBlocking:  loops.auditDone.CarriedBlocking,
		AuditUndisposedAtCap:  loops.auditDone.Undisposed,
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
	l.reviewDone = ReviewLoopDone(specPath, st, l.wf.ReviewCleanRounds, l.wf.EffectiveReviewMaxRounds(), specHash, l.wf.ReviewConvergeOn)
	l.auditDone = AuditLoopDone(specPath, st, l.wf.AuditCleanRounds, l.wf.EffectiveAuditMaxRounds(), specHash)
	l.reviewStale = StateStale(l.reviewRounds, specHash)
	l.reviewInFlight, l.reviewInFlightOld = reviewInFlight(specPath, len(l.reviewRounds))
	numDone := 0
	for i := range tf.Tasks {
		if tf.Tasks[i].Status == model.StatusDone {
			numDone++
		}
	}
	l.phase = DetectPhase(len(tf.Tasks), numDone, l.reviewDone.Done, l.auditDone.Done)
	return l, nil
}

// latestRoundNumber is the number of the latest recorded round, 0 when none.
func latestRoundNumber(rounds []ReviewRound) int {
	if len(rounds) == 0 {
		return 0
	}
	return rounds[len(rounds)-1].Round
}

// reviewInFlight returns the review round emitted and not recorded (0 when
// none) and whether its snapshot differs from the spec as it stands, the case
// in which a new emission is refused until the round is recorded or discarded.
// A spec or snapshot that cannot be read counts as differing: the emission
// guard refuses an unreadable snapshot too.
func reviewInFlight(specPath string, recorded int) (round int, overOldText bool) {
	round = InFlightRound(specPath, PhaseReview, recorded)
	if round == 0 {
		return 0, false
	}
	data, err := os.ReadFile(specPath)
	if err != nil {
		return round, true
	}
	overwrites, err := EmissionOverwrites(specPath, PhaseReview, round, data)
	return round, err != nil || len(overwrites) > 0
}
