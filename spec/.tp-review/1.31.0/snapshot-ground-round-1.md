# tp v1.31.0 — found tasks

> Full track. Source: the `found-tasks` backlog note (`spec/backlog/found-tasks.md`), its operator
> decisions 1–7 and the three interview answers of 2026-11-24 (§11). What the decisions rest on — the
> field's counts and three seeded runs — is `spec/1.31.0-measurements.md`.

tp exists to run for long stretches without a person. When a unit runs into an obvious defect outside
its task, it files a request; tp turns a well-formed request into a task, places it, and the run does
the work. A request that needs a choice goes to a decision unit, which opens a task only where written
text settles the choice; everything else waits in a queue the person settles at the end. Nothing in
the queue holds the run, convergence or the release. A cap bounds how much a spec can grow this way.

## 1. The request

A unit that runs into a defect outside its own task files a request with `tp found add`, one defect
per request, carrying: a location (`file:line`), a reproduction (a command or test that shows it), the
expected and the observed behaviour, and a standalone text of at most 400 characters. A request that
needs a choice between alternatives also carries the question and the alternatives. `blocks` marks a
request the unit's own task cannot pass without. A unit reports what its own work ran into; it never
sweeps code for defects, and every unit prompt that can file requests says so.

`tp found add` validates the shape and records the request; it changes no task. A request with no
location, no reproduction, or a text over the limit is refused with exit 2.

Why: a request is the unit's only output about work it was not given, so its shape is what every later
step checks (measurements §2: bundled defects and line-number drift broke a looser format).

Rejected: units calling `tp add` directly (no cap, no dedup, no placement); free text (nothing to
dedup or route on).

**Acceptance.**
- A request with all four Fix fields records; the same request with no reproduction exits 2.
- A text one character over the limit exits 2 naming the limit.
- The implement, audit-role and decision unit prompts each carry the no-sweep sentence.

## 2. Applying requests

`tp found apply` applies every recorded, unapplied request of a spec under the task-file lock, in this
order: drop a request whose reproduction matches an applied one (whitespace-normalised), merging its
`blocks` mark into the kept one; route each remaining request; within the cap (§8), blocking requests
first, then by arrival. A request is Fix-shaped when it has the four Fix fields and no question;
Fix-shaped requests become found tasks (§3), the rest go to the decision unit (§5). A request whose
location falls inside an open task's files, or that restates an open task's acceptance, is attached
to that task as a note and opens nothing.

`tp run` applies after every unit; an audit unit's requests are applied only after its round is
recorded. In a hand-coordinated session the pool's `Stop` hook refuses the coordinator's stop once
while unapplied requests exist, naming `tp found apply`.

Why: one command owns the cap, the dedup and the placement, so a driven run and a hand-coordinated one
behave the same (interview answer 1). Blocking first: arrival order shed the one defect that stopped a
unit (measurements §2).

