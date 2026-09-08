# Undecided — and refuted

**Not a release, and not a spec.** Two kinds of thing live here, and the distinction is the point:

- **Refuted** — a candidate that was prototyped and did not survive. It is recorded so it is not
  re-proposed, with the measurement that killed it. **A refuted predicate is not a backlog item.**
- **Undecided** — a real need whose *design* has no answer yet. Each names the decision nobody has
  taken.

Writing a release spec for something whose design has no answer is the failure this file exists to
prevent. An entry here is not a draft of a spec and must not be read as one.

**No item carries a version number.** A number that moves leaves stale references and this repository
has paid for that repeatedly; the roadmap in `spec/backlog/README.md` is where ordering belongs.

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

**It settles** when `spec/backlog/round-records-the-text-it-read.md` ships, after which a replay of
any round recorded from then on is well-defined. That spec does not repair the divergent rounds
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
fixtures, in `spec/backlog/ground-command-friction.md`. That is the useful residue: the field was
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

---

## Survived, unscheduled

### Inferring a spec's class

**The claim it refutes.** A draft release proposed a `class: loop | tool` frontmatter field, declared
by the author, on the stated ground that *tp cannot infer it*. A unit told to construct a
counter-example built three predicates against 23 hand-labelled specs and **refuted the "cannot"**: a
density predicate — loop-lexicon occurrences per thousand words — scores 87% leave-one-out with zero
false positives, and it reproduces `CLAUDE.md`'s published loop/tool round medians exactly where two
independent hand-labellings do not.

**The design when a release takes this**: tp derives the class from the predicate and reports it,
frontmatter overrides, both are visible. It is deliberately not in the release that discovered it,
because the lexicon is a new surface that will drift and be argued over for rounds — this
repository's rule is that a new abstraction belongs to the next version.

**It is scheduled** when a release needs the class to budget rounds *before* its own review opens —
that is, when `CLAUDE.md`'s loop/tool round medians are used to pick a cap rather than to explain one
after the fact. Nothing before that point spends the lexicon's drift cost usefully.

**Where the measurements are.** `spec/undecided-measurements.md` §Inferring a spec's class — the
correlations, why the frontmatter field was cut, and the three honest limits.

## Undecided — each names the decision nobody has taken

### The divisible round

**The decision: the split key.** The key the data recommends is **spec location**, and none of the
three candidates named so far is that.

Counting rule, over `spec/.tp-review/0.37.0/audit-round-*.ndjson` — distinct `item_id` values carrying
`role: spec-coverage`, against how many hold a non-`PASS` status in any round:

```bash
python3 - <<'PY'
import json,glob
ids=set(); bad=set()
for f in sorted(glob.glob("spec/.tp-review/0.37.0/audit-round-*.ndjson")):
    for l in open(f):
        if not l.strip(): continue
        r=json.loads(l)
        if r.get("role")!="spec-coverage" or r.get("item_id") is None: continue
        ids.add(r["item_id"])
        if r.get("status")!="PASS": bad.add(r["item_id"])
print("%d distinct items, %d non-PASS: %s" % (len(ids), len(bad), sorted(bad)))
PY
```

At `13bfde30` that is 6 of 97, and the six are `list-0-2`, `table-2-1`, `table-2-13`, `table-2-14`,
`task-document-the-field` and `task-fence-change-rule`. Splitting the items by *count* therefore gives
two shards that are each overwhelmingly `PASS` and neither tells a reader where the findings are.

**It unblocks `spec/backlog/checklist-covers-what-changed.md`**, which names this entry as its real
answer: that release bounds a per-prompt checklist at ten items and says outright that measuring the
bound is the divisible round's job.

**Where the measurements are.** `spec/undecided-measurements.md` §The divisible round — the sub-claim
that did not reproduce and why both readings of it strengthen the conclusion.

### The test-file fence

**The decision: where the permission resolves** — a `tp` call per write inside the hook, or
precomputed into the child environment at spawn.

**And the list-layer semantics of `test_globs`**, because the one precedent is not a merge.
`pickChecks` in `internal/engine/configresolve.go` returns the first present layer and stops, so a
present slice — an explicit empty array included — replaces the layer beneath it rather than merging
with it. Its own doc comment says so, and the pointer-to-slice type exists for that reason:
`Checks *[]Check` in `model.WorkflowOverride` is the **only** list-typed override field in the struct;
every other field there is a scalar pointer or raw JSON. A second list field has no precedent to
inherit beyond this one.

