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

# A call that only reads is not a hand-edit, so it passes whatever it names.
# The matcher names no reader, but a reader that reaches this hook anyway -
# a wider matcher, a user-level registration, a host that routes every MCP
# call through it - would otherwise have its path argument judged like a
# write target, and a codedbpro read of a round file was refused that way with
# the hand-edit message. The list is drop-batch-reads.awk's reader list plus
# the native readers. The tool name is extracted as the path is below, with a
# bare quote on both sides of the key, and a payload naming more than one
# tool_name matches no single entry and is judged in full.
tool_name=$(printf '%s' "$payload" |
	grep -Eo '"tool_name"[[:space:]]*:[[:space:]]*"[^"]*"' |
	sed -e 's/^[^:]*:[[:space:]]*"//' -e 's/"$//')
case $tool_name in
Read | Grep | Glob | LS | NotebookRead | \
	mcp__codedbpro__read | mcp__codedbpro__faster_search | mcp__codedbpro__meta_search | \
	mcp__codedbpro__diff | mcp__codedbpro__lint | mcp__codedbpro__memo)
	exit 0
	;;
esac
# normalize prints the form a path is compared in: lowercased, with runs of
# separators collapsed and `.` / `..` segments resolved away. Without this the
# fence is a spelling match, and every one of `.tp/./config.json`,
# `.tp//config.json`, `.tp/locks/../config.json` (`.tp/locks/` exists in every
# tp project) and `.TP/config.json` names the fenced file while missing the
# pattern. Case is not cosmetic: on a case-insensitive filesystem - APFS and
# NTFS - a write to `.TP/config.json` clobbers the real `.tp/config.json`.
# awk is POSIX and needs no interpreter beyond the hook's own, and one pass over
# one path stays far inside the 10-second bound section 6.4 puts on every hook.
normalize() {
	printf '%s\n' "$1" | awk '
	{
		p = tolower($0)
		gsub(/\/+/, "/", p)
		lead = (substr(p, 1, 1) == "/") ? "/" : ""
		n = split(p, seg, "/")
		split("", out)
		top = 0
		for (i = 1; i <= n; i++) {
			s = seg[i]
			if (s == "" || s == ".") continue
			if (s == "..") {
				if (top > 0 && out[top] != "..") { top--; continue }
				if (lead != "") continue
			}
			out[++top] = s
		}
		r = ""
		for (i = 1; i <= top; i++) r = r (i > 1 ? "/" : "") out[i]
		print lead r
	}'
}

# denied reports whether one path is inside the fence. `?*` requires a non-empty
# remainder, so the `.tp-review` and `.tp/epic-review` directories themselves
# are not denied writes - their contents are. `.tp/epics.json` and
# `.tp/epic-review/` are the epic graph and its rounds (1.3.0 section 7).
# The patterns are all lowercase because they are matched against the
# normalized spelling, never the raw argument.
denied() {
	case $(normalize "$1") in
	*/.tp-review/?* | .tp-review/?*) return 0 ;;
	*.tasks.json) return 0 ;;
	*/.tp/config.json | .tp/config.json) return 0 ;;
	*/.tp/local.json | .tp/local.json) return 0 ;;
	*/.tp/epics.json | .tp/epics.json) return 0 ;;
	*/.tp/epic-review/?* | .tp/epic-review/?*) return 0 ;;
	esac
	return 1
}

