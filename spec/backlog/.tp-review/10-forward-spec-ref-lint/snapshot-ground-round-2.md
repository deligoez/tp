# tp v1.58.0 — `forward-spec-ref`

A spec must not name a spec numbered above itself that has not yet shipped.

This is the only lint-rule candidate in this repository that survived prototyping. The ones that did
not are recorded in `spec/undecided.md` under `## Refuted`, each with the measurement that killed it
(`grep -n '^###' spec/undecided.md` between `## Refuted` and `## Undecided` enumerates them; the
count is not stated here because "lint-rule candidate" admits more than one reading and the earlier
draft of this sentence asserted one without giving it). This one exists because a reference to a
pending spec is not wrong the day it is written — it is wrong the day that spec is renumbered.

**The ordinal that used to close that sentence is withdrawn, and the withdrawal is the point.** An
earlier draft said the renumbering "has now happened five times". `CLAUDE.md` had already retired
that figure — *"The ordinal is not checkable and this file used to assert one"* — after a grounding
round found `CLAUDE.md` saying *fourth* where `spec/candidates.md` said *fifth*. What git records is
one renumbering: `git log --diff-filter=R --name-status -- 'spec/*.md'` at `e309df62` returns the
single commit `e73788ab` with 16 spec renames, and earlier renumberings either predate the tracked
names or were not recorded as renames. **One recorded renumbering with 16 renames is both true and
sufficient**; the rule's motivation never needed a count, and the count is the half that rots.

## 1. Overview

**It is a rot predictor, not a correctness claim.** Every finding it reports resolves today. The
cross-reference sweep `CLAUDE.md` records after the renumbering found **47 citations to renumbered
specs, of which about 35 were stale** — and not one was caught by checking that the target file
exists, because every one of them resolved to a file whose subject had moved. A rule that opens the
target proves nothing; a rule that asks whether the target can still move proves the thing that
matters.

**Those two figures are restated from `CLAUDE.md`, not re-derived, and they cannot be re-derived.**
The sweep repaired the citations it counted, so the corpus state it measured exists in no tagged
tree. What *is* re-derivable is the inference they support, and it holds independently: a variant of
the prototype that stats each target and skips the absent ones loses **zero** findings — 20 → 20 at
`e309df62` and 2 → 2 at `85a46682`, with an empty set difference in both directions. Resolution
discriminates nothing on this corpus, which is the whole of what non-goal 2 needs.

**The repository already has the rule in prose and it did not hold.** `CLAUDE.md` says: *name a
pending release by its subject, never by its filename — outside the roadmap table, which is the one
place a number belongs.* The sharpest instance recorded is `spec/0.36.0.md`'s own paragraph on this
hazard, which says *"naming a file that does not contain the question is worse than naming none"* and
carried two filenames a renumbering then falsified — **the rule refuting itself in place.**

**And it is still not holding.** Every figure in this spec names the tree state it was measured at,
because this rule's subject is a corpus that moves — and it moved three times in the day around the
spec being written. Run in a `git archive` tree of each state:

```
python3 scripts/forward-spec-ref-prototype.py spec 1.0.0
```

| tree state | findings | where |
|---|---|---|
| `85a46682` — the commit that added the prototype | **2** | both on `spec/1.0.0.md:409` |
| `020262d9` — the next morning | **11** | ten in `spec/1.0.1.md`, one in `spec/1.44.0.md` |
| `e309df62` | **20** | nineteen in `spec/1.0.1.md`, one in `spec/1.44.0.md` |

**Counting rule, used everywhere below: one finding per (referrer file, line, target) triple**, so a
line naming two targets counts twice.

At `85a46682` both findings were one sentence in `spec/1.0.0.md` naming the pending set by its
endpoints: *"the set is `spec/0.37.1.md` plus `spec/1.38.0.md`–`spec/1.53.0.md`"*. That sentence was
written by an author who knew the hazard — the same paragraph supplies a `ls | grep` derivation as
the durable form — and **it was repaired four hours later**, at `0c7e3cf9`, whose commit subject is
*"the pending set is derived, not enumerated — it grew past its endpoints"*. So the prose rule is
understood, is stated in the file that governs the work, is broken anyway by the release that shipped
last, and the breakage is repaired within a working day once someone looks. That is the strongest
argument the rule has, and it is a confirmed prediction rather than a live count — which is why it
survives the count moving.

