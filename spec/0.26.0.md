# tp v0.26.0 — Presence-preserving task-file workflow (no default materialization)

## 1. Overview

The project-config feature (v0.24.0) resolves workflow field values at read time in a strict precedence: task-file `workflow` override > `.tp/config.json` project default > built-in default. No CLI flag or environment variable supplies a workflow field value, so those tiers — which exist in tp's broader config model for flag defaults and task-file discovery — never appear as a workflow `source`; in particular `TP_FILE` only selects *which file* provides the task-override layer, it is not itself a value layer. A task file's `workflow` block is meant to hold **only the fields a project explicitly overrides**; every absent field must fall through to the project config or the built-in default.

That contract is broken by **default materialization**. `tp init` writes the built-in defaults into the new task file's `workflow` block, and every subsequent read-modify-write re-materializes them, so the block always contains `review_clean_rounds`, `audit_clean_rounds`, `gate_timeout_seconds`, `checks`, `review_max_rounds`, and `audit_max_rounds` with their default values — even when the project never set them. Because the resolver treats a present field as an explicit override, those materialized defaults **mask `.tp/config.json`**: a project that sets `review_clean_rounds: 3` in its shared config still resolves to `2` for every spec that went through `tp init`.

v0.26.0 makes the task-file `workflow` block **presence-preserving**: it contains only explicitly-set fields, absent fields stay absent across the full read-modify-write lifecycle, and resolution therefore works as designed.

## 2. Design Principles

1. **Resolve, never materialize.** A task file records overrides, not effective values. Defaults live in exactly one place (the built-in defaults) and are applied at read time, never frozen into a task file.
2. **Round-trip fidelity.** Reading a task file and writing it back (via any in-place writer) must not add or remove a `workflow` field as a side effect. Absence is data.
3. **No silent behavior change for existing overrides.** tp stops *materializing* — injecting — unset defaults into the blocks it writes; it never strips a field by value. Any field already present is preserved (presence, not value, is authoritative — §3.5/§7.1), so a genuinely-set field keeps its value and an existing materialized field is not silently removed. Only tp's own write path changes: it no longer injects defaults it was not told to set.
4. **Minimal surface, audited read path.** No new command, flag, or config knob — this is a correctness fix to serialization and `tp init`. It does reroute every effective-value read from the task-file struct to the resolution path; because the new storage type is the presence-tracked override — a type distinct from the resolved `Workflow` (§4.1) — any missed read site becomes a compile error rather than a silent regression, so the reroute is bounded and verifiable, not an open-ended bundle.

## 3. The invariant

