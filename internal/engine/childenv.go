package engine

import (
	"crypto/rand"
	"maps"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"
)

// The child environment of §3.1.1: every unit tp run spawns carries these, so a
// unit can address its own artifacts without the driver passing them through
// prose. TP_ROUND and TP_ROUND_DIR are the exception — they exist only in a
// round-based phase — and TP_UNATTENDED is §5.1's fence, applied on the same
// terms as the rest.
const (
	EnvRunID      = "TP_RUN_ID"
	EnvRunDir     = "TP_RUN_DIR"
	EnvRoundDir   = "TP_ROUND_DIR"
	EnvRound      = "TP_ROUND"
	EnvTaskFile   = "TP_FILE"
	EnvUnitID     = "TP_UNIT_ID"
	EnvUnitKind   = "TP_UNIT_KIND"
	EnvUnitSeq    = "TP_UNIT_SEQ"
	EnvPhase      = "TP_PHASE"
	EnvUnattended = "TP_UNATTENDED"
)

// UnitEnv is one unit attempt's identity as the driver hands it to a child: the
// run it belongs to, the cycle it works on, and the unit itself.
//
// Round is a pointer because the oracle reports `round` null outside a
// round-based phase, and null is what makes TP_ROUND and TP_ROUND_DIR absent
// rather than empty (§3.1.1) — an empty round directory path would name the
// repository root, which is where a role unit's findings file must never land.
type UnitEnv struct {
	RunID    string   // TP_RUN_ID — a ULID, one per run
	Root     string   // repository root; children are spawned there
	TaskFile string   // TP_FILE — the task file the driver resolved
	Phase    string   // TP_PHASE — the oracle's phase
	Round    *int     // TP_ROUND — the oracle's round, nil when it reports null
	Kind     UnitKind // TP_UNIT_KIND
	ID       string   // TP_UNIT_ID — the unit's durable subject
	Seq      int      // TP_UNIT_SEQ — counts attempts, unique within a run
}

// ChildEnv returns the environment for one unit's child process, in the order
// §3.2.1 fixes: the parent's environment, runnerEnv merged over it, and the
// driver's own variables applied last.
//
// The order is the whole point. A `runner.env` entry can extend a child's
// environment but can never rewrite the identity the driver assigned it, so an
// entry for TP_UNATTENDED — or for TP_RUN_ID, or TP_ROUND_DIR — never reaches
// the child. Outside a round-based phase the two round variables are removed
// rather than emptied, whatever the parent or runnerEnv carried.
//
// parent takes os.Environ()'s "KEY=VALUE" form; an entry without "=" is not one
// and is dropped. The result is sorted, so the same unit always produces the
// same environment. u is read, never retained or modified.
func ChildEnv(parent []string, runnerEnv map[string]string, u *UnitEnv) []string {
	merged := make(map[string]string, len(parent)+len(runnerEnv)+len(driverEnv(u)))
	for _, entry := range parent {
		if key, value, ok := strings.Cut(entry, "="); ok {
			merged[key] = value
		}
	}
	maps.Copy(merged, runnerEnv)
	maps.Copy(merged, driverEnv(u))
	if u.Round == nil {
		delete(merged, EnvRound)
		delete(merged, EnvRoundDir)
	}

	out := make([]string, 0, len(merged))
	for key, value := range merged {
		out = append(out, key+"="+value)
	}
	slices.Sort(out)
	return out
}

// driverEnv returns the variables the driver owns for one unit — the set that
// wins over runnerEnv. The two round variables are present only when the oracle
// reported a round; ChildEnv removes any inherited copy when it did not.
func driverEnv(u *UnitEnv) map[string]string {
	env := map[string]string{
		EnvRunID:      u.RunID,
		EnvRunDir:     RunDir(u.Root, u.RunID),
		EnvTaskFile:   u.TaskFile,
		EnvPhase:      u.Phase,
		EnvUnitID:     u.ID,
		EnvUnitKind:   string(u.Kind),
		EnvUnitSeq:    strconv.Itoa(u.Seq),
		EnvUnattended: "1",
	}
	if u.Round != nil {
		env[EnvRound] = strconv.Itoa(*u.Round)
		env[EnvRoundDir] = RoundDir(u.Root, u.TaskFile, u.Phase, *u.Round)
	}
	return env
}

