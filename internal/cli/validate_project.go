package cli

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"

	"github.com/deligoez/tp/internal/engine"
	"github.com/deligoez/tp/internal/model"
	"github.com/deligoez/tp/internal/output"
)

// workflowDeviations reports each workflow field where a task file's override
// differs from a value the project config explicitly sets. A field the project
// does not set carries no policy and is not a deviation.
//
// notify_cmd is absent by construction: §7 reads it from .tp/local.json only,
// so neither layer compared here can carry it and it can never deviate.
func workflowDeviations(file string, override, project *model.WorkflowOverride) []map[string]any {
	devs := make([]map[string]any, 0)
	add := func(field, ov, pv string) {
		devs = append(devs, map[string]any{"file": file, "field": field, "override": ov, "project": pv})
	}
	cmpInt := func(field string, o, p *int) {
		if o != nil && p != nil && *o != *p {
			add(field, strconv.Itoa(*o), strconv.Itoa(*p))
		}
	}
	// Budget fields are decimal dollars; -1 precision prints the shortest form
	// that round-trips, so 2.5 reports as "2.5" rather than "2.500000".
	cmpFloat := func(field string, o, p *float64) {
		if o != nil && p != nil && *o != *p {
			add(field, strconv.FormatFloat(*o, 'f', -1, 64), strconv.FormatFloat(*p, 'f', -1, 64))
		}
	}
	cmpInt("gate_timeout_seconds", override.GateTimeoutSeconds, project.GateTimeoutSeconds)
	cmpInt("lock_timeout_seconds", override.LockTimeoutSeconds, project.LockTimeoutSeconds)
	cmpInt("review_clean_rounds", override.ReviewCleanRounds, project.ReviewCleanRounds)
	cmpInt("audit_clean_rounds", override.AuditCleanRounds, project.AuditCleanRounds)
	cmpInt("review_max_rounds", override.ReviewMaxRounds, project.ReviewMaxRounds)
	cmpInt("audit_max_rounds", override.AuditMaxRounds, project.AuditMaxRounds)
	cmpInt("run_max_units", override.RunMaxUnits, project.RunMaxUnits)
	cmpInt("run_max_wall_clock_seconds", override.RunMaxWallClockSeconds, project.RunMaxWallClockSeconds)
	cmpInt("run_max_unit_retries", override.RunMaxUnitRetries, project.RunMaxUnitRetries)
	cmpFloat("run_max_budget_usd", override.RunMaxBudgetUSD, project.RunMaxBudgetUSD)
	cmpFloat("run_max_unit_budget_usd", override.RunMaxUnitBudgetUSD, project.RunMaxUnitBudgetUSD)
	if override.QualityGate != nil && project.QualityGate != nil && *override.QualityGate != *project.QualityGate {
		add("quality_gate", *override.QualityGate, *project.QualityGate)
	}
	// commit_strategy resolves through the project layer like any other field:
	// .tp/config.json sets the project default and tp init authors the
	// task-file value, so the two can genuinely contradict each other.
	if override.CommitStrategy != nil && project.CommitStrategy != nil && *override.CommitStrategy != *project.CommitStrategy {
		add("commit_strategy", *override.CommitStrategy, *project.CommitStrategy)
	}
	if override.ReviewConvergeOn != nil && project.ReviewConvergeOn != nil && *override.ReviewConvergeOn != *project.ReviewConvergeOn {
		add("review_converge_on", *override.ReviewConvergeOn, *project.ReviewConvergeOn)
	}
	// audit_converge_on is compared on its twin's terms, and only compared:
	// v0.37.0 §2 places the refusal of an illegal literal at the write sinks
	// (exit 2) and of an illegal stored value at the consuming audit sinks
	// (exit 1), so this surface reports a task file that contradicts the
	// project policy and lets --strict promote that report to exit 1. An
	// illegal value is reported here as the deviation it is rather than graded.
	if override.AuditConvergeOn != nil && project.AuditConvergeOn != nil && *override.AuditConvergeOn != *project.AuditConvergeOn {
		add("audit_converge_on", *override.AuditConvergeOn, *project.AuditConvergeOn)
	}
	// runner is compared as raw JSON bytes for the same reason --extract hoists
	// it that way: this surface reports the field, it does not interpret it.
	if override.Runner != nil && project.Runner != nil && !bytes.Equal(override.Runner, project.Runner) {
		add("runner", string(override.Runner), string(project.Runner))
	}
	if override.Checks != nil && project.Checks != nil && !checksEqual(*override.Checks, *project.Checks) {
		add("checks",
			fmt.Sprintf("%d entries", len(*override.Checks)),
			fmt.Sprintf("%d entries", len(*project.Checks)))
	}
	return devs
}

