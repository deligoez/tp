# tp v0.24.0 — Project-level configuration (`.tp/`)

tp forces every task to carry `source_sections` so coverage is *derived* from the spec, never
maintained as a parallel hand-kept list (principle P4). Yet workflow **policy** —
`quality_gate`, `review_clean_rounds`, `audit_clean_rounds`, `review_max_rounds`,
`audit_max_rounds`, `checks` — lives only in each spec-adjacent `<base>.tasks.json`. A project
with many specs (one real user has 45 chapters = 45 specs = 45 task files) must hand-copy the
same policy into every file; when one drifts (a chapter's `review_max_rounds` left at 0 instead
of 8), nothing detects it and chapters silently run different policies.

v0.24.0 applies tp's own "derive, don't maintain a parallel list" principle to the policy layer:
a single optional project-level config supplies the workflow defaults, each task file keeps only
its explicit deviations, and effective values are resolved at read time. Per-developer state
(the active-file pointer and flag defaults) lives in a separate, git-ignored local file so that
shared policy and session state never mix. The feature is fully opt-in: with no `.tp/`
directory present, every command behaves byte-for-byte as in v0.23.0.

## 1. Goals

1. Introduce an optional project-level directory `.tp/` holding a committed policy file
   (`.tp/config.json`, workflow defaults) and a git-ignored local file (`.tp/local.json`,
   the active-file pointer and global flag defaults).
2. Resolve effective settings at read time by layering the project config under each task
   file's explicit overrides — never by copying defaults into task files.
3. Preserve zero-config behavior: when no `.tp/` directory is discovered, every command behaves
   exactly as in v0.23.0.
4. Make cross-spec policy drift visible: report every task file whose override deviates from the
   project default across a well-defined project-wide scan.
5. Retire the standalone `.tp-active` marker by folding the active-file pointer into
   `.tp/local.json`, with a backward-compatibility window that ends in v0.25.0.

## 2. The `.tp/` directory

### 2.1 Files and responsibilities

1. `.tp/config.json` holds the single top-level key `workflow` (project-wide workflow-field
   defaults) and MUST be committed to version control; import convergence enforcement and CI
   depend on it traveling with the repository.
2. `.tp/local.json` holds `active` (the active task-file pointer) and `defaults` (global flag
   defaults). It is per-developer, per-session state and MUST NOT be committed.
3. Whenever tp writes any file under `.tp/`, it first ensures `.tp/.gitignore` exists and
   contains a `local.json` entry (creating or appending idempotently), so `.tp/local.json` is
   ignored even when the `.tp/` directory was created by hand rather than by tp.
4. Both files are optional and independent: either may exist without the other, and an absent
   file is equivalent to an empty object `{}`.

### 2.2 Discovery

1. The `.tp/` directory is discovered exactly once per invocation from a single deterministic
   anchor: the current working directory, walking upward and testing each ancestor (including
   the anchor itself and the boundary directory) for a `.tp/` directory, stopping at the first
   hit.
2. The upward walk halts at the repository boundary — the first ancestor containing a `.git`
   directory **or** a `.git` file (git worktrees and submodules use a `.git` file) — or the
   filesystem root, whichever comes first, and never reads a `.tp/` above that boundary. The
   directory containing `.git` is called the project root.
3. Discovery is never re-anchored to a `--file` in another directory: the project config found
   from the working directory is the single config instance that supplies `workflow` for
   whatever task file the invocation resolves.
4. When no `.tp/` directory is found, tp uses built-in defaults and per-task-file settings
   exactly as in v0.23.0.

### 2.3 Schema and validation

1. `.tp/config.json` accepts one top-level key, `workflow`, an object using the same fields as a
   task file's `workflow` block (`quality_gate`, `gate_timeout_seconds`, `review_clean_rounds`,
   `audit_clean_rounds`, `review_max_rounds`, `audit_max_rounds`, `checks`).
2. `.tp/local.json` accepts two top-level keys, `active` (string) and `defaults` (object of
   boolean flag values).