// RunBase returns the <base> that names a cycle's run artifacts —
// .tp/run-<base>.json, .tp/locks/run-<base>.lock, .tp/last_failure-<base>.json
// and .tp/rounds/<base>: the task file's name with its .tasks.json suffix
// removed. They are named per task file (§3.5), so two cycles in one repository
// never collide.
func RunBase(taskFile string) string {
	return strings.TrimSuffix(filepath.Base(taskFile), ".tasks.json")
}

// RunDir returns a run's directory — TP_RUN_DIR, the absolute path of
// .tp/runs/<run-id>, which holds what belongs to one run: its logs and its
// escalation records (§3.1.1).
func RunDir(root, runID string) string {
	return absoluteUnder(root, filepath.Join(".tp", "runs", runID))
}

// RoundDir returns a round's directory — TP_ROUND_DIR,
// .tp/rounds/<base>/<phase>-r<round>. It is keyed by the cycle rather than by
// the run, so a round interrupted by a driver death keeps the role files it
// already collected and the next run's record unit merges the completed set
// (§3.1.1).
//
// §3.1.1 writes the path repository-relative; the value handed to a child is
// that path resolved under the repository root, absolute for the reason
// TP_RUN_DIR is: a child, and the Stop hook that reads the same path, must not
// have to assume its own working directory.
func RoundDir(root, taskFile, phase string, round int) string {
	name := phase + "-r" + strconv.Itoa(round)
	return absoluteUnder(root, filepath.Join(".tp", "rounds", RunBase(taskFile), name))
}

// absoluteUnder joins rel under root and makes the result absolute, falling
// back to the joined path when the working directory cannot be read — a path
// that is merely relative is a better answer there than an empty one.
func absoluteUnder(root, rel string) string {
	joined := filepath.Join(root, rel)
	abs, err := filepath.Abs(joined)
	if err != nil {
		return joined
	}
	return abs
}

// crockfordAlphabet is Crockford's base32 alphabet, the ULID encoding: the
// digits and the upper-case letters minus I, L, O and U.
const crockfordAlphabet = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"

// ULID component widths: 48 bits of millisecond timestamp in 10 characters,
// then 80 bits of randomness in 16, for the canonical 26-character text.
const (
	ulidTimeBytes = 6
	ulidTimeChars = 10
	ulidRandBytes = 10
	ulidRandChars = 16
)

// NewULID returns a new ULID: a millisecond timestamp followed by 80 random
// bits, Crockford-base32 encoded. TP_RUN_ID is one (§3.1.1), which is what
// makes a run directory both unique and sortable by start time without the
// driver keeping a counter anywhere.
func NewULID() string {
	ms := uint64(time.Now().UTC().UnixMilli())
	timestamp := make([]byte, ulidTimeBytes)
	for i := ulidTimeBytes - 1; i >= 0; i-- {
		timestamp[i] = byte(ms)
		ms >>= 8
	}
	// crypto/rand.Read fills the slice or the program dies inside it; since Go
	// 1.24 it cannot return an error, so there is no failure mode to handle.
	entropy := make([]byte, ulidRandBytes)
	_, _ = rand.Read(entropy)
	return encodeCrockford(timestamp, ulidTimeChars) + encodeCrockford(entropy, ulidRandChars)
}

// encodeCrockford renders src — read as one big-endian number — as exactly
// chars base32 characters, left-padding with zero bits when the field is wider
// than the source. The timestamp's 48 bits pad into a 50-bit field, which is
// why a ULID's first character is never above '7'.
func encodeCrockford(src []byte, chars int) string {
	out := make([]byte, chars)
	bits := len(src) * 8
	for i := range out {
		value := 0
		for j := range 5 {
			// Index the field from its low end, so the padding lands at the top.
			bit := chars*5 - 1 - (i*5 + j)
			set := 0
			if bit < bits {
				set = int(src[len(src)-1-bit/8]>>(bit%8)) & 1
			}
			value = value<<1 | set
		}
		out[i] = crockfordAlphabet[value]
	}
	return string(out)
}
