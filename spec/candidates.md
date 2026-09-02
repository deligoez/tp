# Candidates — refuted, and undecided

Not a release. Two kinds of thing live here, and the distinction is the point:

- **Refuted** — a candidate that was prototyped and did not survive. It is recorded so it is not
  re-proposed, with the measurement that killed it. A refuted predicate is not a backlog item.
- **Undecided** — a real need whose *design* has no answer yet. Each names the decision nobody has
  taken. None carries a version number, because a number that moves leaves stale references and this
  repository has already paid for that three times.

---

## Refuted

### The unexecutable-split lint rule

**The rule it tried to mechanize is sound:** *a change and the test it invalidates belong to the same
task.* A graph that puts a change in one task and the test it breaks in the next validates cleanly and
cannot be executed — the gate runs at the earlier close and that close is already red.

**The predicate is refuted.** The proposed shape was *a task whose only work is tests, depending on
exactly one task, where both anchor to the same spec section*. Prototyped over this repository's own
**520 tasks in 26 task files**:

| form | fires on | true positives |
|---|---|---|
| depends on exactly 1 task sharing a `source_section` | **114 (21.9%)** | unusable at that rate |
| …and title or acceptance mentions a test | 11 | ~2 — the rest are tasks like *"Clear active pointer"* and *"Handle empty hoist result"*, matched because a tp acceptance almost always mentions its test |
| …restricted to `tags` containing `test` | **1** | **0** |

```
for each spec/*.tasks.json, for each task t with len(t.depends_on)==1:
  p = tasks[t.depends_on[0]]
  if set(t.source_sections) & set(p.source_sections): candidate
```

**The single tag-form hit is a false positive.** `mechanize-candidates-retained` (0.25.0) is tagged
`test` and depends on `per-role-overlap-report` in the same section, but it is not a test-only task —
its acceptance is *"keep the existing output exactly as-is, adding no per-round signal"* and its
closure reads *"No production change"*. It is a negative-requirement task whose deliverable is a
guard, and it executed without the gate ever going red.

**Two structural reasons it cannot be repaired by tightening:**

1. **`tags` is optional and sparse.** It is present on **185 of 520 tasks (36%)**, and `test` appears
   on **15**. A rule whose trigger depends on whether the decomposing agent happened to tag is not a
   check; it fires arbitrarily and its silence means nothing.
2. **Every stronger signal is post-hoc.** `commit_files` would identify a test-only task precisely,
   and it exists only after the task closes — `tp validate` runs at decomposition. This is the same
   wall the example-table rule's narrow form hit: it read data that does not exist when the check runs.

**One confound, stated rather than hidden.** tp's own task files are decomposed by an agent following
`CLAUDE.md`, which already carries the rule — so the absence of true positives may mean the rule is
already obeyed here rather than that the failure does not occur. That weakens "the failure never
happens"; it does not weaken "this predicate finds a false positive and no true one on 520 tasks",
which is what a zero-false-positive bar decides on.

**What could revive it:** a corpus from a project that does *not* follow the rule, or a required
task-kind field that makes "test-only" a fact rather than an inference. Neither exists today.

### The contradictory-comparator lint rule

**What it tried to catch:** a spec stating one numeric rule twice with opposite comparators — *more
than three sections* in one place, *at most three sections* in another.

**Its claimed true positive could not be reproduced, and its false positives are systematic.** The
proxy groups statements by `(number, noun)` and flags a group carrying both a greater-family and a
lesser-family comparator. Run over every `spec/*.md` at the `v0.36.0` and `v0.37.0` tags — both
states the candidate described as *before this cycle's repairs* — and over every historical revision
of the file the defect was said to be in:

| corpus | flagged | true positives |
|---|---|---|
| 46 spec files at `v0.36.0` | 2 | **0** |
| 46 spec files at `v0.37.0` | 2 | **0** |
| every recorded revision of the delta-pass spec | **0** | 0 |
| 47 spec files today, narrowed to same-section grouping | **0** | 0 |

**Both flags are the same false positive and it is not noise.** In `spec/0.31.0.md`,
`mechanize_candidates` needs *"at least two rounds"* and `harness_stale` is false with *"fewer than
two rounds"*. Two different rules sharing a number and a noun — which is what the candidate's own text
said the design work owed: *"a correct predicate has to establish that the two statements govern the
same subject."* No predicate was named, and none is available at lint time.

**The obvious narrowing removes the false positives by removing everything.** Grouping within a
section instead of a file flags **zero** across 47 files. A rule that has never fired on anything
cannot be shown to work.

