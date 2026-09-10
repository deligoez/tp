# tp — The checklist covers what changed

A backlog spec, named by slug; its priority number and its release number are assigned later. Its
measurements are in `checklist-covers-what-changed-measurements.md` beside it; this file stands
without them. It was first written by reading the audit's file selection and re-emitting v0.37.0's
round 7; the 2026-09-11 pass added §4.2, §4.3 and §5 from a field report (WB-3155), verified in the
sidecar under *Field report WB-3155, verified 2026-09-11*.

Class: **tool** — it changes which files a checklist names, what tp says about every cut, and what a
recorded round stores about it, and no convergence signal. Budget it at the tool-class median in
`CLAUDE.md`'s *What a cycle costs* table.

## 1. Overview

`tp audit` hands each code-lens role a file list cut to a fixed cap in path order, and tells only the
roles, in their prompts' headings, that the cut happened. A list the operator named is cut the same
way, while the payload's `truncated` flag — which describes a different cut, the auto-detect cap —
reads false. Once the round is recorded nothing keeps the cut, so no later surface can say how much
was graded. The conformance role's spec excerpt is cut the same silent way. The probes are in the
sidecar under *The 77-byte probe and the draft corrections*, *Moved from the body at the 2026-09-11
pass*, and the field-report section.

The decisions:

1. **Rank by churn, not by filename** (§2).
2. **A list the operator named is not truncated** (§3).
3. **Say so when anything is cut** — at emission, in the recorded round, and in the spec excerpt (§4).
4. **Say so when the audited universe looks wrong** (§5).

**This is a stopgap and says so.** Ranking the capped list better does not audit the rest. Covering a
large surface in bounded prompts is `round-divides-by-section`'s subject; this release makes the
capped list the files that matter and stops every cut going unreported.

