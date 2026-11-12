# tp — Review and audit role files live in the spec's state directory

A backlog decision, decided by the operator in the round-1 fixes of the 1.12.0 bundle. It applies
the remedy of `ground-scratch-in-state-dir.md` to the files review and audit roles write outside a
run.

Class: **decision**.

## 1. The decision

**Context.** Outside `tp run`, each prompt `tp review` and `tp audit` emit names its `output_path`
as `review-<base>-r<N>-<role>.ndjson` / `audit-<base>-r<N>-<role>.ndjson`, relative to the working
directory. Two specs with the same base in different directories, reviewed from one working
directory, name the same file per role and round, and the second panel's rows overwrite the
first's with no error. It is the cause ground's scratch file had, and the remedy is the same.
Inside a run the path is `$TP_ROUND_DIR/role-<role>.ndjson.part` and is unchanged.

**Decision.**

- The role file moves into a subdirectory of the spec's own state directory, one per phase:
  `<spec dir>/.tp-review/<base>/review-inbox/round-<N>-<role>.ndjson` and
  `.../audit-inbox/round-<N>-<role>.ndjson`. Identity comes from location, and an in-flight file
  is never mistaken for a recorded round, which sits one directory up.
- **No fallback read of the old names.** `tp review --merge` and `tp audit --merge` refuse an
  input named in the old shape, `<phase>-<base>-r<N>-<role>.ndjson`, with exit 3 and a hint naming
  the file to move it to; a file there may be the other spec's.
- `--record` of a round removes that round's files from the phase's inbox once the round is on
  disk, and the inbox with them when it empties, as ground's `--record` does.
- The dirty-state classifier reports an inbox file as round bookkeeping carrying its round number.
- `scripts/clean-emissions.sh` judges inbox files on the terms it judges ground's: an old one whose
  round was never recorded is removable, and a leftover whose round is recorded is removable too.
- The plugin's write fence lets exactly these names through; plugin content is already at 1.12.0.

**Upgrade.** A round emitted before the upgrade and not yet recorded has its role files under the
old names in the working directory. The new binary does not read them: `--merge` names each one
and where to move it, and the operator moves them and merges again. A round re-emitted after the
upgrade names the new paths, so the roles can also be run again instead.

**Consequences.** The path in every review and audit prompt outside a run changes. In-flight role
files sit inside the repository under the spec, so they show in the working tree's status until
`--record` removes them.

## 2. Alternatives

- **A path-derived slug in the cwd-relative name.** Rejected for ground's reason: the same spec
  reached by two spellings would name two files.
- **Read the old names once for compatibility.** Rejected: a file there may be the other spec's.

## 3. Non-goals

- The paths inside a run (`$TP_ROUND_DIR`).
- The epic review inbox, which is already in its slice's state directory.
