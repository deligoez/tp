# the-guard-pins-the-whole-listing — measurements

Supplemental material for `the-guard-pins-the-whole-listing.md`; the spec stands without it. Every
block below was moved here on 2026-09-08 from `refusals-that-name-nothing-measurements.md`, which had
in turn taken them verbatim from the spec body earlier the same day. The conventions every mutant
below was run under — the `rsync -a --exclude .git ./ <copy>/` copy outside the repository, the
`git status --porcelain -- internal/` check with its control, and the rule that no line number is
cited — are in that file's **Preamble** and are unchanged here.

## §3 — measured, both directions

Probing the copy for `len(GroundVerdicts())`, `len(GroundKinds())`, `len(GroundTiers())` and
`len(GroundPartialKinds())` gives `6 7 6 3`, so no cell gets four. The next table is the same fact
from the other side — `GroundTiers()[:2]` costs the `tier` cell four of its six values, hence four
failures.

**Measured, both directions, on the shipped guard:**

| mutant at the production call site | `internal/engine` | `internal/cli` |
|---|---|---|
| `GroundTiers()[:2]` — truncate the listing | **red**, four failures naming `run`, `probe`, `red-green`, `break-and-control` | — |
| the same truncation on **all four** call sites at once | **red**, and *counted*: each of the four subtests fails on its own | — |
| `append(GroundTiers(), "document", "corpus", "vibes")` | **green** | **green** |
| the same append on **all four** call sites at once | **green** | **green** |

The mutated refusal reads, in full:

```
field "tier": "squinted" is not one of the values the spec lists: read, query, run, probe, red-green, break-and-control, document, corpus, vibes
```

A refusal telling a unit that `document` is a legal tier — while `document` is a *kind*, and the row
that pairs it with a tier is exactly what `refusals-that-name-nothing.md` is about — and the guard
written to make this message informative passes it.

The obvious repair does not work, and it was measured before being rejected:

| assertion | shipped message | listing appended | listing prepended |
|---|---|---|---|
| `Contains(msg, ": " + strings.Join(names, ", "))` | pass | **pass** | fail |
| `HasSuffix(msg, ": " + strings.Join(names, ", "))` | pass | **fail** | fail |

## Routed here at the 2026-09-08 re-verification

Two of the items routed to `refusals-that-name-nothing-measurements.md` at that re-verification land
on this spec's subject — a guard that does not cover the sink it names. They are recorded in this
sidecar; the spec body is not edited.

- **No guard covers the invalid-check sink.** v0.33.0 test 34 says a registered check that cannot run
  must surface in `mechanize_candidates`; at `HEAD` no test registers an invalid check and asserts
  that it does. The citation half of the item shipped — `review_suppression_test.go` now names
  v0.33.0 — and the sink half did not. Source: `spec/0.35.0-candidates.md` item 6.
- **The hint guard's two blind spots.** `internal/cli/hint_coverage_test.go` exempts four files by
  name — `config.go`, `config_extract.go`, `set_local.go`, `set_project.go` — with the reason
  recorded in the comment above `taskFileCommands`, and it says nothing at all about bare
  `os.Exit(ExitValidation)` sites, of which there are **51** across `internal/` at `dd89c566`
  (`faster_search 'os.Exit(ExitValidation)'`, counting call sites). The exemption is honest; the
  second gap is unnamed. Source: `spec/0.35.0-candidates.md` item 8.
