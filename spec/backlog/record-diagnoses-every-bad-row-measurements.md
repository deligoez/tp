# record-diagnoses-every-bad-row — measurements

Supplemental material for `record-diagnoses-every-bad-row.md`; the spec stands without it.

Every block below was moved out of `ground-command-friction-measurements.md` on 2026-09-08, where it
sat under the heading of the section it came from, and before that out of the spec body verbatim.
Figures are quoted at the ref each block names; re-derive rather than reading one off the page.

The grounding programme this section's measurement came out of, and the two independent field reports
that confirmed it, are described in `what-the-record-does-not-say-measurements.md` under *§1 The
evidence base* and *§1.1 The second evidence base*. Report A confirmed this defect independently —
*"reported only the FIRST offending row, so the loop is fix-one, re-run, hit-the-next — I ended up
telling every agent about it in its brief, which is duty the prompt should carry"* — arriving at the
brief-instruction workaround separately; that workaround is
`scratch-name-is-unique-per-spec-measurements.md`'s "§2", and the one below is a different one.

## §3 `--record` diagnoses one bad row per invocation

**Why this matters more than its size.** It is a **P2 violation on tp's own terms**: what is easy for
one row is not equally easy for *N*. The tell is what the operator did instead — during the grounding
programme the orchestrator wrote its own whole-file validator against §4.1's kind–tier table and ran
it before every `--record`, four times catching two or three violations at once that tp would have
reported one at a time. **When the caller has to reimplement the tool's validator to use the tool
efficiently, the validator is missing something.**

## §8 Tests — the mutant-column prose trimmed from row 1 (was row 4)

Row 4: The three break *differently* to kill a second mutant: one that collects at most one violation per class. **Not** to defeat a dedup-by-message mutant, which an earlier draft gave as the reason and which cannot be defeated that way — `GroundLineError.Error()` is `"line %d: %v"`, so two *identical* breakages already produce distinct messages (measured: `line 1: …` and `line 5: …`), and a same-way fixture is what would catch a mutant deduplicating on the reason alone.
