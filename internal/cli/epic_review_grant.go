package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/deligoez/tp/internal/engine"
	"github.com/deligoez/tp/internal/output"
)

// epicGrantResult is tp epic review --grant-round's payload: the round the
// grant lets the slice emit and record past its cap.
type epicGrantResult struct {
	StateDir   string `json:"state_dir"`
	Round      int    `json:"round"`
	NextAction string `json:"next_action"`
}

// epicGrantRoundAlone lists the flags --grant-round refuses beside it: the
// other modes, and the batch flags spec/1.7.0.md §5 adds, since a grant is
// given one slice at a time (§8, non-goals). A flag the command does not
// define yet is skipped, so the batch flags are refused from the release that
// adds them without this list changing.
var epicGrantRoundAlone = []string{"record", "record-inbox", "discard", "status", "check", "resolve", "resolve-all", "force", "role", "milestones", "all", "report"}

// refuseEpicGrantBeside exits 2 when --grant-round comes with another mode or
// a batch flag.
func refuseEpicGrantBeside(cmd *cobra.Command) {
	for _, name := range epicGrantRoundAlone {
		if f := cmd.Flags().Lookup(name); f != nil && f.Changed {
			output.Error(ExitUsage, "--grant-round takes no --"+name+": it grants one round to one slice and is its own call",
				"run tp epic review <slice> --grant-round alone, then emit the granted round with tp epic review <slice>")
			os.Exit(ExitUsage)
		}
	}
}

// epicGrantHint names the grant to a slice whose graph changed after its
// cap, before the project cap the operator could raise instead.
func epicGrantHint(key engine.EpicSliceKey) string {
	return fmt.Sprintf("the operator grants one verification round with tp epic review %s --grant-round, "+
		"or raises review_max_rounds for every slice (tp set --workflow --project review_max_rounds=<N>, .tp/config.json)", epicSliceLabel(key))
}

// runEpicReviewGrant grants the slice one round past review_max_rounds
// (spec/1.7.0.md §8). Under TP_UNATTENDED it exits 2 before reading anything,
// escalating grant-round; a slice below its cap, unchanged since its last
// round, or with a grant already open exits 4, the cap refusal's code, and
// writes nothing.
func runEpicReviewGrant(key engine.EpicSliceKey) {
	if engine.Unattended() {
		refuseUnattended("tp epic review --grant-round", engine.EscalateGrantRound)
	}
	run := loadEpicReviewRun(key)
	st := epicReviewLoop(&run)
	label := epicSliceLabel(key)
	if reason := epicGrantRefusal(&st, label); reason != "" {
		output.Error(ExitState, "no round granted to "+label+": "+reason, epicStatusNextAction(key, &st, epicChecksPass(run.checks)))
		os.Exit(ExitState)
	}
	round := st.Round + 1
	if err := engine.WriteEpicReviewGrant(run.stateDir, round); err != nil {
		output.Error(ExitFile, "cannot write the epic review grant: "+err.Error(), epicReviewStateHint)
		os.Exit(ExitFile)
	}
	result := epicGrantResult{
		StateDir:   epicReviewStateDirOut(&run),
		Round:      round,
		NextAction: fmt.Sprintf("emit round %d: tp epic review %s", round, label),
	}
	if err := output.JSON(result); err != nil {
		output.Error(ExitFile, err.Error(), internalEncodeHint)
		os.Exit(ExitFile)
	}
}

// epicGrantRefusal says why the slice cannot be granted a round, "" when it
// can: only a slice at its cap and stale, with no grant open, takes one.
func epicGrantRefusal(st *engine.EpicReviewStatus, label string) string {
	switch {
	case st.GrantedRound > 0:
		return fmt.Sprintf("round %d is already granted and not yet recorded: emit or record it with tp epic review %s", st.GrantedRound, label)
	case st.Done:
		return "the slice's loop is done by " + st.DoneBy
	case !st.CapReached:
		return fmt.Sprintf("%d rounds recorded, below review_max_rounds: the slice emits its next round without a grant", st.Round)
	case !st.Stale:
		return fmt.Sprintf("the graph has not changed since round %d, so one more round has nothing new to read", st.Round)
	}
	return ""
}
