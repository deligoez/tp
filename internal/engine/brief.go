package engine

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/deligoez/tp/internal/model"
)

// OutScopePrefix marks the optional trailing out-of-scope line in a closure
// reason (§7.2). A closing unit appends it to report a finding it noticed
// outside the scope fence; tp preserves the line verbatim in closed_reason
// and surfaces it in tp report so it reaches a human.
const OutScopePrefix = "Out of scope:"

// ScopeFenceText returns the scope-fence statement every implementation
// brief carries in its Scope section (§7.1). The acceptance criteria are the
// boundary: only they are implemented; code outside them is not touched; the
// task file and .tp-review are not hand-edited; and a real out-of-fence
// problem is reported (a trailing "Out of scope:" line) rather than fixed.
// tp brief assembles this text; it is a statement, not a commit-path check.
func ScopeFenceText() string {
	return `Implement only this task's acceptance criteria. Do not refactor, rename, reformat, or "clean up" code outside them. Do not hand-edit the task file or anything under .tp-review/. If you find a real problem outside the fence, report it in the closure evidence as a trailing "Out of scope:" line instead of fixing it.`
}

// ExtractOutOfScope returns the out-of-scope note a closure reason carries in
// its "Out of scope:" line (§7.2), or "" when the reason has none. Only the
// text following the prefix is returned, trimmed; the prefix itself is not, so
// a report field named for the concept does not duplicate it. tp report uses
// this to surface out-of-fence findings a closing unit recorded.
func ExtractOutOfScope(closedReason string) string {
	for _, line := range strings.Split(closedReason, "\n") {
		if !strings.HasPrefix(line, OutScopePrefix) {
			continue
		}
		return strings.TrimSpace(strings.TrimPrefix(line, OutScopePrefix))
	}
	return ""
}

// CloseRecipeText builds the "How to close" section text an implementation
// brief carries (§8). effectiveStrategy is the already-resolved concrete
// committing behavior — CommitStrategyBuiltin or CommitStrategyHC; an "auto"
// name is resolved to one of these by the caller via EffectiveCommitStrategy
// before this call, so the recipe never holds an "auto" branch (§8.1).
// qualityGate is the resolved quality_gate command verbatim (§8.2). The recipe
// states the exact close command, the red-gate rule, the evidence contract,
// the "--" separator, and — under hc — the rejected invocations (§8.4). tp
// brief assembles this text; it is a statement of the close path, not a check
// tp runs.
func CloseRecipeText(effectiveStrategy, qualityGate string) string {
	command, rejected := closeRecipeCommand(effectiveStrategy)

	var b strings.Builder
	b.WriteString("How to close this unit.\n\n")

	// §8.2: the resolved gate verbatim, and the rule a red gate is never closed over.
	b.WriteString("1. Run the gate and confirm it is green before closing. A red gate is never closed over: --skip-gate is a human decision, never the unit's.\n")
	if qualityGate != "" {
		fmt.Fprintf(&b, "   quality_gate: %s\n", qualityGate)
	} else {
		b.WriteString("   quality_gate: (none configured)\n")
	}
	b.WriteString("\n")

	// §8.1: the exact close command for the effective commit_strategy.
	b.WriteString("2. Close the task with the exact command for the effective commit_strategy:\n")
	b.WriteString(command)
	b.WriteString("\n")

	// §8.3: the closure reason is an evidence contract.
	b.WriteString("3. Write the closure reason as an evidence contract:\n")
	b.WriteString("   - One line per acceptance criterion, each stating what was implemented and how it was verified.\n")
	b.WriteString("   - No bare \"done\", \"wip\", or \"deferred\".\n")
	b.WriteString("   - Written in English.\n")
	b.WriteString("   - The first line becomes the next unit's summary of this work, so lead with the substantive claim.\n")
	b.WriteString("   - An optional trailing \"Out of scope:\" line reports a finding outside the fence instead of fixing it.\n\n")

	// §8.4: the "--" separator precedes the reason in both recipes.
	b.WriteString("The \"--\" separator precedes the reason because a reason often starts with \"- \" (a bullet), which a flag parser would otherwise treat as a flag.\n")

	// §8.4: under hc a bare tp done, tp commit, and --auto-commit are rejected.
	if rejected != "" {
		b.WriteString("\n")
		b.WriteString(rejected)
		b.WriteString("\n")
	}

	return b.String()
}

