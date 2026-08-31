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
# remainder, so the `.tp-review` directory itself is not a denied write - its
# contents are. The patterns are all lowercase because they are matched against
# the normalized spelling, never the raw argument.
denied() {
	case $(normalize "$1") in
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
	grep -Eo '"(file_path|notebook_path|file)"[[:space:]]*:[[:space:]]*"[^"]*"' |
	sed -e 's/^[^:]*:[[:space:]]*"//' -e 's/"$//')

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
	printf '%s\n' \
		"tp scope fence: $path is tp's own state and must not be hand-edited (v0.35.0 §6.2)." \
		'Change it with the tp command that owns it: task files through tp done / tp set / tp import / tp remove,' \
		'.tp/config.json through tp config, .tp/local.json through tp use, and the round files under .tp-review/' \
		'through tp review / tp audit. A finding outside your scope belongs in the closure evidence, not in an edit.' >&2
	exit 2
done

exit 0
