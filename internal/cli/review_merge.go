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

func runReviewMerge(args []string, outputPath string) error {
	if len(args) == 0 {
		output.Error(ExitUsage, "at least 1 file required for merge")
		os.Exit(ExitUsage)
		return nil
	}

	totalFiles := len(args)
	allFindings := loadMergeFindings(args)
	unique := clusterMergeFindings(allFindings)

	// Build NDJSON output
	var buf strings.Builder
	for _, f := range unique {
		line, err := json.Marshal(f)
		if err != nil {
			continue
		}
		buf.Write(line)
		buf.WriteByte('\n')
	}

	ndjsonOutput := buf.String()
	duplicatesRemoved := len(allFindings) - len(unique)

	// Build severity breakdown
	bySeverity := make(map[string]int)
	for _, f := range unique {
		sev, _ := f["severity"].(string)
		bySeverity[sev]++
	}

	// Build JSON summary
	summary := map[string]any{
		"merged_count":       len(unique),
		"input_files":        totalFiles,
		"duplicates_removed": duplicatesRemoved,
		"by_severity":        bySeverity,
		"overlap_report":     computeOverlapReport(unique),
	}

	// Write output based on mode
	if outputPath != "" {
		// -o: NDJSON to file, JSON summary to stdout
		if err := os.WriteFile(outputPath, []byte(ndjsonOutput), 0o600); err != nil {
			output.Error(ExitFile, fmt.Sprintf("cannot write output file: %s", err))
			os.Exit(ExitFile)
			return nil
		}
		summary["output_path"] = outputPath
		return output.JSON(summary)
	}

	if IsJSONOutput() {
		// --json without -o: JSON with findings array
		summary["output_path"] = "stdout"
		summary["findings"] = unique
		return output.JSON(summary)
	}

	// Default: raw NDJSON to stdout
	fmt.Print(ndjsonOutput)

	// Summary to stderr
	fmt.Fprintf(os.Stderr, "merged: %d unique findings from %d files (%d duplicates removed)\n",
		len(unique), totalFiles, duplicatesRemoved)

	return nil
}

// loadMergeFindings reads and validates the review findings from the input
// files, skipping blank/invalid lines and rows missing the required severity and
// finding fields (with a stderr warning). It aborts on a missing/unreadable file
// (exit 3) or when no valid finding survives (exit 1).
// loadMergeFindings reads and validates the review findings from the input
// files, skipping blank, malformed (invalid JSON), and incomplete (missing any
// of location, severity, finding) lines with a stderr warning that names which.
// It aborts only on a missing/unreadable file (exit 3). An all-empty or
// all-invalid set of inputs is a valid clean result (§3.1) and yields zero
// findings without failing, so the merge→record chain works on a clean round.
func loadMergeFindings(args []string) []map[string]any {
	for _, path := range args {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			output.Error(ExitFile, fmt.Sprintf("file not found: %s", path))
			os.Exit(ExitFile)
		}
	}

	allFindings := make([]map[string]any, 0)
	for _, path := range args {
		f, err := os.Open(path)
		if err != nil {
			output.Error(ExitFile, fmt.Sprintf("cannot open file: %s", path))
			os.Exit(ExitFile)
		}
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" {
				continue
			}
			var finding map[string]any
			if err := json.Unmarshal([]byte(line), &finding); err != nil {
				fmt.Fprintf(os.Stderr, "warning: skipping malformed line (invalid JSON) in %s\n", path)
				continue
			}
			if missing := missingFindingFields(finding); len(missing) > 0 {
				fmt.Fprintf(os.Stderr, "warning: skipping incomplete line (missing %s) in %s\n", strings.Join(missing, ", "), path)
				continue
			}
			allFindings = append(allFindings, finding)
		}
		f.Close()
	}

	return allFindings
}

// missingFindingFields returns the review-finding fields that are absent or empty
// in the row (§3.2). role is attribution metadata that the rest of the pipeline
// (record/resolve) treats as optional, so it is not a merge gate.
func missingFindingFields(row map[string]any) []string {
	missing := make([]string, 0, 3)
	for _, k := range []string{"location", "severity", "finding"} {
		if s, _ := row[k].(string); s == "" {
			missing = append(missing, k)
		}
	}
	return missing
}

// clusterMergeFindings clusters the findings by (location key, class) (§8), then
// returns each cluster's representative row (§8.4) annotated with its found_by
// attribution, sorted by severity, then location, then finding text.
func clusterMergeFindings(allFindings []map[string]any) []map[string]any {
	cfs := make([]engine.ClusterFinding, len(allFindings))
	for i, f := range allFindings {
		cfs[i] = clusterFindingFromRow(f)
	}
	clusters := engine.ClusterFindings(cfs)

	unique := make([]map[string]any, 0, len(clusters))
	for _, c := range clusters {
		row := allFindings[c.Representative(cfs)]
		roles, count := c.Attribution(cfs)
		row["found_by"] = count
		if count > 0 {
			row["found_by_roles"] = roles
		} else {
			delete(row, "found_by_roles")
		}
		unique = append(unique, row)
	}

	sort.SliceStable(unique, func(i, j int) bool {
		si := engine.SeverityRank(asString(unique[i]["severity"]))
		sj := engine.SeverityRank(asString(unique[j]["severity"]))
		if si != sj {
			return si < sj
		}
		li, lj := asString(unique[i]["location"]), asString(unique[j]["location"])
		if li != lj {
			return li < lj
		}
		return asString(unique[i]["finding"]) < asString(unique[j]["finding"])
	})
	return unique
}

// clusterFindingFromRow projects an NDJSON finding row onto the fields the
// clustering and attribution machinery reads (§8).
func clusterFindingFromRow(f map[string]any) engine.ClusterFinding {
	return engine.ClusterFinding{
		Location: asString(f["location"]),
		Class:    asString(f["class"]),
		Role:     asString(f["role"]),
		Severity: asString(f["severity"]),
		Finding:  asString(f["finding"]),
	}
}

// asString returns the string value of a decoded JSON field, or "" when the key
// is absent or not a string.
func asString(v any) string {
	s, _ := v.(string)
	return s
}
