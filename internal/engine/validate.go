package engine

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/deligoez/tp/internal/model"
)

// ValidationResult holds the output of tp validate.
type ValidationResult struct {
	Valid               bool              `json:"valid"`
	Errors              int               `json:"errors"`
	Warnings            int               `json:"warnings"`
	AtomicityViolations int               `json:"atomicity_violations"`
	Checks              map[string]string `json:"checks"`
	Findings            []Finding         `json:"findings,omitempty"`
}

// Validate runs all validation checks on a task file.
func Validate(tf *model.TaskFile, specPath string, specExists, strict bool) *ValidationResult {
	result := &ValidationResult{
		Checks: make(map[string]string),
	}

	// 1. Schema validation
	schemaFindings := validateSchema(tf)
	result.Findings = append(result.Findings, schemaFindings...)
	if countErrors(schemaFindings) > 0 {
		result.Checks["schema"] = fmt.Sprintf("%d errors", countErrors(schemaFindings))
	} else {
		result.Checks["schema"] = "pass"
	}

	// 2. Atomicity checks
	atomFindings := validateAtomicity(tf)
	result.Findings = append(result.Findings, atomFindings...)
	result.AtomicityViolations = len(atomFindings)
	if len(atomFindings) > 0 {
		result.Checks["atomicity"] = fmt.Sprintf("%d warnings", len(atomFindings))
	} else {
		result.Checks["atomicity"] = "pass"
	}

	// 3. Referential integrity
	refFindings := validateReferences(tf)
	result.Findings = append(result.Findings, refFindings...)
	if countErrors(refFindings) > 0 {
		result.Checks["references"] = fmt.Sprintf("%d errors", countErrors(refFindings))
	} else {
		result.Checks["references"] = "pass"
	}

	// 4. Cycle detection
	cycleFindings := validateCycles(tf)
	result.Findings = append(result.Findings, cycleFindings...)
	if countErrors(cycleFindings) > 0 {
		result.Checks["cycles"] = fmt.Sprintf("%d errors", countErrors(cycleFindings))
	} else {
		result.Checks["cycles"] = "pass"
	}

	// 5. ID uniqueness
	uniqueFindings := validateUniqueness(tf)
	result.Findings = append(result.Findings, uniqueFindings...)
	if countErrors(uniqueFindings) > 0 {
		result.Checks["uniqueness"] = fmt.Sprintf("%d errors", countErrors(uniqueFindings))
	} else {
		result.Checks["uniqueness"] = "pass"
	}

	// 6. Coverage (section-level)
	if specExists {
		covFindings := ValidateCoverage(tf, specPath)
		result.Findings = append(result.Findings, covFindings...)
		if countErrors(covFindings) > 0 {
			result.Checks["coverage"] = fmt.Sprintf("%d errors", countErrors(covFindings))
		} else {
			result.Checks["coverage"] = "pass"
		}
	} else {
		result.Checks["coverage"] = "skipped (spec not found)"
	}

	// 7. Line coverage
	switch {
	case specExists:
		lineFindings := ValidateLineCoverage(tf, specPath)
		result.Findings = append(result.Findings, lineFindings...)
		switch {
		case countErrors(lineFindings) > 0:
			result.Checks["line_coverage"] = fmt.Sprintf("%d errors", countErrors(lineFindings))
		case countWarnings(lineFindings) > 0:
			result.Checks["line_coverage"] = fmt.Sprintf("%d warnings", countWarnings(lineFindings))
		default:
			result.Checks["line_coverage"] = "pass"
		}
	default:
		result.Checks["line_coverage"] = "skipped (spec not found)"
	}

	// Count totals
	for i := range result.Findings {
		switch result.Findings[i].Severity {
		case "error":
			result.Errors++
		case "warning":
			if strict && result.Findings[i].Rule == "atomicity" {
				result.Errors++
				result.Findings[i].Severity = "error"
			} else {
				result.Warnings++
			}
		}
	}

	result.Valid = result.Errors == 0
	return result
}

