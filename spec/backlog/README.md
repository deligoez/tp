# Backlog — unreleased work, named by slug, ordered here

Every file in this directory is a pending spec named by its **subject**. The order lives in this
file and nowhere else. The files used to carry a priority number in their name (`01-…`, `02a-…`);
that number rotted every citation the moment the order changed — a sweep at the last reordering found
that every one of the citations to these files carried the number, so none could survive a
re-prioritisation. A release number is assigned at the tag, once.

**These are not release-ready specs.** Each was written against the tree and grounded, none has been
reviewed. On 2026-09-08 every one was re-verified against `HEAD` after `v1.0.1`, cut to its decisions
and ordered by measured benefit. On **2026-09-11** every one was re-verified again, against `18032abe`
(after `v1.1.1`), together with a field report from a Laravel project on `v1.1.1` (WB-3155) whose
thirty-three items were each reproduced or refuted before any was used; *What the 2026-09-11 pass
changed* below says what that moved.

## What decides the order

The operator's complaint is the ordering rule, in their own words: **tp's cycles drag on, and they
jump into things they should not be solving.** Two field reports measured both halves, the `v1.0.1`
and `v1.1.0` cycles measured them again on this repository, and WB-3155 measured them a third time:

- **drags on** — a field cycle recorded hundreds of findings and not one disposition over eight
  rounds, so every finding's only exit was editing the spec; WB-3155 reports losing a week the same
  way on the review side, and lost seventy-five audit dispositions on the audit side because a
  disposition written to a file that is no longer read reports success (reproduced). On this repository the audit channel cannot accept a finding at all.
- **jumps into the wrong things** — a plain `tp audit` hands each code role ten files of however many
  changed, silently; WB-3155 showed the same cut surviving the workaround tp's own notice recommends,
  with a `FAIL` dropped at merge because two shards gave two files the same item id.

A spec ranks **first by whether tp, at `HEAD`, silently loses the agent's work or reports something
false** — the hotfix tier, admitted only on a reproduction at `18032abe` — then by whether it has a
measurement against one of the two halves above, then by class (tool before loop, because a
loop-class cycle costs about twice a tool-class one; the derivation is `CLAUDE.md`'s *What a cycle
costs*), then by size. A spec with no measured benefit ranks below every one that has one, whatever
its subject.

## The order

