# tp — `next_action` and `--check` tell the truth

A backlog spec, named by slug; its priority number and its release number are assigned later. Its
measurements are in `next-action-and-check-tell-the-truth-measurements.md` beside it, and this file
stands without them.

Class: **tool** — it changes what three status surfaces report and what one gate reads, and no rule
by which a round is graded.

## 1. The decision

**Context.** Three surfaces answer a driver's question with something other than the answer.

- `tp ground <spec> --status --check` exits 0 over a recorded round whose rows hold `FAIL`s —
  refuted claims standing in the spec — because it gates on coverage and nothing else. Two field
  reports on `tp ground` stopped their loop on that exit code, and a fixture reproduces it at `HEAD`
  (sidecar). The same `--status` payload carries no `next_action`, where review's and audit's both do.
- Between a repair and the next ground round, `--status` is identical to what it was before the
  repair. Nothing says that the repaired sentences carry no disposition yet, and a field report
  (WB-3155) reported a repair taking several rounds to settle with nothing budgeting them.
- A registered check that could not run is reported `passed: false`, like a check that found
  violations, and its class is still stamped into every role prompt under *"do NOT report findings of
  these classes"* — so reviewers are told a class is mechanically checked when no check ran. And a
  check's command has no way to name the spec under review: it reaches a spec through whatever the
  active pointer names, so a check can pass about one spec while another spec's reviewers are told its
  class is mechanized (sidecar).

**Decision.** Three changes to what tp reports and one help-text fix, none adding a flag, a unit kind
or a workflow field: grounding's `--check` exits 1 over a standing `FAIL` (§2); grounding's `--status`
carries `next_action` and the count of units a repair left ungraded (§3); a registered check's exit
code is a contract, a check that could not run suppresses nothing, and a check's command can name the
spec and round the loop is grading (§4); `tp review --help` names `regression` among the perspectives
(§5).

**Consequences.** A driver branching on grounding's exit code stops with no `FAIL` standing rather than
with false claims in the spec; `tp run` schedules no grounding unit today, so the beneficiary is that
driver when it exists and any script branching on `$?` until then. The gate has one hole that clears
it without a repair — deleting one of two byte-identical sentences can hand the survivor the deleted
copy's `PASS`. `what-the-record-does-not-say` closes it with a multiplicity fence on the carry,
shipped together with its carried-verdict override because the two share one sentence of the ground
prompt. That spec no longer carries a test characterising the hole, so if this release ships first
the hole stands, with no test naming it, until that one does. A class whose check cannot run goes
back to the reviewers until the check runs again.

**Cut on 2026-09-11.** An earlier draft carried a fifth change: an advisory `next_action` branch in
review recommending the uncounted regression delta pass after a large repair. It had no measured cost,
it needed a three-condition branch and a carve-out under `tp run`, and the rule it would have
recommended is already `skills/tp/SKILL.md`'s review loop, step 6. The sidecar keeps its history
(*Cut on 2026-09-11: the delta-pass branch*).

## 2. `tp ground --status --check` exits 1 on a standing `FAIL`

**The decision: `--check` exits 1 when the latest *recorded* round carries a row whose `verdict` is
`FAIL`.** The two conditions it already has are unchanged, and `skills/tp/SKILL.md`'s Step 1.5
sentence saying a round of nothing but `FAIL`s is fully covered and exits 0 goes with the behaviour it
describes.

**Recorded, not emitted.** An emitted-and-unrecorded round is already exit 1 on the coverage
condition, so the two readings never disagree about the exit code — only about what the third
condition reads, and it reads the latest recorded round file.

**`FAIL` and nothing else.** `FAIL` is the only verdict that asserts the spec is wrong: `UNVERIFIABLE`
is a settled answer, `NOT-A-CLAIM` asserts nothing, `QUESTION` is explicitly non-blocking in the same
SKILL.md step that asks for the repairs. `PARTIAL` is the deliberate exclusion, and it is narrower
than the loop: SKILL.md stops its ground loop only when the breakdown carries no `FAIL` **or
`PARTIAL`** you have not repaired, so a `true-when-written` `PARTIAL` is a complete row and an open
repair the gate does not see. What the gate buys is the case no prose caveat reaches — a machine can
gate on a standing `FAIL` without judging prose. Whether SKILL.md's condition should narrow to match
is SKILL.md's decision and is not taken here.