### 1.1 The count nearly doubled mid-repair, and the twenty are not false positives

At `e309df62` nineteen of the twenty findings are in `spec/1.0.1.md`, a spec whose subject is four
measured defects in `tp ground` and which names, by filename, the pending releases that take the
related work. Ten arrived when that file was created; the rest arrived as it was repaired. **The
rule's violation count nearly doubled because this repository, mid-repair, did the thing the rule
forbids.** Four dispositions were considered and the release takes the fourth.

1. *Exempt a reference that names the release which fixes the thing being described.* Rejected: the
   rule receives `[]string` and has no model of what a sentence is about, so the exemption is not
   expressible in the predicate. Every implementable form is a marker in the text — a second
   convention, whose only enforcement would be the convention it exists to enforce.
2. *Exempt test-table rows.* Rejected on that ground and a worse one: a syntactic carve-out keyed on
   a leading `|` would exempt exactly the rows most likely to be copied forward into the next spec,
   which is how a filename outlives the sentence that justified it.
3. *Move the boundary.* Rejected: §2.1 measures that a boundary below the newest tag re-admits the
   refuted draft, and §3 measures that one above it silences the rule entirely.
4. **Ship with the count as measured, and treat the twenty as true positives.** Under §2's own table
   they are *pending → higher pending*: every one rots if either end moves, and one of them —
   `spec/1.0.1.md:282`, enumerating *"twenty files `spec/1.38.0.md` through `spec/1.58.0.md`"* — is
   the endpoint form this rule was written for. The remedy is non-goal 4's human sentence, and
   `0c7e3cf9` is the worked example of it.

**Twenty findings measure compliance, not imprecision.** A false-positive bar is a claim about
findings that cannot rot, and §2.1's single measured false positive is still the only one anyone has
produced. Severity `warning` (non-goal 5) means the number gates nothing while it is worked down.

## 2. The predicate

For each spec file whose **name is a version**:

1. Take the referrer's version from the filename. A file whose name is not a version yields none and
   is skipped.
2. Find every `spec/<major>.<minor>.<patch>.md` reference in the text, **outside fenced blocks**.
3. Report the reference when the target is numbered **above the referrer** *and* **above the shipped
   boundary**.

Severity `warning`, rule name `forward-spec-ref` — a hyphenated slug in the register the engine
already uses (`broken-cross-ref`, `duplicate-paragraph`, `numbering-gap`). **No count of the existing
rules is stated here**, because the earlier draft's *twelve* reproduces under no rule anyone has
offered. At `e309df62`: `grep -rhE '^func Check[A-Za-z]*\(' internal/engine --include='*.go' | grep
-v _test` returns **13** declarations; `awk -F'|' '/\| `tp lint` \|/ {print $2, $4}' README.md`
documents **16** rule identifiers `tp lint` can emit; **9** of those carry `warning` in the severity
cell. Twelve is reachable only as thirteen-minus-`structured-elements`, which nothing states — and §3
of this same spec says *thirteen* for an overlapping set.

**What the predicate does not reach, stated so §1 does not over-claim.** Step 2 matches the *path*
form only: a spec naming a pending release as `v1.53.0` or `spec/1.53.0` (no `.md`) is invisible to
the rule, while `CLAUDE.md`'s convention — *never by its filename* — is about the number as much as
the path. The pattern is unanchored too, so a path ending in `spec/1.2.3.md` matches wherever it
sits. Neither is a defect in the predicate; both bound what the rule can be said to enforce.

### 2.1 The shipped boundary is load-bearing, and a draft without it was refuted

*"Target numbered above the referrer"* alone looks sufficient in all four directions until it is run
for false positives. The same prototype with the boundary clause disabled:

