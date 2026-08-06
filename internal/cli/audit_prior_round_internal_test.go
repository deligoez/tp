package cli

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// captureCLIStderr runs fn with os.Stderr redirected to a pipe and returns what
// was written. This is an in-package test so a helper that emits nothing on
// stdout can be driven directly, without a subprocess and its JSON payload.
func captureCLIStderr(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	orig := os.Stderr
	os.Stderr = w
	// defer, not an inline restore after fn: a panic or runtime.Goexit inside
	// fn would otherwise leave os.Stderr pointing at a pipe nobody drains and
	// nobody closes, so every later test in this package writes into it. The
	// sibling helper in internal/output does the same.
	defer func() { os.Stderr = orig }()
	done := make(chan string, 1)
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)
		done <- buf.String()
	}()
	fn()
	if err := w.Close(); err != nil {
		t.Fatalf("close pipe: %v", err)
	}
	return <-done
}

// TestLoadAuditPriorRound_MissingRoundFileIsAnnounced: when state.json names a
// prior audit round whose NDJSON file is gone, the whole prior-round section
// disappears from every role prompt. Dropping it silently makes a round-2
// prompt indistinguishable from a round-1 one, so the role re-derives findings
// it was meant to re-check. review.go, review_record.go and
// review_regression.go all announce the identical condition.
func TestLoadAuditPriorRound_MissingRoundFileIsAnnounced(t *testing.T) {
	dir := t.TempDir()
	specPath := filepath.Join(dir, "spec.md")
	if err := os.WriteFile(specPath, []byte("# Spec\n"), 0o600); err != nil {
		t.Fatalf("write spec: %v", err)
	}
	stateDir := filepath.Join(dir, ".tp-review", "spec")
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		t.Fatalf("mkdir state: %v", err)
	}
	state := `{"spec":"spec.md","review_rounds":[],"audit_rounds":[` +
		`{"round":1,"findings":1,"clean":false,"recorded_at":"2024-01-01T00:00:00Z",` +
		`"file":"audit-round-1.ndjson","spec_hash":"sha256:x","id_scheme":"slug"}]}`
	if err := os.WriteFile(filepath.Join(stateDir, "state.json"), []byte(state), 0o600); err != nil {
		t.Fatalf("write state: %v", err)
	}
	// audit-round-1.ndjson is deliberately absent.

	var prior map[string]*auditPriorRound
	stderr := captureCLIStderr(t, func() { prior = loadAuditPriorRound(specPath) })

	if prior != nil {
		t.Errorf("a missing round file yields no prior-round context, got %d roles", len(prior))
	}
	const want = "round 1 file audit-round-1.ndjson is missing; skipping its rows"
	if !strings.Contains(stderr, want) {
		t.Errorf("dropped the prior-round section without a word: stderr = %q, want it to contain %q", stderr, want)
	}
}
