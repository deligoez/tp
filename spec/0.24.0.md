# tp v0.24.0 — Project-level configuration (`.tp/config.json`)

tp forces every task to carry `source_sections` so coverage is *derived* from the spec, never
maintained as a parallel hand-kept list (principle P4). Yet workflow **policy** —
`quality_gate`, `review_clean_rounds`, `audit_clean_rounds`, `review_max_rounds`,
`audit_max_rounds`, `checks` — lives only in each spec-adjacent `<base>.tasks.json`. A project
with many specs (one real user has 45 chapters = 45 specs = 45 task files) must hand-copy the
same policy into every file; when one drifts (a chapter's `review_max_rounds` left at 0 instead
of 8), nothing detects it and chapters silently run different policies.

v0.24.0 applies tp's own "derive, don't maintain a parallel list" principle to the policy layer:
a single optional project-level config supplies the defaults, each task file keeps only its
explicit deviations, and effective values are resolved at read time. The feature is fully
opt-in: with no config file present, behavior is byte-for-byte identical to v0.23.0.

## 1. Goals

1. Introduce one optional project-level configuration file, `.tp/config.json`, holding
   workflow defaults, the active-file pointer, and global flag defaults for a whole repository.
2. Resolve effective settings at read time by layering the project config under each task
   file's explicit overrides — never by copying defaults into task files.
3. Preserve zero-config behavior: when no `.tp/config.json` is discovered, every command
   behaves exactly as in v0.23.0.
4. Make cross-spec policy drift visible: report every task file whose override deviates from
   the project default.
5. Retire the standalone `.tp-active` marker by folding the active-file pointer into the
   project config, with a bounded backward-compatibility window.

## 2. The `.tp/config.json` file

### 2.1 Location and discovery

1. The config lives in a `.tp/` directory at the project root: `.tp/config.json`.
2. Discovery walks upward from the resolved task file's directory (or the current working
   directory when no task file is in play), testing each ancestor for `.tp/config.json`, and
   stops at the first hit.
3. The upward walk halts at the repository boundary (the directory containing `.git`) or the
   filesystem root, whichever comes first, and never reads a `.tp/config.json` above that
   boundary — discovery is deterministic and cannot pick up a stray parent-directory config.
4. When no `.tp/config.json` is found, tp uses built-in defaults and per-task-file settings
   exactly as in v0.23.0.
5. `.tp/config.json` must be committed to version control, like `.tp-review/`; import
   enforcement and CI depend on it traveling with the repository.

### 2.2 Schema

The file is a JSON object with three optional top-level keys:

