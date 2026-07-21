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

	// Check all files exist before processing
	for _, path := range args {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			output.Error(ExitFile, fmt.Sprintf("file not found: %s", path))
			os.Exit(ExitFile)
			return nil
		}
	}

	var allFindings []map[string]any
	totalFiles := len(args)

	for _, path := range args {
		f, err := os.Open(path)
		if err != nil {
			output.Error(ExitFile, fmt.Sprintf("cannot open file: %s", path))
			os.Exit(ExitFile)
			return nil
		}

		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" {
				continue
			}

			var finding map[string]any
			if err := json.Unmarshal([]byte(line), &finding); err != nil {
				fmt.Fprintf(os.Stderr, "warning: skipping invalid JSON line in %s: %s\n", path, line)
				continue
			}

			// Validate required fields
			severity, hasSeverity := finding["severity"]
			findingText, hasFinding := finding["finding"]

			severityStr, sevIsString := severity.(string)
			findingStr, findIsString := findingText.(string)

			if !hasSeverity || !hasFinding || !sevIsString || !findIsString || severityStr == "" || findingStr == "" {
				fmt.Fprintf(os.Stderr, "warning: skipping line missing required fields (severity, finding) in %s\n", path)
				continue
			}

			allFindings = append(allFindings, finding)
		}

		f.Close()
	}

	if len(allFindings) == 0 {
		output.Error(ExitValidation, "no valid findings in input files")
		os.Exit(ExitValidation)
		return nil
	}

	// Cluster review findings by (location key, class) per §8. Each cluster's
	// representative (§8.4) supplies the emitted row, annotated with its cluster's
	// found_by attribution so the per-role overlap report can be derived from the
	// merged findings alone.
	cfs := make([]engine.ClusterFinding, len(allFindings))
	for i, f := range allFindings {
		cfs[i] = clusterFindingFromRow(f)
	}
	clusters := engine.ClusterFindings(cfs)

	unique := make([]map[string]any, 0, len(clusters))
	for _, c := range clusters {
		rep := c.Representative(cfs)
		row := allFindings[rep]
		roles, count := c.Attribution(cfs)
		row["found_by"] = count
		if count > 0 {
			row["found_by_roles"] = roles
		} else {
			delete(row, "found_by_roles")
		}
		unique = append(unique, row)
	}

	// Deterministic output order: highest severity, then location, then finding.
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