**What could revive it:** a reproducible instance. The candidate asserted it *"caught the defect that
motivated it"*; that defect is not in any tagged state of the repository, so either it lived only in
an uncommitted draft or the prototype differed from the one described. Either way the claim is not
checkable, and a lint rule whose only evidence is unreproducible is not a release.

### The example-table lint rule

Refuted in both forms its spec named: the keyword-and-shape heuristic fires on **3.9–22.6%** of this
repository's 1,032 spec sections against a shipped bar of *zero* false positives, and the narrower
form reads task acceptance criteria — data that does not exist when `tp lint` runs, since lint is
workflow step 2 and decomposition is step 5.

### The corpus-replay gate, as a procedure

A gate phrased *"replay the recorded rounds in `spec/.tp-review/`"* cannot run: **35 recorded review
rounds carry a snapshot whose sha256 is not the `spec_hash` that round recorded**, so a replay
compares against the wrong text and reports clean.

**Two things about this were previously stated wrong and are corrected here.** Three specs withdrew it
citing a missing disposition; the disposition exists — **1,406 rows carry `resolved.status`, 1,308
`fixed` and 98 `wontfix`** — so that was never the reason. And the defect is bounded rather than
endemic: 19 of the 35 are v0.31.0's cycle alone and the most recent 27 rounds hold none.

**So the honest form is narrower than a withdrawal.** A replay *cannot gate the release that
introduces the mechanism it would test*, because it must wait for rounds to accumulate. The
replacement that works is a gate stated as an invariant checkable against a live tree — which is what
the release that pins the emit-time hash makes structurally possible.

---

## Survived prototyping — one lint rule

### `forward-spec-ref` — a spec must not name a spec numbered above itself

| corpus | violations |
|---|---|
| 51 spec files, pre-repair | **18** |
| the same today | **0** |

Every one of the eighteen was verified stale by hand during the review that found them, and each was
reworded to name the release's *subject* with no loss — so **a legitimate need to name a
higher-numbered file never arose, and the false-positive rate is zero by construction.**

**The predicate needs the shipped boundary, and a draft that claimed otherwise was falsified by a
stress test.** *"Target numbered above the referrer"* alone looked sufficient across four directions —
until it was run for false positives and produced one: `spec/0.36.0.md:566` names `spec/0.37.0.md`,
which has since shipped, so that reference is frozen at both ends and will never rot. Flagging it is
noise, and one false positive in nineteen fails this project's own zero-FP bar.

| direction | at risk? | instances |
|---|---|---|
| shipped → **pending** | **yes** — every renumber breaks it | 18 |
| shipped → later **shipped** | no — both ends frozen | 1, and it is the false positive |
| pending → *lower* pending | no — the target ships first and stops moving | 2, both fine |
| pending → *higher* pending | **yes** — the target can still move | 0 |

So the rule is: **the target is numbered above the referrer *and* is not yet shipped.** 18 findings,
0 false positives.

**It still fits tp's lint architecture, but the fit is narrower than it first appeared.** All thirteen
existing rules are pure functions of the spec's own text — **not one receives a path** — and the
repo-aware checks (`check-spec-code-citations.py`) live outside lint as `workflow.checks` scripts for
exactly that reason. This rule stays pure only if the *caller* supplies two strings: the referrer's own
version (from the filename it already holds) and the latest shipped version. **The caller therefore
takes a git dependency `tp lint` does not have today**, which is the open cost, not a detail. A file
whose name is not a version (`candidates.md`, `0.35.0-candidates.md`) yields no self-version and is
skipped; a caller that cannot determine the shipped boundary reports nothing rather than guessing.

**It is a rot predictor, not a correctness claim.** A reference to a higher-numbered spec is not wrong
the day it is written; it is wrong the day that spec is renumbered, and that has now happened four
times.

### Refuted alongside it: `broken-cross-ref` extended across files

The obvious companion — flag `spec/X.md §Y` where X has no §Y — was prototyped and **its marginal
yield over `forward-spec-ref` is zero.**

| | |
|---|---|
| broken sectioned refs across both trees, snapshots included | **312** |
| of those, target ≤ referrer (the only case `forward-spec-ref` cannot see) | **0** |

Every broken section reference in the entire corpus points at a **higher-numbered** spec, so the
cheaper rule — which never opens the target file — catches all of them. The cross-file rule would also
have needed `spec/.tp-review/` excluded: 163 of its 173 raw hits are inside round snapshots, frozen
photographs whose references were correct when taken.

