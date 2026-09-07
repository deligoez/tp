# tp v1.1.0 — The spec follows tested behaviour

The release after `v1.0.1`, written before any backlog file is opened, from what that cycle measured
and from a survey of how other tools and communities write and review specifications. Its
measurements and the survey's sources are in `spec/1.1.0-measurements.md`; this file states the
decision and stands without it.

Class: **loop** — it changes what a review round accepts and what counts as a resolved finding, so it
is reviewed by the mechanism it changes. Budget it at the loop-class median rather than the tool-class
one; both are derived, not stated, by the command under *What a cycle costs* in `CLAUDE.md`.

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

**Decision.** Four things, three in tp and one in the skill:

1. **A review finding carries its evidence into the record.** `tp review <spec> --record` refuses a
   row that does not say what was run or read to reach the finding.
2. **A finding can leave a round by going forward.** A third disposition, `routed`, names the task
   that will settle it. A routed finding is subtracted from the round's surviving set exactly as
   `wontfix` is, and `tp validate` warns when a routed finding names no task in the task file.
3. **The record stays readable.** Rounds recorded before this release load, clean and converge as
   they did; the evidence requirement is enforced at `--record` only.
4. **The skill states what a spec is about.** A sentence that would have to change if the
   implementation changed is not spec. The reviewed document carries decisions, alternatives,
   non-goals and executable acceptance rows; function names, derivation paths and exit-code branches
   of behaviour the release will create are filled in by the implementing task, not predicted by the
   author.

**Consequences.** A round's finding count stops being the loop's only currency: a finding about
unbuilt behaviour has a destination other than the spec, and a finding without a run or a read
behind it does not enter the record at all. What this does not do is verify the evidence — that is
the undecided *evidence contract* in `spec/undecided.md`, and this release ships the carrier only.

**Alternatives considered.** A `kind` label on findings (`decision | behaviour | text`) with
convergence counting only the first two: rejected because a label is a reading and the cycle showed
readings are what fail; routing is an act the orchestrator takes with a task in hand. A lexical
`implementation-detail` lint over spec prose: not taken, because every lexical candidate prototyped
in this repository so far has died on its corpus, and this one has not been prototyped — it is
recorded under *Undecided*, with the prediction that it fires on most of the shipped corpus. A fixed
round count in place of convergence, as the surveyed tools use: not taken here, because
`review_max_rounds` already provides the cap and changing what `tp import` accepts at a cap stop is
the operator's fence, not this release's.

## 2. The evidence field on a review finding

A row handed to `tp review <spec> --record` carries two new keys beside `severity`, `finding` and
`location`:

| key | type | rule |
|---|---|---|
| `evidence_kind` | string | one of `run`, `read` |
| `evidence` | string | non-empty; the command whose output supports the finding, or the artifact and line that was read |

`run` means the reviewer executed something and the finding rests on its output. `read` means the
finding rests on text or code the reviewer opened, named by path and line. The ground phase records
a wider `tier` vocabulary per row; this field keeps the two values that were by far the most used
there and the only distinction the measured cycles turned on: whether anything was executed. The
counts are under *The corpus carries no evidence key* in the measurements file.

A row missing either key, or carrying an `evidence_kind` outside the pair, aborts the record at exit
1, naming every offending line at once, and no round file is written — the same refusal shape
`tp audit --record` applies to its category enum. `--merge` passes both keys through unchanged and
does not require them; requiring them at merge would let a role's whole file be dropped silently,
which is the failure `--merge`'s own exit-1 rule exists to prevent.

The emitted role prompt names both keys in its output format and states the rule in one sentence:
*a finding without evidence is not recorded*. The exact wording is REFERENCE material; what the
tests pin is that the prompt names the keys and that the record refuses their absence.

## 3. `routed` — the disposition that goes forward

`tp review <recorded round file> --resolve <index> routed "<task id>"` marks a finding as owned by a
task. The row's `resolved` object carries `status: routed` and `evidence: <task id>`, with the same
`resolved_at` the other dispositions write. `--resolve-all … routed "<task id>"` routes every
surviving finding to one task, which is the form a decomposition step uses.

Three rules, each with a reason:

- **A routed finding is subtracted from the surviving set**, so a round whose only open findings are
  routed is clean under either `review_converge_on` value. The finding has not disappeared; it has a
  named owner and a phase that will run it.
- **The task id is required.** A routing that names nothing is a `wontfix` without evidence, and that
  is already refused.
- **A row that arrives at `--record` already marked `routed` is refused**, as a pre-resolved `fixed`
  is. Routing is the orchestrator's act, taken with the task list in hand; a role does not know the
  tasks and cannot take it.

`tp validate` gains one warning, `routed-finding-unowned`: for every recorded review round of the
active spec, a row whose `resolved.status` is `routed` and whose `resolved.evidence` matches no task
id in the task file. It is a warning and not an error because the task file may legitimately be
mid-edit; it is not gated because nothing in this release gates on a finding's destination. The
warning names the round file and the row index so the operator can route it again.

`tp resume`'s `unresolved_findings` and `next_action` treat routed rows as resolved, because they are.

## 4. What ships in `skills/tp/SKILL.md`

The rules are the release as much as the code is. They extend Step 0.5 and Step 2 of the shipped
file and repeat nothing already there; the survey sources behind each are in the measurements file.

- **A sentence that would change if the implementation changed is not spec.** Apply it per sentence
  while writing. What survives it is a decision, a rationale, a non-goal, or an acceptance row whose
  subject is observable from outside. A function name, a package, a derivation path or an exit-code
  branch for behaviour the release will create fails it and moves to the implementing task, which
  names it after the test is green.
