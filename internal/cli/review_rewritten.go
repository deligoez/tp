package cli

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/deligoez/tp/internal/engine"
	"github.com/deligoez/tp/internal/output"
)

// rewrittenRefusal is a refusal --rewritten decides under the state lock and
// reports once the lock is released.
type rewrittenRefusal struct {
	code      int
	msg, hint string
}

func (r *rewrittenRefusal) Error() string { return r.msg }

// runReviewRewritten implements `tp review <spec> --rewritten "<evidence>"`
// (spec/backlog/reconcile.md §2.1): it appends an epoch placed, per phase,
// after the last recorded round that read an older text (EpochPosition), so
// those rounds stop feeding the lists of earlier findings an emission carries
// while the rounds over the current text keep feeding them. It writes
// state.json only — no round, no disposition — and changes no convergence
// rule or cap.
//
// Refused: blank evidence (exit 2); no recorded round in either phase (exit
// 2); a round of either phase in flight, since it was built from the lists
// the epoch cuts (exit 3, the emission guard's code) — once it is recorded
// the epoch keeps it if it read the current text; and an epoch whose position
// in both phases equals the newest epoch's, since no round over an older text
// is left uncut (exit 2).
func runReviewRewritten(specPath, evidence string, force, noState bool) error {
	if force || noState {
		output.Error(ExitUsage, "--rewritten cannot be combined with --force or --no-state",
			`--rewritten writes only the epoch: tp review <spec> --rewritten "<evidence>"`)
		os.Exit(ExitUsage)
		return nil
	}
	if strings.TrimSpace(evidence) == "" {
		output.Error(ExitUsage, "--rewritten requires evidence: what was rewritten, and why",
			`tp review <spec> --rewritten "<evidence>"`)
		os.Exit(ExitUsage)
		return nil
	}
	specHash, err := engine.SpecHash(specPath)
	if err != nil {
		output.Error(ExitFile, fmt.Sprintf("cannot read spec: %s", specPath), specFileMissingHint)
		os.Exit(ExitFile)
		return nil
	}

	// Decided once before the lock, since taking it creates the state
	// directory a refusal must not leave behind, and again under it, since a
	// round may be emitted or recorded in between.
	decide := func() (*engine.ReviewState, error) {
		st, loadErr := engine.LoadReviewState(specPath)
		if loadErr != nil && !engine.IsRebuildableStateIndex(loadErr) {
			return nil, loadErr
		}
		if loadErr != nil {
			st = nil
		}
		if refusal := refuseEpoch(specPath, st, specHash); refusal != nil {
			return nil, refusal
		}
		return st, nil
	}
	var epoch engine.Epoch
	_, lockErr := decide()
	if lockErr == nil {
		lockErr = engine.WithReviewStateLock(specPath, func() error {
			st, err := decide()
			if err != nil {
				return err
			}
			return appendEpoch(specPath, st, specHash, evidence, &epoch)
		})
	}
	var refusal *rewrittenRefusal
	if errors.As(lockErr, &refusal) {
		output.Error(refusal.code, refusal.msg, refusal.hint)
		os.Exit(refusal.code)
		return nil
	}
	if lockErr != nil {
		exitStateError(lockErr)
		return nil
	}
	return output.JSON(map[string]any{"spec": specPath, "epoch": epoch})
}

// appendEpoch places the epoch after the last recorded round of each phase,
// stores it into *epoch for the payload, and saves the state.
// appendEpoch places the epoch at each phase's EpochPosition — after its last
// recorded round over an older text — stores it into *epoch for the payload,
// and saves the state.
func appendEpoch(specPath string, st *engine.ReviewState, specHash, evidence string, epoch *engine.Epoch) error {
	*epoch = engine.Epoch{
		AfterReviewRound: engine.EpochPosition(st, engine.PhaseReview, specHash),
		AfterAuditRound:  engine.EpochPosition(st, engine.PhaseAudit, specHash),
		Evidence:         evidence,
		At:               time.Now().UTC().Format(time.RFC3339),
	}
	st.Epochs = append(st.Epochs, *epoch)
	return engine.SaveReviewState(specPath, st)
}

// refuseEpoch returns the refusal an epoch over st meets, or nil.
func refuseEpoch(specPath string, st *engine.ReviewState, specHash string) *rewrittenRefusal {
	if st == nil || (len(st.ReviewRounds) == 0 && len(st.AuditRounds) == 0) {
		return &rewrittenRefusal{code: ExitUsage,
			msg:  "--rewritten has nothing to cut: the spec has no recorded round",
			hint: fmt.Sprintf("an epoch separates recorded rounds from a rewritten text; run the review loop: tp review %s", specPath)}
	}
	for _, phase := range []struct {
		name   string
		rounds []engine.ReviewRound
	}{{engine.PhaseReview, st.ReviewRounds}, {engine.PhaseAudit, st.AuditRounds}} {
		if r := engine.InFlightRound(specPath, phase.name, len(phase.rounds)); r > 0 {
			return inFlightEpochRefusal(specPath, phase.name, r)
		}
	}
	for _, phase := range []string{engine.PhaseReview, engine.PhaseAudit} {
		if engine.EpochPosition(st, phase, specHash) != engine.EpochCut(st, phase) {
			return nil
		}
	}
	return &rewrittenRefusal{code: ExitUsage,
		msg:  "--rewritten has nothing to cut: every recorded round already read the current text, or was cut by an earlier epoch",
		hint: "an epoch cuts the rounds that read an older text; rewrite the spec first, or keep the loop going"}
}

// inFlightEpochRefusal names the round in flight and the ways past it: its
// record, or — when it was emitted over text the spec no longer holds — its
// discard, the operator's decision.
func inFlightEpochRefusal(specPath, phase string, round int) *rewrittenRefusal {
	hint := fmt.Sprintf("record %s round %d first (tp %s %s --record <file>), then start the epoch", phase, round, phase, specPath)
	if changed, _ := inFlightSnapshotChanged(specPath, phase, round); changed {
		hint += fmt.Sprintf("; discarding its emission instead is the operator's decision (tp %s %s --force, %s)",
			phase, specPath, engine.EscalateDiscardEmission)
	}
	return &rewrittenRefusal{code: ExitFile,
		msg:  fmt.Sprintf("%s round %d is in flight and unrecorded: its prompts were built from the lists of earlier findings an epoch cuts", phase, round),
		hint: hint}
}

// addEpochs puts the spec's epochs on a --status payload of either phase, and
// nothing when there are none. The list survives --compact: it is why an
// emission carries fewer earlier findings than the rounds hold.
func addEpochs(result map[string]any, st *engine.ReviewState) {
	if st != nil && len(st.Epochs) > 0 {
		result["epochs"] = st.Epochs
	}
}

// inFlightSnapshotChanged reports whether round's emission snapshot for phase
// differs from the spec as it now stands.
func inFlightSnapshotChanged(specPath, phase string, round int) (bool, error) {
	data, err := os.ReadFile(specPath)
	if err != nil {
		return false, err
	}
	overwrites, err := engine.EmissionOverwrites(specPath, phase, round, data)
	return len(overwrites) > 0, err
}
