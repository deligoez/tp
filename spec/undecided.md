# Undecided — and refuted

**Not a release, and not a spec.** Two kinds of thing live here, and the distinction is the point:

- **Refuted** — a candidate that was prototyped and did not survive. It is recorded so it is not
  re-proposed, with the measurement that killed it. **A refuted predicate is not a backlog item.**
- **Undecided** — a real need whose *design* has no answer yet. Each names the decision nobody has
  taken.

Writing a release spec for something whose design has no answer is the failure this file exists to
prevent. An entry here is not a draft of a spec and must not be read as one.

**No item carries a version number.** A number that moves leaves stale references and this repository
has paid for that repeatedly; the roadmap table in `CLAUDE.md` is the one place the numbers belong.
Name a candidate by its subject.

**Every figure below was re-derived at `HEAD` and states its counting rule.** Where a figure could
not be re-derived, the entry says so instead of repeating it. Measurements are taken at commit
`27f84468` unless the entry names another ref, and where a corpus was moving under the measurement it
was read from `git archive` rather than from the working tree.

---

## Refuted

### Six requirements-smell lint rules — the whole adjacent family

**What was tried.** `vague-language` is one member of a family the requirements literature and
ISO/IEC/IEEE 29148 name: subjective, ambiguous, non-verifiable, vague, superlative, comparative,
loophole, vague pronoun. Six candidates were prototyped against this repository's own corpus — 67
files (`spec/*.md` and `spec/backlog/*.md`), 19,715 prose lines, **1,529 candidate findings judged**.
The bar was the one this repository already applies: zero false positives at warning severity, at
least one true positive, and it must catch the defect that motivates it.

**All six are refuted, and the reason is a property of the corpus rather than of the patterns.** These
specs are *forensic documents*: they argue from measurements, so their superlatives, pronouns,
percentages and non-verifiable adjectives live in **evidence prose, not in requirements**. *"the
strongest finding of that audit round"* is a claim about a measurement, not an acceptance property,
and no line-scoped pattern separates the two. `vague-language` survives here because its eight words
happen to be rare in that register; every neighbouring family is not.

Each is refuted by its own measurement, phrased so it cannot be re-proposed without new evidence:

| candidate | what killed it |
|---|---|
| **loophole** (`as appropriate`, `if necessary`, …) | With deferral contexts excluded, the yield over the whole corpus is **one** hit, and that one is resolved by the following sentence. A 24-form wide sweep found no additional loophole forms. Not re-proposable without exhibiting a loophole that qualifies a live requirement here — none exists. |
| **superlative / comparative** | 104 hits after requiring no named baseline; only **6** sit on a line carrying a normative modal, and all 6 are false. The rest are the forensic register. Not re-proposable without a discriminator separating a *measured* superlative from a *required* one. |
| **non-verifiable** (`efficient`, `robust`, `clean`, …) | 388 hits, **256 of them `clean`** — a defined technical term here (`clean_rounds`, `consecutive_clean`, *a clean tree*). The narrowing to normative clauses was built and run: **5 hits, 5 false**, two of them matching the tail of `self-sufficient`. Not re-proposable: the narrowing exists and returns zero true positives. |
| **vague pronoun** | 377 hits, every one read unambiguous anaphora. The rule detects *pronouns*, not *ambiguity* — it carries no antecedent model, so it cannot test the claim in its own name. Not re-proposable without an antecedent resolver. |
| **open-ended enumeration** (`and more`, `such as`) | All five `and more` hits are comparative conjunctions (`and more narrowly`, `and more strictly`), firing on the correct line for a mechanism that is not the smell. |
| **percentage with no denominator** | The derivation lives at *section* scope while the rule is *line*-scoped, so it flags table rows whose deriving command sits above them, and every figure in the file that exists to derive them. Not re-proposable at line scope. |

**The one condition under which this reopens.** These rules were measured against specs written in the
style this repository is now leaving behind. If the sidecar rule works — forensics move to
`<base>-measurements.md` and the body keeps decisions — the register the rules drown in is no longer in
the body. **Reopen only after the sidecar rule has been applied to a stated number of specs, and then
re-run the same six candidates against the same corpus.** Until that condition is met this stays
refuted; "it might work later" is not a reason to re-propose it.

**Two predictions were on record before the run and the scorecard is worth keeping**, because it is
this repository's own rule applied to a forecast rather than to a role: one clause of four was right in
its verdict, and **that one's stated mechanism was wrong**. `non-verifiable` was predicted to die from
the corpus mixing English and Turkish — measured, the corpus carries Turkish letters on **5 lines in 2
files**, and the apostrophe-suffix trap yields **zero** tokens. It dies for an unrelated reason.
`vague pronoun` was predicted to duplicate `vague-language` — measured, **0 of 369 lines are shared**,
and the same holds for every candidate. Both predictions reached a defensible verdict *through a
mechanism the corpus does not contain*, which is precisely the failure mode this file exists to record.

**One real defect surfaced while prototyping and is not part of this entry** — it went to the release
that was being written at the time: `CheckVagueLanguage` is the only rule in `internal/engine/vague.go`
that does not track fenced code blocks, so a share of its corpus findings fire *inside* fences, on
examples of its own output.

### The unexecutable-split lint rule

**The rule it tried to mechanize is sound:** *a change and the test it invalidates belong to the same
task.* A graph that puts a change in one task and the test it breaks in the next validates cleanly
and cannot be executed — the gate runs at the earlier close and that close is already red.

**The predicate is refuted.** The proposed shape was *a task whose only work is tests, depending on
exactly one task, where both anchor to the same spec section.*

Counting rule, re-derived over every `spec/*.tasks.json` at `HEAD` — for each task `t` with exactly
one entry in `depends_on` that names a task in the same file, `t` is a candidate when
`set(t.source_sections) & set(parent.source_sections)` is non-empty:

| form | fires on | true positives |
|---|---|---|
| depends on exactly 1 task sharing a `source_section` | **128 of 551 (23.2%)** | unusable at that rate |
| …and `tags` contains `test` | **1** | **0** |

The corpus grew since the original run — **551 tasks in 27 task files** at `HEAD` against the 520 in
26 recorded then, and the rate moved from 21.9% to 23.2%. The tag-form hit is the same task.

**The middle row of the original table did not reproduce and is dropped rather than repeated.** It
recorded 11 hits for *"title or acceptance mentions a test"* with roughly two true positives; its
counting rule was never written down, and no reading of that phrase lands near 11. A `test` substring
over title and acceptance gives **77**; a word-boundary `\btests?\b` in the title alone gives **2**,
in the acceptance alone **74**.

**The single tag-form hit is a false positive.** `mechanize-candidates-retained` in
`spec/0.25.0.tasks.json` is tagged `test` and depends on `per-role-overlap-report` in the same
section, but it is not a test-only task — its acceptance is *"keep the existing output exactly as-is,
adding no per-round signal"* and its closure reads *"No production change"*. It is a
negative-requirement task whose deliverable is a guard, and it executed without the gate ever going
red.

