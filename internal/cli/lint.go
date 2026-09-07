package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/spf13/cobra"

	"github.com/deligoez/tp/internal/engine"
	"github.com/deligoez/tp/internal/output"
)

type lintResult struct {
	File               string                     `json:"file"`
	Errors             int                        `json:"errors"`
	Warnings           int                        `json:"warnings"`
	Info               int                        `json:"info"`
	Frontmatter        lintFrontmatter            `json:"frontmatter"`
	Findings           []engine.Finding           `json:"findings"`
	StructuredElements *engine.StructuredElements `json:"structured_elements,omitempty"`
	// FloorSize and Cut are §2's two floor quantities, siblings of
	// structured_elements and emitted on every invocation. Neither carries
	// omitempty: a spec whose floor is empty must say so, and an absent key
	// reads as an older tp rather than as a zero.
	FloorSize int `json:"floor_size"`
	Cut       int `json:"cut"`
	// ReviewPanel is §2's third field: the reviewer role ids a round-1
	// `tp review <spec>` emission of this spec carries, with no
	// --diff-from. A list rather than a count, because a count cannot say
	// WHICH role is missing. No omitempty, for the same reason as the two
	// fields above, and it is built with make rather than declared: a nil
	// slice serializes as null here, and every other list field in this
	// payload answers [].
	ReviewPanel []string `json:"review_panel"`
}

// lintFrontmatter is the frontmatter object of tp lint --json output.
type lintFrontmatter struct {
	Present   bool     `json:"present"`
	Lines     *string  `json:"lines"` // "1-K", null without frontmatter
	Domain    string   `json:"domain"`
	LensRoles []string `json:"lens_roles"`
}

// lintRoleCorpusOrAbort validates every role file under .tp/reviewers and
// .tp/auditors (both phases, §3.6). A malformed or invalid role file aborts lint
// with exit 3 and a repair-or-delete hint; a valid or absent corpus is a no-op.
func lintRoleCorpusOrAbort(specPath string) {
	startDir := filepath.Dir(specPath)
	for _, phase := range []string{engine.PhaseReviewers, engine.PhaseAuditors} {
		if _, _, err := engine.LoadRoleCorpus(startDir, phase); err != nil {
			output.Error(ExitFile, err.Error(), "repair or delete the offending role file")
			os.Exit(ExitFile)
		}
	}
}

// lintReviewPanel is §2's `review_panel`: the reviewer role ids a round-1
// `tp review <spec>` emission of this spec carries, with no --diff-from.
//
// It goes through engine.ResolveRolePanel, the pure half §7's split created, so
// lint gains no second derivation of a panel tp already resolves. The wrapper in
// review.go is deliberately not the call: it refuses — exit 2 on an emptied
// reviewer phase — and it writes the advisory output.Notice loop, and lint
// describes a spec rather than refusing one or gaining a stderr channel it has
// never had (§7's table, third row).
//
// An error means the corpus would not resolve, which lintRoleCorpusOrAbort has
// already exited 3 on for a malformed role file; what survives to here is the
// embedded corpus failing to load. There is no panel to name, and lint is
// read-only, so it reports the empty list rather than guessing at one.
func lintReviewPanel(specPath string) []string {
	ids := make([]string, 0)
	panel, err := engine.ResolveRolePanel(specPath, engine.PhaseReviewers)
	if err != nil {
		return ids
	}
	for i := range panel.Roles {
		ids = append(ids, panel.Roles[i].ID)
	}
	return ids
}

func newLintCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "lint <spec.md>",
		Short: "Deterministic spec quality checks",
		Args:  cobra.ExactArgs(1),
		RunE:  runLint,
	}
}

