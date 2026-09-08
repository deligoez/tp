# Undecided — measurements

**Not a release, and not a spec.** This is the sidecar for `spec/undecided.md`: the register keeps a
decision or a refutation, a reopen condition or a settling bar, and a pointer; the forensics that
earned each verdict live here, one `## <entry title>` section per entry.

**Nothing here is a decision.** An entry in the register may cite a section below; a section below
never states what should be done. Where a figure moves with the tree it is a fenced derivation
command run from the repository root; where it does not, it is anchored to a commit or a tag. A
figure carried in from elsewhere and never re-derived says so, and must not be re-quoted as a
measurement.

Measurements were taken at commit `13bfde30` unless the section names another ref.

---

## Six requirements-smell lint rules — the whole adjacent family

**The prediction scorecard.** Two predictions were on record before the run, and the scorecard is
this repository's own rule applied to a forecast rather than to a role: one clause of four was right
in its verdict, and **that one's stated mechanism was wrong**. `non-verifiable` was predicted to die
from the corpus mixing English and Turkish — measured, the corpus carries Turkish letters on **5
lines in 2 files**, and the apostrophe-suffix trap yields **zero** tokens. It dies for an unrelated
reason. `vague pronoun` was predicted to duplicate `vague-language` — measured, **0 of 369 lines are
shared**, and the same holds for every candidate. Both predictions reached a defensible verdict
*through a mechanism the corpus does not contain*, which is precisely the failure mode
`spec/undecided.md` exists to record.

**The fence defect the prototyping surfaced, which is not part of the entry.** `CheckVagueLanguage`
was the only rule in `internal/engine/vague.go` that did not track fenced code blocks, so a share of
its corpus findings fired *inside* fences, on examples of its own output. It went to the release
being written at the time and shipped; `internal/engine/vague_fence_test.go` pins it.

**The corpus the six were run against.** 67 files (`spec/*.md` and `spec/backlog/*.md`), 19,715 prose
lines, **1,529 candidate findings judged**, at the state of the tree when the prototypes were run.
That state is not tagged and the file set has since moved, so the three figures are anchored to the
run rather than re-derivable; the six per-candidate verdicts in the register carry their own counting
rules and are what the refutation rests on.

## The unexecutable-split lint rule

**The post-mortem of the original table's middle row.** It recorded 11 hits for *"title or acceptance
mentions a test"* with roughly two true positives; its counting rule was never written down, and no
reading of that phrase lands near 11. Re-derived under three readings: a `test` substring over title
and acceptance gives **77**; a word-boundary `\btests?\b` in the title alone gives **2**, in the
acceptance alone **74**. The row was dropped rather than repeated.

**The single tag-form hit that was classified.** `mechanize-candidates-retained` in
`spec/0.25.0.tasks.json` is tagged `test` and depends on `per-role-overlap-report` in the same
section, but it is not a test-only task — its acceptance is *"keep the existing output exactly as-is,
adding no per-round signal"* and its closure reads *"No production change"*. It is a
negative-requirement task whose deliverable is a guard, and it executed without the gate ever going
red. It is the hit the original single-fire count named; the tag form fires on more than one task
today and the newer hits have not been classified.

**One confound, stated rather than hidden.** tp's own task files are decomposed by an agent following
`CLAUDE.md`, which already carries the rule — so the absence of true positives may mean the rule is
already obeyed here rather than that the failure does not occur. That weakens *"the failure never
happens"*; it does not weaken *"this predicate finds a false positive and no true one over the whole
corpus"*, which is what a zero-false-positive bar decides on.

## The contradictory-comparator lint rule

**The prototype is not committed anywhere in the tree, so the flag counts cannot be re-derived.** The
original run reported 2 flags with 0 true positives over the spec corpus at both the `v0.36.0` and
`v0.37.0` tags — the two states the candidate described as *before this cycle's repairs* — and zero
flags when grouping was narrowed to a single section. Those three figures are carried as relayed.

**The corpus size does re-derive, and moved for a reason unrelated to the rule.** Top-level
`spec/*.md` counts **46** at `v0.36.0`, **46** at `v0.37.0`, **59** at `27f84468` and **46** at
`13bfde30` — the drop is the 2026-09-08 move of the pending specs under `spec/backlog/`, not a
deletion. The original's *"47 spec files today"* was true when it was written and is not now, which
is the ordinary fate of a figure with no derivation beside it.

