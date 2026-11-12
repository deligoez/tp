package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

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

// epicWaysForward names what an unrecorded round leaves open, in order:
// record it as emitted, or discard it, the operator's decision.
func epicWaysForward(label string) string {
	return fmt.Sprintf("record the round as emitted with tp epic review %s --record-inbox, or, as the operator's decision, discard it and emit again with tp epic review %s --force",
		label, label)
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
// under the roles still holding the old prompts. It exits 3 unless force,
// which discards the old emission and exactly the files its stored panel
// named, said on stderr; under TP_UNATTENDED a --force that would discard
// exits 2 (discard-emission). It returns only when the emission may proceed.
func guardEpicEmission(run *epicReviewRun, panel []engine.EpicSnapshotRole, force bool) {
	changes, rerr := epicEmissionChanges(run, panel)
	if rerr != nil {
		exitEpicRunError(rerr)
	}
	if len(changes) == 0 {
		return
	}
	label := epicSliceLabel(run.key)
	if force && engine.Unattended() {
		refuseUnattended(fmt.Sprintf("--force discarding epic review round %d's unrecorded emission of %s", run.round, label), engine.EscalateDiscardEmission)
		return
	}
	if !force {
		exitEpicRunError(epicChangedError(run, changes))
		return
	}
	old, _, err := engine.EpicStoredPanel(run.stateDir, run.round)
	var removed []string
	if err == nil {
		removed, err = engine.RemoveEpicPanelFiles(engine.EpicRepoRoot(run.tpDir), old)
	}
	if err != nil {
		output.Error(ExitFile, "cannot remove the discarded emission's role files: "+err.Error(), epicReviewStateHint)
		os.Exit(ExitFile)
	}
	output.Notice(fmt.Sprintf("--force: discarding epic review round %d's unrecorded emission of %s (%s); removed %d role file(s) it named",
		run.round, label, strings.Join(changes, "; "), len(removed)))
}