**It settles** when a second list-typed override exists in `model.WorkflowOverride`, because the
resolution site is then chosen once for both rather than argued for one field against a sample of one.

### The identifier set behind class families

**The decision: its own yield — what result would make it worth shipping.** The bar is unset, which
is what makes this undecided rather than refuted: a low yield is not an impossibility.

**Only the denominator re-derives.** `v0.37.0`'s review rounds hold **630** recorded rows in total
(counting rule: non-blank lines across `spec/.tp-review/0.37.0/review-round-*.ndjson`), which matches
the original. The grouping does not re-derive, because **the normaliser is in no committed file** — a
search for its name across the whole tree returns nothing. The relayed grouping figures (0–8.7% of
findings, median 0.5% over six cycles; one family of three findings on `v0.37.0`) are carried as
relayed and are not measurements anyone can reproduce today; a release that wants them has to rebuild
the normaliser first.

### The evidence contract

**The decision: what the open sections would have to name.** Its sections state a rule and no field,
no writer and no arithmetic — which is why it is here and not scheduled.

The draft is reachable as `git show a4b70c3e:spec/0.49.0.md` (that number now belongs to a different
subject entirely, which is why the commit is the citation). Counting rule: headings whose text begins
`Open:`. There are **five**, not the four recorded — *the generator cannot author an experiment*,
*`UNVERIFIED` has no legal place in the loop*, *a declared evidence mode measures nothing*, *closure
evidence has no per-line carrier*, and *what would gate this*.

**Its two ready pieces have already been lifted out** and are releases of their own: the
forced-commitment brief, and mutation score as a documented gate entry. What is left is the part with
no design.

**A third open question joined it at `spec/1.1.0.md`'s review round 3: who reads a stored
`evidence`.** That release ships the carrier and nothing reads it back — `reviewFinding` in
`internal/cli/review.go` has no such field. A decision saying the previous-round injection carries it
was written and cut, because the channel is not one: three sites inject previous-round findings —
`buildFindingsSummary` (`review.go`), `buildRegressionPrompt` (`review_regression.go`) and
`buildVerifyPrompt` (`review_verify.go`) — with different caps and labels, and only the last prints
`resolved.evidence` under the name *evidence*. Any decision has to name a site, a bound and a label,
and naming a site is the implementation sentence `1.1.0` exists to keep out of a spec. **No backlog
file holds this** — `brief-carries-the-forcing-sentences.md` kept only its forcing sentences at the
2026-09-08 re-verification, and the section it dropped was the *audit* injection, not the review one.

**Where the measurements are.** `spec/1.1.0-measurements.md`, *`evidence` is write-only: the three
injection sites, and where the reading half went*, carries the caps and the runs;
`spec/undecided-measurements.md` §The evidence contract summarises which site labels what.

### A registered check that outlives its release

**The decision narrows to one thing, and it is the sharpest live item in this file.** It is no longer
*whether a registration can outlive its release* — one does. It is whether `checks[].cmd` should gain
a first-class `{spec}` substitution in place of a subshell in a config field, which placeholders the
set should hold, and whether the round number is exposed.

**Why the workaround works, and how it fails.** `engine.RunCommand` hands its command string to
`sh -c` verbatim and substitutes nothing, so a project-layer registration reaches its spec through the
shell instead: each `cmd` ends in
`"$(tp resume 2>/dev/null | python3 -c 'import sys,json;print(json.load(sys.stdin)["spec"])')"`, which
resolves at `13bfde30` to `spec/1.1.0.md`. Two checks are registered that way today —
`code-citation-drift` and `method-only-in-ungraded-sidecar`, both reported by `tp config --resolved`
with `"source": "project"` — and the same absence of substitution is why every task-file registration
hardcodes a path. A subshell in a committed config **fails silently when `tp resume` cannot answer.**

**The suppression hazard is real history and is only half closed.** The project-layer registration
closes it for `code-citation-drift`; `test-inventory-drift` is still registered in one task file only,
so the same hazard stands for it.

