# Undecided — and refuted

**Not a release, and not a spec.** Three kinds of thing live here, and the distinction is the point:

- **Refuted** — a candidate that was prototyped and did not survive. It is recorded so it is not
  re-proposed, with the measurement that killed it. **A refuted predicate is not a backlog item.**
- **Decided — routed to a pending spec** — a decision was taken, and a backlog spec or its sidecar now
  carries it. The entry states the decision, names the carrier, and keeps the counting rules and
  measurement pointer it was registered with.
- **Decided — closed** — a decision was taken and nothing carries it forward, because the answer is
  *no* or *not yet worth it*. Each names why, and the condition that would reopen it.

**The 2026-09-08 decision pass.** Every entry that stood under *Undecided* and *Survived, unscheduled*
was decided that day, the last of them (*Cross-repo specs*) by the operator. The decision text was appended to each named backlog sidecar under *Decided at
the 2026-09-08 decision pass*, and the decisions with no pending spec are listed in
`spec/backlog/README.md` under *Decided, awaiting a spec*. This file records the decision; the spec
that takes it is where it is implemented.

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

## Decided — routed to a pending spec

**A decision pass was taken on 2026-09-08.** Each entry below states the decision, names the backlog
spec or sidecar that now carries it, and keeps the counting rules and measurement pointers it was
registered with. The decision text was appended to each named sidecar under *Decided at the
2026-09-08 decision pass*, so the spec that implements it does not have to come back here.

### The divisible round

**Decided: the split key is spec location — the section.** A round is divided into shards by section,
each shard one prompt carrying that section's checklist items; per-item convergence is unchanged. It
is deliberately **not** folded into `spec/backlog/checklist-covers-what-changed.md`, because that
would double a spec already ranked second; it is a follow-on tool spec, *round-divides-by-section*,
to be written after that one ships.

**Who carries it.** `spec/backlog/checklist-covers-what-changed-measurements.md`, *Decided at the
2026-09-08 decision pass*; and `spec/backlog/README.md` under *Decided, awaiting a spec*, because the
follow-on spec has no file yet.

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
two shards that are each overwhelmingly `PASS` and neither tells a reader where the findings are —
which is the measurement the decision rests on.

**Where the measurements are.** `spec/undecided-measurements.md` §The divisible round — the sub-claim
that did not reproduce and why both readings of it strengthen the conclusion.

### A registered check that outlives its release

**Decided: `checks[].cmd` gains `{spec}` and `{round}` substitution and nothing else**, and a check's
**exit code is a contract tp defines for its own registrations** — `0` passed, `1` found violations,
`2` or higher cannot run. A check that cannot run neither passes nor suppresses its class: it is
reported as `ran: false`. Defining the status this way is legitimate here where it was not for
`gocognit`, because these commands are tp's own registrations rather than a third-party tool's
convention that tp merely observes.

**Who carries it.** `spec/backlog/next-action-and-check-tell-the-truth.md` — its subject is checks
telling the truth; the decision is appended to its sidecar.

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

**The cannot-run half, measured.** `internal/engine/mechanized.go:34-36` states the governing
principle — an entry tp will never run is not evidence that its class is mechanically checked — and
applies it to schema validity alone, so a check that is schema-valid and fails to execute suppresses
its class anyway: measured at `c75e5c3d` in a clone with `{"class":"my-broken-class","cmd":"exit 2"}`
registered at the project layer, one `tp review <spec>` emission reports
`mechanical_checks: [{… "exit_code": 2, "passed": false}]` and stamps
`do NOT report findings of these classes: my-broken-class` into all four role prompts in the same
payload. That is the behaviour the exit-code contract above ends.

**Where the measurements are.** `spec/undecided-measurements.md` §A registered check that outlives its
release — the two measurements that were false as written, and the per-task-file registrations.

### Cross-site key agreement in the review prompt

**Decided: the review prompt renders one set at all three sites, from one Go constant.** The set is
the record-required four — `severity`, `finding`, `location`, `evidence` — plus `role` and `class`,
both **mandatory**: `class` is the dedup key, so *Optional* was a fiction. `category` is kept, because
`by_category` ships. The review-side `category` enum is validated at the record sink the way the audit
side already is, as a **warning**-severity refusal that names the row. The audit phase keeps its own
status-based vocabulary, stated once.

**Who carries it.** `spec/backlog/a-findings-exits-agree.md`; the decision is appended to its sidecar.

**The three sites, read at `13bfde30`** (the reading recorded at `c407bb7e` predates two changes to
them):

| site | keys it names |
|---|---|
| `findingFormat`'s JSON example (`internal/cli/review.go`) | `severity`, `category`, `location`, `finding`, `evidence`, `suggestion` |
| the optional-`class` sentence beside it | `class` |
| `outputContractInstruction`, review branch | `role`, `location`, `class`, `severity`, `evidence` |

