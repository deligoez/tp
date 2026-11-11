package cli

import (
	"errors"
	"os"
	"path/filepath"
	"slices"

	"github.com/spf13/cobra"

	"github.com/deligoez/tp/internal/engine"
	"github.com/deligoez/tp/internal/model"
	"github.com/deligoez/tp/internal/output"
)

// epicReviewUsageHint answers a tp epic review call that names no slice, or two.
const epicReviewUsageHint = "pass exactly one of --milestone <m>, --epic <id> or --epics <id,id,...> [--name <name>]"

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
	// Key names the slice in a batch emission (spec/1.7.0.md §5), as --status
	// --all prints it; a one-slice emission leaves it out.
	Key               *engine.EpicSliceKeyRecord   `json:"key,omitempty"`
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
	handoff  []engine.EpicContextBlock // epics a changed unit's text names, for regression
}

func newEpicReviewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "review",
		Short: "Emit one prompt per graph role over a slice of .tp/epics.json: a milestone, an epic, or a set of epics",
		Args:  epicReviewArgs,
		RunE:  runEpicReview,
	}
	cmd.Flags().String("milestone", "", "Review every epic of this milestone, plus its gate")
	cmd.Flags().String("epic", "", "Review this one epic")
	cmd.Flags().StringSlice("epics", nil, "Review this set of epics, comma-separated CSV (quote an id holding a comma); repeatable")
	cmd.Flags().String("name", "", "With --epics: key the set's state by this name, so the set may grow between rounds")
	cmd.Flags().StringSlice("milestones", nil, "Emit each of these milestones' rounds in one payload, comma-separated; with --status or --report, read each one's slice")
	cmd.Flags().Bool("all", false, "Emit every tracked milestone's round in one payload; with --status or --report, read every slice directory under .tp/epic-review/")
	cmd.Flags().Bool("report", false, "With --all or --milestones: list each finding class with the slices, rounds and distinct epics it was raised on")
	cmd.Flags().String("role", "", "Emit only this role's prompt")
	cmd.Flags().String("record", "", "Record this NDJSON findings file as the slice's next round")
	cmd.Flags().Bool("status", false, "Print the slice's convergence status")
	cmd.Flags().Bool("check", false, "With --status: exit 0 only when the loop is done and every mechanical check passes")
	cmd.Flags().Bool("resolve", false, "Mark one finding of the last recorded round fixed/wontfix/duplicate (positionals: 0-based index, status, evidence)")
	cmd.Flags().Bool("resolve-all", false, "Mark every unresolved finding of the last recorded round with a status (positionals: status, evidence)")
	cmd.Flags().Bool("force", false, "With --resolve or --resolve-all: re-resolve findings that already carry a disposition")
	cmd.Flags().Bool("grant-round", false, "Grant a stale slice at review_max_rounds one more round to emit and record (the operator's decision)")
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
	if grant, _ := cmd.Flags().GetBool("grant-round"); grant {
		refuseEpicGrantBeside(cmd)
		runEpicReviewGrant(epicReviewSliceKey(cmd))
		return nil
	}
	if scope, batch := epicBatchFlags(cmd); batch {
		runEpicReviewBatch(cmd, scope)
		return nil
	}
	refuseEpicReportOutsideBatch(cmd)
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
	runEpicReviewEmit(cmd, key)
	return nil
}

// runEpicReviewEmit emits the slice's next round: its prompts, narrowed by
// --role, and the emission's state.
func runEpicReviewEmit(cmd *cobra.Command, key engine.EpicSliceKey) {
	run := loadEpicReviewRun(key)
	refuseEpicReviewAtCap(&run)
	roles, err := engine.ResolveEpicRoleCorpus(filepath.Dir(run.tpDir))
	if err != nil {
		output.Error(ExitFile, err.Error(), "fix the role files under .tp/epic-reviewers/")
		os.Exit(ExitFile)
		return
	}
	prompts := buildEpicReviewPrompts(&run, roles)
	roleName, _ := cmd.Flags().GetString("role")
	q := roleQuery{name: roleName, given: cmd.Flags().Changed("role"), specDir: filepath.Dir(run.tpDir)}
	prompts, skipped := filterByRole(prompts, func(p *epicReviewPrompt) string { return p.Role }, q, nil)
	if err := writeEpicReviewEmission(&run); err != nil {
		output.Error(ExitFile, "cannot write the epic review state: "+err.Error(), epicReviewStateHint)
		os.Exit(ExitFile)
		return
	}
	result := epicReviewPayload(&run, prompts)
	result.SkippedRoles = skippedRolesForPayload(q.given, len(prompts), skipped)
	if err := output.JSON(result); err != nil {
		output.Error(ExitFile, err.Error(), internalEncodeHint)
		os.Exit(ExitFile)
	}
}