**Two structural reasons it cannot be repaired by tightening:**

1. **`tags` is optional and sparse.** Re-derived at `HEAD`: present on **216 of 551 tasks (39%)**,
   with `test` on **15**. A rule whose trigger depends on whether the decomposing agent happened to
   tag is not a check; it fires arbitrarily and its silence means nothing.
2. **Every stronger signal is post-hoc.** `commit_files` would identify a test-only task precisely,
   and it exists only after the task closes — `tp validate` runs at decomposition.

**One confound, stated rather than hidden.** tp's own task files are decomposed by an agent following
`CLAUDE.md`, which already carries the rule — so the absence of true positives may mean the rule is
already obeyed here rather than that the failure does not occur. That weakens *"the failure never
happens"*; it does not weaken *"this predicate finds a false positive and no true one on 551 tasks"*,
which is what a zero-false-positive bar decides on.

**What could revive it:** a corpus from a project that does *not* follow the rule, or a required
task-kind field that makes "test-only" a fact rather than an inference. Neither exists today.

### The contradictory-comparator lint rule

**What it tried to catch:** a spec stating one numeric rule twice with opposite comparators — *more
than three sections* in one place, *at most three sections* in another. The proxy grouped statements
by `(number, noun)` and flagged a group carrying both a greater-family and a lesser-family
comparator.

**Its claimed true positive could not be reproduced, and its false positives are systematic.** The
original run reported 2 flags with 0 true positives over the spec corpus at both the `v0.36.0` and
`v0.37.0` tags — the two states the candidate described as *before this cycle's repairs* — and zero
flags when grouping was narrowed to a single section.

**The prototype is not committed anywhere in the tree, so the flag counts cannot be re-derived.**
What does re-derive is the corpus size — `git ls-tree -r --name-only <tag> spec/` restricted to
top-level `spec/*.md` returns **46** at `v0.36.0` and **46** at `v0.37.0`, against **59** at `HEAD`.
The original's *"47 spec files today"* was true when it was written and is not today, which is the
ordinary fate of a figure with no derivation beside it.

**The false-positive instance is checkable and still stands.** In `spec/0.31.0.md`,
`mechanize_candidates` fires on *"a class appearing in at least two rounds"* while `harness_stale` is
false with *"fewer than two rounds"* — two different rules sharing a number and a noun, which is
exactly what the candidate's own text said the design work owed: *"a correct predicate has to
establish that the two statements govern the same subject."* No predicate was named, and none is
available at lint time.

**The obvious narrowing removes the false positives by removing everything.** Grouping within a
section instead of a file flagged zero across the whole corpus. A rule that has never fired on
anything cannot be shown to work.

**What could revive it:** a reproducible instance. The candidate asserted it *"caught the defect that
motivated it"*; that defect is in no tagged state of the repository, so either it lived only in an
uncommitted draft or the prototype differed from the one described. Either way the claim is not
checkable, and a lint rule whose only evidence is unreproducible is not a release.

### The example-table lint rule

Refuted in both forms its spec named: the keyword-and-shape heuristic fired on **3.9–22.6%** of this
repository's spec sections against a shipped bar of *zero* false positives, and the narrower form
reads task acceptance criteria — data that does not exist when `tp lint` runs, since lint is workflow
step 2 and decomposition is step 5. The second half is structural and needs no corpus to check.

**Neither firing rate is re-derivable and the denominator has moved.** No prototype is committed. The
original's 1,032 sections does not reproduce under the nearest stated rule: heading lines at any
level across `spec/*.md` at `HEAD`, fenced blocks not excluded, count **1,251**. The rates above are
carried as relayed and must not be re-quoted as measurements.

### The identifier pass

**What it was.** A `tp lint` pass over a spec's inline code spans. An identifier is an
environment-variable-shaped token, a path-like token, or a `--flag`, extracted from *within* a span
rather than from spans that consist of one. A span is an **introduction** when it sits in a table's
first column, in a heading, or on the left of an `is`/`are` clause; every other appearance is a
**reference**. A reference with no introduction anywhere in the file reports
`unintroduced-identifier`; an introduction with no reference reports `unreferenced-identifier`.

**Why it is here rather than under Undecided.** Its own spec made *"neither pass produces a finding
on any shipped spec"* a precondition for shipping, while its own Non-Goal forbade resolving an
identifier introduced in one spec and referenced in another. Those two cannot both hold on this
corpus, and the measurement below is what settles it: the pass fires on **58 of 59** shipped specs.
The gate can never pass, and a candidate whose gate can never pass is refuted, not undecided.

**Re-derived, with the counting rule stated.** The pass was reimplemented from its own specification
and run over `git archive HEAD` and `git archive v0.37.0`. Corpus: top-level `spec/*.md` only. Spans:
inline backtick spans outside fenced blocks. One finding per `(file, identifier, kind)`:

| corpus | files | `unintroduced` | `unreferenced` | files with a finding |
|---|---|---|---|---|
| `HEAD` | 59 | **1,425** | 116 | **58** |
| `v0.37.0` | 46 | **997** | 94 | 45 |

The relayed figure was 979 over 46 specs. At the same corpus size this rule gives **997** — 1.8%
apart, which identifies the relayed count as `unintroduced-identifier` alone over the `v0.37.0`
corpus. The figure is reproduced; the conclusion does not turn on which of the two numbers is used.

**The other half of the entry was overstated and is corrected.** *"All references to things that
exist elsewhere in the repository"* is not what the corpus says. Counting rule: an identifier
resolves when its literal text appears in at least one git-tracked file outside `spec/`, or names a
path that exists. **1,260 of 1,425 (88.4%)** resolve at `HEAD`; **850 of 997 (85.3%)** at `v0.37.0`.
The remainder are mostly illustrative paths in the older specs (`docs/foo.md`, `spec-r1.md`) and bare
basenames of files that exist under a directory the spec did not spell.

**What could revive it:** a known-identifier set seeded from tp's own surface — its flags, its
environment variables, its real paths — so that a reference resolving outside the file is not a
finding. That is a different predicate rather than a tightening of this one, it has never been
prototyped, and it would have to answer where the set comes from and what keeps it current. Until it
is built and run, nothing here is a backlog item.

### The corpus-replay gate, as a procedure

A gate phrased *"replay the recorded rounds in `spec/.tp-review/`"* cannot run: a recorded round's
snapshot is written at emission and its `spec_hash` is re-read at record, so a fraction of the corpus
is not the text its round reviewed, and a replay compares against the wrong spec and reports clean.

**Re-derived at `HEAD`.** Counting rule: for every `spec/.tp-review/*/state.json`, compare each
round's `spec_hash` against `sha256` of the snapshot file tp names for that round.

| phase | rounds recorded | with a snapshot | hash mismatch |
|---|---|---|---|
| review | 172 | 172 | **35** |
| audit | 108 | 88 | **3** |