**One unconditional exit, one hole.** Repairing the unit's text moves its hash, so the unit leaves the
carry and is re-asked; re-deciding a carried unit in a later round is the override
`what-the-record-does-not-say` makes sayable, permitted only when the ground beneath it moved. The
hole — two byte-identical sentences, the failing one cleared when the other is deleted — is closed
by that spec's multiplicity fence, not here.

## 3. `tp ground --status` carries `next_action` and what a repair left ungraded

**The decision: `--status` carries `next_action`, under that name and in that role.** Review's and
audit's `--status` both carry it — *"the single next step"*, in SKILL.md's words for each — and
grounding's payload does not. It is a pair with the gate above: the exit code says *not yet*; the key
says *what to do*, which is how a driver branches without parsing prose. It names the step SKILL.md's
ground loop names for the state it reads — repair a standing `FAIL`, run the next round, or leave the
loop — and it is reporting, so a driver that ignores it is exactly as correct as one that reads it.

**And `--status` reports `ungraded`: how many current floor units the next emission would ask rather
than carry.** That is the text written since the last recorded round — a repaired sentence has a new
hash and an added one has none on record — and every such unit owes a disposition. The number already
exists at emission as the floor less what was carried; what is missing is the report *before* the
emission, when the operator is deciding whether the repair is finished. When `ungraded` is non-zero,
`next_action` names the next ground round.

**It is reported at `--status`, not at `--record`.** The field report asked for the count after
`--record`; at that moment the repair has not happened yet, so the only honest number there is zero.

**`ungraded` gates nothing.** A repair of a `FAIL` already keeps `--check` at 1 through §2, until a
round re-decides the unit. A gate on `ungraded` itself would read the working tree rather than the
record, and would make `--check` a staleness gate that every later edit to the spec trips — a
different signal from *no refuted claim stands*.

## 4. A registered check's exit code is a contract

**The decision: a `workflow.checks` entry exits `0` when it passed, `1` when it found violations, and
`2` or higher when it could not run.** A check the shell cannot start, or that tp stops at its
timeout, is in the third state too.

**A check that could not run neither passes nor suppresses its class.** Its entry is reported
`ran: false`. Its class leaves the *"do NOT report findings of these classes"* list of every role
prompt of that emission and stops being withheld from `--status`'s `next_action`, so reviewers are
asked for the class as though no check were registered. It does not pass: `tp review <spec> --status
--check` still exits 1 while it stands. A check that ran and found violations keeps suppressing its
class, which is registration doing its job and the behaviour
`TestReviewSuppression_FailingCheckStillSuppressesItsClass` pins.

**Why tp may define the status here and not elsewhere.** `CLAUDE.md` records that `gocognit`'s exit
code cannot tell a result from a failure, which is why that guard reads stderr. A `checks[].cmd` entry
is different in kind: it is tp's own registration, written for tp, so tp states what its exit code
means rather than observing a third-party convention.

### 4.1 A check names the spec and round it grades

**The decision: `checks[].cmd` gains two placeholders, `{spec}` and `{round}`, and nothing else.**
Wherever tp runs a registered check, it replaces `{spec}` with the path of the spec the command was
invoked on and `{round}` with the round the loop is collecting — the round an emission emits, or at
`--status` the one after the latest recorded round. Each is substituted as a single shell-quoted word,
so a path with a space or a shell metacharacter reaches the check as one argument.

**Why.** A check grades a spec, and today its command can learn which one only by asking the active
pointer, typically through `tp resume` in a subshell. That answers for the pointer's spec rather than
for the one on the command line, and when `tp resume` cannot answer the check receives an empty path.
With the placeholder the spec under review is what the check reads, and a `PASS` is about that spec.

**Nothing else is substituted.** Braces are ordinary shell and awk syntax, so only the two exact
tokens are replaced and every other `{…}` reaches the shell untouched. A check that names neither runs
exactly as today.

## 5. `tp review --help` names `regression`

**The decision, as a task: the `--perspective` flag's help text names `regression`.** It reads
*"documentation, testing, or code-audit"* while `--perspective regression` is accepted. It is one
line, and it is why the pass goes unrun.

## 6. Non-Goals

1. **No new flag, no new unit kind and no workflow field.** Nothing here adds a knob to
   `.tp/config.json` or to a task file's `workflow` block, and nothing reads one.
