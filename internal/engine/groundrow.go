package engine

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
	"strings"
)

// GroundRowError reports the cell of §7.2's table a row failed, naming the
// field.
//
// It is a typed error carrying the field rather than a formatted sentence
// because §7.2 states the rejection as "with the field named". As data that is
// a property of the value a caller reads back, so rewording the message cannot
// quietly stop it holding; a test asserting on the sentence would be pinning
// the wording instead of the property.
type GroundRowError struct {
	// Field is the key the failure is about, spelled as it is on the wire:
	// the §7.2 field whose cell failed, or — for a key that is no cell at all
	// — the offending key itself.
	Field string
	// Msg says what is wrong with that cell.
	Msg string
}

func (e *GroundRowError) Error() string {
	return fmt.Sprintf("field %q: %s", e.Field, e.Msg)
}

// groundRowErr is the only constructor for a cell failure, so a rejection that
// does not name a field is not reachable from this file.
func groundRowErr(field, msg string) error {
	return &GroundRowError{Field: field, Msg: msg}
}

// The shapes §7.2 gives three of its cells, each pinned to the form the
// emitting side already produces: `FloorIndexRow.ID` is
// `fmt.Sprintf("u%d", i+1)`, `FloorAnchorOf` returns "§" followed by a dotted
// number and `§0` before the first heading, and `FloorTextSHA` returns twelve
// lowercase hex characters. A row whose value cannot have come from the floor
// is rejected here rather than surviving into the record as a §8 join key that
// matches nothing.
var (
	groundUnitIDRe  = regexp.MustCompile(`^u[1-9]\d*$`)
	groundAnchorRe  = regexp.MustCompile(`^§\d+(\.\d+)*$`)
	groundTextSHARe = regexp.MustCompile(`^[0-9a-f]{12}$`)
)

// groundRowKeys is the exact set of top-level keys §7.2's table names; any
// other key is a rejection, on allowedRoleKeys' precedent (rolefile.go:18,
// which does the same for a role file's key set).
//
// The cell-by-cell validation below says nothing about a key that is no cell,
// so without this a misspelled `carried_from` is silently dropped — a row that
// records as decided while the thing it meant to say is gone. Every entry here
// is a field the parse above reads, and a test matches the set against §7.2's
// table parsed out of the spec in both directions, so a cell added there with
// no entry here fails rather than being rejected on every row that carries it.
var groundRowKeys = map[string]bool{
	"unit_id":      true,
	"anchor":       true,
	"text_sha":     true,
	"verdict":      true,
	"kind":         true,
	"tier":         true,
	"evidence":     true,
	"partial_kind": true,
	"held_at":      true,
	"causes":       true,
	"ordinal":      true,
	"carried_from": true,
	"note":         true,
}

// GroundCause is one entry of a `QUESTION` row's `causes` array: §6's
// falsifiable cause with the prediction that would remove it. The pair is a
// struct rather than free text because §6's whole point is that a record can be
// checked for a prediction.
//
// How many entries are legal, and that each must carry both halves, is §6's
// bound and is enforced in groundCauses below; a previous revision of this
// comment said neither was decided here, which stopped being true when the
// bound landed.
type GroundCause struct {
	Cause      string `json:"cause"`
	Prediction string `json:"prediction"`
}

// groundCausesMin and groundCausesMax are §6's bound: "name three to five
// falsifiable causes and rank them", restated as `causes`' meaning in §7.2's
// table and in §3's verdict table.
//
// Both ends are load-bearing and neither is a round number chosen for tidiness.
// The lower one is the rule itself — the point of §6 is that single-cause
// reasoning anchors on the first plausible idea, so a list of two is the
// failure mode with one extra entry. The upper one is what stops the list
// becoming a place to put everything that was thought of, which is not a
// ranking.
const (
	groundCausesMin = 3
	groundCausesMax = 5
)

