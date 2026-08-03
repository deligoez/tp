# tp v0.31.1 — Close the v0.31.0 gaps

## 1. Overview

v0.31.0 added the `review_converge_on` workflow field and relied on `golangci-lint run` as the recommended Go quality gate. Two gaps surfaced during the v0.31.0 audit:

- The quality gate has a **gofmt blind spot**: golangci-lint v2 does not run formatters unless a `formatters:` section enables them, so `golangci-lint run` reports "0 issues" on a gofmt-dirty file. tp's own gate missed a real gofmt violation this way, and every user who copied tp's recommended gate inherits the same blind spot.
- `review_converge_on` is fully wired through resolution, `tp config --resolved`, and `tp set`, but is **missing from the two cross-task-file enumerations**: `tp config --extract` (hoisting) and `tp validate --project` (drift). An identical value is not hoisted, and a divergent value is not reported as drift.

This release closes both gaps. It is a patch: no new commands, no new flags, no behavior change to any converged v0.31.0 path.

## 2. Quality gate: close the gofmt blind spot

### 2.1 Enable a formatter in the repo lint config

tp's own `.golangci.yml` gains a `formatters:` section that enables `gofmt`, so `golangci-lint run` — the command tp's project `quality_gate` invokes — reports a gofmt-dirty file as a `gofmt` issue and exits non-zero. Verified empirically: without the section a gofmt-dirty file yields a zero-issue, exit-zero run; with `formatters: enable: [gofmt]` the same file is reported as one `gofmt` formatting issue and the run exits non-zero. The existing linters and their settings are unchanged.

### 2.2 Fix the recommended-gate guidance in the docs

Every doc that recommends `go test ./... && golangci-lint run` as the Go quality gate (`CLAUDE.md`, `README.md`, `skills/tp/SKILL.md`) notes that golangci-lint v2 runs formatters only when a `formatters:` section enables them, so a gofmt check must be enabled (or `gofmt -l` added to the gate) for the gate to catch formatting. The guidance change is advisory text only; tp does not change the default gate string that `tp init --quality-gate` authors (the gate command stays user-authored).

## 3. Hoist `review_converge_on` in `tp config --extract`

`review_converge_on` is a per-task-settable string field (`tp set --workflow review_converge_on=...`), so when every task file sets it to the same value `tp config --extract` must hoist it into `.tp/config.json` like the other common fields, then thin it out of the task files.

### 3.1 computeCommonPolicy detects a common review_converge_on

`computeCommonPolicy` treats `review_converge_on` as common exactly when every override sets it and all values are equal, using the same all-equal-string logic already applied to `quality_gate`. When the values differ, or any task file leaves it unset, it is not common.

### 3.2 hoistedFields lists review_converge_on

`hoistedFields` appends `"review_converge_on"` to the deterministic field list when the common policy sets it, so the field name appears in the `hoisted` output array and is passed to `StripTaskWorkflowFields`.

### 3.3 mergeCommon writes review_converge_on

`mergeCommon` copies a non-nil common `review_converge_on` into the destination project override, preserving every other hand-set project field, so the hoisted value lands in `.tp/config.json`.

## 4. Report `review_converge_on` drift in `tp validate --project`

### 4.1 workflowDeviations compares review_converge_on

`workflowDeviations` reports a deviation when a task file's `review_converge_on` override and the project's `review_converge_on` are both set and differ, using the same both-set string comparison already applied to `quality_gate`. A field the project does not set carries no policy and is not a deviation. Under `--strict` a `review_converge_on` deviation is a validation error (exit 1), consistent with every other drift field.

## 5. Non-Goals

1. **`commit_strategy` in extract/validate.** `commit_strategy` is read-only per task (settable only by `tp init` authoring or `tp set --workflow --project`), so hoisting per-task overrides for it and reporting its drift are intentionally out of scope; its absence from both enumerations is by design, not a gap this release closes.
2. **Data-driven workflow-field set.** Making the workflow-field enumerations iterate a single source of truth (so a new field can never be forgotten in one site) is a larger refactor deferred to a future minor release; this patch adds `review_converge_on` to the two sites by hand, matching the existing per-field style.
3. **Changing the default gate command.** tp does not alter the gate string `tp init --quality-gate` writes, nor force a formatter into a user's gate; §2.2 is guidance only.
4. **gofumpt.** §2.1 enables `gofmt`, the standard zero-config formatter matching the check the v0.31.0 audit used; adopting the stricter `gofumpt` is not part of this release.

## 6. Tests

1. `config --extract`: given task files that all set `review_converge_on` to the same value, the field is hoisted (appears in `hoisted`, written to the project config, thinned from task files); given divergent or partially-unset values, it is not hoisted.
2. `mergeCommon`: a common `review_converge_on` is written into the destination override while an unrelated hand-set project field is preserved.
3. `validate --project`: a task file whose `review_converge_on` differs from the project value produces a deviation; `--strict` makes it exit 1; an equal value (or a project that does not set it) produces no deviation.
4. The gofmt gate change (§2.1) is a repo lint-config change with no Go unit test for the formatter behavior itself; it is verified by `golangci-lint run` reporting a deliberately gofmt-dirty fixture.
5. A deterministic guard test reads the repo `.golangci.yml` and asserts its `formatters:` section enables `gofmt`, so silently dropping that section — which would re-open the exact blind spot §2.1 closes — fails a test rather than passing unnoticed.
