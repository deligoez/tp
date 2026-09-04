# tp v1.46.0 — Mutation score as a gate entry

> **This file is decisions.** Every figure below about gremlins' output was produced by **running it**,
> not by reading its documentation — in an `rsync -a --exclude .git` copy of `HEAD`, except the one
> run named below as a throwaway module. Five kinds of run, each named where its figures are used:
> `gremlins unleash ./internal/model --workers 4 -o out.json` (§1, §2, §3); the same command a second
> time in the same directory (§4); the same with `--test-cpu 2` (§4, and §7 row 3's fixture figures);
> `gremlins unleash <pkg> --dry-run -o dry.json` for `./internal/model` and `./internal/engine`
> (§1, §3); and a throwaway two-file module (§4). A figure
> **borrowed** from an older record is marked with the state it was measured at and the command that
> retrieves it, and is never presented as this tree's. §3 exists because those runs contradicted what
> gremlins' own field names say; §4 exists because a run contradicted `CLAUDE.md`.
>
> **Every figure here is from the FIRST gremlins run in a directory nothing had touched.** §4 records
> why: measured in **five** independent copies, a second `gremlins unleash` in the same directory
> returns the refused signature with no `--test-cpu` in its argv. A run after the first in one
> directory is evidence of nothing, and the only such runs quoted below are quoted *as* that finding.

## 1. Overview

`CLAUDE.md` has carried mutation testing as a trigger, not a gate, since v0.34.1, with a standing
warning that **the survivor count is not a score to drive down**. That stays true. What is missing is
weaker and more important: nothing records that a run **happened**, or **over which mutants**, so
every figure quoted from one is unverifiable afterwards.

This release adds one gate entry that **establishes a run completed over a known mutant set, and
records the set**. It does not grade the result.

**Cheap, once narrowed.** `gremlins unleash ./internal/model --workers 4 -o out.json` at `HEAD` is
**81 mutants in about 10 seconds** of wall clock (`elapsed_time` 9.17 on the run recorded here; that
figure is run-local, and four first runs this cycle spread 9.17–9.78 — the three others are in
`spec/.tp-review/1.46.0/ground-round-2.ndjson` under `u10`, so quote the count and not the second).
The expensive package is `./internal/engine`: `gremlins unleash ./internal/engine --dry-run` reports
`Runnable: 1541, Not covered: 279` — **1820 mutants** — and the same dry run with `-o` writes 1820
rows into `files[].mutations[]`, so the printed counts and the row list agree. (§3 explains why that
particular agreement is *not* the cross-check the entry gates on: both come from one pass.)
`CLAUDE.md`'s 1540 and its 53 minutes are the same package measured at tag `v0.35.0` —
`grep -n '1540 mutants' spec/0.35.0-candidates.md` retrieves the breakdown — so budget against 1820.
Per-package narrowing is an exact substitute: in default mode gremlins runs only the mutated
package's own tests, so an unchanged package's mutants cannot change verdict.

## 2. The entry reads the machine-readable output, never stdout

`gremlins unleash -o <file>` writes JSON. Measured shape:

```
go_module, files[{file_name, mutations[{type, status, line, column}]}],
test_efficacy, mutations_coverage, elapsed_time, mutator_statistics,
mutants_total, mutants_killed, mutants_lived, mutants_not_viable, mutants_not_covered
```

**stdout is not parsed** — and the reason is not that the two streams carry different mutants, which
an earlier draft asserted and a run refutes. Matching records on
(status, mutator type, file basename, line, column) for the `./internal/model` run: |stdout| **81**,
|JSON| **81**, stdout-only **0**, JSON-only **0**. stdout prints the whole `files[]` list, mutant for
mutant, in every field the JSON carries.

What differs is everything around that list, and the asymmetry runs both ways. stdout prints
`Timed out: N` and `Skipped: N`, which the JSON exports nowhere (§4). The JSON carries `go_module`,
`mutants_total` and `mutator_statistics`, which appear in no stdout line — stdout's
`Mutator coverage: 67.90%` is `mutations_coverage`, a different key — and it renders elapsed as
`9.17` against stdout's `9 seconds 170 milliseconds`. The JSON is read because its shape is a Go
struct with eleven json-tagged fields (`internal/report/internal/structure.go`, gremlins v0.6.0);
stdout's is a presentation (`fullRunReport` in the same package's `report.go`) and changes with it.

