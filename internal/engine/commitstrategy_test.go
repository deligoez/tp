package engine

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func csPtr(s string) *string { return &s }

func TestResolveCommitStrategy(t *testing.T) {
	cases := []struct {
		name      string
		taskOver  *string
		project   *string
		wantName  string
		wantWarns bool
	}{
		{"absent everywhere resolves to auto", nil, nil, CommitStrategyAuto, false},
		{"task override wins over project", csPtr("hc"), csPtr("builtin"), CommitStrategyHC, false},
		{"project applies when task absent", nil, csPtr("builtin"), CommitStrategyBuiltin, false},
		{"explicit auto stays auto", csPtr("auto"), nil, CommitStrategyAuto, false},
		{"present unrecognized resolves to builtin with warning", csPtr("squash"), nil, CommitStrategyBuiltin, true},
		{"unrecognized task value wins even over valid project", csPtr("squash"), csPtr("hc"), CommitStrategyBuiltin, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			name, warning := ResolveCommitStrategy(tc.taskOver, tc.project)
			assert.Equal(t, tc.wantName, name)
			assert.Equal(t, tc.wantWarns, warning != "", "warning presence")
		})
	}
}

func TestEffectiveCommitStrategy(t *testing.T) {
	cases := []struct {
		name          string
		resolved      string
		hcAvailable   bool
		wantEffective string
		wantHCMissing bool
	}{
		{"auto with hc present resolves to hc", CommitStrategyAuto, true, CommitStrategyHC, false},
		{"auto with hc absent falls back to builtin", CommitStrategyAuto, false, CommitStrategyBuiltin, false},
		{"explicit hc stays hc when present", CommitStrategyHC, true, CommitStrategyHC, false},
		{"explicit hc stays hc when absent, flags missing", CommitStrategyHC, false, CommitStrategyHC, true},
		{"builtin stays builtin", CommitStrategyBuiltin, true, CommitStrategyBuiltin, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			effective, hcMissing := EffectiveCommitStrategy(tc.resolved, tc.hcAvailable)
			assert.Equal(t, tc.wantEffective, effective)
			assert.Equal(t, tc.wantHCMissing, hcMissing)
		})
	}
}
