package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/deligoez/tp/internal/output"
)

// roundStats holds computed statistics for a single review round.
type roundStats struct {
	File         string   `json:"file"`
	InFile       int      `json:"in_file"`
	New          int      `json:"new"`
	Resolved     int      `json:"resolved"`
	Unresolved   int      `json:"unresolved"`
	DeltaPercent *float64 `json:"delta_percent"` // nil for R1
}

// severityBreakdown holds fix-status counts for a severity level.
type severityBreakdown struct {
	Fixed     int `json:"fixed"`
	Wontfix   int `json:"wontfix"`
	Duplicate int `json:"duplicate"`
	Remaining int `json:"remaining"`
}

// convergenceResult holds the final convergence report.
type convergenceResult struct {
	Rounds      []roundStats                 `json:"rounds"`
	Convergence map[string]any               `json:"convergence"`
	BySeverity  map[string]*severityBreakdown `json:"by_severity"`
	ByCategory  map[string]int               `json:"by_category"`
}

func runReviewReport(args []string) error {
	files, err := resolveReportFiles(args)
	if err != nil {
		output.Error(ExitUsage, err.Error())
		os.Exit(ExitUsage)
		return nil
	}

	if len(files) == 0 {
		output.Error(ExitUsage, "no NDJSON files provided", "provide file paths or a directory containing *.ndjson files")
		os.Exit(ExitUsage)
		return nil
	}

	// Parse all rounds
	roundFindings := make([][]map[string]any, len(files))
	for i, f := range files {
		findings, parseErr := parseNDJSONFile(f)
		if parseErr != nil {
			output.Error(ExitFile, fmt.Sprintf("cannot read %s: %v", f, parseErr))
			os.Exit(ExitFile)
			return nil
		}
		roundFindings[i] = findings
	}

	// Compute per-round stats
	rounds := computeRoundStats(files, roundFindings)

	// Compute convergence
	converged := isConverged(rounds)

	// Compute severity and category breakdowns
	bySeverity := computeSeverityBreakdown(roundFindings)
	byCategory := computeCategoryBreakdown(roundFindings)

	result := convergenceResult{
		Rounds: rounds,
		Convergence: map[string]any{
			"converged":    converged,
			"total_rounds": len(rounds),
		},
		BySeverity: bySeverity,
		ByCategory: byCategory,
	}

	if output.IsJSON() {
		return output.JSON(result)
	}

	// TTY output
	printReportTTY(result)
	return nil
}

// resolveReportFiles expands args to a sorted list of NDJSON file paths.
// If a single arg is a directory, it scans for *.ndjson files sorted alphabetically.
func resolveReportFiles(args []string) ([]string, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("no files or directory provided")
	}

	// Check if single arg is a directory
	if len(args) == 1 {
		info, err := os.Stat(args[0])
		if err == nil && info.IsDir() {
			return scanDirectoryForNDJSON(args[0])
		}
	}

	// Validate all files exist
	for _, path := range args {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			return nil, fmt.Errorf("file not found: %s", path)
		}
	}

	return args, nil
}

func scanDirectoryForNDJSON(dir string) ([]string, error) {
	var files []string
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("cannot read directory: %s", dir)
	}

	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".ndjson") {
			files = append(files, filepath.Join(dir, entry.Name()))
		}
	}

	if len(files) == 0 {
		return nil, fmt.Errorf("no .ndjson files found in directory: %s", dir)
	}

	sort.Strings(files)
	return files, nil
}

// parseNDJSONFile reads an NDJSON file and returns findings as maps.
func parseNDJSONFile(path string) ([]map[string]any, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	findings := make([]map[string]any, 0)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var finding map[string]any
		if err := json.Unmarshal([]byte(line), &finding); err != nil {
			continue // skip invalid lines
		}
		findings = append(findings, finding)
	}

	return findings, scanner.Err()
}

// keySetForRound returns a set of identity keys for a round's findings.
func keySetForRound(findings []map[string]any) map[string]bool {
	keys := make(map[string]bool)
	for _, f := range findings {
		category, _ := f["category"].(string)
		location, _ := f["location"].(string)
		findingText, _ := f["finding"].(string)
		key := findingIdentityKey(category, location, findingText)
		keys[key] = true
	}
	return keys
}

