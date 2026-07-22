package engine

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/deligoez/tp/internal/model"
)

// clampWorkflowRanges unsets any out-of-range override field so it falls back
// to the built-in default at resolution (gate_timeout_seconds→600, caps→0,
// clean_rounds→2), returning a warning for each. This matches the read-time
// fallback v0.23.0 already applies to a hand-edited task file.
func clampWorkflowRanges(wo *model.WorkflowOverride) []string {
	var warnings []string
	check := func(name string, p **int, lo, hi int) {
		if *p != nil && (**p < lo || **p > hi) {
			warnings = append(warnings, fmt.Sprintf("workflow.%s: %d is out of range [%d,%d], using the built-in default", name, **p, lo, hi))
			*p = nil
		}
	}
	check("gate_timeout_seconds", &wo.GateTimeoutSeconds, 30, 3600)
	check("review_clean_rounds", &wo.ReviewCleanRounds, 1, 10)
	check("audit_clean_rounds", &wo.AuditCleanRounds, 1, 10)
	check("review_max_rounds", &wo.ReviewMaxRounds, 0, 50)
	check("audit_max_rounds", &wo.AuditMaxRounds, 0, 50)
	return warnings
}

// ClampWorkflowRanges is the exported form of the range clamp so callers outside
// the engine (tp config --resolved source attribution) can drop an out-of-range
// override field to absent before inspecting its presence (§3.4).
func ClampWorkflowRanges(wo *model.WorkflowOverride) []string {
	return clampWorkflowRanges(wo)
}

// MalformedConfigError signals a corrupt config file (unreadable, not valid
// JSON, or a non-object top-level value) that must not be silently ignored: the
// reading command aborts with exit 3 and this repair-or-delete hint.
type MalformedConfigError struct {
	Path string
	Err  error
}

func (e *MalformedConfigError) Error() string {
	return fmt.Sprintf("malformed config %s: %v", e.Path, e.Err)
}

// Hint returns the actionable repair-or-delete hint for the error shape.
func (e *MalformedConfigError) Hint() string {
	return fmt.Sprintf("repair or delete %s", e.Path)
}

func (e *MalformedConfigError) Unwrap() error { return e.Err }

// knownDefaultFlags is the set of recognized flags inside a local.json defaults block.
var knownDefaultFlags = map[string]bool{"compact": true, "quiet": true, "no_color": true}

// knownWorkflowKeys is the set of recognized keys inside a config workflow block.
var knownWorkflowKeys = map[string]bool{
	"quality_gate": true, "commit_strategy": true, "gate_timeout_seconds": true,
	"review_clean_rounds": true, "audit_clean_rounds": true,
	"review_max_rounds": true, "audit_max_rounds": true, "checks": true,
}

// parseWorkflowOverride leniently parses a workflow object: an unknown key or a
// value of the wrong JSON type is collected as a validation warning and left
// unset, so the field falls back to its inherited or built-in value. A workflow
// value that is not an object is itself a warning.
func parseWorkflowOverride(raw json.RawMessage) (wo model.WorkflowOverride, warnings []string) {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		return wo, []string{"workflow: value is not an object, ignored"}
	}
	intField := func(key string, v json.RawMessage) *int {
		var n int
		if err := json.Unmarshal(v, &n); err != nil {
			warnings = append(warnings, "workflow."+key+": expected a number, ignored")
			return nil
		}
		return &n
	}
	for k, v := range m {
		if !knownWorkflowKeys[k] {
			warnings = append(warnings, "unknown workflow key: "+k)
			continue
		}
		switch k {
		case "quality_gate":
			var s string
			if err := json.Unmarshal(v, &s); err != nil {
				warnings = append(warnings, "workflow.quality_gate: expected a string, ignored")
			} else {
				wo.QualityGate = &s
			}
		case "commit_strategy":
			var s string
			if err := json.Unmarshal(v, &s); err != nil {
				warnings = append(warnings, "workflow.commit_strategy: expected a string, ignored")
			} else {
				wo.CommitStrategy = &s
			}
		case "gate_timeout_seconds":
			wo.GateTimeoutSeconds = intField(k, v)
		case "review_clean_rounds":
			wo.ReviewCleanRounds = intField(k, v)
		case "audit_clean_rounds":
			wo.AuditCleanRounds = intField(k, v)
		case "review_max_rounds":
			wo.ReviewMaxRounds = intField(k, v)
		case "audit_max_rounds":
			wo.AuditMaxRounds = intField(k, v)
		case "checks":
			var cs []model.Check
			if err := json.Unmarshal(v, &cs); err != nil {
				warnings = append(warnings, "workflow.checks: expected an array, ignored")
			} else {
				wo.Checks = &cs
			}
		}
	}
	return wo, warnings
}

