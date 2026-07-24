package engine

import "strings"

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