**The defect is bounded rather than endemic, and the distribution is nothing like uniform.** The 35
sit in six cycles — `0.31.0` carries **19** of them, then `0.32.0` 7, `0.24.0` 4, `0.31.2` 2,
`0.35.0` 2, `0.29.0` 1. **No cycle after `0.35.0` carries one**: the 31 review rounds recorded for
`0.36.0`, `0.37.0` and `1.0.0` are all clean.

**Quote the count, not the percentage.** The numerator has not moved while every denominator has, so
the ratio improves without the defect changing. Re-derive it rather than reading a figure here.

**Two things about this were previously stated wrong and stay corrected.** Three specs withdrew the
gate citing a missing disposition; the disposition exists. Counting rule — every `resolved` object
across `spec/.tp-review/*/*.ndjson` — gives **1,566 rows** at `HEAD`, `fixed` **1,467** and `wontfix`
**99** (the entry that carried 1,406 / 1,308 / 98 was right about the shape and is simply older):

```
python3 -c 'import json,glob,collections;c=collections.Counter(json.loads(l)["resolved"]["status"] for f in glob.glob("spec/.tp-review/*/*.ndjson") for l in open(f) if l.strip() and isinstance(json.loads(l).get("resolved"),dict));print(c)'
```

**So the honest form is narrower than a withdrawal.** A replay *cannot gate the release that
introduces the mechanism it would test*, because it must wait for rounds to accumulate. The
replacement that works is a gate stated as an invariant checkable against a live tree — which is what
pinning the round's hash at emission makes structurally possible.

### Refuted alongside `forward-spec-ref`: `broken-cross-ref` extended across files

The obvious companion to the one lint rule that survived prototyping — flag `spec/X.md §Y` where `X`
has no `§Y` — was prototyped and **its marginal yield over `forward-spec-ref` is zero.**

**Re-derived at `HEAD`, with the counting rule stated.** Every `.md` file in the tree, snapshots
included; a hit is one occurrence of `spec/<version>.md` followed within 80 characters by `§<number>`
on a line outside a fenced block; a hit is *broken* when the target file exists and no heading in it
carries that number as its first token:

| | |
|---|---|
| `.md` files scanned | 378 |
| raw hits | 643 |
| of those inside `spec/.tp-review/` snapshots | **603 (93.8%)** |
| target file no longer exists (unresolvable, not broken) | 242 |
| broken with the target present | 5 |
| of those, target ≤ referrer — the only case `forward-spec-ref` cannot see | **0** |

**The absolute counts do not reproduce; the load-bearing one does.** The original recorded 312 broken
refs and 163 of 173 raw hits inside snapshots. Neither is reachable under this rule and the original
rule was not recorded — most of the gap is that the original appears to have counted a missing target
file as broken. What reproduces exactly is the zero: **every broken section reference in the corpus
points at a higher-numbered spec**, so the cheaper rule — which never opens the target file — catches
all of them, and the snapshot share reproduces in shape (93.8% here against 94.2% there).

**It is not refuted in principle** — a mistyped section reference into a shipped spec would be its
alone. There is simply no instance of one, and a rule with no demonstrated independent case does not
ship. It would also have needed `spec/.tp-review/` excluded, since almost every raw hit is inside a
round snapshot: a frozen photograph whose references were correct when it was taken.

### `floor_by_section` — an assertion no mutant of its own field can fail

**What was tried.** `tp lint` was to report `floor_size` grouped by the floor's anchor, so an author
could see which section a round's grading cost sits in. Drafted into `spec/1.0.1.md` §2 across three
rounds; dropped in review round 1 and confirmed dead in round 2.

**Why it died, in three independent measurements.** Its only assertion was that the values sum to
`floor_size`, and that sum is **invariant under every possible anchor misassignment**: each uncut
unit receives exactly one anchor whatever the mapping, so any grouping totals the same. A tester
built the mutant and ran it — correct versus mutant on `spec/1.0.1.md` gives seven keys against six,
both summing to 62. A field whose sole claim passes an implementation that assigns every section the
wrong count and loses a section key entirely is not pinned by anything. Separately it was the only
one of the four candidate fields **unbounded in output size** (about 149 bytes at seven anchors and
1,641 at ninety-one, on a command that honours neither `--compact` nor `--quiet`), and the only one
whose row named no decision it would feed.

**The six anchor defects it kept trying to describe are real and are now owned elsewhere**, with
fixtures, in `spec/backlog/ground-command-friction.md`. That is the useful residue: the field was
an attempt to publish a quantity whose keys nobody had checked, and checking them is the actual work.

**It reopens** when `engine.FloorAnchorOf`'s anchors are pinned by fixtures rather than by arithmetic,
and when an assertion exists that a wrong grouping can fail — a per-key expectation, not a total.

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
so the field could only ever report a high number: across six of this repository's specs it stayed
inside a narrow band near the top of its range. (The measurement is over each spec's latest emitted
floor; `tp ground --units` emits a round when the spec has moved, so it was taken in a copy.) And the direction is inverted — §4's shipped rule
replaces a figure with a *reference*, references are code spans, and a code span is in the
numerator. **A spec scores higher for obeying the rule.**

**A narrower variant does discriminate, and was still not shipped.** Restricting the numerator to
`floorHasDigit` alone, measured over the same six specs, gives a spread roughly twice as wide as the
two-arm form, and the spec that obeys the reference rule most closely scores *lowest* — the correct
direction. It was left out anyway, because discrimination is not the open question: what a figure
share *means* rests on one spec's `kind` table, n = 1, so shipping it would publish a contract for
an unmeasured hypothesis. That is the class `class_median_rounds` was removed for in the same
release.

**It reopens** when a second cycle's `kind`-by-finding-rate table exists, at which point the
narrow-numerator variant is the one to implement, not the two-arm one. The measurement command is in
`spec/1.0.1-measurements.md`.

### `forward-spec-ref` — the rule's population vanished with the version numbers

**What was tried.** A lint rule flagging a citation of a spec numbered *above* the citing one — a
forward reference to a release that may be renumbered before it ships. It survived prototyping once
and had a backlog spec; the spec was dropped, not shipped.

**Why it died.** Its population is gone. `python3 scripts/forward-spec-ref-prototype.py spec 1.0.1`
finds **1** reference at `HEAD` (`spec/1.0.1.md` citing `spec/1.0.2.md`, a file that no longer
exists), and the same command over `spec/backlog` finds **0** — because pending specs are no longer
numbered at all. The replacement predicate, *"a backlog spec is named by slug, never by priority
number"*, had no clean positive set to prototype against: every citation carried the priority number
in the filename it cited, which is why the filenames are now slug-only rather than why a lint should
exist. `scripts/forward-spec-ref-prototype.py` is now referenced by nothing but this entry.