3.1 **tp writes only what was set.** tp never writes a `workflow` field it was not explicitly told to set — by `tp init --quality-gate`/`--commit-strategy`, by `tp set --workflow <field>=<value>`, or by an authored/imported task file. Among the fields tp itself writes, a field is present **if and only if** it was explicitly set; a never-set field is absent from the JSON. (This describes tp's write behavior, not a property retro-applied to legacy files: a present field in a pre-existing or imported task file is always treated as an override regardless of value — see §7.1.)

3.2 **Round-trip preservation.** Reading a task file and writing it back **in place** through `WriteTaskFile` leaves the set of present `workflow` fields unchanged. This binds every in-place writer — currently `tp add`, `tp claim`, `tp next`, `tp set`, `tp done`, `tp close`, `tp reopen`, `tp remove`, and `tp commit` — with one exception: `tp set --workflow <field>=<value>` intentionally adds or updates exactly the one field named (§6.1). Two commands mutate a task file's `workflow` block **by design** and are outside this round-trip invariant: `tp import` (a source→target operation, §3.6) and `tp config --extract` (which *thins* task files, deliberately removing the fields it hoists into `.tp/config.json`, §7.1). A `"workflow": {}` block stays `{}`; a block with only `review_max_rounds: 4` keeps exactly that one field; a source file that arrives with no `workflow` key at all is normalized to `"workflow": {}` on the next write (§4.4), never populated with defaults.

3.3 **`tp init` writes a sparse block.** `tp init <spec>` with no workflow flags writes `"workflow": {}`. `--quality-gate "<cmd>"` writes `{"quality_gate": "<cmd>"}` and nothing else; `--commit-strategy "<value>"` writes `{"commit_strategy": "<value>"}` and nothing else. No convergence/timeout/checks field is written unless a flag sets it.

3.4 **Resolution unchanged and now effective.** Effective values still resolve at read time by the §1 precedence (task-override > project config > built-in). A task file with `"workflow": {}` inherits every field from `.tp/config.json` (when present) or the built-in default. `tp config --resolved` attributes each workflow field to the layer that supplied it via a `source` label — one of `"override"` (the task file set it), `"project"` (the `.tp/config.json` default), or `"default"` (the built-in) — the v0.24.0 label set, unchanged — so a field shows `source: "override"` only when the task file explicitly set it.

3.5 **Explicit zero is still explicit.** A field a project deliberately set to a boundary value (e.g. `review_max_rounds: 0` = no cap) remains present and wins over the project layer. Presence — not value — distinguishes an override from an absence (§3.1). An out-of-range explicit value (e.g. `review_clean_rounds: 0`, outside 1–10) entered through `tp set --workflow` is rejected at write time (§6.1, exit 1). The same value present in an authored or imported file is not rejected; it is preserved in the JSON but **clamped at resolution** — treated as unset, falling back to the built-in default, with a warning (§7.1). Presence, not validity, is what §3.1 preserves.

3.6 **`tp import` presence.** `tp import` reads a source file and writes the target task file (`--force` replacing any prior target). It preserves the *source's* `workflow`-field presence verbatim — no default materialization — and never inherits a pre-existing target's `workflow` block. A source with no `workflow` key produces `"workflow": {}` in the target (§4.4). Import is not a round-trip: "unchanged" is measured against the imported source, not the replaced target.

## 4. Presence-preserving serialization

4.1 **One canonical override type.** The task-file `workflow` is stored and round-tripped as a single **presence-tracked override** — the same absence-vs-explicit-zero representation the v0.24.0 resolver already uses (`WorkflowOverride`: optional pointer fields with `omitempty`). It is never stored as a defaults-applied `Workflow` value that writes every field. The canonical override represents **every** task-file workflow field, so none is dropped when the defaults-applied `Workflow` leaves the storage path: `quality_gate` and `commit_strategy` (presence-tracked `*string`), the five convergence/timeout integers (`review_clean_rounds`, `audit_clean_rounds`, `gate_timeout_seconds`, `review_max_rounds`, `audit_max_rounds`, each a `*int`), and `checks` (a `*[]Check`). Every field is presence-tracked by its optional pointer, so presence — not value — marks an override (§3.5) uniformly: an explicit empty-string `quality_gate` or `commit_strategy` (a present "no gate" / "no strategy" override), an explicit `review_max_rounds: 0` (no cap), and an explicit empty `checks: []` are each present and distinct from absence, and each survives round-trip; a nil pointer is absent and inherits the lower layer. Applying built-in defaults on unmarshal (which erases which fields were present) is removed from the storage path. Representing a field in the override type does not make it user-settable — the settable set is fixed separately (§6.1).

4.2 **Effective values come only from resolution.** What commands act on — the resolved quality gate, commit strategy, clean-round counts, timeout, caps — comes only from the resolution path (`ResolveWorkflow` / `EffectiveWorkflowForTaskFile`), which layers task-override > project config > built-in and returns a freshly resolved `Workflow`. No command reads an effective value off the **task-file storage struct** — the struct that, before this fix, baked defaults in at unmarshal time. The resolution path's own return value is a fully-populated `Workflow`; that resolved struct is the *only* sanctioned defaults-bearing value, and reading effective values off it is correct. The prohibition is specifically on reading effective values off *storage*. Because the storage type is now the presence-tracked override (pointer fields, a type distinct from `Workflow`), a direct effective read of a task-file struct no longer compiles — so the read-site reroute is compiler-enforced.

4.3 **`checks` presence.** `checks` is a presence-tracked pointer-to-slice. An **unset** `checks` (nil) is absent from the JSON and inherits the project config's `checks` (replace semantics). An **explicit empty** `checks: []` (present, non-nil, zero-length) is written as `[]` and overrides the project config to no checks — presence, not emptiness, distinguishes the two, exactly as for every other field (§3.5).

4.4 **The `workflow` container is always an object.** The top-level `workflow` key is always emitted as a JSON object; when the override carries no set fields it is written as `"workflow": {}`. This constant-cost structural anchor is deliberate and is **not** in tension with §4.3's field-level absence or §2's "absence is data": absence carries data **within** the block (which override fields are set), while the block itself is a fixed, deterministic container consistent with tp's already-thinned task files (`spec/0.24.0.tasks.json` and older carry `{}`). A source file with no `workflow` key is normalized to `{}` on write (§3.2); an empty block and an absent key resolve identically, so the normalization changes no effective value.

## 5. `tp init`

5.1 `tp init` stops writing the built-in convergence/timeout/checks defaults into the shell. The created `<base>.tasks.json` has `"workflow": {}` unless a workflow flag was passed (§3.3).

5.2 The init shell still holds zero tasks and the empty coverage block, unchanged. Only the `workflow` block becomes sparse.

## 6. `tp set --workflow`

6.1 `tp set --workflow <field>=<value>` adds or updates exactly that field in the task file's `workflow` block, leaving every other field's presence untouched (it never materializes siblings). The **settable set is unchanged from v0.24.0**: the five convergence/timeout integers (`review_clean_rounds`, `audit_clean_rounds`, `gate_timeout_seconds`, `review_max_rounds`, `audit_max_rounds`) and `checks` (a JSON-array value with replace semantics; setting it marks the field present per §4.3). `quality_gate` and `commit_strategy` are **not** settable here — they remain authored only by `tp init` (§3.3), unchanged by this release even though they are now presence-tracked in the override type (§4.1). Validation happens **before** the file is written: an integer outside its range — `review_clean_rounds`/`audit_clean_rounds` 1–10, `gate_timeout_seconds` 30–3600, `review_max_rounds`/`audit_max_rounds` 0–50 (0 = no cap, a valid boundary) — or a value that does not parse as the field's type is a validation error (exit 1); an unknown or non-settable field name (including `quality_gate`/`commit_strategy`) is a usage error (exit 2). In every rejected case the task file is left unmodified. Unsetting a field is out of scope for this release (§9).

6.2 `tp set --workflow --project <field>=<value>` continues to edit `.tp/config.json` and is unaffected.

## 7. Existing task files

7.1 Backward compatibility is not a design constraint for this release; the goal is the cleanest correct model. Existing task files with a materialized `workflow` block still parse — their present fields read as explicit overrides (they always did), so a legacy present-but-never-set field is indistinguishable from a genuine override and stays in force until the file is deliberately thinned. Nothing rewrites or thins such files automatically. A project that wants to stop masking its `.tp/config.json` thins its task files deliberately; `tp config --extract` (v0.24.0) still hoists shared policy into `.tp/config.json` and thins the task files accordingly, and is unaffected by this release. An authored or imported task file may also carry an out-of-range workflow value; tp does not reject it on load (only `tp set --workflow` hard-validates, §6.1). Such a value is preserved in the JSON but clamped at resolution — unset to fall back to the built-in default, with a warning (tp's existing `clampWorkflowRanges` read-time fallback) — so "present regardless of value" (§3.1) means present in the file, not necessarily in force.

## 8. Migration (tp's own repo)

8.1 tp dogfoods the fix: `spec/0.25.0.tasks.json` currently carries a materialized `workflow` block. As part of this release it is thinned to `"workflow": {}` — but only after a verified equivalence check, not on assertion. `tp config --resolved` is captured for the file before thinning and again after, and the two must report an identical `value` for every workflow field; any field whose materialized value differs from what `.tp/config.json` plus the built-in defaults resolve to is a **migration blocker**, not a silent overwrite. For tp's own repo the materialized values equal the project config, so the thinning is value-preserving — the check makes that a verified fact rather than an assumption.

## 9. Non-Goals

1. No `tp set --workflow --unset <field>` to remove a field — presence is only added in this release; removal is a separate feature.
2. No change to the resolution precedence or to `.tp/config.json`'s schema.
3. No new command or flag.
4. No automatic thinning of existing task files (§7.1) — migration stays a deliberate, user-invoked step.
5. No change to how the built-in defaults themselves are defined (still one source of truth in code).
6. No removal of `commit_strategy`. It is written by `tp init --commit-strategy` yet consumed by no command; this release keeps it as a presence-tracked, round-tripped field (§4.1) and does not remove the flag or field. Retiring the vestigial field is a separate cleanup.
7. No change to the `tp set --workflow` settable set or to the read-only status of `quality_gate`/`commit_strategy` (§6.1) — they become presence-tracked in storage but stay init-authored.

## 10. Tests / Acceptance

10.1 `tp init <spec>` with no workflow flags produces `"workflow": {}`; with `--quality-gate "<cmd>"` produces exactly `{"quality_gate": "<cmd>"}`; with `--commit-strategy "<value>"` produces exactly `{"commit_strategy": "<value>"}`.

10.2 **Round-trip:** a task file with `"workflow": {}` survives `tp add`, `tp claim`, `tp next`, `tp done`, `tp close`, `tp remove`, `tp commit`, `tp set` (a non-workflow field), and `tp reopen` with its `workflow` block still `{}`. A task file with only `{"review_max_rounds": 4}` keeps exactly that one field across the same operations. A source file with no `workflow` key is normalized to `{}` on the next write, not populated with defaults.

10.3 **No masking:** with `.tp/config.json` `{"workflow": {"review_clean_rounds": 3}}` and a `tp init`-created task file, `tp config --resolved` reports `review_clean_rounds: {value: 3, source: "project"}` (not `2` / `source: "override"`), and `tp review <spec> --status --check` exits non-zero until 3 clean rounds are recorded, then exits 0.

10.4 **Explicit override still wins:** a task file that sets `review_clean_rounds: 5` resolves to 5 over a project config's 3. An invalid explicit set — `tp set --workflow review_clean_rounds=0` (below 1–10) or `tp set --workflow audit_max_rounds=51` (above 0–50) — exits 1 with a hint and leaves the file unmodified, while `tp set --workflow review_max_rounds=0` (a valid no-cap boundary) succeeds and writes `{"review_max_rounds": 0}`.

10.5 `tp set --workflow review_max_rounds=4` on a `{}` block yields `{"review_max_rounds": 4}` and does not add any sibling field.

10.6 **`checks` presence:** an unset `checks` is absent and inherits the project config's `checks`; an explicit `checks` override is written present (a non-empty array as given, an explicit empty as `[]`), and an explicit empty overrides the project config to no checks (§4.3).

10.7 **`commit_strategy` round-trip:** a task file with `{"commit_strategy": "squash"}` keeps exactly that field across the §10.2 operations, and `tp config --resolved` attributes it to `source: "override"`.

10.8 **Out-of-range on load:** an authored or imported task file with `review_clean_rounds: 0` is not rejected; the field is preserved in the JSON but resolves to the built-in default (2) with a warning (§7.1) — whereas the same value via `tp set --workflow review_clean_rounds=0` exits 1 (§6.1).

10.9 **`quality_gate` round-trip:** a task file with only `{"quality_gate": "<cmd>"}` keeps exactly that field across the §10.2 operations, and `tp config --resolved` attributes it to `source: "override"`.

10.10 **Non-settable field:** `tp set --workflow quality_gate=<cmd>` is rejected as a usage error (exit 2) and leaves the file unmodified; the settable set is the five integers plus `checks` (§6.1).

10.11 **Import presence:** importing a source whose `workflow` is `{}` (or absent) produces a target with `"workflow": {}`; importing a source with only `{"review_max_rounds": 4}` produces a target with exactly that field — no default materialization, no inheritance from any prior target (§3.6).