```
python3 scripts/forward-spec-ref-prototype.py spec 1.0.0 --no-boundary
```

**adds exactly one finding and removes none** — 2 → 3 at `85a46682`, 20 → 21 at `e309df62`. That
invariant is the durable form of this measurement; the denominators move and the added finding does
not. It is the same one in both states, `spec/0.36.0.md:566 -> spec/0.37.0.md`, and it is a real
reference outside any fence: `sed -n '566p' spec/0.36.0.md` at `e309df62` is *"3. **Convergence
arithmetic.** `spec/0.37.0.md` owns the rule that turns a row set into a verdict."*, a Non-Goal
deferring a question to that release. Both ends shipped — `git tag --sort=-v:refname` lists v1.0.0,
v0.37.0, v0.36.0 — so that reference cannot rot, and flagging it is noise.

| direction | at risk? | why |
|---|---|---|
| shipped → **pending** | **yes** | every renumber breaks it; this is the whole finding class |
| shipped → later **shipped** | no | both ends frozen — and this is the measured false positive |
| pending → *lower* pending | no | the target ships first and stops moving |
| pending → *higher* pending | **yes** | the target can still move |

**Any positive count fails the bar, so the ratio is not the argument.** This project's bar for a lint
rule is zero false positives — the bar that refuted the example-table rule and the
contradictory-comparator rule — and the boundaryless draft produces one at every state measured (1 in
3 at `85a46682`, 1 in 21 at `e309df62`). The boundary clause is not a refinement; without it the rule
does not ship.

**A boundary of `0.0.0` is that refuted draft, not merely something like it.** In an out-of-repo
probe the finding *set* at boundary `0.0.0` equals the `--no-boundary` set exactly — 21 == 21 at
`e309df62`, 3 == 3 at `85a46682`, set-equal and not merely equal in count. That is what makes the
degradation contract in §3 load-bearing rather than defensive, and it is what test row 7 pins.

### 2.2 Fenced blocks, which the live corpus now demonstrates

The prototype's first version did not skip fences and, run over the renumbered corpus, flagged a
**transcript line** — a fixture spec named inside a shell session, not a reference to anything. The
code-citation checker already gets this right; a rule that flags example output teaches the reader to
skip it, which is worse than not running.

**An earlier draft of this section said the live corpus could not demonstrate the fence rule. That was
true when it was written and is now false, and the section is stronger for it.** At `85a46682` the
boundaried run and `--no-fence-skip` return the identical two findings, because no spec then quoted a
`spec/<version>.md` string inside a fence. At `e309df62`, `diff` of the two runs adds exactly four,
**every one a shell transcript inside a fence in `spec/1.0.1.md`** — lines 173, 174, 250 and 389, of
the form `$ tp ground spec/1.56.0.md --status --check`. Those are precisely the case the paragraph
above describes: a fixture spec named inside a shell session, not a reference to anything.

