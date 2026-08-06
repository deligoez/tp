package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/deligoez/tp/internal/engine"
	"github.com/deligoez/tp/internal/model"
)

// roleSpecCoverage is the one auditor id with a dedicated routing path: it
// alone receives the spec-derived checklist and its own file selection.
const roleSpecCoverage = "spec-coverage"

const claudeMDExcerptLineCap = 50

// auditDispositionBlock is the §2.2 paragraph, rendered verbatim once per
// shared-arm prompt right after the checklist: it tells a role how to dispose
// of a file holding nothing in its domain, so an out-of-lens file is recorded
// as PASS instead of manufacturing a false blocker.
const auditDispositionBlock = "## Disposition\n" +
	"A file containing nothing in this role's domain is a PASS, not a PARTIAL. Record it as\n" +
	"PASS with evidence_file set to that path and evidence_lines set to the full range you\n" +
	"read (for example \"1-120\"), meaning: the whole file was inspected and nothing in this\n" +
	"role's domain appears in it. Reserve PARTIAL and FAIL for a defect you actually found.\n"

// routeChecklist builds the spec-coverage checklist: spec-derived and finding
// items in the pinned order table_row and list_item ascending by spec_line,
// then task_acceptance in task-file order, then finding by index. Every other
// role's items are synthetic file_check items built in the shared arm of
// generateRoleAuditPrompts.
func routeChecklist(specEntries, findingsEntries []checklistEntry, taskToFiles map[string][]string) []ChecklistItem {
	structural := make([]checklistEntry, 0, len(specEntries))
	taskItems := make([]checklistEntry, 0)
	for _, e := range specEntries {
		if e.Type == "task_acceptance" {
			taskItems = append(taskItems, e)
		} else {
			structural = append(structural, e)
		}
	}
	sort.SliceStable(structural, func(i, j int) bool { return structural[i].SpecLine < structural[j].SpecLine })

	spec := make([]ChecklistItem, 0, len(specEntries)+len(findingsEntries))
	for i := range structural {
		spec = append(spec, specItemOf(&structural[i], taskToFiles))
	}
	for i := range taskItems {
		spec = append(spec, specItemOf(&taskItems[i], taskToFiles))
	}
	for i := range findingsEntries {
		spec = append(spec, specItemOf(&findingsEntries[i], taskToFiles))
	}

	return spec
}

// specItemOf converts a spec-derived or finding entry into a checklist item
// with its deterministic expected_evidence.
func specItemOf(e *checklistEntry, taskToFiles map[string][]string) ChecklistItem {
	evidence := fmt.Sprintf("search code under section %q for keywords from item text", e.Section)
	switch e.Type {
	case "task_acceptance":
		taskID := strings.TrimPrefix(e.ID, "task-")
		if paths := taskToFiles[taskID]; len(paths) > 0 {
			evidence = "files changed by task commit: " + strings.Join(paths, ", ")
		}
	case "finding":
		text := e.Text
		if len(text) > 120 {
			text = text[:120]
		}
		evidence = "verify the fix for: " + text
	}
	return ChecklistItem{
		ItemID:           e.ID,
		Type:             e.Type,
		SpecLine:         e.SpecLine,
		Section:          e.Section,
		Text:             e.Text,
		ExpectedEvidence: evidence,
	}
}

// fileCheckItems builds one synthetic checklist item per affected file. Each
// item's id is file-<roleID>-<slug>, where <slug> derives from the item's
// subject (file path + checklist text) so the same file keeps the same id
// across rounds regardless of position (§10.3). Collisions — two items
// yielding the same slug — get a -2, -3, … suffix.
func fileCheckItems(files []engine.AuditFileEntry, roleID string) []ChecklistItem {
	items := make([]ChecklistItem, 0, len(files))
	seen := make(map[string]int)
	prefix := "file-" + roleID + "-"
	for _, f := range files {
		text := fmt.Sprintf("Apply the %s role rules to %s", roleID, f.Path)
		slug := slugifySubject(f.Path + " " + text)
		count := seen[slug]
		seen[slug] = count + 1
		itemID := prefix + slug
		if count > 0 {
			itemID = fmt.Sprintf("%s%s-%d", prefix, slug, count+1)
		}
		items = append(items, ChecklistItem{
			ItemID:           itemID,
			Type:             "file_check",
			SpecLine:         0,
			Section:          f.Path,
			Text:             text,
			ExpectedEvidence: "inspect file: " + f.Path,
		})
	}
	return items
}