func countErrors(findings []Finding) int {
	n := 0
	for _, f := range findings {
		if f.Severity == "error" {
			n++
		}
	}
	return n
}

func countWarnings(findings []Finding) int {
	n := 0
	for _, f := range findings {
		if f.Severity == "warning" {
			n++
		}
	}
	return n
}

func validateSchema(tf *model.TaskFile) []Finding {
	var findings []Finding
	if tf.Version == 0 {
		findings = append(findings, Finding{Severity: "error", Rule: "schema", Message: "version is required"})
	}
	if tf.Spec == "" {
		findings = append(findings, Finding{Severity: "error", Rule: "schema", Message: "spec is required"})
	}
	if len(tf.Tasks) == 0 {
		findings = append(findings, Finding{Severity: "error", Rule: "schema", Message: "tasks array is empty"})
	}
	for i := range tf.Tasks {
		t := &tf.Tasks[i]
		if t.ID == "" {
			findings = append(findings, Finding{Severity: "error", Rule: "schema", Message: "task id is required"})
		}
		if t.Title == "" {
			findings = append(findings, Finding{Severity: "error", Rule: "schema", Message: fmt.Sprintf("task %s: title is required", t.ID)})
		}
		if !model.ValidStatus(t.Status) {
			findings = append(findings, Finding{Severity: "error", Rule: "schema", Message: fmt.Sprintf("task %s: invalid status %q", t.ID, t.Status)})
		}
		if t.Acceptance == "" {
			findings = append(findings, Finding{Severity: "error", Rule: "schema", Message: fmt.Sprintf("task %s: acceptance is required", t.ID)})
		}
		if t.EstimateMinutes == 0 {
			findings = append(findings, Finding{Severity: "error", Rule: "schema", Message: fmt.Sprintf("task %s: estimate_minutes is required", t.ID)})
		}
	}
	return findings
}

var conjunctionRegex = regexp.MustCompile(`\band\b|,|\+`)

func validateAtomicity(tf *model.TaskFile) []Finding {
	var findings []Finding
	for i := range tf.Tasks {
		t := &tf.Tasks[i]
		if t.EstimateMinutes < 1 || t.EstimateMinutes > 15 {
			findings = append(findings, Finding{Severity: "warning", Rule: "atomicity", Message: fmt.Sprintf("task %s: estimate_minutes %d outside 1-15 range", t.ID, t.EstimateMinutes)})
		}
		words := strings.Fields(t.Title)
		if len(words) > 8 {
			findings = append(findings, Finding{Severity: "warning", Rule: "atomicity", Message: fmt.Sprintf("task %s: title has %d words (max 8)", t.ID, len(words))})
		}
		if conjunctionRegex.MatchString(t.Title) {
			findings = append(findings, Finding{Severity: "warning", Rule: "atomicity", Message: fmt.Sprintf("task %s: title contains conjunction (and/,/+)", t.ID)})
		}
		if len(t.SourceSections) > 2 {
			findings = append(findings, Finding{Severity: "warning", Rule: "atomicity", Message: fmt.Sprintf("task %s: source_sections has %d entries (max 2)", t.ID, len(t.SourceSections))})
		}
		if len(t.Description) > 300 {
			findings = append(findings, Finding{Severity: "warning", Rule: "atomicity", Message: fmt.Sprintf("task %s: description is %d chars (max 300)", t.ID, len(t.Description))})
		}
		criteria := ParseAcceptanceCriteria(t.Acceptance)
		if len(criteria) > 3 {
			findings = append(findings, Finding{Severity: "warning", Rule: "atomicity", Message: fmt.Sprintf("task %s: acceptance has %d criteria (max 3)", t.ID, len(criteria))})
		}
	}
	return findings
}

func validateReferences(tf *model.TaskFile) []Finding {
	var findings []Finding
	ids := make(map[string]bool)
	for i := range tf.Tasks {
		ids[tf.Tasks[i].ID] = true
	}
	for i := range tf.Tasks {
		t := &tf.Tasks[i]
		for _, dep := range t.DependsOn {
			if dep == t.ID {
				findings = append(findings, Finding{Severity: "error", Rule: "self-dependency", Message: fmt.Sprintf("task %s depends on itself", t.ID)})
			}
			if !ids[dep] {
				findings = append(findings, Finding{Severity: "error", Rule: "dangling-reference", Message: fmt.Sprintf("task %s depends on %s which does not exist", t.ID, dep)})
			}
		}
	}
	return findings
}

