package cli

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/deligoez/tp/internal/engine"
	"github.com/deligoez/tp/internal/output"
)

// epicResolveUsageForm / epicResolveAllUsageForm name the positional shape of
// tp epic review's dispositions, which write the slice's last recorded round.
const epicResolveUsageForm = "tp epic review <slice> --resolve <0-based index>[,<index>...] <fixed|wontfix|duplicate> [evidence]"

const epicResolveAllUsageForm = "tp epic review <slice> --resolve-all <fixed|wontfix|duplicate> [evidence]"

// epicResolveResult is the payload of tp epic review --resolve and --resolve-all.
type epicResolveResult struct {
	StateDir      string `json:"state_dir"`
	Round         int    `json:"round"`
	Index         *int   `json:"index,omitempty"`
	Indices       []int  `json:"indices,omitempty"`
	ResolvedCount *int   `json:"resolved_count,omitempty"`
	SkippedCount  *int   `json:"skipped_count,omitempty"`
	Status        string `json:"status"`
	NextAction    string `json:"next_action"`
}

// parseEpicResolveIndices reads one 0-based index or a comma-separated list
// of them, exiting 2 on anything else or on an index named twice.
func parseEpicResolveIndices(arg string) []int {
	parts := strings.Split(arg, ",")
	indices := make([]int, 0, len(parts))
	seen := map[int]bool{}
	for _, p := range parts {
		index, err := strconv.Atoi(p)
		if err != nil || seen[index] {
			output.Error(ExitUsage, fmt.Sprintf("invalid index %q: must be 0-based integers, comma-separated and each once; expected %s", arg, epicResolveUsageForm))
			os.Exit(ExitUsage)
		}
		seen[index] = true
		indices = append(indices, index)
	}
	return indices
}

// runEpicReviewResolve dispositions the listed findings of the slice's last
// recorded round, as tp review --resolve does a findings file. args: indices,
// status, evidence?. Every index is checked before any row is written, so one
// out of range, or one already resolved without force, changes no row.
func runEpicReviewResolve(key engine.EpicSliceKey, args []string, force bool) {
	if len(args) < 2 || len(args) > 3 {
		output.Error(ExitUsage, "usage: "+epicResolveUsageForm)
		os.Exit(ExitUsage)
	}
	indices := parseEpicResolveIndices(args[0])
	status, evidence := epicResolveDisposition(args[1:])
	run, path, round := epicResolveTarget(key)
	lockErr := engine.WithFileLock(path, func() error {
		findings, readErr := readNDJSON(path)
		if readErr != nil {
			return readErr
		}
		selected := make([]map[string]any, 0, len(indices))
		for _, index := range indices {
			if index < 0 || index >= len(findings) {
				output.Error(ExitUsage, fmt.Sprintf("finding index %d out of range (0-%d) in round %d", index, len(findings)-1, round))
				os.Exit(ExitUsage)
			}
			epicRefuseResolved(findings[index], index, force)
			selected = append(selected, findings[index])
		}
		fenceReviewAcceptance(status, selected)
		at := time.Now().UTC().Format(time.RFC3339)
		for _, row := range selected {
			row["resolved"] = map[string]any{"status": status, "evidence": evidence, "resolved_at": at}
		}
		return writeNDJSON(path, findings)
	})
	if lockErr != nil {
		exitEpicResolveError(path, lockErr)
	}
	result := epicResolvePayload(&run, round, status)
	if len(indices) == 1 {
		result.Index = &indices[0]
	} else {
		result.Indices = indices
	}
	printEpicResolve(&result)
}