| # | file | subject | class | why here |
|---|---|---|---|---|
| **hotfix** | | | | |
| 1 | `audit-records-what-was-graded.md` | a `file_check` id is stable across rounds and shards; `--merge`/`--record` refuse conflicting duplicates; the conformance role reads every closing sha; a disposition that does not reach a recorded round says so, and `--resolve` stops recommending a re-record that fabricates a round | tool | a `FAIL` dropped at merge and a round converged over files nobody graded; the skill's own audit step sends dispositions to a file that is no longer read (reproduced) |
| 2 | `a-task-file-write-names-its-target.md` | every task-file write names its file, a stale `tp use` pointer is announced, a claim can be given back, batch `commit` takes an array, a bullet is one criterion | tool | `tp remove` deleted an open task from another spec's file at exit 0 through a leftover pointer (reproduced); a `wip` task has no exit but a false closure |
| 3 | `the-floor-names-what-it-cut.md` | the splitter does not end a sentence at an ordinal or a glued list marker; cut spans can be named; absorbs `floor-anchors-need-fixtures` | tool | a claim split at `6.` has its claim half cut and graded by no round (field, reproduced); on this repository's own specs a list glued to its lead-in line loses items the same way (sidecar census) |
| 4 | `emitting-does-not-lose-a-round.md` | re-emitting over an unrecorded round whose spec changed refuses, in all three phases; the scratch filename carries the spec | tool | one full round of grading lost on this repository; the overwrite reproduces in all three phases |
| 5 | `a-findings-exits-agree.md` | one predicate for open findings; a refusal writes nothing; `--record` names its file; `--status` counts dispositions; the carry header stops calling accepted rows unresolved; `--round` is refused as a flag | tool | a clean audit round makes `tp resume` report unresolved findings; `duplicate` disagrees between two surfaces (both reproduced) |
| **high** | | | | |
| 6 | `a-finding-can-leave-an-audit-round.md` | an accepted audit finding clears its round, the next round's auditor sees the acceptance, and the open counts read the same set; the disposition write is fenced | loop | drags on — the audit channel accepts nothing today; `v1.1.0` needed two cap raises for it |
| 7 | `checklist-covers-what-changed.md` | rank the audit checklist by churn, never cap a named list, record graded/total per role, say so when anything is cut | tool | wrong things — ten files per role at `HEAD`, silently; the conformance role's spec excerpt is cut without notice |
| 8 | `next-action-and-check-tell-the-truth.md` | `tp ground --status --check` exits 1 over a standing `FAIL`; ground gains `next_action`; a check that could not run does not suppress its class | tool | a driver stops on exit 0 with `FAIL`s standing (both field reports, reproduced) |
| 9 | `hooks-fence-the-target.md` | the write-deny hook fences the write target, not every path a call names; the test-file fence's permission is computed at spawn; `test_globs` follows the checks-list layering | tool | decided 2026-09-08; WB-3155's graders were refused a read the ground prompt tells them to make (reproduced) |
| **medium** | | | | |
| 10 | `reconcile.md` | a rewritten spec starts a new epoch: the stale blocker names its command, prior rounds stop feeding the carry, a round's hash is the text it read | tool | WB-3155 hand-dispositioned eight round files after a rewrite; `spec-stale` names no command |
| 11 | `gate-sequence.md` | `tp gate`, one executor, the gate checked once at init/import, a per-spec `quality_gate` | tool | an unrunnable gate imports at exit 0; a single spec's gate cannot be set without `import --force`; an unattended run can rewrite the project gate at exit 0 (all reproduced) |
| 12 | `refusals-that-name-nothing.md` | a refusal names each bad row, its cause and the accepted set, and cites none of tp's own design sections; absorbs record-diagnoses, the guard listing and the kind/tier gloss | tool | drags on — pairings refused by a message naming no set; `--merge`'s hint blames format for a missing field |
| 13 | `round-knows-its-panel.md` | the round records the panel it expected; a round missing a role is not clean | loop | `--check` exits 0 with two of three roles never run (fixture); check `audit_converge_on: blocking` first |
| 14 | `brief-carries-the-forcing-sentences.md` | the audit framing emits the two sentences a human brief carries by hand | loop | the one controlled measurement here; ships after row 6, same prompt block |
| 15 | `what-the-record-does-not-say.md` | a carried verdict can be overridden and is counted; the carry's join is fenced; claims are counted as claims | tool | one field instance of a stale `PARTIAL` carried indefinitely |
| **low** | | | | |
| 16 | `an-unreadable-file-is-named.md` | tp says when it could not read — a mistyped frontmatter key, an unreadable file, an invalid task file, raw stderr `--quiet` cannot silence, a cut inside a rune | tool | wrong things, weakly — no field instance; the frontmatter typo is live |
| 17 | `a-round-can-be-driven-from-the-envelope.md` | the ground envelope carries `asked`, `floor_delta` and `cut` | tool | drags on in mechanism only |
| 18 | `a-rewrite-names-what-it-dropped.md` | list the normative units of an earlier version absent from the current one — prototype first, with a kill condition | tool | WB-3155 found two live rules lost in a rewrite by hand; unproven that a listing separates loss from rewording |

## Not releases

- `red-gate-procedure.md` — a short doc note for the close recipe; runs on today's `&&` gate.
- `mutation-run-check.md` — a script plus one `CLAUDE.md` line, written the next time a pre-release
  mutation run happens; it excludes itself from the per-task gate and needs no tp code.
- `repair-locality.md` — a script, not a feature: the number saturates on Step 0.5-sized cycles and
  gates nothing. Its source is in its sidecar until someone commits it under `scripts/`.

## Stubs — forwarding only, not in the order

Each keeps its `-measurements.md` sidecar and its `.tp-review/<slug>/` rounds as history, because
shipped specs, round files or `CLAUDE.md` cite the path. The stub says what moved where.

| stub | became |
|---|---|
| `decomposition-is-built-before-import.md` | dropped — its motivating rule shipped as `skills/tp/SKILL.md` Step 2 |
| `floor-anchors-need-fixtures.md` | a test task of `the-floor-names-what-it-cut` |
| `scratch-name-is-unique-per-spec.md` | `emitting-does-not-lose-a-round` |
| `loops-own-state-writes.md` | its atomic writes → `a-findings-exits-agree`; its gate digest item dropped |
| `round-records-the-text-it-read.md` | its emit-time hash → `reconcile`; its streak reset parked in `spec/undecided.md` |
| `what-the-carry-can-promise.md` | `what-the-record-does-not-say` |
| `record-diagnoses-every-bad-row.md`, `the-guard-pins-the-whole-listing.md` | `refusals-that-name-nothing` |
| `guards-read-what-production-reads.md` | dropped — its guards bind a frozen shipped spec; the doc-comment fix → `refusals-that-name-nothing` |
| `an-invalid-task-file-is-reported.md`, `context-is-cut-on-a-rune-boundary.md` | `an-unreadable-file-is-named` |
| `two-advisories.md` | the untracked-task-file advisory → `a-task-file-write-names-its-target`; the stderr sweep → `an-unreadable-file-is-named`; the binary advisory dropped |
| `ground-command-friction.md` | split on 2026-09-08 into the ground specs above |
| `12-repair-locality.md` | `repair-locality.md` (a shipped spec cites the numbered path) |

