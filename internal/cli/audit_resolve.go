package cli

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/deligoez/tp/internal/engine"
	"github.com/deligoez/tp/internal/output"
)

// auditResolveUsageForm / auditResolveAllUsageForm name the expected positional
// shape of the audit-side mirror of tp review --resolve (§3.3). The dispositions
// and the --force / --resolve-all flags are the review counterparts' byte for
// byte; the one addition is the selector, which is a 0-based index or the
// `role:item_id` key the oracle hands an audit-fix unit — so a unit can name its
// own row without first locating an index.
const auditResolveUsageForm = "tp audit <results.ndjson> --resolve <0-based index|role:item_id> <fixed|wontfix|duplicate> [evidence]"

const auditResolveAllUsageForm = "tp audit <results.ndjson> --resolve-all <fixed|wontfix|duplicate> [evidence]"

// runAuditResolve disposes one audit row in the round's results file.
// args: [file, selector, status, evidence?]
//
// The results file is the one the round's audit-record unit wrote,
// $TP_ROUND_DIR/merged.ndjson, which is also what that unit passes to
// tp audit --record. Disposing a row is the whole durable write of an audit-fix
// unit: a finding correctly closed with no code change at all satisfies §3.3's
// predicate, which is why the kind needs this command to exist.
func runAuditResolve(args []string, force bool) error {
	if len(args) < 3 {
		output.Error(ExitUsage, "usage: "+auditResolveUsageForm)
		os.Exit(ExitUsage)
		return nil
	}

	filePath := args[0]
	selector := args[1]
	status := args[2]
	evidence := ""
	if len(args) >= 4 {
		evidence = args[3]
	}

	requireResultsPositional(filePath, "--resolve", auditResolveUsageForm)
	requireResolveStatus(status)
	requireAcceptanceEvidence(status, evidence)
	fenceAuditAcceptance(status)
	requireResultsFileExists(filePath)

	var rows []map[string]any
	var indices []int
	disposed := 0

	lockErr := engine.WithFileLock(filePath, func() error {
		var readErr error
		rows, readErr = readNDJSON(filePath)
		if readErr != nil {
			return readErr
		}

		var selErr string
		indices, selErr = auditRowIndices(rows, selector)
		if selErr != "" {
			output.Error(ExitUsage, selErr)
			os.Exit(ExitUsage)
			return nil
		}

		disposed = disposeSelectedRows(rows, indices, selector, status, evidence, force)
		return writeNDJSON(filePath, rows)
	})

	if lockErr != nil {
		exitResolveError(filePath, lockErr)
		return nil
	}

	fmt.Fprintf(os.Stderr, "resolved audit row %s as %s\n", selector, status)
	result := map[string]any{
		"index":    indices[0],
		"disposed": disposed,
		"selector": selector,
		"status":   status,
		"evidence": evidence,
		"file":     filePath,
	}
	if allFindingsResolved(rows) {
		result["next_step"] = auditResolveNextStep(filePath)
	}
	return output.JSON(result)
}

// runAuditResolveAll disposes every undisposed audit row, skipping the ones
// already carrying a disposition unless --force is given — tp review
// --resolve-all's behaviour on the audit artifact.
// args: [file, status, evidence?]
func runAuditResolveAll(args []string, force bool) error {
	if len(args) < 2 {
		output.Error(ExitUsage, "usage: "+auditResolveAllUsageForm)
		os.Exit(ExitUsage)
		return nil
	}

	filePath := args[0]
	status := args[1]
	evidence := ""
	if len(args) >= 3 {
		evidence = args[2]
	}

	requireResultsPositional(filePath, "--resolve-all", auditResolveAllUsageForm)
	requireResolveStatus(status)
	requireAcceptanceEvidence(status, evidence)
	fenceAuditAcceptance(status)
	requireResultsFileExists(filePath)

	resolvedCount := 0
	skippedCount := 0

	lockErr := engine.WithFileLock(filePath, func() error {
		rows, readErr := readNDJSON(filePath)
		if readErr != nil {
			return readErr
		}

		for _, row := range rows {
			if _, ok := row["resolved"]; ok && !force {
				skippedCount++
				continue
			}
			row["resolved"] = disposition(status, evidence)
			resolvedCount++
		}

		return writeNDJSON(filePath, rows)
	})

	if lockErr != nil {
		exitResolveError(filePath, lockErr)
		return nil
	}

	fmt.Fprintf(os.Stderr, "resolved %d audit rows as %s (%d already resolved, skipped)\n", resolvedCount, status, skippedCount)
	return output.JSON(map[string]any{
		"resolved_count": resolvedCount,
		"skipped_count":  skippedCount,
		"status":         status,
		"file":           filePath,
		"next_step":      auditResolveNextStep(filePath),
	})
}

