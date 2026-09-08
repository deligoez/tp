# Backlog — unreleased work, named by slug, ordered here

Every file in this directory is a pending spec named by its **subject**. The order lives in this
file and nowhere else. The files used to carry a priority number in their name (`01-…`, `02a-…`);
that number rotted every citation the moment the order changed — a sweep at the last reordering found
that every one of the citations to these files carried the number, so none could survive a
re-prioritisation. Two files keep their old numbered name as a forwarding stub because a shipped
artifact cites that path; nothing else does. A release number is assigned at the tag, once.

**These are not release-ready specs.** Each was written against the tree and grounded, none has been
reviewed. On 2026-09-08 every one was re-verified against `HEAD` after `v1.0.1`, cut to its decisions
(the measurements moved to a `<slug>-measurements.md` beside it), split or merged on the seams the
verification found, and ordered by measured benefit. The survey behind that is summarised under
*What the re-verification changed* below; its per-file records are in each sidecar.

## What decides the order

The operator's complaint is the ordering rule, in their own words: **tp's cycles drag on, and they
jump into things they should not be solving.** Two field reports measured both halves, and the
`v1.0.1` and `v1.1.0` cycles measured them again on this repository:

- **drags on** — a field cycle recorded hundreds of findings and not one disposition over eight
  rounds, so every finding's only exit was editing the spec; the document nearly doubled and zero
  tasks were decomposed. On this repository the audit channel cannot accept a finding at all: a
  `wontfix` written into the recorded round changes nothing in `--status` (measured at `HEAD`, see
  `a-finding-can-leave-an-audit-round-measurements.md`). Five grading rounds of `v1.0.1` refuted five
  sets of sentences about unbuilt behaviour; the implementing tasks then found what the rounds could
  not, by building.
- **jumps into the wrong things** — over one release's seven audit rounds, thirteen files reached a
  code lens while twenty-nine the spec cited never did; the file its hardest rounds were spent on was
  on the generated checklist in none of them. The cut is live at `HEAD`: a plain `tp audit` on this
  tree hands each code role ten files of forty-nine, silently.

