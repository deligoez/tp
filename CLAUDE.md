# tp — Task Plan

Spec-to-task lifecycle manager for AI coding agents. Go CLI tool.

## Four Foundational Principles

| # | Principle | Definition |
|---|-----------|------------|
| P1 | AX First | Every decision optimizes for the AI agent |
| P2 | Batch Parity | What's easy for 1 task must be equally easy for N tasks |
| P3 | Minimal Tokens | Every output byte costs agent context |
| P4 | Agent Plans, Tool Executes | Agent produces decisions, tool deterministically executes |

**Always evaluate a change through the AX lens: does this reduce token overhead or round-trips for
the agent?** What tp is, how it installs, and what AX buys the reader are `README.md`'s job.

## Quick Reference

```bash
# Build
go build ./cmd/tp

# Test
go test ./...

# Lint
golangci-lint run

# Quality gate (the project gate in .tp/config.json; run after every change)
./scripts/check-suite-state.sh && golangci-lint run && ./scripts/check-deadcode.sh && ./scripts/check-complexity.sh
# step 1 wraps `go test -count=1 -race ./...` in a before/after hash of spec/.tp-review/ and .tp/rounds/

# Round artifacts no recorded round owns (dry run; --apply removes)
./scripts/clean-emissions.sh

# Stripped binary (production, <10MB)
go build -ldflags="-s -w" -o tp ./cmd/tp
```

## Commands

The command index is in `README.md`; the exact form of every command and flag, plus the workflows
that use them, is the inventory in `skills/tp/SKILL.md`. Do not restate either here — a fourth copy
is a fourth thing to drift.

## Project Structure

```
cmd/tp/               Main entry point
internal/cli/         Cobra commands, one file per command or mode (review_*.go, audit_*.go, run_*.go, ...)
internal/engine/      Core logic (toposort, closure, validate, lint, discover, lock, excerpt, config resolve, ...)
internal/model/       Data types (TaskFile, Task, Workflow, Coverage)
internal/output/      Formatting (JSON/TTY, compact, colors, hint errors)
internal/fakerunner/  The scripted runner child `tp run`'s tests spawn
spec/                 Specs, one file per version/feature; spec/backlog/ holds decision candidates
skills/tp/            SKILL.md (workflows, command inventory) + REFERENCE.md (fields, exit codes, schemas)
.claude-plugin/       marketplace.json + plugin.json (the plugin manifest)
hooks/, agents/       The plugin's Claude Code hooks and agents
scripts/              Gate steps, complexity baselines, round-prep and registered check scripts
.tp/                  Project workflow config, the reviewer/auditor role corpus, routed classes
docs/                 lessons.md (history, read on demand) and qa.md (manual QA)
```

## How work enters

Triage every piece of work before touching it; the operator confirms the track.

- **Fix** — a defect with a reproduction (an entry in `BUGS.md`, a field report, a red test). Write
  the failing test from the reproduction and watch it fail for the stated reason; fix; run the gate;
  one review of the diff; commit with `hc`. No spec, no ground, no review rounds.
- **Decision** — a choice between alternatives with user-visible consequences. A ≤2-page note
  (decision, alternatives, non-goals), the change built in a clone first, 2 reviewer roles, at most
  2 rounds. Findings about detail the code will settle are deferred to task acceptance.
- **Full** — a new capability too large to build in a clone first. The tp loop, with
  `review_max_rounds`/`audit_max_rounds` set at init and the growth check on.

A fix that turns out to need a design choice stops and becomes a decision. Repairs to a spec after
its first round delete or narrow; they do not add requirements.

Open work is `BUGS.md` (the fix track) and the two decision candidates in `spec/backlog/` —
`checklist-covers-what-changed.md`, then `reconcile.md`; `spec/backlog/ARCHIVE.md` holds the rest.

## Rules

1. **Run the gate after every change**, by hand when no `tp done` runs it. Step 3 needs `deadcode`
   and step 4 `gocognit` plus `golangci-lint` on PATH (`go install golang.org/x/tools/cmd/deadcode@latest`,
   `go install github.com/uudashr/gocognit/cmd/gocognit@latest`). The gate and CI are the same list
   (`TestCIRunsEveryStepOfTheProjectGate`). When step 1 reports round state changed, fix the test
   with `relocatedSpec`, never `--no-state`.
2. **Complexity is a ratchet.** `scripts/check-complexity.sh` fails on a new `gocognit` or `funlen`
   violation and equally on a stale entry in `scripts/baseline-complexity.txt` or
   `scripts/baseline-funlen.txt`, so the lists only shrink.
3. **Write the test first and watch it fail** for the reason you expect; a test never seen red
   proves nothing.
4. **Guard a dangerous value at the sink, not at the entry point** — `engine.SafeGitRev` wherever a
   caller-supplied string becomes a git argument.
5. **Never assert a quantifier you did not count.** Bind a claim to a named artifact, or let a test
   enforce it; prefer either to prose, and prose to a number.
