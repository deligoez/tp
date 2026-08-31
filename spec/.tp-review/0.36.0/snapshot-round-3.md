# tp v0.36.0 — The emitted round

## 1. Overview

tp emits one prompt per role per round. That prompt carries the checklist, the role rules, the
affected files and the output schema — everything about *what to judge*. It carries nothing about
*how the unit must behave while judging*, and every invocation emits the whole panel to whoever
asked, however many roles that caller actually needs.

The consequence is that every round costs the orchestrator the same manual additions, and a round
that does not get them loses work or produces false findings. Two independent cycles measured the
same shortfalls, on different projects, in different languages:

- tp's own v0.35.0 audit — nine rounds, four roles, this repository.
- A field cycle on a PHP package, nine rounds, five roles, driven by a different operator.

This release makes the emitted prompt self-sufficient and lets a caller ask for one role's prompt
instead of the panel's. It does not change the checklist, the role rules, the output schema, or how
convergence is counted.

**Where the benefit lands, stated up front because it is not uniform.** §2 and §3 both address the
**interactive path** — an orchestrator that spawns sub-agents itself, this repository's documented
fallback and the PHP cycle's method. Under `tp run` §2's constraint is already a fence (§2.1) and
§3's salvage is already refused by the driver (§3.1), so on that path the two clauses are
belt-and-braces rather than the mechanism. §4 is the reverse: its measured cost is paid hardest
under `tp run`. The release is worth shipping because the two paths need different halves of it,
not because every half serves both.

**What it costs.** Two clauses go into every emitted role prompt: **375 bytes, 69 words**, measured
on the exact strings §2 and §3 fix. Against this spec's own review prompt of 10,238 bytes that is
**3.7%**; against a 75 KB audit role prompt it is under a half percent. The clauses are emitted on
every path, including the one where §2's constraint is already enforced by a fence — tp emits the
prompt before anything knows which runner will consume it, so a conditional emission is not
computable at the point of emission. §4's flag adds no prompt bytes and removes them from the
payload.

## 2. The prompt does not carry its own isolation constraint

**The field instance is a false finding, not an inconvenience.** In round 2 of the PHP cycle an
auditor mutated a file in the repository to test whether a test was tautological — a legitimate
experiment, and the one this project's own evidence rule asks for. A *different* auditor, running
concurrently, read the mutated file and reported the static analyser's failure as a real defect. The
finding was caused entirely by the absence of isolation guidance in the prompt both agents received.
The operator then added "DO NOT modify the working tree" to all five role prompts, by hand, every
round.

tp's own cycle paid the same cost without the failure: the orchestrator injected "Do NOT modify any
repo source file" into all four role prompts for nine consecutive rounds.

### 2.1 Which fence exists, and on which path

tp ships two `PreToolUse` hooks and they are not interchangeable.

| Hook | Registered in | Denies a source write? |
|---|---|---|
| `pre-tool-use-write-deny.sh` (`spec/0.35.0.md` §6.2) | `hooks.json` | **No** — denies four tp state paths, allows every source file |
| `pre-tool-use-role-write-allow.sh` (`spec/0.35.0.md` §6.3) | frontmatter of `agents/tp-reviewer.md`, `agents/tp-auditor.md` | **Yes** — a role unit may write exactly one path |

Both were verified against the shipped hooks rather than read: `internal/cli/audit.go` is allowed by
the §6.2 hook and denied by the §6.3 hook.

The split is drawn on §6.3, and **the clause is for the path §6.3 does not reach**. A runner that
loads the agent definitions gets the fence; an orchestrator that spawns sub-agents itself loads
nothing and gets no fence. Under `tp run` the clause therefore restates a rule the fence already
enforces — which is why §1 prices it rather than claiming it. `spec/0.35.0.md` §6.3 itself calls the
restriction "defence in depth", which is a statement that it is not meant to be the only thing that
is true.

### 2.2 The clause

Emitted verbatim as this exact string, LF-separated, with no leading or trailing blank line:

```text
Do not modify the working tree. Read anything; write only the output file this prompt names. If proving a defect would require changing code, report it with its evidence instead of making the edit.
```

