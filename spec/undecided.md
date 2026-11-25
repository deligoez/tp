# Undecided — and refuted

> **2026-09-11:** most `spec/backlog/` files cited below were archived. `spec/backlog/ARCHIVE.md`
> gives the sha to read each at and what became of it; open defects are in `BUGS.md`.

**Not a release, and not a spec.** Three kinds of thing live here, and the distinction is the point:

- **Refuted** — a candidate that was prototyped and did not survive. It is recorded so it is not
  re-proposed, with the measurement that killed it. **A refuted predicate is not a backlog item.**
- **Parked** — a decision was taken and the work waits for a named trigger. Each row names the
  trigger, and the decision note in `spec/backlog/` when one carries it.
- **Decided — closed** — a decision was taken and nothing carries it forward, because the answer is
  *no*, *not yet worth it*, or *superseded*. Each names why, and the condition that would reopen it.

**The 2026-09-08 decision pass.** Every entry that stood under *Undecided* and *Survived, unscheduled*
was decided that day, the last of them (*Cross-repo specs*) by the operator. On 2026-11-14 the
backlog notes that carried those decisions were archived, and each decision was either parked or
closed here.

Writing a release spec for something whose design has no answer is the failure this file exists to
prevent. An entry here is not a draft of a spec and must not be read as one.

**No item carries a version number.** A number that moves leaves stale references and this repository
has paid for that repeatedly; ordering belongs to `BUGS.md` and the notes in `spec/backlog/`.

**This file is a register, not a forensics document.** Every entry is three things and nothing else:
the decision or the refutation, the reopen condition or the settling bar, and a pointer to where the
measurement lives — **`spec/undecided-measurements.md` §<entry title>** unless the entry names a
backlog sidecar or a commit instead. Nothing was deleted when the forensics moved: a figure that is
not below is in the sidecar, with its counting rule. A figure that moves with the tree is a fenced
derivation command; a literal is anchored to a commit or a tag, `13bfde30` unless stated.

---

## Refuted

### Six requirements-smell lint rules — the whole adjacent family

**All six are refuted, and the reason is a property of the corpus rather than of the patterns.** These
specs are *forensic documents*, so their superlatives, pronouns, percentages and non-verifiable
adjectives live in **evidence prose, not in requirements**, and no line-scoped pattern separates a
claim about a measurement from an acceptance property. `vague-language` survives here because its
eight words happen to be rare in that register; every neighbouring family is not. Each candidate is
refuted by its own measurement, phrased so it cannot be re-proposed without new evidence:

| candidate | what killed it |
|---|---|
| **loophole** (`as appropriate`, `if necessary`, …) | With deferral contexts excluded, the yield over the whole corpus is **one** hit, and that one is resolved by the following sentence. A 24-form wide sweep found no additional loophole forms. Not re-proposable without exhibiting a loophole that qualifies a live requirement here — none exists. |
| **superlative / comparative** | 104 hits after requiring no named baseline; only **6** sit on a line carrying a normative modal, and all 6 are false. The rest are the forensic register. Not re-proposable without a discriminator separating a *measured* superlative from a *required* one. |
| **non-verifiable** (`efficient`, `robust`, `clean`, …) | 388 hits, **256 of them `clean`** — a defined technical term here (`clean_rounds`, `consecutive_clean`, *a clean tree*). The narrowing to normative clauses was built and run: **5 hits, 5 false**, two of them matching the tail of `self-sufficient`. Not re-proposable: the narrowing exists and returns zero true positives. |
| **vague pronoun** | 377 hits, every one read unambiguous anaphora. The rule detects *pronouns*, not *ambiguity* — it carries no antecedent model, so it cannot test the claim in its own name. Not re-proposable without an antecedent resolver. |
| **open-ended enumeration** (`and more`, `such as`) | All five `and more` hits are comparative conjunctions (`and more narrowly`, `and more strictly`), firing on the correct line for a mechanism that is not the smell. |
| **percentage with no denominator** | The derivation lives at *section* scope while the rule is *line*-scoped, so it flags table rows whose deriving command sits above them, and every figure in the file that exists to derive them. Not re-proposable at line scope. |

**The one condition under which this reopens, and it has now half fired.** These rules were measured
against specs written in the style this repository is leaving behind; the reopen condition was that
the sidecar rule be applied to a stated number of specs and the same six candidates re-run against the
same corpus. **The trigger fired on 2026-09-08**: every file under `spec/backlog/` was cut to its
decisions with the forensics moved to a `<slug>-measurements.md` beside it (`spec/backlog/README.md`
records the pass). **The re-run has not happened.** Until it does this stays refuted — the trigger
condition is not the result.

**Where the measurements are.** `spec/undecided-measurements.md` §Six requirements-smell lint rules —
the whole adjacent family carries the corpus size, the prediction scorecard, and the `vague-language`
fence defect the prototyping surfaced (which shipped, and is pinned by
`internal/engine/vague_fence_test.go`).

### The unexecutable-split lint rule

**The rule it tried to mechanize is sound:** *a change and the test it invalidates belong to the same
task.* A graph that puts a change in one task and the test it breaks in the next validates cleanly
and cannot be executed — the gate runs at the earlier close and that close is already red.

**The predicate is refuted.** The proposed shape was *a task whose only work is tests, depending on
exactly one task, where both anchor to the same spec section.* The broad form fires on roughly a
quarter of this repository's tasks, unusable at any bar; the `tags: [test]` narrowing fires on a
handful, and the one hit that has been classified is a false positive.

