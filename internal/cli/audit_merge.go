package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
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
	rows, inputs := loadAuditMergeRows(args)
	// §8a.4: the same rule as the review merge — an input whose every content
	// line was skipped drops a whole role, and an unattended driver reads only
	// the exit code.
	dropped := droppedInputs(inputs)
	unique := dedupAuditRows(rows)

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
	// §9.3 / §8.4: the audit overlap_report gives a trim-candidate signal over
	// non-PASS rows clustered by (item_id, category); it is explanatory and is
	// omitted under --compact.
	if !IsCompact() {
		summary["overlap_report"] = computeAuditOverlapReport(unique)
	}

	if outputPath != "" {
		if err := os.WriteFile(outputPath, []byte(ndjson), 0o600); err != nil {
			output.Error(ExitFile, fmt.Sprintf("cannot write output file: %s", err), outputFileHint)
			os.Exit(ExitFile)
			return nil
		}
		summary["output_path"] = outputPath
		return finishMerge(output.JSON(summary), dropped)
	}

	if IsJSONOutput() {
		summary["output_path"] = "stdout"
		summary["rows"] = unique
		return finishMerge(output.JSON(summary), dropped)
	}

	fmt.Print(ndjson)
	fmt.Fprintf(os.Stderr, "merged: %d rows from %d files (%d duplicates removed, %d non-PASS)\n",
		len(unique), totalFiles, duplicatesRemoved, findingsCount)
	return finishMerge(nil, dropped)
}

// loadAuditMergeRows reads and validates audit-result rows from the input files,
// skipping blank, malformed (invalid JSON), and incomplete (missing item_id or
// status) lines with a stderr warning that names which. It aborts only on a
// missing/unreadable file (exit 3), and returns the §8a.4 per-input accounting
// beside the rows: blank lines count as neither, so an all-empty set of inputs
// is a valid clean result and yields zero rows without failing. An input whose
// content lines all fail is a dropped role, which runAuditMerge turns into
// exit 1.
func loadAuditMergeRows(args []string) ([]map[string]any, []mergeInputCounts) {
	for _, path := range args {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			output.Error(ExitFile, fmt.Sprintf("file not found: %s", path), ndjsonInputFileHint)
			os.Exit(ExitFile)
		}
	}

	rows := make([]map[string]any, 0)
	inputs := make([]mergeInputCounts, 0, len(args))
	for _, path := range args {
		f, err := os.Open(path)
		if err != nil {
			output.Error(ExitFile, fmt.Sprintf("cannot open file: %s", path), ndjsonInputFileHint)
			os.Exit(ExitFile)
		}
		counts := mergeInputCounts{Path: path}
		scanner := bufio.NewScanner(f)
		scanner.Buffer(make([]byte, 0, 64*1024), ndjsonLineCap) // audit notes can be long
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" {
				continue
			}
			var row map[string]any
			if err := json.Unmarshal([]byte(line), &row); err != nil {
				fmt.Fprintf(os.Stderr, "warning: skipping malformed line (invalid JSON) in %s\n", path)
				counts.Skipped++
				continue
			}
			itemID, idOK := row["item_id"].(string)
			status, stOK := row["status"].(string)
			if !idOK || !stOK || itemID == "" || status == "" {
				var missing []string
				if !idOK || itemID == "" {
					missing = append(missing, "item_id")
				}
				if !stOK || status == "" {
					missing = append(missing, "status")
				}
				fmt.Fprintf(os.Stderr, "warning: skipping incomplete line (missing %s) in %s\n", strings.Join(missing, ", "), path)
				counts.Skipped++
				continue
			}
			counts.Parsed++
			rows = append(rows, row)
		}
		if err := scanner.Err(); err != nil {
			// Aborting, not warning: a read that failed produces zero rows, and
			// zero rows is what a genuinely clean round looks like — so a
			// swallowed error here lets an input tp never read record a clean
			// round. The old warning also named one cause (an over-long line)
			// for every failure, including reading a directory.
			f.Close()
			output.Error(ExitFile, fmt.Sprintf("cannot read %s: %v", path, err), ndjsonReadHint(err))
			os.Exit(ExitFile)
		}
		f.Close()
		inputs = append(inputs, counts)
	}

	return rows, inputs
}

// dedupAuditRows drops exact (role, item_id) duplicates, keeping the first
// occurrence, and returns the rows sorted by (role, item_id) for deterministic
// output.
func dedupAuditRows(rows []map[string]any) []map[string]any {
	seen := make(map[string]bool)
	unique := make([]map[string]any, 0, len(rows))
	for _, r := range rows {
		role, _ := r["role"].(string)
		itemID, _ := r["item_id"].(string)
		key := role + "\x00" + itemID
		if seen[key] {
			continue
		}
		seen[key] = true
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
	return unique
}
