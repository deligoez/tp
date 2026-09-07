# Backlog — unreleased work, deliberately without version numbers

These files were named after releases (`1.38.0`, `1.44.0`, …) and the numbers were a liability. A
number is a promise about ordering, and this set has been renumbered three times; `CLAUDE.md` records
the last sweep finding **47 citations to renumbered specs, of which ~35 were stale**. The numbers here
are a **priority**, not a version: `01` ships before `02`, and nothing else is implied. A release
number is assigned at the tag, once.

**These are not release-ready specs.** Each one was written against the tree and ground, none has been
reviewed, and several are marked below for merging or cutting. The plan is to re-split them by
priority — merge the ones that share a subject, delete what does not survive the cut — and only then
put one through `lint → ground → review → decompose → implement → audit`.

## What decides the order

The operator's complaint is the ordering rule, in their own words: **tp's cycles drag on, and they
jump into things they should not be solving.** Two field reports measured both halves:

- **drags on** — a cycle recorded **584 findings and 0 dispositions** over 8 rounds. The acceptance
  channel was never used, so every finding's only exit was editing the spec; the document grew
  852 → 1547 lines and **seven of round 8's eight heaviest findings sat inside round 7's own repairs**.
  Zero tasks were decomposed.
- **jumps into the wrong things** — over v0.37.0's seven audit rounds, **13 files ever reached a code
  lens while 29 that `spec-coverage` cited never did**, among them that release's own new engine file.
  `internal/cli/unattended.go`, where the release's hardest four rounds were spent, changed in 6 of 6
  inter-round diffs and was on the generated checklist in **0 of 7 rounds**.

So `01` and `02` are the second half, `03` and `05` the first. Everything from `06` down is tp's own
housekeeping: real, measured, and not what the operator is paying for right now.

## The order

| # | file | subject | class | note |
|---|---|---|---|---|
| 00 | `00-a-finding-can-leave-a-round.md` | the audit side of the acceptance channel: a finding accepted with recorded justification counts toward `clean` | loop | first after `1.1.0`, which takes the review side. Formerly `spec/1.0.2.md`; two ground rounds are recorded, a third was emitted and never graded. Its sidecar is `00-a-finding-can-leave-a-round-measurements.md` — the first backlog file to carry one |
| 01 | `01-checklist-covers-what-changed.md` | rank the audit checklist by churn, never truncate what the operator named, say so when it truncates | tool | the "wrong things" half, measured above |
| 02a | `02a-round-knows-its-panel.md` | the round records the panel it expected; a round missing an expected role is not clean | loop | absorbs 02b |
| 02b | `02b-what-a-rounds-rows-say.md` | **to be absorbed by 02a, then deleted** | — | see *Merges* |
| 03 | `03-next-action-recommends-delta-pass.md` | one branch on a shipped surface after a repair touching more than three sections | tool | the review-side analogue of ground's carry |
| 04a | `04a-ground-command-friction.md` | four measured defects in `tp ground`'s own surfaces | tool | 987 lines; needs cutting before it is worth a cycle |
| 04b | `04b-ask-and-envelope-two-zeros.md` | `# 0 in floor, 0 cut` and `# 0 in floor, N cut` produce byte-identical asks | tool | merge into 04a; its round 1 was never repaired |
| 05 | `05-forced-commitment-in-the-brief.md` | three sentences tp emits, so an unattended run gets the brief a human writes by hand | loop | briefed roles filed 15 and 23 against a control of 35, both rounds |
| 06a | `06a-round-records-the-text-it-read.md` | the round's `spec_hash` is written at emit, not at record | loop | merge with 06b |
| 06b | `06b-spec-hash-reset.md` | `consecutive_clean` resets when the spec it is a claim about changes | loop | same mechanism's other half |
| 07a | `07a-gate-sequence.md` | `quality_gate` as an ordered array of named entries | tool | merge with 07b, 07c |
| 07b | `07b-mutation-score-gate-entry.md` | establish a run completed and over which mutants before reading a score | tool | one input type of 07a |
| 07c | `07c-red-gate-procedure.md` | the bounded procedure a unit follows when the gate goes red | — | **not a release**: text in a brief, enforcing nothing. A `SKILL.md` section |
| 08a | `08a-binary-not-built-from-head.md` | advisory when the running binary is not built from `HEAD` | tool | merge with 08b |
| 08b | `08b-untracked-task-file.md` | a second advisory, once, at `PhaseRelease` | tool | two triggers, two sections, one cycle |
| 09 | `09-loops-own-state-writes.md` | the round findings file is not atomic; the gate's walk cannot see an empty watched directory | tool | its third defect moved to the hotfix — see *Already taken* |
| 10 | `10-forward-spec-ref-lint.md` | a spec must not name a spec numbered above itself that has not yet shipped | tool | **re-check whether this survives**: the backlog no longer carries version numbers, which may remove the rule's whole subject |
| 11 | `11-reconcile.md` | records why the spec moved between rounds without overwriting what the round read | loop | needs 06a |
| 12 | `12-repair-locality.md` | the share of a round's findings sitting in text the previous round wrote, reported and gating nothing | loop | measures the "drags on" half; reports only |
| 13a | `13a-guards-read-what-production-reads.md` | three test helpers re-parse Markdown while production toggles on fences | tool | merge with 13b |
| 13b | `13b-refusals-that-name-nothing.md` | refusals that do not name the set they refused against | tool | same subject; §9 adds two silent-failure findings routed here by `v1.0.1`'s audit round 2 |
| 14 | `14-what-the-carry-can-promise.md` | deleting the earlier of two identical units carries the `PASS` onto the survivor | loop | keep separate: it changes carry semantics |