// GroundRow is one line of `ground-round-N.ndjson`: §7.2's field table as a
// type.
//
// **Presence is encoded in the value, not in a parallel flag**, and that is
// what makes the conditional cells checkable. §7.2 has five cells whose
// legality depends on another cell being present — `evidence` on `tier`,
// `held_at` on `partial_kind`, `causes` on `verdict` — so a separate "was this
// key in the JSON" bitmask would be a second source of truth that can disagree
// with the fields beside it. Instead every optional cell has a zero value that
// is not a legal value: an enum's "" is outside its listing, a required string
// is rejected when present and empty, and `ordinal`/`carried_from` are 1-based
// so 0 is not a round or an index. The validator therefore reads the same
// struct a caller does, and there is no flag to leave stale.
//
// `unit_id` is the one exception and needs the pointer: `null` is a legal
// value there — it records a reader-added claim — and has to be distinguishable
// from a missing key, which is rejected. A row that exists has the key, so a
// nil UnitID always means the JSON said null.
type GroundRow struct {
	// UnitID is `u<N>` over every unit §2.1 produces, or nil for the `null`
	// of a reader-added claim.
	UnitID *string
	// Anchor is the `§n(.n)*` section the unit sits in.
	Anchor string
	// TextSHA is the first 12 hex of the sha256 of the unit's text as emitted.
	TextSHA string
	// Verdict is one of §3's six.
	Verdict GroundVerdict
	// Kind is one of §4.1's seven, "" when the row omits it.
	Kind GroundKind
	// Tier is the deepest tier the unit reached, "" when the row omits it.
	Tier GroundTier
	// Evidence is what a later reader re-runs, "" when the row omits it.
	Evidence string
	// PartialKind is which of §3's three shapes a PARTIAL row is, "" otherwise.
	PartialKind GroundPartialKind
	// HeldAt is the commit, tag or date a true-when-written claim held at.
	HeldAt string
	// Causes is §6's ranked causes, nil when the row omits the key.
	Causes []GroundCause
	// Ordinal is the 1-based index among units sharing this TextSHA.
	Ordinal int
	// CarriedFrom is the round a carried-forward disposition was first decided
	// in, 0 when the row omits it.
	CarriedFrom int
	// Note is free text.
	Note string
}

// ParseGroundRow decodes and validates one line of `ground-round-N.ndjson`
// against §7.2's table, returning a *GroundRowError naming the field of the
// first cell that fails.
//
// Decoding and validation are one call rather than two, so a GroundRow obtained
// from the wire has necessarily been through the table.
func ParseGroundRow(line []byte) (GroundRow, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(line, &raw); err != nil {
		return GroundRow{}, fmt.Errorf("row is not a JSON object: %w", err)
	}
	if raw == nil {
		return GroundRow{}, fmt.Errorf("row is not a JSON object: null")
	}

	if err := groundUnknownKey(raw); err != nil {
		return GroundRow{}, err
	}

	var row GroundRow
	if err := decodeGroundRowUnit(raw, &row); err != nil {
		return GroundRow{}, err
	}
	if err := decodeGroundRowDisposition(raw, &row); err != nil {
		return GroundRow{}, err
	}
	if err := validateGroundRowRequired(&row); err != nil {
		return GroundRow{}, err
	}
	if err := validateGroundRowConditional(&row); err != nil {
		return GroundRow{}, err
	}
	return row, nil
}

// decodeGroundRowUnit decodes the cells that identify the unit and place the
// row in the record: which unit it is, where that unit sits, and the §8
// bookkeeping.
func decodeGroundRowUnit(raw map[string]json.RawMessage, row *GroundRow) error {
	var err error
	if row.UnitID, err = groundUnitID(raw); err != nil {
		return err
	}
	if row.Anchor, err = groundText(raw, "anchor"); err != nil {
		return err
	}
	if row.TextSHA, err = groundText(raw, "text_sha"); err != nil {
		return err
	}
	if row.Ordinal, err = groundIndex(raw, "ordinal"); err != nil {
		return err
	}
	if row.CarriedFrom, err = groundIndex(raw, "carried_from"); err != nil {
		return err
	}
	if row.Note, err = groundFreeText(raw, "note"); err != nil {
		return err
	}
	return nil
}

// decodeGroundRowDisposition decodes the cells that carry the disposition
// itself and what it was reached by.
func decodeGroundRowDisposition(raw map[string]json.RawMessage, row *GroundRow) error {
	var err error
	if row.Verdict, err = groundEnumCell(raw, "verdict", ParseGroundVerdict); err != nil {
		return err
	}
	if row.Kind, err = groundEnumCell(raw, "kind", ParseGroundKind); err != nil {
		return err
	}
	if row.Tier, err = groundEnumCell(raw, "tier", ParseGroundTier); err != nil {
		return err
	}
	if row.Evidence, err = groundText(raw, "evidence"); err != nil {
		return err
	}
	if row.PartialKind, err = groundEnumCell(raw, "partial_kind", ParseGroundPartialKind); err != nil {
		return err
	}
	if row.HeldAt, err = groundText(raw, "held_at"); err != nil {
		return err
	}
	if row.Causes, err = groundCauses(raw); err != nil {
		return err
	}
	return nil
}

// validateGroundRowRequired enforces the four cells §7.2 requires on every row
// whatever the verdict, and the shape of the three that have one. `unit_id` is
// required too and is enforced in groundUnitID, where absence and `null` have
// to be told apart.
func validateGroundRowRequired(row *GroundRow) error {
	if row.Anchor == "" {
		return groundRowErr("anchor", "is required")
	}
	if !groundAnchorRe.MatchString(row.Anchor) {
		return groundRowErr("anchor", "must be a §n(.n)* section, as §0 is before the first heading")
	}
	if row.TextSHA == "" {
		return groundRowErr("text_sha", "is required")
	}
	if !groundTextSHARe.MatchString(row.TextSHA) {
		return groundRowErr("text_sha", "must be the first 12 lowercase hex characters of a sha256")
	}
	if row.Verdict == "" {
		return groundRowErr("verdict", "is required")
	}
	if row.Ordinal == 0 {
		return groundRowErr("ordinal", "is required")
	}
	return nil
}

