# tp v1.1.0 — The spec follows tested behaviour

The release after `v1.0.1`, written before any backlog file is opened, from what that cycle measured
and from a survey of how other tools and communities write and review specifications. Its
measurements and the survey's sources are in `spec/1.1.0-measurements.md`; this file states the
decision and stands without it.

Class: **tool** — it adds a required field to what `tp review --record` accepts and changes no
convergence signal, so the loop is not reviewing its own mechanism here. Budget it at the *everything
else* median rather than the *loop reviews itself* one; both are in `CLAUDE.md`'s *What a cycle
costs* table.

## 1. The decision

**Context.** A review finding today leaves a round in exactly two ways: the spec changes, or the
finding is dispositioned `wontfix`/`duplicate` with evidence. Both exits keep the loop on the
document. The `v1.0.1` cycle measured what happens when the document's subject is code that does not
exist yet: five grading rounds refuted five successive sets of sentences about `tp lint`'s new
fields, each set written by the repair for the round before, while the first five implementing tasks
each corrected something a previous unit had asserted — every correction from a run, none from a
reading. Two field cycles measured the same shape from the other side: one recorded hundreds of
findings and not a single disposition over eight rounds, its spec growing by most of its length with
zero tasks decomposed; the other spent its hardest rounds on files the audit checklist never showed
it. The survey found no tool and no community that reviews behaviour prose to convergence: the ones
that ship separate *what* from *how*, forbid code structure in the reviewed document, defer corner
cases to implementation, and verify behaviour by executing examples rather than by reading sentences.
The one measured study of adversarial review agents found the loop optimises for agreement until each
finding must cite an artifact that confirms or refutes it.

**Decision.** Three things, two in tp and one already in the skill:

1. **A review finding carries its evidence into the record.** `tp review <spec> --record` refuses a
   row that does not say what was run or read to reach the finding.
2. **The record stays readable.** Rounds recorded before this release load, clean and converge as
   they did; the evidence requirement is enforced at `--record` only.
3. **The skill states what a spec is about.** A sentence that would have to change if the
   implementation changed is not spec. The reviewed document carries decisions, alternatives,
   non-goals and executable acceptance rows; function names, derivation paths and exit-code branches
   of behaviour the release will create are filled in by the implementing task, not predicted by the
   author.

**Consequences.** A finding that does not name what was done to reach it does not enter the record at
all. What this does not do is verify the evidence — that is the undecided *evidence contract* in
`spec/undecided.md`, and this release ships the carrier only.

**Alternatives considered.** A `kind` label on findings (`decision | behaviour | text`) with
convergence counting only the first two: rejected because a label is a reading and the cycle showed
readings are what fail. A lexical `implementation-detail` lint over spec prose: not taken, because it
has not been prototyped, and this repository's rule is that a lint candidate is prototyped against
its own corpus before a spec names it. That corpus has refused candidates and accepted others —
`spec/undecided.md`'s *Refuted* section holds refused ones and `README.md`'s rule table holds
accepted ones, `broken-cross-ref` and `duplicate-paragraph` among them — and this candidate has been
through neither; it is recorded under *Undecided* in `spec/undecided.md`. A fixed round count in
place of convergence, as the surveyed tools use: not taken here, because `review_max_rounds` already
provides the cap and changing what `tp import` accepts at a cap stop is the operator's fence, not
this release's.

## 2. The evidence field on a review finding

A row handed to `tp review <spec> --record` carries two new keys beside `severity`, `finding` and
`location`:

| key | type | rule |
|---|---|---|
| `evidence_kind` | string | one of ground's `tier` vocabulary, as `spec/1.0.0.md` §4.1 defines it |
| `evidence` | string | non-empty; what was done at that tier — the command and the output it produced, the query, the probe that was built, or the artifact and line that was read |

The vocabulary is ground's rather than a new pair because ground already separates shapes a two-value
`run`/`read` enum merges: a guard is evidence only once its subject has been broken and the control
run, and a defect only once a test has been observed failing. Both are "a run" under two values, and
ground refuses to call them one. Membership in that list is the whole of the rule — **tp records
evidence, it does not verify it** — and whether the tier a row names says anything about the claim
that row makes is the open *evidence contract* in `spec/undecided.md`.

