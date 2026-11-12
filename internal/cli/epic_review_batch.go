package cli

import (
	"os"
	"path/filepath"
	"slices"

	"github.com/spf13/cobra"

	"github.com/deligoez/tp/internal/engine"
	"github.com/deligoez/tp/internal/model"
	"github.com/deligoez/tp/internal/output"
)

// epicBatchUsageHint answers a batch call tp epic review refuses.
const epicBatchUsageHint = "pass --milestones <m,m,...> or --all and no one-slice flag; record, resolve and grant one slice at a time with --milestone <m>, --epic <id> or --epics <ids>"

// epicBatchScope is the slices a batch call names (spec/1.7.0.md §5): the
// milestones --milestones lists, in the order given, or --all.
type epicBatchScope struct {
	milestones []string
	all        bool
}

// epicBatchRefused lists the flags --milestones and --all refuse beside
// them: the one-slice flags, which name a slice the batch names itself, and
// the writes the non-goals keep one slice at a time.
var epicBatchRefused = []string{"milestone", "epic", "epics", "name", "record", "resolve", "resolve-all", "force"}

// epicBatchFlags reads --milestones and --all, reporting false when neither
// is set. Both, a refused flag beside either, or a --milestones list that is
// empty or holds an empty name exits 2 before anything is read.
func epicBatchFlags(cmd *cobra.Command) (epicBatchScope, bool) {
	listed := cmd.Flags().Changed("milestones")
	all := cmd.Flags().Changed("all")
	if !listed && !all {
		return epicBatchScope{}, false
	}
	if listed && all {
		exitEpicBatchUsage("--milestones and --all exclude each other")
	}
	for _, name := range epicBatchRefused {
		if cmd.Flags().Changed(name) {
			exitEpicBatchUsage("--milestones and --all take no --" + name + ": a batch emits and reads slices, one-slice modes stay one slice at a time")
		}
	}
	milestones, _ := cmd.Flags().GetStringSlice("milestones")
	if listed && (len(milestones) == 0 || slices.Contains(milestones, "")) {
		exitEpicBatchUsage("--milestones needs at least one milestone and no empty one")
	}
	return epicBatchScope{milestones: milestones, all: all}, true
}

// exitEpicBatchUsage exits 2 with msg and the batch usage hint.
func exitEpicBatchUsage(msg string) {
	output.Error(ExitUsage, msg, epicBatchUsageHint)
	os.Exit(ExitUsage)
}

// runEpicReviewBatch runs the batch mode the flags name.
func runEpicReviewBatch(cmd *cobra.Command, scope epicBatchScope) {
	if report, _ := cmd.Flags().GetBool("report"); report {
		runEpicReviewReport(cmd, scope)
		return
	}
	if status, _ := cmd.Flags().GetBool("status"); status {
		check, _ := cmd.Flags().GetBool("check")
		runEpicReviewBatchStatus(scope, check)
		return
	}
	runEpicReviewBatchEmit(cmd, scope)
}

// epicBatchMilestones names the milestones a batch emission asks, each once:
// --milestones in the order given, or under --all every milestone the tracks
// hold, in track order.
func epicBatchMilestones(eg *epicReviewGraph, scope epicBatchScope) []string {
	names := scope.milestones
	if scope.all {
		names = slices.Concat(eg.g.Milestones...)
	}
	seen := map[string]bool{}
	out := make([]string, 0, len(names))
	for _, m := range names {
		if !seen[m] {
			seen[m] = true
			out = append(out, m)
		}
	}
	return out
}

// epicBatchSkip is one slice a batch emission did not emit, and why.
type epicBatchSkip struct {
	Key    engine.EpicSliceKeyRecord `json:"key"`
	Reason string                    `json:"reason"`
}

// epicBatchResult is a batch emission's payload: each emitted slice's round,
// as a one-slice emission prints it, and the slices skipped.
type epicBatchResult struct {
	Slices  []epicReviewResult `json:"slices"`
	Skipped []epicBatchSkip    `json:"skipped"`
}

// epicBatchItem is one slice of a batch emission, prepared and not yet written.
type epicBatchItem struct {
	run    epicReviewRun
	result epicReviewResult
}

