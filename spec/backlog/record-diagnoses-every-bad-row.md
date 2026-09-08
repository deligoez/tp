# tp — `--record` diagnoses every bad row

Class: tool

## 1. Overview

**`--record` diagnoses one bad row per invocation**, so a payload with *N* schema violations costs *N*
round trips. This was `ground-command-friction.md` §3, split out of that file on 2026-09-08 together
with the non-goal that fences `--record`'s atomicity and its two test rows; it was the second of the
four defects one orchestrator measured against this repository's own specs, and it can ship without
any of the others. It is one of the two defects an independent field report confirmed. Every claim
below was run rather than reasoned about; the measurements and the commands that derive them are in
`record-diagnoses-every-bad-row-measurements.md`, and no figure from that file is restated here. The
grounding programme and the two field reports are described in
`what-the-record-does-not-say-measurements.md` under *§1 The evidence base* and *§1.1 The second
evidence base*.

## 2. `--record` diagnoses one bad row per invocation

**Two properties, and only one of them is a defect.**

**Atomicity is correct and stays.** `engine.RecordGroundRound`'s doc comment states it and gives the
reason from §7.2 of the ground spec: *"a partially valid round would make coverage a lie"* — a round
recorded with its bad rows dropped is counted as a round in which those units were decided, and
nothing in the record says otherwise. Validation completes before anything is opened, created or
truncated. **Nothing here changes that.**

**Diagnosis is first-error-only, and that is the defect.** `parseGroundRows` returns on the first
invalid row, so the caller reports one. Measured on a payload with three rows broken the same way (a
`document` claim given `tier: run`, which §4.1 refuses):

```
{"error":"line 2: field \"tier\": \"run\" says nothing about a \"document\" claim (§4.1), and a PASS row must be reached at a tier that does", ...}
```

One line named, three broken. Each fix is a full round trip, and in the reset-native model a round
trip is a subagent re-invocation. It is a **P2 violation on tp's own terms** — what is easy for one row
is not equally easy for *N* — and the measurements file's "§3" records the whole-file validator the
operator wrote to get around it.

**The decision: collect every row's violations and report them together**, keeping the write atomic
exactly as it is. The refusal stays one refusal — it gains a list.

## 3. Non-Goals

1. **No change to `--record`'s atomicity or to §7.2's table.** The corpus exercised both and they
   held; §2 changes what a refusal *says*, never what it refuses.
2. **No new workflow field, no gate and no convergence effect.** Nothing here adds a knob to
   `.tp/config.json` or to a task file's `workflow` block, nothing reads one, and nothing changes
   `clean`, a streak, coverage or an exit code — the `--check` gate is
   `spec/backlog/next-action-and-check-tell-the-truth.md`'s.

## 4. Tests

Every row derives from a numbered decision and names an input that must fail it. The mutant-column
reasoning trimmed from row 1 is in the measurements file's "§8".

| # | from | assertion | the mutant that must fail it |
|---|---|---|---|
| 1 | §2 | a payload with **three** rows broken three *different* ways reports three violations naming three distinct line numbers | report the first and stop — the shipped behaviour; the three break differently to also kill a mutant collecting at most one violation per class |
| 2 | §2 *atomicity* | after a refused `--record`, the state directory is byte-identical to what the preceding emission left | validate row by row as each is appended, which satisfies §2's wording for a payload whose first row is bad and breaks it for every other |