func runLint(_ *cobra.Command, args []string) error {
	specPath := args[0]

	// A malformed or invalid role file aborts lint (§3.6), for either phase.
	lintRoleCorpusOrAbort(specPath)

	lines, headings, err := parseSpecFile(specPath)
	if err != nil {
		// tp's FIRST contact with the spec path: lintRoleCorpusOrAbort above
		// reads only .tp role files, so a failure here is a spec-path mistake
		// and takes the shared hint. Left hintless it inherited the code-3
		// default, which is TASK-file advice ("run 'tp use <file>' … 'tp init
		// <spec>'") — the wrong object for a command that takes no task file.
		output.Error(ExitFile, err.Error(), specFileMissingHint)
		os.Exit(ExitFile)
		return nil
	}

	totalLines := len(lines)

	// make, not var: a nil slice serializes as null, and tp lint on a clean spec
	// answered "findings": null where every other list field answers [].
	findings := make([]engine.Finding, 0)
	findings = append(findings, engine.CheckHeadingHierarchy(headings)...)
	findings = append(findings, engine.CheckEmptySections(headings, totalLines)...)
	findings = append(findings, engine.CheckDuplicateHeadings(headings)...)
	findings = append(findings, engine.CheckOrphanReferences(lines, headings)...)
	findings = append(findings, engine.CheckVagueLanguage(lines)...)
	findings = append(findings, engine.CheckSectionSize(headings, totalLines, 50)...)
	findings = append(findings, engine.CheckSpecSize(totalLines, 500)...)
	findings = append(findings, engine.CheckDuplicateConsecutiveLines(lines)...)
	findings = append(findings, engine.CheckNumberingGaps(headings)...)
	findings = append(findings, engine.CheckOrphanListItems(lines)...)
	findings = append(findings, engine.CheckDuplicateParagraphs(lines)...)
	findings = append(findings, engine.CheckBrokenCrossRefs(lines, headings)...)
	structFindings, structElems := engine.CheckStructuredElements(lines, headings)
	findings = append(findings, structFindings...)

	// Frontmatter state: structural errors and shape warnings are lint findings
	fm := engine.ParseFrontmatter(specPath)
	findings = append(findings, fm.Errors...)
	findings = append(findings, fm.Warnings...)
	fmObj := lintFrontmatter{
		Present:   fm.Present,
		Domain:    fm.Domain,
		LensRoles: make([]string, 0),
	}
	if fm.Present {
		l := fmt.Sprintf("%d-%d", fm.Lines.Start, fm.Lines.End)
		fmObj.Lines = &l
	}
	for _, role := range engine.LensRoleOrder {
		if len(fm.Lens[role]) > 0 {
			fmObj.LensRoles = append(fmObj.LensRoles, role)
		}
	}

	taskFindings := checkTaskFileQuality(specPath)
	findings = append(findings, taskFindings...)

	specScopeFindings := checkAffectedFilesScope(lines, headings)
	findings = append(findings, specScopeFindings...)

	// §2's two floor quantities. Both come from engine.FloorSize over the rows
	// engine.FloorIndexRows derives — the one derivation `tp ground` also
	// reads, so lint's floor_size is checkable against
	// `tp ground <spec> --units` rather than being a second definition of it.
	// §2 records the two exported functions that look like the answer and are
	// not: FloorUnits returns the candidates with the arms' cuts included, and
	// FloorIndexRows gives every candidate a row unconditionally, so their
	// difference is identically zero.
	//
	// The text is the one every other rule above reads: parseSpecFile blanks
	// the frontmatter and leaves absolute line numbers alone. `--units` reads
	// the spec's bytes as they are, so the two agree whenever the spec carries
	// no frontmatter — which is why §6 row 1's fixture carries none. They may
	// agree with one too: whether a frontmatter block moves the floor depends on
	// its CONTENT, since the arms cut a candidate unit carrying no digit, no
	// code span and no listed verb. Measured on two six-line blocks over the
	// same fixture, one moved the listing from 2 units to 3 and the other moved
	// nothing — which is why the fixture carries none rather than a harmless one.
	floorText := strings.Join(lines, "\n")
	floorRows := engine.FloorIndexRows(floorText, engine.FloorAnchorOf(floorText))
	floorSize := engine.FloorSize(floorRows)

	result := lintResult{
		File:               specPath,
		Frontmatter:        fmObj,
		Findings:           findings,
		StructuredElements: structElems,
		FloorSize:          floorSize,
		Cut:                len(floorRows) - floorSize,
		ReviewPanel:        lintReviewPanel(specPath),
	}
	for _, f := range findings {
		switch f.Severity {
		case "error":
			result.Errors++
		case "warning":
			result.Warnings++
		case "info":
			result.Info++
		}
	}

	if err := output.JSON(result); err != nil {
		// Nothing about the caller's path is wrong: tp built a result it could
		// not encode. The same hint tp review --status and tp audit --status
		// pass for this failure, and not the code-3 task-file default.
		output.Error(ExitFile, err.Error(), internalEncodeHint)
		os.Exit(ExitFile)
	}

	if result.Errors > 0 {
		os.Exit(ExitValidation)
	}
	return nil
}

func parseSpecFile(path string) ([]string, []*engine.Heading, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()

	var lines []string
	scanner := bufio.NewScanner(f) // line-cap: spec markdown, not NDJSON; a read error is returned, not swallowed
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return nil, nil, err
	}
	// Frontmatter lines are blanked so lint rules and structured element
	// extraction skip them while absolute line numbers stay untouched
	lines = engine.BlankFrontmatterLines(lines)

	headings, err := engine.ParseHeadings(path)
	if err != nil {
		return nil, nil, err
	}

	return lines, headings, nil
}

func resolveTaskFilePath(specPath string) string {
	dir := filepath.Dir(specPath)
	base := strings.TrimSuffix(filepath.Base(specPath), filepath.Ext(specPath))
	return filepath.Join(dir, base+".tasks.json")
}