**Counting rule, over every `spec/*.tasks.json`** — for each task `t` with exactly one entry in
`depends_on` that names a task in the same file, `t` is a candidate when
`set(t.source_sections) & set(parent.source_sections)` is non-empty:

```bash
python3 - <<'PY'
import json,glob
tot=cand=tagform=0
for f in sorted(glob.glob("spec/*.tasks.json")):
    ts=json.load(open(f)).get("tasks",[]); byid={t["id"]:t for t in ts}
    for t in ts:
        tot+=1
        d=t.get("depends_on") or []
        if len(d)==1 and d[0] in byid and \
           set(t.get("source_sections") or []) & set(byid[d[0]].get("source_sections") or []):
            cand+=1
            if "test" in (t.get("tags") or []): tagform+=1
print("tasks %d  candidates %d  and tags contains test %d" % (tot,cand,tagform))
PY
```

**Two structural reasons it cannot be repaired by tightening:**

1. **`tags` is optional and sparse** — present on well under half the corpus, `test` on a fraction of
   that. A rule whose trigger depends on whether the decomposing agent happened to tag is not a
   check; it fires arbitrarily and its silence means nothing.
   `python3 -c 'import json,glob;t=[x for f in glob.glob("spec/*.tasks.json") for x in json.load(open(f)).get("tasks",[])];print(len(t),"tasks,",sum(1 for x in t if x.get("tags")),"tagged,",sum(1 for x in t if "test" in (x.get("tags") or [])),"tagged test")'`
2. **Every stronger signal is post-hoc.** `commit_files` would identify a test-only task precisely,
   and it exists only after the task closes — `tp validate` runs at decomposition.

**What could revive it:** a corpus from a project that does *not* follow the rule, or a required
task-kind field that makes "test-only" a fact rather than an inference. Neither exists today.

**Where the measurements are.** `spec/undecided-measurements.md` §The unexecutable-split lint rule —
the dropped middle row of the original table, the classification of the one tag-form hit, and the
confound that tp's own decomposer already obeys the rule.

### The contradictory-comparator lint rule

**What it tried to catch:** a spec stating one numeric rule twice with opposite comparators — *more
than three sections* in one place, *at most three sections* in another. The proxy grouped statements
by `(number, noun)` and flagged a group carrying both a greater-family and a lesser-family
comparator.

**Its claimed true positive could not be reproduced, and its false positives are systematic.** The
prototype is committed nowhere, so its flag counts cannot be re-derived at all.

**The false-positive instance is checkable and still stands.** In `spec/0.31.0.md`,
`mechanize_candidates` fires on *"a class appearing in at least two rounds"* while `harness_stale` is
false with *"fewer than two rounds"* — two different rules sharing a number and a noun, which is
exactly what the candidate's own text said the design work owed: *"a correct predicate has to
establish that the two statements govern the same subject."* No predicate was named, and none is
available at lint time.

**The obvious narrowing removes the false positives by removing everything**: grouping within a
section instead of a file flagged zero across the whole corpus, and a rule that has never fired on
anything cannot be shown to work.

**What could revive it:** a reproducible instance. The candidate asserted it *"caught the defect that
motivated it"*; that defect is in no tagged state of the repository, so either it lived only in an
uncommitted draft or the prototype differed from the one described. Either way the claim is not
checkable, and a lint rule whose only evidence is unreproducible is not a release.

**Where the measurements are.** `spec/undecided-measurements.md` §The contradictory-comparator lint
rule — the relayed flag counts marked as relayed, and the corpus-size derivation that shows why a
bare "spec files today" figure rots.

### The example-table lint rule

**Refuted in both forms its spec named.** The keyword-and-shape heuristic fires on a double-digit
share of this repository's spec sections against a shipped bar of *zero* false positives; and the
narrower form reads task acceptance criteria, **data that does not exist when `tp lint` runs, since
lint is workflow step 2 and decomposition is step 5**. The second half is structural, needs no corpus
to check, and is what makes this refuted rather than untested. The firing rates are carried as relayed
and must not be re-quoted as measurements.

**It reopens** only if `tp lint` ever runs after decomposition, which would give the narrow form its
input; or if the shape heuristic is prototyped against this repository's own `spec/*.md` and returns
zero false positives at warning severity.

**Where the measurements are.** `spec/undecided-measurements.md` §The example-table lint rule.

### The identifier pass

**What it was.** A `tp lint` pass over a spec's inline code spans. An identifier is an
environment-variable-shaped token, a path-like token, or a `--flag`, extracted from *within* a span.
A span is an **introduction** in a table's first column, in a heading, or on the left of an `is`/`are`
clause; every other appearance is a **reference**. A reference with no introduction in the file
reports `unintroduced-identifier`; an introduction with no reference reports
`unreferenced-identifier`.

**Why it is here rather than under Undecided.** Its own spec made *"neither pass produces a finding
on any shipped spec"* a precondition for shipping, while its own Non-Goal forbade resolving an
identifier introduced in one spec and referenced in another. Those two cannot both hold here:
reimplemented from its own specification and run over `git archive v0.37.0`, the pass fires on **45 of
that tag's 46 top-level `spec/*.md` files**. A candidate whose gate can never pass is refuted.

**What could revive it:** a known-identifier set seeded from tp's own surface — its flags, its
environment variables, its real paths — so that a reference resolving outside the file is not a
finding. That is a different predicate rather than a tightening of this one, it has never been
prototyped, and it would have to answer where the set comes from and what keeps it current. Until it
is built and run, nothing here is a backlog item.