**It ships after `audit-records-what-was-graded`.** Under positional item ids, a list whose order moves
every round moves a suffix onto another file every round, so §2 alone would make that defect fire more
often (that spec's sidecar, *Why this ships before the churn ranking*).

An earlier draft proposed carrying last round's open findings into file selection; it was withdrawn,
and the grounds are in the sidecar under *Withdrawn: a file carrying last round's open finding*.

## 2. Rank the code roles' list by churn

The list is partitioned into a priority group — paths containing one of the danger substrings `lock`,
`validate`, `auth`, `secret` or `perm` — and everything else, taken in that order up to the cap.
Today neither group is ranked beyond path order.

**Decision.** Both groups are ordered by churn, descending — lines added plus lines deleted — with
ties broken by path so the order stays total. The churn figure is the one every emitted file entry
already shows as its `diff_summary`; no new input is collected.

**The priority group survives, and the two keys are orthogonal.** The substrings encode *this is
dangerous when it changes*; churn encodes *how much it changed*. A file that is both outranks a file
that is only one, which is what a stable partition with a churn sort inside each group gives.
Collapsing to one churn ranking would demote a small change to a locking path beneath an unrelated
churn spike, which is the trade the substrings exist to refuse.

**A path with no measured churn sorts last, not first.** Its absence means the file did not change,
or no comparison covers it, or the statistics probe failed for every path at once. None of the three
is evidence of churn, so ranking an unmeasured file ahead of a measured one would promote the files tp
knows least about. When the probe fails for every path, the order is path order — the shipped one —
and the sidecar records that run under *Moved from the body at the 2026-09-11 pass*.

**What it is worth, and where it stops,** is in the sidecar under *What churn ranking is worth,
measured*: the file v0.37.0's hardest audit rounds were about was cut in every round by path order and
would have reached the list in every round by churn. The ranking reorders the files that survive the
auto-detect cap, not the release diff above it, and it cannot surface a mechanical sweep whose files
differ only in how many identical lines each received.

## 3. A list the operator named is not truncated

**Decision.** When `--affected-files` or `--affected-from-tasks` replaced the universe, the code-role
cap does not apply.

An operator naming files is saying *these are the files*. Cutting that list in path order discards the
one input tp holds that is better than its own heuristic. The auto-detect cap's notice already advises
*name the rest with `--affected-files`*; this decision is what makes following that advice work past
the cap.

**No cap replaces it.** A prompt built from a very long named list may exceed what a role can read,
and this release does not know that bound — measuring it is `round-divides-by-section`'s job. What
ships here is the narrower claim: tp stops silently overriding an explicit instruction. If a named list
is too large, the operator is the party who can see it and split it.

## 4. Say so when anything is cut

### 4.1 At emission

The only truncation notice tp writes is the auto-detect cap's. It cannot stand in for the code-role
cut in either direction: above the auto-detect cap it fires and says nothing about the second cut that
follows; below it, and for any named list, nothing is printed at all.

**Decision.** Whenever a code role's list is shorter than the pool it was cut from, a notice names
both numbers and §3's remedy, **and the emission payload carries the pool size per role.** The notice
is stderr, which `--quiet` erases, so a driver must be able to read the pool from the payload. The
applied count is already there (`checklist_count`); the missing number is the pre-cap total.

### 4.2 In the recorded round

Once a round is recorded, no surface can say that a role saw part of its pool: the round state holds
the verdicts and none of the counts, so `next_action` reads the same whether a role saw its whole pool
or part of it.

**Decision.** The recorded round stores, per code role, the **files graded** — the distinct
`file_check` items the role's recorded rows answer — and the **files in the pool** — the pre-cap total
that round's emission reported under §4.1. `tp audit <spec> --record`, `--status` and `next_action`
say *graded N of M* for any role where the two differ. A round with no tp emission behind its number
stores no pool, and those surfaces say the pool is unknown rather than printing a guess.

Files graded counts files only because `audit-records-what-was-graded` makes one `file_check` id name
one file; under positional ids the same count would not mean files.

### 4.3 In the conformance role's spec excerpt

The spec-coverage prompt carries the spec text cut at a fixed byte budget. The cut is marked inside
the prompt only, and it can fall inside a multi-byte character, which the JSON payload then renders as
a replacement character. Nothing on stderr or in the payload says the conformance role read part of
the spec.

**Decision.** The payload states the excerpt's size and the spec's, so a cut is visible outside the
prompt, and the cut falls on a character boundary. The budget is unchanged: the fix for a large
conformance prompt is dividing the round by section, which is `round-divides-by-section` — the sidecar
records why the field case points there rather than at the excerpt.

## 5. Say so when the audited universe looks wrong

Two ways the universe can silently hold the wrong files, both reproduced in the sidecar. With a stale
local base ref, `--base` pulls in files that reached `HEAD` only through merging the base branch. With
`--affected-from-tasks`, a changed file no task touched is left out.

**Decisions.**

1. When `--base` is given and the audited range holds files that the first-parent history from `HEAD`
   to the base changes only through merge commits, a notice counts them and asks whether `--base` is
   stale. It is a question and not a filter: a branch that merged its own sub-branch has legitimate
   files of that shape, so selection is unchanged.
2. With `--affected-from-tasks`, a notice names the changed files — those the same audit would detect
   without the flag — that belong to no task. Selection is unchanged.

## 6. Non-Goals

1. **No new workflow field** — not the cap, not the ranking key, not a notice threshold. A workflow
   field is a fenced surface with four write sinks and its own resolution order; this release is a
   sort key, a conditional, recorded counts and notices.
2. **No change to spec-coverage's ranking key or its cap.** Its list is ranked by how many tasks map
   to a file, which is the right key for a conformance lens. Which commits a task maps through is
   `audit-records-what-was-graded`; §4.3 reports the excerpt cut and does not change the budget.
3. **No raise of the code-role cap, and no change to the auto-detect cap.** Raising the first trades a
   coverage hole for a prompt-size hole this release has not measured. The second is the larger hole,
   and it is fenced here so that it is not read as covered: what ships reorders the files that
   survived it, and the sidecar's *What churn ranking is worth, measured* carries its per-round effect.
4. **No change to the universe's drop rules.** Binaries, fixtures and deleted files stay dropped, and
   the universe stays sorted before selection so the partition is deterministic.
5. **No retroactive effect.** Rounds already recorded keep the lists they were emitted with and gain no
   counts.
6. **No change to the diff base.** Ranking inter-round diffs rather than the release diff would shrink
   the universe, but it is a separate decision with its own failure mode — a round's own repair commit
   becomes the whole audited surface — and the measured inter-round sets do not all fit under the cap.

## 7. Tests

Every row derives from a numbered decision, names the artifact it depends on, and names a mutant that
must fail it. Every fourth column names a change to production code; a widening of a test's own
fixture is a fixture hazard, not a mutant. Rows whose fixture runs at `HEAD` quote the value measured
there; rows marked *deferred* have a subject that does not exist at `HEAD`, so their counts belong to
the implementing task's acceptance.

| # | from | assertion | the mutant that must fail it |
|---|---|---|---|
| 1 | §2 | over a universe whose path-first files have the least churn, the code roles receive the highest-churn cap's worth of paths, ties by path. **The fixture's own properties are asserted first**: the test `require`s that the path and churn orders differ before asserting which one the code produced, and that no path matches a danger substring — one such path is promoted under both orderings, so the two lists would not be disjoint | keep path order — the shipped behaviour, which returns a disjoint set on this fixture |
| 2 | §2 *priority* | a path matching a danger substring outranks a higher-churn path matching none, and within the priority group churn still decides | drop the partition and sort the whole universe by churn, which demotes a `lock` file beneath an unrelated churn spike |
| 3 | §2 *absent* | a path with no measured churn sorts **last** | treat a missing figure as zero and sort ascending, or as the maximum — either puts unmeasured files at the head |
| 4 | §3 | with `--affected-files` naming 25 files, every code-lens role receives all 25. At `HEAD`: `10 of 25` | apply the cap regardless — the shipped behaviour, which discards 15 of the operator's own files |
| 5 | §4.1 | a cut emits a notice naming both numbers, **and** the payload carries the pool per role — asserted separately, with `--quiet` and without, and as the **presence of the pre-cap total**, never the absence of a digit | route the fact through the notice alone, which `--quiet` erases |
| 6 | §4.1 | the notice fires on a universe below the auto-detect cap, where the existing notice stays quiet — an auto-detected pool larger than the code-role cap and smaller than the auto-detect cap | keep the existing notice's gate, which is the defect |
| 7 | §6.2 | spec-coverage's list is byte-identical before and after, on a fixture where the code roles' list changes | apply the churn key to spec-coverage, reordering a lens whose key is task coverage |
| 8 | §4.2 | an auto-detected pool larger than the cap, one role recording rows for all but one of its items: after `--record`, `--status` reports that role *graded* one fewer than its cap *of* the pool, and the other roles the cap of the pool. At `HEAD` neither surface carries a per-role count | store the checklist length as the graded count, which reports the role's unanswered item as graded |
| 9 | §4.2 *no emission* | a round recorded under a number no emission produced reports its pool as unknown. *Deferred* | carry the previous round's pool forward |
| 10 | §4.3 | a spec longer than the excerpt budget with a two-byte character straddling it: the payload states the excerpt's size and the spec's, and the excerpt ends on a character boundary. At `HEAD`: stderr 0 bytes, no payload field, and the payload's excerpt ends in U+FFFD | cut at the byte budget, the shipped behaviour |
| 11 | §4.3 *uncut* | a spec under the budget reports an excerpt size equal to the spec's. *Deferred* | report the budget as the excerpt size whatever the spec's length |
| 12 | §5 d1 | a branch that merged the base branch, audited with `--base` naming a local ref older than the merged commit, emits the stale-base notice counting 1 file; with `--base` naming the current base, no notice. At `HEAD`: no notice either way, and the stale base's universe holds 2 files against 1 | fire whenever the range holds a merge commit, which also fires on the current base |
| 13 | §5 d2 | a done task's files plus one committed file no task touched, audited with `--affected-from-tasks`: a notice names the untouched file. At `HEAD`: stderr 0 bytes and the file absent from `files` | no notice, the shipped behaviour |
