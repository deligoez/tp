# tp v1.58.0 — `forward-spec-ref`

A spec must not name a spec numbered above itself that has not yet shipped.

This is the only lint-rule candidate in this repository that survived prototyping. Four others were
built and refuted, and their measurements are in `spec/candidates.md`. This one exists because a
reference to a pending spec is not wrong the day it is written — it is wrong the day that spec is
renumbered, and that has now happened five times.

## 1. Overview

**It is a rot predictor, not a correctness claim.** Every finding it reports resolves today. The sweep
after the fifth renumbering found **47 citations to renumbered specs, of which about 35 were stale** —
and not one was caught by checking that the target file exists, because every one of them resolved to
a file whose subject had moved. A rule that opens the target proves nothing; a rule that asks whether
the target can still move proves the thing that matters.

**The repository already has the rule in prose and it did not hold.** `CLAUDE.md` says: *name a
pending release by its subject, never by its filename — outside the roadmap table, which is the one
place a number belongs.* The sharpest instance recorded is `spec/0.36.0.md`'s own paragraph on this
hazard, which says *"naming a file that does not contain the question is worse than naming none"* and
carried two filenames a renumbering then falsified — **the rule refuting itself in place.**

**And it is still not holding.** Measured at HEAD, after the rule's own evidence was written:

```
python3 scripts/forward-spec-ref-prototype.py spec 1.0.0
```

reports **two** findings, both in `spec/1.0.0.md`, in a sentence that names the pending set by its
endpoints: *"the set is `spec/0.37.1.md` plus `spec/1.38.0.md`–`spec/1.53.0.md`"*. That sentence is
written by an author who knew the hazard — the same line supplies a `ls | grep` derivation as the
durable form — and it still carries two numbers that will rot when either endpoint moves. This is the
strongest argument the rule has: the prose rule is understood, is stated in the file that governs the
work, and is broken anyway, by the release that shipped last.

## 2. The predicate

For each spec file whose **name is a version**:

1. Take the referrer's version from the filename. A file whose name is not a version yields none and
   is skipped.
2. Find every `spec/<major>.<minor>.<patch>.md` reference in the text, **outside fenced blocks**.
3. Report the reference when the target is numbered **above the referrer** *and* **above the shipped
   boundary**.

Severity `warning`, rule name `forward-spec-ref`, matching the register of the twelve named rules the
engine already ships (`broken-cross-ref`, `duplicate-paragraph`, `numbering-gap`, …).

### 2.1 The shipped boundary is load-bearing, and a draft without it was refuted

*"Target numbered above the referrer"* alone looks sufficient in all four directions until it is run
for false positives. Measured at HEAD, the same prototype with the boundary clause disabled:

```
python3 scripts/forward-spec-ref-prototype.py spec 1.0.0 --no-boundary
```

returns **three** findings against the boundaried run's two. The one it adds is
`spec/0.36.0.md:566 -> spec/0.37.0.md` — a shipped spec naming a later spec that **has since
shipped**. Both ends are frozen; that reference cannot rot, and flagging it is noise.

| direction | at risk? | why |
|---|---|---|
| shipped → **pending** | **yes** | every renumber breaks it; this is the whole finding class |
| shipped → later **shipped** | no | both ends frozen — and this is the measured false positive |
| pending → *lower* pending | no | the target ships first and stops moving |
| pending → *higher* pending | **yes** | the target can still move |

One false positive in three findings fails this project's zero-false-positive bar, which is the bar
that refuted the example-table rule and the contradictory-comparator rule. The boundary clause is not
a refinement; without it the rule does not ship.

### 2.2 Fenced blocks, and why the live corpus cannot demonstrate this

The prototype's first version did not skip fences and, run over the renumbered corpus, flagged a
**transcript line** — a fixture spec named inside a shell session, not a reference to anything. The
code-citation checker already gets this right; a rule that flags example output teaches the reader to
skip it, which is worse than not running.