A spec ranks by whether it has a **measurement against one of those two halves**, then by class
(tool before loop, because a loop-class cycle costs about twice a tool-class one — the derivation is
`CLAUDE.md`'s *What a cycle costs*), then by size. A spec with no measured benefit ranks below every
one that has one, whatever its subject.

## The order

| # | file | subject | class | measured benefit |
|---|---|---|---|---|
| 1 | `a-finding-can-leave-an-audit-round.md` | audit `--resolve` re-stamps the recorded round, so an accepted finding clears it | loop | drags on — the audit channel accepts nothing today (probe in its sidecar); the field cycle above |
| 2 | `checklist-covers-what-changed.md` | rank the audit checklist by churn, never cap an operator-named list, say so when it truncates | tool | wrong things — the measurement above; live at `HEAD` |
| 3 | `brief-carries-the-forcing-sentences.md` | the audit framing emits the two sentences a human brief carries by hand | loop | drags on — the only controlled measurement in this directory: briefed roles filed 15 and 23 against a control of 35, both rounds |
| 4 | `decomposition-is-built-before-import.md` | one probe round builds each task's smallest change in a clone before `tp import` | loop | drags on — three of ten code tasks in `v1.1.0` needed no code; sixty-eight broken tests that five grading rounds never saw |
| 5 | `a-findings-exits-agree.md` | one predicate for open findings across `--status`, `tp resume`, `--check`; refusals write nothing; payloads name the file they wrote | tool | a refused `--role` still writes state at `HEAD`; `duplicate` disagrees between two surfaces |
| 6 | `repair-locality.md` | report the share of a round's findings sitting in text the previous repair wrote | tool | drags on — the number the complaint is made of, reported by nothing today |
| 7 | `next-action-and-check-tell-the-truth.md` | `tp ground --status --check` exits 1 over a standing `FAIL`; ground gains `next_action`; review's `next_action` recommends the delta pass after a wide repair | tool | a driver stops on exit 0 with `FAIL`s standing — both field reports, reproduced |
| 8 | `round-knows-its-panel.md` | the round records the panel it expected; a round missing an expected role is not clean; `--status` reports the round in flight; role-scoped convergence as a seam | loop | `--check` exits 0 today on two rounds where two of three roles never ran (measured); `--check` is not the ship signal until this ships |
| 9 | `round-records-the-text-it-read.md` | `spec_hash` is the hash of the emission snapshot; the clean streak resets when consecutive rounds read different text | loop | thirty-five of one hundred seventy-seven recorded review rounds carry a hash the round did not read |
| 10 | `gate-sequence.md` | `quality_gate` as an ordered array of named entries, `tp gate` runs it, CI invokes it; two narrow guards widened | tool | five of one release's thirteen audit rounds went to CI restating the gate |
| 11 | `refusals-that-name-nothing.md` | refusals name the set they refused against; an invalid task file is reported instead of yielding zero findings | tool | a silent wrong answer, measured; plus four more routed at the re-verification (its sidecar) |
| 12 | `two-advisories.md` | the binary is not built from `HEAD`; the task file is untracked at release | tool | raw-stderr sites `--quiet` cannot silence, counted with its counting rule in its sidecar |
| 13 | `loops-own-state-writes.md` | the round findings file is written atomically; the gate's digest sees a directory-only change | tool | housekeeping; no field instance |
| 14 | `reconcile.md` | `--reconcile --note` records why the spec moved, as a typed field; absorbs `spec_moved_mid_round` | tool | one fabricated zero-byte round in the corpus |
| 15 | `ground-command-friction.md` | the remaining `tp ground` surface defects, plus the two-zeros tasks | tool | none against either complaint |
| 16 | `what-the-carry-can-promise.md` | a multiplicity fence on the carry's join | loop | none fired in forty-three recorded rounds |

## Not releases

- `red-gate-procedure.md` — a `skills/tp/SKILL.md` section; ships as a doc task of `gate-sequence`.
  The section does not exist in the skill at `HEAD`, whatever an earlier version of this file said.
- `mutation-run-check.md` — a script plus one `CLAUDE.md` line; it excludes itself from the per-task
  gate and needs no tp code.

## Registered questions, ranked by what they unblock

`spec/undecided.md` is the register — every entry there names the decision nobody has taken — and
`spec/undecided-measurements.md` is its forensics. Neither is a spec, and an entry is not a draft of
one. The entries below are the ones that stand in front of a file in this directory: **the third
column names the backlog spec each unblocks, and *The order* above is what ranks those.** An entry
listed here cannot be settled by writing its spec first.

| # | registered question | unblocks |
|---|---|---|
| 1 | *The divisible round* | `checklist-covers-what-changed.md`, which names it as its real answer |
| 2 | *`NewRootCmd` writes package globals* | `gate-sequence.md` |
| 3 | *A registered check that outlives its release* | `gate-sequence.md` |
| 4 | *`scope` on audit rows* | `round-knows-its-panel.md` §4a, and `a-finding-can-leave-an-audit-round.md` |
| 5 | *A durable home for an accepted finding*, with *Making `severity` checkable* | `a-finding-can-leave-an-audit-round.md` |
| 6 | *A review-side `accepted_blocking`* | `a-findings-exits-agree.md` |
| 7 | *Claim enumeration in the grounding floor*, with the one-round exemption for a sentence rewritten in answer to a finding | `ground-command-friction.md` |
| 8 | *`t.Parallel()` in the engine package* | `mutation-run-check.md` |
| 9 | *A prior-round section for `tp review`* | `repair-locality.md` — it changes how that spec's number is read |

The remaining entries of `spec/undecided.md` unblock nothing in this directory and are ranked on
their own merit in that file.

## What the re-verification changed

- **Three items this file called "already taken by the hotfix" were not.** `v1.0.1` shipped none of
  them: `unresolved_findings` still counts the complement of the answer, a refused `--role` still
  writes a snapshot, and an accepted audit finding still gates. They are rows 1 and 5 above.
- **Dropped:** the forward-spec-ref lint — its population vanished with the version numbers (one
  finding on `spec/`, none here) and its replacement predicate had no positive set; what the corpus
  needs is a dead-path check, recorded in `spec/undecided.md`. The PASS-note counter — it measures a
  constant. The whitespace set for the floor — no unit in the corpus is affected. The flip rows in the
  brief spec — no recorded round measures what returning them would do.
- **Merged:** the emit-time hash with the reset it enables (row 9); the two advisories (row 14); the
  two-zeros tasks into the ground friction spec (row 15); the guard-helper spec into the refusals
  spec as tasks (row 13). Rejected: the mutation check into the gate sequence — it is not an entry
  of that gate.
- **Split:** the old hotfix file into rows 1 and 5 (a loop-class change and a tool-class one); the
  ground friction spec's `--check` and `next_action` decisions into row 7, where the delta-pass
  branch joins them.
- **Forwarding stubs** at `02b-what-a-rounds-rows-say.md` and `12-repair-locality.md`, because shipped
  sidecars cite those paths.

The three candidates files — `spec/0.33.0-candidates.md`, `spec/0.34.0-candidates.md` and
`spec/0.35.0-candidates.md` — were re-verified against `HEAD` the same day, and each now opens with a
dated block saying what shipped, what moved and what is still open. Most of their items had shipped.
Twelve open ones were routed into the sidecars those blocks name — the refusals, advisory,
findings-exits, state-writes, checklist and mutation-check sidecars — so nothing open is carried by a
candidates file alone. The items whose *design* has no answer had no spec to go to and are registered
in `spec/undecided.md` instead; *Registered questions, ranked by what they unblock* above lists them.

## Ground records

A spec's ground rounds live under `.tp-review/<slug>/` beside this file and moved with the file when it
was renamed or merged. Directories of absorbed or dropped specs stay as history — they are rows in the
corpus derivations. Every cleaned spec changed its text, so its next ground round starts with no carry;
that is the expected cost of the cut, not a defect. Several directories hold a round that was emitted
and never recorded; `scripts/clean-emissions.sh` names them, and they are not deleted by hand.

## How to pick one up

Read the file and its sidecar, apply Step 0.5 of `skills/tp/SKILL.md`, build the change in a clone
and run the suite before the first review round, and **then** give it a version number — at the tag,
not before.