// closeRecipeCommand returns the exact close command text for the effective
// strategy (§8.1) and, for hc only, the rejected-invocation note (§8.4). An
// unrecognized strategy resolves to the builtin recipe, matching
// EffectiveCommitStrategy's default-to-builtin behavior.
func closeRecipeCommand(effectiveStrategy string) (command, rejected string) {
	switch effectiveStrategy {
	case CommitStrategyHC:
		command = "   commit with hc, then:\n   tp done <id> --commit <sha> -- \"<evidence>\"   (repeat --commit per sha)\n"
		rejected = "Under hc, a bare \"tp done\", \"tp commit\", and \"--auto-commit\" are all rejected: make the commit with hc, then record the SHA with \"tp done <id> --commit <sha>\"."
	default:
		command = "   tp done <id> --auto-commit -- \"<evidence>\"\n"
		rejected = ""
	}
	return command, rejected
}

// --- Prior-work selection (§5) ---

// Prior-work selection bounds and defaults (§5).
const (
	// PriorMin/PriorMax bound the --prior flag's accepted range (§5.4); an
	// out-of-range value is a usage error the CLI maps to exit 2.
	PriorMin = 0
	PriorMax = 20
	// priorDefaultRecency is the recency count when --prior is absent (§5.1).
	priorDefaultRecency = 5
	// priorDefaultTotal is the total-entry cap when --prior is absent (§5.4):
	// dependency-derived entries plus recency, with deps always included even
	// when they alone exceed it.
	priorDefaultTotal = 12
	// priorWorkFileCap is the maximum commit_files paths shown per prior-work
	// entry (§5.2); the remainder is summarized as a "+N more" count.
	priorWorkFileCap = 5
)

// PriorWorkEntry is a single prior-work entry a brief carries (§5.2): the task
// id, title, the first line of closed_reason (the evidence summary the closing
// unit wrote), the commits that closed it, and the files those commits touched
// capped at five paths with a "+N more" count and the true file total.
type PriorWorkEntry struct {
	ID               string   `json:"id"`
	Title            string   `json:"title,omitempty"`
	EvidenceSummary  string   `json:"evidence_summary"`
	CommitSHAs       []string `json:"commit_shas,omitempty"`
	CommitFiles      []string `json:"commit_files,omitempty"`
	CommitFilesMore  int      `json:"commit_files_more,omitempty"`
	CommitFilesTotal int      `json:"commit_files_total,omitempty"`
}

// PriorWorkResult is the brief's prior-work selection (§5): the ordered
// entries (done transitive dependencies first, then most-recently-closed done
// tasks), whether this is the first unit of the project, and the count of
// recency entries dropped by the total cap or --prior.
type PriorWorkResult struct {
	Entries      []PriorWorkEntry `json:"entries"`
	IsFirstUnit  bool             `json:"is_first_unit"`
	OmittedCount int              `json:"omitted_count,omitempty"`
}

// ValidatePriorCount checks a --prior value against the documented range
// (§5.4). It returns an error the CLI maps to exit 2 (usage) when priorSet is
// true and the value is outside [PriorMin, PriorMax]. An unset --prior (the
// default) always passes.
func ValidatePriorCount(prior int, priorSet bool) error {
	if priorSet && (prior < PriorMin || prior > PriorMax) {
		return fmt.Errorf("--prior %d is out of range [%d, %d]", prior, PriorMin, PriorMax)
	}
	return nil
}