**It settles** when a release takes `checks[].cmd` substitution. No pending spec under `spec/backlog/`
proposes one — `spec/backlog/repair-locality.md` names `workflow.checks` once, in passing, and takes
no decision on it — so nothing is currently scheduled to settle it.

**Where the measurements are.** `spec/undecided-measurements.md` §A registered check that outlives its
release — the two measurements that were false as written, and the per-task-file registrations.

### The write-deny fence's reach

**The decision: whether the fence should know where the repository is, and what it should anchor on**
— `CLAUDE_PROJECT_DIR`, a git-root walk, or nothing.

Re-derived by running the hook rather than by reading it. `denied()` in
`hooks/pre-tool-use-write-deny.sh` matches `*/.tp-review/?* | .tp-review/?*`, an unanchored glob. Fed
a `Write` payload naming `/private/tmp/throwaway-copy/spec/.tp-review/1.0.0/state.json` — a path in no
repository at all — the hook prints its scope-fence message and exits **2**.

**The reach is wider than "writes", and wider than one file.** During the 2026-09-08 pass the
installed hook refused two **read-only** MCP calls whose payloads merely *named* `.tp/config.json` and
`spec/0.25.0.tasks.json`, one of them a batch whose other five operations touched neither. So the
fence matches on any argument string in the payload: it reaches reads, and one fenced path in a
batched multi-file call refuses the whole call.

**Not a defect.** The fence is fail-closed and correct where it is meant to apply, and the reach costs
a workaround rather than a wrong result. What has to be weighed is the other direction: a wrong anchor
is a fence that silently stops fencing, a worse failure than the one it removes.

**It settles** when the hook matches on the tool's write *target* rather than on any argument string —
at which point both halves of the reach close at once, and the anchor question becomes answerable
against a target rather than against a payload.

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

**Prototype first, and expect it to die there.** Two things to measure before it reaches a spec:

- **The corpus is Go string literals, not markdown.** Swept at `27f84468`: **78** lines across **19**
  Go files carry a literal `\n---`, and **zero** `.md` files under `internal/` carry a frontmatter
  block. Its subject is test source, which puts it in the same family as
  `scripts/check-test-inventory.py` rather than in the lint table.
- **Predicate 2's false-positive rate is unmeasured, and it has an obvious source**: a fixture may
  declare the default *on purpose*, as the control arm of a pair. A rule that cannot tell a control
  from a dud fires on both.

**It settles** when the sweep of the other 18 files is run and predicate 2 has more than one instance.
One instance is a bug report, not a rule.

**Where the measurements are.** `spec/undecided-measurements.md` §`frontmatter-key-namespace`.

### A fenced command that runs and prints the wrong thing

**The decision: what a fenced-command check compares its output against.** §4.1 of `spec/1.0.1.md`
asks that every fenced command run and print something. That is a liveness check, and liveness is
not truth: the instance that motivated this ran, exited 0, and printed a full set of numbers, not one
of which was a figure the prose beside it stated. The repair to that instance has shipped — `README.md`
now globs both round directories inside the fenced command and derives its counts there — which
removes the instance and not the gap in the rule.

**Prototype first, and expect it to die there.** The obvious predicate — compare each fenced command's
output against the figures in the surrounding prose — needs a mapping from a figure in prose to a
position in a command's output, and that mapping exists nowhere. The weaker one, *"a fenced derivation
must not sit beside a literal number the prose asserts"*, is checkable and would fire across much of
this corpus; whether it fires anywhere it should is unmeasured, against a bar of zero false positives.

**Where the measurements are.** `spec/undecided-measurements.md` §A fenced command that runs and
prints the wrong thing.

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

**The decision: whether a repair is graded once before the arms drop it, and in what form.** The
proposal is that a sentence rewritten in response to a finding is exempt from the cut for one round,
so the repair is graded once; the cheapest form is not an exemption but a reported `cut` delta.
Neither the cost of the exemption nor whether it is expressible in the floor's own terms has been
measured, and that is what keeps it undecided.

**Where the measurements are.** `spec/backlog/ground-command-friction-measurements.md` under "§11.1"
carries the three instances; `spec/undecided-measurements.md` §A sentence rewritten in answer to a
finding is exempt from the cut for one round summarises why the cheaper form is the one to cost first.