// epicBatchSetup is what every slice of a batch emission shares.
type epicBatchSetup struct {
	graph  epicReviewGraph
	policy engine.EpicReviewPolicy
	roles  []model.Role
	query  roleQuery
}

// runEpicReviewBatchEmit emits each named milestone's round in one payload
// (spec/1.7.0.md §5). The graph is read once and every slice prepared before
// any slice writes state; a slice that is done, at its cap or failing is
// listed under skipped with its reason, and the rest still emit.
func runEpicReviewBatchEmit(cmd *cobra.Command, scope epicBatchScope) {
	setup := loadEpicBatchSetup(cmd)
	result := epicBatchResult{Slices: []epicReviewResult{}, Skipped: []epicBatchSkip{}}
	items := []epicBatchItem{}
	for _, m := range epicBatchMilestones(&setup.graph, scope) {
		key := engine.EpicSliceKey{Milestone: m}
		item, reason := prepareEpicBatchItem(&setup, key)
		if reason != "" {
			result.Skipped = append(result.Skipped, epicBatchSkip{Key: engine.EpicSliceKeyRecordOf(key), Reason: reason})
			continue
		}
		items = append(items, item)
	}
	for i := range items {
		if err := writeEpicReviewEmission(&items[i].run); err != nil {
			result.Skipped = append(result.Skipped, epicBatchSkip{
				Key: engine.EpicSliceKeyRecordOf(items[i].run.key), Reason: "cannot write the epic review state: " + err.Error(),
			})
			continue
		}
		result.Slices = append(result.Slices, items[i].result)
	}
	if err := output.JSON(result); err != nil {
		output.Error(ExitFile, err.Error(), internalEncodeHint)
		os.Exit(ExitFile)
	}
}

// loadEpicBatchSetup reads the graph, the review policy and the role corpus
// once, exiting as a one-slice emission does when one cannot be read.
func loadEpicBatchSetup(cmd *cobra.Command) epicBatchSetup {
	eg := loadEpicReviewGraph()
	roles, err := engine.ResolveEpicRoleCorpus(filepath.Dir(eg.tpDir))
	if err != nil {
		output.Error(ExitFile, err.Error(), "fix the role files under .tp/epic-reviewers/")
		os.Exit(ExitFile)
	}
	roleName, _ := cmd.Flags().GetString("role")
	refuseEpicRoleMissing(roleQuery{name: roleName, given: cmd.Flags().Changed("role")}, roles)
	return epicBatchSetup{
		graph:  eg,
		policy: epicReviewPolicy(eg.tpDir),
		roles:  roles,
		query:  roleQuery{name: roleName, given: cmd.Flags().Changed("role"), specDir: filepath.Dir(eg.tpDir)},
	}
}

// prepareEpicBatchItem prepares key's emission without writing it, or says
// why the slice cannot emit: it fails to prepare, its loop is done, or it is
// at its cap.
func prepareEpicBatchItem(setup *epicBatchSetup, key engine.EpicSliceKey) (item epicBatchItem, skipReason string) {
	run, rerr := prepareEpicReviewRun(&setup.graph, key)
	if rerr != nil {
		return epicBatchItem{}, rerr.msg
	}
	st, err := engine.EpicReviewLoopStatus(run.stateDir, run.all, setup.policy)
	if err != nil {
		return epicBatchItem{}, err.Error()
	}
	switch {
	case st.Done:
		return epicBatchItem{}, "the slice's loop is done by " + st.DoneBy + ": nothing to emit"
	case st.CapReached:
		msg, hint := epicCapRefusal(key, &st)
		return epicBatchItem{}, msg + ": " + hint
	}
	if rerr := readEpicReviewStates(&run); rerr != nil {
		return epicBatchItem{}, rerr.msg
	}
	built, skipped, rerr := buildEpicReviewPrompts(&run, setup.roles)
	if rerr != nil {
		return epicBatchItem{}, rerr.msg
	}
	prompts, skipped := filterByRole(built, func(p *epicReviewPrompt) string { return p.Role }, setup.query, skipped)
	result := epicReviewPayload(&run, setup.roles, prompts, len(built))
	rec := engine.EpicSliceKeyRecordOf(key)
	result.Key = &rec
	result.SkippedRoles = skippedRolesForPayload(setup.query.given, len(prompts), skipped)
	return epicBatchItem{run: run, result: result}, ""
}
