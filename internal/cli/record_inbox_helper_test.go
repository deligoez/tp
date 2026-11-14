package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/deligoez/tp/internal/engine"
)

// recordViaInbox records the emitted review round of dir's spec.md the way a
// hand-run round is recorded since the emission stores the roles it owes: each
// NDJSON row of rows goes to the inbox file of the role it names, a row naming
// no owed role goes to the first owed role's file with its role rewritten to
// that one, every other owed role gets an empty file, and tp review spec.md
// --record-inbox records the round.
func recordViaInbox(t *testing.T, dir, rows string) (stdout, stderr string, code int) {
	t.Helper()
	return recordViaInboxEnv(t, dir, nil, "spec.md", rows)
}

// recordViaInboxEnv is recordViaInbox with extra environment entries for the
// tp child.
func recordViaInboxEnv(t *testing.T, dir string, env []string, specRel, rows string, extra ...string) (stdout, stderr string, code int) {
	t.Helper()
	writeInboxRows(t, dir, specRel, rows)
	args := append([]string{"review", specRel, "--record-inbox"}, extra...)
	if env == nil {
		return runTP(t, dir, args...)
	}
	return runTPEnv(t, dir, env, args...)
}

// writeInboxRows writes the emitted round's inbox for recordViaInbox.
func writeInboxRows(t *testing.T, dir, specRel, rows string) {
	t.Helper()
	specAbs := specRel
	if !filepath.IsAbs(specAbs) {
		specAbs = filepath.Join(dir, specRel)
	}
	inbox, err := engine.ReviewInboxPanel(specAbs)
	require.NoError(t, err)
	require.True(t, inbox.Emitted && inbox.Stored, "round %d of %s was emitted with its owed roles", inbox.Round, specRel)
	require.NotEmpty(t, inbox.Roles)

	bodies := make(map[string]*strings.Builder, len(inbox.Roles))
	for _, role := range inbox.Roles {
		bodies[role] = &strings.Builder{}
	}
	for line := range strings.SplitSeq(rows, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		role := inbox.Roles[0]
		var row map[string]any
		if json.Unmarshal([]byte(line), &row) != nil {
			bodies[role].WriteString(line + "\n")
			continue
		}
		if named, _ := row["role"].(string); slices.Contains(inbox.Roles, named) {
			bodies[named].WriteString(line + "\n")
			continue
		}
		if _, named := row["role"]; named {
			row["role"] = role
			b, mErr := json.Marshal(row)
			require.NoError(t, mErr)
			line = string(b)
		}
		bodies[role].WriteString(line + "\n")
	}
	for role, body := range bodies {
		path := engine.RoleInboxPath(specAbs, engine.PhaseReview, inbox.Round, role)
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o750))
		require.NoError(t, os.WriteFile(path, []byte(body.String()), 0o600))
	}
}