The set comparison, run against one clean run's `out.json` and its captured stdout — basename because
stdout prints no directory — prints `81 81 0 0`:

```sh
gremlins unleash ./internal/model --workers 4 -o out.json > out.stdout   # first run, fresh copy
python3 -c "
import json,os,re
P=re.compile(r'^\s*(KILLED|LIVED|NOT COVERED|TIMED OUT|NOT VIABLE|SKIPPED|RUNNABLE)\s+(\S+) at (\S+):(\d+):(\d+)\$')
s={m.groups() for m in map(P.match,open('out.stdout')) if m}
d=json.load(open('out.json'))
j={(m['status'],m['type'],os.path.basename(f['file_name']),str(m['line']),str(m['column'])) for f in d['files'] for m in f['mutations']}
print(len(s),len(j),len(j-s),len(s-j))"
```

## 3. `mutants_total` is not the total, and the entry must not use it

The entry's mutant count is the length of `files[].mutations[]`, cross-checked against a second
gremlins invocation. Neither half is the obvious choice, and both were got wrong once.

### 3.1 The field named total counts the tested subset

Measured on `./internal/model`:

| | |
|---|---|
| `mutants_total` | **55** |
| `mutants_killed` + `mutants_lived` | 55 |
| `mutants_killed` + `mutants_lived` + `mutants_not_covered` | **81** |
| rows in `files[].mutations[]` | **81** |

**The field excludes not-covered mutants**, so it names the *tested* subset while reading as the
population. A gate using it as a denominator silently drops 26 of 81 here — **32%** — and the
direction is flattering: the excluded mutants are exactly the ones nothing tested.

### 3.2 The second derivation is not another field of this file

**The entry's mutant count is the length of `files[].mutations[]`**, and it needs a second,
independent derivation or "over which mutants" is asserted rather than checkable. **That derivation
is not another field of the same file, and two drafts tried to make it one** — round 1 asserted
`killed + lived + not_covered`, the repair widened it to *every status the file reports*, and
grounding round 2 refuted both. The exemplar run `CLAUDE.md` points at is the counterexample to the
narrow form, and it is the run this release exists to make checkable: **1540 mutants, 1033 killed,
253 lived, 227 not covered, 27 timed out**, so `1033 + 253 + 227 = 1513` against a length of
**1540** — short by exactly the timeouts (`grep -n '1540 mutants' spec/0.35.0-candidates.md`).
Widening to the whole schema does not repair it, because the omitted statuses are precisely the
shortfall: the JSON exports counters for **four** statuses only — `mutants_killed`, `mutants_lived`,
`mutants_not_viable`, `mutants_not_covered` —
while `mutations[]` also carries `TIMED OUT`, `SKIPPED` and `RUNNABLE` rows. Measured on two
complete, untruncated files: a repeat `./internal/model` run, **81** rows against a four-counter sum
of **28** (53 `TIMED OUT` rows uncounted); the `./internal/engine` dry run, **1820** rows against
**279** (1541 `RUNNABLE` rows uncounted). Both are whole files, and a sum-based check rejects both.

**And even where the sums do agree, they are not independent.** `newReport`
(`internal/report/report.go`, gremlins v0.6.0) walks the mutant slice **once**, appending a row to
`files[…]` and incrementing that mutant's status counter in the same loop iteration; `mutants_total`
is then `lived + killed + notViable`, arithmetic over those counters. Counters and rows are one pass
counted twice, so comparing them inside a file gremlins wrote is a tautology — one that cannot fail
where it matters and does fail where it should not.

### 3.3 The second derivation is a dry run of the same package

**The second derivation is a second invocation: `--dry-run` over the same package.** A dry run
enumerates the mutants without testing them, in a separate process from a separate discovery pass.
Measured, its row count agrees with the real run to the digit:

| package | dry run `Runnable + Not covered` | dry run rows | real run rows |
|---|---|---|---|
| `./internal/model` | 55 + 26 = **81** | **81** | **81** |
| `./internal/engine` | 1541 + 279 = **1820** | **1820** | not run here (53 minutes at `v0.35.0`) |