- **The reviewed document is a decision record; behaviour is verified by executing, not by reading.**
  The review loop's subject is whether the decisions are right and consistent. Whether the behaviour
  is right is the audit's subject, on code. A review finding that says *the code will not do X* is
  routed to the task that builds X, not repaired in prose.
- **A finding leaves a round in one of three ways**: the spec changes, a disposition records why the
  text is already right, or the finding is routed to a task. The third is not a way to hide a
  finding — `tp validate` names a routed finding that no task owns.
- **A repair subtracts before it adds.** The safest output of a repair is fewer claims. A repair that
  writes a new sentence about behaviour in place of the one it removed has written a claim nobody has
  run; if it must be written, the repair unit runs it before committing. This is the existing
  runtime-sentence rule at its point of application, and the cycle that shipped that rule violated it
  in four consecutive repairs.
- **A review round ends at its cap by design, not only by failure.** The surveyed tools gate on a
  fixed number of approvals. When `review_max_rounds` is reached and the open findings are about
  behaviour the release will create, the next step is to route them and decompose; the operator's
  `--force` at import is the acknowledgement that the review ended on decisions, and the audit is
  the verification round that follows, on code.

## 5. Non-Goals

- **tp does not verify evidence.** It records that a reviewer named a run or a read; it does not
  re-run the command or open the file. That is the *evidence contract* in `spec/undecided.md` and it
  has no design yet.
- **No `kind` taxonomy on findings**, for the reason under *Alternatives considered*.
- **No change to what `tp import` accepts at a cap stop.** `--force` stays the operator's decision;
  this release changes what the operator can say about the findings before taking it.
- **No lexical lint for implementation detail in spec prose.** Recorded under *Undecided* with a
  prediction; prototyped before any spec names it, as this repository's rule requires.
- **No change to the audit phase.** Audit rows already carry `evidence_file` and `evidence_lines`.
  The audit-side sibling of decision 2 — an accepted audit finding entering the round's `clean`
  verdict — is the first backlog file, formerly `spec/1.0.2.md`, and stays there.
- **No change to ground.** Ground rows already carry `tier` and `evidence`; this release borrows the
  distinction, not the mechanism.
- **The survey's other findings are routed, not taken.** The panel-record, the checklist cap, the
  emit-time hash and the brief's forced sentences each have a backlog file; the survey changed none of
  their order.

## 6. Tests

Each row names its fixture, its mutant, and what the mutant changes. A row whose mutant cannot make it
red is not in this table; two counts per row are filled by the implementing task under Step 0.5's
rule, since the subject does not exist at `HEAD`.

| # | WHEN | tp SHALL | fixture | mutant |
|---|---|---|---|---|
| 1 | a findings file handed to `tp review <spec> --record` holds a row with no `evidence` key | exit 1 naming the line, write no round file, leave `review_rounds` unchanged | one-row file, the row otherwise valid | a record path that skips the evidence check: exit 0 and one recorded round |
| 2 | a row carries `evidence_kind` outside `run`/`read` | exit 1 naming the line and listing the two legal values | one-row file with `evidence_kind: guess` | a check that tests presence but not the enum: exit 0 |
| 3 | every row carries both keys legally | record the round as today | two-row file, one `run`, one `read` | — this row pins that 1 and 2 did not over-refuse; its mutant is any refusal on a legal file |
| 4 | `tp review --merge` reads inputs whose rows lack `evidence` | merge and exit 0, rows passed through | two input files without the keys | a merge that applies the record rule: exit 1 |
| 5 | `--resolve <idx> routed "<task>"` is applied to a recorded round file | the row carries `resolved.status: routed` and `resolved.evidence: <task>`, and `--status` reports `consecutive_clean` one higher than before when no other finding survives | a one-finding recorded round | routed not subtracted from the surviving set: `consecutive_clean` unchanged |
| 6 | `--resolve <idx> routed` is applied with an empty task id | exit 2 naming the usage form | same round | a resolve path that accepts an empty evidence for `routed`: exit 0 |
| 7 | a row arrives at `--record` already carrying `resolved.status: routed` | exit 1 with the same hint shape as a pre-resolved `fixed` | one-row file | a record that treats `routed` like `wontfix` at arrival: exit 0 |
| 8 | `tp validate` runs with a recorded round holding a routed finding whose task id is in no task | one `routed-finding-unowned` warning naming the round file and index | task file with one task, round with a finding routed to another id | a validate that does not read recorded rounds: zero warnings |
| 9 | the same, with the task id present | zero `routed-finding-unowned` warnings | same, id corrected | a validate that warns on every routed finding: one warning |
| 10 | `tp review <spec> --status` reads a round recorded before this release, whose rows carry no evidence keys | exit 0 (or 1 under `--check`, as before) and the same `consecutive_clean` as the previous binary reports | every `spec/*.md` in this repository with recorded review rounds, both binaries | a status path that applies the record-time check on load: exit 1 on the corpus |
| 11 | a review role prompt is emitted | the prompt's output-format section names `evidence` and `evidence_kind` | any spec, `--role implementer` | an emission that drops the keys from the format: absent |
| 12 | `tp resume` reads a spec whose open findings are all routed | `unresolved_findings` counts none of them | the round from row 5 | resume counting routed as open: one |

Row 10 is the corpus test and the one that protects the record: the corpus under `spec/.tp-review/`
and `spec/backlog/.tp-review/` holds no `evidence` key on any review row today, and the derivation is
the command in the measurements file under *The corpus carries no evidence key*.
