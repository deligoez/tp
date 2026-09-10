# tp — Repair locality (not a release)

A backlog note, not a spec: `spec/backlog/README.md` lists it under *Not releases*. The script's source
and every measurement are in `repair-locality-measurements.md` beside it, and this file stands without
them. `12-repair-locality.md` forwards here, because a shipped spec cites that path.

## 1. The question

A repair round rewrites part of the spec; the next round reviews it and files findings. **How much of
what a round finds sits in text the round before it just wrote?** The diagnosis has been reached by
hand, and `CLAUDE.md` still carries one such reading, with no rule a reader could re-derive it by. The
measurement answers it from two recorded snapshots and one round file.

## 2. The decision: a script, not a release

**It becomes a script under `scripts/`, run by hand.** Its source is in the sidecar under *The
script* until someone commits it there. It does not become tp code, for three reasons.

- **It saturates on the cycles that matter now.** Every review cycle recorded from v1.0.0 on — the
  Step 0.5-sized specs — lands in the same quadrant of §3: nearly every finding in changed text, most
  sections changed, concentration close to one. A number that reads the same on every current cycle
  separates nothing (sidecar, *Re-verified 2026-09-11*).
- **It gates nothing, and no `next_action` reads it.** A status key no driver branches on is output
  every agent pays for and none acts on.
- **Its one shared input is derived where it is acted on.** Which sections changed between two
  snapshots is also what other pending work would read; it is derived once, in whichever release first
  acts on it, rather than by a release that only reports it.

`spec/undecided.md`'s *A prior-round section for `tp review`* names this script as the measurement
that would reopen it; nothing else waits on it.

## 3. Reading the two numbers

**One number ranks the cycles backwards.** The share of findings in changed text, alone, calls a
cycle whose repairs rewrote most of the spec the most repair-local; **concentration** — that share
divided by the share of sections changed — calls the same cycle the least. On the three cycles the
script was first run on, the share puts v0.37.0 first and concentration puts it last, while v0.36.0
put most of its findings into a small part of the file. So the script prints both and their ratio,
never one.

| | high concentration | low concentration |
|---|---|---|
| **high share** | the loop is chasing its own repairs in a small area | the repairs are rewriting most of the spec each round |
| **low share** | healthy — findings are spread over text the loop did not just write | — |

v0.36.0 is the top-left and v0.37.0 the top-right; every cycle from v1.0.0 on is top-right too.

**The number is a prompt to read what the last repair wrote, not a verdict on the document.** A cycle
in the top-right is not thereby failing: what it says is that the round's ask had been almost entirely
replaced by the round before it.

## 4. What the script decides

The counting rules, stated because a figure whose rule is unstated is §1's complaint:

1. A section is any heading of level two or deeper whose text opens with a section id, at any depth.
2. A section is changed when the line diff from snapshot N−1 to snapshot N attributes a changed line to
   it in the new snapshot; the denominator is the new snapshot's section count.
3. A finding is inside when the section id parsed from its `location` equals a changed id or descends
   from one — downward only, and on the separator, so `§1` does not swallow `§10`.
4. A pair is skipped when either snapshot's hash differs from the hash its round recorded; a round with
   no predecessor, no snapshot or an empty round file reports nothing, since a first round would
   otherwise score zero, which reads as healthy.
5. Review rounds only: audit snapshot pairs are mostly byte-identical and many audit rows carry no
   `location` (sidecar, *Why the audit phase is out*).
6. A cycle's figures are medians over its rounds, the pooled figure is the median over every round
   rather than a median of medians, and the ratio divides the two unrounded medians.

**What the committed script should add**, both decided while this was a release: a `location` that is
a bare heading number (`4.2.3`) parses like one with a `§`; and the raw counts — findings inside,
findings located, sections changed, sections total — print beside the percentages. The sidecar's
version parses the `§` form only and prints the unparseable count alone.

**`spec/1.0.0.md` §8 cites *the repair-locality spec §1.1* for this table and its derivation.** Both
are in the sidecar under *The script*; the earlier form of the table that spec quotes is explained
under *Two earlier readings of the table*.