**So the entry invokes gremlins twice per package — once `--dry-run`, once for real — and requires
the two `files[].mutations[]` lengths to be equal. It never sums status counters, in any
combination.** The dry run costs **about 33 seconds of wall clock** on `./internal/engine` against that
package's 53 minutes — a 96× saving, so the second derivation is close to free. **Two clocks, and the
entry means the outer one:** gremlins reports its own elapsed as `1 second 805 milliseconds`, which
excludes the coverage build it does first; `/usr/bin/time -p` around the same first-run-in-a-fresh-copy
invocation gives `real 32.88`. Quote the wall clock, because that is what a gate entry spends. Both
figures are run-local — re-derive with
`/usr/bin/time -p gremlins unleash ./internal/engine --dry-run -o out.json`. This is what makes the recorded mutant set
checkable rather than asserted — and this repository's own rule is that a claim bound to a named
artifact cannot rot silently.

**What that gives up, said plainly.** The dry run cross-checks the *population*, not the *statuses*.
`TIMED OUT`, `SKIPPED` and `RUNNABLE` have no counter anywhere in the eleven-key schema, so the entry
**records** the per-status histogram of `files[].mutations[]` (§5) and checks nothing against it.
That is the honest limit of a format fixed by upstream, not a gap to be closed by a cleverer sum.

## 4. The corruption tell is checked, because the timeout count is not in the file

### 4.1 What the entry can see

`CLAUDE.md` records a measured corruption: `--test-cpu N` makes gremlins pass `-cpu N` to
`exec.Command` as a single argument, so `go test` never starts, every real survivor becomes
`TIMED OUT`, and efficacy reads **100.00%** — *"it manufactures exactly the number someone would be
pleased by."*

**There is no timed-out key in the JSON.** stdout prints `Timed out: 0`; the machine-readable output
has `elapsed_time` and nothing else matching. Measured — the eleven top-level keys are listed in §2
and none of them counts timeouts. **So a gate reading the file cannot see the failure mode directly.**

**What it can see is the signature.** The entry refuses to report a score when
`mutants_lived == 0 && mutants_not_covered > 0`. Under the corruption every survivor moves out of
`lived` while `not_covered` is untouched — the coverage profile is collected before any mutant runs —
so the pair is exactly the fingerprint.

### 4.2 Neither the file nor the argv settles it

**It refuses rather than fails, and the reason is stronger than the one this section used to give.**
It cited `CLAUDE.md` for a peer repository hitting `Lived: 0` beside a non-zero not-covered count
honestly. That is not what `CLAUDE.md` recorded — the peer's honest case was the weaker `Lived: 0`
plus 100%, and `CLAUDE.md` went on to assert that the pair *with* a non-zero not-covered count could
not occur honestly. Read it *as an assertion* at `git show 0f7f164f^:CLAUDE.md` — from `0f7f164f`
onward those words appear in `CLAUDE.md` only as reported history, so quoting `HEAD` would
misattribute a claim that file now repudiates. **The assertion is false, and this cycle measured it.**
A two-file package — one function fully killed by a table test, one never called — run with no flags
at all:

```
Killed: 1, Lived: 0, Not covered: 6
Timed out: 0, Not viable: 0, Skipped: 0
Test efficacy: 100.00%
```

So the signature is **necessary under the corruption and not sufficient**.

**The section used to close by saying that what decides is the argv — whether `--test-cpu` was
passed. A run refutes that, and `CLAUDE.md` has been corrected for it in turn (`be3464c8`).**
Measured in **five** independent `rsync -a --exclude .git` copies of `HEAD`: the **first**
`gremlins unleash ./internal/model --workers 4 -o <file>` in a fresh directory returns
`Killed 50, Lived 5, Not covered 26, Timed out 0, 90.91%`; **every later run in that same directory —
byte-identical argv, with no `--test-cpu` anywhere — returns
`Killed 2, Lived 0, Not covered 26, Timed out 53, 100.00%`**, which is the refused signature with the
flag absent. Five clean first runs, eight corrupt later ones. Priming the test cache first
(`go test ./internal/model` twice before gremlins ever ran) does not prevent it, so the cache is not
the mechanism; the mechanism is unknown and was not chased past that. **So `--test-cpu` is
sufficient for the signature and not necessary, and reading the argv proves nothing in either
direction.**

