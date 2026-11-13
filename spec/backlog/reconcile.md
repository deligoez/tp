# tp — A rewritten spec starts a new epoch

A backlog decision for 1.14.0, cut down on 2026-11-13 to what is still missing at v1.12.0: the parts
of the 2026-09 draft that shipped since (a round's `spec_hash` is its snapshot's hash, v1.2.2; the
`spec-stale` message names a command, v1.2.2) are gone from it. The draft and its field report are in
`reconcile-measurements.md`. The operator delegated the night's decisions to the agent, which took a
second opinion from the tp-smart session.

Class: **decision**.

## 1. Context

When a spec is rewritten, its recorded rounds describe a document that no longer exists. Two things
remain wrong at v1.12.0:

- The next review round's prompts still carry every earlier round's findings under "DO NOT
  re-report", every one of them about the old text; the regression pass and the audit's prior-round
  section read the old rounds the same way, and an audit acceptance carries forward from the round
  before at `--record`.
- The `spec-stale` blocker names `tp review <spec>`, but with a round in flight whose snapshot is the
  old text that emission is refused, so the blocker points at a command with no exit; and its data
  carries the spec only, not the command.

## 2. Decision

1. **`tp review <spec> --rewritten "<evidence>"` starts an epoch.** It appends `{after_round,
   evidence, at}` to the state's epochs; the evidence is required. Rounds that read an older text than
   the spec at the time of the epoch stop feeding any list of earlier findings an emission carries —
   the review carry list, the regression pass, the audit prior-round section and the audit acceptance
   carry — in both loop phases; nothing recorded changes, no disposition is written, and no
   convergence rule changes. `--status` of both phases reports the epochs. It is a state-writing mode
   placed on the task-file side's rules like every other, and refused with a round in flight (that
   round was built from the lists the epoch cuts; record or discard it first) and when it would cut
   nothing (every recorded round already read the current text, or was cut by an earlier epoch).

   A spec reverted to an earlier text cuts the rounds that read that text the first time too: the
   epoch's position is one number per phase, after the last round over a text other than the current
   one, and that conservative result is intended, not a bug.

   An epoch changes prompts only, and that is why it is an ordinary mode and not an operator
   decision. Convergence already reads only the trailing clean rounds, so an open row in an earlier
   round blocks nothing today and blocks nothing after; the round cap keeps counting every recorded
   round, the epoch's earlier ones included. Cutting a carry list errs in the blocking direction: a
   finding that still holds is raised again and has to be settled again — the carry list is what keeps
   an unresolved high from being re-raised today (`BUGS.md`), so cutting it at an epoch makes the old
   text's highs visible to the new text's roles. In particular an audit
   finding accepted without a change before the epoch is not carried, so the operator may be asked
   for that acceptance a second time. With the position defined by the text each round read, any edit
   to the spec makes `--rewritten` available; that is acceptable for the same reason, since
   everything an epoch cuts errs toward more review, and it is why the mode stays an ordinary one.
2. **The `spec-stale` blocker names its exit as data.** Its data carries the command that clears it;
   with a round in flight whose snapshot differs from the spec, the command is the record of that
   round (or its discard, as the operator's decision), not a new emission.

## 3. Alternatives

- **Void the rounds** (mark every earlier finding `wontfix`). Rejected: a `wontfix` asserts someone
  read the finding and chose not to fix it, and disposing the rows manufactures convergence.
- **An uncounted note beside a stale round.** Rejected: it clears nothing, so no blocker can name it.

## 4. Non-goals

- Changing how a round's hash is computed or when staleness clears (v1.2.2, v1.12.0).
- Deleting or rewriting recorded rounds.
- Letting an epoch clear `spec-stale`: only a round that read the current text does.
- Resetting the round cap.
- Epic review: its carry is per unit and is its own subject.
