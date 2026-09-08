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
| 1 | `a-finding-can-leave-an-audit-round.md` | audit `--resolve` re-stamps the recorded round, so an accepted finding clears it; **takes `a-findings-exits-agree.md` §4 with it** (the disposition write is fenced under `TP_UNATTENDED` and requires non-empty evidence), because a disposition that clears a round without that fence is an escape hatch an agent can write for itself | loop | drags on — the audit channel accepts nothing today (probe in its sidecar); the field cycle above |
| 2 | `checklist-covers-what-changed.md` | rank the audit checklist by churn, never cap an operator-named list, say so when it truncates | tool | wrong things — the measurement above; live at `HEAD` |
| 3 | `brief-carries-the-forcing-sentences.md` | the audit framing emits the two sentences a human brief carries by hand | loop | drags on — the only controlled measurement in this directory: briefed roles filed 15 and 23 against a control of 35, both rounds |
| 4 | `decomposition-is-built-before-import.md` | one probe round builds each task's smallest change in a clone before `tp import` | loop | drags on — three of ten code tasks in `v1.1.0` needed no code; sixty-eight broken tests that five grading rounds never saw |
| 5 | `a-findings-exits-agree.md` | one predicate for open findings across `--status`, `tp resume`, `--check`; refusals write nothing; payloads name the file they wrote (its §4 ships with row 1) | tool | a refused `--role` still writes state at `HEAD`; `duplicate` disagrees between two surfaces |
| 6 | `emitting-does-not-lose-a-round.md` | re-emitting an unrecorded round is idempotent and says so; emitting over an unrecorded, changed round refuses instead of overwriting its floor | tool | drags on — one full round of grading lost on this repository; six briefs of workaround in a field report for a fear that is false |
| 7 | `repair-locality.md` | report the share of a round's findings sitting in text the previous repair wrote | tool | drags on — the number the complaint is made of, reported by nothing today |
| 8 | `record-diagnoses-every-bad-row.md` | `--record` names every bad row in one invocation, not the first | tool | drags on — the orchestrator wrote its own whole-file validator, which caught two or three violations at once on four occasions |
| 9 | `next-action-and-check-tell-the-truth.md` | `tp ground --status --check` exits 1 over a standing `FAIL`; ground gains `next_action`; review's `next_action` recommends the delta pass after a wide repair | tool | a driver stops on exit 0 with `FAIL`s standing — both field reports, reproduced |
| 10 | `the-floor-names-what-it-cut.md` | cut spans get ids and hashes; bare list markers are not units; no live figure about a file's own floor; the two zeros are told apart | tool | wrong things — thirteen rows across twenty-eight rounds carry `unit_id: null`, and a round's sharpest finding sat in cut text more than once |
| 11 | `round-knows-its-panel.md` | the round records the panel it expected; a round missing an expected role is not clean; `--status` reports the round in flight; role-scoped convergence as a seam | loop | `--check` exits 0 today on two rounds where two of three roles never ran (measured); `--check` is not the ship signal until this ships |
| 12 | `round-records-the-text-it-read.md` | `spec_hash` is the hash of the emission snapshot; the clean streak resets when consecutive rounds read different text | loop | thirty-five of one hundred seventy-seven recorded review rounds carry a hash the round did not read |
| 13 | `gate-sequence.md` | `quality_gate` as an ordered array of named entries, `tp gate` runs it, CI invokes it; two narrow guards widened | tool | five of one release's thirteen audit rounds went to CI restating the gate |
| 14 | `refusals-that-name-nothing.md` | the ground pairing refusal names the tiers it would have accepted | tool | drags on — thirty-three of forty-two pairings writable from the spec's own values are refused by a message naming no set (its sidecar) |
| 15 | `an-unreadable-file-is-named.md` | the review ranking names an unreadable file instead of dropping it in silence | tool | wrong things — a role handed a doc set with a file missing and `reviewed_files` under-reporting, measured against a control matrix |
| 16 | `an-invalid-task-file-is-reported.md` | `tp lint` reports an unreadable or unparseable task file instead of zero findings at exit 0 | tool | wrong things — both failures give zero findings, empty stderr, exit 0 against a control's one finding |
| 17 | `two-advisories.md` | the binary is not built from `HEAD`; the task file is untracked at release | tool | raw-stderr sites `--quiet` cannot silence, counted with its counting rule in its sidecar |
| 18 | `what-the-record-does-not-say.md` | a `FAIL` cleared by an unrelated edit; a silent carry override; a coverage denominator counting non-claims; a verdict the skill never named | tool | wrong things — one field instance of a stale `PARTIAL` carried indefinitely on `spec/1.1.0.md`; the coverage half has spread but no round cost |
| 19 | `scratch-name-is-unique-per-spec.md` | the scratch filename carries the spec, so two concurrent groundings do not overwrite each other at exit 0 | tool | drags on, weakly — every brief in the grounding programme carried a hand-written override |
| 20 | `a-round-can-be-driven-from-the-envelope.md` | the envelope carries `asked` and `floor_delta`; one panel, any number of readers | tool | drags on in mechanism only — the ask set must be rebuilt by a join tp already computes |
| 21 | `guards-read-what-production-reads.md` | the enum guard's doc comment matches a binding that exists; test helpers parse fences the way production does | tool | drags on, weakly — three of fifteen measured inputs are false failures on a correct document |
| 22 | `loops-own-state-writes.md` | the round findings file is written atomically; the gate's digest sees a directory-only change | tool | housekeeping; no field instance |
| 23 | `reconcile.md` | `--reconcile --note` records why the spec moved, as a typed field; absorbs `spec_moved_mid_round` | tool | one fabricated zero-byte round in the corpus |
| 24 | `what-the-carry-can-promise.md` | a multiplicity fence on the carry's join | loop | none fired in forty-three recorded rounds |
| 25 | `the-guard-pins-the-whole-listing.md` | the enum-refusal guard asserts the whole listing, so appended garbage reddens it | tool | none — guard soundness; the mutant is green in both packages and no operator cost is measured |
| 26 | `floor-anchors-need-fixtures.md` | `FloorAnchorOf` bills six kinds of unit to a predictable anchor, with fixtures | tool | none — but it is the reopen condition for `floor_by_section` in `spec/undecided.md` |
| 27 | `context-is-cut-on-a-rune-boundary.md` | a lint finding's `Context` is cut on a rune, never inside one | tool | none — fires on zero of the files under `spec/` |

