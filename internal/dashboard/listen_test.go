package dashboard

import (
	"net"
	"os"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
)

// listenError is the error net.Listen returns when bind fails with errno.
func listenError(errno syscall.Errno) error {
	return &net.OpError{Op: "listen", Net: "tcp", Err: os.NewSyscallError("bind", errno)}
}

// TestListenFailure_SaysTakenOnlyForAnAddressInUse is §1's bind failure: an
// address in use names the port as taken and the process holding it, and any
// other failure names the port and the real error without calling it taken.
func TestListenFailure_SaysTakenOnlyForAnAddressInUse(t *testing.T) {
	t.Parallel()
	msg, hint := ListenFailure(7377, listenError(syscall.EADDRINUSE))
	assert.Contains(t, msg, "port 7377 is taken")
	assert.Contains(t, hint, "stop the process holding port 7377")

	msg, hint = ListenFailure(7377, listenError(syscall.EACCES))
	assert.NotContains(t, msg, "taken")
	assert.Contains(t, msg, "7377")
	assert.Contains(t, msg, syscall.EACCES.Error())
	assert.NotContains(t, hint, "stop the process holding")
}