## Merges

Each merge is one cycle instead of two or three. The split rule — *if a piece can be released
independently, it is its own release* — is not violated by merging a spec with its own dependency, and
the measured cost of a cycle is set by its **class** (loop ≈ 23 rounds, tool ≈ 11) rather than by its
length, so folding three loop-class specs into one cycle is cheaper than three.

- **02a + 02b.** 02b's three parts split cleanly: the accepted-finding half went to the hotfix (see
  below), and its other two — a fenced list field reading `expected_roles`, and reporting a `PASS` row
  that carries a note — belong with 02a's panel record. The `PASS`-note half does not technically need
  `expected_roles`; it goes there for subject, not dependency.
- **04a + 04b.** Both are `tp ground`'s own surfaces.
- **06a + 06b.** The emit-time hash and the reset that consumes it are one mechanism. `11` stays out:
  it adds a command and takes a design decision.
- **07a + 07b + 07c.** 07b is an input type of 07a; 07c is not a release at all.
- **08a + 08b.** The roadmap kept these apart because the triggers differ. A different trigger is not a
  release boundary when a release costs 4–8 rounds of fixed overhead.
- **13a + 13b.** One subject: a guard or a refusal that does not carry its own claim. **14 stays out**
  — it is loop-class, and merging it would make the whole cycle run at loop price.

## Already taken by the hotfix

Do not re-specify these; they ship in `spec/1.0.1.md`:

- `unresolved_findings` returning the complement of the answer, plus the two counters.
- A refused `--role` invocation writing a snapshot before refusing — **this was `09`'s first defect**.
  Drop it from `09` when the hotfix ships, or two releases claim the same repair.
- An accepted audit finding blocking convergence forever — **this was `02b`'s headline**.

## Deferred with a reason

`04a` is the largest file here and it is deliberately not near the top: the operator's complaint is
about the review and audit loops, and `tp ground` is neither. It moves up once the first four ship.

## How to pick one up

Read the file, cut what the round-1 grounding already answered, apply the merge above, and **then**
give it a version number — at the tag, not before.