So the fence rule is justified three ways and needs only the first two: the transcript instance,
`ParseHeadingsFromScanner`'s own precedent in `internal/engine/lint.go` (`if
strings.HasPrefix(strings.TrimSpace(line), "```") { inCodeBlock = !inCodeBlock; continue }`, followed
by `if inCodeBlock { continue }`), and now a live corpus that discriminates. Its test stays a
constructed fixture — a corpus assertion would be a hostage to whichever spec last pasted a
transcript — but §5's fixture-assertion rule is now satisfiable against the tree as well.

### 2.3 What is *not* stated here, and why

**No unpinned count of violations.** Every figure in this spec names its tree state, because two
recorded figures were re-derived for it and **neither reproduced**. The record is
`git show 85a46682:spec/candidates.md`, under *"### `forward-spec-ref`"*; that path is now a 44-line
forwarding note whose opening line reads *"This file holds no content"*, so the figures survive only
in git history and the citation has to name the object rather than the path.

| recorded | re-derived | how it differs |
|---|---|---|
| *"51 spec files, pre-repair → 18"* | **0** at `dc8213d2` and at `8f7d2e92`, boundary `0.37.0` | those are the only two commits whose `spec/` holds 51 `.md` files (`git ls-tree --name-only <c> spec/ \| grep -cE '\.md$'`), and both return zero |
| *"the same today → 0"* | **2** at `85a46682`, **20** at `e309df62`, boundary `1.0.0` | `spec/1.0.0.md` introduced the two; `spec/1.0.1.md` then introduced most of the rest |

**An earlier draft diagnosed the first row wrongly, and the wrong diagnosis is the more useful half.**
It said the recorded 18 came from *"a different corpus state, which the record does not name"*. The
record does name one — *51 spec files, pre-repair* — and the draft's own re-derivation was run at
`v0.37.0`, whose `spec/` holds **46** `.md` files. It substituted a differently-sized tree for the one
the record named, and then blamed the record for naming none. The real defect sits one step further
in: **no tree matching the record's description reproduces its count.** The 18 comes back at 46 files
(48 findings), at 51 files (0), and at HEAD (20) — in none of them.

A figure over a glob has two moving parts — the corpus and the boundary — and a record that fixes
neither is not checkable. Fixing one is not enough either, which is what that row demonstrates: it
fixes the corpus *by description*, and the description locates no tree. **The durable form is a
literal commit-ish and a literal boundary string**, which is what every figure in this spec now
carries. Any implementation of this rule should expect the count to move and should not carry one in
a comment.

## 3. Where the rule lives, and the git dependency

**This is the decision the spec exists to take.** All thirteen `Check*` functions in `internal/engine`
are pure functions of their arguments — the whole parameter set across the thirteen is `[]string`,
`[]*Heading` and `int` (`CheckBrokenCrossRefs(lines, headings)`,
`CheckSectionSize(headings, totalLines, maxSectionLines)`) — and **not one receives a path**. Two of
them take a config threshold and `CheckSpecSize(totalLines, maxLines int)` receives no text at all,
so *"functions of the spec's own text"* is loose; the property that matters is that they are
callable, and testable, without touching a filesystem. Derive it at `e309df62` with
`grep -rhE '^func Check[A-Za-z]*\(' internal/engine --include='*.go' | grep -v _test`.

The one repo-aware check tp runs (`scripts/check-spec-code-citations.py`) lives outside lint as a
`workflow.checks` script for exactly that reason. This rule needs two strings the spec's text does
not contain: the referrer's own version, and the latest shipped version.

**The decision: the engine rule stays pure and the CLI supplies both strings.**

```go
func CheckForwardSpecRefs(lines []string, selfVersion, shippedBoundary string) []Finding
```

`runLint` derives `selfVersion` from the path it already holds (`internal/cli/lint.go`'s
`specPath := args[0]`) and `shippedBoundary` from the newest version tag. The impurity is one git
read in the CLI layer.

**The reason for that placement is `Check*` purity, not engine purity, and an earlier draft stated
the second.** It said the CLI "is where tp already puts its I/O", which the package refutes: at
`e309df62`, `grep -rnE 'os\.ReadFile|os\.Open|os\.WriteFile|os\.Create|exec\.Command|os\.Stat|filepath\.Walk' internal/engine --include='*.go' | grep -v _test.go`
returns **61** call sites across **28** non-test files, `internal/engine/lint.go`'s own `os.ReadFile`
among them. What is true, and is the whole of the argument, is that the three files declaring the
thirteen `Check*` functions (`lint_rules.go`, `vague.go`, `structured.go`) contain **zero** `os.` or
`exec.` call sites. Putting the git read in the CLI keeps that property; it does not preserve a
purity the engine never had.

**Degradation is silence, not a guess** — and silence is the safe direction in only one of the two
ways the boundary can be wrong. If either string is empty — a non-version filename, a tree with no
tags, a `git` that fails — the rule returns nothing.

**Both wrong boundaries were measured, and they fail in opposite directions.** At `e309df62`, over
the whole corpus: boundary `0.0.0` → 21 findings, `0.36.0` → 21, `1.0.0` (the true newest tag) → 20,
`1.60.0` → **0**, `2.0.0` → **0**. A boundary *below* the newest tag re-admits §2.1's refuted draft
exactly (the set-equality above); a boundary *above* it suppresses every true positive and reports
nothing at all. Neither is hypothetical once the boundary comes from `git tag`: a repository with a
stray `v9.0.0` tag reaches the second, a shallow clone with no tags reaches the empty string. Silence
is acceptable when tp *knows* it cannot answer, which is what the empty string encodes; it is not
acceptable as the outcome of a boundary tp believed. This is why the boundary is never guessed.

### 3.1 Why not a `workflow.checks` script

It is the obvious alternative, and the argument an earlier draft gave against it rested on two
factual claims, **both false at `e309df62` and both false when they were written**. They are recorded
here rather than deleted, because the conclusion survives and the argument for it has to change.

**Claim 1: *"`engine.RunCommand` performs no substitution, so every registration hardcodes a spec
path — and every one has."*** The first half is true — `internal/engine/runcmd.go`'s
`func RunCommand(command, dir string, timeout time.Duration, tailLines int) RunResult` passes
`command` straight to `exec.CommandContext(ctx, "sh", "-c", command)` with nothing inspecting or
rewriting the string. **The inference does not follow, and this repository's own registration is the
counterexample.** Because the command reaches `sh -c`, the *shell* substitutes: `.tp/config.json`'s
live entry is `python3 scripts/check-spec-code-citations.py "$(tp resume 2>/dev/null | python3 -c
'…["spec"]')"`, which contains no spec path at all. `git log -S'code-citation-drift' -- .tp/config.json
'spec/*.tasks.json'` returns four commits, and both shapes are present: three carry the hardcoded
form (`8df3d156`, `22b769c9`, `a291438e`, each ending in a literal `spec/0.3x.0.md`) and `4d155a88`
carries the command-substitution form. *"Every one has"* was a claim of exhaustiveness over a set its
author did not enumerate, and the enumeration takes one command.

**Claim 2: *"`tp config --resolved` reports `checks: []` today."*** Running the command it names
refutes it. At `e309df62`, `tp config --resolved` reports `workflow.checks` with `"source":
"project"` and one entry, `code-citation-drift`. It has been there since `4d155a88`, *"register
code-citation-drift at the project layer"*, **2026-09-02 15:12 — two days before this spec was
written** — and `git show 85a46682:.tp/config.json` already carries the identical entry at the commit
that added this spec's own prototype. Every clause built on the empty list falls with it:
*"unmechanized for four releases"* is false, *"the registration died with the release"* is not what
happened (a project-layer registration cannot die with a release), and *"nobody was told it had come
back"* inverts the situation — it is back, and this spec was written after it came back.

**The conclusion stands on a hazard that is recorded and does hold.** `CLAUDE.md` states it: *"a
check registered before its subject exists suppresses that finding class for every reviewer while
verifying only whatever part of its subject already exists — which may be nothing"*, with v0.34.0
§7.1 as the worked instance, where a check registered for eight rounds hid three stale claims. That
hazard is about registering a check **early**, not about one dying with its release. It applies here
because a `workflow.checks` registration for `forward-spec-ref` would tell every reviewer to stop
reporting the class from the moment it is registered, whatever the script then does.

A rule whose subject is **the spec's own text** does not need that machinery, and putting it there
buys the suppression hazard for nothing. The code-citation check belongs outside lint because its
subject is the *code*; this rule's subject is the document lint already reads.

## 4. Non-Goals

1. **No cross-file section validation.** The obvious companion — flag `spec/X.md §Y` where X has no
   §Y — was prototyped and refuted: **its marginal yield over this rule is zero.** The recorded
   derivation is `git show 85a46682:spec/candidates.md` — *"broken sectioned refs across both trees,
   snapshots included | 312"*, *"of those, target <= referrer (the only case `forward-spec-ref`
   cannot see) | 0"*. It also needed `spec/.tp-review/` excluded, since most of its raw hits are
   round snapshots, frozen photographs whose references were correct when taken. **Two instruments
   agree on that ratio and not on the totals**: the record says *"163 of its 173 raw hits"* (94.2%),
   a matcher built independently for the grounding round says 586 of 621 at `020262d9` (94.4%). The
   record's matcher is not in the tree, so the ~3.6× gap cannot be reconciled and the ratio is what
   the non-goal rests on.

   **The zero is a fact about today's corpus, not a structural property, and an earlier draft
   asserted the second** — *"every broken section reference in the corpus points at a higher-numbered
   spec"*. The independent construction at `020262d9` finds 35 raw pairs in `spec/*.md`, 2 of them
   broken, and **both point at a *lower*-numbered spec**, the case that universal denies. Both are
   disqualifiable on inspection, so the marginal yield really is zero and the non-goal holds; nothing
   prevents a lower-numbered broken reference being written tomorrow. It is not refuted in principle;
   there is simply no instance of the case that would be its alone.
2. **The rule does not open the target file.** It does not check that the target exists, that it has
   the section named, or that its subject is what the referrer says. Every one of the ~35 stale
   citations resolved; resolution is not the signal. That figure is restated from `CLAUDE.md` and is
   not re-derivable (§1), so the load-bearing evidence is the probe: a stat-and-skip variant of the
   prototype loses **zero** findings at `e309df62` and at `85a46682` alike.
3. **No git dependency beyond one boundary string.** Not a tag list, not a history walk, not a
   per-target shipped check. One string, resolved once per `tp lint` invocation.
4. **No autofix, no rewrite.** The remedy is a human sentence — name the release's subject — and a
   mechanical rewrite would produce the prose this rule exists to prevent.
5. **No effect on `tp lint`'s exit code beyond the existing severity contract.** `warning` behaves
   the way every other `warning` rule behaves: `internal/cli/lint.go` reaches
   `os.Exit(ExitValidation)` only under `if result.Errors > 0` — its three other exits are file and
   encode failures, not findings — so a warning changes no exit code (confirmed by running
   it — `tp lint CLAUDE.md` at `e309df62` reports 0 errors, 3 warnings, 17 info and exits 0). This
   release introduces no new severity and no new gate. **No count of sibling rules is stated**, for
   the reason §2 gives.
6. **The roadmap table is not exempted by the rule, and the reason is not the one an earlier draft
   gave.** That draft said *"lint does not read it"*. Running the shipped command refutes that:
   `tp lint CLAUDE.md` at `e309df62` succeeds, exits 0 and returns 20 findings — `runLint` lints
   whatever path `args[0]` names, with no spec-directory or naming restriction. What actually keeps
   the roadmap out of scope is §2 step 1: `CLAUDE.md` is not a version-named file, so it yields no
   self-version and is skipped. The two reasons behave differently and the difference is worth
   stating — "lint does not read it" would also exempt a hypothetical `spec/ROADMAP.md`, while the
   real mechanism exempts every non-version filename and nothing else, including `spec/undecided.md`
   and `spec/candidates.md` (22 of the 62 files in `spec/` at `e309df62`; the prototype prints the
   count as *"22 non-version files skipped"*). So the one place a number legitimately belongs is out
   of scope by construction rather than by a carve-out, and a carve-out inside the rule would be a
   second home for the convention.

## 5. Tests

Every row names the mutant that must fail it.

| # | from | assertion | the mutant that must fail it |
|---|---|---|---|
| 1 | §2 | a referrer at `1.0.0` naming `spec/1.38.0.md`, boundary `1.0.0`, is reported once with the reference's line number | return no finding — the shipped behaviour, since the rule does not exist |
| 2 | §2.1 | a referrer at `0.36.0` naming `spec/0.37.0.md`, boundary `1.0.0`, is **not** reported | drop the boundary comparison and keep only *target > referrer*, which is the measured false positive |
| 3 | §2.1 | the same pair with boundary `0.36.0` **is** reported | hard-code the boundary rather than taking it from the caller, which makes the rule's answer depend on when it was compiled |
| 4 | §2.2 | a reference inside a ```` ``` ```` fence is not reported, and the same reference outside one is | drop the fence toggle — the prototype's own defect, which flagged a shell transcript |
| 5 | §2.2 | a fence opened and never closed suppresses every later reference rather than panicking — **and, in the same test, those same references are reported once the fence is closed** | treat an unterminated fence as closed at the next line, which re-admits the transcript case |
| 6 | §3 | on a **constructed** fixture named `0.35.0-candidates.md` holding a reference to `spec/1.38.0.md`, no self-version is derived and nothing is reported | parse a version out of any leading digits, which makes the fixture a referrer at `0.35.0` and reports its reference |
| 7 | §3 | an empty `shippedBoundary` reports nothing, on input that would otherwise report | treat empty as `0.0.0`, which is §2.1's refuted draft reached through the degradation path |
| 8 | §3 | an empty `selfVersion` reports nothing, on input that would otherwise report | treat empty as `0.0.0`, which reports references in files the rule is meant to skip entirely |
| 9 | §2 | a reference to the referrer's **own** version is not reported | use `>=` rather than `>`, which flags a spec's own filename in its own header |
| 10 | §4.2 | on a **constructed** fixture, a reference to a higher, unshipped spec whose target file does not exist is still reported | stat the target and skip when absent, which is the resolution check §4.2 forbids |
| 11 | §2 | a reference whose target **equals** the shipped boundary is not reported | use `>=` rather than `>` on the boundary comparison, which re-admits the shipped→later-shipped direction one version at a time |

**Four mutants were built and run over `spec/` in out-of-repo archives. Two do not discriminate there
at all (rows 6 and 10), and a third (row 8) discriminates for a different reason than the one stated.
That is why rows 6 and 10 now say *constructed fixture* and row 8's harm clause was rewritten.**

| row | the earlier draft's claim | measured at `e309df62` (and `85a46682`) |
|---|---|---|
| 6 | the leading-digit mutant "flags `0.35.0-candidates.md`'s own recorded evidence" | **adds zero findings** — 20 → 20; only the skipped count moves, 22 → 4. That file's one `spec/<version>.md` reference is to `spec/0.35.0.md`, equal to the referrer and excluded by `>` |
| 8 | empty `selfVersion` as `0.0.0` "flags every reference in every non-version file" | flags **12 of the 40** such references, in **2** of the 22 files; the other 28 stay below the boundary, which survives this mutation. The stated harm is **row 7's** mutant |
| 10 | a stat-and-skip implementation "reports nothing on the corpus this rule was written for" | **it reports everything** — 20 → 20 and 2 → 2, set difference empty in both directions |
| 9 | — | the `>=` mutant adds **12** self-references at `e309df62` and **zero** at `85a46682`, so the row became demonstrable on the corpus in the day after the spec was written |

**Row 10's own sentence was the measurably false one.** The distinction it draws is right — a
reference that no longer resolves is the *end state* of the rot, and what this rule catches is a
reference that still resolves to a file whose subject has moved — but it cannot rest on the corpus,
for the reason non-goal 2 gives: every stale citation resolved, so a stat-and-skip implementation
skips none of them and its output is identical to the correct rule's. Two other passages of this spec
said exactly that, and the sentence contradicting them was written beside them. There is a structural
reason as well: §3 fixes the signature as
`CheckForwardSpecRefs(lines []string, selfVersion, shippedBoundary string)`, which receives **no
directory**, so that mutant cannot be written inside the engine except by statting relative to the
process working directory. **A mutant the release's own architecture forbids is not one a test can be
built against.**

**The corpus fixture is asserted, not assumed.** A test running the rule over `spec/*.md` must
`require` the properties its verdict rests on — at `e309df62`, 40 version-named files and 22
non-version-named ones, and, for a fence assertion, at least one fenced `spec/<version>.md`
reference (satisfiable today, and not at `85a46682`). This repository has already lost a round to a
guard whose fixture could not distinguish the behaviour it was named for, and **rows 6, 8 and 10 are
that failure caught one phase earlier**: each rested on a corpus property nobody asserted, and
measurement found all three absent.
