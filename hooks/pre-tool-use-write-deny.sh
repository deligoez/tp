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

# drop_batch_reads prints the payload with every read operation of a batch cut
# out, and prints nothing and exits 1 when there is nothing it can safely cut.
#
# codedbpro's batch is in the matcher because it carries writes, but its ops
# carry reads too: {"ops":[{"tool":"read","args":{"file":...}}, ...]}. Judging
# every path in the payload refused a batch that only read a round snapshot -
# the file the ground prompt tells a grader to read - while the same read made
# as its own call never reaches this hook, because the matcher names no read
# tool. So a batch is judged op by op, each as the same call made alone would
# be: an op whose tool is a codedbpro reader is dropped before the extraction
# below runs, and everything else - every write, every op it cannot classify,
# every key outside the ops - is judged exactly as before. The reader list is
# the codedbpro tools the matcher does not name; the tool that separates a
# read from a write is the op's `tool`, never its argument key, because
# faster_search and replace both take `path`.
#
# Telling which op a path belongs to needs structure a grep cannot see, so
# this walks the payload's JSON tokens. It stays within what the fence can
# trust: it only ever removes, it removes an op only when it has read the
# whole payload without surprise and the op names exactly one tool, spelled
# plainly, from the reader list; any doubt leaves the payload whole and the
# hook as strict as it was. It splits on `"` once, so the cost is one pass
# over the payload: a string's contents are measured, never walked, and a
# quote escaped inside one - an odd run of backslashes before it - continues
# the string rather than ending it.
drop_batch_reads() {
	awk '
	# The walk keeps one frame per open container: its type (o/a), its role
	# (top, ti = tool_input, ops, op, or none), the key it is on and what it
	# expects next (k = key, c = colon, v = value, a = after a value).
	function push(c, p,    role) {
		role = ""
		if (d > 0 && ty[d] == "o") {
			if (ex[d] != "v") { ok = 0; return }
			ex[d] = "a"
			if (fr[d] == "top" && key[d] == "tool_input" && c == "{") role = "ti"
			else if (fr[d] == "ti" && key[d] == "ops" && c == "[") role = "ops"
		} else if (d > 0 && fr[d] == "ops" && c == "{") {
			role = "op"; opstart = p; optool = ""; toolkeys = 0; escaped = 0
		} else if (d == 0 && c == "{") role = "top"
		d++; ty[d] = (c == "{") ? "o" : "a"; fr[d] = role; ex[d] = "k"; key[d] = ""
	}
	# An op is cut only when it named exactly one `tool` key, its value is a
	# plain string on the reader list, and none of its keys is spelled with an
	# escape that could decode to a second `tool`.
	function pop(c, p) {
		if (d == 0 || (c == "}") != (ty[d] == "o")) { ok = 0; return }
		if (ty[d] == "o" && ex[d] != "k" && ex[d] != "a") { ok = 0; return }
		if (fr[d] == "op" && toolkeys == 1 && !escaped && (optool in readop)) {
			spans++; sa[spans] = opstart; sb[spans] = p
		}
		d--
	}
	function str(v) {
		if (d == 0) { ok = 0; return }
		if (ty[d] != "o") return
		if (ex[d] == "k") {
			key[d] = v; ex[d] = "c"
			if (fr[d] == "op" && v == "tool") toolkeys++
			if (fr[d] == "op" && index(v, "\\")) escaped = 1
			if (fr[d] == "top" && v == "tool_name") names++
		} else if (ex[d] == "v") {
			ex[d] = "a"
			if (fr[d] == "top" && key[d] == "tool_name") toolname = v
			if (fr[d] == "op" && key[d] == "tool") optool = v
		} else ok = 0
	}
	# Anything else outside a string is whitespace or part of a number,
	# true, false or null - a value, never a key.
	function other(c) {
		if (c == " " || c == "\t" || c == "\r" || c == "\n") return
		if (d == 0) { ok = 0; return }
		if (ty[d] != "o") return
		if (ex[d] == "v") ex[d] = "a"
		else if (ex[d] != "a") ok = 0
	}
	BEGIN {
		nr = split("read faster_search meta_search diff lint memo", r, " ")
		for (i = 1; i <= nr; i++) readop[r[i]] = 1
	}
	{ s = (NR == 1) ? $0 : s "\n" $0 }
	END {
		n = split(s, q, "\"")
		ok = 1; d = 0; pos = 1; instr = 0; spans = 0; names = 0; toolname = ""
		for (i = 1; i <= n && ok; i++) {
			piece = q[i]; len = length(piece)
			if (instr) {
				sval = (spieces++ == 0) ? piece : "\001"
				bs = 0
				while (bs < len && substr(piece, len - bs, 1) == "\\") bs++
				if (bs % 2 == 1 && i < n) {
					# an escaped quote: the string goes on into the next piece
				} else if (i == n) ok = 0
				else { instr = 0; str(sval) }
			} else {
				for (j = 1; j <= len && ok; j++) {
					c = substr(piece, j, 1)
					if (c == "{" || c == "[") push(c, pos + j - 1)
					else if (c == "}" || c == "]") pop(c, pos + j - 1)
					else if (c == ":") { if (d == 0 || ty[d] != "o" || ex[d] != "c") ok = 0; else ex[d] = "v" }
					else if (c == ",") { if (d == 0) ok = 0; else if (ty[d] == "o") { if (ex[d] != "a") ok = 0; else ex[d] = "k" } }
					else other(c)
				}
				if (i < n) { instr = 1; spieces = 0 }
			}
			pos += len + 1
		}
		if (instr || d != 0) ok = 0
		if (!ok || names != 1 || toolname != "mcp__codedbpro__batch" || spans == 0) exit 1
		out = ""; from = 1
		for (k = 1; k <= spans; k++) { out = out substr(s, from, sa[k] - from); from = sb[k] + 1 }
		printf "%s", out substr(s, from)
	}'
}

# Only a batch is walked, so every other tool's payload reaches the extraction
# byte for byte. A walk that fails or finds nothing to drop leaves `payload`
# as it arrived: the pass can make the fence less strict only by the reads it
# positively identified.
case $payload in
*'"mcp__codedbpro__batch"'*)
	if reduced=$(printf '%s\n' "$payload" | drop_batch_reads); then
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
	printf '%s\n' \
		"tp scope fence: $path is tp's own state and must not be hand-edited (v0.35.0 §6.2)." \
		'Change it with the tp command that owns it: task files through tp done / tp set / tp import / tp remove,' \
		'.tp/config.json through tp config, .tp/local.json through tp use, and the round files under .tp-review/' \
		'through tp review / tp audit. A finding outside your scope belongs in the closure evidence, not in an edit.' >&2
	exit 2
done

exit 0
