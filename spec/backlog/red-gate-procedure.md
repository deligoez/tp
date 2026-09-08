# tp — The red-gate procedure (a skill section, not a release)

Class: doc — ships as a task inside the gate-sequence release

This is a `skills/tp/SKILL.md` section and, later, the same text emitted by the close recipe. It is
text and enforces nothing; the mechanical proxies considered and rejected are in the sidecar under
"Why this is not a check". At `HEAD`, `skills/tp/SKILL.md` does **not** yet carry the section — a
search of the skill for "red gate" returns nothing — while the rule it extends, *a red gate is never
closed over: `--skip-gate` is a human decision, never the unit's*, is already emitted by the brief's
close recipe (`internal/engine/brief.go`, surfaced by `tp brief` and `tp next --brief`). It ships as a
task inside `spec/backlog/gate-sequence.md`, because step 1 runs *one entry*, which is only a name once
`quality_gate` is an ordered array of named entries.

## The procedure

Every step is a rule this repository already paid for; the contribution is the sequence and the bound,
not the steps.

1. **Isolate, then reproduce.** Run the failing entry alone, not the gate. A failure that does not
   reproduce is a finding about the gate, not about the change.
2. **Observe the failure before editing it.** `CLAUDE.md`'s rule read in the other direction: a fix
   accepted without watching the failure first proves nothing, and an assertion never seen failing
   may be a tautology that passes identically either way.
3. **Name three to five falsifiable causes and rank them, before testing any.** Each states its
   prediction: *if X is the cause, then changing Y removes it.* A cause with no prediction is a
   guess. Then **show the ranked list and carry on** — the operator often re-ranks it instantly from
   knowledge the unit does not have, and a unit that stops to wait has turned a checkpoint into an
   escalation.
4. **Re-run the entry, then the whole gate.** Both, in that order. In v0.31.2 a red `golangci-lint`
   stood behind a green test suite across **ten task closes**, which is what this step exists to
   catch (the sidecar's "Step 4's incident, derived" carries the counting rule).
5. **Exit by name when the investigation ends.**

## When it ends, and how

**It ends on a condition, not a count.** Either the next step would be a user-only decision, or an
attempt produced no new information about the failure. An attempt count is a gameable proxy — a unit
that must finish in three attempts learns to declare victory on the third.

**And it ends on a minimal reproduction, which is a checkable bound rather than a feeling.** Once the
entry is red, cut inputs, config and steps one at a time, re-running after each cut. **Done when every
remaining element is load-bearing: removing any one of them turns the entry green.** The minimal case
is also the regression test, so the work is not thrown away. Where no seam can hold that test, the
absence is itself the finding: the unit records it rather than writing a test at a seam too shallow
to catch it.

**The exit differs by context**, because naming a command that exits 2 on the path the unit is on is
worse than naming none:

| context | the exit |
|---|---|
| under `tp run` | `tp escalate --decision skip-gate --evidence <text>`; the run stops with `stop_reason: escalation` and the operator answers |
| outside a run | a hand-back: stop, leave the task `wip`, report the wall in the unit's own report, which is the only carrier — no `last_failure` is written on this path |

The probe behind that table, the hand-back's missing writer, the draft's Non-Goals and the tests that
belong to the close-recipe emission are in the sidecar under "The escalate probe", "The hand-back,
and what it does not carry", "Non-Goals" and "Tests".