// runEpicReviewResolveAll dispositions every undisposed finding of the slice's
// last recorded round, or every finding under force. args: status, evidence?.
func runEpicReviewResolveAll(key engine.EpicSliceKey, args []string, force bool) {
	if len(args) < 1 || len(args) > 2 {
		output.Error(ExitUsage, "usage: "+epicResolveAllUsageForm)
		os.Exit(ExitUsage)
	}
	status, evidence := epicResolveDisposition(args)
	run, path, round := epicResolveTarget(key)
	var resolved, skipped int
	lockErr := engine.WithFileLock(path, func() error {
		findings, readErr := readNDJSON(path)
		if readErr != nil {
			return readErr
		}
		resolved, skipped = resolveEveryFinding(findings, status, evidence, force)
		return writeNDJSON(path, findings)
	})
	if lockErr != nil {
		exitEpicResolveError(path, lockErr)
	}
	result := epicResolvePayload(&run, round, status)
	result.ResolvedCount, result.SkippedCount = &resolved, &skipped
	printEpicResolve(&result)
}

// epicResolveDisposition validates status and its evidence from args
// (status, evidence?), exiting 2 on an unknown status or a wontfix or
// duplicate without evidence.
func epicResolveDisposition(args []string) (status, evidence string) {
	status = args[0]
	if len(args) > 1 {
		evidence = args[1]
	}
	if !validResolveStatuses[status] {
		output.Error(ExitUsage, fmt.Sprintf("invalid status: %s (must be fixed, wontfix, or duplicate)", status))
		os.Exit(ExitUsage)
	}
	requireAcceptanceEvidence(status, evidence)
	return status, evidence
}

// epicResolveTarget loads the slice and names its last recorded round file;
// a slice with no recorded round exits 3, since there is nothing to resolve.
func epicResolveTarget(key engine.EpicSliceKey) (run epicReviewRun, path string, round int) {
	run = loadEpicReviewRun(key)
	path, round = engine.EpicReviewLastRoundPath(run.stateDir)
	if round == 0 {
		output.Error(ExitFile, "no recorded round to resolve for "+epicSliceLabel(key),
			fmt.Sprintf("record a round first with tp epic review %s --record <file>", epicSliceLabel(key)))
		os.Exit(ExitFile)
	}
	return run, path, round
}

// epicRefuseResolved exits 1 when the finding already carries a disposition
// and force is not set, as tp review --resolve does.
func epicRefuseResolved(finding map[string]any, index int, force bool) {
	existing, ok := finding["resolved"]
	if !ok || force {
		return
	}
	current := "unknown"
	if rm, isMap := existing.(map[string]any); isMap {
		if s, isString := rm["status"].(string); isString {
			current = s
		}
	}
	output.Error(ExitValidation, fmt.Sprintf("finding %d already resolved as %s", index, current), "use --force to re-resolve")
	os.Exit(ExitValidation)
}

// epicResolvePayload starts the payload both disposition modes print.
func epicResolvePayload(run *epicReviewRun, round int, status string) epicResolveResult {
	return epicResolveResult{
		StateDir: epicReviewStateDirOut(run),
		Round:    round,
		Status:   status,
		NextAction: fmt.Sprintf("read the slice's loop with tp epic review %s --status, or emit the next round with tp epic review %s",
			epicSliceLabel(run.key), epicSliceLabel(run.key)),
	}
}

// printEpicResolve writes the payload as JSON.
func printEpicResolve(result *epicResolveResult) {
	if err := output.JSON(result); err != nil {
		output.Error(ExitFile, err.Error(), internalEncodeHint)
		os.Exit(ExitFile)
	}
}

// exitEpicResolveError is exitResolveError for the slice's round file: lock
// contention keeps its exit 4, but any other failure is about tp's own
// recorded round, not a path the caller passed, so it carries epicRoundFileHint.
func exitEpicResolveError(path string, err error) {
	var lockErr *engine.LockTimeoutError
	if errors.As(err, &lockErr) {
		exitResolveError(path, err)
		return
	}
	output.Error(ExitFile, fmt.Sprintf("cannot update %s: %v", path, err), epicRoundFileHint)
	os.Exit(ExitFile)
}
