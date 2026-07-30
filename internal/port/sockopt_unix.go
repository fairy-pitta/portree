//go:build unix

package port

import (
	"errors"
	"syscall"
)

// disableReuseAddr clears SO_REUSEADDR (set by Go's listener defaults) so the
// probe bind conflicts with every existing listener on the port, regardless
// of which address that listener bound.
func disableReuseAddr(network, address string, c syscall.RawConn) error {
	var sockErr error
	if err := c.Control(func(fd uintptr) {
		sockErr = syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 0)
	}); err != nil {
		return err
	}
	return sockErr
}

// stackUnavailable reports whether a listen error means the address family is
// not available on this host (e.g. IPv6 disabled) rather than the port being
// in use. Such a stack should be skipped by the probe, not counted as busy.
func stackUnavailable(err error) bool {
	return errors.Is(err, syscall.EAFNOSUPPORT) || errors.Is(err, syscall.EADDRNOTAVAIL)
}