### Cross-site key agreement in the review prompt

**The decision: whether the review prompt's three key-naming sites should name one set, and where the
source of truth would live.** `spec/1.1.0.md` §5 row 8 carried this as a SHALL for one grading round
and it was cut — because satisfying it falls outside that release's scope, not because the sites
disagreeing is desirable.

**Re-measured at `13bfde30`** by reading the three sites, since the reading recorded at `c407bb7e`
predates two changes to them:

| site | keys it names |
|---|---|
| `findingFormat`'s JSON example (`internal/cli/review.go`) | `severity`, `category`, `location`, `finding`, `evidence`, `suggestion` |
| the optional-`class` sentence beside it | `class` |
| `outputContractInstruction`, review branch | `role`, `location`, `class`, `severity`, `evidence` |

`evidence` is common to the first and third (it reached the contract block *after* `a4fd187f`); what
still differs is `category`, `finding` and `suggestion` in the example alone, against `role` and
`class` in the contract alone. Each obvious way to equalise them costs something a release would have
to decide first: `outputContractInstruction` is shared with the audit phase, so editing it changes the
audit prompt too; dropping `category` from the example empties `by_category`, a key
`tp review --report` ships; and naming one set promotes `class` from *Optional* to mandatory.

**One more thing belongs to this same decision, routed from `spec/0.35.0-candidates.md` item 13**: the
review-side `category` enum is a bare string literal inside the prompt template, with no constant, no
validator and no sink check, while the audit side declares typed constants with a validator
(`internal/engine/audit_category.go`) and rejects anything else at the record sink. One field name,
two disjoint vocabularies, two standards — and whether the review enum should be validated is not
separable from which set the three sites should name.

**What has no answer is which set is right**, not how to render one set at three sites. A release
that takes this fixes the required set for both phases at once, or states why the two phases keep
different ones; that is the decision nobody has taken.

**Where the measurements are.** `spec/undecided-measurements.md` §Cross-site key agreement in the
review prompt — the `c407bb7e` reading, why both stale rows went stale, and the cost of each way of
equalising the three.

### `NewRootCmd` writes package globals

**The decision: whether `NewRootCmd` returns a command bound to *fresh* per-call storage, or whether
the package globals become explicitly single-writer with the constructor refusing a second call.** The
first is the real repair and is a wider change than it looks — it changes how every flag's default is
read; the second is a fence and would have caught the instance below on the first parallel test rather
than the thirtieth. Neither has been taken.

**Provenance.** Recorded in `spec/backlog/gate-sequence.md` as a gate step that is green on the first
run and red on the second, and moved here because that spec takes no decision on it and no row of its
tests depends on it. The gate-defect argument stays there and is not re-made here.

**The instance, in two sentences.** Two parallel tests that each construct a root command report **30
races per run** under `-race`, because `NewRootCmd()` binds tp's package-level flag variables through
pflag. It was **closed test-side, not at the source**: `3b9204e1` added a mutex-guarded
`newRootCmdForTest()` and the races went to zero, leaving the design smell — a constructor that writes
package globals — for every future parallel test to inherit.

**Is this only a test problem? Answered by counting rather than left open.** At `13bfde30`,
`NewRootCmd` is defined in `internal/cli/root.go` and called from exactly one production site,
`Execute()` in the same file, which `cmd/tp/main.go` calls once — so today it is test-only. What makes
it a design defect rather than a test defect is that **nothing says so**: a second in-process caller
would share the flag variables silently, and would find out the way the gate did.

**Where the measurements are.** `spec/undecided-measurements.md` §`NewRootCmd` writes package globals.

### `t.Parallel()` in the engine package

**The decision: whether `internal/engine`'s tests may be marked `t.Parallel()` — and the only answer
that will be accepted is a paired gremlins run, `--workers` pinned, before and after, compared on
efficacy and the timeout count rather than on wall time.** Until that measurement exists,
`internal/cli` is free to parallelize and `internal/engine` is not. It was carried in
`spec/backlog/mutation-run-check.md` as an open question and moved here because that check takes no
decision on it and no row of its tests depends on it.

