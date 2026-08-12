package cli

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/deligoez/tp/internal/engine"
	"github.com/deligoez/tp/internal/model"
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
	Rounds              []roundStats                  `json:"rounds"`
	Convergence         map[string]any                `json:"convergence"`
	BySeverity          map[string]*severityBreakdown `json:"by_severity"`
	ByCategory          map[string]int                `json:"by_category"`
	ByClass             map[string]int                `json:"by_class"`
	MechanizeCandidates []mechanizeCandidate          `json:"mechanize_candidates"`
	OverlapReport       []engine.RoleOverlap          `json:"overlap_report"`
}

// mechanizeCandidate is a finding class worth turning into a mechanical check.
type mechanizeCandidate struct {
	Class      string `json:"class"`
	RoundsSeen int    `json:"rounds_seen"`
	Total      int    `json:"total"`
}

// mechanizeRegisterHint accompanies mechanize_candidates in --record output.
const mechanizeRegisterHint = "write a mechanical check for each candidate class and register it: tp set --workflow checks='[...]'"

// reportInputUsageHint names what --report wants when the invocation named
// nothing to report on. It is the text that used to sit on an unreachable
// branch below runReviewReport's resolve call, where no invocation could reach
// it: resolveReportFiles either fails or returns a non-empty list.
const reportInputUsageHint = "provide file paths or a directory containing *.ndjson files"

func runReviewReport(args []string) error {
	files, err := resolveReportFiles(args)
	if err != nil {
		// A bad PATH is a file error (exit 3) with the NDJSON-input hint — the
		// same answer --merge gives for the same operator typo, which used to
		// differ so one mistake was reported two ways depending on which mode
		// caught it. A bad INVOCATION — no argument, or a directory holding no
		// NDJSON — stays exit 2: the path was fine, there was nothing to report
		// on, and §10.5 of spec/0.16.0-review-orchestration.md pins both rows at
		// 2. It is NOT the exit code --merge gives the same empty directory (3),
		// and that is not a divergence to close here: --merge takes no directory
		// at all, so its 3 is the answer to "this file is unreadable", not to
		// "this directory is empty". What was wrong was the hint — both usage
		// rows inherited "see tp --help", which repairs neither — so they carry
		// the one that does.
		var pathErr *reportPathError
		if errors.As(err, &pathErr) {
			output.Error(ExitFile, err.Error(), ndjsonInputFileHint)
			os.Exit(ExitFile)
			return nil
		}
		output.Error(ExitUsage, err.Error(), reportInputUsageHint)
		os.Exit(ExitUsage)
		return nil
	}

	// Parse all rounds
	roundFindings := make([][]map[string]any, len(files))
	for i, f := range files {
		findings, parseErr := parseNDJSONFile(f)
		if parseErr != nil {
			output.Error(ExitFile, fmt.Sprintf("cannot read %s: %v", f, parseErr), ndjsonReadHint(parseErr))
			os.Exit(ExitFile)
			return nil
		}
		roundFindings[i] = findings
	}

	// Compute per-round stats
	rounds := computeRoundStats(files, roundFindings)

	// Compute convergence
	converged := isConverged(rounds)

	// Compute severity, category, and class breakdowns
	bySeverity := computeSeverityBreakdown(roundFindings)
	byCategory := computeCategoryBreakdown(roundFindings)
	byClass := computeClassBreakdown(roundFindings)

	result := convergenceResult{
		Rounds: rounds,
		Convergence: map[string]any{
			"converged":    converged,
			"total_rounds": len(rounds),
		},
		BySeverity:          bySeverity,
		ByCategory:          byCategory,
		ByClass:             byClass,
		MechanizeCandidates: computeMechanizeCandidates(roundFindings),
	}

	// The per-role overlap report is computed from the latest round's merged
	// findings alone (§8.5); an empty history yields an empty report.
	if len(roundFindings) > 0 {
		result.OverlapReport = computeOverlapReport(roundFindings[len(roundFindings)-1])
	} else {
		result.OverlapReport = []engine.RoleOverlap{}
	}

	if output.IsJSON() {
		return output.JSON(result)
	}

	// TTY output
	printReportTTY(&result)
	return nil
}

