# Gate slots

A decision note (Decision track). It is built after rolling implement slots
(`rolling-implement.md`) land and before the found-tasks implement phase runs at
`run_max_parallel` 8.

## Operator decisions (2026-11-18)

- **A cap on concurrent gates.** `tp run` limits how many quality gates run at once, so that eight
  slots overlap on agent time rather than on test load. tp's own config sets it to **2**
  (superseded on 2026-11-19, below).
- **A full suite goes through `tp gate`.** A unit runs a full suite only through `tp gate`, which
  takes the same slot. The brief and CLAUDE.md tell units to run only targeted package tests
  directly. This is a behavioural rule, not a mechanical one: tp cannot fence an agent's own
  commands. The run measures how many test processes run outside the slots, at their peak.
- **The first real run uses 8.** It is not staged through 4 first.
- **tp's own `quality_gate` is unchanged.** It is not lowered to `-p 2 -parallel 4`.

## Operator decisions (2026-11-19)

- **tp's own config sets the cap to 4**, not 2. The 2026-11-18 line is superseded.

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
  main gate waiting behind unit gates would stall the whole pool. The main gate's own `tp done`,
  which the driver spawns for the close, holds `main.lock` under the run's slot directory
  exclusively while it waits, and releases it once it holds a slot. A unit gate tries the slots only
  under a shared, non-blocking hold of that same lock, released right after the try. While the main
  gate waits, no unit gate can take the shared hold, so none takes a slot; and since the exclusive
  hold cannot be granted while a unit's try is in flight, no unit can be part-way into a slot when
  the main gate starts waiting. No probe, take and probe again is needed. Both are `flock`s on
  files opened close-on-exec, so a crashed holder leaves nothing behind and the gate's own child
  never inherits them. The cap binds the total: at most N gates at once, the main gate included.
- **A killed slot holder.** The cap bounds slot holders. The driver kills no unit's process group,
  and a gate's command runs in the holder's own group, so a holder killed from outside frees its
  slot at once while the gate's test processes may run on. Those orphans are outside the cap, and
  they are what the out-of-slot sampler counts.
- **Time.** The driver has no unit or attempt timeout: an attempt ends only when its child exits.
  `gate_timeout_seconds` starts once a slot is taken, so a wait can neither kill a unit nor spend an
  attempt. The run's wall-clock cap counts the wait, since it is real time. The wait is measured on
  its own, and the run state records unit time (`duration_seconds`), gate time and wait time as
  separate numbers. The waiting child writes its times to a file under the run directory
  (`gate-<seq>.json` for a unit, `gate-main-<id>.json` for a main-branch close): `waiting_since`,
  then `waited_seconds` and the gate's own seconds, each write atomic. The driver copies a unit's
  file onto its row (`gate_seconds`, `wait_seconds`) when the unit exits, and a main gate's onto
  the run's `main_gates` rows, whose waits the run totals sum and keep the longest of.
- **Signals.** A signal during the wait makes the unit exit cleanly without asking to close, and
  without holding a slot.
- **Fairness.** Slots are polled, not queued fairly. This is accepted and documented, and the
  longest wait is measured.
- **`tp gate`.** It runs the resolved `quality_gate` from the project directory of the current
  checkout. In a unit's worktree that is the worktree, the same directory rule as the unit's
  `tp done` gate, never the main root. Inside a run it holds a slot; outside one it runs at once. It
  prints the gate's result and closes nothing.
- **Gating twice is avoided.** When `tp gate` passes on a clean tree inside a run, it writes a pass
  record, `gate-pass-<seq>.json` under the run directory, holding the tree and the gate command. A
  following `tp done` in the unit's worktree on the same tree, with the same gate command and a
  clean worktree, does not run the gate again; it still records the gated tree in its pending close,
  so v1.21.0's main-branch reuse applies after it. Besides its wait accounting, the pass record is
  the one thing `tp gate` writes. The measurement reports full gates per unit.

## What must be shown

- A test: with the cap at 2, never more than 2 gates run at once, the main gate included.
- A test: a reused main gate takes no slot.
- A test: a child killed while it holds a slot frees the slot at once.
- A test: a wait longer than `gate_timeout_seconds` neither kills the unit nor spends an attempt.
- A test: with every slot full and the main gate waiting, the first slot to free goes to the main
  gate, not to a waiting unit gate.
- A test: when the process holding the "main waiting" lock is killed, a waiting unit gate takes a
  slot.
- A test: a unit that gets a signal while waiting exits without asking to close and without holding
  a slot.
- A test: `tp gate` in a worktree gates the worktree's code.
- A test: `tp gate` and then `tp done` on the same clean tree run one full gate, not two.
- **Measurement.** The expected numbers are written before the run, on rolling's fixture and against
  rolling's peak-gates baseline. At cap 2 against unlimited:
  - the peak of concurrent gates;
  - the wait, in total and at its longest;
  - the difference in wall time.

  The first 8-slot run of the found-tasks implement phase also reports the test processes outside
  the slots, by a method fixed before the run:
  - a sampler records the running test binaries at a fixed interval;
  - it leaves out the child processes of gates that hold a slot;
  - it reports the peak and the median, and records the load average beside them;
  - it writes each sample to disk as it is taken, not at the end.

  Since the run goes straight to 8, the sampler is the only early warning. If the run crashes, its
  samples are the evidence.

## Non-goals

Fencing an agent's own `go test` calls; changing tp's `quality_gate`; fair queuing.
