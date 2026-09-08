# guards-read-what-production-reads — measurements

Supplemental material for `guards-read-what-production-reads.md`; the spec stands without it. Every
block below was moved here on 2026-09-08 from `refusals-that-name-nothing-measurements.md`, which had
in turn taken them verbatim from the spec body or from the former guard-helper spec. The conventions
every mutant below was run under — the `rsync -a --exclude .git ./ <copy>/` copy outside the
repository, the `git status --porcelain -- internal/` check with its control, and the rule that no
line number is cited in the spec body — are in that file's **Preamble** and are unchanged here.

## §4 — §7.2's `meaning` column

Reading the four `meaning` cells of §7.2's field table in `spec/1.0.0.md` — the rows whose first
cell is `verdict`, `kind`, `tier` and `partial_kind`, named rather than cited by line because the
range this paragraph first carried was eleven lines off and pointed at §7.2's heading instead of its
table:

| cell | what §7.2's `meaning` column says | enum values present there |
|---|---|---|
| `verdict` | *one of §3's six* | none |
| `kind` | *one of §4.1's seven*, plus `` `NOT-A-CLAIM` ``, `` `tier` ``, `` `evidence` ``, `` `note` `` | **none** — the backticked tokens are one *verdict* value and three field names |
| `tier` | *one of §4.1's tier table* | none |
| `partial_kind` | `two-readings`, `reason-not-conclusion`, `true-when-written` | all three |

## From the guard spec

The guard-helper spec (formerly `13a-guards-read-what-production-reads.md`, absorbed into the refusals
spec as tasks on 2026-09-08 and then deleted) carried the measurements below. Its §2 table and its §8 meta-paragraphs are
copied verbatim; the line numbers in them were measured against `HEAD` at the time and are not
re-derived here.

### The guard spec's §2 — what is true today, measured

Every row is an edit to `spec/1.0.0.md` in a copy, then `go test -count=1`. The recipe is one line:

```
rsync -a --exclude .git . /tmp/probe && cd /tmp/probe   # then edit spec/1.0.0.md, then:
go test -count=1 ./internal/engine ./internal/cli
```

**Every row starts from a pristine `spec/1.0.0.md`.** The recipe above does not say so and the rows
require it: keep a copy of the file beside the probe and restore it between rows, or the mutants
stack and no row after the first measures what its cell claims.

"13th verb" below means `probed` appended to §2.1's real arms row — the defect a reader is trying to
catch. A **silent** row is the dangerous one: a real defect present, suite green.

| # | the input | packages run | result at `HEAD` |
|---|---|---|---|
| 1 | nothing changed | engine + cli | green |
| 2 | 13th verb alone | engine | **red** — the guard works when nothing is hiding |
| 3 | an unfenced decoy `\| **verb** \|` row **above** §2.1, plus the 13th verb | engine | red — the file-wide-first-match door is **closed** |
| 4 | an unfenced decoy row **inside** §2.1, plus the 13th verb | engine | red at `floor_test.go:1298` |
| 5 | a **fenced** decoy row inside §2.1, plus the 13th verb | engine | red at `floor_test.go:1298` — **closed, and the handover said open** |
| 6 | a **fenced** decoy row inside §2.1, spec otherwise **correct** | engine | **red — a false failure**: §2.1 step 1 rules a fenced block non-content, and quoting the arms table reddens the suite |
| 7 | a fenced quotation of the line `### 2.1 The floor` **above** §2.1 carrying a 12-verb decoy row, plus the 13th verb | engine + cli | **green — silent** |
| 8 | a fenced block inside §2.1 carrying a 12-verb decoy row and then a `### ` line, plus the 13th verb | engine + cli | **green — silent** |
| 9 | a fenced `### ` line inside §2.1, spec otherwise **correct** | engine | **red — a false failure**, and the message says *"a second one, fenced or not"* when there are **zero** |
| 10 | a 13th verb spelled `` `Re-ran` `` | engine + cli | **green — silent** |
| 11 | 13th and 14th verbs spelled `` `re‑ran` `` (U+2011) and `` `ran2` `` | engine + cli | **green — silent** |
| 12 | a fenced `### 7.3 …` line inside §7.2 above its table | engine | red in **three** tests, at **two** locations — `TestTheAllowedKeySetIsExactlySection72sTable` and `TestEveryFieldSection72NamesHasARejectionCase` at `groundrow_test.go:624`, and `TestThePerVerdictTableIsTheOneSection72States` at `groundtierrule_test.go:161`, which is the third helper in another file — a false failure |
| 13 | a fenced decoy field table inside §7.2 above the real one | engine | red in both §7.2 field callers — a false failure |
| 14 | a real 14th field `` `bogus` `` in §7.2's table | engine | red — the field guard works when nothing is hiding |
| 15 | a real 14th field `` `Bogus_field` `` in §7.2's table | engine + cli | **green — silent** |