// computeRoundStats computes per-round statistics for convergence tracking.
func computeRoundStats(files []string, roundFindings [][]map[string]any) []roundStats {
	rounds := make([]roundStats, 0, len(files))

	// Track cumulative union of all previous rounds' keys
	cumulativeKeys := make(map[string]bool)
	var prevKeys map[string]bool
	prevUnresolved := 0

	for i, findings := range roundFindings {
		currentKeys := keySetForRound(findings)

		inFile := len(findings)

		var newCount, resolvedCount, unresolvedCount int

		if i == 0 {
			// R1: all findings are new
			newCount = inFile
			resolvedCount = 0
			unresolvedCount = newCount
		} else {
			// New: key not in UNION of all previous rounds
			for key := range currentKeys {
				if !cumulativeKeys[key] {
					newCount++
				}
			}

			// Resolved: key in immediately preceding round but not current
			for key := range prevKeys {
				if !currentKeys[key] {
					resolvedCount++
				}
			}

			unresolvedCount = prevUnresolved + newCount - resolvedCount
		}

		var deltaPercent *float64
		if i > 0 && prevUnresolved > 0 {
			d := float64(unresolvedCount-prevUnresolved) / float64(prevUnresolved) * 100
			d = math.Round(d*100) / 100 // round to 2 decimal places
			deltaPercent = &d
		} else if i > 0 {
			// prev was 0, current may have new findings
			if unresolvedCount > 0 {
				// Can't compute meaningful percentage from 0
				deltaPercent = nil
			} else {
				zero := 0.0
				deltaPercent = &zero
			}
		}

		rs := roundStats{
			File:         filepath.Base(files[i]),
			InFile:       inFile,
			New:          newCount,
			Resolved:     resolvedCount,
			Unresolved:   unresolvedCount,
			DeltaPercent: deltaPercent,
		}
		rounds = append(rounds, rs)

		// Update cumulative keys
		for key := range currentKeys {
			cumulativeKeys[key] = true
		}
		prevKeys = currentKeys
		prevUnresolved = unresolvedCount
	}

	return rounds
}

// isConverged returns true when the last 2+ rounds have 0 findings.
func isConverged(rounds []roundStats) bool {
	if len(rounds) < 2 {
		return false
	}
	// Check if last 2 rounds have 0 in_file
	for i := len(rounds) - 2; i < len(rounds); i++ {
		if rounds[i].InFile != 0 {
			return false
		}
	}
	return true
}

var severityOrder = []string{"critical", "high", "medium", "low", "unknown"}

// computeSeverityBreakdown computes fix-status counts per severity.
func computeSeverityBreakdown(roundFindings [][]map[string]any) map[string]*severityBreakdown {
	result := make(map[string]*severityBreakdown)

	// Track all findings ever seen, keyed by identity key
	type findingInfo struct {
		severity string
		resolved string // resolved status if present
	}
	allFindings := make(map[string]*findingInfo)

	// Last round keys for determining "remaining"
	var lastRoundKeys map[string]bool
	if len(roundFindings) > 0 {
		lastRoundKeys = keySetForRound(roundFindings[len(roundFindings)-1])
	}

	hasResolvedField := false

	// Collect all findings across all rounds
	for _, findings := range roundFindings {
		for _, f := range findings {
			category, _ := f["category"].(string)
			location, _ := f["location"].(string)
			findingText, _ := f["finding"].(string)
			severity, _ := f["severity"].(string)
			key := findingIdentityKey(category, location, findingText)

			if severity == "" {
				severity = "unknown"
			}

			resolvedStatus, _ := f["resolved"].(string)
			if resolvedStatus != "" {
				hasResolvedField = true
			}

			existing, exists := allFindings[key]
			if !exists {
				allFindings[key] = &findingInfo{severity: severity, resolved: resolvedStatus}
			} else if resolvedStatus != "" {
				// Update resolved status if newer round has it
				existing.resolved = resolvedStatus
			}
		}
	}

	// Initialize severity buckets
	for _, sev := range severityOrder {
		result[sev] = &severityBreakdown{}
	}

	// Categorize each finding
	for key, info := range allFindings {
		sev := info.severity
		if _, ok := result[sev]; !ok {
			result[sev] = &severityBreakdown{}
		}

		if hasResolvedField {
			switch info.resolved {
			case "fixed":
				result[sev].Fixed++
			case "wontfix":
				result[sev].Wontfix++
			case "duplicate":
				result[sev].Duplicate++
			default:
				result[sev].Remaining++
			}
		} else {
			// No resolved field: remaining = present in latest round, fixed = disappeared
			if lastRoundKeys[key] {
				result[sev].Remaining++
			} else {
				result[sev].Fixed++
			}
		}
	}

	// Remove empty severity levels
	for sev, bd := range result {
		if bd.Fixed == 0 && bd.Wontfix == 0 && bd.Duplicate == 0 && bd.Remaining == 0 {
			delete(result, sev)
		}
	}

	return result
}

