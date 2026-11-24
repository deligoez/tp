# tp v1.31.0 — found tasks

> Full track. Source: the `found-tasks` backlog note (`spec/backlog/found-tasks.md`), its operator
> decisions 1–7 and the three interview answers of 2026-11-24 (§12). What the decisions rest on — the
> field's counts and three seeded runs — is `spec/1.31.0-measurements.md`.

tp exists to run for long stretches without a person. When a unit runs into an obvious defect outside
its task, it files a request; tp turns a well-formed request into a task, places it, and the run does
the work. A request that needs a choice goes to a decision unit, which opens a task only where written
text settles the choice; everything else waits in a queue the person settles at the end. A queue entry
holds only the finders its defect blocks, and the release only through them. A cap bounds how much
a spec can grow this way.

## 1. The request

A unit that runs into a defect outside its own task files a request with `tp found add`, one defect
per request. The four Fix fields are a location (`file:line`), a reproduction (a command or test that
shows it), the expected behaviour and the observed behaviour. A request that needs a choice also
carries the question and the alternatives. `blocks`, set only by a unit working a task, marks a
request its task cannot pass without. A unit reports what its own work ran into; it never sweeps code
for defects, and every unit prompt that can file requests says so.

`tp found add` validates the shape and records the request in the spec's run state; it changes no
task. A request with no location, no reproduction, or neither all four Fix fields nor a question is
refused with exit 2.

Why: a request is the unit's only output about work it was not given, so its shape is what every later
step checks (measurements §2: a looser format let one request bundle two defects and one defect arrive at two lines).

**Acceptance.**
- A request with all four Fix fields records; the same request with no reproduction exits 2.
- The implement and audit-role unit prompts each carry the no-sweep sentence.

## 2. Applying requests

`tp found apply` applies every recorded, unapplied request of a spec under the task-file lock. A
request whose file and reproduction (whitespace-normalised) match one in its batch or one whose
outcome is open is a duplicate: it opens nothing; its finder is placed against the kept outcome
(§4). A Fix-shaped request (the four Fix fields, no question) becomes a found task (§3), blocking
ones first, then by arrival, within the cap (§8); the rest go to the decision unit (§5). A request
whose location falls inside an open task's files opens nothing, does not count against the cap, and
is attached to that task as a note; that task is its outcome.

`tp run` applies after every unit; an audit unit's requests are applied only after its round is
recorded. In a hand-coordinated session the pool's `Stop` hook refuses the coordinator's stop once
while unapplied requests exist, naming `tp found apply`.

Why: one command owns the cap, the dedup and the placement, so a driven run and a hand-coordinated one
behave the same (interview answer 1). Blocking first: arrival order shed the one defect that stopped a
unit (measurements §2).