**What replaces the argv test is procedural, and this release implements it rather than printing it
as a caution: the entry runs gremlins in a directory it creates for that run, and a run that is not
the first in its directory is not a result.** The `rsync` copy was already mandatory (§6) because
gremlins rewrites source in place; it is now also what makes the figure readable at all.

### 4.3 What the refusal says

**The refusal names the observation, not a cause, and not a remedy it cannot support.** An earlier
draft had it say *"re-run it without the flag"* — presupposing a flag that need not be there, in the
same breath as declining to assert corruption *"because on this evidence it cannot"*. On the
run-order finding that message is wrong exactly where the refusal is right: there is no flag to
remove, and re-running in place reproduces the refusal. What the entry emits instead:

> `mutants_lived` is 0 while `mutants_not_covered` is 26, so no score is reported. This pair occurs
> under `--test-cpu` corruption and also occurs honestly. Re-run as the first run in a fresh copy,
> and read the `Timed out` count on stdout — this file does not carry it.

It points at stdout because the timed-out count is what separated the two in every case measured this
cycle — **53** under the corruption and under a repeat run, **0** in the honest two-file probe — and
it is the one count the JSON does not export.

**`--test-cpu` is never passed and `--workers` always is.** The first is *a* corruption, not the only
one; the second is what makes two runs comparable. Both belong in the entry rather than in an
operator's memory, which is where they have lived for three releases.

## 5. The entry does not grade

**No efficacy threshold, no survivor budget, no ratchet on the score.** `CLAUDE.md`'s rule is that
survivors must be *classified*, not driven down — v0.34.1's ten survivors in one file were two
equivalent mutants, three on an undocumented backoff schedule, and four on a documented contract with
no boundary test, and only the last group was worth acting on. A number that gates invites the three
cheap ways to move it.

**What the entry gates on is completeness**: both output files exist and parse, the real run's
`elapsed_time` is non-zero, its `files[].mutations[]` length equals the dry run's (§3), and §4's
signature is absent. A run that fails any of these did not happen, whatever it printed.

**The score is recorded, not judged.** `test_efficacy`, `mutations_coverage`, the four exported
status counts, the per-status histogram of `files[].mutations[]` — which is the only place `TIMED
OUT`, `SKIPPED` and `RUNNABLE` appear at all — and the per-file mutation list are written to a
committed artifact per package, so a later claim about a package's mutation score has something to be
checked against. `internal/engine/validate.go` — **46 survivors at 24.6% efficacy**, measured at the
`v0.35.0` pre-release run and recorded there rather than here
(`grep -n '46 survivors' spec/0.35.0-candidates.md`; `CLAUDE.md` rounds it to 25%) — is the standing
backlog. This release does not re-measure it: it is the exemplar of a figure with nowhere to live.

**Load-sensitivity is stated, not corrected — and the record behind it is weaker than it reads.**
`CLAUDE.md`'s mutation bullet carries two figures with no state attached
(`grep -n '92 timeouts' CLAUDE.md`): a **second** run on a busy machine turning 75 kills into 80
timeouts, and the same package measuring 92 timeouts busy against 88 idle. This release does not
re-measure either, so both are cited rather than asserted. The first also does not isolate load: §4
measures a *second* run in one directory turning **50 kills into 53 timeouts with the machine idle**,
which is the same shape, so what that observation distinguishes is unsettled. The entry runs at the
two decision points `CLAUDE.md` already names — when a version's new engine surface is complete, and
before the release tag — and never while task gates are running.

## 6. Non-Goals

1. **Not in `quality_gate`'s per-task run.** Ten seconds for one small package is cheap; the engine's
   **1820** mutants are not (§1 derives that count at `HEAD`), and a gate that runs per task would
   run it hundreds of times.
2. **No efficacy threshold.** §5 states why, and it is this repository's own standing rule.
3. **No use of `-D`/`--diff`.** It does not work in v0.6.0 — measured, an empty diff produced the
   unrestricted set — and it has three open upstream bugs. `CLAUDE.md` records that set as **80**
   mutants for `./internal/model` when the bullet was written — its `--test-cpu` comparison table's
   unflagged row is `49 + 5 + 0 + 26` (`grep -n 'no flag |' CLAUDE.md`) — while the same package
   carries **81** at `HEAD` (§1). The figure is dated; the conclusion is not. Per-package narrowing
   replaces it exactly.