`evidence` is common to the first and third (it reached the contract block *after* `a4fd187f`); what
still differs is `category`, `finding` and `suggestion` in the example alone, against `role` and
`class` in the contract alone. `outputContractInstruction` is shared with the audit phase, which is
why the decision states the audit vocabulary separately rather than equalising the two phases.

**Routed from `spec/0.35.0-candidates.md` item 13 and settled by the same decision**: the review-side
`category` enum is a bare string literal inside the prompt template, with no constant, no validator
and no sink check, while the audit side declares typed constants with a validator
(`internal/engine/audit_category.go`) and rejects anything else at the record sink.

**Where the measurements are.** `spec/undecided-measurements.md` §Cross-site key agreement in the
review prompt — the `c407bb7e` reading, why both stale rows went stale, and the cost of each way of
equalising the three.

### `NewRootCmd` writes package globals

**Decided: the fence, not the rewrite.** The constructor refuses a second in-process call — a guarded
once, with a panic that names the caller — and fresh per-call storage waits until a second production
caller exists. The rewrite is the wider change (it moves how every flag's default is read) and buys
nothing today; the fence would have caught the instance below on the first parallel test rather than
the thirtieth.

**Who carries it.** `spec/backlog/gate-sequence.md`, as a task; the decision is appended to its
sidecar. The gate-defect argument stays in that spec and is not re-made here.

**The instance, in two sentences.** Two parallel tests that each construct a root command report **30
races per run** under `-race`, because `NewRootCmd()` binds tp's package-level flag variables through
pflag. It was **closed test-side, not at the source**: `3b9204e1` added a mutex-guarded
`newRootCmdForTest()` and the races went to zero, leaving the design smell — a constructor that writes
package globals — for every future parallel test to inherit.

**Is this only a test problem? Answered by counting rather than left open.** At `13bfde30`,
`NewRootCmd` is defined in `internal/cli/root.go` and called from exactly one production site,
`Execute()` in the same file, which `cmd/tp/main.go` calls once — so today it is test-only. What makes
it a design defect rather than a test defect is that **nothing says so**: a second in-process caller
would share the flag variables silently. The fence is what says so.

**Where the measurements are.** `spec/undecided-measurements.md` §`NewRootCmd` writes package globals.

### `t.Parallel()` in the engine package

**Decided: `internal/engine` stays serial.** The paired gremlins run that could change that — a fresh
`rsync` copy per arm, `--workers` pinned, compared on efficacy and the timeout count rather than on
wall time — is the **first execution** of `spec/backlog/mutation-run-check.md`, which is when it is
cheapest to run and when someone is already reading gremlins output. `internal/cli` stays free to
parallelize; `fa68051b` applied it there.

**Who carries it.** `spec/backlog/mutation-run-check-measurements.md`, *Decided at the 2026-09-08
decision pass*.

**Why the undecided part was `internal/engine` alone**: that is the package
`gremlins unleash ./internal/engine` mutates. gremlins runs the mutated package's own tests once per
mutant, those runs are already measured as load-sensitive, and `t.Parallel()` multiplies concurrency
*inside* each mutant by gremlins' own `--workers`. The call count is a derivation, not a figure to
quote: `rg -c 't\.Parallel\(\)' -g '*_test.go' --no-filename | paste -sd+ | bc`.

**The protocol that paired run needs.** Each arm must be the **first** gremlins run in its own fresh
`rsync -a --exclude .git` copy — one for the serial tree, one for the parallelized tree, neither
directory reused. Without that the "after" arm is confounded by run order: it returns the corruption
signature and reads as a mutation signal `t.Parallel()` destroyed. Watching for `Lived: 0` beside
`Not covered > 0` is **not** a substitute — that signature is necessary under the corruption and not
sufficient, and the argv does not settle it either.

**Where the measurements are.** `spec/backlog/mutation-run-check-measurements.md` under "Neither the
file nor the argv settles it"; `spec/undecided-measurements.md` §`t.Parallel()` in the engine package
holds the six-row re-derivation of the `internal/cli` figures, which audit the half this decision
excludes.

### A sentence rewritten in answer to a finding is exempt from the cut for one round

**Decided: no exemption.** `tp ground --status` reports a `cut` **delta** per round — units cut that
were rewritten since the previous round — so a repair is visible once without being graded twice. The
exemption was the expensive form of the same want; the delta is the cheap one and does not touch the
floor's arms.

**Who carries it.** `spec/backlog/ground-command-friction.md`; the decision is appended to its
sidecar.

**Where the measurements are.** `spec/backlog/ground-command-friction-measurements.md` under "§11.1"
carries the three instances; `spec/undecided-measurements.md` §A sentence rewritten in answer to a
finding is exempt from the cut for one round summarises why the cheaper form is the one to cost first.

### Claim enumeration in the grounding floor

**Decided: the floor's own arms define a claim.** Intuition counts — 11 where a spec carried 17, and
10 where another carried 17 again after a second read — are not a measurement and are retired. The one
measured leftover, a bare ordered-list marker becoming a floor unit, is
`spec/backlog/ground-command-friction.md` §5 and stays there.

**Who carries it.** `spec/backlog/ground-command-friction-measurements.md` §5 keeps the measured
piece; the decision is appended to that sidecar as a one-line note.

### A durable home for an accepted finding

**Decided: a repository-level `.tp/accepted.ndjson`**, appended by the audit resolve that accepts,
surfaced by `tp resume` and `tp status` as `accepted_open` until a task file's `covered_by` names the
finding id. That satisfies the property which decides between the three shapes and was already
agreed: the target must be readable by the next cycle's decomposition without a human remembering it
exists.

**Who carries it.** `spec/backlog/a-finding-can-leave-an-audit-round.md`, which ships the
stops-blocking half of the same subject; the decision is appended to its sidecar.

**Where the measurements are.** `spec/undecided-measurements.md` §From the rows spec — the three
options as they survive in `git show 3a83be30:spec/0.41.0.md` §2.

### An audit-side `nonblocking_open`

**Decided: emit it**, and invert the guards that pin the key's absence — under
`audit_converge_on: blocking` only. Under that setting a clean round can carry `warning` and `info`
rows, so the audit phase has the accepted-open state the review-side field was built to make visible;
the count is emitted and only the breakdown is missing.

**Who carries it.** `spec/backlog/a-finding-can-leave-an-audit-round.md`; the decision is appended to
its sidecar. It was the only one of these questions with no adjacent release; it has one now.

`engine.RoleStreak`'s `Open` (`internal/engine/rolestreaks.go`) is the count that exists today and it
is **severity-blind** — documented there as the role's non-PASS row count in the latest round, with no
reference to severity — reaching the payloads through `auditSignalFields` in
`internal/cli/audit_record.go`. Four places pin the key's absence by name and a fifth states it in a
comment; those five are what the decision inverts.

**Where the measurements are.** `spec/undecided-measurements.md` §From the rows spec — the four
pinning places and the fifth comment, each cited by symbol or by phrase rather than by line.

### A review-side `accepted_blocking`

**Decided: one counter**, on the payload `spec/backlog/a-findings-exits-agree.md` §2 already rewrites.
That release fixes the shape of `unresolved_findings` and gives it three siblings bound by an
identity; the accepted-blocking count joins them there rather than arriving on its own.

**Who carries it.** `spec/backlog/a-findings-exits-agree.md`; the decision is appended to its sidecar.

**The gap it closes.** The review side already emits `nonblocking_open` and it fires only for the case
that does not matter: a probe on three one-round trees found a round whose only finding is a
`critical` resolved `wontfix` returning a payload whose **key set is identical** to a round recorded
from an empty findings file, while a round holding one open `medium` gains a key. The surface
announces the harmless case and is silent on the one a reader would want stopped at.

**Where the measurements are.** `spec/undecided-measurements.md` §From the rows spec — the three-tree
probe at `5058fc99`, with each tree's `clean`, `consecutive_clean` and key-set difference.

### Making `severity` checkable

**Decided: severity stays self-declared.** What makes it trustworthy enough to gate on is the forced
commitment in the brief, not a validator — the mechanism this repository has measured six times. What
*is* validated is the **vocabulary**, at the record sink, as a **warning** that names the row rather
than a rejection: fifteen of one hundred twelve audit round files are off-vocabulary today, so a
rejection would refuse history.

**Who carries it.** `spec/backlog/refusals-that-name-nothing.md` — a refusal that names the row is its
subject; the decision is appended to its sidecar.
`spec/backlog/a-finding-can-leave-an-audit-round.md` is the reader that makes severity load-bearing,
through its test row 1b grading acceptance from the row's `severity` under `audit_converge_on:
blocking`.

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

`invalidCategoryRows` in `internal/cli/audit_record.go` validates **`category`** alone today; the
asymmetry this entry pinned is what the warning-severity vocabulary check removes.

**Where the measurements are.** `spec/undecided-measurements.md` §From the rows spec — the category
sink's early return and the test that pins it, the corrected "nothing reads the field" clause, and the
fifteen offending round files by cycle.

### Which channel a degraded scan reports on

**Decided: a payload field always, because it survives `--quiet` and a driver reads payloads; the
notice is additional.** `runStateOf` in `internal/cli/run_status.go` gains a `lock_unreadable` payload
field. This also answers the channel question *The evidence contract* was holding, so no release has
to state the general principle before this one is fixed.

**Who carries it.** `spec/backlog/two-advisories.md`; the decision is appended to its sidecar.

**The two repairs that disagreed.** `internal/cli/validate_project.go` reports an incomplete scan on
**both** channels: an `output.Notice` on stderr *and* a `skipped` entry in the payload. `runStateOf`
reports an unprobeable run lock as a **notice only**, returning `in_flight` with no payload field — so
under `--quiet` it is indistinguishable from a live run. Routed from `spec/0.35.0-candidates.md` item
17, which measured the second half by making `.tp/locks` a regular file and running
`tp run --status --quiet`.

### The test-file fence and the write-deny fence's reach

**Decided together, as one small tool spec — `hooks-fence-the-target` — which has no file yet.** Three
parts, one subject: the write-deny hook matches on the tool's write **target** (the file path argument
of a write-capable tool), not on any argument string, so reads and batched multi-file calls stop being
refused; the test-file fence resolves its permission **precomputed into the child environment at
spawn**, never a `tp` call per write inside the hook; and `test_globs` follows `pickChecks` — a present
list replaces the layer beneath it rather than merging with it.

**Who carries it.** `spec/backlog/README.md` under *Decided, awaiting a spec*, until the spec is
written.

**The reach, re-derived by running the hook rather than by reading it.** `denied()` in
`hooks/pre-tool-use-write-deny.sh` matches `*/.tp-review/?* | .tp-review/?*`, an unanchored glob. Fed
a `Write` payload naming `/private/tmp/throwaway-copy/spec/.tp-review/1.0.0/state.json` — a path in no
repository at all — the hook prints its scope-fence message and exits **2**. During the 2026-09-08
pass the installed hook also refused two **read-only** MCP calls whose payloads merely *named*
`.tp/config.json` and `spec/0.25.0.tasks.json`, one of them a batch whose other five operations touched
neither. So the fence matches on any argument string in the payload: it reaches reads, and one fenced
path in a batched multi-file call refuses the whole call. Matching on the write target closes both
halves at once, and leaves the anchor question answerable against a target rather than against a
payload — which is why the anchor (`CLAUDE_PROJECT_DIR`, a git-root walk, or nothing) is not decided
here.

**The list-layer precedent, for `test_globs`.** `pickChecks` in `internal/engine/configresolve.go`
returns the first present layer and stops, so a present slice — an explicit empty array included —
replaces the layer beneath it. Its own doc comment says so, and the pointer-to-slice type exists for
that reason: `Checks *[]Check` in `model.WorkflowOverride` is the **only** list-typed override field in
the struct, every other field there being a scalar pointer or raw JSON. `test_globs` becomes the
second, and follows it.

**The reach is not a defect, which is why the anchor is deferred rather than answered.** The fence is
fail-closed and correct where it is meant to apply, and the reach costs a workaround rather than a
wrong result. A wrong anchor is the other direction: a fence that silently stops fencing, a worse
failure than the one it removes.

### Inferring a spec's class

**Decided: `tp lint` reports a derived `class`** beside `floor_size` and `review_panel` — no gate, and
no frontmatter override until one is argued for. It is scheduled with the next release that touches
lint's report, rather than given a release of its own.

**Who carries it.** `spec/backlog/README.md` under *Decided, awaiting a spec*, as a one-line row.

**The claim it settles.** A draft release proposed a `class: loop | tool` frontmatter field, declared
by the author, on the stated ground that *tp cannot infer it*. A unit told to construct a
counter-example built three predicates against 23 hand-labelled specs and **refuted the "cannot"**: a
density predicate — loop-lexicon occurrences per thousand words — scores 87% leave-one-out with zero
false positives, and it reproduces `CLAUDE.md`'s published loop/tool round medians exactly where two
independent hand-labellings do not. The lexicon is a new surface that will drift, which is why the
decision reports the class and gates nothing on it.

**Where the measurements are.** `spec/undecided-measurements.md` §Inferring a spec's class — the
correlations, why the frontmatter field was cut, and the three honest limits.

---

## Decided — closed

**Closed means no pending spec carries it and none is expected to.** Each entry names why, and the
condition that would reopen it. A reopen condition is a measurement, not an opinion.

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

**It reopens** only if `spec/backlog/repair-locality.md`, once shipped, shows the share of findings
sitting in repaired text **rising** with the suppression list on.

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

**Where the measurements are.** `spec/trim-pass.md` — this entry's measurements file, and not a
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
