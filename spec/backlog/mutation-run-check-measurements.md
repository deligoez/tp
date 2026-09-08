# mutation-run-check — measurements

Supplemental material for `mutation-run-check.md`; the spec stands without it.

## How the figures were produced

> Every figure below about gremlins' output was produced by **running it**,
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

**Cheap, once narrowed.** `gremlins unleash ./internal/model --workers 4 -o out.json` at `HEAD` is
**81 mutants in about 10 seconds** of wall clock (`elapsed_time` 9.17 on the run recorded here; that
figure is run-local, and four first runs this cycle spread 9.17–9.78 — the three others are in
`spec/backlog/.tp-review/mutation-run-check/ground-round-2.ndjson` under `u10`, so quote the count
and not the second). The expensive package is `./internal/engine`:
`gremlins unleash ./internal/engine --dry-run` reports `Runnable: 1541, Not covered: 279` —
**1820 mutants** — and the same dry run with `-o` writes 1820 rows into `files[].mutations[]`, so the
printed counts and the row list agree. (§3 explains why that particular agreement is *not* the
cross-check the entry gates on: both come from one pass.) `CLAUDE.md`'s 1540 and its 53 minutes are
the same package measured at tag `v0.35.0` — `grep -n '1540 mutants' spec/0.35.0-candidates.md`
retrieves the breakdown — so budget against 1820. Per-package narrowing is an exact substitute: in
default mode gremlins runs only the mutated package's own tests, so an unchanged package's mutants
cannot change verdict.

## The set comparison of stdout against the JSON

Matching records on (status, mutator type, file basename, line, column) for the `./internal/model`
run: |stdout| **81**, |JSON| **81**, stdout-only **0**, JSON-only **0**. stdout prints the whole
`files[]` list, mutant for mutant, in every field the JSON carries. stdout's `Mutator coverage: 67.90%`
is `mutations_coverage`, a different key — and it renders elapsed as `9.17` against stdout's
`9 seconds 170 milliseconds`.

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

## The second derivation is not another field of this file

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

## The dry run's cost

Measured, the dry run's row count agrees with the real run to the digit:

| package | dry run `Runnable + Not covered` | dry run rows | real run rows |
|---|---|---|---|
| `./internal/model` | 55 + 26 = **81** | **81** | **81** |
| `./internal/engine` | 1541 + 279 = **1820** | **1820** | not run here (53 minutes at `v0.35.0`) |

The dry run costs **about 33 seconds of wall clock** on `./internal/engine` against that
package's 53 minutes — a 96× saving, so the second derivation is close to free. **Two clocks, and the
entry means the outer one:** gremlins reports its own elapsed as `1 second 805 milliseconds`, which
excludes the coverage build it does first; `/usr/bin/time -p` around the same first-run-in-a-fresh-copy
invocation gives `real 32.88`. Quote the wall clock, because that is what a gate entry spends. Both
figures are run-local — re-derive with
`/usr/bin/time -p gremlins unleash ./internal/engine --dry-run -o out.json`. This is what makes the recorded mutant set
checkable rather than asserted — and this repository's own rule is that a claim bound to a named
artifact cannot rot silently.

## Neither the file nor the argv settles it

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

The refusal points at stdout because the timed-out count is what separated the two in every case
measured this cycle — **53** under the corruption and under a repeat run, **0** in the honest two-file
probe — and it is the one count the JSON does not export.

## The standing backlog figure

`internal/engine/validate.go` — **46 survivors at 24.6% efficacy**, measured at the
`v0.35.0` pre-release run and recorded there rather than here
(`grep -n '46 survivors' spec/0.35.0-candidates.md`; `CLAUDE.md` rounds it to 25%) — is the standing
backlog. This release does not re-measure it: it is the exemplar of a figure with nowhere to live.

## Load-sensitivity

**Load-sensitivity is stated, not corrected — and the record behind it is weaker than it reads.**
`CLAUDE.md`'s mutation bullet carries two figures with no state attached
(`grep -n '92 timeouts' CLAUDE.md`): a **second** run on a busy machine turning 75 kills into 80
timeouts, and the same package measuring 92 timeouts busy against 88 idle. This release does not
re-measure either, so both are cited rather than asserted. The first also does not isolate load: §4
measures a *second* run in one directory turning **50 kills into 53 timeouts with the machine idle**,
which is the same shape, so what that observation distinguishes is unsettled. The entry runs at the
two decision points `CLAUDE.md` already names — when a version's new engine surface is complete, and
before the release tag — and never while task gates are running.

## `-D`/`--diff`: the dated figure

`CLAUDE.md` records the unrestricted set as **80** mutants for `./internal/model` when the bullet was
written — its `--test-cpu` comparison table's unflagged row is `49 + 5 + 0 + 26`
(`grep -n 'no flag |' CLAUDE.md`) — while the same package carries **81** at `HEAD`. The figure is
dated; the conclusion is not.

## Routed here at the 2026-09-08 re-verification

One item from the candidates files lands on this spec's subject. It is recorded in this sidecar; the
spec body is not edited.

- **`internal/engine/bookkeeping.go` is the standing survivor cluster nothing carries.** v0.35.0's
  pre-release run measured it at **0 killed / 4 lived / 17 not covered** — a file with no mutant
  killed at all, which is the one reading that separates "the suite tests this weakly" from "the
  suite does not reach this". The record in `spec/0.35.0-candidates.md` item 16 is accurate and its
  `CLAUDE.md` correction was applied, but the figure itself has been carried by no file since; it
  belongs with this spec's other standing figure. Re-derive rather than quote — the rule for that
  run, including the fresh-copy requirement, is under *How the figures were produced* above. Source:
  `spec/0.35.0-candidates.md` item 16.

## Decided at the 2026-09-08 decision pass

From `spec/undecided.md`, *`t.Parallel()` in the engine package*, which was moved out of this check
because it took no decision on it.

**Decided: `internal/engine` stays serial.** `internal/cli` remains free to parallelize — `fa68051b`
applied it there.

**The paired run that could change that is the first execution of this check**, which is when it is
cheapest: someone is already running gremlins and reading its output, so the second arm costs one
extra copy rather than a dedicated session. The protocol is the one under *How the figures were
produced* and *Neither the file nor the argv settles it* above — each arm the **first** gremlins run
in its own fresh `rsync -a --exclude .git` copy, one serial tree and one parallelized tree, neither
directory reused, `--workers` pinned, compared on **efficacy and the timeout count** rather than on
wall time. Without the fresh-copy rule the "after" arm returns the corruption signature and reads as
a mutation signal `t.Parallel()` destroyed.
