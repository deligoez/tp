#!/bin/sh
# subagent-stop-claim.sh - the tp plugin's SubagentStop hook
# (spec/backlog/agent-bound-claims.md, Decision 3).
#
# The spawn hook claimed the marked task for the Agent call's tool_use_id.
# Claude Code keeps that id in the agent's meta.json beside its transcript
# (`toolUseId`), so this hook links the stop to the spawn without state of its
# own, asks `tp claims --spawn` which of its tasks are neither done nor
# waiting in any worktree's copy, and blocks the stop once while any is left.
# The second stop arrives with stop_hook_active set and passes, so an agent
# that truly cannot finish is never held. No meta.json, no claims, or a stop
# inside a tp run unit (whose driver accounts for its tasks) exits 0.

set -u

[ -n "${TP_UNIT_KIND:-}" ] && exit 0

payload=$(cat)

if printf '%s' "$payload" | grep -Eq '"stop_hook_active"[[:space:]]*:[[:space:]]*true'; then
	exit 0
fi

field() {
	printf '%s' "$1" |
		grep -Eo "\"$2\"[[:space:]]*:[[:space:]]*\"[^\"]*\"" | sed -n 1p |
		sed -e 's/^[^:]*:[[:space:]]*"//' -e 's/"$//'
}

transcript=$(field "$payload" agent_transcript_path)
case $transcript in
*.jsonl) ;;
*) exit 0 ;;
esac
meta=${transcript%.jsonl}.meta.json
[ -f "$meta" ] || exit 0
spawn=$(field "$(cat "$meta")" toolUseId)
printf '%s\n' "$spawn" | grep -Eq '^[A-Za-z0-9_-]{1,128}$' || exit 0
command -v tp >/dev/null 2>&1 || exit 0

from=$(field "$payload" cwd)
[ -n "$from" ] || from=${CLAUDE_PROJECT_DIR:-$PWD}
cd "$from" 2>/dev/null || exit 0

block() {
	printf '{"decision":"block","reason":"%s"}\n' "$1"
	exit 0
}

if ! out=$(tp claims --spawn "$spawn" 2>/dev/null); then
	# A tp older than the plugin has no claims command, and then made no
	# claim either: nothing to guard.
	tp claims --help >/dev/null 2>&1 || exit 0
	block "tp claims --spawn $spawn failed, so whether the task bound to your spawn is closed is unknown. Close it with tp done <id> --file <task file>, or record tp set <id> waiting=... --file <task file>, then stop."
fi

# The unaccounted tasks as "<id> <file>" lines, kept only when both are plain
# id and path shapes, since they are written into a JSON string below.
pairs=$(printf '%s' "$out" |
	grep -Eo '"id":"[A-Za-z0-9][A-Za-z0-9._-]*","file":"[A-Za-z0-9._/-]+","accounted":false' |
	sed -e 's/^"id":"//' -e 's/","file":"/ /' -e 's/","accounted":false$//')
if [ -z "$pairs" ]; then
	case $out in
	*'"unaccounted":[]'*) exit 0 ;;
	esac
	block "The task bound to your spawn is not closed (see tp claims --spawn $spawn). Close it with tp done, or record tp set <id> waiting=..., then stop."
fi

steps=''
IFS='
'
for pair in $pairs; do
	id=${pair%% *}
	file=${pair#* }
	steps="$steps Task $id ($file): close it with evidence - tp done $id --file $file -- \\\"<what was done and how it was verified>\\\" (under commit_strategy hc, or in a git worktree of your own, commit your work first and add --commit <sha> before the --) - or, if it waits on something outside your control, record tp set $id waiting=\\\"<what>\\\" --file $file."
done

block "The task bound to your spawn is not closed. Run these from the repository root.$steps Then stop."
