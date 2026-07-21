package engine

import (
	"embed"
	"fmt"
	"strings"

	"github.com/deligoez/tp/internal/model"
)

// Phase names for the two role corpora; the phase is inferred from the directory
// (§3.1), never stored on a role.
const (
	PhaseReviewers = "reviewers"
	PhaseAuditors  = "auditors"
)

//go:embed corpus
var defaultCorpusFS embed.FS

// defaultCorpusOrder is the §5.1 default-corpus table: per domain, the ordered
// role ids for each phase. The order is the emitted panel order (byte-identical
// to the pre-v0.25.0 hardcoded panels for software), so it is authored, not
// alphabetical.
var defaultCorpusOrder = map[string]map[string][]string{
	"software": {
		PhaseReviewers: {"implementer", "tester", "architect"},
		PhaseAuditors:  {"spec-coverage", "security", "maintainability-conventions"},
	},
	"prose": {
		PhaseReviewers: {"coherence", "soundness"},
		PhaseAuditors:  {"spec-coverage", "soundness"},
	},
}

// DefaultCorpusDomains lists the domains tp ships an embedded corpus for, in
// selection-preference order (software is the default domain).
func DefaultCorpusDomains() []string {
	return []string{"software", "prose"}
}

// HasDefaultCorpus reports whether tp ships an embedded corpus for domain.
func HasDefaultCorpus(domain string) bool {
	_, ok := defaultCorpusOrder[domain]
	return ok
}

// DefaultCorpus returns the embedded default roles for a domain and phase
// ("reviewers" or "auditors") in the §5.1 panel order. Every embedded file is
// parsed and validated through the shared role validator, so a malformed
// embedded file is a programming error surfaced here.
func DefaultCorpus(domain, phase string) ([]model.Role, error) {
	byPhase, ok := defaultCorpusOrder[domain]
	if !ok {
		return nil, fmt.Errorf("no embedded corpus for domain %q (known: %s)", domain, strings.Join(DefaultCorpusDomains(), ", "))
	}
	ids, ok := byPhase[phase]
	if !ok {
		return nil, fmt.Errorf("unknown phase %q (want %s or %s)", phase, PhaseReviewers, PhaseAuditors)
	}
	roles := make([]model.Role, 0, len(ids))
	for _, id := range ids {
		data, err := defaultCorpusRaw(domain, phase, id)
		if err != nil {
			return nil, err
		}
		role, err := ParseRoleBytes(data, id)
		if err != nil {
			return nil, fmt.Errorf("embedded corpus role %s/%s/%s is invalid: %w", domain, phase, id, err)
		}
		roles = append(roles, role)
	}
	return roles, nil
}

// defaultCorpusRaw returns the raw JSON bytes of one embedded role file, so eject
// (§5.3) can write byte-identical copies.
func defaultCorpusRaw(domain, phase, id string) ([]byte, error) {
	path := "corpus/" + domain + "/" + phase + "/" + id + ".json"
	data, err := defaultCorpusFS.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("embedded corpus missing %s: %w", path, err)
	}
	return data, nil
}

// DefaultRoleFile is one embedded role's id and its raw JSON bytes, for
// byte-identical eject (§5.4).
type DefaultRoleFile struct {
	ID   string
	Data []byte
}

// DefaultCorpusFiles returns the embedded role files for a domain and phase in
// §5.1 panel order, each carrying the raw JSON bytes so eject writes copies
// byte-identical to the embedded prompts (§5.4).
func DefaultCorpusFiles(domain, phase string) ([]DefaultRoleFile, error) {
	byPhase, ok := defaultCorpusOrder[domain]
	if !ok {
		return nil, fmt.Errorf("no embedded corpus for domain %q (known: %s)", domain, strings.Join(DefaultCorpusDomains(), ", "))
	}
	ids, ok := byPhase[phase]
	if !ok {
		return nil, fmt.Errorf("unknown phase %q (want %s or %s)", phase, PhaseReviewers, PhaseAuditors)
	}
	out := make([]DefaultRoleFile, 0, len(ids))
	for _, id := range ids {
		data, err := defaultCorpusRaw(domain, phase, id)
		if err != nil {
			return nil, err
		}
		out = append(out, DefaultRoleFile{ID: id, Data: data})
	}
	return out, nil
}
