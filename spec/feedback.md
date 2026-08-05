# tp — Unversioned Feedback

Findings and improvement ideas that are not yet assigned to a release. When a
new version spec is opened, fold the relevant entries into it and delete them
from this file. Nothing here is scheduled; nothing here is normative.

## 1. lint: `orphan-list-item` fires on wrapped list items

**Observed** 2026-08-05, running `tp lint` on an external spec
(`~/Developer/deligoez/projects/cr/spec/0.1.0.md`, 766 lines, 202 numbered
items). Twelve `orphan-list-item` findings of the form
`numbered list starts at N (expected 1)`, every one of them a false positive.

**Cause, confirmed in `internal/engine/vague.go`.** The scanner groups
consecutive lines matching `^(\d+)\.\s` and calls `flush()` on any line that
does not match. A numbered item whose text wraps onto an indented continuation
line therefore closes its own group. Given

```
1. v0.1 MUST ship four default roles, one per axis: `intent-coverage`,
   `correctness`, `convention`, `test-adequacy`.
2. `cr init --eject-roles` MUST write the defaults as editable files that are
   byte-identical to the built-ins.
3. A malformed role or profile file MUST abort the command with exit code 3.
4. Role resolution order MUST be per-repository, then global, then built-in.
```

items 1 and 2 each form a single-element group that flushes silently (the
`len(numbers) < 2` guard suppresses the report), then items 3 and 4 form a
two-element group starting at 3, which reports. The existing test case
`non-list between` encodes exactly this flush behaviour, so the rule is doing
what it was written to do; the specification of "a non-list line" is what is
wrong.

**Impact.** Severity is `info`, so no gate breaks and no exit code changes. The
cost is noise: a well-formed spec that wraps its list items at 80 columns emits
one finding per numbered block, which trains the reader to ignore the rule.
This affects any spec written to a column limit, which is all of tp's own specs
that happen to wrap.

**Proposed fix.** Treat an indented continuation line as belonging to the open
list group rather than terminating it: while a group is open, a line that is
blank-prefixed by at least the indent of the current item's text and does not
itself match the item pattern extends the current item. Keep flushing on
unindented prose, headings, tables, and code fences, so the `non-list between`
case still behaves.

**Care needed.** The change must not swallow a genuine orphan: a list that truly
starts at 3 after an indented block quote should still report. Add regression
cases for a wrapped item, an item wrapped twice, a wrapped item followed by an
unindented paragraph, and a nested sub-list.