**Where the measurements are.** `spec/undecided-measurements.md` §The identifier pass — the full
counting rule, both corpus rows, the reconciliation of the relayed 979, and the corrected resolution
share.

### The corpus-replay gate, as a procedure

A gate phrased *"replay the recorded rounds in `spec/.tp-review/`"* cannot run: a recorded round's
snapshot is written at emission and its `spec_hash` is re-read at record, so a fraction of the corpus
is not the text its round reviewed, and a replay compares against the wrong spec and reports clean.

**The defect is bounded rather than endemic.** At `13bfde30` the divergent review rounds sit in six
cycles, `0.31.0` carrying more than half, and no cycle after `0.35.0` carries one. So the honest
argument is not *"a quarter of the corpus is wrong"* but **"the record cannot distinguish a corrupt
round from a clean one, so a low rate and a high rate look identical to a reader"** — which survives
the rate falling to zero. Quote the count, not the percentage.

**So the honest form is narrower than a withdrawal.** A replay *cannot gate the release that
introduces the mechanism it would test*, because it must wait for rounds to accumulate. The
replacement that works is a gate stated as an invariant checkable against a live tree — which is what
pinning the round's hash at emission makes structurally possible.

**One thing three specs stated wrong stays corrected here**: they withdrew the gate citing a missing
`disposition`, and the disposition exists. Counting rule — every `resolved` object across both
round-directory globs:

```bash
python3 -c 'import json,glob,collections;c=collections.Counter(json.loads(l)["resolved"]["status"] for p in ("spec/.tp-review/*/*.ndjson","spec/backlog/.tp-review/*/*.ndjson") for f in glob.glob(p) for l in open(f) if l.strip() and isinstance(json.loads(l).get("resolved"),dict));print(c, sum(c.values()))'
```

**It settles** when the emit-time hash ships — `spec/backlog/round-records-the-text-it-read.md` §2,
folded into `spec/backlog/reconcile.md` on 2026-09-11 — after which a replay of any round recorded
from then on is well-defined. That spec does not repair the divergent rounds
already recorded, so a replay over the historical corpus stays undefined whatever ships.

**Where the measurements are.** `spec/undecided-measurements.md` §The corpus-replay gate, as a
procedure — the snapshot-hash derivation with its per-cycle breakdown, and the disposition counts at
three successive readings.

### `broken-cross-ref` extended across files

The obvious companion to the one lint rule that survived prototyping — flag `spec/X.md §Y` where `X`
has no `§Y` — was prototyped and **its marginal yield over the cheaper rule is zero.**

**The counting rule, and the one row that decides it.** Every `.md` file in the tree, snapshots
included; a hit is one occurrence of `spec/<version>.md` followed within 80 characters by `§<number>`
on a line outside a fenced block; a hit is *broken* when the target file exists and no heading in it
carries that number as its first token. Measured at `27f84468`:

| | |
|---|---|
| broken with the target present, target ≤ referrer — the only case the cheaper rule cannot see | **0** |

**It is not refuted in principle** — a mistyped section reference into a shipped spec would be its
alone. There is simply no instance of one, and a rule with no demonstrated independent case does not
ship.

**It reopens** on a single demonstrated instance: one broken `§` reference whose target spec is at or
below the referrer's number, outside `spec/.tp-review/`. Until one exists the rule has no independent
population.

**Where the measurements are.** `spec/undecided-measurements.md` §`broken-cross-ref` extended across
files — the full six-row table, the reconciliation of the original's 312 broken refs, and why
`spec/.tp-review/` would have to be excluded.

### `floor_by_section` — an assertion no mutant of its own field can fail

**What was tried.** `tp lint` was to report `floor_size` grouped by the floor's anchor, so an author
could see which section a round's grading cost sits in. Drafted into `spec/1.0.1.md` §2 across three
rounds; dropped in review round 1 and confirmed dead in round 2.

**Why it died.** Its only assertion was that the values sum to `floor_size`, and that sum is
**invariant under every possible anchor misassignment** — so a mutant that assigns every section the
wrong count and loses a section key entirely passes it. Separately it was the only one of the four
candidate fields unbounded in output size, and the only one whose row named no decision it would feed.

**The six anchor defects it kept trying to describe are real and are now owned elsewhere**, with
fixtures, as one test task of `spec/backlog/the-floor-names-what-it-cut.md` (folded there on
2026-09-11 from `spec/backlog/floor-anchors-need-fixtures.md` §2, itself until 2026-09-08 a section
of `spec/backlog/ground-command-friction.md`). That is the useful residue: the field was
an attempt to publish a quantity whose keys nobody had checked, and checking them is the actual work.

**It reopens** when `engine.FloorAnchorOf`'s anchors are pinned by fixtures rather than by arithmetic,
and when an assertion exists that a wrong grouping can fail — a per-key expectation, not a total.

**Where the measurements are.** `spec/undecided-measurements.md` §`floor_by_section` — the mutant's
key counts and their equal sums, and the output-size figures.

### `spec_bytes` — a lint field whose only use was a product tp does not support

**What was tried.** `tp lint` was to report the spec's byte size beside `review_panel`, so that a
reader could multiply the two and get what one review round reads. Designed and drafted into
`spec/1.0.1.md` §2; refuted in that spec's third grounding round before any code was written.

