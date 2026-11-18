# Gate slots

A decision note (Decision track). It is built after rolling implement slots
(`rolling-implement.md`) land and before the found-tasks implement phase runs at
`run_max_parallel` 8.

## Operator decisions (2026-11-18)

- **A cap on concurrent gates.** `tp run` limits how many quality gates run at once, so that eight
  slots overlap on agent time rather than on test load. tp's own config sets it to **2**.
- **A full suite goes through `tp gate`.** A unit runs a full suite only through `tp gate`, which
  takes the same slot. The brief and CLAUDE.md tell units to run only targeted package tests
  directly. This is a behavioural rule, not a mechanical one: tp cannot fence an agent's own
  commands. The run measures how many test processes run outside the slots, at their peak.
- **The first real run uses 8.** It is not staged through 4 first.
- **tp's own `quality_gate` is unchanged.** It is not lowered to `-p 2 -parallel 4`.

## Decision

- **The field.** `run_max_parallel_gates` is an int in the `run_max_*` family. Unset means
  unlimited, which is today's behaviour: v1.20.0 decided that unit gates run concurrently, and a
  default of 1 would undo that silently. Raising it is the operator's, fenced under `TP_UNATTENDED`
  like the other caps. Lowering it is free.
- **The mechanism.** N slot files under the run directory, taken with a cross-process `flock`. A
  requester tries every slot without blocking and retries at a short interval when all are full.
  The kernel releases a slot when its holder dies, so a slot is never left stale: no pid file, no
  counter. A child finds the slots from the run root and run id its environment already carries. A
  hand-run `tp done` outside a run takes no slot.
- **Who takes a slot.**
  - a unit's worktree gate, inside its close request;
  - `tp gate` inside a run;
  - the main-branch gate at integration.

  A reused main gate (`gated_tree`) takes no slot.
- **The main gate goes first.** Under the serial rule nothing refills while a main gate runs, so a
  main gate waiting behind unit gates would stall the whole pool. The driver therefore sets a "main
  waiting" mark, and while it is set no unit gate takes a new slot. The cap binds the total: at
  most N gates at once, the main gate included.
- **Time.** `gate_timeout_seconds` starts once a slot is taken. The wait is measured on its own, and
  the run state records unit time, gate time and wait time as three separate numbers. A unit that is
  waiting is never killed by a timeout and never spends an attempt; any clock that would count the
  wait has the wait subtracted or is documented as a limit.
- **Signals.** A signal during the wait makes the unit exit cleanly without asking to close, and
  without holding a slot.
- **Fairness.** Slots are polled, not queued fairly. This is accepted and documented, and the
  longest wait is measured.
- **`tp gate`.** It runs the resolved `quality_gate` from the project directory. Inside a run it
  holds a slot; outside one it runs at once. It prints the gate's result, writes nothing and closes
  nothing.

## What must be shown

- A test: with the cap at 2, never more than 2 gates run at once, the main gate included.
- A test: a reused main gate takes no slot.
- A test: a child killed while it holds a slot frees the slot at once.
- A test: a wait longer than `gate_timeout_seconds` neither kills the unit nor spends an attempt.
- **Measurement.** The expected numbers are written before the run, on rolling's fixture and against
  rolling's peak-gates baseline. At cap 2 against unlimited:
  - the peak of concurrent gates;
  - the wait, in total and at its longest;
  - the difference in wall time.

  The first 8-slot run of the found-tasks implement phase also reports the peak number of test
  processes outside the slots.

## Non-goals

Fencing an agent's own `go test` calls; changing tp's `quality_gate`; fair queuing.
