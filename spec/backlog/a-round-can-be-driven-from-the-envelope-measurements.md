# a-round-can-be-driven-from-the-envelope — measurements

Supplemental material for `a-round-can-be-driven-from-the-envelope.md`; the spec stands without it.

Every block below was moved out of `ground-command-friction-measurements.md` on 2026-09-08, where it
sat under the heading of the section it came from, and before that out of the spec body verbatim. The
section numbers in the headings are the original file's: its §10.1 is this spec's §2.1, its §10.2 is
§2.2, its §10.3 is §2.3. Figures are quoted at the ref each block names; re-derive rather than reading
one off the page.

The two field reports these findings came from are described in
`what-the-record-does-not-say-measurements.md` under *§1.1 The second evidence base*. Their own
figures — round counts, ask curves, floor sizes, token totals — rest on attribution rather than on
evidence, are quoted as the reports' numbers and never re-derived, and no decision turns on one.

## §10.1 There is no machine-readable ask set

Report A calls this *"the single biggest papercut"*. **All three of its claims hold.** Measured on
this spec's round 2 (then `spec/1.56.0.md`) in a copy outside this repository — floor 61 units, all 61 carried:

- **`carried` is a count.** The envelope's keys are `carried, floor, floor_size, output_path, prompt,
  round, snapshot, spec`, and `carried` is the integer 61. No list of ids appears anywhere in it.
- **The `(carried)` marking is prose only.** `grep -c '(carried)' ` over the emitted `prompt` returns
  **62** — 61 index rows plus the ask sentence. It is inside a string a driver would have to parse.
- **The floor file on disk is unmarked.** `grep -c '(carried)' spec/backlog/.tp-review/ground-command-friction/floor-ground-round-2.txt`
  returns **0**. That is deliberate and stays: `runGround`'s own comment gives the reason — *"a copy in
  the floor file would be a second statement of the same fact with nothing comparing the two"* — and
  §2's floor is the artifact the round is graded against.

So a driver that wants the ask set reconstructs it. Report A's reconstruction — join
`floor-ground-round-N.txt` against `ground-round-(N-1).ndjson` on `(text_sha, ordinal)` — is
**exactly `GroundCarriedRows`**, which joins on the same pair for the same reason. *"A 20-line script
every driver will rewrite"* is precise: the script is a reimplementation of shipped tp code, and this
is the same P2 tell §3 records, in a different surface.

## §10.2 Floor growth conflates two different things

Report A's rounds ran 191 → 225 → 224 → 226 floor units, because repairing a spec adds sentences. A
unit is in the ask set either because its text changed or because it never existed, and the report
asks whether anything separates them.

**Measured: nothing does.** On the §11 fixture, round 3 emits with `u2` edited (previously `PASS`ed,
hash moved) and `u5` written fresh in a new section. Both are un-marked in the index, both are in the
ask set, and the ask reads *"This round owes a disposition for 2 of the 3 floor units above"*.
`floor_size` 3, `carried` 1. No key, marker or sentence in the envelope, the prompt, the floor file or
`--status` separates the re-ask from the first ask.

**Half of the report's framing does not survive, and the half that does is the useful one.**
New-versus-edited is **not recoverable from hashes at all** — identity *is* the hash, so an edited
sentence and a deleted-plus-added pair are the same event to every artifact tp writes, and no
decision here pretends otherwise. What *is* exact and cheap is the comparison the report actually
wants for its growth question: **the previous round's floor as a set of hashes against this one's.**

## §10.3 A large floor has no sharding path

Both reports. Report A split 191 units into 5 slices by section anchor, each agent spending 100–250k
tokens on 27–49 units, and writes: *"'Spawn ONE sub-agent on that prompt' does not survive a 191-unit
floor. The instruction reads as a correctness constraint (no panel, no roles) but lands as a capacity
claim."* Report B: **213 asked units out of a 240-unit floor** — §1.1 and this section quote different
quantities and the file conflated them until now (240 is that spec's floor, 213 its round-1 ask) —
306k tokens, 83 tool calls: *"it held, but a spec 2× this size would blow the context."*

**The emitted prompt does not say it.** Measured over the emission for this spec (then `spec/1.56.0.md`), at round 1
and round 2 alike: `grep -iE 'sub-agent|panel|alone|isolation|single'` over the `prompt` string
returns **one** line — *"joined, whitespace collapsed to single spaces, a list or blockquote marker
dropped,"* — about canonicalisation, and **nothing about how many readers the prompt has.** An earlier
draft said four lines; re-running returns one, and the error ran against this section's own case,
because at one match the claim is stronger. The sentence Report A quotes is `skills/tp/SKILL.md`'s
ground-loop step
*"Spawn **one** sub-agent on that prompt"*, and the reason the same section gives for the singular is
in the step above it: *"grounding asks one question of every unit, so there is no panel and no
role"*. **That is a claim about the panel, not about capacity** — Report A's diagnosis is correct,
and it is correct about SKILL.md rather than about the emission.

**Slicing already works, and that is measured, not inferred.** On the fixture, a payload holding one
row for one of three owed units records at **exit 0** with `rows: 1`, `--status` reports
`dispositioned: 2 of 3`, and `--status --check` exits **1**. So a slice is a well-formed round
contribution, the shortfall is reported, and the gate holds until the slices are all in. NDJSON
concatenates, and grounding has no `--merge` to need.

(The paragraphs ranking this repository's largest floors were deleted from the spec rather than
moved: the ranking they asserted does not hold at the current tree.)