// slugifySubject derives a stable slug from an audit item's subject text
// (§10.3): the subject is lowercased; runs of non-alphanumeric characters
// collapse to a single "-"; leading and trailing "-" are trimmed; then the
// result is truncated to at most 40 characters with any trailing "-" trimmed.
// Because the subject always includes a file path, the slug always contains at
// least one letter (§10.9).
func slugifySubject(s string) string {
	s = strings.ToLower(s)
	var b strings.Builder
	dash := false
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			dash = false
			continue
		}
		if !dash {
			b.WriteRune('-')
			dash = true
		}
	}
	slug := strings.Trim(b.String(), "-")
	if len(slug) > 40 {
		slug = strings.TrimRight(slug[:40], "-")
	}
	return slug
}

// invertTaskFiles converts path->tasks into task->sorted paths.
func invertTaskFiles(taskFiles map[string][]string) map[string][]string {
	out := make(map[string][]string)
	for path, ids := range taskFiles {
		for _, id := range ids {
			out[id] = append(out[id], path)
		}
	}
	for id := range out {
		sort.Strings(out[id])
	}
	return out
}

// priorAuditRow is one of a role's own non-PASS rows from the previous
// recorded audit round, carried into that role's next-round prompt (§10.2):
// the auditor re-checks each item rather than repeating the prior verdict.
// ChangedSince is nil (omitted) for rows with no file path; it is non-nil
// (true or false) when the row carries an evidence_file.
type priorAuditRow struct {
	Role         string `json:"role"`
	ItemID       string `json:"item_id"`
	Status       string `json:"status"`
	EvidenceFile string `json:"evidence_file,omitempty"`
	ChangedSince *bool  `json:"changed_since,omitempty"`
}

// auditPriorRound is the role-scoped prior-round context embedded in a
// round-2+ audit prompt (§10.2): that role's own non-PASS rows from the
// previous recorded round. legacy is true when the prior round was recorded
// before stable item ids (no id_scheme marker, §10.9), so its ids are
// positional and not comparable to this round's.
type auditPriorRound struct {
	rows   []priorAuditRow
	legacy bool
}

// renderPriorRoundSection renders the role-scoped prior-round section for a
// round-2+ audit prompt (§10.2): that role's own non-PASS rows from the
// previous recorded round, framed as context to re-check — not a verdict to
// repeat. Returns "" when the role has no prior non-PASS rows, so a round-1
// prompt (or a role that was all-PASS) carries no section at all.
func renderPriorRoundSection(prior *auditPriorRound) string {
	if prior == nil || len(prior.rows) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("\n## Prior Round: context to re-check, not a verdict to repeat\n")
	b.WriteString("These are your own non-PASS rows from the previous round. Re-check each item against the code and record your own status. Do NOT repeat the prior verdict without verifying.\n\n")
	if prior.legacy {
		b.WriteString("Note: the prior round was recorded before stable item ids, so these ids are positional (file-<role>-<n>) and NOT comparable to this round's stable ids.\n\n")
	}
	for _, r := range prior.rows {
		data, _ := json.Marshal(r)
		b.Write(data)
		b.WriteString("\n")
	}
	b.WriteString("\n")
	return b.String()
}

// buildRolePrompt renders the §3.1 body order for one role, drawing its Role
// Rules from the corpus role's focus (§7.2) rather than a hardcoded map.
func buildRolePrompt(role string, rules []string, items []ChecklistItem, files []engine.AuditFileEntry, fileCap, fileTotal int, specContent, claudeExcerpt string, prior *auditPriorRound, f *promptFraming, inlinedContent string) auditPrompt {
	var b strings.Builder
	b.WriteString("## Role\n" + role + "\n\n")

	b.WriteString("## Role Rules\n")
	for _, r := range rules {
		b.WriteString("- " + r + "\n")
	}
	b.WriteString("\n")

	if role == roleSpecCoverage {
		b.WriteString("## Spec Excerpt\n" + specContent + "\n\n")
	}
	// §2.3: every role taking the shared arm receives the project conventions;
	// spec-coverage judges against the ## Spec Excerpt block instead.
	if role != roleSpecCoverage && claudeExcerpt != "" {
		b.WriteString("## Project Context\n" + claudeExcerpt + "\n\n")
	}

	b.WriteString("## Checklist\n[\n")
	for i, item := range items {
		data, _ := json.Marshal(item)
		b.Write(data)
		if i < len(items)-1 {
			b.WriteString(",")
		}
		b.WriteString("\n")
	}
	b.WriteString("]\n\n")

	// §2.2: shared-arm roles only — spec-coverage's checklist holds spec
	// elements, over which the out-of-domain disposition is meaningless.
	if role != roleSpecCoverage {
		b.WriteString(auditDispositionBlock + "\n")
	}

	b.WriteString(renderPriorRoundSection(prior))

	// §2.6: the header states the role's own cap, or the applied count against
	// the pre-cap total once the cap has bitten — never a fixed literal 20.
	if fileTotal <= len(files) {
		fmt.Fprintf(&b, "## Affected Files (max %d)\n", fileCap)
	} else {
		fmt.Fprintf(&b, "## Affected Files (%d of %d)\n", len(files), fileTotal)
	}
	for _, fl := range files {
		if role == roleSpecCoverage && len(fl.Tasks) > 0 {
			fmt.Fprintf(&b, "- %s (tasks: %s; diff: %s)\n", fl.Path, strings.Join(fl.Tasks, ", "), fl.DiffSummary)
		} else {
			fmt.Fprintf(&b, "- %s (diff: %s)\n", fl.Path, fl.DiffSummary)
		}
	}
	b.WriteString("\n")

	b.WriteString(renderAuditOutputSchema())
	b.WriteString(outputContractInstruction(role, engine.PhaseAuditors))

	// §10.7: the inliner role's file contents are inlined whole (complete and
	// authoritative); every other role gets named paths only (stated below).
	if inlinedContent != "" {
		b.WriteString("\n" + inlinedContent)
	}
	b.WriteString(renderFraming(f))

	return auditPrompt{
		Role:           role,
		OutputPath:     f.outputPath,
		Prompt:         b.String(),
		ChecklistCount: len(items),
		ChecklistItems: items,
		AffectedFiles:  files,
	}
}

