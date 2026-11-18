package engine

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// GatePass is a tp gate that passed inside a run on a clean checkout: the
// tree it gated and the command it ran. The unit's following tp done of that
// same tree with that same command stands on it instead of running the gate
// again (spec/backlog/gate-slots.md).
type GatePass struct {
	Tree    string `json:"tree"`
	GateCmd string `json:"gate_cmd"`
	At      string `json:"at"`
}

// GatePassPath is a unit attempt's gate pass under its run directory, keyed by
// its seq so a retry never stands on its predecessor's pass.
func GatePassPath(runDir string, seq int) string {
	return filepath.Join(runDir, fmt.Sprintf("gate-pass-%d.json", seq))
}

// RecordGatePass writes the pass, replacing an earlier one of the attempt.
func RecordGatePass(runDir string, seq int, tree, gateCmd string) error {
	data, err := json.MarshalIndent(&GatePass{Tree: tree, GateCmd: gateCmd, At: time.Now().UTC().Format(time.RFC3339Nano)}, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(runDir, 0o750); err != nil {
		return err
	}
	return writeFileAtomic(GatePassPath(runDir, seq), append(data, '\n'))
}

// GatePassCovers reports whether the attempt's recorded pass gated tree with
// gateCmd, and when it passed. An empty tree, a checkout no pass can stand
// for, is covered by nothing.
func GatePassCovers(runDir string, seq int, tree, gateCmd string) (time.Time, bool) {
	if tree == "" || gateCmd == "" {
		return time.Time{}, false
	}
	data, err := os.ReadFile(GatePassPath(runDir, seq)) //nolint:gosec // under the run dir, keyed by the driver's own seq
	if err != nil {
		return time.Time{}, false
	}
	var p GatePass
	if json.Unmarshal(data, &p) != nil || p.Tree != tree || p.GateCmd != gateCmd {
		return time.Time{}, false
	}
	at, err := time.Parse(time.RFC3339Nano, p.At)
	if err != nil {
		return time.Time{}, false
	}
	return at.UTC(), true
}

// MainGatedTree is the tree a gate on the main tree gates, when its pass may
// stand for that tree's close: HEAD's tree, while the working tree differs
// from HEAD only in paths tp writes (a claimed task file, .tp/), the rule
// ReusableGate holds the main-branch close to. "" otherwise.
func MainGatedTree(root, taskFile, spec string) string {
	tree, err := gitOut(root, "rev-parse", "HEAD^{tree}")
	if err != nil || !onlyTPOwnedChanges(root, taskFile, spec) {
		return ""
	}
	return tree
}