| Key | Type | Purpose |
|-----|------|---------|
| `workflow` | object | Project-wide workflow-field defaults (same fields as a task file's `workflow` block) |
| `active` | string | Repo-relative path to the active task file (replaces the `.tp-active` marker) |
| `defaults` | object | Default values for global flags (`compact`, `no_color`, `quiet`) applied when the flag is absent from the CLI |

1. Every key is optional; an empty `{}` is valid and equivalent to no file.
2. Unknown top-level keys are ignored and reported as a validation warning, never an error.
3. A malformed or unreadable `.tp/config.json` aborts config-consuming commands with exit 3 and
   a `repair or delete <path>` hint; tp never silently ignores a corrupt config.

## 3. Layered resolution (resolve-at-read)

### 3.1 Precedence

Every setting is resolved per-field by the following precedence ladder, highest wins:

| Rank | Source | Scope |
|------|--------|-------|
| 1 | CLI flag or environment variable (`--file`, `TP_FILE`, `--compact`, …) | Per-invocation |
| 2 | Task file's explicit `workflow` field (override) | Per-spec |
| 3 | `.tp/config.json` (`workflow`, `active`, `defaults`) | Project-wide |
| 4 | Built-in default | Global |

1. Resolution is per-field (sparse merge): a task file that sets only `review_max_rounds`
   inherits every other workflow field from the project config.
2. An explicit field in a task file is always treated as an override, even when its value
   equals the project default — presence, not value, defines an override.
3. `tp init` never materializes project defaults into the created task file; a freshly
   initialized task file carries no `workflow` block and inherits everything from the project
   config, so changing the project config instantly changes every inheriting spec.
4. `quality_gate`, read-only in a task file (authored at `tp init --quality-gate`), becomes
   authorable at the project level; a task file's own `quality_gate` still overrides the
   project value when present.
5. `checks` uses replace semantics across layers: a task file with a `checks` array replaces
   the project `checks` entirely; a task file without one inherits the project `checks`.

### 3.2 Inspecting the resolved config

1. `tp config` prints the resolved effective configuration as JSON for the active (or `--file`)
   task file.
2. `tp config --resolved` annotates each field with the layer that supplied it
   (`cli` / `override` / `project` / `default`) so the source of every effective value is
   auditable.
3. `tp config` honors `--compact` and `--json` like other read commands.

## 4. Editing the project config

1. `tp set --workflow --project field=value` writes the field to `.tp/config.json` instead of
   the active task file, creating `.tp/config.json` (and the `.tp/` directory) when absent.
2. `--project` writes reject the same out-of-range values as per-task `tp set --workflow`
   (`gate_timeout_seconds` 30-3600, `review_clean_rounds`/`audit_clean_rounds` 1-10,
   `review_max_rounds`/`audit_max_rounds` 0-50) with exit 1.
3. `tp set --workflow --project quality_gate="…"` is allowed (project-level `quality_gate` is
   authorable, unlike the per-task read-only field).
4. Managed task fields remain rejected; `--project` only accepts `workflow` keys, `active`, and
   `defaults` entries.

## 5. Active file and global flag defaults

### 5.1 Active-file pointer

1. The project config's `active` key holds the active task file; it takes the place of the
   `.tp-active` marker in the discovery chain: `--file` > `TP_FILE` > `.tp/config.json:active` >
   legacy `.tp-active` > auto-detect.
2. `tp use <file>` writes the path to `.tp/config.json:active` and no longer creates a
   `.tp-active` file.
3. `tp use --clear` removes the `active` key from `.tp/config.json`.
4. `tp use` with no argument prints the resolved active file and the layer that supplied it.

### 5.2 `.tp-active` deprecation

1. A pre-existing `.tp-active` marker is still read as a lower-priority fallback for one minor
   release, and its use emits a deprecation warning naming the `tp use` migration path.
2. When both `.tp/config.json:active` and a legacy `.tp-active` exist, the project config wins.

### 5.3 Global flag defaults

1. `defaults` supplies values for `compact`, `no_color`, and `quiet` applied when the
   corresponding flag is absent from the CLI; an explicit CLI flag always overrides the default.
2. Unknown `defaults` keys are ignored with a validation warning.

## 6. Drift / deviation reporting

1. `tp validate --project` scans every task file discovered under the project root and reports,
   per file, each workflow field whose override differs from the project config value.
2. Each deviation line names the task file, the field, the override value, and the project
   default: `chapter-03.tasks.json: review_max_rounds=0 overrides project default 8`.
3. Deviations are reported as informational findings, not errors; `--strict` promotes them to a
   non-zero exit so CI can forbid unreviewed policy drift.
4. When no `.tp/config.json` is present, `tp validate --project` reports that no project config
   was found and exits 0.

## 7. Migration and backward compatibility

1. Upgrading the binary to v0.24.0 requires no migration: with no `.tp/config.json` present,
   every command behaves exactly as in v0.23.0, and existing task files and any `.tp-active`
   marker keep working unchanged.
2. Adopting the project config is opt-in and incremental — create `.tp/config.json` by hand or
   run the extractor; nothing in an existing repository is rewritten automatically on upgrade.
3. `tp config --extract` reads the workflow blocks of all discovered task files, writes their
   common policy to `.tp/config.json`, and removes the matching fields from each task file's
   `workflow` block, leaving only genuine deviations behind.
4. `tp config --extract` is a convenience, never automatic; it prints a summary of what it
   hoisted and what deviations it left in place.
5. Existing task files with full `workflow` blocks keep working unchanged; every explicit field
   is simply treated as an override until the author thins it.

## 8. Non-goals

1. No multi-level directory cascading beyond the single project layer; resolution is exactly
   two layers (project + per-task), not an N-level include chain.
2. No user-home / global config (`~/.config/tp/`) in this release; a user layer above the
   project layer is a possible future third rank and is out of scope here.
3. No change to task, coverage, review, or audit semantics; v0.24.0 only adds the configuration
   layer and its resolution, editing, inspection, drift, and migration surfaces.