// checksEqual reports whether two checks arrays are equal as sets of
// {class, cmd} pairs, so a reordered but otherwise equal checks is not a
// deviation.
func checksEqual(a, b []model.Check) bool {
	if len(a) != len(b) {
		return false
	}
	set := make(map[model.Check]int, len(a))
	for _, c := range a {
		set[c]++
	}
	for _, c := range b {
		set[c]--
		if set[c] < 0 {
			return false
		}
	}
	return true
}

// runValidateProject implements `tp validate --project`: it reports each task
// file's workflow-field deviations from the project config. Deviations are
// informational (exit 0) unless --strict promotes them to exit 1.
func runValidateProject() error {
	surfaceConfigWarnings()
	tpDir := engine.DiscoverTPDir(".")
	root := engine.ProjectRoot(".")
	configPath := filepath.Join(tpDir, "config.json")

	present := false
	var statErr error
	if tpDir != "" {
		present, statErr = fileExists(configPath)
	}
	if statErr != nil {
		// Not "there is no project config": there is no way to tell. Reporting
		// the absent form would answer a question this run could not ask, and
		// --strict would then pass a gate over a policy it never read.
		loc := relOrSelf(root, configPath)
		output.Notice(fmt.Sprintf("warning: project config unreadable at %s: %v", loc, statErr))
		return output.JSON(map[string]any{
			"project_config": false,
			"deviations":     []any{},
			"skipped":        []string{loc},
		})
	}
	if !present {
		output.Info("no project config found (.tp/config.json)")
		return output.JSON(map[string]any{"project_config": false, "deviations": []any{}})
	}

	project := engine.ProjectWorkflowOverride()
	files, scanErr := engine.ScanProjectTaskFiles(root)

	deviations := make([]map[string]any, 0)
	skipped := make([]string, 0)
	// A config that does not parse sets no field, so every deviation silently
	// becomes a non-deviation. surfaceConfigWarnings has already said so on
	// stderr, which --quiet erases; this is the same fact in the payload a
	// driver actually branches on.
	if _, _, cfgErr := engine.LoadProjectConfig(tpDir); cfgErr != nil {
		skipped = append(skipped, relOrSelf(root, configPath))
	}
	if scanErr != nil {
		// The walk stops at the first directory it cannot read, so files holds
		// only what was reached before it. Report the gap on the same channel a
		// malformed task file uses: an incomplete scan that prints an empty
		// deviation list is indistinguishable from a genuinely clean project.
		loc := root
		var pathErr *fs.PathError
		if errors.As(scanErr, &pathErr) {
			loc = pathErr.Path
		}
		loc = relOrSelf(root, loc)
		skipped = append(skipped, loc)
		output.Notice(fmt.Sprintf("warning: project scan incomplete at %s: %v", loc, scanErr))
	}
	for _, f := range files {
		rel := relOrSelf(root, f)
		override, err := engine.LoadTaskWorkflowOverride(f)
		if err != nil {
			skipped = append(skipped, rel)
			output.Notice(fmt.Sprintf("warning: skipping malformed task file %s: %v", rel, err))
			continue
		}
		deviations = append(deviations, workflowDeviations(rel, &override, &project)...)
	}

	result := map[string]any{"project_config": true, "deviations": deviations}
	if len(skipped) > 0 {
		result["skipped"] = skipped
	}
	if validateStrict && len(deviations) > 0 {
		_ = output.JSON(result)
		os.Exit(ExitValidation)
	}
	return output.JSON(result)
}

// relOrSelf abbreviates p relative to root, falling back to p itself when the
// two share no common base. A path is always reported, never silently dropped.
func relOrSelf(root, p string) string {
	rel, err := filepath.Rel(root, p)
	if err != nil {
		return p
	}
	return rel
}

// fileExists reports whether p is present. A stat error that is not "does not
// exist" answers a different question than the caller asked, so it is returned
// rather than folded into false: read as absence it makes an unreadable file
// look like a missing one, and both callers act on that difference.
func fileExists(p string) (bool, error) {
	_, err := os.Stat(p)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	return false, err
}
