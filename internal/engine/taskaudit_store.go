package engine

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// PhaseTaskAudit names the task audit wherever a phase string keys a path:
// its inbox, its run-time round directory and its prompts' framing.
const PhaseTaskAudit = "task-audit"

// TaskAuditEmissionPath is the frozen emission of task audit seq: the batch's
// members, files, roles and the HEAD it was emitted at, written before any
// prompt and removed when the task audit records.
func TaskAuditEmissionPath(specPath string, seq int) string {
	return filepath.Join(roleInboxDir(specPath, PhaseTaskAudit), fmt.Sprintf("batch-%d.json", seq))
}

// WriteTaskAuditEmission freezes rec as task audit rec.Seq's emission.
func WriteTaskAuditEmission(specPath string, rec *TaskAuditRecord) error {
	path := TaskAuditEmissionPath(specPath, rec.Seq)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		return err
	}
	// A sibling role unit reads it lock-free: it sees no file or the whole one.
	tmp, err := os.CreateTemp(filepath.Dir(path), ".batch-*.json")
	if err != nil {
		return err
	}
	if _, err := tmp.Write(append(data, '\n')); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmp.Name())
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmp.Name())
		return err
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		_ = os.Remove(tmp.Name())
		return err
	}
	return nil
}

// ReadTaskAuditEmission reads task audit seq's frozen emission: nil and no
// error when none was emitted.
func ReadTaskAuditEmission(specPath string, seq int) (*TaskAuditRecord, error) {
	data, err := os.ReadFile(TaskAuditEmissionPath(specPath, seq))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var rec TaskAuditRecord
	if err := json.Unmarshal(data, &rec); err != nil {
		return nil, fmt.Errorf("task audit %d's emission %s does not parse: %w", seq, TaskAuditEmissionPath(specPath, seq), err)
	}
	return &rec, nil
}

// FileChangedSince reports whether path differs between rev and HEAD in the
// repository holding dir. A rev git cannot use, an unsafe one included,
// counts as changed: a reading nobody can compare is no reason to skip one.
func FileChangedSince(dir, rev, path string) bool {
	if rev == "" || !SafeGitRev(rev) {
		return true
	}
	// :(top) reads path from the repository root, as every caller hands it,
	// whatever directory under the root dir is.
	cmd := exec.Command("git", "diff", "--quiet", "--no-ext-diff", rev, "HEAD", "--", ":(top)"+path)
	cmd.Dir = dir
	return cmd.Run() != nil
}

// GitHead is the commit HEAD names in the repository holding dir, or "" when
// git cannot say.
func GitHead(dir string) string {
	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