// SelectPriorWork computes the brief's prior-work selection (§5) for the task
// identified by taskID. The selection is, in order and deduplicated:
//  1. Every done task taskID transitively depends on — always included, even
//     when that count exceeds the default 12-entry total cap — in
//     dependency-safe order.
//  2. The most recently closed done tasks (adjacent context the dependency
//     graph does not capture), up to the recency limit.
//
// When priorSet is false the recency limit is priorDefaultRecency (5), with the
// combined entry count capped at priorDefaultTotal (12); dependency-derived
// entries are never dropped to satisfy the cap. When priorSet is true the
// recency limit is priorCount (the 12-entry cap is overridden, up to 20) and
// dependency-derived entries remain always included on top. An out-of-range
// priorCount returns an error the caller maps to exit 2.
func SelectPriorWork(tf *model.TaskFile, taskID string, priorCount int, priorSet bool) (*PriorWorkResult, error) {
	if err := ValidatePriorCount(priorCount, priorSet); err != nil {
		return nil, err
	}

	byID := indexTasksByID(tf)
	target, ok := byID[taskID]
	if !ok {
		return nil, fmt.Errorf("task %q not found", taskID)
	}

	// §5.1.1: every done transitive dependency, in dependency-safe order.
	depSet := doneTransitiveDeps(target, byID)
	depOrder := topoOrderSubset(depSet)

	// §5.1.2: the most recently closed done tasks not already in the dep set.
	candidates := recencyCandidates(tf, taskID, depSet)
	sortRecencyDesc(candidates)

	recencyLimit := resolveRecencyLimit(len(depOrder), priorCount, priorSet)
	if recencyLimit > len(candidates) {
		recencyLimit = len(candidates)
	}
	omitted := len(candidates) - recencyLimit
	if omitted < 0 {
		omitted = 0
	}

	entries := make([]PriorWorkEntry, 0, len(depOrder)+recencyLimit)
	for _, t := range depOrder {
		entries = append(entries, buildPriorWorkEntry(t))
	}
	for i := 0; i < recencyLimit; i++ {
		entries = append(entries, buildPriorWorkEntry(&candidates[i]))
	}

	return &PriorWorkResult{
		Entries:      entries,
		IsFirstUnit:  len(depSet) == 0 && len(candidates) == 0,
		OmittedCount: omitted,
	}, nil
}

// indexTasksByID returns a lookup of task id → *Task pointing into tf.Tasks.
func indexTasksByID(tf *model.TaskFile) map[string]*model.Task {
	m := make(map[string]*model.Task, len(tf.Tasks))
	for i := range tf.Tasks {
		m[tf.Tasks[i].ID] = &tf.Tasks[i]
	}
	return m
}

// doneTransitiveDeps returns the set of DONE tasks reachable from target via
// depends_on (the transitive closure), excluding target itself. The walk
// traverses every status — a not-yet-done dependency still transitively links
// to the done tasks beneath it — and filters to done at the end (§5.1.1).
func doneTransitiveDeps(target *model.Task, byID map[string]*model.Task) map[string]*model.Task {
	seen := make(map[string]bool)
	stack := append([]string(nil), target.DependsOn...)
	for len(stack) > 0 {
		dep := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if dep == target.ID || seen[dep] {
			continue
		}
		seen[dep] = true
		if t, ok := byID[dep]; ok {
			stack = append(stack, t.DependsOn...)
		}
	}
	done := make(map[string]*model.Task, len(seen))
	for id := range seen {
		if t, ok := byID[id]; ok && t.Status == model.StatusDone {
			done[id] = t
		}
	}
	return done
}

// topoOrderSubset returns the subset's tasks in dependency-safe order (a task
// appears after the tasks it depends on), with ids as the deterministic
// tiebreak — mirroring TopoSort's alphabetical determinism. Any member a
// (theoretically impossible) cycle leaves unprocessed is appended sorted, so
// the result is always complete.
func topoOrderSubset(subset map[string]*model.Task) []*model.Task {
	inDegree := make(map[string]int, len(subset))
	dependents := make(map[string][]string)
	for id, t := range subset {
		inDegree[id] = 0
		for _, d := range t.DependsOn {
			if _, ok := subset[d]; ok {
				inDegree[id]++
				dependents[d] = append(dependents[d], id)
			}
		}
	}

	var queue []string
	for id, deg := range inDegree {
		if deg == 0 {
			queue = append(queue, id)
		}
	}
	sort.Strings(queue)

	ordered := make([]*model.Task, 0, len(subset))
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		ordered = append(ordered, subset[id])
		for _, dep := range dependents[id] {
			inDegree[dep]--
			if inDegree[dep] == 0 {
				queue = append(queue, dep)
			}
		}
		sort.Strings(queue)
	}

	// Completeness safety net: append any member a cycle left unprocessed.
	if len(ordered) < len(subset) {
		placed := make(map[string]bool, len(ordered))
		for _, t := range ordered {
			placed[t.ID] = true
		}
		var leftover []*model.Task
		for _, t := range subset {
			if !placed[t.ID] {
				leftover = append(leftover, t)
			}
		}
		sort.Slice(leftover, func(i, j int) bool { return leftover[i].ID < leftover[j].ID })
		ordered = append(ordered, leftover...)
	}
	return ordered
}

