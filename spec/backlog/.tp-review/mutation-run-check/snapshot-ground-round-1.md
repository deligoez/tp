# tp v1.46.0 — Mutation score as a gate entry

> **This file is decisions.** Every claim about gremlins' output below was produced by **running it**
> — `gremlins unleash ./internal/model --workers 4 -o out.json` in an `rsync` copy — not by reading
> its documentation. Two of the four decisions exist because the run contradicted what its own field
> names say.

## 1. Overview

`CLAUDE.md` has carried mutation testing as a trigger, not a gate, since v0.34.1, with a standing
warning that **the survivor count is not a score to drive down**. That stays true. What is missing is
weaker and more important: nothing records that a run **happened**, or **over which mutants**, so
every figure quoted from one is unverifiable afterwards.

This release adds one gate entry that **establishes a run completed over a known mutant set, and
records the set**. It does not grade the result.

**Cheap, once narrowed.** `./internal/model` is **81 mutants in 10 seconds**. The hour `CLAUDE.md`
warns about is `./internal/engine`'s 1540. Per-package narrowing is an exact substitute — in default
mode gremlins runs only the mutated package's own tests, so an unchanged package's mutants cannot
change verdict.

## 2. The entry reads the machine-readable output, never stdout

`gremlins unleash -o <file>` writes JSON. Measured shape:

```
go_module, files[{file_name, mutations[{type, status, line, column}]}],
test_efficacy, mutations_coverage, elapsed_time, mutator_statistics,
mutants_total, mutants_killed, mutants_lived, mutants_not_viable, mutants_not_covered
```

**stdout is not parsed.** It carries a line the JSON does not (§4) and is otherwise the same numbers
in a form that changes with the tool's presentation.

## 3. `mutants_total` is not the total, and the entry must not use it

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

**The entry's mutant count is the length of `files[].mutations[]`**, and it asserts that this equals
`killed + lived + not_covered`. Two independent derivations of one number, from the same file, is
what makes "over which mutants" checkable rather than asserted — and this repository's own rule is
that a claim bound to a named artifact cannot rot silently.

## 4. The corruption tell is checked, because the timeout count is not in the file

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

**It refuses rather than fails.** `CLAUDE.md` records that a peer repository hit a package producing
`Lived: 0` beside a non-zero not-covered count **honestly**, and doubted a correct result. The entry
says *this result is not usable, re-run it*, and names the flag; it does not assert the run was
corrupt.

**`--test-cpu` is never passed and `--workers` always is.** The first is the corruption; the second
is what makes two runs comparable. Both belong in the entry rather than in an operator's memory,
which is where they have lived for three releases.

## 5. The entry does not grade

**No efficacy threshold, no survivor budget, no ratchet on the score.** `CLAUDE.md`'s rule is that
survivors must be *classified*, not driven down — v0.34.1's ten survivors in one file were two
equivalent mutants, three on an undocumented backoff schedule, and four on a documented contract with
no boundary test, and only the last group was worth acting on. A number that gates invites the three
cheap ways to move it.

**What the entry gates on is completeness**: the output file exists, parses, carries a non-zero
`elapsed_time`, its two mutant-count derivations agree, and §4's signature is absent. A run that
fails any of these did not happen, whatever it printed.

**The score is recorded, not judged.** `test_efficacy`, `mutations_coverage`, the four status counts
and the per-file mutation list are written to a committed artifact per package, so a later claim about
a package's mutation score has something to be checked against. `internal/engine/validate.go` at 25%
efficacy with 46 survivors is the standing backlog, and today that figure exists only in a paragraph.

**Load-sensitivity is stated, not corrected.** A run on a busy machine turned 75 kills into 80
timeouts; the same package measured 92 timeouts busy and 88 idle. The entry runs at the two decision
points `CLAUDE.md` already names — when a version's new engine surface is complete, and before the
release tag — and never while task gates are running.

## 6. Non-Goals

1. **Not in `quality_gate`'s per-task run.** Ten seconds for one small package is cheap; the engine's
   1540 mutants are not, and a gate that runs per task would run it hundreds of times.
2. **No efficacy threshold.** §5 states why, and it is this repository's own standing rule.
3. **No use of `-D`/`--diff`.** It does not work in v0.6.0 — measured, an empty diff produced all 80
   mutants — and has three open upstream bugs. Per-package narrowing replaces it exactly.
4. **No repair of gremlins.** The missing timeout field and the `mutants_total` naming are upstream's;
   this release works around both and says so.
5. **No run inside the repository.** `rsync` copy, always, because gremlins rewrites source in place.

## 7. Tests

Every row derives from a numbered decision, names the artifact it depends on, and names a mutant that
must fail it.

| # | from | assertion | the mutant that must fail it |
|---|---|---|---|
| 1 | §3 | on the measured `./internal/model` output, the entry reports **81** mutants | read `mutants_total`, which reports 55 — the shipped-looking choice, wrong by 32% |
| 2 | §3 *cross-check* | an output whose `files[].mutations[]` length disagrees with `killed+lived+not_covered` is rejected as incomplete | derive the count once, so a truncated file passes with a self-consistent smaller number |
| 3 | §4 | an output with `mutants_lived: 0` and `mutants_not_covered: 26` is refused, naming `--test-cpu` | report the efficacy, which is 100.00% under the corruption |
| 4 | §4 *honest* | the refusal says the result is unusable and does **not** assert corruption | word it as a corruption verdict, which is wrong for the peer repository's honest 100% |
| 5 | §4 *clean* | `mutants_lived: 0` with `mutants_not_covered: 0` is **accepted** — an exhaustive suite is not a corrupt run | trigger on `lived == 0` alone, which refuses a genuinely perfect package |
| 6 | §5 | the entry's exit status does not depend on `test_efficacy` — asserted by feeding two outputs differing only in that field | gate on the score, which is the rule this repository has held since v0.34.1 |
| 7 | §4 *flags* | the invoked command line contains `--workers` and does **not** contain `--test-cpu` — asserted on recorded argv under a stub binary | assert on the script's text, which this repository has measured three times to be insufficient |

**Row 7 runs the entry under a stub and reads argv.** Reading a script for a flag was measured
insufficient three times here — a bare `Contains` was satisfied by the flag appearing in a comment,
and a per-line scan kept the last matching line with no notion of the `if`/`else` around it.