func checkTaskFileQuality(specPath string) []engine.Finding {
	taskPath := resolveTaskFilePath(specPath)
	if _, err := os.Stat(taskPath); os.IsNotExist(err) {
		return nil
	}

	data, err := os.ReadFile(taskPath)
	if err != nil {
		return nil
	}

	var tf struct {
		Tasks []struct {
			Acceptance string `json:"acceptance"`
		} `json:"tasks"`
	}
	if err := json.Unmarshal(data, &tf); err != nil {
		return nil
	}

	var findings []engine.Finding

	removalVerbs := regexp.MustCompile(`(?i)\b(removed|deleted|dropped|silindi|kaldırıldı)\b`)
	completionVerbs := regexp.MustCompile(`(?i)\b(corrected|fixed|updated|adjusted|düzenlendi|güncellendi)\b`)

	for _, task := range tf.Tasks {
		acc := task.Acceptance
		if acc == "" {
			continue
		}

		words := strings.Fields(acc)
		if len(words) < 10 {
			findings = append(findings, engine.Finding{
				Severity: "info",
				Rule:     "acceptance-quality",
				Message:  fmt.Sprintf("acceptance is very short (%d words), may lack specificity", len(words)),
			})
		}

		hasRemoval := removalVerbs.MatchString(acc)
		if hasRemoval {
			findings = append(findings, engine.Finding{
				Severity: "warning",
				Rule:     "acceptance-quality",
				Message:  "acceptance describes removal but not final state",
				Context:  acc,
			})
		}

		hasCompletion := completionVerbs.MatchString(acc)
		if hasCompletion && !hasRemoval {
			findings = append(findings, engine.Finding{
				Severity: "warning",
				Rule:     "acceptance-quality",
				Message:  "acceptance uses vague completion verb without describing result",
				Context:  acc,
			})
		}
	}

	return findings
}

func checkAffectedFilesScope(lines []string, headings []*engine.Heading) []engine.Finding {
	var findings []engine.Finding

	affectedHeadingIdx := -1
	for i, h := range headings {
		lower := strings.ToLower(h.Text)
		if strings.Contains(lower, "affected") && strings.Contains(lower, "files") {
			affectedHeadingIdx = i
			break
		}
	}
	if affectedHeadingIdx == -1 {
		return findings
	}

	tableStart, tableEnd := findTableRange(lines, headings, affectedHeadingIdx)
	if tableStart < 0 {
		return findings
	}

	tableLines := lines[tableStart:tableEnd]
	if len(tableLines) == 0 {
		return findings
	}

	headerCells := parseTableRow(tableLines[0])
	if len(headerCells) == 0 {
		return findings
	}

	scopeCol := -1
	actionCol := -1
	modifyPattern := regexp.MustCompile(`(?i)^(modify|change|değiştir|güncelle)\b`)
	noChangePattern := regexp.MustCompile(`(?i)^no\b`)
	actionHeader := regexp.MustCompile(`(?i)^(action|type|change type|op)$`)
	scopeHeader := regexp.MustCompile(`(?i)^(scope|description|details|note)$`)
	for i, cell := range headerCells {
		trimmed := strings.TrimSpace(cell)
		if scopeCol < 0 && scopeHeader.MatchString(trimmed) {
			scopeCol = i
		}
		if actionCol < 0 && actionHeader.MatchString(trimmed) {
			actionCol = i
		}
	}
	if scopeCol < 0 || actionCol < 0 {
		return findings
	}

	for _, line := range tableLines[1:] {
		cells := parseTableRow(line)
		if len(cells) <= scopeCol || len(cells) <= actionCol {
			continue
		}
		actionValue := strings.TrimSpace(cells[actionCol])
		if !modifyPattern.MatchString(actionValue) || noChangePattern.MatchString(actionValue) {
			continue
		}

		scopeValue := strings.TrimSpace(cells[scopeCol])
		if len(scopeValue) < 10 {
			findings = append(findings, engine.Finding{
				Severity: "warning",
				Rule:     "affected-files-scope",
				Message:  fmt.Sprintf("affected files row with '%s' action has no scope description (need >= 10 chars)", actionValue),
				Context:  strings.Join(cells, " | "),
			})
		}
	}

	return findings
}

func findTableRange(lines []string, headings []*engine.Heading, headingIdx int) (start, end int) {
	if headingIdx >= len(headings) {
		return -1, -1
	}
	startLine := headings[headingIdx].Line

	nextHeadingLine := len(lines) + 1
	if headingIdx+1 < len(headings) {
		nextHeadingLine = headings[headingIdx+1].Line
	}

	tableStart := -1
	for i := startLine; i < nextHeadingLine && i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "" {
			continue
		}
		if strings.HasPrefix(strings.TrimSpace(lines[i]), "|") && strings.HasSuffix(strings.TrimSpace(lines[i]), "|") {
			tableStart = i
			break
		}
	}
	if tableStart < 0 {
		return -1, -1
	}

	tableEnd := tableStart + 1
	for i := tableStart + 1; i < nextHeadingLine && i < len(lines); i++ {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed == "" {
			continue
		}
		if !strings.HasPrefix(trimmed, "|") {
			break
		}
		tableEnd = i + 1
	}

	return tableStart, tableEnd
}

func parseTableRow(line string) []string {
	trimmed := strings.TrimSpace(line)
	trimmed = strings.TrimPrefix(trimmed, "|")
	trimmed = strings.TrimSuffix(trimmed, "|")
	parts := strings.Split(trimmed, "|")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		result = append(result, strings.TrimSpace(p))
	}
	return result
}
