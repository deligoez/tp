package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

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
	rows := loadAuditMergeRows(args)
	unique := dedupAuditRows(rows)

	var buf strings.Builder
	for _, r := range unique {
		line, err := json.Marshal(r)
		if err != nil {
			continue
		}
		buf.Write(line)
		buf.WriteByte('\n')
	}
	ndjson := buf.String()
	duplicatesRemoved := len(rows) - len(unique)

	byStatus := make(map[string]int)
	byRole := make(map[string]map[string]int)
	findingsCount := 0
	for _, r := range unique {
		status, _ := r["status"].(string)
		byStatus[status]++
		if status != "PASS" {
			findingsCount++
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
	}
	// §9.3 / §8.4: the audit overlap_report gives a trim-candidate signal over
	// non-PASS rows clustered by (item_id, category); it is explanatory and is
	// omitted under --compact.
	if !IsCompact() {
		summary["overlap_report"] = computeAuditOverlapReport(unique)
	}

	if outputPath != "" {
		if err := os.WriteFile(outputPath, []byte(ndjson), 0o600); err != nil {
			output.Error(ExitFile, fmt.Sprintf("cannot write output file: %s", err))
			os.Exit(ExitFile)
			return nil
		}
		summary["output_path"] = outputPath
		return output.JSON(summary)
	}

	if IsJSONOutput() {
		summary["output_path"] = "stdout"
		summary["rows"] = unique
		return output.JSON(summary)
	}

	fmt.Print(ndjson)
	fmt.Fprintf(os.Stderr, "merged: %d rows from %d files (%d duplicates removed, %d non-PASS)\n",
		len(unique), totalFiles, duplicatesRemoved, findingsCount)
	return nil
}

// loadAuditMergeRows reads and validates audit-result rows from the input files,
// skipping blank/invalid lines and rows missing item_id or status (with a stderr
// warning). It aborts on a missing/unreadable file (exit 3) or when no valid row
// survives (exit 1).
// loadAuditMergeRows reads and validates audit-result rows from the input files,
// skipping blank, malformed (invalid JSON), and incomplete (missing item_id or
// status) lines with a stderr warning that names which. It aborts only on a
// missing/unreadable file (exit 3). An all-empty or all-invalid set of inputs is
// a valid clean result (§3.1) and yields zero rows without failing.
func loadAuditMergeRows(args []string) []map[string]any {
	for _, path := range args {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			output.Error(ExitFile, fmt.Sprintf("file not found: %s", path))
			os.Exit(ExitFile)
		}
	}

	rows := make([]map[string]any, 0)
	for _, path := range args {
		f, err := os.Open(path)
		if err != nil {
			output.Error(ExitFile, fmt.Sprintf("cannot open file: %s", path))
			os.Exit(ExitFile)
		}
		scanner := bufio.NewScanner(f)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024) // audit notes can be long
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" {
				continue
			}
			var row map[string]any
			if err := json.Unmarshal([]byte(line), &row); err != nil {
				fmt.Fprintf(os.Stderr, "warning: skipping malformed line (invalid JSON) in %s\n", path)
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
				continue
			}
			rows = append(rows, row)
		}
		f.Close()
	}

	return rows
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