```bash
for ref in v0.36.0 v0.37.0 27f84468 HEAD; do \
  printf '%s %s\n' "$ref" "$(git ls-tree -r --name-only $ref spec/ | grep -c '^spec/[^/]*\.md$')"; done
```

## The example-table lint rule

**Neither firing rate is re-derivable and the denominator has moved.** No prototype is committed. The
keyword-and-shape heuristic was reported to fire on **3.9–22.6%** of this repository's spec sections;
the original's 1,032 sections does not reproduce under the nearest stated rule. Heading lines at any
level across `spec/*.md`, fenced blocks not excluded, counted **1,251** at `27f84468`. The rates are
carried as relayed and must not be re-quoted as measurements.

The denominator moves with the tree, so run the rule rather than reading the figure — it prints a
smaller number today than at `27f84468`, because the pending specs moved under `spec/backlog/`:

```bash
grep -h '^#' spec/*.md | wc -l
```

## The identifier pass

**Re-derived, with the counting rule stated.** The pass was reimplemented from its own specification
and run over `git archive v0.37.0` and over the then-`HEAD` `27f84468`. Corpus: top-level `spec/*.md`
only. Spans: inline backtick spans outside fenced blocks. One finding per `(file, identifier, kind)`:

| corpus | files | `unintroduced` | `unreferenced` | files with a finding |
|---|---|---|---|---|
| `v0.37.0` | 46 | **997** | 94 | **45** |
| `27f84468` | 59 | **1,425** | 116 | 58 |

The `v0.37.0` row is the one the refutation rests on, because it is anchored to a tag: 45 of 46
shipped specs carry a finding, so a shipping precondition of *"neither pass produces a finding on any
shipped spec"* can never be met on this corpus. The `27f84468` row is the same rule at a moving ref
and is kept only to show the shape holds as the corpus grows.

**Reconciling the relayed figure.** The relayed count was 979 over 46 specs. At the same corpus size
this rule gives **997** — 1.8% apart, which identifies the relayed count as `unintroduced-identifier`
alone over the `v0.37.0` corpus. The figure is reproduced; the conclusion does not turn on which of
the two numbers is used.

**The other half of the entry was overstated and is corrected.** *"All references to things that
exist elsewhere in the repository"* is not what the corpus says. Counting rule: an identifier
resolves when its literal text appears in at least one git-tracked file outside `spec/`, or names a
path that exists. **850 of 997 (85.3%)** resolve at `v0.37.0`; **1,260 of 1,425 (88.4%)** at
`27f84468`. The remainder are mostly illustrative paths in the older specs (`docs/foo.md`,
`spec-r1.md`) and bare basenames of files that exist under a directory the spec did not spell.

## The corpus-replay gate, as a procedure

**Counting rule.** For every `state.json` under `spec/.tp-review/` and `spec/backlog/.tp-review/`,
compare each recorded round's `spec_hash` against `sha256` of the snapshot file tp names for that
round — `snapshot-round-N.md` for review, `snapshot-audit-round-N.md` for audit. Strip the `sha256:`
prefix before comparing, or every round reads as divergent:

```bash
python3 - <<'PY'
import json,glob,os,hashlib,collections
res=collections.Counter(); mism=collections.Counter(); cyc=collections.Counter()
for sj in glob.glob("spec/.tp-review/*/state.json")+glob.glob("spec/backlog/.tp-review/*/state.json"):
    d=json.load(open(sj)); base=os.path.dirname(sj)
    for phase,key,pfx in (("review","review_rounds","snapshot-round-"),
                          ("audit","audit_rounds","snapshot-audit-round-")):
        for r in d.get(key) or []:
            res[phase]+=1
            snap=os.path.join(base, "%s%s.md" % (pfx, r.get("round")))
            if not os.path.exists(snap):
                res[phase+" no snapshot"]+=1; continue
            res[phase+" with snapshot"]+=1
            if (r.get("spec_hash") or "").replace("sha256:","") != \
               hashlib.sha256(open(snap,"rb").read()).hexdigest():
                mism[phase]+=1
                if phase=="review": cyc[os.path.basename(base)]+=1
print(dict(res)); print("mismatch", dict(mism)); print("review mismatch by cycle", dict(cyc))
PY
```