// computeCategoryBreakdown counts findings in the latest round by category.
func computeCategoryBreakdown(roundFindings [][]map[string]any) map[string]int {
	result := make(map[string]int)

	if len(roundFindings) == 0 {
		return result
	}

	// Count all unique findings across all rounds by category
	seen := make(map[string]string) // key -> category
	for _, findings := range roundFindings {
		for _, f := range findings {
			category, _ := f["category"].(string)
			location, _ := f["location"].(string)
			findingText, _ := f["finding"].(string)
			key := findingIdentityKey(category, location, findingText)
			if category == "" {
				category = "uncategorized"
			}
			seen[key] = category
		}
	}

	for _, cat := range seen {
		result[cat]++
	}

	return result
}

// printReportTTY outputs the convergence report in TTY format.
func printReportTTY(result convergenceResult) {
	w := os.Stdout

	_, _ = fmt.Fprintln(w, "Convergence Report")
	_, _ = fmt.Fprintln(w, strings.Repeat("=", 60))
	_, _ = fmt.Fprintln(w)

	// Convergence table
	_, _ = fmt.Fprintf(w, "%-20s %7s %5s %8s %10s %8s\n", "Round", "In File", "New", "Resolved", "Unresolved", "Δ%")
	_, _ = fmt.Fprintln(w, strings.Repeat("-", 60))

	for i, r := range result.Rounds {
		delta := "—"
		if r.DeltaPercent != nil {
			delta = fmt.Sprintf("%+.1f%%", *r.DeltaPercent)
		}
		label := fmt.Sprintf("R%d (%s)", i+1, r.File)
		_, _ = fmt.Fprintf(w, "%-20s %7d %5d %8d %10d %8s\n",
			label, r.InFile, r.New, r.Resolved, r.Unresolved, delta)
	}

	_, _ = fmt.Fprintln(w)

	// Status line
	converged, _ := result.Convergence["converged"].(bool)
	totalRounds, _ := result.Convergence["total_rounds"].(int)
	if converged {
		_, _ = fmt.Fprintf(w, "Status: CONVERGED after %d rounds\n", totalRounds)
	} else {
		lastRound := result.Rounds[len(result.Rounds)-1]
		_, _ = fmt.Fprintf(w, "Status: NOT CONVERGED (%d unresolved after %d rounds)\n",
			lastRound.Unresolved, totalRounds)
	}

	_, _ = fmt.Fprintln(w)

	// By severity breakdown
	if len(result.BySeverity) > 0 {
		_, _ = fmt.Fprintln(w, "By Severity:")
		for _, sev := range severityOrder {
			bd, ok := result.BySeverity[sev]
			if !ok {
				continue
			}
			parts := make([]string, 0, 4)
			if bd.Fixed > 0 {
				parts = append(parts, fmt.Sprintf("%d fixed", bd.Fixed))
			}
			if bd.Wontfix > 0 {
				parts = append(parts, fmt.Sprintf("%d wontfix", bd.Wontfix))
			}
			if bd.Duplicate > 0 {
				parts = append(parts, fmt.Sprintf("%d duplicate", bd.Duplicate))
			}
			if bd.Remaining > 0 {
				parts = append(parts, fmt.Sprintf("%d remaining", bd.Remaining))
			}
			_, _ = fmt.Fprintf(w, "  %-10s %s\n", sev+":", strings.Join(parts, ", "))
		}
		_, _ = fmt.Fprintln(w)
	}

	// By category breakdown
	if len(result.ByCategory) > 0 {
		_, _ = fmt.Fprintln(w, "By Category:")
		// Sort categories alphabetically
		cats := make([]string, 0, len(result.ByCategory))
		for cat := range result.ByCategory {
			cats = append(cats, cat)
		}
		sort.Strings(cats)
		for _, cat := range cats {
			_, _ = fmt.Fprintf(w, "  %-20s %d\n", cat+":", result.ByCategory[cat])
		}
	}
}
