# Candidate: epic review at scale

> A decision candidate, not a spec. Source: a field sweep of `tp epic review` on v1.6.2 by the
> requesting project — 18 milestone slices × 3 roles (54 units plus re-runs), 203 round-1 findings,
> all dispositioned, 243 graph writes. No crash and no wrong verdict. Shape only; no project data.

Every item is friction or a gap the epic layer meets once a graph is reviewed slice by slice in
bulk. None is a reproduced defect with a wrong result, so none is fix-track; each needs a choice.

## Friction

1. **The role prompt's output path is cwd-relative.** `epic-review-<hash>-r<N>-<role>.ndjson`
   resolves against the unit's cwd (usually the repo root) and leaves untracked files there. Under
   `tp run`, `TP_ROUND_DIR` already makes it absolute (`epicRoleOutputPath`); spec review and audit
   share the relative form by design. Options: an absolute path under the slice state dir, or a
   `--findings-dir` flag.
2. **`-o /dev/stdout` on `tp review --merge` fails** with `open /dev/stdout.tp-merge-NNN: operation
   not permitted`: the atomic write creates its temporary beside the target (`merge_inputs.go`,
   accepted consequences listed there). Options: `-o -` for stdout, or a hint naming the cause.
3. **A class that recurs across slices is invisible.** One class appeared in 6 of 18 slices;
   `mechanize_candidates` counts per slice, so it was never suggested.
   - (a) a cross-slice report, e.g. `tp epic review --report` over every slice directory;
   - (b) the class was "no DIRECT edge to the provider" while every offender reached it
     transitively. `tag_checks.provided_by` accepts a transitive dependency
     (`lintEpicTagChecks` → `transitiveDeps`), so the class cannot be mechanised today; it would
     need a `direct: true` option on a tag check.
4. **The phase-economist role's prompt is 2.5–3× the other two roles on large slices** (on a
   66-epic slice 229 KB against 82 KB; equal on small slices). Unmeasured here: what context it adds.
   A cap or a summary mode; 229 KB is near the read ceilings agents have.
5. **No batch emission or status across slices.** Emit, merge, record and status ran in shell loops
   over 18 slices. Ask: `tp epic review --milestones a,b,…` / `--all`, and one `--status` table over
   every slice.

## Gaps

6. **Mixed graph edits are not atomic.** A set-milestone refused as `later-milestone-dependency`
   because the unlink that would make it legal came later in the same plan; only `add --bulk` lints
   the final state. Ask: `tp epic apply <ndjson>` with link, unlink, set, gate and add rows, linted
   once on the after-state, all or nothing.
7. **No any-of dependency.** A capability that may start when either of two conditions holds can
   only be written as a third condition epic whose acceptance says "A or B", leaving the OR to the
   operator. An `any_of` edge or condition kind.
8. **A stale slice past its cap has only a global fix.** A slice at `review_max_rounds` whose graph
   then changed is stale, and `next_action`'s only route is raising the project-wide cap, which
   raises it for every other slice too. A per-slice verification allowance, or a cap keyed per slice.

## What worked

17 units self-emitting with `tp epic review --milestone X --role R` ran concurrently across 6 slices;
emission stayed idempotent and nothing collided.
