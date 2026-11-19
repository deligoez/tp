package engine

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAcquireGateSlot_CapZeroIsNoCap: a cap of 0 is no cap. The gate starts at
// once, holding nothing, and no slot directory is made.
func TestAcquireGateSlot_CapZeroIsNoCap(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), "gate-slots")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	slot, err := AcquireGateSlot(ctx, dir, 0, false)
	require.NoError(t, err, "a cap of 0 never waits")
	slot.Release()
	assert.NoDirExists(t, dir)
}