## Registered questions

`spec/undecided.md` is the register and `spec/undecided-measurements.md` is its forensics. Neither is
a spec, and an entry is not a draft of one. The 2026-09-08 decision pass took every registered
question; the 2026-09-11 pass added three closed entries (a check library from the field, the parked
streak reset, and a refuted `as_of` lint) and recorded a field instance against *Cross-repo specs*
without reopening it. Nothing in this order waits on a decision.

## Decided, awaiting a spec

- **`round-divides-by-section`** — *the divisible round* is decided: the split key is spec **location**
  (the section). A round is divided into shards by section, each shard one prompt carrying that
  section's checklist items; per-item convergence is unchanged. A **tool** spec, written after
  `checklist-covers-what-changed.md` ships, and only after `audit-records-what-was-graded.md`, since
  every shard would otherwise restart the id collision that release removes. WB-3155's half-megabyte
  conformance prompt is its field case. Recorded in `checklist-covers-what-changed-measurements.md`.
- **A derived `class` in `tp lint`'s report** — `tp lint` reports a derived `class` beside `floor_size`
  and `review_panel`, no gate and no frontmatter override until one is argued for; scheduled with the
  next release that touches lint's report rather than given one of its own.

## What the 2026-09-11 pass changed

- **The field report was verified item by item before any of it was used.** Of thirty-three items,
  the ones that moved this order reproduced at `18032abe`; several did not, and each host sidecar says
  which under *Field report WB-3155, verified 2026-09-11*. Refuted or corrected there: the atomicity bar
  never changed (the spec was rewritten), coverage is repairable without `import --force`, `--base`
  already diffs from the merge-base, fixed review findings are not suppressed, `commit_shas` already
  takes an array in a batch, and the "auto-pick among several task files" mechanism was wrong — several
  files refuse; the silent write comes from a leftover pointer.
- **Four specs were written**: rows 1, 2, 9 and 18. Rows 1 and 2 are field defects no spec owned;
  row 9 was decided on 2026-09-08 and had no file.
- **Hotfix tier added** above the measured-benefit rule, admitted only on a reproduction.
- **Folded or dropped**: the twelve stubs above. The bottom of the old order was guard and code
  hygiene with no measured cost; a housekeeping release of it would have been padding, so each item
  went to a spec that already edits the same file, or was dropped with its reason.
- **Rescoped**: `gate-sequence` to what a field user needs (its CI and pin-guard parts are chores);
  `reconcile` to the rewrite epoch; `next-action-and-check-tell-the-truth` lost its delta-pass branch;
  `repair-locality` left the order as a script.
- **Corrections to specs as written**: `a-finding-can-leave-an-audit-round` kept a silent success for a
  resolve into the wrong file, the field's exact trap — it now cites row 1's notice;
  `a-findings-exits-agree` defined a closed finding two ways; `the-floor-names-what-it-cut` said cut
  units have no id (they have had one since `v1.0.0`) and its marker rule would not have caught the
  ordinal split.

## Ground records

A spec's ground rounds live under `.tp-review/<slug>/` beside this file and moved with the file when it
was renamed or merged. Directories of absorbed or dropped specs stay as history — they are rows in the
corpus derivations. Every cleaned spec changed its text, so its next ground round starts with no carry;
that is the expected cost of the cut, not a defect. Several directories hold a round that was emitted
and never recorded; `scripts/clean-emissions.sh` names them, and they are not deleted by hand.

## How to pick one up

Read the file and its sidecar. The 2026-09-11 rewrites were written under Step 0.5 of
`skills/tp/SKILL.md`; a body this pass did not rewrite predates those rules, and is rewritten under
them before ground round 1 — moving text to the sidecar loses nothing, and a sentence about how
unbuilt code works is not kept. Build the change in a clone and run the suite before the first review
round, and **then** give it a version number — at the tag, not before.