# The fence guards the repository the session works in, not every file that
# happens to carry a fenced name: an auditor that writes a *.tasks.json fixture
# into a mktemp -d directory is writing a fixture, not hand-editing this
# project's task file, and was refused as one while the fence matched names
# alone. The repository is git's top level from the session's project
# directory - $CLAUDE_PROJECT_DIR, which Claude Code sets for every hook, else
# the hook's working directory - or that directory itself outside git.
#
# A path is inside when either its spelling or its physical form - the nearest
# existing ancestor resolved through symlinks, the rest appended unchanged, as
# the role allowlist's physical_path does - lies under the root in either of
# its forms. Both forms are needed and neither widens what passes: git names
# the top level physically while $PWD and the agent may use a symlinked
# spelling (/tmp is /private/tmp on macOS), and a path reaching the repository
# through a symlink elsewhere is still the repository's file. Only a path that
# is outside in every form leaves the fence. The climb is bounded for the
# reason physical_path gives: each step is a subshell, and a hook killed at its
# timeout fails open.
physical() {
	ph_dir=$1
	ph_rest=''
	ph_climb=0
	while [ "$ph_climb" -le 8 ]; do
		if ph_real=$(cd "$ph_dir" 2>/dev/null && pwd -P); then
			printf '%s%s\n' "$ph_real" "$ph_rest"
			return
		fi
		case $ph_dir in / | '') break ;; esac
		ph_rest="/${ph_dir##*/}$ph_rest"
		ph_dir=${ph_dir%/*}
		[ -n "$ph_dir" ] || ph_dir=/
		ph_climb=$((ph_climb + 1))
	done
	printf '%s\n' "$1"
}

# repo_roots prints the session repository's root in both forms, normalized.
repo_roots() {
	rr_from=${CLAUDE_PROJECT_DIR:-$PWD}
	rr_top=$(git -C "$rr_from" rev-parse --show-toplevel 2>/dev/null) || rr_top=''
	[ -n "$rr_top" ] || rr_top=$rr_from
	normalize "$rr_top"
	normalize "$(physical "$rr_top")"
}

# in_repo reports whether one path lies inside the session's repository. A
# root of / holds every path.
in_repo() {
	case $1 in
	/*) ir_abs=$1 ;;
	*) ir_abs=$PWD/$1 ;;
	esac
	ir_text=$(normalize "$ir_abs")
	ir_phys=$(normalize "$(physical "$ir_abs")")
	for ir_root in $roots; do
		case $ir_root in /) return 0 ;; esac
		case $ir_text/ in "$ir_root"/*) return 0 ;; esac
		case $ir_phys/ in "$ir_root"/*) return 0 ;; esac
	done
	return 1
}

# A codedbpro batch is judged op by op, each as the same call made alone would
# be: drop-batch-reads.awk, beside this script and shared with the role
# allowlist, cuts every read op out of the payload, and the extraction below
# judges what is left - every write, every op it cannot classify, every key
# outside the ops - exactly as before. Without this a batch that only read a
# round snapshot, the file the ground prompt tells a grader to read, was
# refused with the hand-edit message, while the same read made as its own call
# never reaches this hook. The awk file says how it classifies and why it can
# only ever remove.
#
# Only a batch is walked, so every other tool's payload reaches the extraction
# byte for byte. A walk that fails, finds nothing to drop, or cannot find the
# classifier at all leaves `payload` as it arrived: the pass can make the fence
# less strict only by the reads it positively identified.
case $payload in
*'"mcp__codedbpro__batch"'*)
	if reduced=$(printf '%s\n' "$payload" | awk -f "${0%/*}/drop-batch-reads.awk" 2>/dev/null); then
		payload=$reduced
	fi
	;;
esac

# Write, Edit and MultiEdit name their target `file_path`. Notebook payloads
# have been seen under both `file_path` and `notebook_path`, so both are read
# and one hook covers all four tools in the matcher. codedbpro's create, edit
# and patch name it `file`; its `replace` names neither, taking `path` for a
# single target and `paths` for a list.
#
# `path` and `paths` were missing until a hook run measured it: replace sits in
# this hook's own matcher, so it was matched, nothing was extracted, the loop
# below never ran, and a fenced write exited 0. The fence failed open on a tool
# it names itself, and the existing test could not see it because it fed every
# tool a `file_path` — proving the fence refuses an argument replace cannot send.
paths=$(printf '%s' "$payload" |
	grep -Eo '"(file_path|notebook_path|file|path)"[[:space:]]*:[[:space:]]*"[^"]*"' |
	sed -e 's/^[^:]*:[[:space:]]*"//' -e 's/"$//')

# `paths` is a JSON array, so it needs its own pass: take the array body, then
# every string inside it. The key itself is dropped -- it is quoted too, and
# `grep -Eo` over the segment would otherwise return it as a target named
# "paths". Newline is an IFS whitespace character, so an empty result here adds
# no field to the loop below.
paths="$paths
$(printf '%s' "$payload" |
	grep -Eo '"paths"[[:space:]]*:[[:space:]]*\[[^]]*\]' |
	grep -Eo '"[^"]*"' |
	grep -v '^"paths"$' |
	sed -e 's/^"//' -e 's/"$//')"

IFS='
'
for path in $paths; do
	# A path argument carrying a backslash is refused outright rather than
	# decoded. The hook compares the spelling that arrived and never decodes
	# JSON, so every escape sequence is a spelling normalize() cannot see
	# through: a `\` before a `/`, or a `u0074` code-point escape in place of
	# a letter, both name `.tp/config.json` while missing every pattern above.
	#
	# Decoding JSON is not the answer here. Section 6.2 scopes this fence to
	# stopping hand-editing rather than to sandboxing, so a JSON parser in
	# POSIX shell is more than it should carry, and a half-parser would be a
	# new place for the same hole to reappear. Failing closed is what the
	# fence's sibling already does: the role write-allow hook denies when the
	# environment it judges by is absent.
	#
	# The trade is over-denial on a path that genuinely contains a backslash.
	# That is legal but vanishingly rare on the systems a /bin/sh hook runs
	# on, it is refused with a reason rather than silently, and an escaped
	# spelling is not what a hand-editing agent produces by accident. Note
	# the scope: this reads the extracted argument, not the payload, so a
	# file's own contents may carry as many escapes as they like.
	case $path in
	*'\'*)
		printf '%s\n' \
			"tp scope fence: refusing \"$path\" - a write path carrying a backslash escape cannot be judged." \
			'The fence compares path spellings and never decodes JSON, so an escaped path could name' \
			"tp's own state while matching none of the fenced spellings. Write the path plainly, or make" \
			'the change through the tp command that owns the file (tp done / tp set / tp config / tp use).' >&2
		exit 2
		;;
	esac
	denied "$path" || continue
	[ -n "${roots+set}" ] || roots=$(repo_roots)
	in_repo "$path" || continue
	printf '%s\n' \
		"tp scope fence: $path is tp's own state and must not be hand-edited (v0.35.0 §6.2)." \
		'Change it with the tp command that owns it: task files through tp done / tp set / tp import / tp remove,' \
		'.tp/config.json through tp config, .tp/local.json through tp use, .tp/epics.json and .tp/epic-review/' \
		'through tp epic, and the round files under .tp-review/ through tp review / tp audit. A finding outside your scope belongs in the closure evidence, not in an edit.' >&2
	exit 2
done

exit 0
