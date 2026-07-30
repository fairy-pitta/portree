package port

import (
	"context"
	"net"
	"strconv"
)

// IsFree reports whether a TCP port is available for a service to bind.
//
// The probe binds the wildcard address on both stacks with SO_REUSEADDR
// disabled. Both details matter on BSD/macOS:
//   - A 127.0.0.1 probe succeeds even while another process holds the
//     wildcard (e.g. a Node server on "::"), making busy ports look free.
//   - Go enables SO_REUSEADDR on listeners by default, which lets wildcard
//     and specific-address binds coexist, so even a wildcard probe misses
//     specific-address listeners unless SO_REUSEADDR is turned off.
//
// A stack that is entirely absent (e.g. IPv6 disabled via
// net.ipv6.conf.all.disable_ipv6=1, or an IPv6-less container) must not make
// a free port look busy: binding tcp6 there fails with EAFNOSUPPORT /
// EADDRNOTAVAIL for every port. Such errors are treated as "this stack isn't
// here, skip it"; only a real conflict (EADDRINUSE) or permission error
// (EACCES) marks the port unavailable. At least one stack must be probeable —
// if neither binds, IsFree fails closed rather than claiming the port is free.
//
// There is still an inherent TOCTOU race between this check and the moment
// the child process binds the port. This is mitigated by (1) the file-level
// lock in state.FileStore serializing port allocation across concurrent
// portree invocations, and (2) early-exit detection when a service dies
// shortly after starting.
func IsFree(port int) bool {
	lc := net.ListenConfig{Control: disableReuseAddr}
	addr := ":" + strconv.Itoa(port)
	bound := 0
	for _, network := range []string{"tcp4", "tcp6"} {
		ln, err := lc.Listen(context.Background(), network, addr)
		if err != nil {
			if stackUnavailable(err) {
				continue
			}
			return false
		}
		_ = ln.Close()
		bound++
	}
	return bound > 0
}
