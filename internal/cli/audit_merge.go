package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"sort"
	"strings"

	"github.com/deligoez/tp/internal/engine"
	"github.com/deligoez/tp/internal/output"
)

// runAuditMerge merges and deduplicates audit-result NDJSON files (one row per
// checklist item). It validates each row (item_id + status required), drops exact
// (role, item_id) duplicates, sorts deterministically, writes the merged NDJSON,
// and reports a status/role breakdown. Mirrors tp review --merge for the audit
// phase, replacing the manual concatenation of per-role result files.
func runAuditMerge(args []string, outputPath string) error {
	if len(args) == 0 {
		output.Error(ExitUsage, "at least 1 file required for merge")
		os.Exit(ExitUsage)
		return nil
	}

	// §4.1: --merge takes only its explicit NDJSON input files; a spec-looking
	// positional among them is rejected at entry (exit 2) rather than silently
	// parsed as data.
	for _, path := range args {
		if isSpecLookingPath(path) {
			output.Error(ExitUsage, fmt.Sprintf(
				"%s looks like a spec; --merge takes NDJSON input files only: tp audit --merge <a.ndjson> [<b.ndjson> ...]",
				path,
			))
			os.Exit(ExitUsage)
			return nil
		}
	}

	totalFiles := len(args)
	rows, inputs := loadMergeRows(args, auditMergeRule)
	// §8a.4: the same rule as the review merge — an input whose every content
	// line was skipped drops a whole role, and an unattended driver reads only
	// the exit code.
	dropped := droppedInputs(inputs)
	unique, conflicts := dedupAuditRows(rows)

	var buf strings.Builder
	for _, r := range unique {
		line, err := json.Marshal(r)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: dropped unmarshalable merged row (role=%v item_id=%v): %v\n", r["role"], r["item_id"], err)
			continue
		}
		buf.Write(line)
		buf.WriteByte('\n')
	}
	ndjson := buf.String()
	duplicatesRemoved := len(rows) - len(unique)

	byStatus := make(map[string]int)
	byRole := make(map[string]map[string]int)
	// §4: the severity breakdown of the round's non-PASS rows, built in the
	// same loop as by_status and by_role. Every such row lands in one of four
	// named buckets — engine.AuditSeverityBucket is the classifier §2's clean
	// predicate grades on, called rather than copied, so error + unrecognised
	// is exactly the round's blocking-row count and the two sections cannot
	// drift.
	bySeverity := make(map[string]int)
	findingsCount := 0
	for _, r := range unique {
		status, _ := r["status"].(string)
		byStatus[status]++
		if status != "PASS" {
			findingsCount++
			bySeverity[engine.AuditSeverityBucket(r)]++
		}
		if role, _ := r["role"].(string); role != "" {
			if byRole[role] == nil {
				byRole[role] = make(map[string]int)
			}
			byRole[role][status]++
		}
	}

	summary := map[string]any{
		"merged_count":       len(unique),
		"input_files":        totalFiles,
		"duplicates_removed": duplicatesRemoved,
		"by_status":          byStatus,
		"by_role":            byRole,
		"findings":           findingsCount, // rows whose status is not PASS
		"inputs":             inputs,
	}
	// §4: emitted when the round holds at least one non-PASS row, and absent —
	// not empty — otherwise. The condition is a property of the rows and
	// nothing else, deliberately not of the resolved audit_converge_on: --merge
	// takes NDJSON inputs and rejects a spec-looking positional at entry, so it
	// has no spec path, engine.ResolveWorkflow cannot reach the task-override
	// layer from here, and the only substitute would be the active pointer —
	// under which the key would appear or vanish according to .tp/local.json.
	// It is decision-critical, so unlike overlap_report it survives --compact.
	if findingsCount > 0 {
		summary["by_severity"] = bySeverity
	}
	// Absent, not empty, when every key holds one verdict: it is a warning,
	// and the rows are all in the output either way.
	if len(conflicts) > 0 {
		summary["conflicts"] = conflicts
		fmt.Fprintf(os.Stderr, "warning: %d (role, item_id) groups carry disagreeing verdicts; every row is kept (see conflicts)\n", len(conflicts))
	}
	// §9.3 / §8.4: the audit overlap_report gives a trim-candidate signal over
	// non-PASS rows clustered by (item_id, category); it is explanatory and is
	// omitted under --compact.
	if !IsCompact() {
		summary["overlap_report"] = computeAuditOverlapReport(unique)
	}

	if outputPath != "" {
		// §5 row 10, as the review merge applies it: a merge about to exit
		// non-zero writes nothing at `-o`, because the zero-byte file it used
		// to leave chained into `tp audit <spec> --record` as a clean round.
		// output_path is named only when a file is there to name.
		if writeMergeOutput(outputPath, ndjson, dropped) {
			summary["output_path"] = outputPath
		}
		return finishMerge(output.JSON(summary), dropped, auditMergeRule)
	}

	if IsJSONOutput() {
		summary["output_path"] = "stdout"
		summary["rows"] = unique
		return finishMerge(output.JSON(summary), dropped, auditMergeRule)
	}

	fmt.Print(ndjson)
	fmt.Fprintf(os.Stderr, "merged: %d rows from %d files (%d duplicates removed, %d non-PASS)\n",
		len(unique), totalFiles, duplicatesRemoved, findingsCount)
	return finishMerge(nil, dropped, auditMergeRule)
}