Rejected: dedup by exact location (one defect at two lines, two defects at one line); applying audit
requests mid-round (the round's own rows would see half-applied work).

**Acceptance.**
- Two requests in one batch with the same reproduction at different lines open one task.
- A blocking duplicate of an applied request opens nothing; its finder depends on the kept task.
- With the cap one short, a blocking request filed after a non-blocking one is the one applied.
- A request inside an open task's files opens no task and shows on that task.

## 3. The found task

A found task carries `origin` — the finder (a task id, or an audit round and row key) and the
request's location — in place of `source_sections`, never both. Its brief carries the request's
fields and no spec excerpt. Its first step is the reproduction: when the unit cannot make it fail for
the stated reason, it runs `tp found handback <id> "<why>"`; the task closes as done, marked handed
back, and a queue entry (§6) holds its request and any finder that waited on the task. Spec coverage
leaves found tasks out of its denominator; a task audit reaches them through their files.

`tp add` refuses a `source_sections` entry that names no heading of the spec; `origin` joins the
anchors that satisfy today's entry check.

Why: a found task answers a request, not a spec section; a made-up anchor hid that in the field
(measurements §1). The reproduction is the second filter: every Fix-shaped request in the seeded run
turned red, and the field's wrong findings were all non-Fix-shaped.

Rejected: anchoring found tasks to a catch-all section (the field's practice, measurements §1).

**Acceptance.**
- `tp add` with a `source_sections` entry naming no heading exits non-zero naming the entry.
- A found task's brief shows the request's fields and no excerpt; `tp validate` accepts it.
- After `tp found handback`, the task is done and marked handed back, and the queue lists its request.
- Spec coverage reports the same figures before and after a found task is added.

## 4. Placement

Placement adds at most one edge, between a task finder and the request's outcome: its found task,
the open task it was attached to, or the duplicated request's found task. A non-blocking found task
depends on its task finder; an audit row's depends on nothing, and a non-blocking duplicate or
attachment adds no edge. A blocking request makes the finder depend on the outcome; `tp found apply`
returns the finder to open, and the attempt is not counted against its retries.

A blocking request whose edge would close a cycle becomes an open queue entry. One with no task
outcome makes its finder wait, open and not ready, until the request gets a task outcome, a
`conforms` answer, or a queue entry that is settled. Handing back a found task moves the finders
depending on it to wait on the new entry. Settling into a task makes the finder depend on that task;
any other end makes it ready. When only waiting tasks remain, the decision unit runs first if a request
is undecided; the run stops with `no-ready-task` if none is ready after it.

Why: the two cases the seeded run produced (measurements §2), with the smallest graph change; a finder
whose blocker is unfixed waits rather than retry into it (operator, 2026-11-25).

**Acceptance.**
- A non-blocking found task lists the finder in `depends_on`.
- After a blocking request, the finder lists the found task in `depends_on`, is open, and its attempt
  count is unchanged.
- After a blocking request that becomes a queue entry, the finder is open and not in `tp next`'s ready
  set; after `tp found settle` rejects the entry, it is ready.

## 5. The decision unit

A new unit kind and a fourth plugin agent, never the finder, converts a request that needs a choice
into one of three answers:

- `task`: the Fix fields, a reproduction the unit ran and saw fail, and a quote that tp finds verbatim
  (whitespace-normalised) in the cited file — the spec, `CLAUDE.md`, `CONVENTIONS.md`, `REFERENCE.md`.
  tp applies it like a Fix-shaped request (§2), never as a duplicate of the request it answers.
- `conforms`: the reproduction of the expected behaviour passes on the current code, with a verbatim
  quote checked the same way. No task opens.
- `human`: everything else, including any answer that rests on an inference, a combination of
  sections, or the reading of an ambiguous word. It becomes a queue entry.

When in doubt the unit answers `human`. It takes at most 5 requests per unit and runs when undecided
requests exist and no implement unit is ready or running. Its `task` answers count against the cap;
one past the cap becomes an open queue entry, quote and reproduction attached. A rule-9 decision is
never its to make.

Why: the two-answer set forced tasks that changed nothing, and every wrong `task` had expected = observed
(measurements §3); with the third answer and the quote check the contract met its pass bar
(measurements §4). A spec-silent choice is the person's (operator decision 6).

Rejected: no decision unit (the field's citation-settled findings would all wait for the person);
`drop` (it discards a defect; `conforms` says there is none, with evidence).

**Acceptance.**
- A `task` answer whose quote is not in the cited file is refused and the request stays undecided.
- A `conforms` answer opens no task and appears in the queue's decided list.
- A `human` answer adds a queue entry with the question and the alternatives.
- A `task` answer past the cap adds an open queue entry and opens no task.

## 6. The queue

A committed file beside the spec's task file holds the spec's queue. An entry is open (awaiting the
person) or decided (a `conforms` answer, kept so it stays auditable). Each carries its location,
question, alternatives and the finders waiting on it. `tp found settle` takes a batch and settles
each open entry one of two ways: make a task in that spec's task file (not counted against the cap)
or reject with a reason, which may say where the defect went.

`tp resume` shows the count of open entries and "N entries hold M tasks"; the converged payload
carries the open count. This replaces the deferred `tp note` inbox in `spec/undecided.md`.

Why: the person's input comes at the end (the note's direction), and an entry holds only what its
defect blocks (operator decision 3, narrowed 2026-11-25); a committed file survives the machine.

**Acceptance.**
- With an open entry that holds no finder, `tp review/audit --status --check` and `tp resume`'s
  phase are what they would be without it.
- `tp resume` prints the open entry count and how many tasks wait on them.
- One `tp found settle` call makes a task of one entry and rejects another.

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

`found_max` (workflow field, default 5; 0 is uncapped, as for every `*_max` field, at the operator's
request of 2026-11-25) caps the found tasks one spec's task file may ever hold, counting every found
task it has carried, `task` answers included and tasks the person settles excluded (interview answer
3). Fix-shaped requests and `task` answers past it become queue entries. Raising it, or setting 0, is
an operator decision under rule 9 (`raise-found-cap`).

Why: per run, a stop and a restart would lift the cap. It bounds found tasks only; the field's larger
growth, spec work re-decomposed, is a non-goal (§11).

**Acceptance.**
- With `found_max=0`, a sixth Fix-shaped request opens a found task.
- After five found tasks, a sixth Fix-shaped request is queued, even after the run restarts.
- `tp set --workflow found_max=6` under `TP_UNATTENDED` exits 2 with `raise-found-cap`.

## 9. The fence

Under `tp run`, a unit of any kind but `decompose` that calls `tp add`, `tp import`, `tp set
depends_on` or `tp found settle` exits 2 naming `tp found add`.

**Acceptance.**
- With `TP_UNIT_KIND=implement` and `TP_UNATTENDED=1`, `tp add` exits 2 naming `tp found add`; with
  `TP_UNIT_KIND=decompose` it succeeds.

## 10. Tests

Each row's counts at HEAD and under its mutant are confirmed in the implementing task.

| Row | Fixture | Mutant that must fail it |
|---|---|---|
| Dedup keys on the reproduction | two requests, one reproduction, locations at lines 40 and 44 | dedup keyed on `file:line` |
| Blocking first within the cap | cap one short; a non-blocking request, then a blocking one | apply in arrival order |
| The cap holds across a restart | five found tasks applied, the run stopped and started, a sixth Fix-shaped request | count found tasks per run |
| A quote must be in the cited file | a decision `task` answer quoting a sentence absent from the spec | skip the quote check |
| The quote check normalises whitespace | a quote that spans a line break of the spec | compare raw bytes |
| A blocking placement spends no attempt | a unit that files a blocking request and exits without closing | count the exit as a failed attempt |
| The fence | `TP_UNATTENDED=1` with `TP_UNIT_KIND=implement`, then `decompose`, calling `tp add` | fence keyed on `TP_UNATTENDED` alone |
| An unresolved anchor is refused | `tp add` with `source_sections` naming no heading | warn instead of refuse |
| An entry holds only its finders | one open entry holding one finder, one unrelated ready task | mark every open task waiting while an entry is open |

## 11. Non-goals

- The measure → diagnose → decide loop that drove most of the field's growth: spec work re-decomposed
  mid-run is a separate decision.
- Moving rule-9 escalations into the queue: it conflicts with rolling's "escalation drains and stops".

## 12. Decided with the operator

Decisions 1–7 are the backlog note's. The interview of 2026-11-24 added three: (1) found tasks work in
`tp run` and by hand alike, through one apply command; (2) a found task's commit does not reset the
audit's clean-round streak; (3) the cap counts per spec task file, not per run.