// auditRowIndices maps a selector onto the rows it names, returning a usage
// message instead when it names none. A selector carrying a colon is a
// `role:item_id` key (a role id never contains one, which is what lets the two
// forms be told apart without a flag) and names every non-PASS row under the
// key: --merge keeps disagreeing verdicts on one item, so a key can hold a
// finding from each of two shards, and disposing only the first left the
// other open. A PASS row is not a finding, so a key holding only PASS rows is
// refused. Anything else must be the 0-based index the review counterpart
// takes, naming one row.
func auditRowIndices(rows []map[string]any, selector string) (indices []int, usageErr string) {
	if role, itemID, isKey := strings.Cut(selector, ":"); isKey {
		matched := false
		for i, row := range rows {
			if engine.AuditRowRole(row) != role {
				continue
			}
			if id, _ := row["item_id"].(string); id != itemID {
				continue
			}
			matched = true
			if !engine.AuditRowIsPass(row) {
				indices = append(indices, i)
			}
		}
		if len(indices) > 0 {
			return indices, ""
		}
		if matched {
			return nil, fmt.Sprintf("every row under %q is PASS; a PASS row is not a finding and takes no disposition", selector)
		}
		return nil, fmt.Sprintf("no row matches selector %q in the results file", selector)
	}

	index, err := strconv.Atoi(selector)
	if err != nil {
		return nil, fmt.Sprintf(
			"invalid selector %q: must be a 0-based integer or role:item_id; expected %s",
			selector, auditResolveUsageForm,
		)
	}
	if index < 0 || index >= len(rows) {
		return nil, fmt.Sprintf("row index %d out of range (0-%d)", index, len(rows)-1)
	}
	return []int{index}, ""
}

// disposeSelectedRows writes the disposition onto every selected row that
// carries none (every selected row under force) and returns how many it
// wrote. When every selected row is already disposed and force is off it
// exits 1, naming the first row's disposition.
func disposeSelectedRows(rows []map[string]any, indices []int, selector, status, evidence string, force bool) int {
	disposed := 0
	for _, i := range indices {
		if _, ok := rows[i]["resolved"]; ok && !force {
			continue
		}
		rows[i]["resolved"] = disposition(status, evidence)
		disposed++
	}
	if disposed == 0 {
		output.Error(ExitValidation,
			fmt.Sprintf("row %s already resolved as %s", selector, dispositionStatusOf(rows[indices[0]]["resolved"])),
			"use --force to re-resolve")
		os.Exit(ExitValidation)
	}
	return disposed
}

// disposition builds the `resolved` object both resolve paths write. It is the
// object shape tp review --resolve writes, so the durable-write predicates read
// one form whichever side produced it.
func disposition(status, evidence string) map[string]any {
	return map[string]any{
		"status":      status,
		"evidence":    evidence,
		"resolved_at": time.Now().UTC().Format(time.RFC3339),
	}
}

// dispositionStatusOf reads the status out of an existing disposition for the
// already-resolved refusal, reporting "unknown" for anything it cannot read.
func dispositionStatusOf(existing any) string {
	if m, ok := existing.(map[string]any); ok {
		if s, ok := m["status"].(string); ok {
			return s
		}
	}
	return "unknown"
}

// requireResultsPositional rejects a spec-looking positional where the results
// NDJSON is expected (§4.1), naming the expected form rather than reading the
// selector out of the following argument.
func requireResultsPositional(path, flag, form string) {
	if isSpecLookingPath(path) {
		output.Error(ExitUsage, fmt.Sprintf(
			"%s looks like a spec; %s takes the audit results NDJSON as the positional: %s",
			path, flag, form,
		))
		os.Exit(ExitUsage)
	}
}

// requireResolveStatus rejects a disposition outside the three tp review
// --resolve accepts.
func requireResolveStatus(status string) {
	if !validResolveStatuses[status] {
		output.Error(ExitUsage, fmt.Sprintf("invalid status: %s (must be fixed, wontfix, or duplicate)", status))
		os.Exit(ExitUsage)
	}
}

// requireResultsFileExists reports a missing results file as a file error, the
// way the review counterpart reports a missing findings file.
func requireResultsFileExists(path string) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		output.Error(ExitFile, fmt.Sprintf("audit results file not found: %s", path), ndjsonInputFileHint)
		os.Exit(ExitFile)
	}
}

// auditResolveNextStep names what the round's results file is for once every row
// carries a disposition: the audit-record unit re-records the round from it
// (§6.3), which is why a re-run record unit merges nothing over it.
func auditResolveNextStep(path string) string {
	return fmt.Sprintf("tp audit <spec> --record %s", path)
}
