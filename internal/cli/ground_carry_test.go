package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deligoez/tp/internal/engine"
)

// The three-round fixture §11 row 16 asks for, as three states of one spec.
//
// Four sentences do four different jobs across the three rounds, and each is a
// floor unit on §2.1's digit arm:
//
//   - the STABLE claim appears in rounds 2 and 3 unchanged, and is decided for
//     the first time in round 2 — so the disposition round 3 inherits for it
//     names round 2, under either reading of `carried_from`;
//   - the CHANGING claim appears in rounds 2 and 3 with an edit at byte 61, late
//     in a long sentence, leaving its id, its anchor and its ordinal alone;
//   - the REVERTED claim is decided in round 1, replaced in round 2, and back to
//     its round-1 wording in round 3;
//   - the FRESH claim exists only in round 3 and is what that round's payload
//     decides, since §7.1 refuses a record holding no rows.
//
// The headings produce no unit at all — not a cut one — so the floor is exactly
// the sentences, which is what lets this test state each round's floor size
// rather than count around the document's furniture. "Cut" was the wrong word
// here and the require below is what refutes it: a cut unit gets an index row
// with no hash and would still be counted, while a two-heading document indexes
// as 0 in floor and 0 cut. Measured, not read.
const (
	groundCarryStable   = "The stable claim measured 3 things."
	groundCarryPrefix60 = "The changing claim measured 5 things across the whole corpus"
	groundCarryChangedA = groundCarryPrefix60 + " of specs."
	groundCarryChangedB = groundCarryPrefix60 + ", of specs."
	groundCarryRevertR  = "The reverted claim measured 7 things."
	groundCarryRevertR2 = "The reverted claim measured 8 things."
	groundCarryFresh    = "The fresh claim measured 9 things."
)

const groundCarryRound1 = "# Fixture spec\n\n## First\n\n" + groundCarryRevertR + "\n"

const groundCarryRound2 = "# Fixture spec\n\n## First\n\n" +
	groundCarryStable + "\n\n" + groundCarryChangedA + "\n\n" + groundCarryRevertR2 + "\n"

const groundCarryRound3 = "# Fixture spec\n\n## First\n\n" +
	groundCarryChangedB + "\n\n" + groundCarryRevertR + "\n\n" +
	groundCarryStable + "\n\n" + groundCarryFresh + "\n"

// writeGroundCarrySpec puts one of the three states on disk as the spec tp is
// run against, so a round is emitted over the text that round is meant to read.
func writeGroundCarrySpec(t *testing.T, dir, text string) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte(text), 0o600))
}

// groundEmittedFloor reads back the index the emission froze for a round, which
// is the same artifact --record grades against.
func groundEmittedFloor(t *testing.T, dir string, round int) []engine.FloorIndexRow {
	t.Helper()
	data, err := os.ReadFile(groundStatePath(dir, floorFileName(round)))
	require.NoError(t, err)
	rows, err := engine.ParseFloorIndex(string(data))
	require.NoError(t, err)
	return rows
}

// floorFileName names an emitted floor without restating the format string the
// engine owns.
func floorFileName(round int) string {
	return filepath.Base(engine.GroundFloorPath("spec.md", round))
}

// groundUnitFor finds the index row for a unit's text, by the hash the engine
// derives from that text rather than by position.
//
// Position is exactly what this test must not depend on: `unit_id` is numbered
// over every unit §2.1 produces, so the stable claim is a different id in round
// 2 and in round 3 — which is the reason §8 joins on the hash at all.
func groundUnitFor(t *testing.T, floor []engine.FloorIndexRow, text string) engine.FloorIndexRow {
	t.Helper()
	want := engine.FloorTextSHA(text)
	for _, row := range floor {
		if row.TextSHA == want {
			return row
		}
	}
	require.FailNowf(t, "unit not in the emitted floor", "no row carries the hash of %q", text)
	return engine.FloorIndexRow{}
}

// groundCarryRowFor writes a legal §7.2 row for one emitted unit, copying the
// index row's own anchor, hash and ordinal the way the prompt tells a unit to.
//
// The verdict is FAIL and the evidence names the round, so a carried row is
// distinguishable from a fresh one by what it says as well as by
// `carried_from`: §8's carry makes an unrepaired FAIL permanent while its text
// stands, and that is the property this fixture is about.
func groundCarryRowFor(unit engine.FloorIndexRow, round int) string {
	row := map[string]any{
		"unit_id":  unit.ID,
		"anchor":   unit.Anchor,
		"text_sha": unit.TextSHA,
		"ordinal":  unit.Ordinal,
		"verdict":  "FAIL",
		"kind":     "document",
		"tier":     "read",
		"evidence": "read the snapshot of round " + strconv.Itoa(round),
	}
	line, err := json.Marshal(row)
	if err != nil {
		panic(err)
	}
	return string(line)
}

// groundRecordedRows reads a round file back as the rows a later reader sees.
func groundRecordedRows(t *testing.T, dir string, round int) []engine.GroundRow {
	t.Helper()
	data, err := os.ReadFile(groundStatePath(dir, filepath.Base(engine.GroundRoundPath("spec.md", round))))
	require.NoError(t, err)
	rows, err := engine.ParseGroundRows(data)
	require.NoError(t, err)
	return rows
}

