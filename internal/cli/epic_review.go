package cli

import (
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/deligoez/tp/internal/engine"
	"github.com/deligoez/tp/internal/output"
)

// epicReviewUsageHint answers a tp epic review call that names no slice, or two.
const epicReviewUsageHint = "pass exactly one of --milestone <m> or --epic <id>"

// epicReviewStateHint answers a failed write under the slice's state
// directory. The shared stateWriteHint names tp review's .tp-review, not the
// directory tp epic review writes.
const epicReviewStateHint = "check that the .tp/epic-review/ state directory exists and is writable"

// epicReviewPrompt is one graph role's prompt in a tp epic review emission.
type epicReviewPrompt struct {
	Role       string `json:"role"`
	Prompt     string `json:"prompt"`
	OutputPath string `json:"output_path"`
}

// epicReviewResult is tp epic review's emission payload (spec/1.3.0.md §5).
type epicReviewResult struct {
	StateDir          string                       `json:"state_dir"`
	Round             int                          `json:"round"`
	Units             []string                     `json:"units"`
	Context           []string                     `json:"context"`
	Carried           []string                     `json:"carried,omitempty"`
	MechanicalChecks  []engine.EpicMechanicalCheck `json:"mechanical_checks"`
	MechanizedClasses []string                     `json:"mechanized_classes"`
	SkippedRoles      *[]engine.SkippedRole        `json:"skipped_roles,omitempty"`
	Prompts           []epicReviewPrompt           `json:"prompts"`
	NextAction        string                       `json:"next_action"`
}

// epicReviewRun carries what one emission is built from.
type epicReviewRun struct {
	key      engine.EpicSliceKey
	tpDir    string
	stateDir string
	round    int
	slice    engine.EpicSlice // the asked units and their context
	all      []engine.EpicReviewUnit
	carry    engine.EpicCarry
	checks   []engine.EpicMechanicalCheck
}

func newEpicReviewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "review",
		Short: "Emit one prompt per graph role over a milestone's or an epic's slice of .tp/epics.json",
		Args:  epicReviewArgs,
		RunE:  runEpicReview,
	}
	cmd.Flags().String("milestone", "", "Review every epic of this milestone, plus its gate")
	cmd.Flags().String("epic", "", "Review this one epic")
	cmd.Flags().String("role", "", "Emit only this role's prompt")
	cmd.Flags().String("record", "", "Record this NDJSON findings file as the slice's next round")
	cmd.Flags().Bool("status", false, "Print the slice's convergence status")
	cmd.Flags().Bool("check", false, "With --status: exit 0 only when the loop is done and every mechanical check passes")
	cmd.Flags().Bool("resolve", false, "Mark one finding of the last recorded round fixed/wontfix/duplicate (positionals: 0-based index, status, evidence)")
	cmd.Flags().Bool("resolve-all", false, "Mark every unresolved finding of the last recorded round with a status (positionals: status, evidence)")
	cmd.Flags().Bool("force", false, "With --resolve or --resolve-all: re-resolve findings that already carry a disposition")
	return cmd
}

// epicReviewArgs admits positionals only to --resolve and --resolve-all.
func epicReviewArgs(cmd *cobra.Command, args []string) error {
	resolve, _ := cmd.Flags().GetBool("resolve")
	resolveAll, _ := cmd.Flags().GetBool("resolve-all")
	if resolve || resolveAll {
		return nil
	}
	return cobra.NoArgs(cmd, args)
}

