//go:build !unix

package port

import "syscall"

// disableReuseAddr is a no-op on non-Unix platforms, where Go does not set
// SO_REUSEADDR on listeners and plain bind probes already conflict with
// existing listeners.
func disableReuseAddr(network, address string, c syscall.RawConn) error {
	return nil
}

// stackUnavailable is conservatively false on non-Unix platforms: the
// IPv6-disabled failure mode this guards against is a Unix concern, and
// treating an unknown error as "busy" fails closed.
func stackUnavailable(err error) bool {
	return false
}