**It is not refuted in principle** — a mistyped section reference into a shipped spec would be its
alone. There is simply no instance of one in 312 findings, and a rule with no demonstrated
independent case does not ship.

---

## Undecided — each names the decision nobody has taken

| candidate | the decision |
|---|---|
| **the divisible round** | the split key. Measured: `spec-coverage` produced a non-`PASS` on **6 of 97 distinct items (6.2%)** across seven rounds, four of the six in one section — so splitting 74 items by *count* gives two ~97%-PASS shards. The key the data recommends is **spec location**, and none of the three candidates named so far is that |
| **the test-file fence** | where the permission resolves — a `tp` call per write inside the hook, or precomputed into the child environment at spawn. Also `test_globs`' list-layer semantics: `Checks` is the only existing list field and it *replaces* rather than merges |
| **a durable home for an accepted finding** | the target shape. Three options are named, none chosen |
| **the identifier pass** | the known-identifier set. Measured: the pass as written yields **979 findings over 46 specs**, all references to things that exist elsewhere in the repository, while its own Non-Goal forbids resolving them — so its zero-findings gate is unreachable |
| **class families** | its own yield. Measured over six cycles the normaliser groups **0–8.7% of findings, median 0.5%**; on v0.37.0 it forms **one family of three findings out of 630**. The release must state what result would make it worth shipping |
| **the evidence contract** | four sections name no field, no writer and no arithmetic. Its two ready pieces have already been lifted out as the forced-commitment brief and the mutation-score gate entry |
| **a prior-round section for `tp review`** | whether review should have one at all. The audit phase carries a role its own prior non-PASS rows; `tp review` carries nothing, and no measurement says which way that cuts |
| **a registered check that outlives its release** | whether `checks[].cmd` gains a substitution, and which. **Measured: `engine.RunCommand(c.Cmd, dir, …)` runs the command verbatim with no substitution of any kind**, so every registration must hardcode a spec path — and every one has. `.tp/config.json` registers nothing; `0.33.0.tasks.json` and `0.34.0.tasks.json` each register `code-citation-drift` against their own spec, `0.31.2.tasks.json` registers `test-inventory-drift` against its own. **`tp config --resolved` reports `checks: []` today**, so `code-citation-drift` has been unmechanized for four releases. This is the suppression hazard in its worst direction: during the two releases it *was* registered, tp told every reviewer to stop reporting the class, and when the registration died with the release nobody was told it had come back. A `{spec}` placeholder is the obvious fix and its scope — which placeholders, whether the round number is exposed — is the undecided part |
| **making `severity` checkable** | what could check it that is not the row's author. A non-`PASS` row's severity is self-declared — the prompt renders the requirement and nothing validates it, `audit_record.go` validates `category` alone, and nothing on the audit path reads the field. **Two mechanisms were drafted and both withdrawn within a round**: rejecting an out-of-enum severity at `--record` inverts its own precedent (`internal/cli/audit_record.go:282` returns early on an empty category, pinned deliberately by a test) and would refuse fifteen of this repository's own recorded round files; and validation cannot deliver what it was introduced for — **it makes a label well-formed, never truthful, and cannot make an unrun round run** |
| **an audit-side `nonblocking_open`** | whether to invert a guard that pins its absence. Under `blocking` a clean round can carry `warning` and `info` rows; the **count** is already emitted (`role_streaks[].open`, severity-blind, `internal/cli/audit_record.go:152`, on both `--status` and `--record`), so what is missing is the **breakdown**. Four places pin the key's absence by name — `spec/0.31.0.md` §8.4 and its test 18, `skills/tp/REFERENCE.md:641`, and `TestReviewNonBlockingOpen_AuditUnaffected` — and no release has decided to invert them. The `--merge` breakdown is a different surface and shipped separately |

---

## The pre-registered trial that has now run

`CLAUDE.md` used to carry a pre-registration inside a spec, which meant a review round re-read it every
round. The trial has run and its result belongs with the results:

**Forced commitment halves what a role files, and the arms swapped to prove it.** Briefs carrying
*provenance-and-count*, *do you still hold that reasoning*, and *say what should be done about it*
produced 15 and 23 findings against the control arm's **35 in both rounds** — the effect followed the
arm, not the role. Briefed roles **withdrew** findings after verifying by running, and did not file
the ones whose falsifier they could not name.

**What was measured is what a role files, not how many rounds a cycle takes.** The second has never
been measured and must not be claimed.
