# drop-batch-reads.awk - the batch classifier both write hooks share
# (pre-tool-use-write-deny.sh and pre-tool-use-role-write-allow.sh).
#
# It prints a codedbpro batch payload with every read operation cut out, and
# prints nothing and exits 1 when there is nothing it can safely cut. Each hook
# then judges whatever is left by its own rule, unchanged.
#
# Why it exists. codedbpro's batch is in both matchers because it carries
# writes, but its ops carry reads too: {"ops":[{"tool":"read","args":{...}}]}.
# Judging every path in the payload refused a batch that only read - a round
# snapshot at the deny fence, the spec a role was spawned to review at the role
# allowlist - while the same read made as its own call reaches neither hook,
# because neither matcher names a read tool. So a batch is judged op by op,
# each as the same call made alone would be: an op whose tool is a codedbpro
# reader is dropped, and everything else - every write, every op this cannot
# classify, every key outside the ops - is left for the hook to judge. The
# reader list is the codedbpro tools the matchers do not name. The op's `tool`
# is what separates a read from a write, never its argument key, because
# faster_search and replace both take `path`.
#
# Why it is one file. The two hooks need the same classification, and two
# copies of a tokenizer would be two places for the same hole. A hook that
# cannot find this file keeps its payload whole, so a missing classifier makes
# the hook as strict as it was before batches were read op by op, never looser
# (TestBatchReadingFailsClosedWithoutTheSharedClassifier).
#
# How it stays trustworthy without a JSON parser. It only ever removes. It
# removes an op only when it has walked the whole payload without surprise,
# the payload's own tool_name is the batch tool, the op sits directly in
# tool_input.ops, and the op names exactly one `tool` key, spelled plainly,
# whose value is a plain string on the reader list. Any doubt leaves the
# payload whole.
#
# How it stays inside the hooks' time bound. It splits on `"` once, so the cost
# is one pass over the payload: a string's contents are measured, never walked,
# and a quote escaped inside one - an odd run of backslashes before it -
# continues the string rather than ending it. Only the short stretches between
# strings are walked character by character.

# The walk keeps one frame per open container: its type (o/a), its role (top,
# ti = tool_input, ops, op, or none), the key it is on and what it expects next
# (k = key, c = colon, v = value, a = after a value).
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

# An op is cut only when it named exactly one `tool` key, its value is a plain
# string on the reader list, and none of its keys is spelled with an escape
# that could decode to a second `tool`.
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

# Anything else outside a string is whitespace or part of a number, true,
# false or null - a value, never a key.
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
			# A string's value is kept only when it is one piece; one that
			# spans an escaped quote cannot be any name compared below.
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
}
