# tp — Re-emitting an unrecorded epic review round

A backlog decision candidate, named by slug; its priority and release number are assigned later.

Class: **decision**. A behaviour change on epic review emission.

## 1. The decision

**Context.** Re-emitting an epic review round that was emitted and not recorded rebuilds it from
the current graph and role corpus. Spec review and audit do the opposite: an emission whose input
changed is refused, and discarding it is the operator's `--force` (`discard-emission`). Agents may
hold the old prompts while the round is out, so a rebuild after a graph or corpus edit can record
their rows against a round with different roles or units. Nothing reports it.

**Decision (candidate).** Guard the epic emission the way spec review is guarded: a re-emit whose
units, blocks or corpus differ from the stored emission is refused, and `--force` discards it as
the operator's decision, exiting 2 under `TP_UNATTENDED` with `discard-emission`. Spec review's
guard and escalation are the parts to reuse.

## 2. Alternatives

- **Leave it and document it.** A re-emit that works today keeps working; the risk is agents
  answering a round that no longer exists. This is the state at the time of writing.

## 3. Non-goals

- Changing how a recorded round carries into the next.
