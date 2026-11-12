package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/deligoez/tp/internal/engine"
	"github.com/deligoez/tp/internal/output"
)

// epicEmissionChangedReason is the reason a batch emission skips a slice
// whose unrecorded emission changed (spec/backlog/epic-reemit-guard.md).
const epicEmissionChangedReason = "emission-changed"

// epicPanel is the panel an emission built, whatever --role printed, as the
// snapshot stores it: each role's id, the hash of its role file and the
// output path its prompt names, relative to the repository root when it lies
// under it (engine.EpicStoredPath). The built-in regression role is no file
// and hashes to RolesHashBuiltin. An unreadable role file is an exit-3 error.
func epicPanel(run *epicReviewRun, built []epicReviewPrompt) ([]engine.EpicSnapshotRole, *epicRunError) {
	hashes, err := engine.EpicRoleFileHashes(filepath.Dir(run.tpDir))
	if err != nil {
		return nil, &epicRunError{code: ExitFile, msg: err.Error(), hint: "fix the role files under .tp/epic-reviewers/"}
	}
	root := engine.EpicRepoRoot(run.tpDir)
	panel := make([]engine.EpicSnapshotRole, 0, len(built))
	for i := range built {
		hash, ok := hashes[built[i].Role]
		if !ok {
			hash = engine.RolesHashBuiltin
		}
		panel = append(panel, engine.EpicSnapshotRole{ID: built[i].Role, Hash: hash, OutputPath: engine.EpicStoredPath(root, built[i].OutputPath)})
	}
	return panel, nil
}

// epicEmissionChanges reports what changed since run's round was emitted and
// left unrecorded: nil on a first emission or an identical one.
func epicEmissionChanges(run *epicReviewRun, panel []engine.EpicSnapshotRole) ([]string, *epicRunError) {
	changes, err := engine.EpicEmissionChanges(run.stateDir, run.round, run.all, panel)
	if err != nil {
		return nil, &epicRunError{code: ExitFile, msg: fmt.Sprintf("cannot read round %d's emission snapshot: %v", run.round, err), hint: epicRoundFileHint}
	}
	return changes, nil
}

// epicWaysForward is the one sentence every refusal over an unrecorded round
// ends in, naming its two ways forward in order: record it as emitted, or,
// the operator's decision, discard it and emit again.
func epicWaysForward(label string) string {
	return fmt.Sprintf("record the round as emitted with tp epic review %s --record-inbox, or, %s", label, epicDiscardWay(label))
}

// epicDiscardWay is the discard half of epicWaysForward, for a refusal whose
// round can no longer be recorded as emitted.
func epicDiscardWay(label string) string {
	return fmt.Sprintf("as the operator's decision, discard it with tp epic review %s --discard, then emit again", label)
}

// epicChangedError is the refusal of a re-emission whose emission changed.
func epicChangedError(run *epicReviewRun, changes []string) *epicRunError {
	return &epicRunError{code: ExitFile,
		msg: fmt.Sprintf("epic review round %d of %s is in flight and unrecorded, and its emission changed since: %s",
			run.round, epicSliceLabel(run.key), strings.Join(changes, "; ")),
		hint: epicWaysForward(epicSliceLabel(run.key))}
}

// guardEpicEmission is the refusal an epic review emission makes before it
// writes anything, as guardInFlightEmission is spec review's: re-emitting an
// unrecorded round whose units, purposes or panel changed would rebuild it
// under the roles still holding the old prompts. It exits 3 and returns only
// when the emission may proceed; discarding is --discard's, never an
// emission's.
func guardEpicEmission(run *epicReviewRun, panel []engine.EpicSnapshotRole) {
	changes, rerr := epicEmissionChanges(run, panel)
	if rerr != nil {
		exitEpicRunError(rerr)
	}
	if len(changes) > 0 {
		exitEpicRunError(epicChangedError(run, changes))
	}
}