3. Unknown top-level keys, unknown keys inside `workflow`, and unknown keys inside `defaults`
   are ignored and reported as a validation warning on stderr by every command that reads the
   file; they are never errors.
4. A type-mismatched value — `active` that is not a string, or a `defaults` entry that is not a
   boolean — is reported as a validation warning on stderr and treated as unset (ignored), not
   an error.
5. A workflow value outside its documented range in a hand-edited `.tp/config.json` falls back
   to the built-in default at read time (`gate_timeout_seconds`→600, caps→0, clean_rounds→2) and
   `tp validate`/`tp config` warns on stderr — the same read-time fallback v0.23.0 applies to a
   hand-edited task file.
6. A file that is not readable, is not valid JSON, or whose top-level JSON value is not an
   object (e.g. `[]`, `42`, `"x"`, or a 0-byte / whitespace-only file) is malformed: it aborts
   the command that reads it with exit 3 and a `repair or delete <path>` hint. An explicit empty
   object `{}` is valid and equivalent to an absent file. tp never silently ignores a corrupt
   config; because every command that resolves workflow, active, or flag-default values reads
   these files, any such command can abort with exit 3.
7. All warnings and errors from config handling are written to stderr; JSON emitted on stdout is
   never polluted by a warning line.

## 3. Layered resolution (resolve-at-read)

### 3.1 Precedence

Every setting is resolved per-field by the following precedence, highest wins:

| Rank | Source | Notes |
|------|--------|-------|
| 1 | CLI flag | e.g. `--file`, `--compact`, `--no-compact`, `--color`, `--no-color` |
| 2 | Environment variable | `TP_FILE` (active), `NO_COLOR` (`no_color` only); a rank-1 CLI flag overrides its rank-2 env peer |
| 3 | Task file's explicit `workflow` field (override) | workflow fields only |
| 4 | `.tp/config.json` `workflow` (project) / `.tp/local.json` `defaults` (project-local) | — |
| 5 | Built-in default | — |

1. Resolution is per-field (sparse merge): a task file that sets only `review_max_rounds`
   inherits every other workflow field from the project config.
2. Not every field passes through every rank. Workflow fields resolve across ranks 3, 4, 5 only
   (no CLI/env layer). Among the flag `defaults`, only `no_color` has a rank-2 env peer
   (`NO_COLOR`); `compact` and `quiet` resolve across ranks 1, 4, 5 (CLI flag, project-local,
   built-in default) with no env layer.
3. `active` is resolved by the discovery chain in §7.1, not by this workflow ladder.

### 3.2 Presence, not value, defines an override

1. Every workflow field is presence-tracked (an absent key is distinct from an explicit zero
   value): the implementation MUST use optional/pointer fields or raw-key inspection, because
   `review_max_rounds: 0` (0 = no cap) is a legitimate explicit value that must be
   distinguishable from an absent key.
2. A task file field is an override when the key is present, regardless of its value; an absent
   key inherits the project value.
3. `checks` uses replace semantics across layers: a task file with a `checks` key — including an
   explicit empty array `checks: []`, which replaces the project checks with an empty set (no
   checks run) — replaces the project `checks` entirely; only an absent `checks` key inherits
   the project `checks`.

## 4. Editing the project config

1. `tp set --workflow --project field=value` writes the field to the `workflow` block of the
   `.tp/config.json` discovered by §2.2. When no `.tp/` exists, it creates `.tp/config.json`
   (and `.tp/` plus `.tp/.gitignore`) in the project root — the boundary directory that contains
   `.git` (whether `.git` is a directory or a file) — or, when not inside a git repository, in
   the current working directory.
2. `--project` writes reject the same out-of-range values as per-task `tp set --workflow`
   (`gate_timeout_seconds` 30-3600, `review_clean_rounds`/`audit_clean_rounds` 1-10,
   `review_max_rounds`/`audit_max_rounds` 0-50) with exit 1.
3. `tp set --workflow --project quality_gate="…"` is allowed: project-level `quality_gate` is
   authorable, unlike the per-task read-only field.
