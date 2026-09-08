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
	allFindings, inputs := loadMergeFindings(args)
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
		return finishMerge(output.JSON(summary), dropped)
	}

	if IsJSONOutput() {
		// --json without -o: JSON with findings array
		summary["output_path"] = "stdout"
		summary["findings"] = unique
		return finishMerge(output.JSON(summary), dropped)
	}

	// Default: raw NDJSON to stdout
	fmt.Print(ndjsonOutput)

	// Summary to stderr
	fmt.Fprintf(os.Stderr, "merged: %d unique findings from %d files (%d duplicates removed); line indices are 0-based (use with tp review --resolve)\n",
		len(unique), totalFiles, duplicatesRemoved)

	return finishMerge(nil, dropped)
}

// loadMergeFindings reads and validates the review findings from the input
// files, skipping blank, malformed (invalid JSON), and incomplete lines with a
// stderr warning that names which.
// It aborts only on a missing/unreadable file (exit 3), and returns the §8a.4
// per-input accounting beside the findings: blank lines and a bare `[]` count
// as neither, so an
// all-empty set of inputs is a valid clean result and yields zero findings
// without failing, and the merge→record chain works on a clean round. An input
// whose content lines all fail is a dropped role, which runReviewMerge turns
// into exit 1.
func loadMergeFindings(args []string) ([]map[string]any, []mergeInputCounts) {
	for _, path := range args {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			output.Error(ExitFile, fmt.Sprintf("file not found: %s", path), ndjsonInputFileHint)
			os.Exit(ExitFile)
		}
	}

	allFindings := make([]map[string]any, 0)
	inputs := make([]mergeInputCounts, 0, len(args))
	for _, path := range args {
		f, err := os.Open(path)
		if err != nil {
			output.Error(ExitFile, fmt.Sprintf("cannot open file: %s", path), ndjsonInputFileHint)
			os.Exit(ExitFile)
		}
		counts := mergeInputCounts{Path: path}
		scanner := bufio.NewScanner(f)
		scanner.Buffer(make([]byte, 0, 64*1024), ndjsonLineCap)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			// A line holding an empty JSON array counts as neither parsed
			// nor skipped, like a blank one: it is a role saying it found
			// nothing. tp's own code-audit prompt asked for exactly that,
			// and counting it as a skip made the file a dropped role —
			// which fails the whole merge and, since §5 row 10, writes no
			// `-o`, so one clean role took the panel down. The prompt no
			// longer asks for it; this stays for the rounds run from
			// prompts an older binary emitted.
			//
			// The test is a parse rather than a comparison against `[]`
			// because the line is already trimmed above and TrimSpace
			// removes only the outer padding: `[ ]` survives it unchanged
			// and was still dropping the role. A NON-empty array is left
			// where it was — that is a role whose findings would be lost
			// silently, so it stays a skip.
			if line == "" {
				continue
			}
			var finding map[string]any
			if err := json.Unmarshal([]byte(line), &finding); err != nil {
				if isEmptyJSONArray(line) {
					continue
				}
				fmt.Fprintf(os.Stderr, "warning: skipping malformed line (invalid JSON) in %s\n", path)
				counts.Skipped++
				continue
			}
			// §2's one predicate, shared with the --record gate: see
			// missingFindingFields in review_record.go.
			if missing := missingFindingFields(finding); len(missing) > 0 {
				fmt.Fprintf(os.Stderr, "warning: skipping incomplete line (missing %s) in %s\n", strings.Join(missing, ", "), path)
				counts.Skipped++
				continue
			}
			counts.Parsed++
			allFindings = append(allFindings, finding)
		}
		if err := scanner.Err(); err != nil {
			// Aborting, not warning: see loadAuditMergeRows — zero findings is
			// also what a clean round looks like, so a swallowed read error
			// lets an unread input record one.
			f.Close()
			output.Error(ExitFile, fmt.Sprintf("cannot read %s: %v", path, err), ndjsonReadHint(err))
			os.Exit(ExitFile)
		}
		f.Close()
		inputs = append(inputs, counts)
	}

	return allFindings, inputs
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
