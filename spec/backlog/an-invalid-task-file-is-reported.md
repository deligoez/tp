# tp — An invalid task file is reported

Class: **tool** — no command, no flag, no workflow field; one new `tp lint` rule over a check that
returns `nil` on both of its failure paths. Its measurements are in
`an-invalid-task-file-is-reported-measurements.md` beside it; this file stands without them. Every
citation below names a symbol or a quoted string and the file it lives in, never a line number.

## 1. Overview

The subject is `tp lint`'s task-file quality check, which swallows both of its failures: an
unreadable task file and an unparseable one each produce zero findings, empty stderr and exit 0,
indistinguishable from a clean lint, so an operator who corrupts a task file is told their spec is
fine. This spec was split out of `refusals-that-name-nothing.md` on 2026-09-08, where it was §6 of
that file, whose forwarding table maps every moved section to its new slug.

## 2. An invalid task file is reported by `tp lint`, not swallowed

`checkTaskFileQuality` (`internal/cli/lint.go`) resolves the spec's task file and returns `nil` on
both of its failure paths — `os.ReadFile` erroring and `json.Unmarshal` erroring. A missing file
returning `nil` is correct and is a third, separate branch; the other two are not the same case.
Measured against a control at `HEAD`: an unparseable task file and an unreadable one each yield zero
`acceptance-quality` findings, zero bytes on stderr and exit 0, against the control's one finding —
the sidecar's *§6 of the spec — `checkTaskFileQuality`, measured* is the table, with the precondition
that the unreadable file is asserted unreadable first.

Nothing in the payload, on stderr or in the exit code separates a swallowed failure from a clean
lint, so an operator who corrupts a task file is told their spec is fine. It is
`an-unreadable-file-is-named.md`'s mechanism in a different command: the failing party holds the
`error` and drops it. The defect predates `v1.0.0`.

**The decision.** Both failure paths become one `error`-severity finding, rule `task-file-invalid`,
naming the task file's path and the underlying error, so `tp lint` exits non-zero on it; the
`acceptance-quality` walk is skipped for that file, since there is nothing to walk. The missing-file
branch is unchanged: a spec with no task file is the ordinary state before decomposition, and it stays
silent. The check's own read error is reported through the same finding rather than through stderr,
because `--json` consumers read findings and not stderr, and a lint failure that only one channel
carries is the class `an-unreadable-file-is-named.md` closes.

## 3. Non-Goals

1. **`tp lint` does not validate the task file's schema.** §2 reports a file that cannot be read or
   parsed as JSON; a file that parses and violates `tp validate`'s rules is `tp validate`'s to report,
   as today.

## 4. Tests

Every row derives from a numbered decision, names the artifact it depends on, and names a mutant that
must fail it.

| # | from | assertion | the mutant that must fail it |
|---|---|---|---|
| 1 | §2 *unparseable* | on a spec whose task file is `{ this is not json`, `tp lint` reports one `task-file-invalid` finding at `error` severity naming the task file's path, and exits non-zero | the shipped `return nil` after `json.Unmarshal`, under which the run is indistinguishable from the control |
| 1b | §2 *unreadable* | on a spec whose task file is `chmod 000` — asserted unreadable first, as in `an-unreadable-file-is-named.md`'s row 1c — `tp lint` reports the same finding naming the read error, and exits non-zero | the shipped `return nil` after `os.ReadFile` |
| 1c | §2 *the control and the missing file* | a valid task file with one short acceptance still yields exactly one `acceptance-quality` finding and no `task-file-invalid`; a spec with no task file yields neither | report the missing-file branch too, which turns every undecomposed spec's lint red |