**State the limit rather than the evidence: at HEAD the fence-skip changes nothing.** Both runs return
the same two findings, because no spec currently quotes a `spec/<version>.md` string inside a fence.
So the fence rule is justified by the transcript instance and by `ParseHeadingsFromScanner`'s own
precedent (`internal/engine/lint.go:43-49` toggles on ```` ``` ````), not by a count on today's tree —
and its test is a constructed fixture, not the corpus.

### 2.3 What is *not* stated here, and why

**No count of violations.** Two recorded figures were re-derived for this spec and **neither
reproduced**:

| recorded in `spec/candidates.md` | re-derived | why it differs |
|---|---|---|
| *"18 violations pre-repair"* | **48** at `v0.37.0`, boundary `0.37.0` | that tree's specs carry pre-renumbering filenames (`0.48.0.md` naming `0.49.0.md`); the recorded 18 was a different corpus state, which the record does not name |
| *"0 today"* | **2** at HEAD, boundary `1.0.0` | `spec/1.0.0.md` was written after the figure was taken and introduced both |

A figure over a glob has two moving parts — the corpus and the boundary — and a record that fixes
neither is not checkable. The derivations above are the durable form. Any implementation of this rule
should expect the count to move and should not carry one in a comment.

## 3. Where the rule lives, and the git dependency

**This is the decision the spec exists to take.** All thirteen `Check*` functions in `internal/engine`
are pure functions of the spec's own text — `CheckBrokenCrossRefs(lines, headings)`,
`CheckSectionSize(headings, totalLines, maxSectionLines)` — and **not one receives a path**. The
repo-aware checks (`check-spec-code-citations.py`) live outside lint as `workflow.checks` scripts for
exactly that reason. This rule needs two strings the spec's text does not contain: the referrer's own
version, and the latest shipped version.

**The decision: the engine rule stays pure and the CLI supplies both strings.**

```go
func CheckForwardSpecRefs(lines []string, selfVersion, shippedBoundary string) []Finding
```

`runLint` derives `selfVersion` from the path it already holds and `shippedBoundary` from the newest
version tag. The impurity is one git read in the CLI layer, which is where tp already puts its I/O;
the engine keeps the property that makes its rules testable from a `[]string`.

**Degradation is silence, not a guess.** If either string is empty — a non-version filename, a tree
with no tags, a `git` that fails — the rule returns nothing. A boundary the caller guessed would
produce exactly the false-positive class §2.1 exists to remove.

### 3.1 Why not a `workflow.checks` script

It is the obvious alternative and it has a recorded failure mode. `engine.RunCommand` runs a
registered command **verbatim with no substitution of any kind**, so every registration hardcodes a
spec path — and every one has. `tp config --resolved` reports `checks: []` today, so
`code-citation-drift` has been unmechanized for four releases; during the two releases it *was*
registered, tp told every reviewer to stop reporting the class, and when the registration died with
the release nobody was told it had come back.

A rule whose subject is **the spec's own text** does not need that machinery, and putting it there
buys the suppression hazard for nothing. The code-citation check belongs outside lint because its
subject is the *code*; this rule's subject is the document lint already reads.

## 4. Non-Goals

1. **No cross-file section validation.** The obvious companion — flag `spec/X.md §Y` where X has no
   §Y — was prototyped and refuted: **its marginal yield over this rule is zero**, because every
   broken section reference in the corpus points at a higher-numbered spec, so the cheaper rule (which
   never opens the target) catches all of them. It also needed `spec/.tp-review/` excluded, since most
   of its raw hits are inside round snapshots — frozen photographs whose references were correct when
   taken. It is not refuted in principle; there is simply no instance of the case that would be its
   alone.
2. **The rule does not open the target file.** It does not check that the target exists, that it has
   the section named, or that its subject is what the referrer says. Every one of the ~35 stale
   citations resolved; resolution is not the signal.
3. **No git dependency beyond one boundary string.** Not a tag list, not a history walk, not a
   per-target shipped check. One string, resolved once per `tp lint` invocation.
4. **No autofix, no rewrite.** The remedy is a human sentence — name the release's subject — and a
   mechanical rewrite would produce the prose this rule exists to prevent.
5. **No effect on `tp lint`'s exit code beyond the existing severity contract.** `warning` behaves
   the way the twelve other named rules' warnings behave; this release does not introduce a new
   severity or a new gate.
6. **The roadmap table is not exempted by the rule.** `CLAUDE.md` is not a spec file and lint does not
   read it, so the one place a number legitimately belongs is out of scope by construction rather than
   by a carve-out. A carve-out inside the rule would be a second home for the convention.

## 5. Tests

Every row names the mutant that must fail it.

| # | from | assertion | the mutant that must fail it |
|---|---|---|---|
| 1 | §2 | a referrer at `1.0.0` naming `spec/1.38.0.md`, boundary `1.0.0`, is reported once with the reference's line number | return no finding — the shipped behaviour, since the rule does not exist |
| 2 | §2.1 | a referrer at `0.36.0` naming `spec/0.37.0.md`, boundary `1.0.0`, is **not** reported | drop the boundary comparison and keep only *target > referrer*, which is the measured false positive |
| 3 | §2.1 | the same pair with boundary `0.36.0` **is** reported | hard-code the boundary rather than taking it from the caller, which makes the rule's answer depend on when it was compiled |
| 4 | §2.2 | a reference inside a ```` ``` ```` fence is not reported, and the same reference outside one is | drop the fence toggle — the prototype's own defect, which flagged a shell transcript |
| 5 | §2.2 | a fence opened and never closed suppresses every later reference rather than panicking | treat an unterminated fence as closed at the next line, which re-admits the transcript case |
| 6 | §3 | a file named `candidates.md` or `0.35.0-candidates.md` yields no self-version and reports nothing | parse a version out of any leading digits, which makes `0.35.0-candidates.md` a referrer at `0.35.0` and flags its own recorded evidence |
| 7 | §3 | an empty `shippedBoundary` reports nothing, on input that would otherwise report | treat empty as `0.0.0`, which is §2.1's refuted draft reached through the degradation path |
| 8 | §3 | an empty `selfVersion` reports nothing | treat empty as `0.0.0`, which flags every reference in every non-version file |
| 9 | §2 | a reference to the referrer's **own** version is not reported | use `>=` rather than `>`, which flags a spec's own filename in its own header |
| 10 | §4.2 | a reference to a higher, unshipped spec that **does not exist on disk** is still reported | stat the target and skip when absent, which is the resolution check §4.2 forbids and which silently exempts exactly the references a renumbering has already broken |

**Row 10 is the one that decides whether the rule is worth shipping.** A reference that no longer
resolves is the *end state* of the rot; a reference that resolves to a file whose subject has moved is
the state this rule catches. An implementation that skips missing targets passes rows 1–9 and reports
nothing on the corpus this rule was written for.

**The corpus fixture is asserted, not assumed.** A test running the rule over `spec/*.md` must
`require` that the tree contains at least one version-named file and at least one non-version-named
file before asserting the skip behaviour — this repository has already lost a round to a guard whose
fixture could not distinguish the behaviour it was named for.