// groundRowByUnit finds a recorded row by the unit it decides, or reports that
// none does.
func groundRowByUnit(rows []engine.GroundRow, id string) (engine.GroundRow, bool) {
	for i := range rows {
		if rows[i].UnitID != nil && *rows[i].UnitID == id {
			return rows[i], true
		}
	}
	return engine.GroundRow{}, false
}

// TestTheSecondPassCarriesOnOverThreeRounds is §11 row 16, end to end through
// the command.
//
// Three rounds are what separate the two readings row 16 names as mutants. A
// join on `anchor` alone carries the CHANGING claim, whose anchor never moved; a
// lookup across every earlier round resurrects the REVERTED claim, decided in
// round 1 and absent from round 2. Neither is visible in two rounds, and both
// are visible here in the same assertion — the set of ids round 3's file
// decides.
//
// Each round is emitted over the text of its own state of the spec, so the join
// runs against floors tp itself derived rather than against index rows a test
// wrote, and the fixture's premises — the edit at byte 61, each round's floor
// size — are required rather than assumed.
func TestTheSecondPassCarriesOnOverThreeRounds(t *testing.T) {
	require.Len(t, groundCarryPrefix60, 60, "the shared prefix must be exactly 60 bytes")
	require.Equal(t, groundCarryChangedA[:60], groundCarryChangedB[:60],
		"the changing claim's two wordings agree up to byte 60")
	require.NotEqual(t, groundCarryChangedA[60], groundCarryChangedB[60], "and differ at byte 61")

	dir := t.TempDir()

	// Round 1 decides the reverted claim, in the wording round 3 comes back to.
	writeGroundCarrySpec(t, dir, groundCarryRound1)
	groundEmit(t, dir)
	floor1 := groundEmittedFloor(t, dir, 1)
	revert1 := groundUnitFor(t, floor1, groundCarryRevertR)
	rows1 := writeGroundRows(t, dir, groundCarryRowFor(revert1, 1))
	_, stderr, code := runTP(t, dir, "ground", "spec.md", "--record", rows1)
	require.Equal(t, 0, code, "stderr: %s", stderr)

	// Round 2 decides its whole floor. Nothing carries into it: the reverted
	// claim's round-1 wording is not in round 2's text at all.
	writeGroundCarrySpec(t, dir, groundCarryRound2)
	groundEmit(t, dir)
	floor2 := groundEmittedFloor(t, dir, 2)
	require.Len(t, floor2, 3, "round 2's floor is the three sentences; the headings produce no unit")
	rows2 := writeGroundRows(t, dir,
		groundCarryRowFor(groundUnitFor(t, floor2, groundCarryStable), 2),
		groundCarryRowFor(groundUnitFor(t, floor2, groundCarryChangedA), 2),
		groundCarryRowFor(groundUnitFor(t, floor2, groundCarryRevertR2), 2),
	)
	stdout, stderr, code := runTP(t, dir, "ground", "spec.md", "--record", rows2)
	require.Equal(t, 0, code, "stderr: %s", stderr)
	var round2 map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &round2))
	assert.Equal(t, float64(0), round2["carried"],
		"round 1 decided one unit and round 2's text does not hold it, so nothing carries")

	// Round 3 decides the fresh claim alone. Everything else is §8's business.
	writeGroundCarrySpec(t, dir, groundCarryRound3)
	groundEmit(t, dir)
	floor3 := groundEmittedFloor(t, dir, 3)
	require.Len(t, floor3, 4, "round 3's floor is its four sentences")
	stable3 := groundUnitFor(t, floor3, groundCarryStable)
	changed3 := groundUnitFor(t, floor3, groundCarryChangedB)
	revert3 := groundUnitFor(t, floor3, groundCarryRevertR)
	fresh3 := groundUnitFor(t, floor3, groundCarryFresh)
	rows3 := writeGroundRows(t, dir, groundCarryRowFor(fresh3, 3))
	stdout, stderr, code = runTP(t, dir, "ground", "spec.md", "--record", rows3)
	require.Equal(t, 0, code, "stderr: %s", stderr)

	var round3 map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &round3))
	assert.Equal(t, float64(1), round3["rows"], "the unit wrote one row")
	assert.Equal(t, float64(1), round3["carried"], "and inherited exactly one")

	recorded := groundRecordedRows(t, dir, 3)
	require.Len(t, recorded, 2, "round 3's own file holds both, rather than computing one at --status")

	carried, ok := groundRowByUnit(recorded, stable3.ID)
	require.True(t, ok, "the unchanged claim's disposition is in round 3's file")
	assert.Equal(t, engine.VerdictFail, carried.Verdict, "an unrepaired FAIL stands while its text stands")
	assert.Equal(t, engine.TierRead, carried.Tier, "its original tier")
	assert.Equal(t, "read the snapshot of round 2", carried.Evidence, "and its original evidence")
	assert.Equal(t, 2, carried.CarriedFrom, "carried_from names the round the disposition was made in")

	_, ok = groundRowByUnit(recorded, changed3.ID)
	assert.False(t, ok, "a claim edited at byte 61 is uncovered, however unchanged its anchor")
	_, ok = groundRowByUnit(recorded, revert3.ID)
	assert.False(t, ok,
		"a wording last decided in round 1 and absent from round 2 is re-decided, not resurrected")

	own, ok := groundRowByUnit(recorded, fresh3.ID)
	require.True(t, ok, "the payload's own row is there too")
	assert.Zero(t, own.CarriedFrom, "and carries no carried_from, because nobody decided it before")

	stdout, stderr, code = runTP(t, dir, "ground", "spec.md", "--status")
	require.Equal(t, 0, code, "stderr: %s", stderr)
	var status map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &status))
	assert.Equal(t, float64(4), status["emitted"])
	assert.Equal(t, float64(2), status["dispositioned"],
		"one decided this round and one carried; the changed and the reverted claims are open")
}

