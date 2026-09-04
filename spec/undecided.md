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

---

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

---

## Fog — in scope, not yet sharp enough to state as a question

**The test is whether the question can be stated precisely now, not whether it can be answered now.**
An entry here is coarser than the entries above it: one may graduate into several questions, or none,
once the frontier reaches it. Keeping the two apart stops a half-seen problem from being pre-sliced
into confident-looking entries it does not yet deserve.

- **Whether the emission's prohibitions are worth sweeping.** 14% of the review prompt and 12% of
  `CLAUDE.md` steer by ban. A few of those are hard guardrails that earn it. No measurement separates
  the two, and a rewrite without one is prose churn.
