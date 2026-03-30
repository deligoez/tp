# tp — Task Plan

Spec-to-task lifecycle manager for AI coding agents. Go CLI tool.

## Key Concept: AX (Agent Experience)

AX = Agent Experience (analogous to DX = Developer Experience).
Every design decision in tp optimizes for the AI agent as the primary user:
- Minimize round-trips (2-call architecture: `tp plan` + `tp done --batch`)
- Minimize output tokens (concise JSON, `--compact` flag strips ~40%)
- Clear actionable errors with `hint` field ("cannot claim X: blocked by Y")
- Deterministic behavior (no ambiguity, no prompts)
- Fast startup (<50ms per call)

**Always evaluate changes through the AX lens: does this reduce token overhead for the agent?**

## Four Foundational Principles

| # | Principle | Definition |
|---|-----------|------------|
| P1 | AX First | Every decision optimizes for the AI agent |
| P2 | Batch Parity | What's easy for 1 task must be equally easy for N tasks |
| P3 | Minimal Tokens | Every output byte costs agent context |
| P4 | Agent Plans, Tool Executes | Agent produces decisions, tool deterministically executes |

## Quick Reference

```bash
# Build
go build ./cmd/tp

# Test
go test ./...

# Lint
golangci-lint run

# Quality gate (run after every task)
go test ./... && golangci-lint run

# Stripped binary (production, <5MB)
go build -ldflags="-s -w" -o tp ./cmd/tp
```

## Commands

### Primary (2-call architecture)
```bash
tp plan                        # Full execution plan in one call (THE primary command)
tp done <id> <reason>          # Close task with implicit claim + verification
tp done --batch results.ndjson # Batch close from NDJSON (primary close mechanism)
```

### Incremental (fallback)
```bash
tp next                        # Resume WIP or claim next ready task
tp next --peek                 # Preview without claiming
```

### Task State
```bash
tp claim <id> [id...]          # open -> wip (batch: multiple IDs, --all-ready)
tp close <id> <reason>         # wip -> done (low-level, prefer tp done)
tp reopen <id>                 # done -> open (clears gate_passed_at, commit_sha)
tp remove <id>                 # Remove task (--force for dep cleanup)
tp set <id> field=value        # Update field (managed fields protected)
```

### Query
```bash
tp list                        # All tasks (--status, --tag, --ids, --compact)
tp ready                       # Deps-satisfied tasks (--first, --count, --ids)
tp show <id>                   # Full details + spec_excerpt + blocks
tp status                      # Progress summary
tp blocked                     # Tasks waiting on deps
tp graph                       # Dependency tree (--tag, --from)
tp stats                       # Parallelism analysis
```

### Spec & Validation
```bash
tp lint <spec.md>              # Spec quality checks (no task file needed)
tp validate                    # Task file validation (--strict)
```

### Data
```bash
tp init <spec.md>              # Create empty task file
tp add <json>                  # Add task (--stdin, --file for NDJSON bulk)
tp import <file>               # Import + validate (--force to overwrite)
```

### Global Flags
```
--file <path>    Explicit task file
--json           Force JSON output
--compact        Minimal JSON (~40% smaller)
--quiet          Suppress info messages
--no-color       Disable colors
```

## Project Structure

```
cmd/tp/              Main entry point
internal/
  cli/               Cobra commands (plan, done, next, list, claim, close, ...)
  engine/            Core logic (toposort, closure, validate, lint, parallel, discover, lock, excerpt)
  model/             Data types (TaskFile, Task, Workflow, Coverage)
  output/            Formatting (JSON/TTY, compact, colors, hint errors)
docs/
  spec.md            Source specification (1431 lines)
skills/tp/
  SKILL.md           Claude Code skill (3 workflows, 2-call pattern)
.claude-plugin/
  plugin.json        Skill distribution manifest
```

## Tech Stack

- **Language:** Go
- **CLI:** spf13/cobra
- **Colors:** fatih/color (NO_COLOR + TTY detection built-in)
- **File locking:** gofrs/flock
- **Testing:** stretchr/testify
- **JSON:** encoding/json (stdlib)
- **Validation:** Manual struct validation (no JSON Schema library)

## Conventions

- Exit codes: 0=success, 1=validation, 2=usage, 3=file, 4=state
- JSON output when piped or `--json`, colored text in TTY
- `--compact` omits: description, source_sections, source_lines, tags, closed_reason, spec_excerpt
- All write operations use flock; reads are lock-free
- Task status: open -> wip -> done (3 states only, blocked computed from deps)
- Managed fields (tp set rejects): status, closed_at, closed_reason, gate_passed_at, commit_sha
- Pretty-printed JSON with 2-space indentation
- spec_excerpt capped at 2000 chars