// runEpicReview emits a round over the slice the flags name. Neither or both
// slice flags is a usage error decided before anything is read or written.
func runEpicReview(cmd *cobra.Command, args []string) error {
	key := epicReviewSliceKey(cmd)
	if runEpicReviewDisposition(cmd, key, args) {
		return nil
	}
	if cmd.Flags().Changed("record") {
		recordPath, _ := cmd.Flags().GetString("record")
		runEpicReviewRecord(key, recordPath)
		return nil
	}
	if status, _ := cmd.Flags().GetBool("status"); status {
		check, _ := cmd.Flags().GetBool("check")
		runEpicReviewStatus(key, check)
		return nil
	}
	run := loadEpicReviewRun(key)
	refuseEpicReviewAtCap(&run)
	roles, err := engine.ResolveEpicRoleCorpus(filepath.Dir(run.tpDir))
	if err != nil {
		output.Error(ExitFile, err.Error(), "fix the role files under .tp/epic-reviewers/")
		os.Exit(ExitFile)
		return nil
	}
	prompts := buildEpicReviewPrompts(&run, roles)
	roleName, _ := cmd.Flags().GetString("role")
	q := roleQuery{name: roleName, given: cmd.Flags().Changed("role"), specDir: filepath.Dir(run.tpDir)}
	prompts, skipped := filterByRole(prompts, func(p *epicReviewPrompt) string { return p.Role }, q, nil)
	if err := engine.WriteEpicReviewSnapshot(run.stateDir, run.round, run.all); err != nil {
		output.Error(ExitFile, "cannot write the epic review snapshot: "+err.Error(), epicReviewStateHint)
		os.Exit(ExitFile)
		return nil
	}
	result := epicReviewPayload(&run, prompts)
	result.SkippedRoles = skippedRolesForPayload(q.given, len(prompts), skipped)
	if err := output.JSON(result); err != nil {
		output.Error(ExitFile, err.Error(), internalEncodeHint)
		os.Exit(ExitFile)
	}
	return nil
}

// runEpicReviewDisposition runs --resolve or --resolve-all when either is set
// and reports whether it did.
func runEpicReviewDisposition(cmd *cobra.Command, key engine.EpicSliceKey, args []string) bool {
	force, _ := cmd.Flags().GetBool("force")
	if resolve, _ := cmd.Flags().GetBool("resolve"); resolve {
		runEpicReviewResolve(key, args, force)
		return true
	}
	if resolveAll, _ := cmd.Flags().GetBool("resolve-all"); resolveAll {
		runEpicReviewResolveAll(key, args, force)
		return true
	}
	return false
}

// epicReviewSliceKey reads the slice flags, exiting 2 unless exactly one is set.
func epicReviewSliceKey(cmd *cobra.Command) engine.EpicSliceKey {
	milestone, _ := cmd.Flags().GetString("milestone")
	epic, _ := cmd.Flags().GetString("epic")
	if (milestone == "") == (epic == "") {
		output.Error(ExitUsage, "tp epic review takes exactly one of --milestone or --epic", epicReviewUsageHint)
		os.Exit(ExitUsage)
	}
	return engine.EpicSliceKey{Milestone: milestone, Epic: epic}
}

// epicRoundFileHint answers a recorded round tp cannot read back or rewrite.
// The file is tp's own artifact, so neither stateWriteHint's .tp-review nor
// ndjsonInputFileHint's "check the path" describes it.
const epicRoundFileHint = "the file the message names is a recorded epic review round under .tp/epic-review/, tp's own artifact rather than a path you passed: make it readable and writable again, or restore its content"

// loadEpicReviewRun reads the graph and derives the slice and its round. It
// writes nothing: an absent graph exits 3, a slice the graph does not hold 4.
func loadEpicReviewRun(key engine.EpicSliceKey) epicReviewRun {
	tpDir := engine.DiscoverTPDir(".")
	if tpDir == "" {
		output.Error(ExitFile, "epic graph not found: no .tp/ directory up to the git boundary", epicGraphFileHint)
		os.Exit(ExitFile)
	}
	g, unknown, err := engine.LoadEpicGraph(tpDir)
	if err != nil {
		output.Error(ExitFile, err.Error(), epicGraphFileHint)
		os.Exit(ExitFile)
	}
	slice, err := engine.EpicReviewSlice(g, key)
	if err != nil {
		output.Error(ExitState, err.Error(), epicNotFoundHint)
		os.Exit(ExitState)
	}
	stateDir := engine.EpicReviewStateDir(tpDir, key)
	round := engine.EpicReviewNextRound(stateDir)
	carry, err := engine.EpicReviewCarry(stateDir, round, slice.Units)
	if err != nil {
		output.Error(ExitFile, err.Error(), epicRoundFileHint)
		os.Exit(ExitFile)
	}
	return epicReviewRun{
		key: key, tpDir: tpDir, stateDir: stateDir, round: round, all: slice.Units, carry: carry,
		slice:  engine.EpicSlice{Units: carry.Asked, Context: engine.EpicCarriedContext(g, carry.Asked)},
		checks: engine.EpicMechanicalChecks(engine.LintEpicGraph(g, unknown)),
	}
}
