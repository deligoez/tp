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

## 2. review: repair verbosity, not spec quality, set the convergence rate

**Observed** 2026-08-05, nine review rounds on an external spec
(`~/Developer/deligoez/projects/cr/spec/0.1.0.md`, four roles per round, 502
findings total). Blocking counts (critical + high) by round:

```
round     1   2   3   4   5   6   7   8   9
blocking 46  35  37  34  35  14  17  11   7
critical  4   2   1   3   5   0   0   1   1
```

Rounds 1–5 sat flat at 34–46 and showed no trend. The regime changed at round 6
and the spec was not what changed: the author changed how repairs were written.
Through round 5 each fix carried an explanatory clause justifying itself
("…which is exactly the forgery the deleted rule branch allowed"). From round 6
the rule was minimum change, no rationale prose. Average repair went from ~8
lines to ~2, and blocking fell by 60% in one round.

**Cause.** Every explanatory sentence in a normative document is itself
normative surface: a new claim that can contradict another section, and a new
cross-reference that can go stale. Closing N findings by writing M lines of new
prose generates roughly `k·M` new findings. While `M` was large the loop sat at
its fixed point. Round-over-round the majority of each round's blocking findings
were located in text written during the immediately preceding round, not in the
original spec.

**Impact on tp.** `tp review` measures convergence purely by counting findings
per round. It has no notion that the *repair* is the dominant variable, so a
loop that will never converge looks identical to one that is two rounds away.
The workflow gives the agent no guidance on repair discipline, and the agent's
natural instinct — explain the fix so the reader understands it — is the wrong
one for a normative document.

**Proposed fix.** Two parts, neither of which needs new machinery.
(1) State the repair discipline in the review-loop instruction `tp review`
already emits: a fix is the minimum normative change; rationale belongs in the
round record, not in the spec. (2) Track and report *where* each round's
findings land relative to the previous round's edits. `tp` already stores
per-round snapshots; diffing them against the next round's finding locations
would show the author directly that they are chasing their own text, which is
the single most useful signal this loop can give.

**Also observed: severity inflation.** Per-role totals fell steadily
(tester 31→16, trust-economist 27→8) while the blocking *share* held, because
roles promoted narrower items to `high` as the substantive ones ran out. Round
1's criticals were design holes ("the argued forcing is disableable by an
environment variable"); round 5's were missing table rows. Adding an explicit
calibration note to the emitted prompts — prior rounds' severity distribution,
and that an empty result is a valid outcome — corrected this immediately and
contributed to the round-6 drop. Worth emitting by default.

## 3. review --merge: dedup key misses cross-role duplicates

**Observed** in the same nine rounds. `tp review --merge` deduplicates on
`(location, class)`. Class slugs are composed independently by each role, so two
roles reporting the same defect almost never collide.

```
round              1   2   3   4   5   6   7   8   9   total
tp removed         2   1   2   0   0   1   0   0   1     7
same section,
multiple roles    16  11  10  11   6   2   3   2   2    63
```

The right-hand row counts blocking findings sharing a `location` with at least
one other role's finding — a lower bound on conceptual duplication, since it
misses same-defect-different-section cases. Round 4 is the clearest instance:
`§9.6` drew seven findings from four roles, all describing one broken
acceptance flow, and merge removed none of them.

**Impact.** The merged count overstates the work by roughly a third in early
rounds, which distorts the convergence signal the loop is judged on and makes
the author read a fixed set of defects as a larger one. The `overlap_report`
already computes per-role `shared` counts, so the data to notice this is present
and unused.

**Proposed fix.** Do not try to unify class vocabularies — roles are supposed to
name things through their own lens. Instead report a section-level cluster
alongside the existing dedup: group merged findings by `location`, and where one
location carries findings from more than one role, surface them together as a
likely single defect with N reporters. Leave them as separate records; the value
is in the presentation, and in an honest count. A `high` reported independently
by three lenses is also a useful confidence signal, which is currently lost.
