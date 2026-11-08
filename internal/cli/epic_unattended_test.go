package cli_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEpicWriters_UnattendedRefusesEveryWriter is §9 row 12 for the writers:
// under TP_UNATTENDED every §3 writer exits 2 on a legal payload, leaves the
// file byte-identical and points the unit at tp escalate --decision other. A
// fence on tp epic add alone would let the other four write.
func TestEpicWriters_UnattendedRefusesEveryWriter(t *testing.T) {
	t.Parallel()
	payload := epicImportPayload(t, `{
  "version": 1,
  "milestones": [["A"]],
  "epics": [
    {"id": "solo", "title": "Solo", "milestone": "A"}
  ]
}
`)
	writers := map[string][]string{
		"add":            {"epic", "add", `{"id": "e", "title": "E", "milestone": "A"}`},
		"set field":      {"epic", "set", "a", "title=Renamed"},
		"set milestones": {"epic", "set", `milestones=[["A", "B"], ["L1"]]`},
		"remove":         {"epic", "remove", "a"},
		"import":         {"epic", "import", payload, "--force"},
	}
	for name, args := range writers {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			dir := epicGraphDir(t, epicChainWithGate)
			before := readEpicGraphFile(t, dir)

			_, stderr, code := runTPFence(t, dir, true, args...)
			require.Equal(t, 2, code, "stderr: %s", stderr)
			assert.Contains(t, stderr, "tp escalate --decision other")
			assert.Equal(t, before, readEpicGraphFile(t, dir), "a refused writer writes nothing")

			_, stderr, code = runTPFence(t, dir, false, args...)
			require.Equal(t, 0, code, "the payload is legal when attended; stderr: %s", stderr)
		})
	}
}