**The undecided part is `internal/engine` alone**, for one reason: that is the package
`gremlins unleash ./internal/engine` mutates. gremlins runs the mutated package's own tests once per
mutant, those runs are already measured as load-sensitive, and `t.Parallel()` multiplies concurrency
*inside* each mutant by gremlins' own `--workers`. The `internal/cli` half is settled — `fa68051b`
applied it there — and the call count is a derivation, not a figure to quote:
`rg -c 't\.Parallel\(\)' -g '*_test.go' --no-filename | paste -sd+ | bc`.

**The protocol that paired run needs.** Each arm must be the **first** gremlins run in its own fresh
`rsync -a --exclude .git` copy — one for the serial tree, one for the parallelized tree, neither
directory reused. Without that the "after" arm is confounded by run order: it returns the corruption
signature and reads as a mutation signal `t.Parallel()` destroyed. Watching for `Lived: 0` beside
`Not covered > 0` is **not** a substitute — that signature is necessary under the corruption and not
sufficient, and the argv does not settle it either. The mechanism is unknown, and a protocol that
works without one is what the paired run needs.

**Where the measurements are.** `spec/backlog/mutation-run-check-measurements.md` under "Neither the
file nor the argv settles it"; `spec/undecided-measurements.md` §`t.Parallel()` in the engine package
holds the six-row re-derivation of the `internal/cli` figures, which audit the half this decision
excludes.

### A prior-round section for `tp review`

**The decision nobody has taken is whether `tp review` should have one at all.** Routed from
`spec/candidates.md`, whose forwarding table points here; `spec/backlog/repair-locality.md` names the
question and explicitly declines to decide it.

**What the two phases do, in one sentence each.** The audit phase hands a role its **own** prior
non-PASS rows and forces a commitment on each — `loadAuditPriorRound` (`internal/cli/audit.go`)
selects them, `renderPriorRoundSection` (`internal/cli/audit_roles.go`) renders them under *"Prior
Round: context to re-check, not a verdict to repeat"*. Review shows the whole panel everyone's rows
and asks for the opposite — `buildFindingsSummary` (`internal/cli/review.go`) emits `UNRESOLVED
findings from previous rounds — DO NOT re-report:`, panel-wide, capped and truncated.

**So the two phases differ in kind, not in presence.** A re-verification ask and a suppression ask are
not the same instrument, and this repository has already measured what an unexamined suppression costs
elsewhere in the loop.

**It settles** only on a measurement that does not exist: whether returning a reviewer its own prior
rows raises the share of findings sitting in text the round before wrote, or lowers it. A number
nobody has acted on yet is not a rule, and it is not an argument for a mechanism either.

**Where the measurements are.** `spec/undecided-measurements.md` §A prior-round section for
`tp review` — both mechanics in full, and why the repair-locality figures cannot settle it.

### A durable home for an accepted finding

**The decision: the target shape.** An audit finding has three ends — fix it, reject it, accept it as
backlog — tp records all three the same way in `.tp-review/<spec>/`, a directory archived at release
along with the spec. There is no supported path from *accepted* to something a maintainer trips over
later. Three target shapes were named and none chosen.

**What is already agreed, and is the property that decides between them**: the target must be readable
by the next cycle's decomposition without a human remembering it exists.

**It would extend `spec/backlog/a-finding-can-leave-an-audit-round.md`**, which ships only the
stops-blocking half of the same subject: that release makes an accepted finding stop gating a round
and says nothing about where it then lives.

**Where the measurements are.** `spec/undecided-measurements.md` §From the rows spec — the three
options as they survive in `git show 3a83be30:spec/0.41.0.md` §2.

### Making `severity` checkable

**The decision: what could check a severity that is not the row's author.** A non-`PASS` row's severity
is self-declared: the prompt renders the requirement and nothing validates what comes back.
`invalidCategoryRows` in `internal/cli/audit_record.go` validates **`category`** alone — that
asymmetry is the current behaviour this entry pins. Two mechanisms were drafted and both withdrawn
within a round: a `--record` rejection inverts the category sink's own precedent, and would refuse a
measurable share of this repository's recorded round files.

Counting rule for that share — a round file holding at least one row whose `severity` is present and
outside its phase's vocabulary:

