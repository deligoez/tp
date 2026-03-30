# tp — Task Plan

Spec-to-task lifecycle manager for AI coding agents. Go CLI tool.

## Key Concept: AX (Agent Experience)

AX = Agent Experience (analogous to DX = Developer Experience).
Every design decision in tp optimizes for the AI agent as the primary user:
- Minimize round-trips (compound commands: `tp next`, `tp done`)
- Minimize output tokens (concise JSON, `--compact` flag)
- Clear actionable errors ("cannot claim X: blocked by Y. Run `tp show Y`")
- Deterministic behavior (no ambiguity, no prompts)
- Fast startup (<50ms per call)

**Always evaluate changes through the AX lens: does this reduce the number of tp calls an agent needs per task?**

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
```

## Project Structure

```
cmd/tp/           Main entry point
internal/
  cli/            Cobra commands, exit codes, root command
  engine/         Core logic: lint, validation, closure, parallelism, discovery, locking
  model/          Data types: TaskFile, Task, Workflow, Coverage
  output/         Output formatting (JSON/TTY, colors, quiet mode)
docs/
  spec.md         Source specification
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
- All write operations use flock; reads are lock-free
- Task status: open → wip → done (3 states only)
- Pretty-printed JSON with 2-space indentation
