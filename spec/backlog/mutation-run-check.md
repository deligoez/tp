# tp — The mutation-run check (a script and a CLAUDE.md line, not a release)

A backlog entry, named by slug; it is not a release and takes no version. Its measurements are in
`mutation-run-check-measurements.md` beside it; this file stands without them.

Class: **tool** — ships as `scripts/check-mutation-run.sh` plus one `CLAUDE.md` line. Nothing in the
review or audit loop changes, so no cycle is budgeted for it.

## 1. What the check establishes

`CLAUDE.md` has carried mutation testing as a trigger, not a gate, since v0.34.1, with a standing
warning that **the survivor count is not a score to drive down**. What is missing is weaker: nothing
records that a run **happened**, or **over which mutants**, so every figure quoted from one is
unverifiable afterwards. The check **establishes that a run completed over a known mutant set, and
records the set**. A package is the unit of a run (default-mode gremlins runs only the mutated
package's own tests); counts and timings are in the sidecar under "How the figures were produced".

## 2. The check reads the machine-readable output, never stdout

`gremlins unleash -o <file>` writes JSON with eleven top-level keys: `go_module`,
`files[{file_name, mutations[{type, status, line, column}]}]`, `test_efficacy`, `mutations_coverage`,
`elapsed_time`, `mutator_statistics`, `mutants_total`, `mutants_killed`, `mutants_lived`,
`mutants_not_viable`, `mutants_not_covered`. **stdout is not parsed.** Not because the streams carry
different mutants — stdout prints the whole `files[]` list, and the sidecar's "The set comparison of
stdout against the JSON" shows the two equal — but because the JSON's shape is a Go struct with
eleven json-tagged fields (`internal/report/internal/structure.go`, gremlins v0.6.0) while stdout's is
a presentation (`fullRunReport` in the same package's `report.go`) that changes with it.

## 3. `mutants_total` is not the total, and the check must not use it

**The check's mutant count is the length of `files[].mutations[]`.** `mutants_total` is
`killed + lived + not_viable` and **excludes not-covered mutants**, so it names the *tested* subset
while reading as the population, and the direction is flattering: the excluded mutants are exactly
the ones nothing tested. On `./internal/model` the field reads 55 against 81 rows.

**The second derivation is a `--dry-run` of the same package, never a sum of counters.** A dry run
enumerates the mutants without testing them, in a separate process, and its row count agrees with
the real run to the digit on both packages measured. **So the check invokes gremlins twice per
package and requires the two `files[].mutations[]` lengths to be equal.** It never sums status
counters: the JSON exports counters for four statuses while `mutations[]` also carries `TIMED OUT`,
`SKIPPED` and `RUNNABLE` rows, and counters and rows are one pass counted twice, so a sum inside one
file is a tautology (sidecar, "The second derivation is not another field of this file" and "The dry
run's cost"). What that gives up: the dry run cross-checks the *population*, not the *statuses*, so
the check **records** the per-status histogram of `files[].mutations[]` (§5) and checks nothing
against it — the honest limit of a format fixed by upstream, not a gap to be closed by a cleverer sum.

## 4. The corruption tell is checked, because the timeout count is not in the file

`CLAUDE.md` records a measured corruption: `--test-cpu N` makes gremlins pass `-cpu N` to
`exec.Command` as a single argument, so `go test` never starts, every real survivor becomes
`TIMED OUT`, and efficacy reads **100.00%**. No key in §2's list counts timeouts, so a check reading
the file cannot see that directly. **What it can see is the signature: the check refuses to report a
score when `mutants_lived == 0 && mutants_not_covered > 0`.** Under the corruption every survivor
moves out of `lived` while `not_covered` is untouched — the coverage profile is collected before any
mutant runs — so the pair is exactly the fingerprint.

**Neither the file nor the argv settles it.** The signature is necessary under the corruption and
not sufficient — an honest two-file package with one uncalled function reaches it with no flags at
all — and a second `gremlins unleash` in a directory that already holds one returns the refused
signature with no `--test-cpu` anywhere in its argv, in every copy tried; the mechanism is unknown
(sidecar, "Neither the file nor the argv settles it"). **What replaces the argv test is procedural,
and the check implements it rather than printing it as a caution: it runs gremlins in a directory it
creates for that run, and a run that is not the first in its directory is not a result.** The
`rsync` copy was already mandatory (§6); it is now also what makes the figure readable at all.

**The refusal names the observation, not a cause, and not a remedy it cannot support.** An earlier
draft had it say *"re-run it without the flag"*; there is no flag to remove, and re-running in place
reproduces the refusal. What the check emits instead: *`mutants_lived` is 0 while
`mutants_not_covered` is 26, so no score is reported. This pair occurs under `--test-cpu` corruption
and also occurs honestly. Re-run as the first run in a fresh copy, and read the `Timed out` count on
stdout — this file does not carry it.* It points at stdout because the timed-out count is the one
count the JSON does not export. **`--test-cpu` is never passed and `--workers` always is** — the
first is *a* corruption, not the only one; the second is what makes two runs comparable.

## 5. The check does not grade

**No efficacy threshold, no survivor budget, no ratchet on the score** — `CLAUDE.md`'s rule is that
survivors are *classified*, not driven down, and a number that gates invites the cheap ways to move
it. **What the check gates on is completeness**: both output files exist and parse, the real run's
`elapsed_time` is non-zero, its `files[].mutations[]` length equals the dry run's (§3), and §4's
signature is absent. A run that fails any of these did not happen, whatever it printed. **The score
is recorded, not judged**: `test_efficacy`, `mutations_coverage`, the four exported status counts,
the per-status histogram of `files[].mutations[]` and the per-file mutation list are written to a
committed artifact per package, so a later claim about a package's mutation score has something to
be checked against (sidecar, "The standing backlog figure"). The check runs at the two decision
points `CLAUDE.md` already names — when a version's new engine surface is complete, and before the
release tag — and never while task gates are running (sidecar, "Load-sensitivity").

## 6. Non-Goals

1. **Not in `quality_gate`'s per-task run.** One small package is cheap; the engine's mutant set is
   not, and a gate that runs per task would run it hundreds of times. `spec/backlog/gate-sequence.md`
   carries the reciprocal non-goal.
2. **No efficacy threshold.** §5 states why, and it is this repository's own standing rule.
3. **No use of `-D`/`--diff`.** It does not work in v0.6.0 — measured, an empty diff produced the
   unrestricted set — and it has three open upstream bugs. Per-package narrowing replaces it exactly.
4. **No repair of gremlins.** The missing timeout field, the `mutants_total` naming and the
   single-pass counters are all upstream's; the check works around them and says so.
5. **No run inside the repository, and no second run in one copy.** A directory created for that
   run, always — gremlins rewrites source in place, and §4 measures that a later run is not a result.
6. **Not a release.** A script, its tests and one `CLAUDE.md` line; no spec cycle, no round, no
   version. Whether `internal/engine`'s tests may be marked `t.Parallel()` without disturbing the
   mutation signal is not answered here; `spec/undecided.md` carries it as "`t.Parallel()` in the
   engine package".

## 7. Tests

Every row names a mutant that must fail it; rows 7 and 8 run the check under a stub and read argv
and cwd, because reading a script for a flag was measured insufficient three times here.

| # | from | assertion | the mutant that must fail it |
|---|---|---|---|
| 1 | §3 | on the measured `./internal/model` output, the check reports **81** mutants | read `mutants_total`, which reports 55 — the shipped-looking choice |
| 2 | §3 *cross-check* | the count is cross-checked against a `--dry-run` of the same package: two outputs whose `files[].mutations[]` lengths are 81 and 81 pass, 81 against a truncated 55 is rejected — and an output with **1540** rows whose four exported counters sum to **1513** is **accepted**, because the check sums no counters | cross-check inside one file (any sum of `killed`/`lived`/`not_viable`/`not_covered` against the row count), which rejects the recorded 1540/1513 run and the measured 81-vs-28 and 1820-vs-279 files, all three of them complete |
| 3 | §4 | an output with `mutants_lived: 0` and `mutants_not_covered: 26` is refused | report the efficacy, which is 100.00% under the corruption |
| 4 | §4 *honest* | the refusal says the result is unusable, names `--test-cpu` only as one cause among others, and does **not** assert corruption or instruct the operator to remove a flag | word it as a corruption verdict, or as *"re-run it without the flag"* — both wrong for the honest 100% and for the flagless repeat run §4 measures |
| 5 | §4 *clean* | `mutants_lived: 0` with `mutants_not_covered: 0` is **accepted** — an exhaustive suite is not a corrupt run | trigger on `lived == 0` alone, which refuses a genuinely perfect package |
| 6 | §5 | the check's exit status does not depend on `test_efficacy` — asserted by feeding two outputs differing only in that field | gate on the score, which is the rule this repository has held since v0.34.1 |
| 7 | §4 *flags* | the invoked command lines contain `--workers` and do **not** contain `--test-cpu` — asserted on recorded argv under a stub binary. This pins what the check invokes; per §4 it certifies nothing about whether a run was corrupt | assert on the script's text, which this repository has measured three times to be insufficient |
| 8 | §4 *fresh* | each gremlins invocation runs in a directory the check created for that run, and a second invocation in a directory that already holds one is refused — asserted on the recorded working directory under the same stub | reuse one directory across runs, which the sidecar measured returning the refused signature on the second run in every copy |
