package dashboard

import (
	"errors"
	"fmt"
	"syscall"
)

// ListenFailure is the message and hint tp dashboard exits 1 with when it
// cannot bind port: an address in use is a taken port, and any other failure
// is reported as itself, so a refused or unavailable address is not
// mistaken for one another process holds.
func ListenFailure(port int, err error) (msg, hint string) {
	if errors.Is(err, syscall.EADDRINUSE) {
		return fmt.Sprintf("port %d is taken: %v", port, err),
			fmt.Sprintf("stop the process holding port %d, or pass --port naming a free one", port)
	}
	return fmt.Sprintf("cannot listen on port %d: %v", port, err),
		fmt.Sprintf("pass --port naming a port this user may bind on %s", Host)
}
