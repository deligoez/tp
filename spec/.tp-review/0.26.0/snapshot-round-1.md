# tp v0.26.0 — Presence-preserving task-file workflow (no default materialization)

## 1. Overview

The project-config feature (v0.24.0) resolves workflow parameters at read time in a strict precedence: CLI flag > `TP_FILE` env > task-file `workflow` override > `.tp/config.json` project default > built-in default. A task file's `workflow` block is meant to hold **only the fields a project explicitly overrides**; every absent field must fall through to the project config or the built-in default.

That contract is broken by **default materialization**. `tp init` writes the built-in defaults into the new task file's `workflow` block, and every subsequent read-modify-write re-materializes them, so the block always contains `review_clean_rounds`, `audit_clean_rounds`, `gate_timeout_seconds`, `checks`, `review_max_rounds`, and `audit_max_rounds` with their default values — even when the project never set them. Because the resolver treats a present field as an explicit override, those materialized defaults **mask `.tp/config.json`**: a project that sets `review_clean_rounds: 3` in its shared config still resolves to `2` for every spec that went through `tp init`.

v0.26.0 makes the task-file `workflow` block **presence-preserving**: it contains only explicitly-set fields, absent fields stay absent across the full read-modify-write lifecycle, and resolution therefore works as designed.

## 2. Design Principles

1. **Resolve, never materialize.** A task file records overrides, not effective values. Defaults live in exactly one place (the built-in defaults) and are applied at read time, never frozen into a task file.
2. **Round-trip fidelity.** Reading a task file and writing it back (via any command) must not add or remove a `workflow` field as a side effect. Absence is data.
3. **No silent behavior change for existing overrides.** A field a project genuinely set keeps its value; only unset (default-materialized) fields stop being written.
4. **Minimal surface.** No new command, flag, or config knob — this is a correctness fix to serialization and `tp init`.

## 3. The invariant

3.1 A task file's `workflow` block contains a field **if and only if** it was explicitly set — by `tp init --quality-gate`/`--commit-strategy`, by `tp set --workflow <field>=<value>`, or by an authored/imported task file. A never-set field is absent from the JSON.

3.2 **Round-trip preservation.** For any task-file-writing command (`tp done`, `tp close`, `tp set`, `tp reopen`, `tp remove`, `tp add`, `tp claim`, …), reading a task file and writing it back leaves the set of present `workflow` fields unchanged. In particular, a `"workflow": {}` block stays `{}`; a block with only `review_max_rounds: 4` keeps exactly that one field.

3.3 **`tp init` writes a sparse block.** `tp init <spec>` with no workflow flags writes `"workflow": {}`. `--quality-gate "<cmd>"` writes `{"quality_gate": "<cmd>"}` and nothing else; `--commit-strategy` likewise. No convergence/timeout/checks field is written unless a flag sets it.

3.4 **Resolution unchanged and now effective.** Effective values still resolve at read time (CLI > env > task-override > project config > built-in). A task file with `"workflow": {}` inherits every field from `.tp/config.json` (when present) or the built-in default. `tp config --resolved` attributes each field to the layer that actually supplied it, so a field only shows `source: "task"` when the task file explicitly set it.

3.5 **Explicit zero is still explicit.** A field a project deliberately set to a boundary value (e.g. `review_max_rounds: 0` = no cap) remains present and wins over the project layer. Presence — not value — distinguishes an override from an absence (§3.1). Invalid explicit values (e.g. `review_clean_rounds: 0`, outside 1–10) remain a validation error, not a silent fallback.

## 4. Presence-preserving serialization

4.1 The task-file `workflow` is stored and round-tripped as a single **presence-tracked override** — the same absence-vs-explicit-zero representation the v0.24.0 resolver already uses (`WorkflowOverride`, optional/pointer fields with `omitempty`). It is never stored as a defaults-applied `Workflow` value that writes every field. There is one canonical task-file representation of the workflow, and it records only overrides. Applying built-in defaults on unmarshal (which erases which fields were present) is removed from the storage path.

4.2 Effective values — what commands act on (the resolved quality gate, clean-round counts, timeout, caps) — come **only** from the resolution path (`ResolveWorkflow` / `EffectiveWorkflowForTaskFile`), which layers CLI > env > task-override > project config > built-in. No command reads an effective workflow value off a defaults-applied struct. Reading a task file for the purpose of writing it back MUST NOT inject default values into the serialized block.

4.3 The `checks` array is written only when non-empty or explicitly set; an unset `checks` is absent, not `[]`, so it inherits the project config's `checks` (which uses replace semantics).

## 5. `tp init`

5.1 `tp init` stops writing the built-in convergence/timeout/checks defaults into the shell. The created `<base>.tasks.json` has `"workflow": {}` unless a workflow flag was passed (§3.3).

5.2 The init shell still holds zero tasks and the empty coverage block, unchanged. Only the `workflow` block becomes sparse.

## 6. `tp set --workflow`

6.1 `tp set --workflow <field>=<value>` adds or updates exactly that field in the task file's `workflow` block, leaving every other field's presence untouched (it never materializes siblings). Unsetting is out of scope for this release (§9).

6.2 `tp set --workflow --project <field>=<value>` continues to edit `.tp/config.json` and is unaffected.

## 7. Existing task files

7.1 Backward compatibility is not a design constraint for this release; the goal is the cleanest correct model. Existing task files with a materialized `workflow` block still parse — their present fields read as explicit overrides (they always did) — and nothing rewrites or thins them automatically. A project that wants to stop masking its `.tp/config.json` thins its task files deliberately; `tp config --extract` (v0.24.0) still hoists shared policy into `.tp/config.json` and is unaffected.

## 8. Migration (tp's own repo)

8.1 tp dogfoods the fix: `spec/0.25.0.tasks.json` currently carries a materialized `workflow` block. As part of this release it is thinned to `"workflow": {}` (its values equal the project config, so thinning changes no effective value), demonstrating the fixed round-trip on tp's own repo.

## 9. Non-Goals

1. No `tp set --workflow --unset <field>` to remove a field — presence is only added in this release; removal is a separate feature.
2. No change to the resolution precedence or to `.tp/config.json`'s schema.
3. No new command or flag.
4. No automatic thinning of existing task files (§7.1) — migration stays a deliberate, user-invoked step.
5. No change to how the built-in defaults themselves are defined (still one source of truth in code).

## 10. Tests / Acceptance

10.1 `tp init <spec>` with no workflow flags produces `"workflow": {}`; with `--quality-gate "<cmd>"` produces exactly `{"quality_gate": "<cmd>"}`.

10.2 **Round-trip:** a task file with `"workflow": {}` survives `tp add`, `tp claim`, `tp done`, `tp set` (a non-workflow field), and `tp reopen` with its `workflow` block still `{}`. A task file with only `{"review_max_rounds": 4}` keeps exactly that one field across the same operations.

10.3 **No masking:** with `.tp/config.json` `{"workflow": {"review_clean_rounds": 3}}` and a `tp init`-created task file, `tp config --resolved` reports `review_clean_rounds: {value: 3, source: "project"}` (not `2`/`task`), and the review loop requires 3 clean rounds.

10.4 **Explicit override still wins:** a task file that sets `review_clean_rounds: 5` resolves to 5 over a project config's 3; an invalid explicit `review_clean_rounds: 0` is a validation error.

10.5 `tp set --workflow review_max_rounds=4` on a `{}` block yields `{"review_max_rounds": 4}` and does not add any sibling field.
