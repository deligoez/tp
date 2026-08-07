# v0.33.0 — review decision

`tp review spec/0.33.0.md --status --check` does not exit 0. Decomposition proceeded anyway, through
`tp import --force`, on the operator's explicit decision. Recorded so the choice is auditable rather
than implicit.

## What the twelve rounds say

| round | findings | clean (no critical/high) |
|---|---|---|
| 1 | 39 | no — 5 high |
| 2 | 38 | no — 4 high |
| 3 | 28 | **yes** |
| 4 | 27 | no — 1 high |
| 5 | 25 | no — 2 high |
| 6 | 22 | no — 2 high |
| 7 | 22 | no — 4 high |
| 8 | 17 | no — 4 high |
| 9 | 15 | **yes** |
| 10 | 19 | no — 5 high |
| 11 | 22 | no — 2 high |
| 12 | 17 | no — 1 high |

291 findings were raised and every one is resolved `fixed` — none was accepted `wontfix`. Two rounds
were clean; convergence needs two **consecutive** clean rounds and never got them.

## Why it did not converge

The blocking findings were real in every round, and the loop did not stall on the same defect: each
round's repairs created the next round's surface. The clearest instance is the corpus-staleness
machinery. A round-6 finding showed that `RolesStale` cannot see a corpus edited *between* two
recorded rounds; the fix added a per-round `roles_hash` comparison. A round-10 finding showed that
comparison treated an empty hash as a value, so two empties matched and a streak could be counted
under a panel tp cannot name; the fix made empty incomparable. A round-11 finding showed the
non-emptiness terms that fix added to condition 5 were unreachable, because the earlier cause already
withheld the object; the fix removed them. A round-12 finding showed the paragraph *describing* those
terms had been left behind. Four rounds, one mechanism, each finding correct and each smaller than
the last.

The finding count fell 39 → 17 and the severity mix thinned, but the tail did not reach zero inside
this session's subagent budget (~120 of 200 spent on review alone, at five roles per round).

## Why shipping the decomposition anyway is defensible

- **Nothing is open.** Every finding of every round is resolved `fixed`, including all 32 blocking
  ones. The non-convergence is arithmetic — round 12 was recorded with a blocking finding and a
  `fixed` resolution does not retroactively clean a recorded round, which is the correct behaviour.
- **Round 12's blocking finding was a leftover, not a defect in the design.** It named a paragraph
  that still described machinery the same revision had deleted. It is fixed; a thirteenth round would
  very likely record clean.
- **The remaining surface is prose.** The last three rounds produced no finding about what the
  feature does — only about how the spec says it. The implementation phase re-checks the same
  statements against code, which is a stronger test than a fourteenth reading of the text.

## What this costs

`stale: true` on the review record: the spec was revised after round 12 was recorded, so the recorded
rounds hash a text that no longer exists. The audit phase is what re-verifies the current text against
the implementation, and `spec-coverage` reads the spec as it ships.

The honest summary is that this release's spec was reviewed hard and stopped one round short of the
recorded fixed point, by choice rather than by budget exhaustion.
