#!/bin/sh
# pre-tool-use-agent-claim.sh - the tp plugin's PreToolUse hook for the Agent
# tool (spec/backlog/agent-bound-claims.md, Decision 2).
#
# A coordinator that hands a sub-agent a task pastes that task's brief into
# the Agent prompt, and the brief carries the marker line
# `tp-task: <id> <task file>`. This hook claims the marked task for the spawn
# before the agent starts, binding it to the call's tool_use_id, so a claim no
# longer depends on the agent remembering to make it. When the claim is
# refused - the task is done, blocked, held by another spawn, unknown, or the
# lock timed out - the hook exits 2 and the spawn does not happen. A prompt
# with no marker passes untouched, and so does every spawn inside a tp run
# unit, whose driver accounts for its own tasks.
#
# Fields are pulled out with grep, as in pre-tool-use-write-deny.sh: a key
# quoted inside the prompt arrives escaped as \"key\", so a pattern with a bare
# quote on both sides only matches the payload's own fields. Every value is
# checked against its shape here and passed to tp as its own argument.

set -u

[ -n "${TP_UNIT_KIND:-}" ] && exit 0

payload=$(cat)

# A marker is a whole line of tool_input.prompt that starts with
# "tp-task: ". Only the prompt is read: a marker quoted in another field, or
# inside a sentence, is not a request to claim. The prompt arrives as a JSON
# string, so its lines are split on the escape \n; an escaped backslash is set
# aside first so that "\\n" is not read as a line break, and a trailing \r of
# a CRLF line is dropped. Every candidate line is checked against the id and
# path shapes below, and one that fails refuses the spawn.
prompt=$(printf '%s' "$payload" |
	grep -Eo '"prompt"[[:space:]]*:[[:space:]]*"([^"\\]|\\.)*"' | sed -n 1p)
[ -n "$prompt" ] || exit 0
markers=$(printf '%s\n' "$prompt" | awk '
{
	sub(/^"prompt"[[:space:]]*:[[:space:]]*"/, "")
	sub(/"$/, "")
	gsub(/\\\\/, "\001")
	n = split($0, line, /\\n/)
	for (i = 1; i <= n; i++) {
		l = line[i]
		sub(/\\r$/, "", l)
		if (index(l, "tp-task: ") == 1) print l
		# A brief run from a shell prints JSON, and pasted whole its
		# marker sits in the "marker" field, quotes escaped once more.
		if (match(l, /\\"marker\\":[ ]*\\"tp-task: [^\\]*\\"/)) {
			m = substr(l, RSTART, RLENGTH)
			sub(/^\\"marker\\":[ ]*\\"/, "", m)
			sub(/\\"$/, "", m)
			print m
		}
	}
}' | sort -u)
[ -n "$markers" ] || exit 0

refuse() {
	printf 'tp agent claim: %s\n' "$1" >&2
	exit 2
}

# One task per spawn: claiming several and refusing on a later one would
# leave the earlier ones claimed for a spawn that never started.
[ "$(printf '%s\n' "$markers" | wc -l)" -le 1 ] ||
	refuse "the prompt marks more than one task (tp-task:). Spawn one agent per task, each with its own brief."

field() {
	printf '%s' "$payload" |
		grep -Eo "\"$1\"[[:space:]]*:[[:space:]]*\"[^\"]*\"" | sed -n 1p |
		sed -e 's/^[^:]*:[[:space:]]*"//' -e 's/"$//'
}

token_ok() {
	printf '%s\n' "$1" | grep -Eq '^[A-Za-z0-9_-]{1,128}$'
}

spawn=$(field tool_use_id)
token_ok "$spawn" || refuse "the Agent call carries no usable tool_use_id, so its marked task cannot be claimed for it."
session=$(field session_id)
if [ -n "$session" ] && ! token_ok "$session"; then
	refuse "session_id \"$session\" is not an id tp records."
fi

command -v tp >/dev/null 2>&1 ||
	refuse "the prompt marks a task (tp-task:), but tp is not on PATH to claim it. Install tp, or drop the marker line."

from=$(field cwd)
[ -n "$from" ] || from=${CLAUDE_PROJECT_DIR:-$PWD}
root=$(git -C "$from" rev-parse --show-toplevel 2>/dev/null) ||
	refuse "$from is not inside a git repository, and a marker's task file is relative to the repository root."

IFS='
'
for marker in $markers; do
	rest=${marker#tp-task: }
	id=${rest%% *}
	file=${rest#* }
	printf '%s\n' "$id" | grep -Eq '^[A-Za-z0-9][A-Za-z0-9._-]*$' ||
		refuse "marker task id \"$id\" is not a task id."
	case /$file/ in
	*/../* | //*) refuse "marker task file \"$file\" must be relative to the repository root, without '..'." ;;
	esac
	printf '%s\n' "$file" | grep -Eq '^[A-Za-z0-9._/-]+$' ||
		refuse "marker task file \"$file\" is not a task file path."
	if [ -n "$session" ]; then
		err=$(cd "$root" && tp claim "$id" --file "$file" --spawn "$spawn" --session "$session" 2>&1 >/dev/null)
	else
		err=$(cd "$root" && tp claim "$id" --file "$file" --spawn "$spawn" 2>&1 >/dev/null)
	fi || refuse "cannot claim $id for this spawn, so it does not start: $err"
done

exit 0
