# tp — The scratch name is unique per spec

Class: tool

## 1. Overview

**The scratch file the emission names is not unique across specs.** Two units grounding two different
specs are told to write the same filename, and the second overwrites the first at exit 0. This was
`ground-command-friction.md` §2, split out of that file on 2026-09-08 together with its non-goals and
its three test rows; it was the first of the four defects one orchestrator measured against this
repository's own specs, and it can ship without any of the others. Every claim below was run rather
than reasoned about; the measurements and the commands that derive them are in
`scratch-name-is-unique-per-spec-measurements.md`, and no figure from that file is restated here. The
grounding programme that produced the measurement, and the two independent field reports the sibling
splits lean on, are described in `what-the-record-does-not-say-measurements.md` under *§1 The evidence
base* and *§1.1 The second evidence base*.

## 2. The scratch file the emission names is not unique across specs

**Measured.** `internal/cli/ground.go`, in the emission path:

```go
outputPath := fmt.Sprintf("ground-r%d.ndjson", round)
```

The round is in the name; the spec is not, so both of these print **the same string** — assert the
equality, never the literal:

```
tp ground spec/backlog/what-the-carry-can-promise.md | python3 -c 'import sys,json;print(json.load(sys.stdin)["output_path"])'
tp ground spec/backlog/ground-command-friction.md | python3 -c 'import sys,json;print(json.load(sys.stdin)["output_path"])'
```

Two units grounding two specs concurrently are told to write the same file. The second finishes and
**overwrites the first at exit 0** — no lock, no warning, and the first spec's dispositions are gone
with nothing recording that they existed. **The loss is silent; the *swap* mostly is not.** Recording
the survivor against the wrong spec is refused — emitting the first of the two specs above at round 1
and then `--record`ing a payload built from the second's floor exits **1** (*"the emitted floor gives
u3 the hash c0df7c57c079, and this row carries 9c5874db7c50"*), because `groundRowMatchesFloor`
compares `text_sha` **and** `ordinal`. Two carve-outs stop that being a full defence, both rows the
join never reaches: a cut unit's row (`unit_id` absent from the floor) is deliberately not compared,
and one with `unit_id: null` is never joined, so a cross-spec payload of only those records silently.
And no message on any path says a *second spec* was involved.

**This is not the recorded round's name, and the difference is deliberate.**
`engine.groundRoundFileName` carries the reason in its own doc comment: `ground-rN.ndjson` is *"the
scratch file a unit writes and an operator collects"*, while `--record` writes
`ground-round-<N>.ndjson` beside the snapshot and the floor. **That decision is not what this section
changes.** The defect is narrower: a scratch name must be unique among the things that may be
scratched at once, and the spec base is the only thing separating them.

**The decision: put the base in the scratch name** — `ground-<base>-r<N>.ndjson`, `<base>` being the
one the state directory already uses (`spec/.tp-review/<base>/`), so nothing new is derived and no two
specs collide.

**The sibling case is named and NOT taken here.** `roleOutputPath` (`internal/cli/prompt_framing.go`)
has two branches and only the **fallback** collides: outside a run it returns
`review-r<N>-<role>.ndjson` / `audit-r<N>-<role>.ndjson`, qualified by round and role and equally
un-qualified by spec, so two concurrently-reviewed specs collide exactly as §2's do — and outside a
run that fallback is the only branch there is, so it is the path an operator driving rounds by hand
takes. Under `tp run` it does not collide: `TP_ROUND_DIR` is set and the path becomes
`$TP_ROUND_DIR/role-<role>.ndjson.part`. It is left out because this release is `tp ground`'s
friction and because the review/audit collision needs its own acceptance over per-role emission, not
because it is not real. `spec/undecided.md` is not its home either: it has an obvious design and no
undecided part — it wants a release. How the grounding programme worked around the collision is in
the measurements file's "§2".

## 3. Non-Goals

1. **No change to the recorded round's filename.** `ground-round-<N>.ndjson` is deliberate and §2
   says why it stays.
2. **The review/audit scratch-name collision is named in §2 and not fixed here.**
3. **No new workflow field, no gate and no convergence effect.** Nothing here adds a knob to
   `.tp/config.json` or to a task file's `workflow` block, nothing reads one, and nothing changes
   `clean`, a streak, coverage or an exit code — the `--check` gate is
   `spec/backlog/next-action-and-check-tell-the-truth.md`'s.

## 4. Tests

Every row derives from a numbered decision and names an input that must fail it.

| # | from | assertion | the mutant that must fail it |
|---|---|---|---|
| 1 | §2 | two specs emitted at the same round produce **different** `output_path` values, asserted on the pair rather than on either alone | keep the round-only name, under which the two are equal and a concurrent run silently overwrites |
| 2 | §2 | the emitted `output_path` contains the same base the state directory uses, asserted by deriving the base rather than by matching a literal | hardcode a base in the test, which passes for the fixture spec and for no other |
| 3 | §2 *unchanged* | the **recorded** file is still `ground-round-<N>.ndjson` | rename the recorded file to match the scratch name, which is the decision §2 explicitly does not take |