// auditRequiredFields is what every audit-result row must carry, each as a
// non-empty string, for `tp audit --merge` to keep it.
var auditRequiredFields = []string{"item_id", "status"}

// missingAuditFields returns the required keys an audit row lacks, leaves
// empty, or holds as a non-string. Unlike the review predicate it does not
// trim: a whitespace-only value is kept, as it always has been.
func missingAuditFields(row map[string]any) []string {
	missing := make([]string, 0, len(auditRequiredFields))
	for _, k := range auditRequiredFields {
		if s, _ := row[k].(string); s == "" {
			missing = append(missing, k)
		}
	}
	return missing
}

// auditMergeRule is the row rule `tp audit --merge` loads its inputs by.
var auditMergeRule = mergeRowRule{
	noun:     "audit row",
	required: auditRequiredFields,
	missing:  missingAuditFields,
}

// auditConflict names one (role, item_id) whose rows carry disagreeing
// verdicts: every such row is kept, and the group is reported so the caller
// sees that one item holds more than one verdict.
type auditConflict struct {
	Role     string   `json:"role"`
	ItemID   string   `json:"item_id"`
	Rows     int      `json:"rows"`
	Statuses []string `json:"statuses"`
}

// dedupAuditRows collapses (role, item_id) duplicates whose verdict agrees
// (auditVerdictKey), keeping the first, and keeps every row whose verdict
// differs from the rows kept before it. Keeping only the first row of a key
// chose, silently, which of two verdicts on one item was the round's: a second
// shard's error-severity FAIL vanished behind the first shard's PASS and the
// round recorded clean. It returns the rows sorted by (role, item_id), input
// order kept within a key, and one auditConflict per key holding more than one
// row, in that order.
func dedupAuditRows(rows []map[string]any) ([]map[string]any, []auditConflict) {
	kept := make(map[string][]string)
	unique := make([]map[string]any, 0, len(rows))
	for _, r := range rows {
		key := auditRowKey(r)
		verdict := auditVerdictKey(r)
		if slices.Contains(kept[key], verdict) {
			continue
		}
		kept[key] = append(kept[key], verdict)
		unique = append(unique, r)
	}
	sort.SliceStable(unique, func(i, j int) bool {
		ri, _ := unique[i]["role"].(string)
		rj, _ := unique[j]["role"].(string)
		if ri != rj {
			return ri < rj
		}
		ii, _ := unique[i]["item_id"].(string)
		ij, _ := unique[j]["item_id"].(string)
		return ii < ij
	})
	return unique, auditConflicts(unique, kept)
}

// auditConflicts lists the keys of sorted rows that kept more than one row.
func auditConflicts(sorted []map[string]any, kept map[string][]string) []auditConflict {
	conflicts := make([]auditConflict, 0)
	for _, r := range sorted {
		key := auditRowKey(r)
		if len(kept[key]) < 2 {
			continue
		}
		role, _ := r["role"].(string)
		itemID, _ := r["item_id"].(string)
		status, _ := r["status"].(string)
		if n := len(conflicts); n > 0 && conflicts[n-1].Role == role && conflicts[n-1].ItemID == itemID {
			conflicts[n-1].Rows++
			conflicts[n-1].Statuses = append(conflicts[n-1].Statuses, status)
			continue
		}
		conflicts = append(conflicts, auditConflict{Role: role, ItemID: itemID, Rows: 1, Statuses: []string{status}})
	}
	return conflicts
}

// auditRowKey is the (role, item_id) an audit row answers.
func auditRowKey(r map[string]any) string {
	role, _ := r["role"].(string)
	itemID, _ := r["item_id"].(string)
	return role + "\x00" + itemID
}

// auditVerdictKey is what two rows on one item must share to be one verdict:
// the status, the severity and the disposition's status. Notes and evidence
// may differ between two agreeing rows.
func auditVerdictKey(r map[string]any) string {
	status, _ := r["status"].(string)
	severity, _ := r["severity"].(string)
	disposition := ""
	if resolved, ok := r["resolved"].(map[string]any); ok {
		disposition, _ = resolved["status"].(string)
	}
	return status + "\x00" + severity + "\x00" + disposition
}