**What the corpus actually needs is a dead-path check** — a `spec/…md` citation whose target does
not exist — and that is exactly what the dropped spec's §4.2 forbade. Measured at `HEAD` before this
cleanup, counting rule *occurrences of `spec/<version>.md` outside `.tp-review/` across `spec/**`,
`skills/**`, `CLAUDE.md` and `README.md`, minus files that exist*: 81 to `spec/1.3x–1.5x.md`, 6 to
`spec/1.0.2.md`, 1 to `spec/0.37.1.md` — every one resolvable a rename ago. Prototype-first, against
this repository's own `spec/*.md`; zero false positives at warning severity before it reaches a spec.

### A single whitespace set for the floor — corpus-free

**What was tried.** `spec/1.0.0.md` §2.1 names no whitespace set, and `internal/engine/floor.go`
implements its five steps with three predicates that disagree on exactly one byte, U+000B: RE2 `\s`
at the collapse, `isFloorSpaceByte` at the split and `unicode.IsSpace` at the trim. A draft release
(`spec/backlog/what-the-carry-can-promise.md`, its former second decision) named the six ASCII
whitespace bytes as the one set at every site.

**Why it died.** Zero floor units in the corpus carry a byte the predicates disagree on — none carries
TAB, VT, FF, CR, U+00A0 or U+0085 — so the change has no input to fire on; and of ten single-byte
inputs run under both binaries, the change moves hashes only on an interior U+000B. A behaviour change
that moves nothing in the corpus is not worth a loop-class cycle. The measurements, the fixtures and
the withdrawn test rows are in `spec/backlog/what-the-carry-can-promise-measurements.md`.

**It reopens** if a floor unit in the corpus is found to carry one of those bytes, or if a second
implementation of §2.1 in another language disagrees with tp on a `text_sha`.

---

## Survived, unscheduled

### Inferring a spec's class

**The claim it refutes.** A draft release proposed a `class: loop | tool` frontmatter field, declared by
the author, on the stated ground that *tp cannot infer it*. A unit told to construct a counter-example
built three predicates against 23 hand-labelled specs and **refuted the "cannot"**: a density predicate
— loop-lexicon occurrences per thousand words — scores **87% leave-one-out with zero false positives**,
the threshold refitted with each item held out.

**What makes it worth keeping rather than filing as a curiosity.** Applied to the seventeen shipped
cycles' round-1 snapshots, the predicate reproduces `CLAUDE.md`'s published loop/tool round medians
**exactly**, and yields `r(class, rounds) = +0.40`. The same unit's own hand-labels of those same
seventeen give `r ≈ 0` and do **not** reproduce the published split. So two independent human labellings
disagree, and the thing that tracks cycle length is the predicate rather than the label.

**That is also why the frontmatter field was cut from the release.** `class:` is a hand label; a median
computed over hand labels reports measured noise, and the field would have been empty on the day it
shipped — one of sixty-six specs carried it, and no recorded round carried a class at all.

**The design when a release takes this**: tp derives the class from the predicate and reports it,
frontmatter overrides, both are visible. It is deliberately not in the release that discovered it,
because the lexicon is a new surface that will drift and be argued over for rounds — this repository's
rule is that a new abstraction belongs to the next version.

**Honest limits, recorded so the next attempt does not overclaim**: n = 23; the two label sources are
two separate hand-assignments rather than one rule; and leave-one-out fixes the threshold but not the
lexicon, which was chosen after seeing the corpus.

## Undecided — each names the decision nobody has taken

### The divisible round

**The decision: the split key.**

Counting rule, re-derived over `spec/.tp-review/0.37.0/audit-round-*.ndjson` — distinct `item_id`
values carrying `role: spec-coverage`, and how many of them hold a non-`PASS` status in any round:
**6 of 97 distinct items (6.2%)** across that cycle's seven audit rounds. Splitting 74 items by
*count* therefore gives two shards that are each about 97% `PASS`.

The key the data recommends is **spec location**, and none of the three candidates named so far is
that. The six items are `list-0-2`, `table-2-1`, `table-2-13`, `table-2-14`,
`task-document-the-field` and `task-fence-change-rule`.

**One sub-claim did not reproduce, and both readings of it strengthen the conclusion rather than
weaken it.** The original said *"four of the six in one section"*. By the rows' own `location` field,
**all six** carry at least one row at `§3`; by item-id prefix, **three** share `table-2-`. Neither is
four, and either says the findings cluster by location.

### The test-file fence

**The decision: where the permission resolves** — a `tp` call per write inside the hook, or
precomputed into the child environment at spawn.

**And the list-layer semantics of `test_globs`**, because the one precedent is not a merge.
Re-derived at `HEAD`: `pickChecks` in `internal/engine/configresolve.go` returns the first present
layer and stops, so a present slice — an explicit empty array included — replaces the layer beneath
it rather than merging with it. Its own doc comment says so, and the pointer-to-slice type exists for
that reason: `Checks *[]Check` in `model.WorkflowOverride` is the **only** list-typed override field
in the struct; every other field there is a scalar pointer or raw JSON. A second list field has no
precedent to inherit beyond this one.

### The identifier set behind class families

**The decision: its own yield — what result would make it worth shipping.** The bar is unset, which
is what makes this undecided rather than refuted: a low yield is not an impossibility.

Measured over six cycles, the normaliser grouped **0–8.7% of findings, median 0.5%**; on `v0.37.0` it
formed one family of three findings.

**Only the denominator re-derives.** `v0.37.0`'s review rounds hold **630** recorded rows in total
(counting rule: non-blank lines across `spec/.tp-review/0.37.0/review-round-*.ndjson`), which matches
the original. The grouping does not re-derive, because **the normaliser is in no committed file** — a
search for its name across the whole tree returns only the candidates document. The percentages above
are carried as relayed and are not measurements anyone can reproduce today; a release that wants them
has to rebuild the normaliser first.

### The evidence contract

**The decision: what the open sections would have to name.** Its sections state a rule and no field,
no writer and no arithmetic — which is why it is here and not numbered.

The draft is reachable as `git show a4b70c3e:spec/0.49.0.md` (that number now belongs to a different
subject entirely, which is why the commit is the citation). Counting rule: headings whose text begins
`Open:`. There are **five**, not the four recorded — *the generator cannot author an experiment*,
*`UNVERIFIED` has no legal place in the loop*, *a declared evidence mode measures nothing*, *closure
evidence has no per-line carrier*, and *what would gate this*.

**Its two ready pieces have already been lifted out** and are releases of their own: the
forced-commitment brief, and mutation score as a documented gate entry. What is left is the part with
no design.

**A third open question joined it at `spec/1.1.0.md`'s review round 3: who reads a stored
`evidence`.** That release ships the carrier and nothing reads it back — `reviewFinding`
(`internal/cli/review.go:131-139`) has no such field, so the previous-round injection drops it. A
decision saying the injection carries it was written and cut, because the channel is not one: three
sites inject previous-round findings (`review.go:1345`, `review_regression.go:178`,
`review_verify.go:250`), the panel block caps its detailed rows at 50 and truncates its three
free-text renders at 80, 60 and 40 characters, and two of the three sites already print
`resolved.evidence` under the name *evidence*. Any decision here has to name a site, a bound and a
label; naming a site is an implementation sentence, which is what `1.1.0` exists to keep out of a
spec. `spec/1.1.0-measurements.md`'s *`evidence` is write-only: the three injection sites, and where
the reading half went* carries the runs. **No backlog file holds this** — `05-forced-commitment-in-the-brief.md`
§2 is the *audit* prior-round injection, not the review one.