// generateRoleAuditPrompts emits one prompt per non-empty role in corpus
// order. Only spec-coverage routes to a dedicated checklist and file
// selection; every other role takes the shared arm. A role whose routed
// checklist is empty produces no prompt and is named in skipped_roles
// with reason no-checklist-items (§9.1).
func generateRoleAuditPrompts(auditorRoles []model.Role, specItems []ChecklistItem, sel *engine.AuditFileSelection, specContent, claudeExcerpt string, priorByRole map[string]*auditPriorRound, round, requiredClean, consecutiveClean, maxRounds int) ([]auditPrompt, []engine.SkippedRole) {
	prompts := make([]auditPrompt, 0, len(auditorRoles))
	skipped := make([]engine.SkippedRole, 0)
	// §10.7 per-role inliner: the first emitted role whose files fit whole under
	// the per-role reading budget inlines them (complete and authoritative);
	// every later role, and any role that exceeds the budget, gets named paths
	// only. Set once, never reset.
	inlinerDone := false
	for i := range auditorRoles {
		role := &auditorRoles[i]
		var items []ChecklistItem
		var files []engine.AuditFileEntry
		var fileCap, fileTotal int
		switch role.ID {
		case roleSpecCoverage:
			items, files = specItems, sel.SpecCoverage
			fileCap, fileTotal = engine.AuditFileCap, sel.SpecCoverageTotal
		default:
			files = sel.CodeFiles
			items = fileCheckItems(files, role.ID)
			fileCap, fileTotal = engine.CodeFileCap, sel.CodeFilesTotal
		}
		if len(items) == 0 {
			skipped = append(skipped, engine.SkippedRole{Role: role.ID, Reason: engine.SkipNoChecklistItems})
			continue
		}
		filePaths := make([]string, 0, len(files))
		for _, fl := range files {
			filePaths = append(filePaths, fl.Path)
		}
		f := promptFraming{
			phase:            "audit",
			round:            round,
			requiredClean:    requiredClean,
			consecutiveClean: consecutiveClean,
			maxRounds:        maxRounds,
			outputPath:       fmt.Sprintf("audit-r%d-%s.ndjson", round, role.ID),
			hasFiles:         len(filePaths) > 0,
		}
		inlinedContent := ""
		if f.hasFiles {
			// A path tp cannot read is never presented as complete (§10.7):
			// the role is told to read that path itself rather than judging a
			// body it never received. Only the failed path is withheld — the
			// files tp did read stay inlined under the "(incomplete)" header,
			// so one locked file does not send every role back to disk for the
			// whole set.
			inlined := false
			if !inlinerDone {
				if fileSetBytes(filePaths) <= perRoleReadingBudget {
					var unreadable []string
					inlinedContent, unreadable = fileSetRead(filePaths)
					f.filesComplete = len(unreadable) == 0
					f.filesPartial = !f.filesComplete
					f.filePaths = unreadable
					inlinerDone, inlined = true, true
				}
			}
			if !inlined {
				f.filePaths = filePaths
			}
		}
		prompts = append(prompts, buildRolePrompt(role.ID, role.Focus, items, files, fileCap, fileTotal, specContent, claudeExcerpt, priorByRole[role.ID], &f, inlinedContent))
	}
	return prompts, skipped
}