**Why it died.** tp's role prompts **name the spec path and do not inline the spec**, so the product
measures nothing tp actually sends. Measured against the real prompt bytes for a round-1 emission:
on a five-line probe spec the product under-states the read by roughly forty times, and on
`spec/1.0.1.md` it over-states it by roughly three. Wrong in both directions and by different
factors, which is worse than wrong by a constant — a reader cannot calibrate it away.

**It reopens** if tp ever inlines spec text into a role prompt, at which point the product becomes
the right arithmetic rather than a coincidence. Nothing in the current emitter does.

### `floor_figure_share` — a counter that penalised the behaviour its own release exists to encourage

**What was tried.** The share of uncut floor units whose text carries a digit or a code span,
reported by `tp lint` beside `floor_size`. The intent was to say how much of a round's grading cost
is re-derivation of figures, which `spec/1.0.1.md` §4 argues is the least valuable kind of finding.

**Why it died, in two measurements.** The numerator was **two of the three arms of the floor's own
admission predicate** (`inFloor` = digit ∨ code span ∨ measurement verb, `internal/engine/floor.go`),
so the field could only ever report a high number. And the direction is inverted — §4's shipped rule
replaces a figure with a *reference*, references are code spans, and a code span is in the numerator.
**A spec scores higher for obeying the rule.**

**A narrower variant does discriminate, and was still not shipped.** Restricting the numerator to
`floorHasDigit` alone gives a spread roughly twice as wide and the correct direction. It was left out
because discrimination is not the open question: what a figure share *means* rests on one spec's
`kind` table, n = 1, so shipping it would publish a contract for an unmeasured hypothesis — the class
`class_median_rounds` was removed for in the same release.

**It reopens** when a second cycle's `kind`-by-finding-rate table exists, at which point the
narrow-numerator variant is the one to implement, not the two-arm one.

**Where the measurements are.** `spec/undecided-measurements.md` §`floor_figure_share`; the
measurement command itself is in `spec/1.0.1-measurements.md`.

### `forward-spec-ref` — the rule's population vanished with the version numbers

**What was tried.** A lint rule flagging a citation of a spec numbered *above* the citing one — a
forward reference to a release that may be renumbered before it ships. It survived prototyping once
and had a backlog spec; the spec was dropped, not shipped.

**Why it died.** Its population is gone: pending specs are no longer numbered at all, so the rule has
nothing to fire on, and the replacement predicate — *"a backlog spec is named by slug, never by
priority number"* — had no clean positive set, since every citation carried the priority number in the
filename it cited. `scripts/forward-spec-ref-prototype.py` is referenced by no live file; three frozen
round files under `spec/backlog/.tp-review/10-forward-spec-ref-lint/` still name it.

**It reopens as a different rule, and only that way: a dead-path check** — a `spec/…md` citation whose
target does not exist — which is exactly what the dropped spec's §4.2 forbade. That is the check the
corpus actually needs, and the condition on it is this repository's standing one: prototype it against
`spec/*.md` first, zero false positives at warning severity, before it reaches a spec.

**Where the measurements are.** `spec/undecided-measurements.md` §`forward-spec-ref` — the dead-path
population measured before the rename cleanup, with its counting rule.

### A single whitespace set for the floor — corpus-free

**What was tried.** `spec/1.0.0.md` §2.1 names no whitespace set, and `internal/engine/floor.go`
implements its five steps with three predicates that disagree on exactly one byte, U+000B: RE2 `\s`
at the collapse, `isFloorSpaceByte` at the split and `unicode.IsSpace` at the trim. A draft release
(`spec/backlog/what-the-carry-can-promise.md`, its former second decision) named the six ASCII
whitespace bytes as the one set at every site.

**Why it died.** Zero floor units in the corpus carry a byte the predicates disagree on, and of ten
single-byte inputs run under both binaries the change moves hashes only on an interior U+000B. A
behaviour change that moves nothing in the corpus is not worth a loop-class cycle.

**It reopens** if a floor unit in the corpus is found to carry one of those bytes, or if a second
implementation of §2.1 in another language disagrees with tp on a `text_sha`.

**Where the measurements are.** `spec/backlog/what-the-carry-can-promise-measurements.md` — the byte
enumeration, the fixtures and the withdrawn test rows.

### An `as_of` field and a "today" lint rule for measurement sentences

**What was tried.** A field report (WB-3155, 2026-09-11) asked for an `as_of` field on corpus claims and
a `tp lint` rule warning on "today"/"now"/"bugün" in a measurement sentence, because a count over a
growing population turned its whole battery of rows into `true-when-written` PARTIALs. The rule was
prototyped over `spec/*.md` the same day.

**Why it died.** Narrowed to "today" beside a digit outside tables, the rule fired on sentences of which
a sample judged one by one held a single count over a growing set; the rest were motivation sentences
("Today X does Y") and test conditions ("passes today") — roughly one hit in ten. The field's own
remedy already exists twice: a `true-when-written` PARTIAL must carry `held_at`, and Step 0.5's *"a
number does not live in a spec"* moves the count out of the graded body. The corpus holds no Turkish
spec to test "bugün" against.

**It reopens** if a narrower predicate reaches zero false positives at warning severity on `spec/*.md`.

**Where the measurements are.** `spec/undecided-measurements.md` §An `as_of` field and a "today" lint
rule — the counts at each narrowing and how the sample was judged.

---

## Parked — decided, waiting for a trigger

**Parked means the decision is taken and the work waits for a named trigger.** Each row names the
trigger, the measurement pointer and, when there is one, the note that carries the decision.