### A registered check that outlives its release

**This entry was stale at `HEAD` and the correction is the substance.** Two of its measurements are
now false, and the third is what makes the workaround work.

**False as written.** `.tp/config.json` *does* register a check today — `code-citation-drift`, added
at the project layer in commit `4d155a88` — and `tp config --resolved` reports it with
`"source": "project"`, not `checks: []`.

**True, re-derived, and the reason the registration works.** `engine.RunCommand` hands its command
string to `sh -c` verbatim and substitutes nothing, so the project registration reaches its spec
through the shell instead: its `cmd` ends in
`"$(tp resume 2>/dev/null | python3 -c 'import sys,json;print(json.load(sys.stdin)["spec"])')"`,
which resolves at `HEAD` to `spec/1.0.0.md`. The same absence of substitution is why every task-file
registration hardcodes a path — `0.31.2.tasks.json` registers `test-inventory-drift` against its own
spec, `0.33.0.tasks.json` and `0.34.0.tasks.json` each register `code-citation-drift` against theirs.

**So the decision narrows, and it is the sharpest live item in this file.** It is no longer *whether a
registration can outlive its release* — one now does. It is whether `checks[].cmd` should gain a
first-class `{spec}` substitution in place of a subshell in a config field, which placeholders the set
should hold, and whether the round number is exposed. A shell subshell in a committed config is a
workaround that works and fails silently when `tp resume` cannot answer.

**The suppression hazard the entry names is real history and is only half closed.** tp tells every
reviewer to stop reporting a mechanized class, so during the two releases `code-citation-drift` was
registered per task file the class was suppressed, and when the registration died with the release
nobody was told it had come back. The project-layer registration closes that for this class.
`test-inventory-drift` is still registered in one task file only, so the same hazard stands for it.

### The write-deny fence's reach

**The decision: whether the fence should know where the repository is, and what it should anchor on**
— `CLAUDE_PROJECT_DIR`, a git-root walk, or nothing.

Re-derived by running the hook rather than by reading it. `denied()` in
`hooks/pre-tool-use-write-deny.sh` matches `*/.tp-review/?* | .tp-review/?*`, an unanchored glob. Fed
a `Write` payload naming `/private/tmp/throwaway-copy/spec/.tp-review/1.0.0/state.json` — a path in no
repository at all — the hook prints its scope-fence message and exits **2**.

**Not a defect.** The fence is fail-closed and correct where it is meant to apply, and the reach costs
a workaround rather than a wrong result. What has to be weighed is the other direction: an anchor that
is wrong is a fence that silently stops fencing, and that is a worse failure than the one it removes.

### `frontmatter-key-namespace` — a fixture whose frontmatter configures nothing

**The decision: which predicate.** Two are on the table, they catch different defects, and the
obvious one is the weaker.

**What it would catch.** A test fixture writes spec frontmatter to put the code under test into a
particular configuration, and the block does not do it. Two ways for that, both measured in this
repository during 1.0.1's audit:

1. **The key is outside the `tp:` mapping.** tp reads only `tp:`, so a top-level `domain:` key is
   inert. Three instances, all repaired at `c00266a0`: `internal/cli/lint_review_panel_test.go`,
   `internal/cli/role_panel_split_test.go`, `internal/engine/rolepanel_test.go`.
2. **The key is live but its value is the parser's default.** `ParseFrontmatter` returns
   `software` for a spec carrying no frontmatter at all, so a fixture declaring `tp.domain: software`
   is indistinguishable from one declaring nothing. One instance, found the round after the repair
   above and fixed by polarising the fixture — see `spec/1.0.1-measurements.md` §16 for the mutant
   that survived and the three panels measured.

**Predicate 1 is the obvious one and it misses the defect that matters.** The second instance's key
is *inside* `tp:`; a namespace check reads it as correct. Only a default-value check reaches it, and
that is the one that cost a round to find, because a fixture in class 2 is green under a mutant that
deletes the whole frontmatter read. Class 1 is the cheaper check and the shallower defect.

**Prototype first, and expect it to die there.** This repository's rule is that most candidate rules
do. Two things to measure before it reaches a spec:

- **The corpus is Go string literals, not markdown.** Swept at `HEAD`: **78** lines across **19** Go
  files carry a literal `\n---`, and **zero** `.md` files under `internal/` carry a frontmatter
  block. So the rule cannot be a `tp lint` rule over spec files — its subject is test source, which
  puts it in the same family as `scripts/check-test-inventory.py` rather than in the lint table. Both
  the file set and the parse are harder than a lint rule's.
- **Predicate 2's false-positive rate is unmeasured, and it has an obvious source**: a fixture may
  declare the default *on purpose*, as the control arm of a pair. A rule that cannot tell a control
  from a dud fires on both.

**One instance of class 2 is not a class.** Predicate 1 has three; predicate 2 has one, and one
instance is a bug report, not a rule. What would change that is a sweep of the other 18 files for
fixtures whose declared value equals the parser's default — not run, and the honest reason the entry
is here rather than in a release.

### A fenced command that runs and prints the wrong thing

**The decision: what a fenced-command check compares its output against.** §4.1 of `spec/1.0.1.md`
asks that every fenced command run and print something. That is a liveness check, and liveness is
not truth.

**The instance, from this release's own audit round 3.** `README.md`'s *"What grounding finds"*
block, added at `8dfa6fb3`, globbed only `spec/.tp-review/*/` — while this repository's ground rounds
also live under `spec/backlog/.tp-review/*/`, the two-glob trap `CLAUDE.md` documents. The command
ran, exited 0 and printed a full set of numbers; not one of them was a figure the prose beside it
stated, and adding the second glob printed a third set again. Nothing in §4.1's rule can see that,
because the rule's subject is whether output appeared. The repair was to add the second glob and
delete the prose figures, which is what §4's own *"a number does not live in a spec, a reference
does"* already asks for.

**Prototype first, and expect it to die there.** The obvious predicate — re-run each fenced command
and compare its output against the figures in the surrounding prose — needs a mapping from a figure
in prose to a position in a command's output, and that mapping exists nowhere. The weaker one, *"a
fenced derivation must not sit beside a literal number the prose asserts"*, is checkable and would
fire across much of this corpus; whether it fires anywhere it should is unmeasured, and this
repository's bar is zero false positives at warning severity.

### The `implementation-detail` lint over spec prose

