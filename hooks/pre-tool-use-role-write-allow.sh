#!/bin/sh
# pre-tool-use-role-write-allow.sh - the write allowlist the tp-reviewer and
# tp-auditor agent definitions register for themselves (v0.35.0 sections 6.3
# and 6.4).
#
# Section 6.3 gives a review-role or audit-role unit exactly one path it may
# write - its own findings file at $TP_ROUND_DIR/role-$TP_UNIT_ID.ndjson.part -
# plus the escalation record `tp escalate` writes on its behalf, and denies
# every other write. Claude Code's agent frontmatter carries no per-path
# permission block, so a PreToolUse hook is how a definition says that; the two
# role agents register this one and the implementer does not, because an
# implement unit's durable write is code.
#
# This is a different rule from the plugin-level fence in
# pre-tool-use-write-deny.sh. That one denies four classes of tp's own state to
# every session; this one is an allowlist scoped to the two agents that carry
# it, and it denies by default: anything not on the list is refused.
#
# The .part suffix is deliberate and is the name the role's own prompt gives it
# too (section 6.3), so the prompt and this allowlist name one filename rather
# than two. The driver renames it to role-$TP_UNIT_ID.ndjson only after the
# child exits 0, which is why the final name is NOT writable from in here.
#
# Claude Code sends the tool call on stdin as JSON and reads exit 2 as a
# refusal, with stderr as the reason the agent is given. The path is pulled out
# with grep rather than a JSON parser for the reason the sibling hook documents:
# a path quoted inside the file being written arrives escaped as \"file_path\",
# and the pattern below requires a bare quote on both sides of the key, so it
# can only ever match the tool's own argument.

set -u

payload=$(cat)

# The two permitted paths, derived from the environment the driver gives every
# child (section 3.1). An empty value means the environment does not say which
# file is this unit's own, and an allowlist that cannot build itself refuses
# rather than falls open - a fence that opens when it is confused is the prose
# fence again.
findings=''
escalation=''
if [ -n "${TP_ROUND_DIR:-}" ] && [ -n "${TP_UNIT_ID:-}" ]; then
	findings="$TP_ROUND_DIR/role-$TP_UNIT_ID.ndjson.part"
fi
if [ -n "${TP_RUN_DIR:-}" ] && [ -n "${TP_UNIT_SEQ:-}" ]; then
	escalation="$TP_RUN_DIR/$TP_UNIT_SEQ-escalation.json"
fi

cwd=$PWD

# physical_path echoes a path with its DIRECTORY resolved through symlinks. The
# file itself is usually the one about to be created, so it cannot be resolved -
# but some ancestor of it exists, and `cd ... && pwd -P` is the POSIX way to get
# the physical form of that ancestor.
#
# It walks up to the nearest ancestor that can be entered rather than requiring
# the immediate parent, because resolving only the parent makes the whole
# reconciliation conditional on the round directory already existing. Under
# `tp run` it does - the driver creates it in prepare(), before spawnAll() - so
# the narrower version denied nothing live. What it did do is make this repo's
# own suite pass or fail on whether the checkout path traverses a symlink, and a
# test that passes for a reason unrelated to the code is not evidence about it.
#
# Walking up cannot widen the allowlist: the unresolved components are appended
# to the resolved ancestor unchanged, so two paths still compare equal only when
# they name the same file. A path with no enterable ancestor within the bound
# echoes unchanged, leaving the textual comparisons below as the only ones that
# match - which is exactly what this hook did before the walk existed.
#
# The bound is not decoration. Each step costs a subshell, so an unbounded walk
# makes the hook linear in path depth: a 440-component path measured 3.7s here
# against the 10-second timeout the agent definitions declare, and past that
# timeout the runtime kills the hook - which does NOT exit 2, so an allowlist
# fails OPEN. Eight steps is far more than the real case needs: under `tp run`
# the driver creates the round directory before spawning, so the miss is the
# leaf, and a missing round directory is two or three. Splitting with parameter
# expansion rather than dirname/basename keeps each step to the one subshell
# `cd` needs instead of three.
physical_path_climb_max=8

physical_path() {
	pp_rest=${1##*/}
	pp_dir=${1%/*}
	if [ "$pp_dir" = "$1" ]; then
		pp_dir=.
	elif [ -z "$pp_dir" ]; then
		pp_dir=/
	fi

	pp_climb=0
	while [ "$pp_climb" -le "$physical_path_climb_max" ]; do
		if pp_real=$(cd "$pp_dir" 2>/dev/null && pwd -P); then
			printf '%s/%s' "$pp_real" "$pp_rest"
			return
		fi
		case $pp_dir in
		/ | .) break ;;
		esac
		pp_rest="${pp_dir##*/}/$pp_rest"
		pp_up=${pp_dir%/*}
		if [ "$pp_up" = "$pp_dir" ]; then
			pp_dir=.
		elif [ -z "$pp_up" ]; then
			pp_dir=/
		else
			pp_dir=$pp_up
		fi
		pp_climb=$((pp_climb + 1))
	done
	printf '%s' "$1"
}

# same_path reports whether the path a tool named is the permitted one. The
# comparison is textual first because the file usually does not exist yet - it
# is the one about to be created. The driver may hand the round directory
# absolutely or relative to the session's cwd, and the agent may name the target
# either way, so both directions are reconciled against $PWD; a leading ./ is
# stripped from both sides first.
#
# The physical comparison is last and exists because a textual one over-DENIES:
# when the project path traverses a symlink - on macOS /tmp is /private/tmp, and
# this repo's own QA convention puts projects under /tmp - the driver hands the
# round directory by its symlinked spelling while the child's $PWD is physical,
# so the unit's ONE permitted write was refused. The role then wrote no findings
# and the Stop hook blocked it for not having written them, deterministically on
# every retry. Resolving cannot widen the allowlist: two paths that resolve alike
# are the same file.
same_path() {
	given=$1
	want=$2
	[ -n "$want" ] || return 1

	case $given in ./*) given=${given#./} ;; esac
	case $want in ./*) want=${want#./} ;; esac

	[ "$given" = "$want" ] && return 0

	case $want in
	"$cwd"/*) [ "$given" = "${want#"$cwd"/}" ] && return 0 ;;
	/*) : ;;
	*) [ "$given" = "$cwd/$want" ] && return 0 ;;
	esac

	[ "$(physical_path "$given")" = "$(physical_path "$want")" ] && return 0
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
	same_path "$path" "$findings" && continue
	same_path "$path" "$escalation" && continue

	printf '%s\n' \
		"tp role scope: $path is not this unit's to write (v0.35.0 §6.3)." \
		"A review-role or audit-role unit writes exactly one file: ${findings:-none - TP_ROUND_DIR and TP_UNIT_ID are what name it, and this unit has neither}." \
		"An escalation goes through \`tp escalate\`, which writes the run's escalation record on your behalf." \
		'Everything else - source, specs, docs, another role, another round, the merged file - is outside this unit.' \
		'A finding outside your scope belongs in your findings file, not in an edit.' >&2
	exit 2
done

exit 0
