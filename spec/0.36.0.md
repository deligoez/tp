# tp v0.36.0 — The emitted round

## 1. Overview

tp emits one prompt per role per round. That prompt carries the checklist, the role rules, the
affected files and the output schema — everything about *what to judge*. It carries nothing about
*how the unit must behave while judging*, and it cannot be written anywhere except stdout.

The consequence is that every round costs the orchestrator the same three manual additions, and a
round that does not get them loses work or produces false findings. Two independent cycles measured
the same three, on different projects, in different languages:

- tp's own v0.35.0 audit — nine rounds, four roles, this repository.
- A field cycle on a PHP package, nine rounds, five roles, driven by a different operator.

This release makes the emitted prompt self-sufficient and gives it somewhere to go. It changes
nothing about what a round asks or how convergence is counted.

It is deliberately the smallest thing in the queue and deliberately first, because its cost is paid
by every cycle that follows it — including the cycles that implement the rest.

## 2. The prompt does not carry its own isolation constraint

**The field instance is a false finding, not an inconvenience.** In round 2 of the PHP cycle an
auditor mutated a file in the repository to test whether a test was tautological — a legitimate
experiment, and the one this project's own evidence rule asks for. A *different* auditor, running
concurrently, read the mutated file and reported the static analyser's failure as a real defect.
The finding was caused entirely by the absence of isolation guidance in the prompt both agents
received. The operator then added "DO NOT modify the working tree" to all five role prompts, by
hand, every round.

tp's own cycle paid the same cost without the failure: the orchestrator injected "Do NOT modify any
repo source file" into all four role prompts for nine consecutive rounds, and one auditor still had
to be told separately to restore a mutation byte-identically.

**Mechanism.** The emitted role prompt carries the constraint and the recipe that makes the
constraint livable: a unit may read anything, must not modify the working tree, and when an
experiment needs a mutation it copies the subject out of the repository first and mutates the copy.
The text is emitted, not documented — a rule in `skills/tp/SKILL.md` reaches only readers of that
file, and the unit that needs it is a fresh process that was handed a prompt.

**What this section does not decide.** Whether an auditor should have an execution surface *at all*
— a per-unit workspace, an amended agent definition, a sanctioned way to perturb — is
`spec/0.42.0.md` §9's open question, and this release does not answer it. The two are different
problems that meet here: under `tp run` the §6.2 fence already denies the write, so the constraint
is enforced and this text is a courtesy; on the interactive path nothing denies it, and this text is
the only thing standing between a concurrent pair of auditors and a false finding. This release
covers the second case only.

## 3. A unit that buffers its findings loses all of them

Both cycles lost whole rounds to the same shape. An agent accumulates its findings in context and
writes the NDJSON once, at the end; the process dies before that write; nothing survives.

The field cycle lost **six** subagents this way over one session. The single run that had been
instructed to append each row as it was decided preserved **72 rows** despite dying near the end.
tp's own cycle lost four, and the one told to write early kept 9 of its 10 rows — so the relaunch
completed the round instead of redoing it, and the recovered rows were the expensive ones.

The asymmetry is the argument: buffering saves nothing and risks everything, because a findings file
is append-only NDJSON whose partial state is already valid input to `--merge`.

**Mechanism.** The emitted prompt instructs the unit to write each row as it is decided rather than
once at the end, and says why in one clause, so an agent that is optimising its own tool calls does
not batch the writes back together.

## 4. The prompts have nowhere to go

`tp audit <spec> --out-dir <dir>` is an unknown flag. `--output` exists on both `review` and `audit`
but is scoped to `--merge`, so it names a merged output file and cannot receive emitted prompts.

Both cycles therefore piped the JSON into a script and split it by role, every round. That script is
not incidental work: it re-derives `prompts[].role` and `prompts[].output_path`, both of which tp
already computed, and it is the step most likely to be skipped or mis-written under time pressure.

**Mechanism.** `--out-dir <dir>` writes one file per role, named from the role, and reports the
paths on the machine-readable surface so a driver can spawn from them without parsing prose. It is
additive: without the flag the emitted JSON is unchanged.

## 5. Non-Goals

1. **Sharding a role's checklist.** Splitting one role's work across several units is
   `spec/0.41.0.md`. This release makes the prompt self-sufficient; that one makes the round
   divisible. A prompt that can be written to a file is a precondition for sharding, which is why
   this ships first, but nothing here splits anything.
2. **Deciding the auditor's execution surface.** `spec/0.42.0.md` §9. See §2.
3. **Changing what a round asks.** No checklist item changes, no role rule changes, no convergence
   arithmetic changes. Convergence is `spec/0.37.0.md`.
4. **Enforcing the isolation constraint outside `tp run`.** The interactive path gets the
   instruction, not a fence. A fence there would need a hook the interactive path does not install.

## 6. Tests

1. The emitted role prompt for both `tp review` and `tp audit` contains the working-tree constraint
   and the copy-out recipe, and a prompt for a non-role unit does not.
2. The emitted role prompt instructs incremental append, and states the reason.
3. `tp review <spec> --out-dir <dir>` writes one file per role; the file count equals the panel size
   and each name derives from its role.
4. `tp audit <spec> --out-dir <dir>` does the same.
5. The written paths appear on the machine-readable surface, so a driver spawns from the payload
   rather than from stderr.
6. `--out-dir` on a path that cannot be created exits non-zero with a hint naming the directory,
   and writes no partial set.
7. Without `--out-dir`, the emitted JSON is byte-identical to the current output apart from the two
   new prompt clauses — the flag adds a destination, it does not reshape the payload.
8. `--out-dir` and `--merge`'s `--output` do not collide: passing both is either coherent or refused
   with a hint, never silently one-of.