**The decision: what the predicate is, before whether it is worth having.** `spec/1.1.0.md`'s
*Alternatives considered* names this rule and does not take it. Nothing has been prototyped — there is
no candidate predicate, no run over this repository's own `spec/*.md`, and therefore no
false-positive rate to hold against the zero-at-warning-severity bar.

**Prototype first, and expect it to die there.** The rule this repository applies to every lexical
candidate applies here unchanged: run it over `spec/*.md`, and over the pre-repair text of the defect
that motivated it, before any spec names it as a decision. The motivating defect is on record —
`v1.0.1`'s cycle spent five grading rounds refuting five successive sets of sentences about `tp lint`
fields that did not exist yet, and `spec/1.1.0-measurements.md` carries it. What a candidate must
separate is that text from the acceptance rows in the same documents, which name commands, fields and
exit codes legitimately; the *Refuted* entries above are what happens when a candidate cannot.

### Claim enumeration in the grounding floor

**The decision: what a claim is, before the floor's arms decide which sentences reach it.** The
weakest step of the grounding protocol, inherited from `spec/candidates.md` by
`spec/backlog/ground-command-friction.md` and carried here because it has no design. Intuition counted
11 where a spec carried 17, and 10 where another carried 17 again after a second read. Whether that is
a parsing problem, a definition problem, or irreducibly a reading problem is not yet clear.
`ground-command-friction.md` §5 — a bare ordered-list marker becoming a floor unit — is one small,
measured piece of it; the rest is not.

### A sentence rewritten in answer to a finding is exempt from the cut for one round

**The decision: whether a repair is graded once before the arms drop it, and in what form.** Measured
on `spec/1.1.0.md`'s grounding and recorded in `spec/backlog/ground-command-friction-measurements.md`
under "§11.1": a repair removed a quantifier, the shortened sentence fell below the arms' cut
threshold, and the claim left the floor in the same edit that answered the finding — nothing in the
round reports that. A third instance in the same document wrote a new requirement as a short
standalone sentence that never entered the floor at all. The proposal is that a sentence rewritten in
response to a finding is exempt from the cut for one round, so the repair is graded once; the cheapest
form is not an exemption but a reported `cut` delta, since two of the three instances were repaired by
the author the moment they saw the number. Neither the cost of the exemption nor whether it is
expressible in the floor's own terms has been measured.
### Cross-site key agreement in the review prompt

**The decision: whether the review prompt's three key-naming sites should name one set, and where the
source of truth would live.** `spec/1.1.0.md` §5 row 8 carried this as a SHALL for one grading round
and it was cut — because satisfying it falls outside that release's scope, not because the sites
disagreeing is desirable.

**Measured at `c407bb7e`**, on `tp review spec/1.1.0.md --role implementer` and on a `tp audit`
emission:

| site | keys it names |
|---|---|
| `findingFormat`'s JSON example (`internal/cli/review.go`) | `severity`, `category`, `location`, `finding`, `suggestion` |
| the optional-`class` sentence beside it | `class` |
| `outputContractInstruction` | `role`, `location`, `class`, `severity` |

The three differ by design, and each obvious way to equalise them costs something a release would
have to decide about first:

- `outputContractInstruction` is **shared with the audit phase** — called from `review.go`,
  `review_regression.go` and `audit_roles.go`, and all four auditor prompts carry the block. Editing
  it to match the review example changes the audit prompt too.
- Dropping `category` from the example to match the contract block empties `by_category`, a key
  `tp review --report` ships (`internal/cli/review_report.go`).
- Making the three name one set promotes `class` — labelled *Optional* in the sentence beside the
  example — to mandatory.

**What has no answer is which set is right**, not how to render one set at three sites. A release
that takes this fixes the required set for both phases at once, or states why the two phases keep
different ones; that is the decision nobody has taken.

### `NewRootCmd` writes package globals

**The decision: whether `NewRootCmd` returns a command bound to *fresh* per-call storage, or whether
the package globals become explicitly single-writer with the constructor refusing a second call.** The
first is the real repair and is a wider change than it looks — it changes how every flag's default is
read; the second is a fence and would have caught the instance below on the first parallel test rather
than the thirtieth. Neither has been taken. It was recorded in `spec/backlog/gate-sequence.md` as a
gate step that is green on the first run and red on the second — the same class as that spec's
load-sensitive test, arriving by a different route — and moved here because that spec takes no
decision on it and no row of its tests depends on it.

**Measured during `spec/1.0.1.md`'s implementation.** A unit ran the four-step gate, got four zeroes,
made an unrelated edit, ran it again, and step 1 failed with data races in tests it had never
touched. Isolated: the two tests `TestSkillFlagInventoryIsComplete` and
`TestSkillFlagInventoryRecordsOnlyWhatExists` each construct a root command, both are `t.Parallel()`,
and `NewRootCmd()` binds tp's **package-level** flag variables through pflag. Run alone under `-race`
the pair reports **30 races per run**. It reproduces on a tree five commits earlier and originates at
`fa68051b`, *"mark every eligible top-level test parallel — 50.9s to ~14s"*: the parallelism was the
speedup, and the shared globals were already there.

**Why it is a gate defect and not only a test defect.** The defect is invisible without `-race` and
timing-dependent with it, so whether the gate reddens is a property of machine load rather than of the
code under test. A gate that passes once and fails once teaches its operator to re-run rather than to
look.

**Closed test-side, not at the source.** `3b9204e1` added a mutex-guarded `newRootCmdForTest()` and
the races went to zero. The design smell stands: **a constructor that writes package globals**, so
every future parallel test that builds a root command inherits the same trap, and the fix is a
convention nothing enforces.

**Is this only a test problem? Answered by counting rather than left open.** At `HEAD`, `NewRootCmd`
is defined in `internal/cli/root.go` and called from exactly one production site, `Execute()` in the
same file, which `cmd/tp/main.go` calls once — so a shipped `tp` process constructs one root command
and the globals are never contended. Today it is test-only. What makes it a design defect rather than
a test defect is that **nothing says so**: a second in-process caller — an embedding host, an
in-process driver, a future `tp` subcommand that shells to itself in-process — would share the flag
variables silently, and would find out the way the gate did.

### `t.Parallel()` in the engine package

**The decision: whether `internal/engine`'s tests may be marked `t.Parallel()` — and the only answer
that will be accepted is a paired gremlins run, `--workers` pinned, before and after, compared on
efficacy and the timeout count rather than on wall time.** Until that measurement exists,
`internal/cli` is free to parallelize and `internal/engine` is not. It was carried in
`spec/backlog/mutation-run-check.md` as an open question and moved here because that check takes no
decision on it and no row of its tests depends on it.

**The undecided part is `internal/engine` alone**, for one reason: that is the package
`gremlins unleash ./internal/engine` mutates, and the expensive one. The `internal/cli` half is not
part of this question: `fa68051b`, *"mark every eligible top-level test parallel — 50.9s to ~14s"*,
applied it, and the count of `t.Parallel()` calls in the repository is a derivation rather than a
figure to quote — `rg -c 't\.Parallel\(\)' -g '*_test.go' --no-filename | paste -sd+ | bc` — because
it moves with every test added. What the candidate entry claimed for that half, re-derived at the
time it was moved, with the counting rule beside each figure and the ones that did not reproduce
marked as such:

