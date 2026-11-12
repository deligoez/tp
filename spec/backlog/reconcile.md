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
   evidence, at}` to the state's epochs; the evidence is required. Rounds recorded before the newest
   epoch stop feeding any list of earlier findings an emission carries — the review carry list, the
   regression pass, the audit prior-round section and the audit acceptance carry — in both loop
   phases; nothing recorded changes, no disposition is written, and no convergence rule changes.
   `--status` of both phases reports the epochs. It is a state-writing mode placed on the
   task-file side's rules like every other, and refused with a round in flight (record or discard it
   first).
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