4. `.tp/local.json` `defaults` are written by `tp set --local defaults.<flag>=<bool>`, where
   `<flag>` must be one of `compact`, `quiet`, `no_color` and `<bool>` must be `true`/`false`;
   an unknown flag or non-boolean value is rejected with exit 1. `active` is written only by
   `tp use` (§7.1). `tp set --local` creates `.tp/`, `.tp/.gitignore`, and `.tp/local.json` when
   absent, in the same location as §4.1. All `--project`/`--local` writes acquire the same flock
   used by every other tp write.

## 5. Layered resolution is used everywhere workflow fields are read

1. `tp import` convergence enforcement, the `tp done`/`tp close` quality gate, and the review and
   audit round-budget checks all read the **resolved** (layered) effective values, never the raw
   task-file `workflow` block alone.
2. Worked consequence: a thinned task file with no `review_max_rounds` key inherits the project
   cap; enforcement uses that inherited cap, not the built-in `0`. A task file that omits
   `quality_gate` runs the project `quality_gate` at `tp done`.

## 6. Inspecting the resolved config

1. `tp config` prints the resolved effective configuration as JSON on stdout for the active (or
   `--file`) task file. When no task file resolves, it prints the project layer alone (no
   per-task overrides) and exits 0.
2. `tp config --resolved` annotates each resolved setting with a `value` and the `source` layer
   that supplied it. The `source` enum is `cli`, `env`, `override`, `project`, `local`, or
   `default`: workflow fields report `override`/`project`/`default`; `active` and `defaults`
   values that come from `.tp/local.json` report `local` (not `project`, which is reserved for
   the committed `.tp/config.json`). Example:

   ```json
   {
     "workflow": {
       "review_max_rounds": { "value": 8, "source": "project" },
       "review_clean_rounds": { "value": 3, "source": "override" }
     },
     "active": { "value": "chapter-03/chapter-03.tasks.json", "source": "local" },
     "defaults": { "compact": { "value": true, "source": "local" } }
   }
   ```

3. `tp config` always emits JSON; `--compact` has no effect on it (documented no-op, since the
   output is not task-shaped).

## 7. Active file

### 7.1 Active-file pointer

1. `.tp/local.json`'s `active` key holds a path relative to the project root (the directory
   containing the discovered `.tp/`); it takes the place of the `.tp-active` marker in the
   discovery chain: `--file` > `TP_FILE` > `.tp/local.json:active` > legacy `.tp-active` >
   auto-detect.
2. `tp use <file>` writes the path to `.tp/local.json:active` (creating `.tp/`, `.tp/.gitignore`,
   and `.tp/local.json` as in §4.1) and no longer creates a `.tp-active` file.
3. `tp use --clear` removes the `active` key from `.tp/local.json`; it is a no-op with exit 0
   when no `active` is set, and it also removes a leftover legacy `.tp-active` in the working
   directory.
4. `tp use` with no argument prints the resolved active file and the discovery-chain rank that
   supplied it.

### 7.2 Missing or invalid active target

1. When `active` resolves to a missing or unreadable file, tp emits a warning on stderr and
   continues down the discovery chain (legacy `.tp-active`, then auto-detect) rather than
   aborting; `tp use` with no argument reports the dangling pointer.
2. An `active` value that is absolute or escapes the project root is rejected with a validation
   warning and treated as unset.

### 7.3 `.tp-active` deprecation

1. A pre-existing `.tp-active` marker is still read as a lower-priority fallback (read from the
   working directory only, as in v0.23.0) through the v0.24.x series, and its use emits a
   deprecation warning on stderr naming the `tp use` migration path.
2. When both `.tp/local.json:active` and a legacy `.tp-active` exist, the project-local value
   wins.
3. Support for `.tp-active` is removed in v0.25.0.

## 8. Global flag defaults

