# tp v0.36.0 — The emitted round

## 1. Overview

tp emits one prompt per role per round. That prompt carries the checklist, the role rules, the
affected files and the output schema — everything about *what to judge*. It carries nothing about
*how the unit must behave while judging*, and it cannot be written anywhere except stdout.

The consequence is that every round costs the orchestrator the same manual additions, and a round
that does not get them loses work or produces false findings. Two independent cycles measured the
same shortfalls, on different projects, in different languages:

- tp's own v0.35.0 audit — nine rounds, four roles, this repository.
- A field cycle on a PHP package, nine rounds, five roles, driven by a different operator.

This release makes the emitted prompt self-sufficient and gives it somewhere to go. It changes
nothing about what a round asks or how convergence is counted.

**What it costs, stated up front because the release is nothing but added bytes.** Two clauses go
into every emitted role prompt: roughly 60 words, under 400 bytes, against a prompt that runs 4 KB
for this spec and 75 KB for a large audit role. The clauses are emitted on every path, including the
one where §2's constraint is already enforced by a fence — tp emits the prompt before anything knows
which runner will consume it, so a conditional emission is not computable at the point of emission,
and an unconditional 400 bytes is cheaper than the machinery to avoid it. §4's flag adds no prompt
bytes and removes bytes from the payload.

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

**Which fence exists, and which does not.** tp ships two `PreToolUse` hooks and they are not
interchangeable. `spec/0.35.0.md` §6.2's `pre-tool-use-write-deny.sh`, registered in `hooks.json`,
denies four tp state paths and **allows every source file** — that section says so, and calls itself
a guard against hand-editing rather than a sandbox. §6.3's `pre-tool-use-role-write-allow.sh`,
registered in the frontmatter of `agents/tp-reviewer.md` and `agents/tp-auditor.md`, is the one that
denies a source write: a role unit may write exactly one path and everything else is refused.

Both were verified against the shipped hooks rather than read: `internal/cli/audit.go` is allowed by
the §6.2 hook and denied by the §6.3 hook.

**So the clause is for the interactive path, and the split is drawn on §6.3.** A runner that loads
the agent definitions gets the fence; an orchestrator that spawns sub-agents itself — this
repository's own documented fallback, and the PHP cycle's method — loads nothing and gets no fence.
§6.3 itself calls the restriction "defence in depth", which is a statement that it is not the only
thing that should be true.

**The clause, in the exact words it must be emitted in**, so that §6's tests have something to
assert and an implementer is not inventing the deliverable:

> Do not modify the working tree. Read anything; write only the findings file this prompt names.
> Report a defect you would have had to modify code to prove as a finding with its evidence, not as
> an edit.

**The findings file is excluded on purpose.** §3 requires the unit to append to a file that lives
inside the working tree, so an unqualified "do not modify the working tree" would forbid the
release's own other half. The wording above names the exception rather than leaving a reader to
infer it.

**No perturbation recipe is offered, and that is a decision rather than an omission.** An earlier
draft told the unit to copy the subject out of the repository and mutate the copy. Three things
defeat it: the copy is a second write, which §6.3 forbids outright; a source file moved out of its
package does not build, so the experiment the motivating story describes cannot be run on it; and
`spec/0.42.0.md` §9 has already assessed exactly this recipe and found against it, noting that N
auditors each building a scratch copy would dissolve the concurrency argument v0.35.0 §3.3 rests on.
Giving an auditor a way to perturb is that file's open question. This release constrains; it does not
provision.

**Why the prompt rather than the agent definitions.** The definitions are the cheaper channel — they
cost nothing per round — and they are the right home for a standing rule under `tp run`. They are
also loaded by nothing on the interactive path, which is the only path where this constraint is not
already enforced. Emitting it is therefore the channel that reaches the case that needs it. A future
release that makes the interactive path load a definition should move the clause there and delete it
here.

## 3. A unit that buffers its findings loses all of them

Both cycles lost whole rounds to the same shape. An agent accumulates its findings in context and
writes the NDJSON once, at the end; the process dies before that write; nothing survives.

The field cycle lost **six** subagents this way over one session. The single run that had been
instructed to append each row as it was decided preserved **72 rows** despite dying near the end.
tp's own cycle lost four, and the one told to write early kept 9 of its 10 rows — so the relaunch
completed the round instead of redoing it, and the recovered rows were the expensive ones.

The asymmetry is the argument: buffering saves nothing and risks everything, because a findings file
is append-only NDJSON whose partial state is already valid input to `--merge`, which warns on a
truncated trailing line and exits 0.

**The clause, in the exact words it must be emitted in:**

> Write each finding to that file as you decide it, not once at the end. A run that dies with its
> findings unwritten loses the whole round; a partially written file is still usable.

**What this does not change, stated because a reader could reasonably conclude otherwise.** A
findings file that exists and parses does not mean the unit finished. It never did: completion is
`exit 0` plus the driver's rename of `.part` to the final name (`promoteRoleFindings`, which returns
without renaming unless `exitCode == 0`, §3.3.1), and the §6.2 Stop hook's "exists and parses"
predicate is a liveness check that catches a unit stopping with nothing, not a completeness check.
Incremental append makes partial files the normal intermediate state, so it makes that misreading
easier — it does not create it. A unit that stops cleanly having judged three of forty items merges
as complete today, before this release and after it.