At `13bfde30` that prints 177 review rounds all with a snapshot and **35** divergent, and 112 audit
rounds of which 92 have a snapshot and **3** diverge, 20 predating snapshots entirely. The review
divergences sit in six cycles — `0.31.0` carries **19**, then `0.32.0` 7, `0.24.0` 4, `0.31.2` 2,
`0.35.0` 2, `0.29.0` 1 — and no cycle after `0.35.0` carries one.

**Quote the count, not the percentage.** The numerator has not moved while every denominator has, so
the ratio improves without the defect changing.

**The disposition the three withdrawing specs said was missing.** It exists. Counting rule: every
`resolved` object across both round-directory globs.

```bash
python3 -c 'import json,glob,collections;c=collections.Counter(json.loads(l)["resolved"]["status"] for p in ("spec/.tp-review/*/*.ndjson","spec/backlog/.tp-review/*/*.ndjson") for f in glob.glob(p) for l in open(f) if l.strip() and isinstance(json.loads(l).get("resolved"),dict));print(c, sum(c.values()))'
```

At `13bfde30`: **1,742 rows**, `fixed` 1,584, `wontfix` 145, `duplicate` 13. Earlier readings of the
same shape recorded 1,566 / 1,467 / 99 and 1,406 / 1,308 / 98; each was right about the shape and is
simply older, which is why the command is here and the numbers are anchored.

## `broken-cross-ref` extended across files

**Counting rule, re-derived at `27f84468`.** Every `.md` file in the tree, snapshots included; a hit
is one occurrence of `spec/<version>.md` followed within 80 characters by `§<number>` on a line
outside a fenced block; a hit is *broken* when the target file exists and no heading in it carries
that number as its first token:

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
file as broken. What reproduces exactly is the zero, and the snapshot share reproduces in shape
(93.8% here against 94.2% there).

**It would also have needed `spec/.tp-review/` excluded**, since almost every raw hit is inside a
round snapshot: a frozen photograph whose references were correct when it was taken.

## `floor_by_section`

**The mutant, and why the field's only assertion cannot fail one.** The sole claim was that the
per-anchor values sum to `floor_size`, and that sum is invariant under every possible anchor
misassignment: each uncut unit receives exactly one anchor whatever the mapping, so any grouping
totals the same. A tester built the mutant and ran it — correct versus mutant on `spec/1.0.1.md`
gives **seven keys against six, both summing to 62**. An implementation that assigns every section
the wrong count and loses a section key entirely passes the assertion.

**Unbounded output.** It was the only one of the four candidate fields unbounded in output size —
about **149 bytes at seven anchors and 1,641 at ninety-one**, on a command that honours neither
`--compact` nor `--quiet` — and the only one whose row named no decision it would feed.

## `floor_figure_share`

**Why the two-arm numerator could only report a high number.** The numerator was two of the three
arms of the floor's own admission predicate (`inFloor` = digit ∨ code span ∨ measurement verb,
`internal/engine/floor.go`), so across six of this repository's specs it stayed inside a narrow band
near the top of its range.

**The measurement was taken in a copy, deliberately.** It is over each spec's latest emitted floor,
and `tp ground --units` emits a round when the spec has moved — so running it in the repository would
have advanced rounds the measurement did not own. This is the same trap `CLAUDE.md` records for
`tp audit` probes.

**The narrow variant.** Restricting the numerator to `floorHasDigit` alone, over the same six specs,
gives a spread roughly twice as wide as the two-arm form, and the spec that obeys the reference rule
most closely scores *lowest* — the correct direction. The command is in `spec/1.0.1-measurements.md`.

## `forward-spec-ref`

**The dead-path population, measured at `27f84468` before the rename cleanup.** Counting rule:
occurrences of `spec/<version>.md` outside `.tp-review/` across `spec/**`, `skills/**`, `CLAUDE.md`
and `README.md`, minus files that exist — **81** to `spec/1.3x–1.5x.md`, **6** to `spec/1.0.2.md`,
**1** to `spec/0.37.1.md`. Every one was resolvable a rename ago, which is the point: the population a
dead-path check would serve is created by renames, not by forward references.

**The prototype's own population is gone.** `python3 scripts/forward-spec-ref-prototype.py spec 1.0.1`
finds **1** reference at `27f84468` (`spec/1.0.1.md` citing `spec/1.0.2.md`, a file that no longer
exists), and the same command over `spec/backlog` finds **0**, because pending specs are no longer
numbered at all.

## Inferring a spec's class