// LoadProjectConfig reads and leniently parses tpDir/config.json into a
// ProjectConfig, returning validation warnings for unknown keys and wrong-typed
// values (which are ignored, not errors). A missing file returns an empty
// ProjectConfig, equivalent to an empty object {}. Workflow fields are
// presence-tracked, so an absent key stays distinct from an explicit zero.
func LoadProjectConfig(tpDir string) (model.ProjectConfig, []string, error) {
	var pc model.ProjectConfig
	path := filepath.Join(tpDir, "config.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return pc, nil, nil
		}
		return pc, nil, &MalformedConfigError{Path: path, Err: err}
	}
	var top map[string]json.RawMessage
	if err := json.Unmarshal(data, &top); err != nil {
		return pc, nil, &MalformedConfigError{Path: path, Err: err}
	}
	var warnings []string
	for k := range top {
		if k != "workflow" {
			warnings = append(warnings, "unknown top-level key: "+k)
		}
	}
	if wfRaw, ok := top["workflow"]; ok {
		wo, wfWarn := parseWorkflowOverride(wfRaw)
		pc.Workflow = wo
		warnings = append(warnings, wfWarn...)
		warnings = append(warnings, clampWorkflowRanges(&pc.Workflow)...)
	}
	return pc, warnings, nil
}

// LoadLocalConfig reads and leniently parses tpDir/local.json into a
// LocalConfig, returning validation warnings for unknown keys and wrong-typed
// values (a non-string active, a non-boolean defaults entry, or a non-object
// defaults), which are ignored. A missing file returns an empty LocalConfig.
func LoadLocalConfig(tpDir string) (model.LocalConfig, []string, error) {
	var lc model.LocalConfig
	path := filepath.Join(tpDir, "local.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return lc, nil, nil
		}
		return lc, nil, &MalformedConfigError{Path: path, Err: err}
	}
	var top map[string]json.RawMessage
	if err := json.Unmarshal(data, &top); err != nil {
		return lc, nil, &MalformedConfigError{Path: path, Err: err}
	}
	var warnings []string
	for k := range top {
		if k != "active" && k != "defaults" {
			warnings = append(warnings, "unknown top-level key: "+k)
		}
	}
	if raw, ok := top["active"]; ok {
		var s string
		if err := json.Unmarshal(raw, &s); err != nil {
			warnings = append(warnings, "active: expected a string, ignored")
		} else {
			lc.Active = &s
		}
	}
	if raw, ok := top["defaults"]; ok {
		var m map[string]json.RawMessage
		if err := json.Unmarshal(raw, &m); err != nil {
			warnings = append(warnings, "defaults: value is not an object, ignored")
		} else {
			lc.Defaults = make(map[string]bool, len(m))
			for k, v := range m {
				if !knownDefaultFlags[k] {
					warnings = append(warnings, "unknown defaults key: "+k)
					continue
				}
				var b bool
				if err := json.Unmarshal(v, &b); err != nil {
					warnings = append(warnings, "defaults."+k+": expected a boolean, ignored")
					continue
				}
				lc.Defaults[k] = b
			}
		}
	}
	return lc, warnings, nil
}

// StripTaskWorkflowFields removes the named keys from a task file's raw
// "workflow" block and rewrites the file, leaving every other field intact. It
// is used by tp config --extract to thin a task file after hoisting shared
// policy to the project config.
func StripTaskWorkflowFields(path string, fields []string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var top map[string]json.RawMessage
	if err := json.Unmarshal(data, &top); err != nil {
		return err
	}
	wfRaw, ok := top["workflow"]
	if !ok {
		return nil
	}
	var wf map[string]json.RawMessage
	if err := json.Unmarshal(wfRaw, &wf); err != nil {
		return err
	}
	for _, f := range fields {
		delete(wf, f)
	}
	newWf, err := json.Marshal(wf)
	if err != nil {
		return err
	}
	top["workflow"] = newWf
	out, err := json.MarshalIndent(top, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(out, '\n'), 0o600)
}