```bash
python3 - <<'PY'
import json,glob
def scan(globs, vocab):
    files=[f for g in globs for f in sorted(glob.glob(g))]
    bad=[f for f in files if any(json.loads(l).get("severity") and
         json.loads(l).get("severity") not in vocab for l in open(f) if l.strip())]
    return len(files), len(bad)
print("audit round files %d, off-vocabulary %d" % scan(
    ["spec/.tp-review/*/audit-round-*.ndjson","spec/backlog/.tp-review/*/audit-round-*.ndjson"],
    {"error","warning","info"}))
print("review round files %d, off-vocabulary %d" % scan(
    ["spec/.tp-review/*/review-round-*.ndjson","spec/backlog/.tp-review/*/review-round-*.ndjson"],
    {"critical","high","medium","low"}))
PY
```

**Its acceptance channel is `spec/backlog/a-finding-can-leave-an-audit-round.md`**, whose test row 1b
grades acceptance from the row's `severity` under `audit_converge_on: blocking` — so that release
makes severity load-bearing without making it checkable. `spec/backlog/refusals-that-name-nothing.md`
is the secondary reader.

**Where the measurements are.** `spec/undecided-measurements.md` §From the rows spec — the category
sink's early return and the test that pins it, the corrected "nothing reads the field" clause, and the
fifteen offending round files by cycle.

### An audit-side `nonblocking_open`

**The decision: whether to invert a guard that pins the key's absence.** Under
`audit_converge_on=blocking` a clean round can carry `warning` and `info` rows, so the audit phase now
has the accepted-open state the review-side field was built to make visible. The count is emitted;
what is missing is the breakdown. `engine.RoleStreak`'s `Open` (`internal/engine/rolestreaks.go`) is
that count and it is **severity-blind** — documented there as the role's non-PASS row count in the
latest round, with no reference to severity — and it reaches the payloads through `auditSignalFields`
in `internal/cli/audit_record.go`. Four places pin the key's absence by name and a fifth states it in
a comment; no release has decided to invert any of them.

**It is unowned.** `nonblocking_open` appears nowhere under `spec/backlog/` — a regex sweep of every
file there returns zero matches — so no pending spec proposes, forbids or schedules this. It is the
only one of these four questions with no adjacent release at all.

**Where the measurements are.** `spec/undecided-measurements.md` §From the rows spec — the four
pinning places and the fifth comment, each cited by symbol or by phrase rather than by line.

### A review-side `accepted_blocking`

**The decision: whether an accepted blocking finding gets a counter of its own.** The review side
already emits `nonblocking_open` and it fires only for the case that does not matter: a probe on three
one-round trees found a round whose only finding is a `critical` resolved `wontfix` returning a
payload whose **key set is identical** to a round recorded from an empty findings file, while a round
holding one open `medium` gains a key. The surface announces the harmless case and is silent on the
one a reader would want stopped at.

**Where it would sit.** `spec/backlog/a-findings-exits-agree.md` §2 rewrites `unresolved_findings` on
exactly the payload an `accepted_blocking` count would join, and gives it three siblings bound by an
identity — so that release fixes the shape of the payload without adding this counter to it.

**Where the measurements are.** `spec/undecided-measurements.md` §From the rows spec — the three-tree
probe at `5058fc99`, with each tree's `clean`, `consecutive_clean` and key-set difference.

### The scope pass — whether a section should exist

**The decision: whether to build a scope pass at the front of the cycle, and what signal it would
run on.** Ground and review both presuppose the spec's scope; neither can ask whether a section should
exist. Three internal signals were tested and all three failed — finding density is uninformative in
both directions, cross-role agreement is a coin flip, goal-entailment self-confirms when §1 was written
in the same sitting. A forensics trim was the first proposal and was refuted by a field report that
classified its own diffs. The measurements, the refuted reading and the open questions are in
`spec/trim-pass.md`, which is this entry's measurements file and not a backlog spec.

### `scope` on audit rows

**The decision: whether an audit row carries `scope: spec | codebase`, and who assigns it.** Three
shapes have been named and none costed — a second role classifying independently, a mechanical rule
derived from the checklist item rather than from the finding, or an operator confirmation per label.
The objection that has deferred it four times: `scope` is a judgement made by the very sub-agent that
wrote the row, so one row mislabelled `codebase` lets a genuine spec violation ship — strictly worse
than today's rule of over-counting and wasting rounds.

