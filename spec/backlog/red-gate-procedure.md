# tp — The red-gate procedure (a doc note, not a release)

A backlog entry, named by slug; it is not a release and takes no version. Its measurements are in
`red-gate-procedure-measurements.md` beside it; this file stands without them.

Class: **doc** — a `skills/tp/SKILL.md` section, committed as a plain doc change. It runs on today's
`&&` gate and waits for no release; it no longer depends on `gate-sequence`.

## What it adds

The close recipe `tp brief` emits (`internal/engine/brief.go`) already carries the rule — *a red gate
is never closed over: `--skip-gate` is a human decision, never the unit's* — and says nothing about
what a unit does next. `skills/tp/SKILL.md` has no section for it. This note is that section, and it
enforces nothing; the mechanical proxies considered and rejected are in the sidecar under *Why this is
not a check*.

## The procedure

1. **Run the failing command alone.** A failing close names the gate and its `exit_code`, and its
   `output_tail` ends with the failing command's own output. A failure that does not reproduce alone
   is a finding about the gate, not about the change.
2. **Re-run that command, then the whole gate.** Both, in that order. A red lint once stood behind a
   green test suite across a run of task closes, each closed on a gate its agent read as green; this
   step is what catches that shape (sidecar, *Step 4's incident, derived*, which carries the counting
   rule).
3. **Stop on a condition, not a count** — when the next step would be a user-only decision, or an
   attempt produced no new information about the failure. An attempt count is a proxy a unit learns
   to game. A minimal reproduction also ends it: every remaining input is load-bearing — removing any
   one turns the command green — and that case becomes the regression test. Then exit by name:

| context | the exit |
|---|---|
| under `tp run` | `tp escalate --decision skip-gate --evidence <text>`; the run stops with `stop_reason: escalation` and the operator answers |
| outside a run | a hand-back: stop, leave the task `wip`, report the wall in the unit's own report, which is the only carrier — no `last_failure` is written on this path |

The exit differs by context because naming a command that exits 2 on the path the unit is on is
worse than naming none. The probe behind the table and the hand-back's missing writer are in the
sidecar under *The escalate probe* and *The hand-back, and what it does not carry*.

**Cut at 2026-09-11.** The draft's steps 2 and 3 — observe the failure before editing, and name three
to five ranked falsifiable causes — are gone: the first is `CLAUDE.md`'s *prove a fix by running it*
rule, and the second is what the ground prompt already asks of a grader. The draft's step 1 ran *one
entry* of a named-entry gate, which is why it waited for `gate-sequence`; that array is deferred, and
the step now runs on the `&&` gate. The cut text is in the sidecar under *Cut to a doc note,
2026-09-11*.