4. **No repair of gremlins.** The missing timeout field, the `mutants_total` naming, and the
   single-pass counters §3 measures are all upstream's; this release works around them and says so.
5. **No run inside the repository, and no second run in one copy.** A directory created for that run,
   always — because gremlins rewrites source in place, which is why this non-goal existed before,
   and because §4 measures that a later run in the same directory is not a result.

## 6a. An open question carried in from `spec/candidates.md`

**This section takes no decision.** It is an *Undecided* entry moved out of `spec/candidates.md` —
where every row names the decision nobody has taken — into the release that owns its subject, which
is the gremlins run. It is recorded here as a question. §7 does not depend on it and no row of §7
tests it.

### 6a.1 `t.Parallel()` in `internal/engine`

**The undecided part is `internal/engine` alone**, for one reason: that is the package
`gremlins unleash ./internal/engine` mutates — **1820** mutants, derived at `HEAD` in §1. The
`internal/cli` half is being applied separately and is **not** part of this question.

That half is measured rather than argued. Re-derived at `HEAD`, with the counting rule beside each
figure and the ones that did not reproduce marked as such:

| the entry's figure | at `HEAD` | how it was derived |
|---|---|---|
| `internal/cli` is **1,743** serial test functions | **does not reproduce as an `internal/cli` figure.** The package holds **1,051** top-level `func Test…`; **1,781** is the *repository-wide* count, which is what 1,743 tracks | `rg '^func Test' internal/cli --no-filename \| wc -l` against `rg '^func Test' -g '*_test.go' --no-filename \| wc -l` |
| **1,116** of them fork the tp binary | **the counting rule decides which number this is.** `runTP(` appears at **1,120** *call sites*; **736** top-level test *functions* have a body calling any `runTP*` helper | `rg -o 'runTP\(' internal/cli --no-filename \| wc -l`, against a `re.split(r'(?m)^func ', src)` walk over `internal/cli/*_test.go` counting bodies matching `\brunTP[A-Za-z]*\(` |
| I/O-bound at **8%** of ten cores — **39.7 s** CPU inside **50.5 s** wall | **holds in shape.** First run in a fresh copy: `real 47.37 user 16.14 sys 20.75` — **36.9 s** CPU inside **47.4 s** wall, **7.8%** of ten cores | `/usr/bin/time -p go test ./internal/cli -count=1` in an `rsync -a --exclude .git` copy of `HEAD` |
| the **7** files that `t.Chdir` are skipped, because Go panics on the pair | **holds exactly: 7** | `rg -l 't\.Chdir' internal/cli \| wc -l` |
| **zero** `os.Setenv` in the package | **holds: 0.** The package-level-variable-write half of that claim was not re-derived here | `rg -c 'os\.Setenv' internal/cli \| wc -l` |
| `t.Parallel()` added to **1,026** functions across **225** files | **not a property of the tree** — it names an edit made in a copy that no longer exists. At `HEAD` `internal/cli` holds **234** `*_test.go` files, 7 of them `t.Chdir`, and the repository holds **0** `t.Parallel()` calls | `ls internal/cli/*_test.go \| wc -l`; `rg -c 't\.Parallel\(\)' -g '*_test.go' --no-filename` |
| **54 s → 14 s**, and **15.8 s** under `-race`, four consecutive green runs, `go vet` clean | **borrowed, not re-run.** Measured in an `rsync` copy at v1.0.0's audit round 5; that copy is gone, and the 54 s baseline reads 47.0 s at `HEAD` (row 3) | — |

### 6a.2 Why `internal/engine` is the half that stays undecided

gremlins runs the mutated package's **own** tests once per mutant. This repository has already
measured those runs as load-sensitive — `CLAUDE.md` records the same package at **92** timeouts busy
against **88** idle, and default settings driving load average from **8** to **177**
(`grep -n 'load average' CLAUDE.md`) — and `t.Parallel()` multiplies concurrency *inside* each mutant
by gremlins' own `--workers`. So the question is whether parallelizing `internal/engine` leaves the
mutation signal intact, and the entry names the only answer it will accept: a **paired run —
`--workers` pinned, before and after, compared on efficacy and the timeout count rather than on wall
time.** Until that measurement exists, `internal/cli` is free to parallelize and `internal/engine`
is not.