// epicDiscardResult is tp epic review --discard's payload.
type epicDiscardResult struct {
	StateDir     string `json:"state_dir"`
	Round        int    `json:"round"`
	FilesRemoved int    `json:"files_removed"`
	NextAction   string `json:"next_action"`
}

// runEpicReviewDiscard removes the slice's unrecorded emission, its snapshot
// and exactly the files its stored panel named, and emits nothing, so the
// dispositions it froze can change before the round is emitted again
// (spec/backlog/epic-reemit-guard.md). It is the operator's decision: under
// TP_UNATTENDED it exits 2 (discard-emission) before reading anything. With
// no unrecorded emission it exits 4 and writes nothing.
func runEpicReviewDiscard(key engine.EpicSliceKey) {
	if engine.Unattended() {
		refuseUnattended("tp epic review --discard removes an unrecorded emission, which", engine.EscalateDiscardEmission)
	}
	run := loadEpicReviewRun(key)
	label := epicSliceLabel(key)
	panel, stored, err := engine.EpicStoredPanel(run.stateDir, run.round)
	switch {
	case err != nil:
		exitEpicRunError(&epicRunError{code: ExitFile, msg: fmt.Sprintf("cannot read round %d's emission snapshot: %v", run.round, err), hint: epicRoundFileHint})
	case !stored:
		exitEpicRunError(&epicRunError{code: ExitState, msg: fmt.Sprintf("epic review round %d of %s has no unrecorded emission to discard", run.round, label),
			hint: "emit the round with tp epic review " + label + ", or read its loop with tp epic review " + label + " --status"})
	}
	removed, err := engine.DiscardEpicEmission(run.stateDir, run.round, engine.EpicRepoRoot(run.tpDir), panel)
	if err != nil {
		output.Error(ExitFile, "cannot discard the emission: "+err.Error(), epicReviewStateHint)
		os.Exit(ExitFile)
	}
	output.Notice(fmt.Sprintf("--discard: discarded epic review round %d's unrecorded emission of %s; removed %d role file(s) it named",
		run.round, label, len(removed)))
	result := epicDiscardResult{StateDir: epicReviewStateDirOut(&run), Round: run.round, FilesRemoved: len(removed),
		NextAction: fmt.Sprintf("dispose of the last recorded round's findings if needed (tp epic review %s --resolve), then emit the round again with tp epic review %s", label, label)}
	if err := output.JSON(result); err != nil {
		output.Error(ExitFile, err.Error(), internalEncodeHint)
		os.Exit(ExitFile)
	}
}

// refuseEpicForceAlone exits 2, the invalid-combination code, when --force
// comes without --resolve or --resolve-all: on epic review it only
// re-resolves, and a discard is --discard's.
func refuseEpicForceAlone(cmd *cobra.Command) {
	if !cmd.Flags().Changed("force") || cmd.Flags().Changed("resolve") || cmd.Flags().Changed("resolve-all") {
		return
	}
	output.Error(ExitUsage, "--force only re-resolves on epic review (with --resolve or --resolve-all); to discard an unrecorded emission use tp epic review <slice> --discard",
		"drop --force, or add --resolve <index> or --resolve-all to re-resolve findings that already carry a disposition")
	os.Exit(ExitUsage)
}

// epicDiscardAlone lists the flags --discard refuses beside it: every other
// mode and the flags that narrow one, since a discard is one decision on one
// slice and its own call. The batch flags are epicBatchRefused's.
var epicDiscardAlone = []string{"record", "record-inbox", "resolve", "resolve-all", "status", "check", "history", "report", "grant-round", "role", "force"}

// refuseEpicDiscardBeside exits 2, the invalid-combination code, when
// --discard comes with a flag of epicDiscardAlone.
func refuseEpicDiscardBeside(cmd *cobra.Command) {
	for _, name := range epicDiscardAlone {
		if cmd.Flags().Changed(name) {
			output.Error(ExitUsage, "--discard takes no --"+name+": it removes an unrecorded emission and is its own call",
				"discard with tp epic review <slice> --discard, then run the other mode on its own")
			os.Exit(ExitUsage)
		}
	}
}