// validateGroundRowConditional enforces §7.2's five conditional cells.
//
// `kind` and `tier` go through groundRowRequiredOnAClaim and the other three
// through groundRowIff, and the split is the table's own. The `required`
// column reads "iff verdict ≠ NOT-A-CLAIM" for both, but the `meaning` column
// overrides it in bold — they are **optional on NOT-A-CLAIM**, carried with
// `evidence` when the unit states a decision *and* a checked fact, which the
// first run had five of and had to bury in `note`. So they are required on a
// claim and forbidden nowhere.
//
// The other three are genuine two-sided cells, and one helper enforces both
// directions of all of them: the one-directional reading is then not something
// a caller can produce by leaving a branch out, because there is no branch here
// to leave out.
func validateGroundRowConditional(row *GroundRow) error {
	isClaim := row.Verdict != VerdictNotAClaim
	if err := groundRowRequiredOnAClaim("kind", row.Kind != "", isClaim); err != nil {
		return err
	}
	if err := groundRowRequiredOnAClaim("tier", row.Tier != "", isClaim); err != nil {
		return err
	}
	if err := groundRowIff("evidence", row.Evidence != "", row.Tier != "", "tier is present"); err != nil {
		return err
	}
	if err := groundRowIff("partial_kind", row.PartialKind != "", row.Verdict == VerdictPartial, "the verdict is PARTIAL"); err != nil {
		return err
	}
	if err := groundRowIff("held_at", row.HeldAt != "", row.PartialKind == PartialTrueWhenWritten, "partial_kind is true-when-written"); err != nil {
		return err
	}
	return groundRowIff("causes", row.Causes != nil, row.Verdict == VerdictQuestion, "the verdict is QUESTION")
}

// groundRowIff enforces a two-sided cell: present exactly when cond holds.
func groundRowIff(field string, present, cond bool, when string) error {
	if cond && !present {
		return groundRowErr(field, "is required when "+when)
	}
	if present && !cond {
		return groundRowErr(field, "is only legal when "+when)
	}
	return nil
}

// groundRowRequiredOnAClaim enforces a one-sided cell: required when the row is
// a claim, and legal either way on a NOT-A-CLAIM row.
func groundRowRequiredOnAClaim(field string, present, isClaim bool) error {
	if isClaim && !present {
		return groundRowErr(field, "is required unless the verdict is NOT-A-CLAIM")
	}
	return nil
}

// groundUnknownKey rejects a top-level key that is no cell of §7.2's table.
//
// The offender named is the lexically first of them rather than whichever the
// map happens to yield: Go randomises map iteration order, and an error whose
// message differs between two runs on one input is not something an operator
// can act on or a test can assert.
func groundUnknownKey(raw map[string]json.RawMessage) error {
	unknown := make([]string, 0, len(raw))
	for key := range raw {
		if !groundRowKeys[key] {
			unknown = append(unknown, key)
		}
	}
	if len(unknown) == 0 {
		return nil
	}
	slices.Sort(unknown)
	return groundRowErr(unknown[0], "is not a field §7.2's table names")
}

// groundUnitID decodes §7.2's one nullable cell. The key is required; its
// value may be `null`, which records a reader-added claim, and nil comes back
// only for that.
func groundUnitID(raw map[string]json.RawMessage) (*string, error) {
	msg, ok := raw["unit_id"]
	if !ok {
		return nil, groundRowErr("unit_id", "is required; a reader-added claim writes null, not nothing")
	}
	if bytes.Equal(bytes.TrimSpace(msg), []byte("null")) {
		return nil, nil
	}
	var s string
	if err := json.Unmarshal(msg, &s); err != nil {
		return nil, groundRowErr("unit_id", "must be a string or null")
	}
	if !groundUnitIDRe.MatchString(s) {
		return nil, groundRowErr("unit_id", "must be u<N> over every unit §2.1 produces, or null")
	}
	return &s, nil
}

// groundFreeText decodes a string cell §7.2 constrains only by type, returning
// "" when the key is absent. `note` is the only such cell.
func groundFreeText(raw map[string]json.RawMessage, field string) (string, error) {
	msg, ok := raw[field]
	if !ok {
		return "", nil
	}
	var s string
	if err := json.Unmarshal(msg, &s); err != nil {
		return "", groundRowErr(field, "must be a string")
	}
	return s, nil
}