**Deferred four times, each time on the record**: v0.33.0 Non-Goal 1, `spec/0.34.0-candidates.md` item
1, `spec/0.35.0-candidates.md` item 14, and `spec/0.37.0.md` §2/§5. Current behaviour is pinned by the
row schema in `internal/cli/audit_schema.go`, which declares `category` and `severity` and no `scope`.

**One premise it was carried on has since been falsified.** It was paired with `audit_converge_on` on
the argument that the knob depends on a scope label; v0.37.0 shipped that knob keyed on **`severity`**
instead, so the two are no longer one decision.

**It unblocks two backlog specs.** `spec/backlog/round-knows-its-panel.md` §4a scopes convergence by
**role** (`audit_converge_roles`); a trustworthy `scope` is the field convergence would be scoped on
instead. `spec/backlog/a-finding-can-leave-an-audit-round.md` is the other: an accepted finding's
scope is what a maintainer needs when the acceptance is re-read. It sits beside *Making `severity`
checkable* above, and for the same reason — both ask what makes a self-declared label trustworthy
enough to gate on.

### Cross-repo specs

**The decision: whether a spec may name tasks in another repository at all, and if so where its task
file lives.** Routed from `spec/0.33.0-candidates.md` item 7, which records that at least one team
already works this way — plan in one repo, run agents in both — and that the file-selection and
`commit_shas` paths both assume one repo root.

**No design pass exists.** The only adjacent statement anywhere in the corpus is a non-goal:
`spec/0.31.2.md` names *"Cross-repository task execution"* and says a per-task working directory and
quality gate are a separate release. That defers the work; it does not answer either half of the
decision.

**It reopens** when a field cycle asks for it. Nothing in this repository's own use exercises it, so
prototyping here would measure a corpus of one that does not need the feature.

### Which channel a degraded scan reports on

**The decision: one rule for the channel a degraded result reports on.** Two repairs of the same defect
class — a scan that could not be performed reporting as one that came back clean — chose different
channels, and both shipped. `internal/cli/validate_project.go` reports an incomplete scan on **both**:
an `output.Notice` on stderr *and* a `skipped` entry in the payload. `runStateOf` in
`internal/cli/run_status.go` reports an unprobeable run lock as a **notice only**, returning
`in_flight` with no payload field — so under `--quiet` it is indistinguishable from a live run.

**Routed from `spec/0.35.0-candidates.md` item 17**, which measured the second half by making
`.tp/locks` a regular file and running `tp run --status --quiet`.

**It settles with *The evidence contract* above**, which owns the general question of which channel a
loop signal travels on; deciding this one alone would fix a rule for two call sites while leaving the
principle unstated.

---

## Fog — in scope, not yet sharp enough to state as a question

**The test is whether the question can be stated precisely now, not whether it can be answered now.**
An entry here is coarser than the entries above it: one may graduate into several questions, or none,
once the frontier reaches it. Keeping the two apart stops a half-seen problem from being pre-sliced
into confident-looking entries it does not yet deserve.

- **Whether the emission's prohibitions are worth sweeping.** 14% of the review prompt and 12% of
  `CLAUDE.md` steer by ban. A few of those are hard guardrails that earn it. No measurement separates
  the two, and a rewrite without one is prose churn.
- **No instrument exists for a duplicated Go doc comment.** tp's own `duplicate-paragraph` lint
  reports exactly this shape in a spec and cannot see it in the language tp is written in: `dupl`
  reads tokens rather than comments, and no other enabled linter looks at comment text. Routed from
  `spec/0.35.0-candidates.md` item 12, which found a live instance in `internal/model/io.go` that had
  shipped since v0.29.0. What is not yet statable is whether the subject is comments, doc comments, or
  any duplicated span a token-based linter cannot see.
- **No sentence-level "two homes" instrument exists.** The count that scores how much one statement is
  duplicated across two documents reads **headings**, while the duplications it is meant to find live
  in bullets inside sections both documents legitimately keep — so cutting twelve duplicated statements
  moved the figure by one. Routed from `spec/0.35.0-candidates.md` item 10. The number is a floor and a
  weak one; a sentence-level comparison is what actually found the duplicates, and what a future check
  should run. What is not yet statable is the unit it would compare and what makes two sentences the
  same statement.