| the entry's figure | re-derived | how it was derived |
|---|---|---|
| `internal/cli` is **1,743** serial test functions | **does not reproduce as an `internal/cli` figure.** The package holds **1,051** top-level `func Test…`; **1,781** is the *repository-wide* count, which is what 1,743 tracks | `rg '^func Test' internal/cli --no-filename \| wc -l` against `rg '^func Test' -g '*_test.go' --no-filename \| wc -l` |
| **1,116** of them fork the tp binary | **the counting rule decides which number this is.** `runTP(` appears at **1,120** *call sites*; **736** top-level test *functions* have a body calling any `runTP*` helper | `rg -o 'runTP\(' internal/cli --no-filename \| wc -l`, against a `re.split(r'(?m)^func ', src)` walk over `internal/cli/*_test.go` counting bodies matching `\brunTP[A-Za-z]*\(` |
| I/O-bound at **8%** of ten cores — **39.7 s** CPU inside **50.5 s** wall | **holds in shape.** First run in a fresh copy: `real 47.37 user 16.14 sys 20.75` — **36.9 s** CPU inside **47.4 s** wall, **7.8%** of ten cores | `/usr/bin/time -p go test ./internal/cli -count=1` in an `rsync -a --exclude .git` copy |
| the **7** files that `t.Chdir` are skipped, because Go panics on the pair | **holds exactly: 7** | `rg -l 't\.Chdir' internal/cli \| wc -l` |
| **zero** `os.Setenv` in the package | **holds: 0.** The package-level-variable-write half of that claim was not re-derived | `rg -c 'os\.Setenv' internal/cli \| wc -l` |
| **54 s → 14 s**, and **15.8 s** under `-race`, four consecutive green runs, `go vet` clean | **borrowed, not re-run.** Measured in an `rsync` copy at v1.0.0's audit round 5; that copy is gone, and the 54 s baseline reads 47.0 s in row 3 | — |

**Why `internal/engine` is the half that stays undecided.** gremlins runs the mutated package's
**own** tests once per mutant. This repository has already measured those runs as load-sensitive —
`CLAUDE.md` records the same package at **92** timeouts busy against **88** idle, and default settings
driving load average from **8** to **177** (`grep -n 'load average' CLAUDE.md`) — and `t.Parallel()`
multiplies concurrency *inside* each mutant by gremlins' own `--workers`. So the question is whether
parallelizing `internal/engine` leaves the mutation signal intact.

**The protocol that paired run needs, which the candidate entry did not state.** The entry closed by
telling the reader to watch for `Lived: 0` beside `Not covered > 0`. That instruction is stale:
`spec/backlog/mutation-run-check-measurements.md` measures both halves under "Neither the file nor
the argv settles it" — the signature is **necessary under the corruption and not sufficient**, an
honest two-file probe reaches it with no flags at all, and **the argv does not settle it either**,
because a later run in a directory that already held one returns the refused signature with no
`--test-cpu` anywhere in it. **So each arm must be the FIRST gremlins run in its own fresh
`rsync -a --exclude .git` copy** — one copy for the serial tree, one for the parallelized tree,
neither directory reused. Without that the "after" arm is confounded by run order and the experiment
answers nothing: it would return the refused signature and read as a mutation signal that
`t.Parallel()` destroyed. The mechanism behind the run-order effect is unknown; it was not chased past
ruling out the test cache, and nothing here proposes one. A protocol that works without a mechanism
is what the paired run needs; a guess at the mechanism is not.

### A prior-round section for `tp review`

**The decision nobody has taken is whether `tp review` should have one at all.** Carried in from
`spec/candidates.md` by way of the repair-locality spec, which took no decision on it and whose tests
do not depend on it.

**What the audit phase does.** `loadAuditPriorRound` (`internal/cli/audit.go`) reads the previous
recorded audit round and returns, per role, that role's **own** non-PASS rows;
`renderPriorRoundSection` (`internal/cli/audit_roles.go`) renders them into a round-2+ prompt under
the heading *"Prior Round: context to re-check, not a verdict to repeat"*, with the instruction
*"Re-check each item against the code and record your own status. Do NOT repeat the prior verdict
without verifying."* It returns the empty string when the role has no prior non-PASS rows, so a
round-1 prompt and an all-PASS role carry no section at all. `filesChangedSince` in the same file
tells the role whether its evidence file moved since that round, which is what makes the re-check
answerable rather than rhetorical. So the audit phase hands a role a bounded, role-scoped set of its
own judgements and forces a commitment on each. That is the shape whose absence on the review side is
the question.

**Review carries something, of a different kind.** A search for prior-round machinery by that name in
the review path returns **0** — `rg -n -i 'priorRound|prior round|prior-round|PriorRow' internal/cli/review.go internal/cli/review_*.go`.
But `buildFindingsSummary` (`internal/cli/review.go`) does put previous rounds into every review
prompt, under a heading that asks for the opposite of a re-check — `UNRESOLVED findings from previous
rounds — DO NOT re-report:` — and a second block, `Resolved high/critical (DO NOT regress):`, both
locatable with `rg -n 'DO NOT re-report|DO NOT regress' internal/cli/review.go`. Its shape differs
from the audit section on every axis: it is **panel-wide rather than role-scoped**, it is capped at 50
rows with the remainder reported only as a count, each finding is truncated to 80 characters and a
`wontfix` row's evidence to 40, and its input is the `--findings` file or the loaded round state
rather than the recorded round read per role.

**So the two phases differ in kind, not in presence.** The audit returns a role its own rows and
obliges it to re-verify each; review shows the whole panel everyone's rows and obliges it to stay
quiet about them. A re-verification ask and a suppression ask are not the same instrument, and this
repository has already measured what an unexamined suppression costs elsewhere in the loop.

**No measurement says which way that cuts.** The repair-locality figures are about the adjacent
surface — findings sitting in text the round before wrote — but a share and a ratio cannot say whether
returning a reviewer its own prior rows would raise that share (the role re-treads its own ground) or
lower it (the role withdraws instead of re-filing). A number nobody has acted on yet is not a rule,
and it is not an argument for a mechanism either.
### From the rows spec: four questions

**This entry takes no decision.** Four questions were carried in `02b-what-a-rounds-rows-say.md` §5a,
which moved them out of `spec/candidates.md` into the release that owned their subject; that file is
now a forwarding stub and the questions live here. A spec that presented one of these as settled would
be worse than not moving it: their *design* has no answer.

