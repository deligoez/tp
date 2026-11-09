package cli_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// epicClaimOfB returns the claim .tp/epics.json in dir holds for epic b, nil
// when it holds none.
func epicClaimOfB(t *testing.T, dir string) *struct {
	At string `json:"at"`
	By string `json:"by"`
} {
	t.Helper()
	var g struct {
		Epics []struct {
			ID      string `json:"id"`
			Claimed *struct {
				At string `json:"at"`
				By string `json:"by"`
			} `json:"claimed"`
		} `json:"epics"`
	}
	require.NoError(t, json.Unmarshal(readEpicGraphFile(t, dir), &g))
	for _, e := range g.Epics {
		if e.ID == "b" {
			return e.Claimed
		}
	}
	t.Fatal("epic b not in the graph")
	return nil
}

// TestEpicClaim_SecondClaimExitsOneAndReleaseLetsItThrough is §10 row 11: on
// the chain, claim b records b's holder although b is blocked by the
// condition c; a second claim exits 1 and writes nothing; --release removes
// the claim and the second claim then goes through. A claim that overwrites
// an existing one turns it red.
func TestEpicClaim_SecondClaimExitsOneAndReleaseLetsItThrough(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, bindingChain)

	_, stderr, code := runTP(t, dir, "epic", "claim", "b", "--by", "agent-1")
	require.Equal(t, 0, code, "a blocked epic is claimed; stderr: %s", stderr)
	claim := epicClaimOfB(t, dir)
	require.NotNil(t, claim, "claim b records b's claim")
	assert.Equal(t, "agent-1", claim.By)
	assert.NotEmpty(t, claim.At)

	before := readEpicGraphFile(t, dir)
	_, stderr, code = runTP(t, dir, "epic", "claim", "b", "--by", "agent-2")
	assert.Equal(t, 1, code, "a second claim is refused; stderr: %s", stderr)
	assert.Contains(t, stderr, "agent-1", "the refusal names the holder")
	assert.Equal(t, before, readEpicGraphFile(t, dir), "a refused claim writes nothing")

	_, stderr, code = runTP(t, dir, "epic", "claim", "b", "--release")
	require.Equal(t, 0, code, "stderr: %s", stderr)
	assert.Nil(t, epicClaimOfB(t, dir), "--release removes b's claim")

	_, stderr, code = runTP(t, dir, "epic", "claim", "b", "--by", "agent-2")
	require.Equal(t, 0, code, "stderr: %s", stderr)
	assert.Equal(t, "agent-2", epicClaimOfB(t, dir).By)
}

// TestEpicClaim_RefusesAConditionADoneAndABoundEpic is §7: on the chain with
// b bound, claiming the condition c, the done d or the bound b exits 1 and
// writes nothing.
func TestEpicClaim_RefusesAConditionADoneAndABoundEpic(t *testing.T) {
	t.Parallel()
	dir := boundEpicDir(t, bindingChain)
	before := readEpicGraphFile(t, dir)

	for id, names := range map[string]string{"c": "condition", "d": "done", "b": "b.tasks.json"} {
		_, stderr, code := runTP(t, dir, "epic", "claim", id, "--by", "agent-1")
		assert.Equal(t, 1, code, "claiming %s is refused; stderr: %s", id, stderr)
		assert.Contains(t, stderr, names, "the refusal of %s says why", id)
	}
	assert.Equal(t, before, readEpicGraphFile(t, dir), "a refused claim writes nothing")
}