**Two of these contradict what this release was handed, and both corrections matter to the design.**

Row 5: a fenced in-window decoy is **not** an open door. `require.Len(rows, 1)` counts the fenced row
too, so the helper refuses rather than choosing. What the same fence-blindness *does* buy is row 6 —
the guard reddens on a **correct** document. The hole is in the other direction from the one recorded.

Row 15: `groundSection72Fields`' first-cell class is `` `([a-z][a-z_]*)` ``, and a field it cannot
spell is dropped from the list while the count of thirteen still matches. That is exactly its twin's
character-class hole, in a helper the handover described as failing loud because *"both callers pin
hard — a count of thirteen, set equality with the code's key set"*. They do pin hard, and it does not
help: the pin is on a list the parser silently shortened. **The two helpers are the same defect, not
one strong and one weak**, and a release that fixed only §2.1's cell would leave §7.2's standing.

### The guard spec's §8 — what its test table learned about itself

Every row derives from a numbered decision. **Nine of the twelve name a mutant that their own stated
fixture kills, both directions measured.** Row 9's is marked **not built** — reasoned, not run, and
says so. Rows 10 and 12 name none, and the fourth cell says why: row 10's fixture is red under `HEAD`
and under the seam alike, so no mutant separates anything, and row 12 is the control. The counting
rule is that a row *names a mutant* when its fourth cell describes a change to the guard, not when it
explains why no change is needed.

**Rows 5 and 6 claimed to name one and did not.** Grounding round 1 built both and neither mutant dies
on the fixture its row names. Row 5's named mutant was `HEAD`'s `slices.Index` locate; on the
absent-§2.1 input that mutant *fails*, at `floor_test.go:1289`, with `"-1" is not greater than or
equal to "0"` under *"§2.1 must be findable by its heading"* — it names the anchor, so it **satisfies**
row 5's assertion. Row 6's named fixture, §2's row 4, is red with the mutation and red without it.
Both rows now carry a mutant their fixture kills, with the run in the cell; §3 item 3's own
continuation had named row 5's correctly all along, and the two now agree.

One line on where that came from, because it is this release's own subject looking back at it. The
false claim lived in this section's opening sentence, and a sentence introducing a table is exactly
what §2.1's arms cut — so grounding filed it against a **cut** unit rather than a floor one.
**A release about guards that re-parse a document lost its own strongest claim to the part of the
document its splitter drops.**

## Routed here from v1.1.0's audit round 3 (2026-09-08)

One of the items routed to `refusals-that-name-nothing-measurements.md` from that round lands on this
spec's subject — a symbol whose only callers are guards. It is recorded in this sidecar; the spec body
is not edited.

- **`engine.UnitKinds` is reachable only from tests — the `IsValidCategory` shape.**
  `internal/engine/unitkind.go:48`. Its doc comment says it exists "for callers that need to name the
  set rather than test one value", and there is no such caller: **all fourteen call sites are in
  `_test.go` files** (`faster_search 'UnitKinds()'` over the repository at `e8477464` — `nextunits_test.go`,
  `briefcommand_test.go`, `hooks_stop_test.go`, `driver_spend_test.go`, `runnertemplate_test.go`,
  `runneragent_test.go`, `unitkind_test.go`). `deadcode ./...` **without** `-test` reports it; the
  project gate runs deadcode with `-test`, which is why the gate is green. As with `IsValidCategory`,
  the question to ask is whether the production caller is missing rather than whether the test is.