**The predicate and its correlations.** A density predicate — loop-lexicon occurrences per thousand
words — scores **87% leave-one-out with zero false positives** over 23 hand-labelled specs, the
threshold refitted with each item held out. Applied to the seventeen shipped cycles' round-1
snapshots it reproduces `CLAUDE.md`'s published loop/tool round medians **exactly**, and yields
`r(class, rounds) = +0.40`. The same unit's own hand-labels of those same seventeen give `r ≈ 0` and
do **not** reproduce the published split. So two independent human labellings disagree, and the thing
that tracks cycle length is the predicate rather than the label.

**Why the frontmatter field was cut from the release that discovered this.** `class:` is a hand
label; a median computed over hand labels reports measured noise, and the field would have been empty
on the day it shipped — **one of sixty-six specs carried it, and no recorded round carried a class at
all**.

**Honest limits, recorded so the next attempt does not overclaim.** n = 23; the two label sources are
two separate hand-assignments rather than one rule; and leave-one-out fixes the threshold but not the
lexicon, which was chosen after seeing the corpus.

## The divisible round

**The sub-claim that did not reproduce, and why both readings strengthen the conclusion.** The
original said *"four of the six in one section"*. By the rows' own `location` field, **all six** carry
at least one row at `§3`; by item-id prefix, **three** share `table-2-`. Neither is four, and either
says the findings cluster by location rather than by count — which is the conclusion the entry draws.

**The counting rule for the shard arithmetic.** Distinct `item_id` values carrying `role:
spec-coverage` across `spec/.tp-review/0.37.0/audit-round-*.ndjson`, against how many of them hold a
non-`PASS` status in any round. At `13bfde30` that is 6 of 97, so splitting the items by *count*
gives two shards each about 94% `PASS` and neither shard is informative about where the findings are.

## The evidence contract

**The channel is not one, which is why no decision names a site.** Three sites inject previous-round
findings into a prompt: `buildFindingsSummary` in `internal/cli/review.go`, `buildRegressionPrompt` in
`internal/cli/review_regression.go`, and `buildVerifyPrompt` in `internal/cli/review_verify.go`. Their
bounds and labels differ: the panel block caps its detailed rows at 50 and truncates its free-text
renders at 80, 60 and 40 characters, and of the three only `buildVerifyPrompt` prints
`resolved.evidence` under the label *evidence* — `buildFindingsSummary` labels it `wontfix:` and
`buildRegressionPrompt` prints it unlabelled after an em dash. The runs are in
`spec/1.1.0-measurements.md`, *`evidence` is write-only: the three injection sites, and where the
reading half went*.

## A registered check that outlives its release

**What the entry said before it was corrected, kept because the correction is the substance.** It
recorded `.tp/config.json` as registering no check and `tp config --resolved` as reporting
`checks: []`. Both were false by the time it was read: the project layer registers checks today, and
`tp config --resolved` reports them with `"source": "project"`.

**The suppression history the entry names.** tp tells every reviewer to stop reporting a mechanized
class, so during the two releases `code-citation-drift` was registered per task file, the class was
suppressed for every reviewer; when the registration died with the release nobody was told the class
had come back. The project-layer registration closes that for `code-citation-drift`.
`test-inventory-drift` is registered in one task file only (`0.31.2.tasks.json`), so the same hazard
stands for it. The task-layer registrations are `0.31.2.tasks.json` for `test-inventory-drift`, and
`0.33.0.tasks.json` and `0.34.0.tasks.json` each for `code-citation-drift` against their own specs.

## `frontmatter-key-namespace`

**Why predicate 1 is the cheaper check and the shallower defect.** Predicate 1 flags a key outside the
`tp:` mapping. The second class of instance has its key *inside* `tp:`, so a namespace check reads it
as correct; only a default-value check reaches it, and that is the one that cost a round to find,
because a fixture in class 2 is green under a mutant that deletes the whole frontmatter read.

**One instance of class 2 is not a class.** Predicate 1 has three; predicate 2 has one, and one
instance is a bug report rather than a rule. What would change that is a sweep of the other 18 Go
files carrying a literal `\n---` for fixtures whose declared value equals the parser's default. That
sweep has not been run, and it is the honest reason the entry is not in a release.

## A fenced command that runs and prints the wrong thing