// claudeMDExcerptFor resolves CLAUDE.md next to the resolved task file, then
// in the git repository root, and returns the ## Conventions section span
// (capped at 50 lines) or the first 50 lines. Empty when CLAUDE.md exists in
// neither place.
func claudeMDExcerptFor(specPath string) string {
	candidates := make([]string, 0, 2)
	if _, tfPath := engine.ResolveWorkflow(specPath, flagFile); tfPath != "" {
		candidates = append(candidates, filepath.Join(filepath.Dir(tfPath), "CLAUDE.md"))
	}
	if out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output(); err == nil {
		candidates = append(candidates, filepath.Join(strings.TrimSpace(string(out)), "CLAUDE.md"))
	}
	for _, c := range candidates {
		data, err := os.ReadFile(c)
		if err != nil {
			// An absent CLAUDE.md is the normal case (the excerpt is
			// optional). One that exists but cannot be read silently costs
			// every role its conventions context, so name it.
			if !os.IsNotExist(err) {
				fmt.Fprintf(os.Stderr, "warning: cannot read %s; the conventions excerpt was dropped (%v)\n", c, err)
			}
			continue
		}
		return claudeConventionsExcerpt(strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n"))
	}
	return ""
}

// claudeConventionsExcerpt returns the ## Conventions section span (through
// the line before the next same-or-higher-level heading), capped at 50 lines;
// without that heading, the first 50 lines.
func claudeConventionsExcerpt(lines []string) string {
	start := -1
	for i, line := range lines {
		if strings.TrimSpace(line) == "## Conventions" {
			start = i
			break
		}
	}
	var span []string
	if start >= 0 {
		span = append(span, lines[start])
		for i := start + 1; i < len(lines); i++ {
			trimmed := strings.TrimSpace(lines[i])
			if strings.HasPrefix(trimmed, "# ") || strings.HasPrefix(trimmed, "## ") {
				break
			}
			span = append(span, lines[i])
		}
	} else {
		span = lines
	}
	if len(span) > claudeMDExcerptLineCap {
		span = span[:claudeMDExcerptLineCap]
	}
	return strings.Join(span, "\n")
}

// auditDiffStats parses `git diff --numstat` into path -> {added, deleted}.
func auditDiffStats(base string) map[string][2]int {
	args := []string{"diff", "--numstat"}
	if base != "" && engine.SafeGitRev(base) {
		args = append(args, base)
	}
	out, err := exec.Command("git", args...).Output()
	stats := make(map[string][2]int)
	if err != nil {
		return stats
	}
	for _, line := range strings.Split(string(out), "\n") {
		parts := strings.Fields(line)
		if len(parts) < 3 {
			continue
		}
		added, aErr := strconv.Atoi(parts[0])
		deleted, dErr := strconv.Atoi(parts[1])
		if aErr != nil || dErr != nil {
			continue // binary entries use "-"
		}
		stats[strings.Join(parts[2:], " ")] = [2]int{added, deleted}
	}
	return stats
}

// auditDeletedFiles lists files deleted in the diff.
func auditDeletedFiles(base string) map[string]bool {
	args := []string{"diff", "--name-only", "--diff-filter=D"}
	if base != "" && engine.SafeGitRev(base) {
		args = append(args, base)
	}
	out, err := exec.Command("git", args...).Output()
	deleted := make(map[string]bool)
	if err != nil {
		return deleted
	}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line != "" {
			deleted[line] = true
		}
	}
	return deleted
}

// auditTasksOf reads the spec-adjacent task file's tasks; empty when absent.
func auditTasksOf(specPath string) []model.Task {
	taskPath := strings.TrimSuffix(specPath, filepath.Ext(specPath)) + ".tasks.json"
	tf, err := model.ReadTaskFile(taskPath)
	if err != nil {
		// An absent task file is the normal spec-without-tasks case. A real
		// read or parse error is surfaced: callers build user-facing claims on
		// this result, and a swallowed error turns a corrupt task file into
		// "no done task carries commit_shas" — the wrong problem.
		//
		// errors.Is, not os.IsNotExist: ReadTaskFile wraps with %w and
		// os.IsNotExist does not unwrap, so the guard would never fire and the
		// warning would print on every ordinary run.
		if !errors.Is(err, os.ErrNotExist) {
			fmt.Fprintf(os.Stderr, "warning: cannot read task file %s; treating it as empty (%v)\n", taskPath, err)
		}
		return nil
	}
	return tf.Tasks
}
