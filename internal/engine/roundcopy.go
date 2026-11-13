package engine

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strconv"

	"github.com/deligoez/tp/internal/model"
)

// RoundStateListsFile reports whether path is a recorded round: a file the
// state.json beside it lists as a round's file, in either phase. The answer
// never depends on the active pointer or the working directory, so a
// disposition written into any other file (a driver round's merged.ndjson, a
// copy under /tmp) is known to change no round's verdict.
func RoundStateListsFile(path string) bool {
	abs, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	st, ok := readStateFile(filepath.Join(filepath.Dir(abs), stateFileName))
	if !ok {
		return false
	}
	name := filepath.Base(abs)
	for _, rounds := range [][]ReviewRound{st.ReviewRounds, st.AuditRounds} {
		for i := range rounds {
			if rounds[i].File == name {
				return true
			}
		}
	}
	return false
}

// ReviewResolveOrder is the order a disposition written now into path takes
// among the spec's review rounds (spec/backlog/carried-rows-block.md §2.6):
// the highest review round recorded or emitted in path's state directory. A
// fix written under it is verified once a round numbered above it is
// recorded, so a round already emitted when the fix was made never verifies
// it. false when path is not a recorded review round file.
func ReviewResolveOrder(path string) (int, bool) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return 0, false
	}
	dir := filepath.Dir(abs)
	st, ok := readStateFile(filepath.Join(dir, stateFileName))
	if !ok {
		return 0, false
	}
	name := filepath.Base(abs)
	listed := false
	for i := range st.ReviewRounds {
		listed = listed || st.ReviewRounds[i].File == name
	}
	if !listed {
		return 0, false
	}
	order := latestRecordedRound(st.ReviewRounds)
	if _, err := os.Stat(filepath.Join(dir, snapshotFilename(PhaseReview, len(st.ReviewRounds)+1))); err == nil {
		order = max(order, len(st.ReviewRounds)+1)
	}
	return order, true
}

// DriverRoundCopy reports whether rows are an exact copy, dispositions set
// aside, of the recorded round a driver unit is working: round TP_ROUND of
// phase, for the spec TP_FILE's task file tracks. Inside that round the record
// unit rewrites the round in place (RecordRound), so re-recording such a copy
// after its findings are disposed is the next step. Any other file, or any
// call outside a driver round, would append a round built from rows no
// emission produced, or replace the round's rows with others.
func DriverRoundCopy(phase string, rows []map[string]any) bool {
	round, err := strconv.Atoi(os.Getenv(EnvRound))
	if err != nil || round < 1 {
		return false
	}
	taskFile := os.Getenv(EnvTaskFile)
	if taskFile == "" {
		return false
	}
	tf, err := model.ReadTaskFile(taskFile)
	if err != nil {
		return false
	}
	spec, ok := ResolveSpecPath(taskFile, tf.Spec)
	if !ok {
		return false
	}
	recorded, ok := readNDJSONRows(roundFilePath(spec, phase, round))
	if !ok {
		return false
	}
	return slices.Equal(rowsWithoutDispositions(rows), rowsWithoutDispositions(recorded))
}

// readStateFile parses one state.json, reporting false when it cannot.
func readStateFile(path string) (*ReviewState, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}
	var st ReviewState
	if json.Unmarshal(data, &st) != nil {
		return nil, false
	}
	return &st, true
}

// rowsWithoutDispositions renders each row as its JSON encoding (sorted keys)
// with the `resolved` disposition removed, since a disposition is exactly what
// a resolve adds to a copy of a recorded round.
func rowsWithoutDispositions(rows []map[string]any) []string {
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		c := make(map[string]any, len(r))
		for k, v := range r {
			if k != "resolved" {
				c[k] = v
			}
		}
		data, _ := json.Marshal(c)
		out = append(out, string(data))
	}
	return out
}
