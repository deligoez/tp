# tp — The decomposition is built before it is imported

A backlog spec, named by slug; its priority number and its release number are assigned later. Its
measurements are in `decompose-build-probe-measurements.md` beside it; this file stands without them.

Class: **loop** — it adds an emitted prompt and a recorded result to the cycle, so it is reviewed by
the mechanism it extends. Budget it at the loop-class median in `CLAUDE.md`'s *What a cycle costs*
table.

## 1. The decision

**Context.** Decomposition today is a reading: an agent reads the converged spec and writes tasks
whose acceptance criteria predict what the code will do — which tests turn red, which files change,
how long it takes. Nothing in the cycle runs a task before its implementer does — the checks a task
file passes before import are listed in the measurements file, and none of them executes anything.
The `v1.1.0` cycle measured what that costs: the
fixture migration the decomposition sized on a narrow probe turned out to touch thirty test files;
four tasks' *"these tests turn red"* predictions were wrong for the same reason; and three of the ten
code tasks closed with no production change at all, because the behaviour they were written to
create was already true at `HEAD` — each implementing unit discovered this by building, minutes into
the task. The same cycle's review phase had run three rounds over the spec without seeing any of
it, because a review reads the spec and the fixtures are in the suite. The only step that ever
measured the decomposition was the one implementer who built it in a clone before review; that step
is now a rule in `skills/tp/SKILL.md` for the spec, and this release makes it a phase for the tasks.

**Decision.** Between decomposition and import there is one **probe round**, executed by an agent
unit the way a grounding round is, and recorded by tp:

1. **tp emits a probe prompt per task.** The prompt carries the task, its acceptance criteria, its
   source sections, and one instruction: in a clone outside the repository, make the smallest change
   that would satisfy each criterion, run the full suite, and report three observations — the files
   the change touched, the tests the change turned red, and for each criterion whether the change
   was needed at all or the criterion already held at `HEAD`. The unit builds; it does not judge.
2. **tp records the probe against the task.** `--record` stores the unit's three observations per
   task and reports how many tasks now carry one. A probe is a claim about one acceptance text: when
   that text changes, the probe no longer applies to it.
3. **`tp validate` reads the probe and reports, gating nothing.** Three warnings, each naming the
   task: a task with no probe; a criterion the probe found already true at `HEAD`; and two tasks
   whose probes turned the same test red, which is the split rule — *what the gate sees* — checked
   by observation rather than by reading.
4. **`tp next --brief` carries the task's probe**, so the implementing unit starts from the measured
   file set and red set instead of the decomposer's prediction.

**Consequences.** A decomposition is validated the way the cycle now validates a spec: by running
the thing, once, before the phase that depends on it. The probe's cost is one unit per task in a
clone, paid once; the cost it replaces is discovered mid-task, per task, with the gate running. What
this does not do is make tasks atomic by itself — it makes the non-atomic ones visible, and the
decomposer still splits them.

**Alternatives considered.** Iterating the decomposition through *reading* rounds, with roles
reviewing the task file as they review a spec: rejected, because a task's acceptance is a claim
about unbuilt code and the `v1.0.1` and `v1.1.0` cycles both measured that such claims are refuted
by running and not by reading — a reading round would reproduce the review loop's failure on a
second document. Gating import on the probe: rejected here; `tp import` keeps its current fence and
`--force` semantics, and the warnings are the operator's to read. Making the implementer role's
review-phase judgement of decomposability the probe: rejected, because that role reads the spec
before tasks exist and cannot build what is not yet written down.

## 2. What the probe records

One object per task, written by `--record` and read by `tp validate` and `tp next --brief`:

| key | value |
|---|---|
| `files` | paths the smallest satisfying change touched, as the unit reported them |
| `red` | test names the full suite reported failing after that change |
| `criteria` | one entry per acceptance criterion: `needed` or `already-true` |
| `sha` | the commit the clone was taken from |
| `recorded_at` | timestamp |

A row for a task id that is not in the file, or a `criteria` list whose length differs from the
task's criterion count, is refused at record with the line named, and nothing is written. A task
whose acceptance changes after its probe was recorded has a stale probe; `tp validate` reports it
as a task with no probe, since the prediction it measured no longer exists.

## 3. Non-Goals

- **No gate.** Import is not refused on a missing or unfavourable probe.
- **No automatic split.** The probe reports overlap; the decomposer decides the seam.
- **No task is exempt.** A documentation task is probed like any other; its probe reports no red
  tests and says which criteria were already true, which is cheap and is itself information.
- **No change to closure verification** or to the atomicity limits the skill states.
- **No estimate correction.** `estimate_minutes` stays the decomposer's; the probe does not time.

## 4. Tests

Each row names its fixture and the mutant that turns it red; the value at `HEAD` and the value
under the mutant are filled in by the implementing task, since the subject does not exist yet.

| # | WHEN | tp SHALL | fixture | mutant |
|---|---|---|---|---|
| 1 | a task file with two code tasks is given to the probe emission | emit two prompts, each naming its task id, criteria and source sections | two-task file | an emission that names the file's first task in both prompts |
| 2 | `--record` reads a row whose `criteria` length differs from the task's criterion count | exit 1 naming the line, write nothing | one task with three criteria, a row with two entries | a record that writes the truncated list |
| 3 | `--record` reads a row for an id not in the file | exit 1 naming the line, write nothing | row with id `ghost` | a record that appends a task |
| 4 | `tp validate` runs on a file where one task has no probe | one warning naming that task | two tasks, one probed | a validate that reads no probes |
| 5 | a probe marks a criterion `already-true` | one warning naming the task and the criterion index | one task, second criterion already true | a validate that reports only missing probes |
| 6 | two tasks' probes share a test name in `red` | one warning naming both tasks and the test | two tasks with one common red test | a validate that compares `files` instead of `red` |
| 7 | a task's acceptance text changes after its probe was recorded | `tp validate` reports it as unprobed | probe recorded, then `tp set <id> acceptance=...` | a validate that keys the probe on the id alone |
| 8 | `tp next --brief` claims a probed task | the brief carries `files` and `red` | the task from row 4 | a brief that omits the probe object |
