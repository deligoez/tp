# Found tasks (Full-track candidate)

Seed material for a Full spec, written down so the decisions survive until rolling implement slots
(`rolling-implement.md`) land. It is not the spec: the spec is interviewed, grounded and reviewed
from this.

## Direction (operator, 2026-11-18)

tp exists to run for long stretches without human input. When a unit finds an obvious defect, it
opens the task itself, places it correctly, and the run does the work. Human input is collected at
the very end, and only where it is truly needed.

## Operator decisions

1. **Decisions go to an agent first.** A finding that needs a choice goes to a decision unit first,
   and to the human queue only if that unit cannot settle it.
2. **On by default.** The feature is capped per run at **5** found tasks; overflow goes to the
   queue, and 0 turns the feature off. Raising the cap is the operator's.
3. **The queue blocks nothing.** A non-empty human queue holds neither convergence nor the release.
   Its count is reported in `converged` and in `tp resume`.
4. **Implement and audit units both find.**
5. **One Full spec,** not two Decision notes.
6. **A spec-silent choice goes to the human (2026-11-24).** The decision unit answers `task` only
   when pre-existing text excludes the rejected alternative; any spec-silent choice, or a compound
   finding with one spec-silent part, is `human`. Measured on the field: 62% of the non-Fix-shaped
   findings were spec-silent, and both findings that turned out wrong were non-Fix-shaped.

## Design inputs (from the tp-smart consultation)

- **Classification is a shape check, not a judgement.** A found task must be Fix-shaped:
  - a location (`file:line`);
  - a reproduction;
  - expected and observed behaviour.

  A finding with any of these missing, one that asks for a choice between alternatives, or one
  that says the spec is wrong, goes to the decision unit. The fixing unit is the second filter: it
  sees the reproduction red first, and if it cannot, it hands the task back to the queue instead
  of closing it.
- **Anchor.** A found task carries `origin` (the finding task's id and the location) in place of
  `source_sections`, never both. Synthetic anchors are refused. Consequences:
  - the brief carries no spec excerpt, so the request text must stand alone (one task, one defect,
    with a length cap);
  - spec-coverage leaves found work out of its denominator;
  - the task audit reaches it only through its files.

  `touches` comes from the location; the estimate is a fixed default.
- **Placement**, one of two, both on the finder's own edge:
  - (a) the new task depends on the finder; this is the default;
  - (b) "blocks me": the finder depends on the new task, exits without asking to close, goes back
    to open, and spends no attempt.

  No third task's edges change, so cycles are impossible by construction. Field evidence for (b)
  is uncounted.
- **Writer.** A unit writes a request; the driver applies it under the serial rule, owning the
  cap, the dedup and the placement. Implement units are fenced from direct `tp add` and from
  `tp set depends_on`; first count which unit kinds legitimately call `tp add` under a run. Dedup
  is by exact location only. Found-from-found counts against the same cap.
- **The audit's part.**
  - A non-PASS row on an asked item keeps today's audit-fix path; it never becomes a found task.
  - Only what rule 13 today says to "record with justification and ship" (out-of-surface, or not a
    row) becomes a found task, and only if it is Fix-shaped. The rule 13 sentence changes.
  - An audit unit's request is applied only after its round is recorded.
  - The round after a found task's close is an ordinary counted round, with no third counter. The
    clean-round reset follows today's rule for an audit-fix commit; check it in the code. Check
    also whether the oracle already reads an open task as the implement phase.
- **Decision unit.**
  - It is a fresh agent, never the finder. It is a new unit kind and a fourth plugin agent.
  - It answers either `task` (the Fix fields filled in, plus a citation of what settles the
    choice: a spec section, or a documented rule such as CLAUDE.md, REFERENCE or a behaviour an
    existing test pins, which tp verifies exists) or `human` (its reasons and the alternatives it
    weighed). There is no `drop`.
  - Rule-9 items stay fenced.
  - It runs when undecided findings exist and no implement unit is ready or running, and after
    each recorded audit round. It takes a small, written number of findings per unit.
  - Its tasks count against the same cap.
  - It interprets; it does not legislate.
- **Queue.** A committed file, not run state.
  - Each entry is shaped: the location, the question, the alternatives, the tasks it holds, and
    `answered_by`.
  - `tp resume` shows the count, the oldest entry and "N decisions hold M tasks".
  - A batch command settles every entry: make a task, reject with a reason, or move it to
    `BUGS.md` or the backlog.
  - The human also sees the decision unit's `task` decisions, so they stay auditable.
  - This closes the deferred `tp note` item in `spec/undecided.md`.
- **Readers to sweep.**
  - Every reader of `source_sections`: lint, coverage, brief, `audit --affected-from-tasks`, the
    dashboard, import and export.
  - Whether the growth check counts found tasks as spec growth.

## Field evidence (the requesting project, counted)

- **Tasks added.** 46 imported, 88 now; all 42 additions came through `tp add --bulk`.
- **Sources of the last 23.** 18 came from a measure → diagnose → decide loop whose decisions an
  agent peer made, and 5 from unit findings in about 20 units. Of those 5, 2 are Fix-shaped and 3
  are choices.
- **Not converging.** About one task was added per task closed.

That gives a found-task yield of about 0.1 task per unit, so the human queue is the heavier half.
The evidence is thin, from one field and uncounted before 16:45; the measurements file says so.

## Measured 2026-11-24 (expected split written first)

Strict reading: `task` only where text written before the choice excludes the alternative.

- **Seed window** (re-counted): 22 additions after the cut, not 23 — 17 from the loop, 5 unit
  findings. Findings: 1 Fix-shaped (not 2; a second is Fix only under a loose reading), 1 `task`,
  3 `human`. Decisions: 7 `human`.
- **After the note** (88 → 199 tasks): 111 additions — 74 spec or process work (the non-goal), 21
  unit findings, 11 review or audit findings, 3 unknown. The 32 findings: 10 Fix-shaped, 9 `task`,
  13 `human`.
- **Combined, 37 findings:** 11 Fix-shaped, 10 `task`, 16 `human`. Spec-silent: 16 of the 26
  non-Fix-shaped, 23 of 33 with the decisions. Settled by operator decision 6.
- **Wrong findings:** 2, both non-Fix-shaped (one never reproduced in 47 tries, one a suspicion
  that did not hold); none of the 11 Fix-shaped.
- **Convergence:** 0.94 tasks closed per task added after the note; 12 of the 13 open additions are
  deferred, so the backlog that builds up is a deferral queue.
- **Anchors:** 23 `source_sections` strings name no heading in the spec, and 53 of the 111
  additions anchor to one catch-all section — the field case for `origin` and for refusing
  synthetic anchors.

## Must be measured before building

- A seeded clone run with obvious and ambiguous defects. Count:
  - misrouted findings;
  - wrong tasks, whose reproduction never goes red;
  - cap hits;
  - tokens added per unit;
  - work left undone because the queue holds it.

## Non-goals

- The measure → diagnose → decide loop that drove most of the field's growth. That is spec work
  re-decomposed mid-run, a separate decision.
- Moving rule-9 escalations into the queue. That conflicts with rolling's "escalation drains and
  stops", and is a separate operator decision.