// groundText decodes a string cell whose presence is load-bearing, returning ""
// when the key is absent.
//
// A present-but-blank value is rejected rather than read as absent, which keeps
// "" meaning exactly one thing — the key was not there — and so keeps the
// presence encoding GroundRow's own comment rests on.
//
// It buys less than it looks like, and the difference was measured rather than
// reasoned: on a row whose conditional cell is REQUIRED, reading a blank as
// absent rejects the same row and names the same field, because the cell is
// then missing. The two readings part only where the condition does not hold —
// `{"verdict": "PARTIAL", "partial_kind": "two-readings", "held_at": ""}` is
// accepted outright by a blank-reads-as-absent parser and rejected here. A
// draft of this comment claimed the bypass was `{"tier": "run", "evidence":
// ""}`; running the mutant showed that row is rejected either way.
func groundText(raw map[string]json.RawMessage, field string) (string, error) {
	s, err := groundFreeText(raw, field)
	if err != nil {
		return "", err
	}
	if _, present := raw[field]; present && strings.TrimSpace(s) == "" {
		return "", groundRowErr(field, "is present but blank")
	}
	return s, nil
}

// groundIndex decodes a 1-based integer cell, returning 0 when the key is
// absent. Both cells it serves — `ordinal` and `carried_from` — count from 1,
// so 0 is unambiguously "no value" rather than a value.
func groundIndex(raw map[string]json.RawMessage, field string) (int, error) {
	msg, ok := raw[field]
	if !ok {
		return 0, nil
	}
	var n int
	if err := json.Unmarshal(msg, &n); err != nil {
		return 0, groundRowErr(field, "must be an integer")
	}
	if n < 1 {
		return 0, groundRowErr(field, "must be 1 or more")
	}
	return n, nil
}

// groundEnumCell decodes one of §7.2's enum cells through that enum's own
// parse, returning the zero string when the key is absent. Routing every enum
// cell through the closed parses is what keeps an unrecognised value out of the
// record, where §8's counters could not read it back.
func groundEnumCell[T ~string](raw map[string]json.RawMessage, field string, parse func(string) (T, bool)) (T, error) {
	var zero T
	msg, ok := raw[field]
	if !ok {
		return zero, nil
	}
	var s string
	if err := json.Unmarshal(msg, &s); err != nil {
		return zero, groundRowErr(field, "must be a string")
	}
	v, ok := parse(s)
	if !ok {
		return zero, groundRowErr(field, fmt.Sprintf("%q is not one of the values the spec lists", s))
	}
	return v, nil
}

// groundCauses decodes the `causes` array, returning nil when the key is
// absent.
//
// An explicit `null` is rejected rather than read as absent — the same rule the
// enum and text cells follow, and the one §7.2 states only for `unit_id`, where
// null is declared legal. Measured, as with the blank strings: on a QUESTION
// row the two readings are indistinguishable, because the key is required there
// either way. They part on a row that is not a QUESTION, which is exactly the
// row a Go emitter produces by accident, since a nil slice marshals to `null`.
func groundCauses(raw map[string]json.RawMessage) ([]GroundCause, error) {
	msg, ok := raw["causes"]
	if !ok {
		return nil, nil
	}
	var cs []GroundCause
	if err := json.Unmarshal(msg, &cs); err != nil || cs == nil {
		return nil, groundRowErr("causes", "must be an array of {cause, prediction} objects")
	}
	if len(cs) < groundCausesMin || len(cs) > groundCausesMax {
		return nil, groundRowErr("causes", fmt.Sprintf(
			"must hold %d to %d causes, not %d", groundCausesMin, groundCausesMax, len(cs)))
	}
	for i, c := range cs {
		if err := groundCausePair(i, c); err != nil {
			return nil, err
		}
	}
	return cs, nil
}

// groundCausePair enforces that one entry is a `{cause, prediction}` pair
// rather than an object that merely decoded into one.
//
// Both halves are checked, because §6's rule is the pairing: "a cause with no
// prediction is a vibe", and the pair is a struct so that "a record can be
// checked for one". Absent and blank fail alike, for the reason groundText
// gives — and here the two are not even distinguishable to a reader of the
// record, since neither survives into the row as anything but "".
//
// This is also the only thing that catches a misspelled key inside an entry.
// §7.2's unknown-key rule is stated for top-level keys, so `{"cause": …,
// "predicton": …}` is not rejected as an unknown key; it is rejected because
// the entry then carries no prediction.
func groundCausePair(i int, c GroundCause) error {
	if strings.TrimSpace(c.Cause) == "" {
		return groundRowErr("causes", fmt.Sprintf("entry %d carries no cause", i+1))
	}
	if strings.TrimSpace(c.Prediction) == "" {
		return groundRowErr("causes", fmt.Sprintf(
			"entry %d carries no prediction, and a cause with no prediction is a vibe", i+1))
	}
	return nil
}
