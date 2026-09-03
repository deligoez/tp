package engine

import "slices"

// GroundVerdict is one of §3's six dispositions a `ground-round-N.ndjson` row
// carries. The string values are the wire form: they are what a unit writes
// into the row and what §8's per-verdict breakdown counts, so they are the
// table's spellings byte for byte, upper case and hyphen included.
//
// `NOT-A-CLAIM` is one of the six rather than an omission. §2.2 obliges a
// disposition for every emitted floor unit and §2.1's arms have high recall by
// design, so the floor holds units that assert nothing about the world; without
// a recordable value for those, a conforming run leaves them blank and every
// spec is permanently uncovered.
type GroundVerdict string

const (
	VerdictPass         GroundVerdict = "PASS"
	VerdictPartial      GroundVerdict = "PARTIAL"
	VerdictFail         GroundVerdict = "FAIL"
	VerdictUnverifiable GroundVerdict = "UNVERIFIABLE"
	VerdictQuestion     GroundVerdict = "QUESTION"
	VerdictNotAClaim    GroundVerdict = "NOT-A-CLAIM"
)

// groundVerdictOrder lists the six verdicts in §3's table order. It is the
// single source for both the exported listing and the parse, so a verdict can
// never be accepted without being listed.
var groundVerdictOrder = []GroundVerdict{
	VerdictPass,
	VerdictPartial,
	VerdictFail,
	VerdictUnverifiable,
	VerdictQuestion,
	VerdictNotAClaim,
}

// GroundKind is one of §4.1's seven claim kinds — what the claim is about. The
// kind is what decides which tiers of evidence say anything about it, and a row
// names it so a test can read it back; a prose description cannot be asserted
// over.
type GroundKind string

const (
	KindDocument      GroundKind = "document"
	KindCodeStructure GroundKind = "code-structure"
	KindCorpus        GroundKind = "corpus"
	KindBehaviour     GroundKind = "behaviour"
	KindMechanism     GroundKind = "mechanism"
	KindDefect        GroundKind = "defect"
	KindGuard         GroundKind = "guard"
)

// groundKindOrder lists the seven kinds in §4.1's table order.
var groundKindOrder = []GroundKind{
	KindDocument,
	KindCodeStructure,
	KindCorpus,
	KindBehaviour,
	KindMechanism,
	KindDefect,
	KindGuard,
}

// GroundTier is one of §4.1's six evidence tiers — what the unit actually did.
//
// The type is a string and not an integer on purpose. An integer enum gives
// every tier an ordinal, and §4.1 says in bold that **the tiers are not
// ordered**: past `run`, a "deeper" tier is not more of the same evidence but
// evidence about a different subject, since `probe`, `red-green` and
// `break-and-control` rank rigour on an artifact the unit *built* while `read`,
// `query` and `run` examine the real one. The mutant §11 row 6 names — accept
// anything at or above the kind's first listed tier — is arithmetic on exactly
// such an ordinal. A string tier has none to compare.
type GroundTier string

const (
	TierRead            GroundTier = "read"
	TierQuery           GroundTier = "query"
	TierRun             GroundTier = "run"
	TierProbe           GroundTier = "probe"
	TierRedGreen        GroundTier = "red-green"
	TierBreakAndControl GroundTier = "break-and-control"
)

// groundTierOrder lists the six tiers in §4.1's table order.
//
// Table order, and nothing more. It is what renders the enum for a reader; it
// is deliberately not what TierAcceptableFor reads, which is a per-kind set.
var groundTierOrder = []GroundTier{
	TierRead,
	TierQuery,
	TierRun,
	TierProbe,
	TierRedGreen,
	TierBreakAndControl,
}

