# emitting-does-not-lose-a-round — measurements

Supplemental material for `emitting-does-not-lose-a-round.md`; the spec stands without it.

Every block below was moved out of `ground-command-friction-measurements.md` on 2026-09-08, where it
sat under the heading of the section it came from, and before that out of the spec body verbatim. The
section numbers in the headings are the original file's: its §14 is this spec's §2. Figures are quoted
at the ref each block names; re-derive rather than reading one off the page.

The grounding programme and the two field reports are described in
`what-the-record-does-not-say-measurements.md` under *§1 The evidence base* and *§1.1 The second
evidence base*. That file's *§1.2* records that the commit *"re-emitting an unrecorded round is
idempotent, and a round file holds the carry"* closed half of §14 in `skills/tp/SKILL.md` while these
sections were being written, and that §14's own measurements were taken on fixtures outside this
repository.

## §14 A fear the tree refutes

**The fear is false.** Measured in a copy outside this repository: two consecutive bare emissions on an
unrecorded round return **the same round number, the same floor size and a byte-identical floor file**
(sha256 `40302ef74b9cbfdb…` before and after). Reproduced independently on the §11 fixture at round 3:
round unchanged at 3, `floor_size` 3, and
`shasum -a 256 spec/.tp-review/demo/floor-ground-round-3.txt` identical across the two runs.
**Re-emitting an unrecorded round is idempotent** — `NextGroundRound` answers *recorded rounds + 1*,
and the emission rewrites the same two files.

**The ask survives the correction, and that is why both halves are recorded.** Nothing in the
**output** says it is idempotent — an agent has to know, and Report B reasonably did not — and once a
round is recorded a bare emit legitimately opens round N+1, so the same call is safe in one state and
round-advancing in the next, **with no way to ask which state you are in from the emission's own
output**. That qualifier is load-bearing and an earlier draft omitted it: a second call does answer,
and it was measured — with round 4 emitted and unrecorded, `tp ground <spec> --status` returns
`"round": 4, "dispositioned": 0` at exit 0, and after a `--record` the same call reports a positive
`dispositioned`; the state directory answers it too, by whether `ground-round-4.ndjson` exists. Both
answers cost a call the reader that matters cannot make — the sub-agent holding only its prompt, which
is the same reader the next paragraph names. `tp ground` has four flags today (`--record`, `--status`,
`--check`, `--units`), and none of them re-prints a prompt.

**Half of this was closed while the section was being written, in the document and not in the
output.** The commit *"re-emitting an unrecorded round is idempotent, and a round file holds the
carry"* added the fact to `skills/tp/SKILL.md`'s ground loop, independently measured and agreeing
with the paragraph above. That closes it for a human reading the skill and changes nothing for a
sub-agent holding only its prompt, which is the reader Report B's six briefs were written for.

## The silent overwrite: emitting over an unrecorded round

**Measured on this repository's own hotfix cycle.** A round was graded, the spec repaired before the
round was recorded, and `tp ground <spec>` re-emitted round 1 against the repaired text. The emission
**overwrote the unrecorded round's floor file without a word**, and `--status` then reported a round
with 0 dispositions — indistinguishable, from the record alone, from a round nobody has graded yet.

Recovery was possible only by accident: an `rsync` copy taken for unrelated work held the pre-repair
floor and most of the grader's rows. Recording those made the next round ask 38 of 44 units instead of
44. The cost of the missing guard is therefore measurable: one full round of grading.