| entry | decision | trigger | measurements |
|---|---|---|---|
| A converged payload says what it left open | an audit-side `nonblocking_open`, a count of accepted blocking rows, and audit rows written with review severity words; `spec/backlog/converged-payload-open.md` | the first project that sets `audit_converge_on: blocking` (`review_converge_on` already defaults to `blocking`) | `spec/undecided-measurements.md` §From the rows spec |

---

## Decided — closed

**Closed means no pending spec carries it and none is expected to.** Each entry names why, and the
condition that would reopen it. A reopen condition is a measurement, not an opinion.

### The parallel field report's deferred items

**Closed on 2026-11-18, while v1.24.0 was cut.** A field report from a requesting project, which drove a
spec through tp with up to six implementers at once, made nine proposals. Two already existed
(`tp run`'s implement batches and `touches`, v1.19.0 to v1.21.0), and v1.24.0 takes the gaps. Four
items are deferred here, each with a measurement that would reopen it. Its reporter could not be
reached for follow-up, so no loss attributed to these four was measured.

- **Moving wip and claim state into `.tp/local.json`.** *Refused.* `.tp/local.json` is per machine
  and git-ignored, so another machine or session would not see a task in progress, and `tp resume`
  would stop being the single source between resets. A claim exists precisely so that concurrent
  agents see each other. **It reopens** if a measured cost shows conflicts on the task file between
  trees under `tp run`'s own batch flow, which claims and closes only on the main branch.
- **Folding a close into the task's last code commit.** *Already there, per commit strategy.*
  Under `builtin`, `tp commit` and `tp done --auto-commit` amend the closure into the code commit
  and record the pre-amend sha. A separate `chore(tp): record <id> closure` follows only when the
  amend is unsafe (REFERENCE.md, "Close checkpoint"; `internal/cli/commit.go`,
  `followUpClosureCommitOrExit`). Under `hc`, `tp done` makes no commit: the closed task file is
  bookkeeping, and the agent's own next `hc` plan can carry it. A one-commit-per-close history
  under `hc` is therefore the driver's choice, not tp's. **It reopens** on a measured cost of
  the close commits themselves. A read-only count of the requesting project's log for that night found 89 `chore(tp)`
  commits out of 494, about 25 claims and 53 closes. No cost beyond the count was measured.
- **A per-project brief preamble.** *Deferred.* Text prepended to every brief costs tokens on every
  unit (P3). The report names three rule slips but not what they cost, nor whether the rules were in
  the project's own `CLAUDE.md`, which every agent already reads. **It reopens** on a measured slip
  whose rule was written in the project's instructions and still missed by a unit.
- **An inbox for findings seen during implement (`tp note`).** *Closed by `v1.31.0`.* Found tasks
  replace it: a unit files what it ran into with `tp found add`, and what no unit can settle waits
  in the spec's committed queue for the person (`spec/1.31.0.md` §6).

### The divisible round

**Closed by `v1.22.0`.** The key reopened on 2026-11-14 because half of the code roles' findings sit
beyond the ten-file cut and are real, and yield does not follow churn (`spec/1.21.0-measurements.md`
§The audit's ten-file cut). The next step it named, one constructed audit round with the full list,
ran in `v1.21.1`: a role given a long list in one prompt graded every file by pattern-scanning and
missed most of what ten-file prompts found on the same files (`spec/1.21.1-measurements.md`). So a
whole pool is not graded in one prompt, and the answer taken was the other branch the row named, a
code-lens pass per task at its close: `v1.22.0`'s task audit, each closed task's own files in batches
of at most ten (`spec/1.22.0-measurements.md`). The release audit keeps its ten-file cut.

**It reopens** if a release audit's code role, after every owed task audit is recorded, files a
confirmed finding on a file a task audit graded `PASS` at a commit the file has not changed since.
### The identifier set behind class families

**Closed by a yield bar.** A normaliser for the identifier set behind class families is worth building
only if **at least five percent** of a cycle's findings cluster into families. The relayed figures —
0 to 8.7% of findings, median 0.5% over six cycles, one family of three findings on `v0.37.0` — clear
that bar in at most one cycle out of six.

**It reopens** if a recorded cycle shows the share above five percent, measured with a rebuilt
normaliser. The normaliser is in no committed file: a search for its name across the whole tree
returns nothing, so the relayed grouping figures are carried as relayed and are not measurements
anyone can reproduce today.

**Only the denominator re-derives.** `v0.37.0`'s review rounds hold **630** recorded rows in total
(counting rule: non-blank lines across `spec/.tp-review/0.37.0/review-round-*.ndjson`), which matches
the original.

### The evidence contract

**Closed by `v1.1.0`.** That release shipped the carrier and **cut `evidence_kind`**, for the reason
the *a declared evidence mode measures nothing* section gave. The reader of a stored `evidence` is
`buildVerifyPrompt` (`internal/cli/review_verify.go`), verify mode — the one site that re-verifies a
disposition. The other two injection sites do not read it **by design**: `buildFindingsSummary`
(`review.go`) suppresses, `buildRegressionPrompt` (`review_regression.go`) regresses, and neither is
re-verifying anything. The remaining `Open:` headings of the `a4b70c3e` draft are answered by that cut.

**It reopens** if a second reader is proposed — a site that would print a stored `evidence` under that
name, with a named bound and label.

The draft is reachable as `git show a4b70c3e:spec/0.49.0.md` (that number now belongs to a different
subject entirely, which is why the commit is the citation). Counting rule: headings whose text begins
`Open:`. There are **five**, not the four recorded — *the generator cannot author an experiment*,
*`UNVERIFIED` has no legal place in the loop*, *a declared evidence mode measures nothing*, *closure
evidence has no per-line carrier*, and *what would gate this*. Its two ready pieces were lifted out
long ago and are releases of their own: the forced-commitment brief, and mutation score as a
documented gate entry.

**Where the measurements are.** `spec/1.1.0-measurements.md`, *`evidence` is write-only: the three
injection sites, and where the reading half went*, carries the caps and the runs;
`spec/undecided-measurements.md` §The evidence contract summarises which site labels what.

### `frontmatter-key-namespace` — a fixture whose frontmatter configures nothing

**Closed as not a rule.** Predicate 1 — the key is outside the `tp:` mapping, so tp reads nothing —
has three instances and all three are repaired at `c00266a0`
(`internal/cli/lint_review_panel_test.go`, `internal/cli/role_panel_split_test.go`,
`internal/engine/rolepanel_test.go`). Predicate 2 — the key is live but its value is the parser's
default — stands at **n=1**, and one instance is a bug report, not a rule. The one-time sweep of the
other eighteen Go files is a chore, not a check.

**It reopens** if that sweep finds a second predicate-2 instance.

The corpus is Go string literals, not markdown: swept at `27f84468`, **78** lines across **19** Go
files carry a literal `\n---`, and **zero** `.md` files under `internal/` carry a frontmatter block —
so the subject is test source, in the same family as `scripts/check-test-inventory.py` rather than the
lint table. Predicate 2 also has an obvious false-positive source: a fixture may declare the default
*on purpose*, as the control arm of a pair.

**Where the measurements are.** `spec/undecided-measurements.md` §`frontmatter-key-namespace`;
`spec/1.0.1-measurements.md` §16 for the mutant that survived and the three panels measured.

### A fenced command that runs and prints the wrong thing

**Closed: no predicate compares output to truth.** §4.1 of `spec/1.0.1.md` asks that every fenced
command run and print something, which is liveness rather than truth — the instance that motivated
this ran, exited 0, and printed a full set of numbers, not one of which was a figure the prose beside
it stated. The writing rule in `skills/tp/SKILL.md` **Step 0.5** — figures live in the sidecar as
references — already forbids the literal-beside-derivation shape, so a lint would only restate a rule
the spec-writing step enforces earlier and better.

**It reopens** with a prototype at **zero false positives** over `spec/*.md`. The obvious predicate —
compare each fenced command's output against the figures in the surrounding prose — needs a mapping
from a figure in prose to a position in a command's output, and that mapping exists nowhere.

**Where the measurements are.** `spec/undecided-measurements.md` §A fenced command that runs and
prints the wrong thing.

### The `implementation-detail` lint over spec prose

**Closed.** `spec/1.1.0.md`'s *Alternatives considered* names this rule and does not take it, and it
is not taken here either: the rule *a sentence that changes with implementation is not spec* is
enforced by ground and by the repair rule, not by lint. No predicate was found that separates such a
sentence from the acceptance rows in the same documents, which name commands, fields and exit codes
legitimately — and the *Refuted* entries above are what happens when a candidate cannot make that
separation.

**It reopens** with a prototype at **zero false positives** over `spec/*.md` and over the pre-repair
text of the defect that motivated it. That defect is on record: `v1.0.1`'s cycle spent five grading
rounds refuting five successive sets of sentences about `tp lint` fields that did not exist yet, and
`spec/1.1.0-measurements.md` carries it.

### A prior-round section for `tp review`

**Closed: no.** A re-verification ask and a suppression ask differ in **kind**, not in presence, so
the review phase lacking the audit phase's section is not a gap. Review keeps its panel-wide
suppression list. Routed from `spec/candidates.md`, whose forwarding table points here;
`spec/backlog/repair-locality.md` names the question and explicitly declines to decide it.

**What the two phases do, in one sentence each.** The audit phase hands a role its **own** prior
non-PASS rows and forces a commitment on each — `loadAuditPriorRound` (`internal/cli/audit.go`)
selects them, `renderPriorRoundSection` (`internal/cli/audit_roles.go`) renders them under *"Prior
Round: context to re-check, not a verdict to repeat"*. Review shows the whole panel everyone's rows
and asks for the opposite — `buildFindingsSummary` (`internal/cli/review.go`) emits `UNRESOLVED
findings from previous rounds — DO NOT re-report:`, panel-wide, capped and truncated.

**It reopens** only if the repair-locality measurement — no longer a release since 2026-09-11, a
script whose source is in `spec/backlog/repair-locality-measurements.md` — shows the share of
findings sitting in repaired text **rising** with the suppression list on.

**What the field changed, and what it did not (2026-09-11).** A field report (WB-3155) found the
header lying: accepted `[WONTFIX]` rows sit under *"UNRESOLVED … DO NOT re-report"*. The header is a
defect and `spec/backlog/a-findings-exits-agree.md` takes it; suppressing accepted rows is this
decision and stands. The same report's claim that fixed findings are also suppressed was refuted —
they are listed as resolved with *"DO NOT regress"*, and the regression prompt carries each fix's
evidence. On the audit side, where this entry's contrast lives, the prior-round rows carry no
disposition at all; `spec/backlog/a-finding-can-leave-an-audit-round.md` takes that.

**Where the measurements are.** `spec/undecided-measurements.md` §A prior-round section for
`tp review` — both mechanics in full, and why the repair-locality figures cannot settle it.

### The scope pass — whether a section should exist

**Closed for now: scope is asked at spec-writing**, by the Step 0.5 interview in
`skills/tp/SKILL.md`, not by a pass at the front of the cycle. Ground and review both presuppose the
spec's scope and neither can ask whether a section should exist; the interview asks it before either
runs. Three internal signals were tested and
all three failed — finding density is uninformative in both directions, cross-role agreement is a coin
flip, goal-entailment self-confirms when §1 was written in the same sitting. A forensics trim was the
first proposal and was refuted by a field report that classified its own diffs.

**It reopens** when a signal beats a coin flip on the recorded corpus.

**Where the measurements are.** `spec/undecided-measurements.md` §The scope pass — folded in from the former `spec/trim-pass.md` on 2026-09-08, and not a
backlog spec.

### `scope` on audit rows

**Closed: no field.** The **role already is the scope** — a `spec-coverage` row is spec scope, a
code-lens row is codebase scope. That is the mechanical rule this entry named as its second shape,
derived from the panel rather than from the finding, and it needs no new column and no second
classifier. `spec/backlog/round-knows-its-panel.md` §4a therefore stays **role**-scoped, and
`spec/backlog/a-finding-can-leave-an-audit-round.md` reads an accepted finding's scope off its role
when the acceptance is re-read.

**The objection that deferred it four times is what the role-derived rule answers.** Of the three
shapes named and never costed — a second role classifying independently, a mechanical rule derived
from the checklist item rather than from the finding, or an operator confirmation per label — only the
second avoids the objection: a `scope` a row's own author assigns is a judgement by the very sub-agent
that wrote the row, so one row mislabelled `codebase` lets a genuine spec violation ship, strictly
worse than today's rule of over-counting and wasting rounds. The role is assigned by the panel, not by
the row.

**It reopens** if a role is ever built that files both kinds of row.

Current behaviour is pinned by the row schema in `internal/cli/audit_schema.go`, which declares
`category` and `severity` and no `scope`. The entry was deferred four times, each time on the record:
v0.33.0 Non-Goal 1, `spec/0.34.0-candidates.md` item 1, `spec/0.35.0-candidates.md` item 14, and
`spec/0.37.0.md` §2/§5. One premise it was carried on has since been falsified — it was paired with
`audit_converge_on` on the argument that the knob depends on a scope label, and v0.37.0 shipped that
knob keyed on **`severity`** instead.

### Cross-repo specs

**Closed by the operator on 2026-09-08: a spec does not name tasks in another repository.** Routed
from `spec/0.33.0-candidates.md` item 7, which records that at least one team already works this way —
plan in one repo, run agents in both — and that the file-selection and `commit_shas` paths both assume
one repo root. The cost of the feature is a second repository root threaded through both, and nothing
in this repository's own use exercises it, so prototyping here would measure a corpus of one that does
not need it. The only adjacent statement in the corpus is a non-goal: `spec/0.31.2.md` names
*"Cross-repository task execution"* and defers a per-task working directory and quality gate to a
separate release.

**It reopens** when a field cycle asks for it with a task file that names a second root — a request
rather than a design, because no design pass exists.

**Reconfirmed by the operator on 2026-09-11: not now.** A field report (WB-3155) asked again; the
workaround needs no design — drop the edge (`tp set <id> 'depends_on=[]'`) or split the unit.

### A check library from the field

**Closed for now: no script, no spec.** A field report (WB-3155) proposed a library of ready registered
checks with two of its own scripts as the first candidates, and believed tp had taken them; nothing in
the tree records that. A check is a detector plus its false-positive bill, and neither script is in
hand. **It reopens** when the scripts are contributed and a prototype over `spec/*.md` reaches zero
false positives at warning severity — this repository's standing bar for a check.

**Where the measurements are.** `spec/undecided-measurements.md` §A check library from the field.

### Resetting the clean streak when consecutive rounds read different text

**Closed for now: not worth a cycle.** Parked from `spec/backlog/round-records-the-text-it-read.md`
§3 when that spec folded into `spec/backlog/reconcile.md` on 2026-09-11; its §2 (a round's hash is
the hash of the text it read) went with the fold. The streak reset reproduces in a fixture and has
one instance in the recorded audit corpus, and the review-side mismatch count has not moved since
v0.35.0. **It reopens** when a recorded round after `spec/backlog/emitting-does-not-lose-a-round.md`
ships reads a text its successor did not.

**Where the measurements are.** `spec/undecided-measurements.md` §Resetting the clean streak when
consecutive rounds read different text.

### A registered check that outlives its release

**Closed as superseded.** The 2026-09-08 decision was `{spec}` and `{round}` text substitution in
`checks[].cmd`. v1.20.1 gives every registered check `TP_SPEC`, the reviewed spec's path, in its
environment instead. An inherited `TP_SPEC` or `TP_ROUND` is dropped, and `TP_ROUND` is not set,
because no check reads a round. Registered checks run in the review phase only: emission,
`--status --check`, `--record` and import. On 2026-11-14 tp's own three checks were found reading the
spec from `tp resume`, which always named `spec/1.8.0.md`, so they graded the wrong spec on every
review.

**It reopens** when a registered check needs the round or runs outside the review phase.

**Where the measurements are.** `spec/undecided-measurements.md` §A registered check that outlives
its release.

### Cross-site key agreement in the review prompt

**Closed.** On 2026-11-14 all four review role prompts carried `class` as both optional and mandatory,
and 13 of 469 evidence-era review rows lacked `class`. v1.20.1 deletes the "Optional class" sentence.
The 2026-09-08 decision, rendering the key set at all three sites from one Go constant, is not taken.

**It reopens** on a recorded round where more than one in ten rows lacks a contract key.

**Where the measurements are.** `spec/undecided-measurements.md` §Cross-site key agreement in the
review prompt.

### `NewRootCmd` writes package globals

**Closed.** The 2026-09-08 decision was the fence, not the rewrite; its carrier, `gate-sequence`, was
archived after v1.2.0 (`spec/backlog/ARCHIVE.md`).

**It reopens** when an in-process test observes another test's flag value.

**Where the measurements are.** `spec/undecided-measurements.md` §`NewRootCmd` writes package globals.

### `t.Parallel()` in the engine package

**Closed: `internal/engine` stays serial.** Its carrier, `mutation-run-check`, was dropped
(`spec/backlog/ARCHIVE.md`); the decision needs no carrier.

**It reopens** if the serial engine tests become a measured cost on the gate.

**Where the measurements are.** `spec/undecided-measurements.md` §`t.Parallel()` in the engine
package.

### The grounding floor: a rewritten sentence's exemption, and claim enumeration

**Closed.** Two 2026-09-08 decisions: a sentence rewritten in answer to a finding gets no exemption
from the cut (`tp ground --status` would report a `cut` delta per round instead), and the floor's own
arms define a claim. Their carriers were dropped or not taken (`a-round-can-be-driven-from-the-envelope`,
`the-floor-names-what-it-cut` in `spec/backlog/ARCHIVE.md`).

**It reopens** on a field report that reproduces either case.

**Where the measurements are.** `spec/undecided-measurements.md` §A sentence rewritten in answer to a
finding is exempt from the cut for one round; `the-floor-names-what-it-cut-measurements.md` §5 at the
sha its ARCHIVE row names.

### A durable home for an accepted finding

**Closed as superseded.** The 2026-09-08 decision was a repository-level `.tp/accepted.ndjson`. An
evidenced acceptance now clears the row where it is recorded (`7c4befca`, v1.16.0, v1.18.0), and
`accept-finding` is an operator decision with its own record.

**It reopens** on an operator request for a cross-spec list of accepted findings, served as a read over
existing state, not a new file.

**Where the measurements are.** `spec/undecided-measurements.md` §From the rows spec.

### Which channel a degraded scan reports on

**Closed.** The 2026-09-08 decision was a payload field always, with the notice in addition. Its
carrier, `an-unreadable-file-is-named`, was archived with its reproduced items moved to `BUGS.md`.

**It reopens** when a driver misses a notice because of `--quiet`.

**Where the measurements are.** `two-advisories-measurements.md` at the sha its row in
`spec/backlog/ARCHIVE.md` names.

### The test-file fence and the write-deny fence's reach

**Closed.** The write-deny hook judges the write target since v1.6.1, and a batch op by op since
`fcec0da0`, so reads and batched calls are no longer refused for a path they only name. The
spawn-time test-file permission and `test_globs` were not taken: no reproduction. If `test_globs` is
ever taken, it follows `pickChecks`: a present list replaces the layer beneath it. The role write-allow
hook refusing a fixture write outside the repository was run on 2026-11-14 and refuses (exit 2). That
matches the role prompt's isolation clause, which forbids any write but the output file, so it is not
a defect.

**It reopens** on a reproduced wrong refusal.

**Where the measurements are.** `hooks-fence-the-target` in `spec/backlog/ARCHIVE.md`.

### Inferring a spec's class

**Closed.** The 2026-09-08 decision was that `tp lint` reports a derived `class` beside `floor_size`
and `review_panel`, with no gate and no frontmatter override. No release took it, and no decision has
needed the class since.

**The claim it settled.** A draft release proposed a `class: loop | tool` frontmatter field, declared
by the author, on the stated ground that *tp cannot infer it*. A unit told to construct a
counter-example built three predicates against 23 hand-labelled specs and **refuted the "cannot"**: a
density predicate — loop-lexicon occurrences per thousand words — scores 87% leave-one-out with zero
false positives. The lexicon is a new surface that will drift, which is why the decision gated nothing
on it.

**It reopens** when a decision needs a spec's class as input.

**Where the measurements are.** `spec/undecided-measurements.md` §Inferring a spec's class.

---

## Fog — in scope, not yet sharp enough to state as a question

**The test is whether the question can be stated precisely now, not whether it can be answered now.**
An entry here is coarser than the entries above it: one may graduate into several questions, or none,
once the frontier reaches it. Keeping the two apart stops a half-seen problem from being pre-sliced
into confident-looking entries it does not yet deserve.

- **Whether the emission's prohibitions are worth sweeping.** 14% of the review prompt and 12% of
  `CLAUDE.md` steer by ban. A few of those are hard guardrails that earn it. No measurement separates
  the two, and a rewrite without one is prose churn.
- **No sentence-level "two homes" instrument exists.** The count that scores how much one statement is
  duplicated across two documents reads **headings**, while the duplications it is meant to find live
  in bullets inside sections both documents legitimately keep — so cutting twelve duplicated statements
  moved the figure by one. Routed from `spec/0.35.0-candidates.md` item 10. The number is a floor and a
  weak one; a sentence-level comparison is what actually found the duplicates, and what a future check
  should run. What is not yet statable is the unit it would compare and what makes two sentences the
  same statement.