func validateCycles(tf *model.TaskFile) []Finding {
	deps := make(map[string][]string)
	for i := range tf.Tasks {
		deps[tf.Tasks[i].ID] = tf.Tasks[i].DependsOn
	}

	visited := make(map[string]int) // 0=unvisited, 1=in-progress, 2=done
	var cyclePath []string

	var dfs func(id string) bool
	dfs = func(id string) bool {
		visited[id] = 1
		cyclePath = append(cyclePath, id)
		for _, dep := range deps[id] {
			if visited[dep] == 1 {
				cyclePath = append(cyclePath, dep)
				return true
			}
			if visited[dep] == 0 && dfs(dep) {
				return true
			}
		}
		cyclePath = cyclePath[:len(cyclePath)-1]
		visited[id] = 2
		return false
	}

	for i := range tf.Tasks {
		if visited[tf.Tasks[i].ID] == 0 {
			if dfs(tf.Tasks[i].ID) {
				return []Finding{{
					Severity: "error",
					Rule:     "circular-dependency",
					Message:  fmt.Sprintf("circular dependency: %s", strings.Join(cyclePath, " → ")),
				}}
			}
		}
	}
	return nil
}

func validateUniqueness(tf *model.TaskFile) []Finding {
	var findings []Finding
	seen := make(map[string]bool)
	for i := range tf.Tasks {
		if seen[tf.Tasks[i].ID] {
			findings = append(findings, Finding{Severity: "error", Rule: "duplicate-id", Message: fmt.Sprintf("duplicate task ID: %s", tf.Tasks[i].ID)})
		}
		seen[tf.Tasks[i].ID] = true
	}
	return findings
}

// ValidateCoverage cross-references task source_sections against spec headings.
func ValidateCoverage(tf *model.TaskFile, specPath string) []Finding {
	var findings []Finding

	headings, err := ParseHeadings(specPath)
	if err != nil {
		findings = append(findings, Finding{Severity: "warning", Rule: "coverage", Message: fmt.Sprintf("could not parse spec: %v", err)})
		return findings
	}

	if tf.Coverage.TotalSections != len(headings) {
		findings = append(findings, Finding{Severity: "error", Rule: "coverage", Message: fmt.Sprintf("total_sections is %d but spec has %d headings", tf.Coverage.TotalSections, len(headings))})
	}

	headingTexts := make(map[string]bool)
	for _, h := range headings {
		prefix := strings.Repeat("#", h.Level) + " "
		headingTexts[prefix+h.Text] = true
	}

	for i := range tf.Tasks {
		for _, s := range tf.Tasks[i].SourceSections {
			if !headingTexts[s] {
				findings = append(findings, Finding{Severity: "error", Rule: "coverage", Message: fmt.Sprintf("task %s references non-existent section: %s", tf.Tasks[i].ID, s)})
			}
		}
	}

	if len(tf.Coverage.Unmapped) > 0 {
		findings = append(findings, Finding{Severity: "error", Rule: "coverage", Message: fmt.Sprintf("unmapped sections: %s", strings.Join(tf.Coverage.Unmapped, ", "))})
	}

	expected := tf.Coverage.MappedSections + len(tf.Coverage.ContextOnly) + len(tf.Coverage.Unmapped)
	if expected != tf.Coverage.TotalSections {
		findings = append(findings, Finding{Severity: "error", Rule: "coverage", Message: fmt.Sprintf("coverage arithmetic: mapped(%d) + context_only(%d) + unmapped(%d) = %d ≠ total(%d)", tf.Coverage.MappedSections, len(tf.Coverage.ContextOnly), len(tf.Coverage.Unmapped), expected, tf.Coverage.TotalSections)})
	}

	return findings
}