Rejected: dedup by exact location (one defect at two lines, two defects at one line); applying audit
requests mid-round (the round's own rows would see half-applied work).

**Acceptance.**
- Two requests with the same reproduction at different lines open one task, marked blocking when
  either was.
- With the cap one short, a blocking request filed after a non-blocking one is the one applied.
- A request inside an open task's files opens no task and shows on that task.

## 3. The found task

A found task carries `origin` — the finder's task id and the request's location — in place of
`source_sections`, never both. Its brief carries the request text and no spec excerpt. Its first step
is the reproduction: when the unit cannot make it fail for the stated reason, it runs `tp found
handback <id> "<why>"`, the task leaves the open set, and a queue entry (§6) holds its request. Spec
coverage leaves found tasks out of its denominator; a task audit reaches them through their files.

Anchors become checked at entry: `tp add` refuses a `source_sections` entry that names no heading of
the spec, and a task carrying neither an anchor nor an `origin`.

Why: a found task answers a request, not a spec section; a made-up anchor hid that in the field
(measurements §1). The reproduction is the second filter: every Fix-shaped request in the seeded run
turned red, and the field's wrong findings were all non-Fix-shaped.

Rejected: anchoring found tasks to a catch-all section (the field's practice, measurements §1).

**Acceptance.**
- `tp add` with a `source_sections` entry naming no heading exits non-zero naming the entry.
- A found task's brief shows the request text and no excerpt; `tp validate` accepts it with no anchor.
- After `tp found handback`, the task is not in `tp next`'s ready set and the queue lists its request.
- Spec coverage reports the same figures before and after a found task is added.

## 4. Placement

A found task is placed on the finder's edge only: by default it depends on the finder, so it runs
after. When the request is blocking, the finder depends on the found task instead; the finder exits
without closing, returns to open, and the attempt is not counted against its retries. No third task's
edges change, so placement cannot make a cycle.

Why: the two cases the seeded run produced (measurements §2: two units stopped blocked), with the
smallest graph change.

**Acceptance.**
- A non-blocking found task lists the finder in `depends_on`.
- After a blocking request, the finder lists the found task in `depends_on`, is open, and its attempt
  count is unchanged.

## 5. The decision unit

A new unit kind and a fourth plugin agent, never the finder, converts a request that needs a choice
into one of three answers:

- `task`: the Fix fields, a reproduction the unit ran and saw fail, and a quote that tp finds verbatim
  (whitespace-normalised) in the cited file — the spec, `CLAUDE.md`, `CONVENTIONS.md`, `REFERENCE.md`
  — or a named test. tp applies it like a Fix-shaped request (§2).
- `conforms`: the reproduction of the expected behaviour passes on the current code, with a verbatim
  quote or a named passing test. No task opens.
- `human`: everything else, including any answer that rests on an inference, a combination of
  sections, or the reading of an ambiguous word. It becomes a queue entry.

When in doubt the unit answers `human`. It takes at most 5 requests per unit, runs when undecided
requests exist and no implement unit is ready or running, and after each recorded audit round. Its
`task` answers count against the cap; one past the cap becomes a queue entry already decided, quote
and reproduction attached. A rule-9 decision is never its to make.

Why: two answers forced tasks that changed nothing, and every wrong `task` had expected = observed
(measurements §3); with the third answer and the quote check the contract met its pass bar
(measurements §4). A spec-silent choice is the person's (operator decision 6).

Rejected: no decision unit (the field's citation-settled findings would all wait for the person);
`drop` (it discards a defect; `conforms` says there is none, with evidence).

**Acceptance.**
- A `task` answer whose quote is not in the cited file is refused and the request stays undecided.
- A `conforms` answer opens no task and appears in the queue's decided list.
- A `human` answer adds a queue entry with the question and the alternatives.
- A `task` answer past the cap adds a decided queue entry and opens no task.

## 6. The queue

A committed file under `.tp/` holds one entry per request the run could not turn into work: its
location, question, alternatives, the tasks it holds (finders blocked on it), and `answered_by` (the
decision unit's answer, or empty). `tp found settle` takes a batch and settles each entry one of
four ways: make a task, reject with a reason, move to `BUGS.md`, or move to the backlog. The person
also sees the decision unit's `task` and `conforms` answers there, so they stay auditable.

The queue blocks nothing. `tp resume` shows the count, the oldest entry and "N decisions hold M
tasks"; the converged payload carries the count. This replaces the deferred `tp note` inbox in
`spec/undecided.md`.

Why: the person's input comes at the end, once (operator decision 3); a committed file survives the
context and the machine.

**Acceptance.**
- With a queue entry open, `tp review/audit --status --check` and `tp resume`'s phase are what they
  would be without it.
- `tp resume` prints the queue count and the oldest entry.
- One `tp found settle` call settles several entries of different kinds.

## 7. The audit's part

An audit row on an asked item keeps today's audit-fix path; it never becomes a found task. What rule
13 says to "record with justification and ship" — a finding outside the spec's surface — becomes a
request when it is Fix-shaped, and is recorded and shipped as today otherwise. A found task's commit
does not reset the audit's clean-round streak (interview answer 2); its own task audit and the gate
cover it.

**Acceptance.**
- A Fix-shaped out-of-surface audit finding files a request applied after the round is recorded.
- The audit's consecutive clean count is the same before and after a found task closes.

## 8. The cap

`found_max` (workflow field, default 5, 0 turns the feature off) caps the found tasks one spec's task
file may ever hold, counting every found task the file has carried, `task` answers included
(interview answer 3). Requests past it become queue entries. Raising it is an operator decision under
rule 9 (`raise-found-cap`).

Why: per run, a stop and a restart would lift the cap; per spec it holds in a driven run and by hand
alike, and it bounds the growth the field showed (measurements §1).

**Acceptance.**
- With `found_max=0`, `tp found apply` opens no task and queues every request.
- After five found tasks, a sixth Fix-shaped request becomes a queue entry, including after the run
  restarts.
- `tp set --workflow found_max=6` under `TP_UNATTENDED` exits 2 with `raise-found-cap`.

## 9. The fence

Under a run, a unit of any kind but `decompose` that calls `tp add`, `tp import` or `tp set
depends_on` exits 2 naming `tp found add`.

**Acceptance.**
- With `TP_UNIT_KIND=implement` and `TP_UNATTENDED=1`, `tp add` exits 2 naming `tp found add`; with
  `TP_UNIT_KIND=decompose` it succeeds.

## 10. Non-goals

- The measure → diagnose → decide loop that drove most of the field's growth: spec work re-decomposed
  mid-run is a separate decision.
- Moving rule-9 escalations into the queue: it conflicts with rolling's "escalation drains and stops".
- Counting found tasks as spec growth: the growth check counts spec lines, and found tasks add none.

## 11. Decided with the operator

Decisions 1–7 are the backlog note's. The interview of 2026-11-24 added three:

1. Found tasks work in `tp run` and in a hand-coordinated session alike, through one apply command.
2. A found task's commit does not reset the audit's clean-round streak.
3. The cap counts per spec task file, not per run.

The spec's own loop: review and audit capped at 3 rounds each.