// recencyCandidates returns the done tasks that are neither the target nor in
// the dependency set — the adjacent-context pool recency draws from (§5.1.2).
func recencyCandidates(tf *model.TaskFile, taskID string, depSet map[string]*model.Task) []model.Task {
	var out []model.Task
	for i := range tf.Tasks {
		t := tf.Tasks[i]
		if t.Status != model.StatusDone || t.ID == taskID {
			continue
		}
		if _, ok := depSet[t.ID]; ok {
			continue
		}
		out = append(out, t)
	}
	return out
}

// sortRecencyDesc orders recency candidates most-recently-closed first, with
// the id as a deterministic tiebreak for equal closure times.
func sortRecencyDesc(candidates []model.Task) {
	sort.Slice(candidates, func(i, j int) bool {
		ti := derefTime(candidates[i].ClosedAt)
		tj := derefTime(candidates[j].ClosedAt)
		if !ti.Equal(tj) {
			return ti.After(tj)
		}
		return candidates[i].ID < candidates[j].ID
	})
}

// resolveRecencyLimit returns how many recency entries fit given the done
// dependency count. With --prior set it is priorCount (the 12-entry cap is
// overridden). With the default it is min(recency count 5, room left toward the
// 12-entry total cap), floored at zero; deps are always included beyond the
// cap, so room simply goes to zero rather than negative.
func resolveRecencyLimit(depCount, priorCount int, priorSet bool) int {
	if priorSet {
		return priorCount
	}
	room := priorDefaultTotal - depCount
	if room < 0 {
		room = 0
	}
	if priorDefaultRecency < room {
		return priorDefaultRecency
	}
	return room
}

// buildPriorWorkEntry assembles a brief prior-work entry from a done task
// (§5.2): id, title, the first line of closed_reason, its commit_shas, and the
// touched files capped at five with a "+N more" count and the true total.
func buildPriorWorkEntry(t *model.Task) PriorWorkEntry {
	return PriorWorkEntry{
		ID:               t.ID,
		Title:            t.Title,
		EvidenceSummary:  firstLineOfString(derefStr(t.ClosedReason)),
		CommitSHAs:       t.CommitSHAs,
		CommitFiles:      firstNStrings(t.CommitFiles, priorWorkFileCap),
		CommitFilesMore:  priorWorkFileMore(t),
		CommitFilesTotal: priorWorkFileTotal(t),
	}
}

