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
	requireResultsFileExists(filePath)

	var rows []map[string]any
	index := -1

	lockErr := engine.WithFileLock(filePath, func() error {
		var readErr error
		rows, readErr = readNDJSON(filePath)
		if readErr != nil {
			return readErr
		}

		var selErr string
		index, selErr = auditRowIndex(rows, selector)
		if selErr != "" {
			output.Error(ExitUsage, selErr)
			os.Exit(ExitUsage)
			return nil
		}

		row := rows[index]
		if existing, ok := row["resolved"]; ok && !force {
			output.Error(ExitValidation,
				fmt.Sprintf("row %s already resolved as %s", selector, dispositionStatusOf(existing)),
				"use --force to re-resolve")
			os.Exit(ExitValidation)
			return nil
		}

		row["resolved"] = disposition(status, evidence)
		return writeNDJSON(filePath, rows)
	})

	if lockErr != nil {
		exitResolveError(filePath, lockErr)
		return nil
	}

	fmt.Fprintf(os.Stderr, "resolved audit row %s as %s\n", selector, status)
	result := map[string]any{
		"index":    index,
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

// auditRowIndex maps a selector onto a row index, returning a usage message
// instead when it names no row. A selector carrying a colon is a `role:item_id`
// key — a role id never contains one, which is what lets the two forms be told
// apart without a flag — and anything else must be the 0-based index the review
// counterpart takes.
func auditRowIndex(rows []map[string]any, selector string) (index int, usageErr string) {
	if role, itemID, isKey := strings.Cut(selector, ":"); isKey {
		for i, row := range rows {
			if engine.AuditRowRole(row) != role {
				continue
			}
			if id, _ := row["item_id"].(string); id == itemID {
				return i, ""
			}
		}
		return -1, fmt.Sprintf("no row matches selector %q in the results file", selector)
	}

	index, err := strconv.Atoi(selector)
	if err != nil {
		return -1, fmt.Sprintf(
			"invalid selector %q: must be a 0-based integer or role:item_id; expected %s",
			selector, auditResolveUsageForm,
		)
	}
	if index < 0 || index >= len(rows) {
		return -1, fmt.Sprintf("row index %d out of range (0-%d)", index, len(rows)-1)
	}
	return index, ""
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