**It is one line, not a wrapped paragraph.** Tests assert byte equality against this string, so the
spec gives it unwrapped: a hard-wrapped blockquote would leave an implementer three transcription
decisions (join the lines with a space, or with `\n`; keep the `> ` prefixes or strip them) that
each produce a different artifact and each pass a loose reading of "verbatim".

**"the output file" is the antecedent the prompt already supplies.** The emitted prompt ends with a
`## Unit framing` section that names the file — verified on this spec's own round-3 prompt, which
reads `Write this round's findings to: review-r3-architect.ndjson`. §2.3 pins the clause's position
so that antecedent always precedes it.

**Naming the output file also makes the clause work for both commands.** `tp audit` produces result
rows, not findings; a clause that said "the findings file" would be wrong in half the prompts it is
emitted into. `output_path` is a key both commands emit, so one string serves both.

### 2.3 Placement

Both clauses are appended to the existing `## Unit framing` section, after its last existing line,
**§2's clause first and §3's second**, one blank line between them. Placement is pinned because
both clauses refer to the output file that section names, and because a test that asserts only
presence would pass on a prompt where the reference dangles.

### 2.4 What is deliberately not provisioned

**No perturbation recipe is offered.** An earlier draft told the unit to copy the subject out of the
repository and mutate the copy. Three things defeat it: the copy is a second write, which
`spec/0.35.0.md` §6.3 forbids outright; a source file moved out of its package does not build, so
the experiment the motivating story describes cannot be run on it; and `spec/0.42.0.md` §9 has
already assessed exactly this recipe and found against it, noting that N auditors each building a
scratch copy would dissolve the concurrency argument `spec/0.35.0.md` §3.3 rests on. Giving an
auditor a way to perturb is that file's open question. This release constrains; it does not
provision.

**The clause creates a finding class this release does not close.** Telling a unit to report what it
would have had to edit to prove converts one class of row — a defect whose proof needs a mutation,
a broken precondition or an injected failure — from run evidence into asserted evidence, and tp has
no verdict for "asserted, not run". That gap is `spec/0.42.0.md`'s subject and Non-Goal 2 names it
rather than pretending the clause is free.

**Why the prompt rather than the agent definitions.** The definitions are the cheaper channel — they
cost nothing per round — and they are the right home for a standing rule under `tp run`. They are
also loaded by nothing on the interactive path, which is the only path where this constraint is not
already enforced. A future release that makes the interactive path load a definition should move the
clause there and delete it here.

## 3. A unit that buffers its findings loses all of them

Both cycles lost whole rounds to the same shape. An agent accumulates its rows in context and writes
the NDJSON once, at the end; the process dies before that write; nothing survives.

The field cycle lost **six** subagents this way over one session. The single run that had been
instructed to append each row as it was decided preserved **72 rows** despite dying near the end.
tp's own cycle lost four, and the one told to write early kept 9 of its 10 rows — so the relaunch
completed the round instead of redoing it, and the recovered rows were the expensive ones.

The asymmetry is the argument: buffering saves nothing and risks everything, because a findings file
is append-only NDJSON whose partial state is already valid input to `--merge`. Verified on the
shipped binary: a file whose trailing line is truncated mid-object merges with
`warning: skipping malformed line (invalid JSON)`, keeps every complete row, and exits **0**.

### 3.1 Where the salvage is real, and where it is not

The measured recoveries above are **interactive-path** results, and they do not carry to `tp run`
unchanged. `spec/0.35.0.md` §3.3 defines a role unit's durable write as
`$TP_ROUND_DIR/role-$TP_UNIT_ID.ndjson` existing with **every line parsing**, and §3.3.1 has the
unit write `role-<id>.ndjson.part` while the driver renames it to the final name **only on exit 0**.
A unit that dies therefore leaves rows that are never promoted, and a truncated trailing line would
fail the predicate even if they were.

Under `tp run` the clause's benefit is consequently smaller and different: the `.part` file survives
on disk with its complete rows intact, so a human — or a later release that decides to promote a
partial file — can recover the round's expensive work instead of finding an empty context. It does
not salvage the round automatically, and this spec does not change the driver to make it do so.