// firstLineOfString returns the substring before the first newline, or the
// whole string when it has none (§5.2 "the first line of closed_reason").
func firstLineOfString(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

// priorWorkFileTotal is the true count of files the task's commits touched:
// the stored CommitFilesTotal when the 50-path cap (§6.4) applied, else the
// stored array length (§5.2 "the entry still carries commit_files_total").
func priorWorkFileTotal(t *model.Task) int {
	if t.CommitFilesTotal > 0 {
		return t.CommitFilesTotal
	}
	return len(t.CommitFiles)
}

// priorWorkFileMore is the "+N more" count when more than priorWorkFileCap
// paths exist (§5.2): the true total minus the (up to five) shown paths.
func priorWorkFileMore(t *model.Task) int {
	total := priorWorkFileTotal(t)
	shown := len(t.CommitFiles)
	if shown > priorWorkFileCap {
		shown = priorWorkFileCap
	}
	if total > shown {
		return total - shown
	}
	return 0
}

// firstNStrings returns up to n entries of s as a fresh slice (nil when s is
// empty), so a truncated entry never aliases the source task's slice.
func firstNStrings(s []string, n int) []string {
	if len(s) == 0 {
		return nil
	}
	if len(s) <= n {
		return append([]string(nil), s...)
	}
	return append([]string(nil), s[:n]...)
}

func derefStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func derefTime(t *time.Time) time.Time {
	if t == nil {
		return time.Time{}
	}
	return *t
}

// --- Brief assembly (§3, §4) ---

// identityOneLine is the one-line reset discipline every brief's identity
// carries (§4.2, §4.4): the rule a fresh unit cannot infer from a task record.
// It is the JSON identity value and the compact identity line.
const identityOneLine = "This agent executes one unit and stops; it does not pick up a second task; work not on disk when it returns is lost."

// BriefTask is the "Your unit" machine part of a brief (§4.1): the task id, the
// acceptance text verbatim from the task file (§4.3), the anchors
// (source_sections and source_lines), and the spec_excerpt. spec_excerpt is
// omitted under --compact (§12.1); the acceptance text never is.
type BriefTask struct {
	ID             string   `json:"id"`
	Title          string   `json:"title,omitempty"`
	Acceptance     string   `json:"acceptance"`
	SourceSections []string `json:"source_sections,omitempty"`
	SourceLines    string   `json:"source_lines,omitempty"`
	SpecExcerpt    string   `json:"spec_excerpt,omitempty"`
}

// Brief is the structured machine-readable parts of an implementation brief
// (§4.4): {identity, task, prior_work, close, scope}, in that order. identity is
// the one-line reset discipline; scope is the scope-fence prohibitions
// (ScopeFenceText); close is the close recipe (CloseRecipeText); task and
// prior_work carry the unit and its prior-work selection. The assembled text
// brief (the default output) is built from these same parts by Text.
type Brief struct {
	Identity  string           `json:"identity"`
	Task      BriefTask        `json:"task"`
	PriorWork *PriorWorkResult `json:"prior_work"`
	Close     string           `json:"close"`
	Scope     string           `json:"scope"`

	compact bool
}

// BuildBrief assembles the brief's machine parts (§4.4) for task from tf. The
// effectiveStrategy is the already-resolved concrete committing behavior
// (builtin or hc); qualityGate is the resolved quality_gate command verbatim.
// priorCount/priorSet drive prior-work selection (§5); when priorSet is false
// SelectPriorWork applies its defaults. Under compact, spec_excerpt is omitted
// and prior-work entries collapse to id and evidence summary (§12.1); the
// acceptance text, close recipe, and scope prohibitions are always present.
func BuildBrief(tf *model.TaskFile, task *model.Task, specPath, effectiveStrategy, qualityGate string, priorCount int, priorSet, compact bool) (*Brief, error) {
	prior, err := SelectPriorWork(tf, task.ID, priorCount, priorSet)
	if err != nil {
		return nil, err
	}
	if compact {
		prior = compactPriorWork(prior)
	}

	bt := BriefTask{
		ID:             task.ID,
		Title:          task.Title,
		Acceptance:     task.Acceptance,
		SourceSections: task.SourceSections,
		SourceLines:    task.SourceLines,
	}
	if !compact {
		bt.SpecExcerpt = ExtractSpecExcerptForTask(specPath, task.SourceLines, task.SourceSections)
	}

	return &Brief{
		Identity:  identityOneLine,
		Task:      bt,
		PriorWork: prior,
		Close:     CloseRecipeText(effectiveStrategy, qualityGate),
		Scope:     ScopeFenceText(),
		compact:   compact,
	}, nil
}

// compactPriorWork returns a copy of prior with each entry collapsed to its id
// and evidence summary (§12.1): no file lists, in the minimal one-line-per-entry
// form. is_first_unit and omitted_count are preserved (they are not file lists).
func compactPriorWork(prior *PriorWorkResult) *PriorWorkResult {
	entries := make([]PriorWorkEntry, len(prior.Entries))
	for i, e := range prior.Entries {
		entries[i] = PriorWorkEntry{ID: e.ID, EvidenceSummary: e.EvidenceSummary}
	}
	return &PriorWorkResult{
		Entries:      entries,
		IsFirstUnit:  prior.IsFirstUnit,
		OmittedCount: prior.OmittedCount,
	}
}

// Text assembles the five-section implementation brief text (§4.1) from the
// machine parts, in the fixed order Identity, Scope, Prior work, Your unit, How
// to close — ready to paste into a sub-agent prompt. Under compact the identity
// and scope sections shorten to their core line and prior-work entries collapse
// (§12.1); the acceptance text, close recipe, and scope-fence prohibitions are
// always present.
func (b *Brief) Text() string {
	var s strings.Builder

	// §4.1 Identity (§4.2): what this unit is, and the one-unit-then-stop rule.
	s.WriteString("## Identity\n\n")
	if b.Task.Title != "" {
		fmt.Fprintf(&s, "You are executing one unit of work: %s (%s).\n", b.Task.Title, b.Task.ID)
	} else {
		fmt.Fprintf(&s, "You are executing one unit of work (%s).\n", b.Task.ID)
	}
	if b.compact {
		s.WriteString(b.Identity)
	} else {
		fmt.Fprintf(&s, "%s\n", b.Identity)
	}

	// §4.1 Scope: the acceptance criteria are the boundary, plus the fence (§7).
	s.WriteString("\n\n## Scope\n\n")
	if !b.compact {
		s.WriteString("The acceptance criteria in \"Your unit\" are the boundary of this unit's work: implement only them.\n\n")
	}
	s.WriteString(b.Scope)

	// §4.1 Prior work (§5).
	s.WriteString("\n\n## Prior work\n\n")
	s.WriteString(renderPriorWorkText(b.PriorWork, b.compact))

	// §4.1 Your unit: id, acceptance verbatim (§4.3), anchors, spec_excerpt.
	s.WriteString("\n\n## Your unit\n\n")
	s.WriteString(renderBriefTaskText(&b.Task))

	// §4.1 How to close (§8).
	s.WriteString("\n\n## How to close\n\n")
	s.WriteString(b.Close)
	s.WriteString("\n")

	return s.String()
}

// renderPriorWorkText renders the prior-work section body. The first unit states
// it has no prior work; otherwise each entry is one line per field, collapsed to
// id and evidence summary under compact (§12.1).
func renderPriorWorkText(prior *PriorWorkResult, compact bool) string {
	if prior == nil || prior.IsFirstUnit {
		return "This is the first unit of the project; there is no prior work to review.\n"
	}
	var s strings.Builder
	for _, e := range prior.Entries {
		if compact {
			fmt.Fprintf(&s, "- %s: %s\n", e.ID, e.EvidenceSummary)
			continue
		}
		fmt.Fprintf(&s, "- %s — %s\n", e.ID, e.Title)
		if e.EvidenceSummary != "" {
			fmt.Fprintf(&s, "  %s\n", e.EvidenceSummary)
		}
		if len(e.CommitSHAs) > 0 {
			fmt.Fprintf(&s, "  commits: %s\n", strings.Join(e.CommitSHAs, ", "))
		}
		if len(e.CommitFiles) > 0 {
			fmt.Fprintf(&s, "  files: %s\n", strings.Join(e.CommitFiles, ", "))
			if e.CommitFilesMore > 0 {
				fmt.Fprintf(&s, "  (+%d more)\n", e.CommitFilesMore)
			}
		}
	}
	if prior.OmittedCount > 0 {
		fmt.Fprintf(&s, "\n(%d more prior-work entries omitted)\n", prior.OmittedCount)
	}
	return s.String()
}

// renderBriefTaskText renders the "Your unit" section body: the id, an optional
// title, the acceptance text verbatim, the anchors, and the spec_excerpt when
// present.
func renderBriefTaskText(t *BriefTask) string {
	var s strings.Builder
	fmt.Fprintf(&s, "id: %s\n", t.ID)
	if t.Title != "" {
		fmt.Fprintf(&s, "title: %s\n", t.Title)
	}
	fmt.Fprintf(&s, "\nacceptance (verbatim):\n%s\n", t.Acceptance)
	if len(t.SourceSections) > 0 || t.SourceLines != "" {
		s.WriteString("\nanchors:\n")
		for _, a := range t.SourceSections {
			fmt.Fprintf(&s, "- %s\n", a)
		}
		if t.SourceLines != "" {
			fmt.Fprintf(&s, "- source_lines: %s\n", t.SourceLines)
		}
	}
	if t.SpecExcerpt != "" {
		fmt.Fprintf(&s, "\nspec_excerpt:\n%s\n", t.SpecExcerpt)
	}
	return s.String()
}
