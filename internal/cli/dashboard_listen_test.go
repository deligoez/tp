package cli

import (
	"errors"
	"net"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// takenPort holds a loopback port for the test's lifetime and returns it.
func takenPort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = l.Close() })
	return l.Addr().(*net.TCPAddr).Port
}

// An unchosen default port that is taken falls back to a free port the system
// picks, and says so.
func TestListenDashboard_ATakenDefaultPortFallsBackToAFreeOne(t *testing.T) {
	t.Parallel()
	port := takenPort(t)

	ln, fellBack, err := listenDashboard(t.Context(), port, false)
	require.NoError(t, err)
	t.Cleanup(func() { _ = ln.Close() })

	assert.True(t, fellBack)
	assert.NotEqual(t, port, ln.Addr().(*net.TCPAddr).Port)
}

// A port the caller chose with --port is never swapped for another: a taken
// one stays the bind failure it is.
func TestListenDashboard_AChosenTakenPortIsAnError(t *testing.T) {
	t.Parallel()
	port := takenPort(t)

	ln, fellBack, err := listenDashboard(t.Context(), port, true)
	if ln != nil {
		_ = ln.Close()
	}
	assert.False(t, fellBack)
	assert.True(t, errors.Is(err, syscall.EADDRINUSE), "%v", err)
}

// A free default port is bound as it is, with no fallback.
func TestListenDashboard_AFreeDefaultPortIsBoundAsItIs(t *testing.T) {
	t.Parallel()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := l.Addr().(*net.TCPAddr).Port
	require.NoError(t, l.Close())

	ln, fellBack, err := listenDashboard(t.Context(), port, false)
	require.NoError(t, err)
	t.Cleanup(func() { _ = ln.Close() })

	assert.False(t, fellBack)
	assert.Equal(t, port, ln.Addr().(*net.TCPAddr).Port)
}