**Whether a partial file should ever complete a unit is `spec/0.41.0.md` §2's question**, where N
shards are jointly complete and no single one is, and it should be answered once, there.

### 3.2 The clause

Emitted verbatim as this exact string, on the terms §2.2 and §2.3 fix:

```text
Write each row to the output file as you decide it, not once at the end. A run that dies with its rows unwritten loses the whole round; a partially written file is still usable.
```

### 3.3 What this does not change

A findings file that exists and parses does not mean the unit finished. It never did: completion is
`exit 0` **plus** the driver's rename (`promoteRoleFindings`, `spec/0.35.0.md` §3.3.1), and
`spec/0.35.0.md` §6.2's Stop hook applies an "exists and parses" predicate that is a liveness check
— it catches a unit stopping with nothing — not a completeness check. Incremental append makes
partial files the normal intermediate state, so it makes that misreading easier; it does not create
it. A unit that stops cleanly having judged three of forty items merges as complete today, before
this release and after it. That hole is `spec/0.41.0.md` §2's, as above.

## 4. Every caller gets the whole panel

`tp review <spec>` and `tp audit <spec>` emit one `prompts[]` entry per active role, always, to
whoever ran the command. Under `tp run` each role unit's brief command is exactly that invocation
(`spec/0.35.0.md` §3.3.1), so **every unit receives every role's prompt and reads one**.

Measured on this spec's own round 3, four roles:

| | Bytes |
|---|---|
| One role's prompt | 10,183 – 10,334 |
| One invocation's payload | **41,015** |
| Four units, one round | **164,060** — of which 123,045 is never read |

A human orchestrator pays the same shape differently: both cycles piped the JSON into a script and
split it by role, every round. That script re-derives `prompts[].role`, which tp already computed,
and it is the step most likely to be skipped or mis-written under time pressure.

### 4.1 Mechanism — `--role <name>`

- `tp review <spec> --role <name>` and `tp audit <spec> --role <name>` emit the payload with
  `prompts[]` reduced to the single entry whose `role` equals `<name>`. Every other top-level key is
  unchanged.
- `<name>` is matched against the active role set for that phase — the same set the unrestricted
  invocation would emit. A name that is not in it exits **2** with a hint listing the active roles.
- A name that is in the corpus but *inactive* for this spec — the per-spec deactivation of v0.32.0,
  reported in `skipped_roles` — is refused by the same rule and the same exit code. The hint names
  it as skipped rather than unknown, because the two are different operator mistakes.
- `--role` is legal wherever prompts are emitted and refused with exit **2** where none are. The two
  commands do not have the same flag sets, so the modes are enumerated separately rather than as
  "and audit's equivalents":

| Command | `--role` legal in | `--role` refused in |
|---|---|---|
| `tp review` | default, `--perspective`, `--diff-from`, `--verify` | `--merge`, `--record`, `--status`, `--report`, `--resolve`, `--resolve-all` |
| `tp audit` | default only | `--merge`, `--record`, `--status`, `--resolve`, `--resolve-all` |

Verified against the shipped binary: `tp audit` registers no `--perspective`, `--diff-from`,
`--verify` or `--report`.

### 4.2 Who it is for

P1 asks whether a change serves the agent. Under `tp run` a role unit spends 41 KB of its context to
read 10 KB of it; `--role` removes three quarters of that, per unit, per round. For a human
orchestrator it replaces the split script with one invocation per role.

### 4.3 Why not `--out-dir`

An earlier draft wrote one `<role>.prompt.md` per prompt into a directory
and added a `prompt_path` key. It was cut for reasons that were measured, not argued:

1. Its only P1 claim — that under `tp run` "the spawner reads `prompt_path` and passes a file rather
   than carrying a 75 KB body through its own context" — is false. Verified in
   `internal/engine/childenv.go`: the child environment carries `TP_RUN_ID`, `TP_ROUND_DIR`,
   `TP_UNIT_ID` and five others, and **no prompt at all** — neither a body nor a path. The unit
   calls `tp review` itself. The driver never pays the cost the flag claimed to remove.
2. It multiplies the waste it was meant to fix. Each of N units still emits the whole panel, so a
   four-role round writes sixteen files, twelve of them redundant.