**A durable home for an accepted finding.** The decision nobody has taken is the target shape. An
accepted finding stops blocking (`a-finding-can-leave-an-audit-round.md`) — this is the durability
half: an audit finding has three ends (fix it, reject it, accept it as backlog), tp records all three
the same way in `.tp-review/<spec>/`, and that directory is archived at release along with the spec.
There is no supported path from *accepted* to something a maintainer trips over later. A deferral
whose stated reason is self-renewing can be re-derived every round forever, and its only record is
scheduled for archival on the very release it is deferred past. This repository has been working
around it by hand for four cycles: the candidates files are that durable target, maintained by an
operator. Three options were named and none chosen; the candidates row records only their number, and
the three themselves survive in git, in the pre-2026-09-02 spec that carried this subject —
`git show 3a83be30:spec/0.41.0.md`, whose §2 is *Mechanism*. That filename corresponds to no file
under `spec/` or `spec/backlog/` today — the 2026-09-02 renumbering moved every number and the slug
sweep removed them — which is why the commit is cited rather than a path at `HEAD`. The three: a
checklist file at a stable path, appended to rather than rewritten; an issue template written to disk
for the operator to file; a `TODO` entry with an owner and the finding's `item_id`. What decides
between them is a property rather than a preference, and it is the part already agreed: *the target
must be readable by the next cycle's decomposition without a human remembering it exists.* An option
that only works when someone happens to open it is not better than the round directory it replaces.

**Making `severity` checkable.** The decision nobody has taken is what could check it that is not the
row's author. A non-`PASS` row's severity is self-declared: the prompt renders the requirement and
nothing validates what comes back. `internal/cli/audit_record.go` validates **`category`** alone,
through `invalidCategoryRows` — there is no severity equivalent. One clause of the original entry was
stale and is corrected here rather than carried: it said *"nothing on the audit path reads the
field"*. At `HEAD` two paths read it: `internal/engine/auditclean.go`'s `AuditSeverityBucket` and
`advisoryAuditSeverities` grade on it whenever `audit_converge_on` is `blocking` (v0.37.0), and
`tp audit --merge` buckets `by_severity` through the same classifier. What survives is narrower and
still the point: **nothing validates it**, and under the resolved default — `tp config --resolved`
reports `audit_converge_on` at `all`, source `default` — no grading path reads it at all. Two
mechanisms were drafted and both withdrawn within a round. Rejecting an out-of-enum severity at
`--record` inverts its own precedent: the category sink returns early on an *empty* category —
`if category == "" || engine.IsValidCategory(category)` in `invalidCategoryRows.observe` — and that
early return is pinned deliberately by `TestParseAuditRows_AcceptsTheEnumAndAbsentCategory` in
`internal/cli/audit_category_sink_test.go`, whose comment gives the reason: *"treating that as invalid
would reject every clean round."* (Cite the symbol and the test name, not a line.) And it would refuse
fifteen of this repository's own recorded round files — **15 of 108** at `HEAD`, counting an audit
round file `spec/.tp-review/*/audit-round-*.ndjson` holding at least one row whose `severity` is
present and outside the audit vocabulary `{error, warning, info}` that `auditclean.go` defines. The
offending values are the *review* vocabulary leaking into audit files, and the fifteen are 0.29.0
rounds 1–2, 0.30.0 round 1, 0.31.0 round 1, 0.31.2 rounds 1–5 and 0.32.0 rounds 1–6. The review side
is clean under its own vocabulary: **0 of 172** review round files carry a severity outside
`{critical, high, medium, low}`. And validation cannot deliver what it was introduced for: it makes a
label well-formed, never truthful, and it cannot make an unrun round run.

**An audit-side `nonblocking_open`.** The decision nobody has taken is whether to invert a guard that
pins the key's absence. Under `audit_converge_on=blocking` a clean round can carry `warning` and
`info` rows — `clean: true` with `findings: 1` is a built fixture — so the audit phase now has the
accepted-open state the review-side field was built to make visible. The count is already emitted;
what is missing is the breakdown. `role_streaks[].open` is that count and it is **severity-blind**:
`engine.RoleStreak`'s `Open` is documented in `internal/engine/rolestreaks.go` as the role's *non-PASS
row count in that latest round*, with no reference to severity. It reaches both `tp audit --status`
and `tp audit --record`, and `tp run --status` on an audit-phase stop, through `auditSignalFields` in
`internal/cli/audit_record.go` — anchor on that function, not a line. Four places pin the key's
absence by name, and all four are present at `HEAD`: `spec/0.31.0.md` §8.4 — *"`nonblocking_open` is
review-only (audit convergence has no non-blocking notion, §4.4) and is never an audit field"*; that
spec's test 18, which repeats it as an acceptance row; `skills/tp/REFERENCE.md`'s `--compact`
disposition paragraph, which marks the field *"(review-only, emitted only on an accepted-open clean
round)"* — cited by that phrase, because its line number has moved; and
`TestReviewNonBlockingOpen_AuditUnaffected` in `internal/cli/reviewconvergeon_clean_test.go`, which
asserts that neither `tp audit --record` nor `tp audit --status` emits the key. A fifth statement
exists and is a comment rather than a guard: `internal/engine/reviewclean.go`'s *"Review-only: no
audit payload carries nonblocking_open."* No release has decided to invert any of them. The
`tp audit --merge` breakdown is a different surface and shipped separately — it buckets `by_severity`
through `engine.AuditSeverityBucket` in `internal/cli/audit_merge.go`, on the merged payload rather
than on the round's convergence signal.

**A review-side `accepted_blocking`.** The decision nobody has taken is whether an accepted blocking
finding gets a counter of its own. The review side already emits `nonblocking_open`, and it fires only
for the case that does not matter. Measured at `5058fc99` on three one-round trees built from the same
spec: a round recorded from a zero-byte findings file — `clean: true`, `consecutive_clean: 1`; a round
holding one `critical` finding resolved `wontfix` with evidence — `clean: true`,
`consecutive_clean: 1`; a round holding one **open** `medium` finding — `clean: true`,
`consecutive_clean: 1`, and `nonblocking_open: 1`. `tp review <spec> --status` on the first two returns
payloads whose **key sets are identical** — the symmetric difference is empty — so no key names the
accepted critical. The only value that moves is `review_rounds[].findings`, 0 against 1, which says
nothing about severity or disposition. The third differs from the first by exactly one key,
`nonblocking_open`. So the surface announces the harmless case and is silent on the one a reader would
want stopped at. An `accepted_blocking` count — rows excluded from the surviving set whose severity is
blocking — would close it. It is review-side, true at `HEAD`, and depends on nothing any pending spec
proposes.

---

## Fog — in scope, not yet sharp enough to state as a question

**The test is whether the question can be stated precisely now, not whether it can be answered now.**
An entry here is coarser than the entries above it: one may graduate into several questions, or none,
once the frontier reaches it. Keeping the two apart stops a half-seen problem from being pre-sliced
into confident-looking entries it does not yet deserve.

- **Whether the emission's prohibitions are worth sweeping.** 14% of the review prompt and 12% of
  `CLAUDE.md` steer by ban. A few of those are hard guardrails that earn it. No measurement separates
  the two, and a rewrite without one is prose churn.