// ProjectConfigDir returns the .tp/ directory that project-config writes target:
// the discovered .tp/ when one exists, otherwise a new .tp/ at the project root
// (the git boundary, or the working directory outside a git repository).
func ProjectConfigDir(start string) string {
	if tpDir := DiscoverTPDir(start); tpDir != "" {
		return tpDir
	}
	return filepath.Join(ProjectRoot(start), ".tp")
}

// WriteProjectConfig writes pc to tpDir/config.json with 2-space indentation,
// creating tpDir and its .gitignore first so local.json stays ignored.
func WriteProjectConfig(tpDir string, pc model.ProjectConfig) error {
	if err := os.MkdirAll(tpDir, 0o755); err != nil {
		return err
	}
	if err := EnsureTPGitignore(tpDir); err != nil {
		return err
	}
	data, err := json.MarshalIndent(pc, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(tpDir, "config.json"), append(data, '\n'), 0o600)
}

// WriteLocalConfig writes lc to tpDir/local.json with 2-space indentation,
// creating tpDir and its .gitignore first.
func WriteLocalConfig(tpDir string, lc model.LocalConfig) error {
	if err := os.MkdirAll(tpDir, 0o755); err != nil {
		return err
	}
	if err := EnsureTPGitignore(tpDir); err != nil {
		return err
	}
	data, err := json.MarshalIndent(lc, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(tpDir, "local.json"), append(data, '\n'), 0o600)
}

// EnsureTPGitignore ensures tpDir/.gitignore exists and contains a "local.json"
// entry, so .tp/local.json stays git-ignored even when the .tp/ directory was
// created by hand rather than by tp. It is idempotent: it creates the file when
// absent, appends the entry when the file exists without it, and does nothing
// when the entry is already present. It is invoked whenever tp writes any file
// under .tp/.
func EnsureTPGitignore(tpDir string) error {
	path := filepath.Join(tpDir, ".gitignore")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return os.WriteFile(path, []byte("local.json\n"), 0o600)
		}
		return err
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) == "local.json" {
			return nil // already ignored
		}
	}
	content := string(data)
	if content != "" && !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	content += "local.json\n"
	return os.WriteFile(path, []byte(content), 0o600)
}

// hasGitBoundary reports whether dir contains a .git entry (a directory in a
// normal clone, or a file in a git worktree or submodule).
func hasGitBoundary(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, ".git"))
	return err == nil
}

// FindGitBoundary walks up from start and returns the first ancestor that
// contains a .git entry (directory or file), or "" when none is found up to the
// filesystem root.
func FindGitBoundary(start string) string {
	dir, err := filepath.Abs(start)
	if err != nil {
		return ""
	}
	for {
		if hasGitBoundary(dir) {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

// ProjectRoot returns the project root for a start directory: the directory
// containing the discovered .tp/, or — when no .tp/ exists yet — the git
// boundary directory, or the start directory itself when not inside a git
// repository. Config creation and the project-wide scan use this root.
// Discovery is anchored once at start and is never re-anchored to a --file
// located in another directory. When no .tp/ is found, callers use built-in
// defaults and per-task-file settings exactly as in v0.23.0.
func ProjectRoot(start string) string {
	if tpDir := DiscoverTPDir(start); tpDir != "" {
		return filepath.Dir(tpDir)
	}
	if boundary := FindGitBoundary(start); boundary != "" {
		return boundary
	}
	if abs, err := filepath.Abs(start); err == nil {
		return abs
	}
	return start
}

// DiscoverTPDir discovers the project's .tp/ directory exactly once per
// invocation by walking upward from start, testing each ancestor — including
// start itself and the git-boundary directory — and stopping at the first
// ancestor that contains a .tp/ directory.
//
// The walk halts at the repository boundary (the first ancestor containing a
// .git directory or file) or the filesystem root, whichever comes first, and
// never reads a .tp/ above that boundary. It returns the absolute path to the
// discovered .tp/ directory, or "" when none is found within the boundary.
func DiscoverTPDir(start string) string {
	dir, err := filepath.Abs(start)
	if err != nil {
		return ""
	}
	for {
		if info, statErr := os.Stat(filepath.Join(dir, ".tp")); statErr == nil && info.IsDir() {
			return filepath.Join(dir, ".tp")
		}
		// Stop after testing the git-boundary directory itself.
		if hasGitBoundary(dir) {
			return ""
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "" // filesystem root reached
		}
		dir = parent
	}
}