// writeEpicReviewEmission records an emission in the slice's state directory:
// the round's unit snapshot and the slice's key (spec/1.5.0.md §7), and makes
// the inbox the prompts name (spec/1.7.0.md §1), with the .tp/.gitignore entry
// that keeps it untracked. It removes nothing there, so a role that already
// wrote its file keeps it through a re-emission.
func writeEpicReviewEmission(run *epicReviewRun) error {
	if err := engine.WriteEpicReviewSnapshot(run.stateDir, run.round, run.all); err != nil {
		return err
	}
	if err := engine.WriteEpicSliceKey(run.stateDir, run.key); err != nil {
		return err
	}
	if err := engine.EnsureTPGitignore(run.tpDir); err != nil {
		return err
	}
	return os.MkdirAll(engine.EpicReviewInboxDir(run.stateDir), 0o755)
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

// epicReviewSliceKey reads the slice flags, exiting 2 unless exactly one of
// --milestone, --epic and --epics is set, when --epics lists no id or an
// empty one, or when --name comes without --epics.
func epicReviewSliceKey(cmd *cobra.Command) engine.EpicSliceKey {
	milestone, _ := cmd.Flags().GetString("milestone")
	epic, _ := cmd.Flags().GetString("epic")
	epics, _ := cmd.Flags().GetStringSlice("epics")
	name, _ := cmd.Flags().GetString("name")
	set := cmd.Flags().Changed("epics")
	given := 0
	for _, on := range []bool{milestone != "", epic != "", set} {
		if on {
			given++
		}
	}
	switch {
	case given != 1:
		output.Error(ExitUsage, "tp epic review takes exactly one of --milestone, --epic or --epics", epicReviewUsageHint)
		os.Exit(ExitUsage)
	case cmd.Flags().Changed("name") && !set:
		output.Error(ExitUsage, "--name names a set slice and needs --epics", epicReviewUsageHint)
		os.Exit(ExitUsage)
	case set && (len(epics) == 0 || slices.Contains(epics, "")):
		output.Error(ExitUsage, "--epics needs at least one id and no empty one", epicReviewUsageHint)
		os.Exit(ExitUsage)
	}
	return engine.EpicSliceKey{Milestone: milestone, Epic: epic, Epics: epics, Name: name}
}

// epicRoundFileHint answers a recorded round tp cannot read back or rewrite.
// The file is tp's own artifact, so neither stateWriteHint's .tp-review nor
// ndjsonInputFileHint's "check the path" describes it.
const epicRoundFileHint = "the file the message names is a recorded epic review round under .tp/epic-review/, tp's own artifact rather than a path you passed: make it readable and writable again, or restore its content"

// loadEpicReviewRun reads the graph and derives the slice and its round. It
// writes nothing: an absent graph exits 3, a slice the graph does not hold 4.
// epicReviewGraph is what every slice of one tp epic review call is prepared
// from: the graph, read once, and its mechanical checks, which lint the whole
// graph and so are the same for every slice.
type epicReviewGraph struct {
	tpDir  string
	g      *model.EpicGraph
	checks []engine.EpicMechanicalCheck
}

// epicRunError is a slice that cannot be prepared, with the exit code and
// hint a one-slice call exits with.
type epicRunError struct {
	code int
	msg  string
	hint string
}

// exitEpicRunError exits with err's code and hint.
func exitEpicRunError(err *epicRunError) {
	output.Error(err.code, err.msg, err.hint)
	os.Exit(err.code)
}

// loadEpicReviewGraph reads the graph and lints it. It writes nothing: an
// absent or unreadable graph exits 3.
func loadEpicReviewGraph() epicReviewGraph {
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
	return epicReviewGraph{tpDir: tpDir, g: g, checks: engine.EpicMechanicalChecks(engine.LintEpicGraph(g, unknown))}
}

// prepareEpicReviewRun derives key's slice over eg and its round, writing
// nothing: a slice the graph does not hold is an exit-4 error, a recorded
// round tp cannot read back an exit-3 one.
func prepareEpicReviewRun(eg *epicReviewGraph, key engine.EpicSliceKey) (epicReviewRun, *epicRunError) {
	slice, err := engine.EpicReviewSlice(eg.g, key)
	if err != nil {
		hint := epicNotFoundHint
		if errors.As(err, new(*engine.EpicMilestoneNotFoundError)) {
			hint = epicMilestoneNotFoundHint
		}
		return epicReviewRun{}, &epicRunError{code: ExitState, msg: err.Error(), hint: hint}
	}
	stateDir := engine.EpicReviewStateDir(eg.tpDir, key)
	round := engine.EpicReviewNextRound(stateDir)
	carry, err := engine.EpicReviewCarry(stateDir, round, slice.Units)
	if err != nil {
		return epicReviewRun{}, &epicRunError{code: ExitFile, msg: err.Error(), hint: epicRoundFileHint}
	}
	context := engine.EpicCarriedContext(eg.g, carry.Asked)
	return epicReviewRun{
		key: key, tpDir: eg.tpDir, stateDir: stateDir, round: round, all: slice.Units, carry: carry,
		slice:   engine.EpicSlice{Units: carry.Asked, Context: context},
		checks:  eg.checks,
		handoff: engine.EpicHandoffContext(eg.g, &carry, context),
	}, nil
}

// loadEpicReviewRun reads the graph and derives the slice and its round. It
// writes nothing: an absent graph exits 3, a slice the graph does not hold 4.
func loadEpicReviewRun(key engine.EpicSliceKey) epicReviewRun {
	eg := loadEpicReviewGraph()
	run, err := prepareEpicReviewRun(&eg, key)
	if err != nil {
		exitEpicRunError(err)
	}
	return run
}