`ground-command-friction.md` and the sections `refusals-that-name-nothing.md` no longer holds were
split on 2026-09-08 into the slugs above; both files carry a forwarding table, and their recorded
ground rounds stay under the old names in `.tp-review/`.

## Not releases

- `red-gate-procedure.md` — a `skills/tp/SKILL.md` section; ships as a doc task of `gate-sequence`.
  The section does not exist in the skill at `HEAD`, whatever an earlier version of this file said.
- `mutation-run-check.md` — a script plus one `CLAUDE.md` line; it excludes itself from the per-task
  gate and needs no tp code.

## Registered questions

`spec/undecided.md` is the register and `spec/undecided-measurements.md` is its forensics. Neither is
a spec, and an entry is not a draft of one. **On 2026-09-08 a decision pass took every registered
question**; each decision is recorded in `spec/undecided.md` and appended to the sidecar of the spec
that takes it under *Decided at the 2026-09-08 decision pass*, so no entry in that register stands in
front of a file in this directory. The last one, *Cross-repo specs*, was closed by the operator the
same day: no. Nothing waits on a decision; what waits is a spec, listed next.

## Decided, awaiting a spec

Three decisions have no pending spec carrying them, because the release that would take them does not
exist yet. They are listed here so the order above picks them up rather than rediscovering them.

- **`round-divides-by-section`** — *the divisible round* is decided: the split key is spec **location**
  (the section). A round is divided into shards by section, each shard one prompt carrying that
  section's checklist items; per-item convergence is unchanged. A **tool** spec, written after
  `checklist-covers-what-changed.md` ships — folding it into that one would double a spec already
  ranked second. Recorded in `checklist-covers-what-changed-measurements.md`.
- **`hooks-fence-the-target`** — *the test-file fence* and *the write-deny fence's reach* are decided
  together as one small **tool** spec. The write-deny hook matches on the tool's write **target** (the
  file path argument of a write-capable tool) rather than on any argument string, so reads and batched
  multi-file calls stop being refused; the test-file fence resolves its permission **precomputed into
  the child environment at spawn**, never a `tp` call per write inside the hook; and `test_globs`
  follows `pickChecks` — a present list replaces the layer beneath it. Recorded in
  `spec/undecided.md`; no sidecar carries it.
- **A derived `class` in `tp lint`'s report** — `tp lint` reports a derived `class` beside `floor_size`
  and `review_panel`, no gate and no frontmatter override until one is argued for; scheduled with the
  next release that touches lint's report rather than given one of its own.

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
candidates file alone. The items whose *design* had no answer had no spec to go to and were registered
in `spec/undecided.md` instead; the 2026-09-08 decision pass then decided all but one of them, and
*Registered questions* and *Decided, awaiting a spec* above say where each one went.

## Ground records

A spec's ground rounds live under `.tp-review/<slug>/` beside this file and moved with the file when it
was renamed or merged. Directories of absorbed or dropped specs stay as history — they are rows in the
corpus derivations. Every cleaned spec changed its text, so its next ground round starts with no carry;
that is the expected cost of the cut, not a defect. Several directories hold a round that was emitted
and never recorded; `scripts/clean-emissions.sh` names them, and they are not deleted by hand.

## How to pick one up

Read the file and its sidecar, then **rewrite the body under Step 0.5 of `skills/tp/SKILL.md` before
ground round 1**: every body here predates those rules (graded 2026-09-08: two at the shape, twelve
need their forensics, figures and implementation sentences moved to the sidecar, four are decisions
written as symbol names and need a fresh body). Moving text to the sidecar loses nothing; a sentence
about how unbuilt code works is not kept. Build the change in a clone and run the suite before the
first review round, and **then** give it a version number — at the tag, not before.