// reportPathError marks a resolve failure that is a bad path rather than a bad
// invocation. The distinction decides the exit code: a path tp cannot read is a
// file error, while an argument list with nothing to report on is a usage one.
type reportPathError struct{ msg string }

func (e *reportPathError) Error() string { return e.msg }

// resolveReportFiles expands args to a sorted list of NDJSON file paths.
// If a single arg is a directory, it scans for *.ndjson files sorted alphabetically.
func resolveReportFiles(args []string) ([]string, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("at least 1 findings file required for report")
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
			return nil, &reportPathError{msg: fmt.Sprintf("file not found: %s", path)}
		}
	}

	return args, nil
}

func scanDirectoryForNDJSON(dir string) ([]string, error) {
	var files []string
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, &reportPathError{msg: fmt.Sprintf("cannot read directory: %s", dir)}
	}

	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".ndjson") {
			files = append(files, filepath.Join(dir, entry.Name()))
		}
	}

	if len(files) == 0 {
		return nil, fmt.Errorf("no .ndjson files found in %s", dir)
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
	scanner.Buffer(make([]byte, 0, 64*1024), ndjsonLineCap)
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

		// Update cumulative keys: add current, remove resolved (so reappearing findings count as new)
		for key := range currentKeys {
			cumulativeKeys[key] = true
		}
		for key := range prevKeys {
			if !currentKeys[key] {
				delete(cumulativeKeys, key)
			}
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

			var resolvedStatus string
			if resolvedObj, ok := f["resolved"].(map[string]any); ok {
				resolvedStatus, _ = resolvedObj["status"].(string)
				if resolvedStatus != "" {
					hasResolvedField = true
				}
			} else if rs, ok := f["resolved"].(string); ok && rs != "" {
				// Backward compat: resolved as a plain string
				resolvedStatus = rs
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

// computeClassBreakdown counts unique findings carrying a non-empty class;
// per identity key the first non-empty class wins, mirroring merge dedup.
func computeClassBreakdown(roundFindings [][]map[string]any) map[string]int {
	seen := make(map[string]string) // identity key -> first non-empty class
	for _, findings := range roundFindings {
		for _, f := range findings {
			category, _ := f["category"].(string)
			location, _ := f["location"].(string)
			findingText, _ := f["finding"].(string)
			key := findingIdentityKey(category, location, findingText)
			class, _ := f["class"].(string)
			if existing, ok := seen[key]; !ok || (existing == "" && class != "") {
				seen[key] = class
			}
		}
	}
	result := make(map[string]int)
	for _, class := range seen {
		if class != "" {
			result[class]++
		}
	}
	return result
}

// mechanizeClassesFromRounds loads every recorded round's rows and returns the
// classes of the mechanize candidates (same threshold as computeMechanizeCandidates),
// ordered as that function orders them, with the mechanized classes filtered out.
// It is the branch-3 signal for engine.ReviewNextAction; --status derives it from
// the recorded rounds so branch 3 is reachable there as well as on --record (§8.2).
//
// This separate call over the recorded rounds is what makes --status a sink of
// §3.2's candidate suppression in its own right rather than a projection of
// --record's array: --status emits no mechanize_candidates of its own, so the
// filter has to be applied here for next_action to stop naming a class whose
// check already exists.
func mechanizeClassesFromRounds(specPath string, rounds []engine.ReviewRound, checks []model.Check) []string {
	roundFindings := make([][]map[string]any, 0, len(rounds))
	for i := range rounds {
		rows, found := engine.LoadRoundRows(specPath, &rounds[i])
		if !found {
			continue
		}
		roundFindings = append(roundFindings, rows)
	}
	kept, _ := filterMechanizedCandidates(computeMechanizeCandidates(roundFindings), checks)
	return mechanizeCandidateClasses(kept)
}

// filterMechanizedCandidates drops every candidate whose class is mechanized by
// a valid entry of the effective workflow's checks (§3.2, candidate
// suppression). It runs strictly *after* computeMechanizeCandidates, never
// inside it: the frequency threshold is unchanged and every class is measured
// against the same rounds, so suppressing one class never changes whether
// another crosses it.
//
// over-specification is suppressed here like any other class — §3.1's exemption
// is scoped to the reviewer exclusion list alone (see
// engine.ReviewerExclusionClasses).
//
// It returns both halves of the split, so the withheld set is what this one
// filter dropped rather than a second membership predicate run over the same
// checks: kept feeds mechanize_candidates and next_action, withheld feeds
// mechanized_classes (§3.3). withheld carries each class once because
// computeMechanizeCandidates keys its output by class, and is sorted ascending
// — every member equals a valid checks entry's class and is therefore
// lowercase kebab-case, an alphabet in which byte order and a case-insensitive
// order cannot differ, so sort.Strings is not a choice among comparators.
//
// Both results are always non-nil slices, so a round on which the filter
// removes every candidate still emits mechanize_candidates as [] and never as
// null, and a round on which it removes none emits mechanized_classes as []
// (§3.3).
func filterMechanizedCandidates(candidates []mechanizeCandidate, checks []model.Check) (kept []mechanizeCandidate, withheld []string) {
	kept = make([]mechanizeCandidate, 0, len(candidates))
	withheld = make([]string, 0, len(candidates))
	for _, c := range candidates {
		if engine.IsMechanizedClass(checks, c.Class) {
			withheld = append(withheld, c.Class)
			continue
		}
		kept = append(kept, c)
	}
	sort.Strings(withheld)
	return kept, withheld
}

// mechanizeCandidateClasses projects a candidate slice to its class strings,
// preserving order.
func mechanizeCandidateClasses(candidates []mechanizeCandidate) []string {
	classes := make([]string, 0, len(candidates))
	for _, c := range candidates {
		classes = append(classes, c.Class)
	}
	return classes
}

// computeMechanizeCandidates finds classes appearing in >= 2 distinct rounds
// or >= 5 times within a single round, sorted by total descending, ties
// alphabetical by class.
func computeMechanizeCandidates(roundFindings [][]map[string]any) []mechanizeCandidate {
	type classStat struct {
		rounds   int
		total    int
		maxRound int
	}
	stats := make(map[string]*classStat)
	for _, findings := range roundFindings {
		perRound := make(map[string]int)
		for _, f := range findings {
			class, _ := f["class"].(string)
			if class == "" {
				continue
			}
			perRound[class]++
		}
		for class, n := range perRound {
			s := stats[class]
			if s == nil {
				s = &classStat{}
				stats[class] = s
			}
			s.rounds++
			s.total += n
			if n > s.maxRound {
				s.maxRound = n
			}
		}
	}
	out := make([]mechanizeCandidate, 0)
	for class, s := range stats {
		if s.rounds >= 2 || s.maxRound >= 5 {
			out = append(out, mechanizeCandidate{Class: class, RoundsSeen: s.rounds, Total: s.total})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Total != out[j].Total {
			return out[i].Total > out[j].Total
		}
		return out[i].Class < out[j].Class
	})
	return out
}

// printReportTTY outputs the convergence report in TTY format.
func printReportTTY(result *convergenceResult) {
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

	// By class breakdown (rows with a non-empty class only)
	if len(result.ByClass) > 0 {
		_, _ = fmt.Fprintln(w)
		_, _ = fmt.Fprintln(w, "By Class:")
		classes := make([]string, 0, len(result.ByClass))
		for class := range result.ByClass {
			classes = append(classes, class)
		}
		sort.Strings(classes)
		for _, class := range classes {
			_, _ = fmt.Fprintf(w, "  %-20s %d\n", class+":", result.ByClass[class])
		}
	}

	if len(result.MechanizeCandidates) > 0 {
		_, _ = fmt.Fprintln(w)
		_, _ = fmt.Fprintln(w, "Mechanize Candidates:")
		for _, c := range result.MechanizeCandidates {
			_, _ = fmt.Fprintf(w, "  %-20s rounds_seen=%d total=%d\n", c.Class+":", c.RoundsSeen, c.Total)
		}
		_, _ = fmt.Fprintf(w, "  hint: %s\n", mechanizeRegisterHint)
	}

	if len(result.OverlapReport) > 0 {
		_, _ = fmt.Fprintln(w)
		_, _ = fmt.Fprintln(w, "Per-Role Overlap (latest round):")
		for _, r := range result.OverlapReport {
			flag := ""
			if r.TrimCandidate {
				flag = "  [trim candidate]"
			}
			_, _ = fmt.Fprintf(w, "  %-24s unique=%d shared=%d%s\n", r.Role+":", r.Unique, r.Shared, flag)
		}
	}
}
