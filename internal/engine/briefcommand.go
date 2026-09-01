package engine

import "fmt"

// BriefCommand returns the command that emits this unit's brief — the
// `brief_command` the oracle supplies for the kind (§3.3.1, §4.1).
//
// The driver never executes this itself and never reads its output: it hands
// the string to the child, which runs it as its first act and then does that
// one unit. That is why the round-scoped kinds interpolate `$TP_ROUND_DIR`
// rather than a resolved path — the variable is already in the child's
// environment (§3.1.1), so the command reads the same in a run state, a log and
// a `next_units` entry whatever run it belongs to.
//
// A kind outside the eight returns the empty string, the same defensive
// direction DurableWrite takes: there is no brief for a unit tp cannot name.
func (k UnitKind) BriefCommand(t UnitTarget) string {
	switch k {
	case UnitImplement:
		return "tp next --brief"
	case UnitReviewRole:
		return "tp review " + t.Spec + roleArg(t.ID)
	case UnitAuditRole:
		return "tp audit " + t.Spec + roleArg(t.ID)
	case UnitReviewRecord:
		return recordBriefCommand("review", t.Spec)
	case UnitAuditRecord:
		return recordBriefCommand("audit", t.Spec)
	case UnitReviewResolve:
		return "tp review " + t.Spec + " --status"
	case UnitAuditFix:
		return "tp audit " + t.Spec + " --status"
	case UnitDecompose:
		return "tp resume"
	default:
		return ""
	}
}

// roleArg renders the --role argument a role unit's brief carries (v0.36.0
// §4.2.3), or nothing when the target names no id.
//
// Passing the name is what makes the flag reachable at all: without it every
// unit runs the bare command, receives every role's prompt and reads one, which
// is the cost §4 measures. The empty case is not decoration — UnitTarget is
// built by more than one caller, and a dangling `--role` would be worse than
// the whole panel.
func roleArg(id string) string {
	if id == "" {
		return ""
	}
	return " --role " + id
}

// recordBriefCommand renders the two-step brief the record kinds share, with
// phase naming the tp subcommand ("review" or "audit").
//
// One unit owns both steps because the merge produces the very file the record
// consumes. The merge is guarded by the merged file's absence rather than run
// unconditionally: a record unit that is retried, or a round resumed after an
// interruption, must not merge over the dispositions §6.3 says `merged.ndjson`
// accumulates — `review-resolve` and `audit-fix` write their `resolved` objects
// into that same file, and a second merge would replace them with fresh
// undisposed rows (test 57).
//
// The `role-*.ndjson` glob is deliberately unquoted so the shell expands it;
// every other occurrence names one file.
func recordBriefCommand(phase, spec string) string {
	merged := "$TP_ROUND_DIR/" + mergedFindingsName
	return fmt.Sprintf(
		"[ -f %s ] || tp %s --merge $TP_ROUND_DIR/role-*.ndjson -o %s; tp %s %s --record %s",
		merged, phase, merged, phase, spec, merged,
	)
}

// Succeeded reports whether one attempt at a unit of this kind succeeded: the
// child exited 0 **and** the kind's durable write is present (§3.3.1). Either
// alone is a failed attempt — an exit 0 with nothing written is a child that
// gave up politely, and a non-zero exit over a complete write is a harness
// failure after the fact; the driver retries both (test 51).
//
// The driver reads these two and never the unit's output, so this is the whole
// success test.
func (k UnitKind) Succeeded(exitCode int, t UnitTarget) bool {
	return exitCode == 0 && k.DurableWrite(t)
}
