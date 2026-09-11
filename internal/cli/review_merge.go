package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/deligoez/tp/internal/engine"
	"github.com/deligoez/tp/internal/output"
)

// reviewMergeRule is §2's required set as `tp review --merge` applies it,
// through the one predicate the --record gate shares (missingFindingFields).
var reviewMergeRule = mergeRowRule{
	noun:     "review finding",
	required: requiredFindingFields,
	missing:  missingFindingFields,
}

func runReviewMerge(args []string, outputPath string) error {
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
				"%s looks like a spec; --merge takes NDJSON input files only: tp review --merge <a.ndjson> [<b.ndjson> ...]",
				path,
			))
			os.Exit(ExitUsage)
			return nil
		}
	}

	totalFiles := len(args)
	allFindings, inputs := loadMergeRows(args, reviewMergeRule)
	// §8a.4: an input whose every content line was skipped drops a whole role
	// from the merged set. The payload names the counts; the exit code is what
	// an unattended driver reads.
	dropped := droppedInputs(inputs)
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

	// Build JSON summary. location_clusters (§8a.1) is derived from `unique`
	// after the counts above are already fixed: it is a second cut of the same
	// records, grouped by location instead of by (location, class), and feeds
	// neither merged_count nor duplicates_removed nor by_severity.
	overlapReport, attributionExcludes := overlapReportWithAttribution(unique)
	summary := map[string]any{
		"merged_count":       len(unique),
		"input_files":        totalFiles,
		"duplicates_removed": duplicatesRemoved,
		"by_severity":        bySeverity,
		"overlap_report":     overlapReport,
		"inputs":             inputs,
	}
	// §8a.1 / §8.4: location_clusters is reporting only — it feeds no arithmetic,
	// no flag and no exit code — so it is an explanatory field and --compact
	// strips it, the way it strips attribution_excludes. overlap_report survives
	// because trim_candidate is a decision.
	if !IsCompact() {
		summary["location_clusters"] = computeLocationClusters(unique)
	}
	// §9.2 / §8.4: attribution_excludes surfaces the regression exclusion only
	// when it caused merged_count to exceed the overlap-report finding count;
	// always omitted under --compact.
	if !IsCompact() && len(attributionExcludes) > 0 {
		summary["attribution_excludes"] = attributionExcludes
	}

	// Write output based on mode
	if outputPath != "" {
		// -o: NDJSON to file, JSON summary to stdout. §5 row 10: a merge that is
		// about to exit non-zero writes nothing at that path, so output_path is
		// named only when a file is there to name.
		if writeMergeOutput(outputPath, ndjsonOutput, dropped) {
			summary["output_path"] = outputPath
		}
		return finishMerge(output.JSON(summary), dropped, reviewMergeRule)
	}

	if IsJSONOutput() {
		// --json without -o: JSON with findings array
		summary["output_path"] = "stdout"
		summary["findings"] = unique
		return finishMerge(output.JSON(summary), dropped, reviewMergeRule)
	}

	// Default: raw NDJSON to stdout
	fmt.Print(ndjsonOutput)

	// Summary to stderr
	fmt.Fprintf(os.Stderr, "merged: %d unique findings from %d files (%d duplicates removed); line indices are 0-based (use with tp review --resolve)\n",
		len(unique), totalFiles, duplicatesRemoved)

	return finishMerge(nil, dropped, reviewMergeRule)
}

// dropEmptyArrayLines removes every line that is an empty JSON array
// (isEmptyJSONArray) from a --record input, so a role that found nothing
// records the way a zero-byte file does, and the stored round file holds only
// JSON objects, which every reader of a round file requires. Input with no
// such line is returned unchanged.
func dropEmptyArrayLines(data []byte) []byte {
	lines := strings.Split(string(data), "\n")
	kept := make([]string, 0, len(lines))
	for _, line := range lines {
		if isEmptyJSONArray(strings.TrimSpace(line)) {
			continue
		}
		kept = append(kept, line)
	}
	if len(kept) == len(lines) {
		return data
	}
	return []byte(strings.Join(kept, "\n"))
}

// isEmptyJSONArray reports whether a line is a well-formed JSON array with no
// elements — the shape a role uses to say it found nothing, in whichever
// spelling its emitter chose. `[]`, `[]  ` and `[ ]` all reach here.
func isEmptyJSONArray(line string) bool {
	var arr []json.RawMessage
	return json.Unmarshal([]byte(line), &arr) == nil && len(arr) == 0
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
