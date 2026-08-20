#!/bin/sh
# pre-tool-use-write-deny.sh - the tp plugin's PreToolUse hook (v0.35.0 sections 6.2 and 6.4).
#
# It is what turns the unit scope fence from prose into enforcement. The fence
# has been documented since v0.30.0, and a unit that ignores it produces exactly
# the failure the postmortem described: a boundary that exists only as
# instruction text. The four classes below are tp's own state, and tp's own
# commands are the only thing that may rewrite them.
#
# Shell tools are deliberately outside this hook's matcher. The denial exists to
# stop hand-editing, not to sandbox: tp rewrites `*.tasks.json` on every close,
# and it does so through a shell.
#
# Claude Code sends the tool call on stdin as JSON and reads exit 2 as a refusal,
# with stderr as the reason the agent is given. The path is pulled out with grep
# rather than a JSON parser: that needs no dependency, and it is exact for the
# case that matters, because a path quoted inside the file being written arrives
# escaped as \"file_path\". The pattern below requires a bare quote on both sides
# of the key, so it can only ever match the tool's own argument, never a file's
# contents.

set -u

payload=$(cat)

# denied reports whether one path is inside the fence. `?*` requires a non-empty
# remainder, so the `.tp-review` directory itself is not a denied write - its
# contents are.
denied() {
	case $1 in
	*/.tp-review/?* | .tp-review/?*) return 0 ;;
	*.tasks.json) return 0 ;;
	*/.tp/config.json | .tp/config.json) return 0 ;;
	*/.tp/local.json | .tp/local.json) return 0 ;;
	esac
	return 1
}

# Write, Edit and MultiEdit name their target `file_path`. Notebook payloads
# have been seen under both `file_path` and `notebook_path`, so both are read
# and one hook covers all four tools in the matcher.
paths=$(printf '%s' "$payload" |
	grep -Eo '"(file_path|notebook_path)"[[:space:]]*:[[:space:]]*"[^"]*"' |
	sed -e 's/^[^:]*:[[:space:]]*"//' -e 's/"$//')

IFS='
'
for path in $paths; do
	denied "$path" || continue
	printf '%s\n' \
		"tp scope fence: $path is tp's own state and must not be hand-edited (v0.35.0 §6.2)." \
		'Change it with the tp command that owns it: task files through tp done / tp set / tp import / tp remove,' \
		'.tp/config.json through tp config, .tp/local.json through tp use, and the round files under .tp-review/' \
		'through tp review / tp audit. A finding outside your scope belongs in the closure evidence, not in an edit.' >&2
	exit 2
done

exit 0
