package cli

import (
	"fmt"
	"os"
	"strings"
)

// perRoleReadingBudget is the per-role byte threshold (§10.7, default 12 KB)
// under which a role's inlined source-file contents stay complete and
// authoritative. The first emitted role whose files fit under it inlines them;
// roles that would exceed it, and every role emitted after the first inliner,
// receive named paths only.
const perRoleReadingBudget = 12 * 1024

// promptFraming carries the tp-owned framing wrapped around every emitted
// review/audit role prompt (§10.4–§10.7). A role only customizes its own prompt
// body; this framing is tp's contract around it.
type promptFraming struct {
	phase            string   // "review" | "audit"
	round            int      // current round number (1-based)
	requiredClean    int      // consecutive clean rounds required to converge
	consecutiveClean int      // consecutive clean rounds recorded so far
	maxRounds        int      // round cap; 0 means no cap
	outputPath       string   // §10.4: file this round's findings go to
	filesComplete    bool     // §10.7: source-file contents are inlined & complete
	filesPartial     bool     // §10.7: contents inlined, but filePaths could not be read
	filePaths        []string // §10.7: named paths whose contents are NOT inlined
	hasFiles         bool     // §10.7: any source files are in scope for this role
}

// renderFraming produces the framing block appended to a role prompt. It states
// the output file (§10.4), the reset discipline (§10.5), the loop budget
// (§10.6), and the file-reading situation (§10.7) — explicitly, never implied.
func renderFraming(f *promptFraming) string {
	var b strings.Builder
	b.WriteString("\n\n## Unit framing\n\n")

	// §10.4 output path + §10.5 reset discipline.
	fmt.Fprintf(&b, "Write this round's findings to: %s\n", f.outputPath)
	b.WriteString("Produce findings for this round only, write them to that file, then stop.\n\n")

	// §10.6 loop budget: round number, required clean count, consecutive clean
	// so far, and the remaining budget when a cap is set.
	fmt.Fprintf(&b, "Loop budget: %s round %d; %d consecutive clean round(s) required, %d so far",
		f.phase, f.round, f.requiredClean, f.consecutiveClean)
	if f.maxRounds > 0 {
		remaining := max(f.maxRounds-f.round+1, 1)
		fmt.Fprintf(&b, "; cap %d, %d round(s) remain (this one included)", f.maxRounds, remaining)
	}
	b.WriteString(".\n\n")

	// §10.7 file-reading statement.
	b.WriteString("File reading: ")
	switch {
	case !f.hasFiles:
		b.WriteString("no source files are carried; the spec content above is complete and authoritative.\n")
	case f.filesComplete:
		b.WriteString("the source file contents carried in this prompt are complete and authoritative; you need not read any file yourself.\n")
	case f.filesPartial:
		b.WriteString("the source file contents carried in this prompt are INCOMPLETE — tp inlined every file it could read, but these it could not; read these files yourself before judging them:\n")
		for _, p := range f.filePaths {
			fmt.Fprintf(&b, "  - %s\n", p)
		}
	default:
		b.WriteString("this prompt names source files but does NOT inline their contents — read these files yourself before judging them:\n")
		for _, p := range f.filePaths {
			fmt.Fprintf(&b, "  - %s\n", p)
		}
	}
	return b.String()
}

// fileSetRead reads the listed paths in full and returns a rendered content
// section (path, line count, whole body) together with the paths it could NOT
// read. It does not truncate: §10.7 inlines a role's file contents only when
// they fit whole under the per-role reading budget, so a caller sizes the set
// with fileSetBytes first and calls this only once the set is known to fit.
//
// A non-empty unreadable slice means the section is incomplete, and a caller
// must then never set promptFraming.filesComplete: a role told the carried
// contents are complete and authoritative would report on a file whose body it
// never received. The bodies that WERE read still travel with the prompt under
// the "(incomplete)" header — a caller names only the failed paths for the role
// to read itself, so one locked file costs that file and not the whole set.
func fileSetRead(paths []string) (section string, unreadable []string) {
	seen := make(map[string]bool, len(paths))
	unreadable = make([]string, 0)
	var body strings.Builder
	for _, p := range paths {
		if p == "" || seen[p] {
			continue
		}
		seen[p] = true
		data, err := os.ReadFile(p)
		if err != nil {
			unreadable = append(unreadable, p)
			continue
		}
		fmt.Fprintf(&body, "### %s (%d lines)\n", p, strings.Count(string(data), "\n")+1)
		body.Write(data)
		body.WriteString("\n\n")
	}
	header := "## Affected Files (complete)\n\n"
	if len(unreadable) > 0 {
		header = "## Affected Files (incomplete)\n\n"
	}
	return header + body.String(), unreadable
}

// fileSetBytes reports the total on-disk byte size of the listed paths. It is
// the cheap stat-only probe a caller uses to size the per-role inliner (§10.7)
// before reading any file body with fileSetRead. A path it cannot stat
// contributes nothing and is not reported separately: that path cannot be read
// either, so fileSetRead returns it among its unreadable paths and the caller
// names it there for the role to read itself.
func fileSetBytes(paths []string) int {
	seen := make(map[string]bool, len(paths))
	total := 0
	for _, p := range paths {
		if p == "" || seen[p] {
			continue
		}
		seen[p] = true
		info, err := os.Stat(p)
		if err != nil {
			continue
		}
		total += int(info.Size())
	}
	return total
}