**The instance, from `spec/1.0.1.md`'s own audit round 3.** `README.md`'s *"What grounding finds"*
block, added at `8dfa6fb3`, globbed only `spec/.tp-review/*/` — while this repository's ground rounds
also live under `spec/backlog/.tp-review/*/`, the two-glob trap `CLAUDE.md` documents. The command
ran, exited 0 and printed a full set of numbers; not one of them was a figure the prose beside it
stated, and adding the second glob printed a third set again. Nothing in §4.1's liveness rule can see
that, because the rule's subject is whether output appeared.

**The repair shipped.** `README.md` now globs both round directories in the same fenced command and
derives its counts there rather than asserting them in the prose beside it, which is what §4's own
*"a number does not live in a spec, a reference does"* already asks for.

## A sentence rewritten in answer to a finding is exempt from the cut for one round

**The three instances, measured on `spec/1.1.0.md`'s grounding.** Recorded in
`spec/backlog/ground-command-friction-measurements.md` under "§11.1": a repair removed a quantifier,
the shortened sentence fell below the arms' cut threshold, and the claim left the floor in the same
edit that answered the finding — nothing in the round reports that. A third instance in the same
document wrote a new requirement as a short standalone sentence that never entered the floor at all.

**Why the cheapest form is not an exemption.** Two of the three instances were repaired by the author
the moment they saw the number, which is what makes a reported `cut` delta the candidate to cost
first. Neither the cost of an exemption nor whether it is expressible in the floor's own terms has
been measured.

## Cross-site key agreement in the review prompt

**The reading at `c407bb7e`, before the review contract gained `evidence`.** On
`tp review spec/1.1.0.md --role implementer` and on a `tp audit` emission:

| site | keys it names |
|---|---|
| `findingFormat`'s JSON example (`internal/cli/review.go`) | `severity`, `category`, `location`, `finding`, `suggestion` |
| the optional-`class` sentence beside it | `class` |
| `outputContractInstruction` | `role`, `location`, `class`, `severity` |

Row 1 was already stale when it was written: `findingFormat`'s example has carried `evidence` since
before `a4fd187f`. Row 3 became stale after `a4fd187f`, when `evidence` was added to the review branch
of `outputContractInstruction`. The register carries the reading at `13bfde30`.

**What each way of equalising the three costs.** `outputContractInstruction` is shared with the audit
phase — called from `review.go`, `review_regression.go` and `audit_roles.go`, and all four auditor
prompts carry the block — so editing it to match the review example changes the audit prompt too.
Dropping `category` from the example to match the contract block empties `by_category`, a key
`tp review --report` ships (`internal/cli/review_report.go`). Making the three name one set promotes
`class`, labelled *Optional* in the sentence beside the example, to mandatory.

## `NewRootCmd` writes package globals

**The incident, measured during `spec/1.0.1.md`'s implementation.** A unit ran the four-step gate, got
four zeroes, made an unrelated edit, ran it again, and step 1 failed with data races in tests it had
never touched. Isolated: `TestSkillFlagInventoryIsComplete` and
`TestSkillFlagInventoryRecordsOnlyWhatExists` each construct a root command, both are `t.Parallel()`,
and `NewRootCmd()` binds tp's package-level flag variables through pflag. Run alone under `-race` the
pair reports **30 races per run**. It reproduces on a tree five commits earlier and originates at
`fa68051b`, *"mark every eligible top-level test parallel — 50.9s to ~14s"*: the parallelism was the
speedup, and the shared globals were already there.

**Why it read as a gate defect.** The defect is invisible without `-race` and timing-dependent with
it, so whether the gate reddens is a property of machine load rather than of the code under test. A
gate that passes once and fails once teaches its operator to re-run rather than to look. That
argument is `spec/backlog/gate-sequence.md`'s subject and is not re-made in the register.

## `t.Parallel()` in the engine package

**The `internal/cli` half, re-derived when the entry was moved.** This audits the package the entry's
own decision excludes, and is kept only so the figures are not lost. Each row carries its counting
rule, and the ones that did not reproduce are marked as such:

| the entry's figure | re-derived | how it was derived |
|---|---|---|
| `internal/cli` is **1,743** serial test functions | **does not reproduce as an `internal/cli` figure.** The package holds **1,051** top-level `func Test…`; **1,781** is the *repository-wide* count, which is what 1,743 tracks | `rg '^func Test' internal/cli --no-filename \| wc -l` against `rg '^func Test' -g '*_test.go' --no-filename \| wc -l` |
| **1,116** of them fork the tp binary | **the counting rule decides which number this is.** `runTP(` appears at **1,120** *call sites*; **736** top-level test *functions* have a body calling any `runTP*` helper | `rg -o 'runTP\(' internal/cli --no-filename \| wc -l`, against a `re.split(r'(?m)^func ', src)` walk over `internal/cli/*_test.go` counting bodies matching `\brunTP[A-Za-z]*\(` |
| I/O-bound at **8%** of ten cores — **39.7 s** CPU inside **50.5 s** wall | **holds in shape.** First run in a fresh copy: `real 47.37 user 16.14 sys 20.75` — **36.9 s** CPU inside **47.4 s** wall, **7.8%** of ten cores | `/usr/bin/time -p go test ./internal/cli -count=1` in an `rsync -a --exclude .git` copy |
| the **7** files that `t.Chdir` are skipped, because Go panics on the pair | **holds exactly: 7** | `rg -l 't\.Chdir' internal/cli \| wc -l` |
| **zero** `os.Setenv` in the package | **holds: 0.** The package-level-variable-write half of that claim was not re-derived | `rg -c 'os\.Setenv' internal/cli \| wc -l` |
| **54 s → 14 s**, and **15.8 s** under `-race`, four consecutive green runs, `go vet` clean | **borrowed, not re-run.** Measured in an `rsync` copy at v1.0.0's audit round 5; that copy is gone, and the 54 s baseline reads 47.0 s in row 3 | — |

**The load-sensitivity that makes `internal/engine` the half at issue.** gremlins runs the mutated
package's own tests once per mutant, and `CLAUDE.md` records the same package at **92** timeouts busy
against **88** idle, with default settings driving load average from **8** to **177**
(`grep -n 'load average' CLAUDE.md`). `t.Parallel()` multiplies concurrency inside each mutant by
gremlins' own `--workers`.

## A prior-round section for `tp review`

**What the audit phase does.** `loadAuditPriorRound` (`internal/cli/audit.go`) reads the previous
recorded audit round and returns, per role, that role's **own** non-PASS rows;
`renderPriorRoundSection` (`internal/cli/audit_roles.go`) renders them into a round-2+ prompt under
the heading *"Prior Round: context to re-check, not a verdict to repeat"*, with the instruction *"Re-
check each item against the code and record your own status. Do NOT repeat the prior verdict without
verifying."* It returns the empty string when the role has no prior non-PASS rows, so a round-1 prompt
and an all-PASS role carry no section at all. `filesChangedSince` in the same file tells the role
whether its evidence file moved since that round, which is what makes the re-check answerable rather
than rhetorical.

**What review carries instead.** A search for prior-round machinery by that name in the review path
returns **0** — `rg -n -i 'priorRound|prior round|prior-round|PriorRow' internal/cli/review.go
internal/cli/review_*.go`. But `buildFindingsSummary` (`internal/cli/review.go`) puts previous rounds
into every review prompt under a heading that asks for the opposite of a re-check —
`UNRESOLVED findings from previous rounds — DO NOT re-report:` — plus a second block,
`Resolved high/critical (DO NOT regress):`. Its shape differs on every axis: panel-wide rather than
role-scoped, capped at 50 rows with the remainder reported only as a count, each finding truncated to
80 characters and a `wontfix` row's evidence to 40, and its input is the `--findings` file or the
loaded round state rather than the recorded round read per role.

**Why no measurement settles it.** The repair-locality figures are about the adjacent surface —
findings sitting in text the round before wrote — but a share and a ratio cannot say whether returning
a reviewer its own prior rows would raise that share (the role re-treads its own ground) or lower it
(the role withdraws instead of re-filing).

## From the rows spec

**A durable home for an accepted finding — the three options and why none was chosen.** The candidates
row records only their number; the three themselves survive in git, in the pre-2026-09-02 spec that
carried this subject — `git show 3a83be30:spec/0.41.0.md`, whose §2 is *Mechanism*. That filename
corresponds to no file under `spec/` or `spec/backlog/` today, which is why the commit is cited rather
than a path. The three: a checklist file at a stable path, appended to rather than rewritten; an issue
template written to disk for the operator to file; a `TODO` entry with an owner and the finding's
`item_id`. What decides between them is a property rather than a preference, and it is the part
already agreed: *the target must be readable by the next cycle's decomposition without a human
remembering it exists.* An option that only works when someone happens to open it is not better than
the round directory it replaces. This repository has been working around the gap by hand for four
cycles: the candidates files are that durable target, maintained by an operator.