That remaining hole is real and is **not** repaired here. It is the same question `spec/0.41.0.md`
§2 must answer for sharding, where N partial files are jointly complete and no single one is, and it
should be answered once, there, rather than twice.

## 4. The prompts have nowhere to go

`tp audit <spec> --out-dir <dir>` is an unknown flag. `--output` exists on both `review` and `audit`
but is scoped to `--merge` — verified: `-o` without `--merge` already exits 2 with a hint — so it
names a merged output file and can never receive emitted prompts.

Both cycles therefore piped the JSON into a script and split it by role, every round. That script
re-derives `prompts[].role`, which tp already computed, and it is the step most likely to be skipped
or mis-written under time pressure.

**Mechanism.**

- `--out-dir <dir>` writes one file per emitted prompt, named `<role>.prompt.md`. The role is the
  `prompts[].role` value, which is already constrained to the role-file basenames under
  `.tp/reviewers/` and `.tp/auditors/`; a role whose name is not a safe single path segment is
  refused rather than written.
- Each `prompts[]` entry gains `prompt_path`, the path written. The name is deliberately distinct
  from the existing `output_path`, which means the *findings* destination and is unchanged.
- **With `--out-dir`, `prompts[].prompt` is omitted from the payload.** Emitting the body twice —
  once inline, once on disk — would make the flag cost more than the script it replaces. This is the
  one payload difference the flag makes, and §6 pins it in both directions.
- The flag is legal in every mode that emits prompts: the default mode, `--perspective`,
  `--diff-from` and `--verify`. It is refused with exit 2 in modes that emit none — `--merge`,
  `--record`, `--status`, `--report`, `--resolve`, `--resolve-all`, and audit's equivalents.
- A destination that cannot be written exits **3** (`ExitFile`, tp's vocabulary for a path that
  cannot be used) with a hint naming the directory, and writes no partial set.

**Who it is for.** P1 asks whether a change serves the agent. This one serves the driver: under
`tp run` the spawner reads `prompt_path` and passes a file rather than carrying a 75 KB body through
its own context, and the omission of `prompt` from the payload is the agent-side saving. For a human
orchestrator it replaces a script. Both are real; only the first is a P1 argument, and the release
does not claim more.

## 5. Non-Goals

1. **Sharding a role's checklist.** `spec/0.41.0.md`. This release does not claim to be a
   precondition for it: sharding is more entries in a `prompts[]` array that already carries N of
   them, and that file names `spec/0.40.0.md`'s measurement as its real precondition.
2. **Provisioning an auditor with a way to perturb.** `spec/0.42.0.md` §9. §2 ships a constraint and
   deliberately no recipe, so nothing here crosses that boundary.
3. **Convergence arithmetic.** `spec/0.37.0.md` owns the rule that turns a row set into a verdict.
   This release does not change which role outputs are admitted as complete either — §3 explains why
   the admission predicate is unchanged, and leaves that predicate's own weakness to
   `spec/0.41.0.md`.
4. **Enforcing the isolation constraint on the interactive path.** It gets the instruction, not a
   fence; a fence there needs a hook that path does not install.
5. **Changing what a round asks.** No checklist item, role rule or output schema changes.

## 6. Tests

1. The emitted role prompt for `tp review` and for `tp audit` contains §2's clause verbatim.
2. The emitted role prompt contains §3's clause verbatim.
3. Both clauses are absent from a prompt that is not a role unit's — `--perspective testing`, whose
   single prompt carries role `test-planner` and an empty `output_path`.
4. `tp review <spec> --out-dir <dir>` writes one `<role>.prompt.md` per entry in `prompts[]`; the
   file set equals the emitted role set, and a role listed in `skipped_roles` produces no file.
5. `tp audit <spec> --out-dir <dir>` does the same.
6. Each `prompts[]` entry carries `prompt_path` naming the file written, and the file's bytes equal
   the `prompt` the same invocation would have emitted without the flag.
7. With `--out-dir`, `prompts[].prompt` is absent. Without it, `prompt` is present and `prompt_path`
   is absent.
8. Without `--out-dir`, the payload for a fixed spec and role corpus is unchanged from v0.35.1's
   apart from the two clauses inside `prompts[].prompt` — asserted against a committed fixture of
   that payload, not against a description of it.
9. `--out-dir` with `--merge` exits 2 with a hint naming both flags; the same for `--record`,
   `--status`, `--report`, `--resolve` and `--resolve-all`, and for audit's equivalents.
10. `--out-dir` with `--perspective`, with `--diff-from` and with `--verify` exits 0 and writes the
    files.
11. `--out-dir` pointing at a path that cannot be created exits 3 with a hint naming the directory,
    and no file from that invocation exists afterwards.
12. A role whose name is not a safe single path segment is refused with exit 2 before anything is
    written.