3. `prompt_path` carries no information its consumer lacks: the caller passed the directory and
   `prompts[].role` is on the same object.
4. `<role>.prompt.md` has no phase discriminator, so a review role and an audit role of the same
   name collide in one directory.

`--role` answers the same measurement without any of the four.

## 5. Non-Goals

1. **Sharding a role's checklist.** `spec/0.41.0.md`. This release does not claim to be a
   precondition for it: sharding is more entries in a `prompts[]` array that already carries N of
   them, and that file names `spec/0.40.0.md`'s measurement as its real precondition.
2. **Provisioning an auditor with a way to perturb, or grading the evidence it returns.** §2's
   clause converts the one class §2.4 names — a defect whose proof needs a code change — into an
   asserted finding, and `spec/0.42.0.md` owns both the recipe (§9) and the verdict for un-run
   evidence. §2.4 states the gap rather than closing it.
3. **Convergence arithmetic.** `spec/0.37.0.md` owns the rule that turns a row set into a verdict.
4. **Enforcing the isolation constraint on the interactive path.** It gets the instruction, not a
   fence; a fence there needs a hook that path does not install.
5. **Admitting a partial file as a complete unit.** §3.1 and §3.3 explain why the predicate is
   unchanged and leave it to `spec/0.41.0.md` §2.
6. **Changing what a round asks.** The checklist items, role rules and output schema are untouched.
   The prompt does gain two behavioural clauses — that is §2 and §3, and §1 prices them; the claim
   here is narrower than "nothing changes".

## 6. Tests

**Clause emission.**

1. The emitted role prompt for `tp review` contains §2.2's clause as an exact byte-equal line, and
   the same for `tp audit`.
2. The same, for §3.2's clause.
3. Both clauses fall inside the `## Unit framing` section, after the line naming the output file,
   §2's before §3's — asserted on offsets, not on presence.
4. Both clauses are absent from a prompt that is not a role unit's — `--perspective testing`, whose
   single prompt carries role `test-planner` and an empty `output_path`. `--perspective
   documentation` and `--perspective code-audit` are asserted the same way, so the rule is pinned
   for every non-role prompt rather than for one.
5. Byte cost: the difference between a v0.35.2 role prompt and a v0.36.0 one, for the same spec and
   corpus, is exactly the two clauses plus their separators — asserted as a byte count, which fails
   if either clause is reworded.

**`--role`.**

6. `tp review <spec> --role <name>` emits exactly one `prompts[]` entry, whose `role` is `<name>`
   and whose `prompt` is byte-identical to that role's entry in the unrestricted invocation of the
   same command. `tp audit` likewise.
7. Every top-level key other than `prompts` is byte-identical to the unrestricted payload —
   including `skipped_roles`, which still lists what the spec deactivated.
8. An unknown role exits 2 with a hint listing the active roles.
9. A role that exists in the corpus but is in `skipped_roles` for this spec exits 2, and the hint
   says it is skipped, not unknown. The two hints differ.
10. `--role` with each refusing mode in §4's table exits 2 with a hint naming both flags — run per
    command against that command's own list, not a shared one.
11. `--role` with `--perspective`, with `--diff-from` and with `--verify` exits 0 and emits one
    prompt. This test is `tp review` only; `tp audit` has none of those flags, and asserting them
    there would be asserting a parse error.

**Regression.**

12. Without `--role`, the payload for a fixed spec and role corpus differs from v0.35.2's only by
    the two clauses inside each `prompts[].prompt`. The fixture is generated by the test from the
    current corpus rather than committed: the payload embeds the round number, the loop budget, the
    consecutive-clean count and the `output_path`, so a committed blob would encode this
    repository's review state on the day it was written and fail on the next recorded round. The
    test fixes those inputs explicitly and asserts the diff.

## 7. Release gate

Nothing here ships until §2's and §3's clauses are replayed against the recorded rounds in
`spec/.tp-review/` and shown not to lose a finding tp used to surface. Every mechanism in this
release hides, regroups or narrows what a reviewer sees — `--role` most of all — which is the shape
that cost v0.34.0 §7.1 eight rounds of suppressed findings. The replay is a release gate, not a test
appendix.