### 6a.3 The protocol that paired run needs, which the entry did not state

**The entry closed by telling the reader to watch for `Lived: 0` beside `Not covered > 0`. That
instruction is stale, and correcting it is why the entry belongs to this release rather than to
`spec/candidates.md`.** §4.2 measures both halves: the signature is **necessary under the corruption
and not sufficient** — an honest two-file probe reaches it with no flags at all — and **the argv does
not settle it either**, because a later run in a directory that already held one returns the refused
signature with no `--test-cpu` anywhere in it. §4.2 carries those counts and they are not restated
here.

**So each arm must be the FIRST gremlins run in its own fresh `rsync -a --exclude .git` copy** — one
copy for the serial tree, one for the parallelized tree, neither directory reused. Without that the
"after" arm is confounded by run order and the experiment answers nothing: it would return the
refused signature and read as a mutation signal that `t.Parallel()` destroyed. §6's fifth non-goal
states the same rule for this release's own runs, and this is that rule applied to an experiment the
release does not itself run.

**The mechanism behind the run-order effect is unknown.** It was not chased past ruling out the test
cache (§4.2), and nothing here proposes one. A protocol that works without a mechanism is what the
paired run needs; a guess at the mechanism is not.

## 7. Tests

Every row derives from a numbered decision, names the artifact it depends on, and names a mutant that
must fail it.

| # | from | assertion | the mutant that must fail it |
|---|---|---|---|
| 1 | §3.1 | on the measured `./internal/model` output, the entry reports **81** mutants | read `mutants_total`, which reports 55 — the shipped-looking choice, wrong by 32% |
| 2 | §3.3 *cross-check* | the count is cross-checked against a `--dry-run` of the same package: two outputs whose `files[].mutations[]` lengths are 81 and 81 pass, 81 against a truncated 55 is rejected — and an output with **1540** rows whose four exported counters sum to **1513** is **accepted**, because the entry sums no counters | cross-check inside one file (any sum of `killed`/`lived`/`not_viable`/`not_covered` against the row count), which rejects the recorded 1540/1513 run and the measured 81-vs-28 and 1820-vs-279 files, all three of them complete |
| 3 | §4.1 | an output with `mutants_lived: 0` and `mutants_not_covered: 26` is refused | report the efficacy, which is 100.00% under the corruption |
| 4 | §4.3 *honest* | the refusal says the result is unusable, names `--test-cpu` only as one cause among others, and does **not** assert corruption or instruct the operator to remove a flag | word it as a corruption verdict, or as *"re-run it without the flag"* — both wrong for the honest 100% and for the flagless repeat run §4 measures |
| 5 | §4.1 *clean* | `mutants_lived: 0` with `mutants_not_covered: 0` is **accepted** — an exhaustive suite is not a corrupt run | trigger on `lived == 0` alone, which refuses a genuinely perfect package |
| 6 | §5 | the entry's exit status does not depend on `test_efficacy` — asserted by feeding two outputs differing only in that field | gate on the score, which is the rule this repository has held since v0.34.1 |
| 7 | §4.3 *flags* | the invoked command lines contain `--workers` and do **not** contain `--test-cpu` — asserted on recorded argv under a stub binary. This pins what the entry invokes; per §4 it certifies nothing about whether a run was corrupt | assert on the script's text, which this repository has measured three times to be insufficient |
| 8 | §4.2 *fresh* | each gremlins invocation runs in a directory the entry created for that run, and a second invocation in a directory that already holds one is refused — asserted on the recorded working directory under the same stub | reuse one directory across runs, which this cycle measured returning `Killed 2, Lived 0, Timed out 53, 100.00%` on the second run in five of five copies |

**Rows 7 and 8 run the entry under a stub and read argv and cwd.** Reading a script for a flag was
measured insufficient three times here — a bare `Contains` was satisfied by the flag appearing in a
comment, and a per-line scan kept the last matching line with no notion of the `if`/`else` around it.