**Making `severity` checkable — the precedent a `--record` rejection would invert.** The category sink
returns early on an *empty* category — `if category == "" || engine.IsValidCategory(category)` in
`invalidCategoryRows.observe` — and that early return is pinned deliberately by
`TestParseAuditRows_AcceptsTheEnumAndAbsentCategory` in `internal/cli/audit_category_sink_test.go`,
whose comment gives the reason: *"treating that as invalid would reject every clean round."* Cite the
symbol and the test name, not a line.

**One clause of the original entry was stale and is corrected rather than carried.** It said *"nothing
on the audit path reads the field"*. Two paths read it: `internal/engine/auditclean.go`'s
`AuditSeverityBucket` and `advisoryAuditSeverities` grade on it whenever `audit_converge_on` is
`blocking` (v0.37.0), and `tp audit --merge` buckets `by_severity` through the same classifier. What
survives is narrower and still the point: **nothing validates it**, and under the resolved default —
`tp config --resolved` reports `audit_converge_on` at `all`, source `default` — no grading path reads
it at all. Validation cannot deliver what it was introduced for either: it makes a label well-formed,
never truthful, and it cannot make an unrun round run.

**An audit-side `nonblocking_open` — the four places that pin the key's absence.** All four are
present at `13bfde30`: `spec/0.31.0.md` §8.4 — *"`nonblocking_open` is review-only (audit convergence
has no non-blocking notion, §4.4) and is never an audit field"*; that spec's test 18, which repeats it
as an acceptance row; `skills/tp/REFERENCE.md`'s `--compact` disposition paragraph, which marks the
field *"(review-only, emitted only on an accepted-open clean round)"* — cited by that phrase, because
its line number has moved; and `TestReviewNonBlockingOpen_AuditUnaffected` in
`internal/cli/reviewconvergeon_clean_test.go`, which asserts that neither `tp audit --record` nor
`tp audit --status` emits the key. A fifth statement exists and is a comment rather than a guard:
`internal/engine/reviewclean.go`'s *"Review-only: no audit payload carries nonblocking_open."* The
`tp audit --merge` breakdown is a different surface and shipped separately — it buckets `by_severity`
through `engine.AuditSeverityBucket` in `internal/cli/audit_merge.go`, on the merged payload rather
than on the round's convergence signal.

**A review-side `accepted_blocking` — the three-tree probe.** Measured at `5058fc99` on three
one-round trees built from the same spec: a round recorded from a zero-byte findings file —
`clean: true`, `consecutive_clean: 1`; a round holding one `critical` finding resolved `wontfix` with
evidence — `clean: true`, `consecutive_clean: 1`; a round holding one **open** `medium` finding —
`clean: true`, `consecutive_clean: 1`, and `nonblocking_open: 1`. `tp review <spec> --status` on the
first two returns payloads whose **key sets are identical** — the symmetric difference is empty — so
no key names the accepted critical. The only value that moves is `review_rounds[].findings`, 0 against
1, which says nothing about severity or disposition. The third differs from the first by exactly one
key, `nonblocking_open`.

**The off-vocabulary severity denominators.** Counting rule: a round file holding at least one row
whose `severity` is present and outside its phase's vocabulary — `{error, warning, info}` for audit
(the set `internal/engine/auditclean.go` defines), `{critical, high, medium, low}` for review.

```bash
python3 - <<'PY'
import json,glob
def scan(globs, vocab):
    files=[f for g in globs for f in sorted(glob.glob(g))]
    bad=[f for f in files if any(json.loads(l).get("severity") not in vocab
         and json.loads(l).get("severity") for l in open(f) if l.strip())]
    return len(files), bad
t,b=scan(["spec/.tp-review/*/audit-round-*.ndjson",
          "spec/backlog/.tp-review/*/audit-round-*.ndjson"], {"error","warning","info"})
print("audit round files", t, "off-vocabulary", len(b))
t,b=scan(["spec/.tp-review/*/review-round-*.ndjson",
          "spec/backlog/.tp-review/*/review-round-*.ndjson"], {"critical","high","medium","low"})
print("review round files", t, "off-vocabulary", len(b))
PY
```

At `13bfde30`: **15 of 112** audit round files and **0 of 177** review round files. The offending
values are the *review* vocabulary leaking into audit files, and the fifteen are 0.29.0 rounds 1–2,
0.30.0 round 1, 0.31.0 round 1, 0.31.2 rounds 1–5 and 0.32.0 rounds 1–6.