// groundRecordSecondRound emits a second round over the same spec and records
// one row against a unit of its floor, returning the invocation's streams and
// exit code.
//
// The payload is impeccable by construction — the row is built from the floor tp
// itself emitted — so any refusal the caller sees is about something else.
func groundRecordSecondRound(t *testing.T, dir string) (stdout, stderr string, code int) {
	t.Helper()
	groundEmit(t, dir)
	floor := groundEmittedFloor(t, dir, 2)
	require.NotEmpty(t, floor, "the second round must have a floor to record against")
	rows := writeGroundRows(t, dir, groundCarryRowFor(floor[0], 2))
	return runTP(t, dir, "ground", "spec.md", "--record", rows)
}

// TestACorruptPrecedingRoundIsExitThreeAndNotExitOne pins the ORDER of the two
// branches --record's error mapping has, which is the only thing that separates
// them.
//
// A truncated round file fails to parse as a *GroundLineError, and §8 wraps that
// in a *GroundCarryError because the file is tp's own. Both types are reachable
// through errors.As from the same error, so a mapping that asks about the line
// error first answers 1 — "a row fails validation" — and sends the operator to
// fix a line of a file they did not write. The verdict rests on the envelope's
// own `code` field and on the round file not appearing, never on the wording.
func TestACorruptPrecedingRoundIsExitThreeAndNotExitOne(t *testing.T) {
	dir := t.TempDir()
	writeGroundCarrySpec(t, dir, groundCarryRound2)
	groundEmit(t, dir)
	floor := groundEmittedFloor(t, dir, 1)
	require.NotEmpty(t, floor)
	rows := writeGroundRows(t, dir, groundCarryRowFor(floor[0], 1))
	_, controlStderr, code := runTP(t, dir, "ground", "spec.md", "--record", rows)
	require.Equal(t, 0, code,
		"the control: round 1 records cleanly, so round 2's refusal is about round 1's FILE. stderr: %s", controlStderr)

	roundFile := groundStatePath(dir, filepath.Base(engine.GroundRoundPath("spec.md", 1)))
	require.NoError(t, os.WriteFile(roundFile, []byte("{\n"), 0o600))

	_, stderr, code := groundRecordSecondRound(t, dir)

	assert.Equal(t, 3, code, "tp's own broken artifact is a file failure, not a rejected payload")
	assert.Equal(t, float64(3), groundErrorEnvelope(t, stderr)["code"])
	_, statErr := os.Stat(groundStatePath(dir, filepath.Base(engine.GroundRoundPath("spec.md", 2))))
	assert.True(t, os.IsNotExist(statErr),
		"and no round file is written, because a round that could not inherit would claim coverage it does not have")
}

// TestAFloorThatDoesNotParseIsRefused is the other artifact --record now reads
// rather than merely opens.
//
// §8's carry asks, of each emitted floor unit, whether the preceding round
// decided the same text — so the index has to be read back, and a half-written
// one read short would shrink §8's denominator silently and make coverage look
// higher than it is. The fixture breaks the floor of the round being recorded,
// which is the one the rows are graded against.
func TestAFloorThatDoesNotParseIsRefused(t *testing.T) {
	dir := t.TempDir()
	writeGroundCarrySpec(t, dir, groundCarryRound2)
	groundEmit(t, dir)
	floor := groundEmittedFloor(t, dir, 1)
	require.NotEmpty(t, floor)
	rows := writeGroundRows(t, dir, groundCarryRowFor(floor[0], 1))

	floorFile := groundStatePath(dir, floorFileName(1))
	require.NoError(t, os.WriteFile(floorFile, []byte("u1 §0 not-an-index-row\n"), 0o600))

	_, stderr, code := runTP(t, dir, "ground", "spec.md", "--record", rows)

	assert.Equal(t, 3, code, "an index tp cannot read back is a file failure")
	assert.Equal(t, float64(3), groundErrorEnvelope(t, stderr)["code"])
	_, statErr := os.Stat(groundStatePath(dir, filepath.Base(engine.GroundRoundPath("spec.md", 1))))
	assert.True(t, os.IsNotExist(statErr), "and the round is not recorded against a floor tp could not read")
}
