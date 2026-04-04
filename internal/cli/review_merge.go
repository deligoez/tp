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

	// Deduplicate: keep highest severity for each identity key
	sevRank := map[string]int{"critical": 0, "high": 1, "medium": 2, "low": 3}
	getSevRank := func(s string) int {
		if r, ok := sevRank[s]; ok {
			return r
		}
		return 4 // unknown
	}

	type dedupEntry struct {
		key     string
		finding map[string]any
		rank    int
	}

	seen := make(map[string]*dedupEntry)
	order := make([]string, 0)

	for _, f := range allFindings {
		category, _ := f["category"].(string)
		location, _ := f["location"].(string)
		findingText, _ := f["finding"].(string)

		key := findingIdentityKey(category, location, findingText)
		sev, _ := f["severity"].(string)
		rank := getSevRank(sev)

		if existing, exists := seen[key]; exists {
			// Keep the one with highest severity (lower rank = higher severity)
			if rank < existing.rank {
				existing.finding = f
				existing.rank = rank
			}
		} else {
			seen[key] = &dedupEntry{key: key, finding: f, rank: rank}
			order = append(order, key)
		}
	}

	// Collect unique findings
	unique := make([]map[string]any, 0, len(seen))
	for _, key := range order {
		unique = append(unique, seen[key].finding)
	}

	// Sort by severity (critical first), then category alphabetically
	sort.SliceStable(unique, func(i, j int) bool {
		si := getSevRank(unique[i]["severity"].(string))
		sj := getSevRank(unique[j]["severity"].(string))
		if si != sj {
			return si < sj
		}
		ci, _ := unique[i]["category"].(string)
		cj, _ := unique[j]["category"].(string)
		return ci < cj
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
		"merged_count":      len(unique),
		"input_files":       totalFiles,
		"duplicates_removed": duplicatesRemoved,
		"by_severity":       bySeverity,
	}

	// Write output based on mode
	if outputPath != "" {
		// -o: NDJSON to file, JSON summary to stdout
		if err := os.WriteFile(outputPath, []byte(ndjsonOutput), 0o644); err != nil {
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
