# tp — Re-emitting an unrecorded epic review round

A backlog decision candidate, named by slug; its priority and release number are assigned later.

Class: **decision**. A behaviour change on epic review emission.

## 1. The decision

**Context.** Re-emitting an epic review round that was emitted and not recorded rebuilds it from
the current graph and role corpus. Spec review and audit do the opposite: an emission whose input
changed is refused, and discarding it is the operator's `--force` (`discard-emission`). Agents may
hold the old prompts while the round is out, so a rebuild after a graph or corpus edit can record
their rows against a round with different roles or units. Nothing reports it. Concretely, rows an
agent already wrote against the old emission may cite units the rebuilt snapshot no longer has.

The same class predates it: deleting an epic from the graph, or moving it out of a slice, takes its
open review rows with it, so state can cite a unit that no longer exists.

**Decision.** The emission snapshot also stores the emitted roles, each with a hash of its role
file, beside the units, blocks and purposes. Re-emitting an unrecorded round compares the stored
emission with the rebuilt one over the whole panel, whatever `--role` the call names: identical, the
re-emit stays idempotent; different, it is refused with exit 3, as spec review refuses, naming the
round and what changed; the ways forward are below. `--force` beside `--resolve` keeps today's
meaning. A batch emission skips such a slice with the reason `emission-changed`, naming the discard
command for that slice. A snapshot from before this rule, holding no roles, is compared on what it
holds. Progress is not a change: closing work between an emission and its re-emit must not refuse.

A row with no resolution whose unit is no longer in the slice refuses the emission (exit 4), naming
the rows; the way out is `--resolve`. A row resolved `fixed` on a vanished unit passes, since the
removal was the fix. Part B's refusal for a unit no role reads follows the same distinction.

Dispositions are frozen while a round is out: `--resolve` or `--resolve-all` on any row an emitted,
unrecorded round was built from is refused (exit 3). The guard's refusal and this one name the same
two ways forward: record the round as emitted, after which a disposition takes effect from the next
round, or discard it with `--discard`, the operator's decision (`discard-emission`, exit 2 under
`TP_UNATTENDED`), which removes the emission and the files it named and emits nothing. A
disposition therefore never changes an emission behind the prompts already handed out, and the
guard needs no disposition comparison. Emitting many slices at once freezes each slice's
dispositions until it is recorded or discarded, one slice at a time. An empty hand-made `--record`
file is refused for a round whose emission named role files, since it would record roles that never
ran as having found nothing; `--record-inbox` proves they ran.

Spec review spells the same decision `--force`. Epic review cannot: a discard there must be
possible without a new emission, because dispositions are frozen while one is out, so `--force`
keeps its one meaning on epic review, re-resolving with `--resolve`.

## 2. Alternatives

- **Leave it and document it.** A re-emit that works today keeps working; the risk is agents
  answering a round that no longer exists.
- **Rebuild silently, refuse only a role change.** Rejected: a graph edit changes the units the
  agents' prompts named just as much.
- **Refuse every row on a vanished unit.** Rejected: moving or deleting an epic is the normal fix
  for a placement or surplus finding, and the way out would be to mark a fixed row `wontfix`.

## 3. Non-goals

- Changing how a recorded round carries into the next.