2. **`next_action` and `ungraded` gate nothing.** Neither has an exit code of its own.
3. **No change to grounding's carry, its `(text_sha, ordinal)` join or its recorded filename**, and no
   gate on `PARTIAL` (§2).
4. **The byte-identical-sentence hole is not closed here** — it is `what-the-record-does-not-say`'s.
5. **No change to `--record`'s candidate list for registered classes.** It reads the registration
   without running the check, so it cannot know whether the check can run; §4 applies where tp runs it.

## 7. Tests

Every row derives from a numbered decision and names a mutant that must fail it. Rows 1, 2, 7, 9 and
10 read a value that exists at `HEAD`, and the sidecar quotes it under *Re-verified 2026-09-11*; the
other rows' subjects do not exist yet, so their two counts belong to the implementing task's
acceptance.

| # | from | assertion | the mutant that must fail it |
|---|---|---|---|
| 1 | §2 | a round whose rows are fully covered and hold at least one `FAIL` exits **1** from `--status --check` | keep the two shipped conditions, under which a fully covered round holding `FAIL`s exits 0 |
| 2 | §2 *bounded* | the same fixture with the `FAIL` replaced by a `PARTIAL` and by a `QUESTION`, each in turn, exits **0** | gate on any non-`PASS` verdict, which passes row 1 and makes every recorded `PARTIAL` row blocking |
| 3 | §2 *no deadlock* | a round-2 payload that decides a `FAIL`-carrying unit afresh records at exit 0 and `--check` then exits 0, on a fixture whose spec text did not change | carry the `FAIL` unconditionally, so a unit once refuted can never be re-decided |
| 4 | §3 | `--status`'s payload carries `next_action`, asserted by naming the key rather than by matching its text | assert on the sentence, which pins prose a rewording breaks while the key survives — a test-side mutant |
| 5 | §3 *ungraded* | after a recorded round, editing one unit's sentence and adding one new sentence makes `--status` report `ungraded: 2` before any emission, and the emission that follows carries exactly the floor less those two | report the count the last emission computed, which reads 0 after a repair until the next emission |
| 6 | §3 *no gate* | on a recorded round with no `FAIL`, editing only a `PASS` unit's sentence leaves `--status --check` at exit 0 while `ungraded` is 1 | gate on `ungraded`, turning `--check` into a staleness gate |
| 7 | §4 | with two checks registered, one `exit 1` and one `exit 2`, the emission reports the second `ran: false`, and every role prompt's suppression list names the first class and not the second | the shipped suppression, which names both classes in every role prompt |
| 8 | §4 *not a pass* | with only the `exit 2` check registered and nothing else failing, `tp review <spec> --status --check` exits 1 | treat a check that could not run as skipped, which lets `--check` pass on a check nobody ran |
| 9 | §4 *boundary* | an `exit 127` check — a command the shell cannot find — is reported `ran: false` like `exit 2` | read only exit `2` as could-not-run, which leaves the commonest cannot-run case suppressing its class |
| 10 | §4.1 | two specs in one repository, the marker string in the second only, the active pointer on the second: reviewing the first with a check `grep -q <marker> {spec}` reports it `passed: false`, and the same check reviewing the second reports `passed: true` | substitute the active pointer's spec — the subshell workaround's behaviour, under which reviewing the first passes |
| 11 | §4.1 *round* | an emission of round 3 runs a check `test {round} = 3` and reports it passed; `--status` after round 3 is recorded runs `test {round} = 4` and reports it passed | substitute the latest recorded round, which fails the emission case |
| 12 | §4.1 *quoted* | with the spec at a path containing a space, a check `test -f {spec}` reports passed | substitute the bare path, which splits it into two words and fails `test` |
| 13 | §4.1 *nothing else* | a check `echo b \| awk '{print}' \| grep -qx b` reports passed | replace any `{word}` token, which rewrites the awk program into one that prints nothing |
| 14 | §5 | `--perspective`'s help text names every value the flag accepts, derived from the accepted set rather than from a literal list | add a perspective to the accepted set and leave the help text alone — the derived guard fails naming it, a literal list passes |

**Row 2 is what keeps §2 narrower than SKILL.md's loop**, and row 6 is what keeps §3 a report: each is
the row a broader implementation of its section passes everything else with.
