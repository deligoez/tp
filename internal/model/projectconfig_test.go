package model

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProjectConfig_ParsesWorkflowKey(t *testing.T) {
	var pc ProjectConfig
	require.NoError(t, json.Unmarshal([]byte(`{"workflow":{"review_max_rounds":8}}`), &pc))
	require.NotNil(t, pc.Workflow.ReviewMaxRounds)
	assert.Equal(t, 8, *pc.Workflow.ReviewMaxRounds)
}

func TestProjectConfig_AbsentEqualsEmpty(t *testing.T) {
	// An empty object contributes no overrides, matching an absent file.
	var empty ProjectConfig
	require.NoError(t, json.Unmarshal([]byte(`{}`), &empty))
	assert.True(t, empty.Workflow.IsEmpty(), "empty {} config must set no workflow overrides")

	// The zero value (as used for an absent file) is likewise empty.
	var absent ProjectConfig
	assert.True(t, absent.Workflow.IsEmpty(), "zero-value config must set no workflow overrides")
}

func TestWorkflowOverride_PresenceDistinctFromZero(t *testing.T) {
	// An explicit zero (review_max_rounds: 0 = no cap) is a present override,
	// distinct from an absent key.
	var set WorkflowOverride
	require.NoError(t, json.Unmarshal([]byte(`{"review_max_rounds":0}`), &set))
	require.NotNil(t, set.ReviewMaxRounds)
	assert.Equal(t, 0, *set.ReviewMaxRounds)
	assert.False(t, set.IsEmpty())

	var unset WorkflowOverride
	require.NoError(t, json.Unmarshal([]byte(`{}`), &unset))
	assert.Nil(t, unset.ReviewMaxRounds)
	assert.True(t, unset.IsEmpty())
}

func TestWorkflowOverride_ChecksPresentEmptyVsAbsent(t *testing.T) {
	// An explicit empty checks array is present (replace with nothing);
	// an absent checks key is nil (inherit).
	var present WorkflowOverride
	require.NoError(t, json.Unmarshal([]byte(`{"checks":[]}`), &present))
	require.NotNil(t, present.Checks)
	assert.Empty(t, *present.Checks)

	var absent WorkflowOverride
	require.NoError(t, json.Unmarshal([]byte(`{}`), &absent))
	assert.Nil(t, absent.Checks)
}

func TestLocalConfig_ActiveAndDefaults(t *testing.T) {
	var lc LocalConfig
	require.NoError(t, json.Unmarshal([]byte(`{"active":"a/a.tasks.json","defaults":{"compact":true}}`), &lc))
	require.NotNil(t, lc.Active)
	assert.Equal(t, "a/a.tasks.json", *lc.Active)
	assert.True(t, lc.Defaults["compact"])

	// Absent local file is equivalent to an empty object: nil pointers/maps.
	var absent LocalConfig
	require.NoError(t, json.Unmarshal([]byte(`{}`), &absent))
	assert.Nil(t, absent.Active)
	assert.Nil(t, absent.Defaults)
}