6. **Commit with `hc`, never raw `git commit`.** Task-closing commits follow `commit_strategy`: under
   `hc` (tp's own, since `hc` is installed here) commit first, then `tp done <id> "evidence" --commit <sha>`.
7. **English in every committed artifact** — commit messages, closure reasons, code comments, docs,
   release notes, and recorded round rows including `class` slugs (SKILL.md, "Recording language").
8. **Dogfood the freshly built binary** — `go build -o /tmp/tp-dev/tp ./cmd/tp`, rebuilt after every
   implementing commit, for every tp call including review and audit; never the lagging PATH release.
9. **Operator decisions are never the agent's own**: `--skip-gate`, `tp import --force`, raising
   `review_max_rounds`/`audit_max_rounds` (default 3; an explicit 0 is uncapped) or any `run_max_*`,
   `audit_converge_on: blocking`, changing `quality_gate`, accepting a critical/high review finding
   or any audit finding without a change (`accept-finding`), and `--force` over an unrecorded
   emission (`discard-emission`). Under `tp run`
   each exits 2; the unit records it with `tp escalate --decision <name>` (the closed set is
   `engine.EscalationDecisions`) and the run stops for the operator. Policy: SKILL.md's "Gate, Budget
   & Escalation Policy".
10. **Measurement traps**: from a non-TTY child use `git -c diff.external= diff --no-ext-diff`;
    `tp review`/`tp audit` write round state even without `--record`, so probe in a `git clone`,
    never the repo; confirm a zero-match search with one that must match.
11. **Mutation testing** (`gremlins`) runs before a release only, in a fresh `rsync` copy, with
    `--workers` pinned and never `--test-cpu`; only the first run in a directory counts.
12. **Plugin content**: any change under `skills/`, `agents/`, `hooks/` or `.claude-plugin/` after a
    tag needs a `plugin.json` version bump, and the guard reads committed history — run it after
    committing.
13. **An audit ships on spec conformance**: `spec-coverage` clean two rounds running
    (`spec_coverage_clean_rounds`) and no FAIL in any round; record out-of-surface findings with
    justification and ship, never over an open spec-scoped finding (SKILL.md, Workflow D).

## Release

Before `gh release create`:

1. `skills/tp/SKILL.md`, `README.md` and `CLAUDE.md` reflect every new command, flag, lint rule and
   workflow change, and all three are committed in the tag.
2. Bump `.claude-plugin/plugin.json`'s `version` before tagging, every release (and on any plugin
   content change after a tag, rule 12). The field pins what `claude plugin update` offers, and
   `hooks/session-start.sh` reads it as the minimum tp version, so the release ships a binary at or
   above it. Order: bump → commit → tag (`TestPluginVersionIsNotBehindTheLatestTag`).
3. Dogfood any migration the release introduces on tp's own repo, and commit the result.
4. `go fix ./...` (check `git status`, not its summary line), `govulncheck ./...`, and the mutation
   run of rule 11.

After `gh release create`, in order:

```bash
go install github.com/deligoez/tp/cmd/tp@v<VERSION>
claude plugin update tp@tp   # Claude Code users: the plugin carries the hooks and agents
npx skills update -g         # other agents: updates every installed global skill source
```

Use the exact version tag, not `@latest`. Then verify by running: `claude plugin details tp@tp` must
report the new version and the full inventory (Skills 1 / Agents 3 / Hooks 3) — `claude plugin
validate` passing is not evidence that the plugin loads.

## Tech Stack

- **Language:** Go
- **CLI:** spf13/cobra
- **Colors:** fatih/color (NO_COLOR + TTY detection built-in)
- **File locking:** gofrs/flock
- **Testing:** stretchr/testify
- **JSON:** encoding/json (stdlib)
- **Validation:** Manual struct validation (no JSON Schema library)

## Conventions

tp's own behavior — exit codes, managed fields, workflow field ranges, `--compact` semantics,
acceptance delimiters, field aliases, discovery order — is documented once, in
`skills/tp/SKILL.md` and `skills/tp/REFERENCE.md`; the lint rule table lives in `README.md`. What
follows is only what is a convention of *this codebase*:

- JSON output when piped or `--json`, colored text in TTY
- Pretty-printed JSON with 2-space indentation
- All write operations use flock; reads are lock-free
- Task status is stored as `open -> wip -> done` (three states only; blocked is computed from deps)
- `unused`, `unparam`, `gocritic` and `dupl` are enabled, so an orphaned symbol, an unused parameter, a range that copies a large struct, or a clone at or above `dupl`'s default 150-token threshold each fail the gate

## Pointers

- `BUGS.md` — reproduced defects, the fix track's queue.
- `spec/backlog/` — decision candidates only.
- `docs/lessons.md` — the measurements and history this file used to carry. Read on demand; nothing
  in it is a rule.
- `docs/qa.md` — the manual QA recipe and checklist.