## 3. What ships in `skills/tp/SKILL.md`

The four rules are already in the tree, under `skills/tp/SKILL.md`'s *What a spec is about (v1.1.0)*,
landed by commit `4914215f` before this file was written. What this release does with them is
distribute them: they reach a user at the tag, not at the commit. The survey sources behind each are
in the measurements file.

## 4. Non-Goals

- **tp does not verify evidence.** It records that a reviewer named what it did; it does not re-run
  the command or open the file. That is the *evidence contract* in `spec/undecided.md` and it has no
  design yet.
- **No `kind` taxonomy on findings**, for the reason under *Alternatives considered*.
- **No change to what `tp import` accepts at a cap stop.** `--force` stays the operator's decision;
  this release changes what the operator can say about the findings before taking it.
- **No lexical lint for implementation detail in spec prose**, for the reason under *Alternatives
  considered*.
- **No change to the audit phase.** Audit rows already carry `evidence_file` and `evidence_lines`.
  The audit side of the acceptance channel is `spec/backlog/00-a-finding-can-leave-a-round.md`.
- **No change to ground.** Ground rows already carry `tier` and `evidence`; this release takes
  ground's vocabulary and adds nothing to ground.
- **The survey's other findings each already have a backlog file.** The panel record, the checklist
  cap, the emit-time hash and the brief's forced sentences; the survey changed none of their order.

## 5. Tests

Each row names its fixture, its mutant, and what the mutant changes. A row whose mutant cannot make it
red is not in this table; two counts per row are filled by the implementing task under Step 0.5's
rule, since the subject does not exist at `HEAD`.

| # | WHEN | tp SHALL | fixture | mutant |
|---|---|---|---|---|
| 1 | a findings file handed to `tp review <spec> --record` holds rows with no `evidence` key | exit 1 naming **every** offending line, write no round file, leave `review_rounds` unchanged | a four-row file whose second and fourth rows omit `evidence`, the other two legal | a check that names the first offender and stops: the fourth line goes unnamed while the assertion on the second still passes |
| 2 | a row's `evidence_kind` is outside ground's tier vocabulary | exit 1 naming **every** offending line and listing the legal values, and record the same file once the offending rows are removed | a four-row file: `guess` and `Run` illegal, `read` and `break-and-control` legal | the first-offender shape of row 1; and a check holding a two-value `run`/`read` pair, which refuses the legal `break-and-control` row |
| 3 | every row carries both keys legally | record the round as today | a three-row file, one `run`, one `read`, one `probe` | this row pins that 1 and 2 did not over-refuse; its mutant is any refusal on a legal file |
| 4 | `tp review --merge` clusters two findings sharing `(location, class)` that differ only in one of them carrying the evidence keys | the merged row carries the evidence, whichever role supplied it | two input files with identical `location`, `class`, `severity` and finding text, the evidenced row's role sorting after the unevidenced one | a representative chosen by severity, then role, then finding text, with evidence no tiebreaker: the evidence is dropped, the merged row still asserts `found_by: 2`, and `--record` then refuses the merged file |
| 5 | a row is missing a field that one of `--merge` and `--record` requires | both refuse it, and a row both accept is accepted by both | two rows: one carrying full evidence but no `location` and no `severity`, one legal on every key | the two gates keep different required-field sets: `--merge` exits 1 having skipped the first row while `--record` records it at exit 0 |
| 6 | `tp review <spec> --status` reads a round recorded before this release, whose rows carry no evidence keys | exit 0 (or 1 under `--check`, as before) and the same `consecutive_clean` the previous binary reports | a pre-`1.1.0` round file copied under `testdata` — not the live corpus, which stops being uniformly pre-`1.1.0` at this release's own first recorded round | a status path that applies the record-time check on load: exit 1 |
| 7 | a review role prompt is emitted | the prompt's output-format section names `evidence` and `evidence_kind` | any spec, `--role implementer` | an emission that drops the keys from the format: absent |