// groundAcceptableTiers is §4.1's third column: for each kind, the SET of tiers
// that say anything about a claim of that kind.
//
// A set, spelled as a set. There is no per-kind slice here and so no "the
// kind's first listed tier" for a comparison to reach for — the ordering §11
// row 6 rules out is not merely unused, it has nothing to be written against.
// The sets are the whole rule: a tier is acceptable for a kind or it is not.
//
// Where the entries come from, one line per §4.1 row: running a command says
// nothing about what a text contains, so `document` takes only `read`; a
// call-graph or dead-code tool is a query over the tree, so `code-structure`
// takes `read` and `query`; reading a command is not running it, so `behaviour`
// refuses `read`; a claim about a mechanism that does not exist yet is settled
// by building a probe; and a guard that passes proves nothing until its subject
// is broken and the control is run.
var groundAcceptableTiers = map[GroundKind]map[GroundTier]bool{
	KindDocument:      {TierRead: true},
	KindCodeStructure: {TierRead: true, TierQuery: true},
	KindCorpus:        {TierQuery: true},
	KindBehaviour:     {TierRun: true, TierRedGreen: true},
	KindMechanism:     {TierProbe: true},
	KindDefect:        {TierRedGreen: true},
	KindGuard:         {TierBreakAndControl: true},
}

// GroundVerdicts returns the six verdicts in §3's table order, for callers that
// need to name the set rather than test one value. The copy keeps a caller from
// reordering the sequence every other reader depends on.
func GroundVerdicts() []GroundVerdict { return groundEnumListing(groundVerdictOrder) }

// GroundKinds returns the seven kinds in §4.1's table order, as a copy.
func GroundKinds() []GroundKind { return groundEnumListing(groundKindOrder) }

// GroundTiers returns the six tiers in §4.1's table order, as a copy.
//
// The order is the order §4.1 prints them in and carries no rank. Nothing in
// this package decides acceptability from a tier's position in it.
func GroundTiers() []GroundTier { return groundEnumListing(groundTierOrder) }

// ParseGroundVerdict maps a recorded row's `verdict` onto the enum, reporting
// false for anything outside §3's six.
func ParseGroundVerdict(s string) (GroundVerdict, bool) {
	return parseGroundEnum(groundVerdictOrder, s)
}

// ParseGroundKind maps a recorded row's `kind` onto the enum, reporting false
// for anything outside §4.1's seven.
func ParseGroundKind(s string) (GroundKind, bool) {
	return parseGroundEnum(groundKindOrder, s)
}

// ParseGroundTier maps a recorded row's `tier` onto the enum, reporting false
// for anything outside §4.1's six.
func ParseGroundTier(s string) (GroundTier, bool) {
	return parseGroundEnum(groundTierOrder, s)
}

// TierAcceptableFor reports whether tier is in the set §4.1 grants kind.
//
// It takes the typed enums rather than two strings, so the only way a caller
// reaches this predicate with a value off the wire is through the Parse
// functions above: that is where the enums close, and a row carrying an unknown
// kind or tier is rejected before it gets here. A value outside either enum
// answers false — the safe direction, since nothing unrecognised should be
// certified as evidence.
//
// This is the whole of the kind–tier rule. Which verdicts must satisfy it is
// §7.2's separate question and is decided elsewhere.
func TierAcceptableFor(kind GroundKind, tier GroundTier) bool {
	return groundAcceptableTiers[kind][tier]
}

// groundEnumListing copies an enum's ordered listing, so a caller mutating what
// it got back cannot reorder the package's own table.
func groundEnumListing[T ~string](order []T) []T {
	out := make([]T, len(order))
	copy(out, order)
	return out
}

// parseGroundEnum maps a wire value onto one of order's members.
//
// One parse for all three enums, so they cannot drift into different rules. It
// does not trim or case-fold: the values are the ones tp emits in the prompt's
// own schema, so a near-miss like "pass" is a unit's bug and surfacing it is
// the point — silently accepting it would let a row into the permanent record
// that §8's counter then cannot read back.
func parseGroundEnum[T ~string](order []T, s string) (T, bool) {
	v := T(s)
	if slices.Contains(order, v) {
		return v, true
	}
	var zero T
	return zero, false
}