1. `.tp/local.json`'s `defaults` object supplies default values for the boolean global flags
   `compact`, `quiet`, and `no_color`, applied when the corresponding flag is absent from the
   CLI. `--json` is intentionally excluded (output mode is already auto-selected by piping) and
   is a documented omission.
2. Because a project default can set a boolean to true, tp adds negating flags — `--no-compact`,
   `--no-quiet`, `--color` — so any boolean default can be overridden in both directions from a
   single invocation; an explicit CLI flag always wins.
3. `no_color` is resolved from, in order: an explicit `--color`/`--no-color` flag, the `NO_COLOR`
   environment variable, `defaults.no_color`, then TTY detection — so the project default sits
   below the standard `NO_COLOR` env and explicit flags, consistent with fatih/color behavior.

## 9. Drift / deviation reporting

1. `tp validate --project` scans every task file under the project (§10) and reports, per file,
   each workflow field whose override differs from a value the project config **explicitly
   sets**, naming the file, the field, the override value, and the project value:
   `chapter-03/chapter-03.tasks.json: review_max_rounds=0 overrides project default 8`.
2. A field the project config does not set carries no project policy, so a task override of such
   a field is not a deviation and is not reported.
3. Equality for the `checks` array is order-insensitive set comparison over `{class, cmd}`
   pairs; a reordered but equal `checks` is not a deviation.
4. Deviations are informational and exit 0 by default; `--strict` promotes any deviation to
   exit 1 (tp's validation exit code) so CI can forbid unreviewed policy drift.
5. When no `.tp/config.json` is present, `tp validate --project` reports that no project config
   was found and exits 0.

## 10. Project-wide task-file discovery

1. `tp validate --project` and `tp config --extract` share one discovery rule: starting at the
   project root (the directory containing the discovered `.tp/`), recurse downward into its
   subdirectories, collecting every `*.tasks.json`.
2. The recursion skips `.git`, `.tp`, `node_modules`, and `vendor` directories, and does not
   descend into a nested submodule (any subdirectory that itself contains a `.git` entry).
3. This project-wide scan is distinct from v0.23.0's single-file auto-detect (current directory
   plus one level); it exists only for the two project-scoped commands above.

## 11. Migration and backward compatibility

1. Upgrading the binary to v0.24.0 requires no migration: with no `.tp/` directory present,
   every command behaves exactly as in v0.23.0, and existing task files and any `.tp-active`
   marker keep working unchanged.
2. Adopting the project config is opt-in and incremental — create `.tp/config.json` by hand or
   run the extractor; nothing in an existing repository is rewritten automatically on upgrade.
3. `tp config --extract` hoists a workflow field into `.tp/config.json` only when **every**
   discovered task file sets that field with an identical value; a field that is absent from any
   file, or whose value diverges across files, is left untouched in every file. It then removes
   each hoisted field from every task file's `workflow` block.
4. `tp config --extract --dry-run` prints the hoist/removal plan without writing. Without
   `--dry-run`, and inside a git repository, `--extract` refuses to run on a dirty working tree
   (exit 4) so the mutation is reviewable; outside a git repository it skips the clean-tree check
   and warns. It refuses when `.tp/config.json` already exists (exit 4) unless `--force`, which
   overwrites that file's `workflow` block with the newly computed common policy. All writes
   acquire the standard flock.
5. Existing task files with full `workflow` blocks keep working unchanged; every explicit field
   is simply treated as an override until the author thins it or runs `--extract`.

## 12. Non-goals

1. No multi-level directory cascading beyond the single project layer; workflow resolution is
   exactly two layers (project + per-task) sitting under the per-invocation CLI/env ranks, not
   an N-level include chain.
2. No cross-project resolution in a monorepo: the config is anchored once at the working
   directory, so a `--file` pointing into a different project still resolves against the
   working directory's project config.
3. No user-home / global config (`~/.config/tp/`) in this release; a user layer would sit
   between the project layer and built-in defaults and is out of scope here.
4. No change to task, coverage, review, or audit semantics; v0.24.0 only adds the configuration
   layer and its resolution, editing, inspection, drift, and migration surfaces.
