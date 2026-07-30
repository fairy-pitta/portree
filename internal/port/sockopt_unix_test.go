//go:build unix

package port

import (
	"net"
	"os"
	"syscall"
	"testing"
)

func TestStackUnavailable(t *testing.T) {
	// Absent-stack errors (e.g. IPv6 disabled) must be tolerated so a free
	// port is not misreported as busy.
	if !stackUnavailable(syscall.EAFNOSUPPORT) {
		t.Error("EAFNOSUPPORT should be treated as stack-unavailable")
	}
	if !stackUnavailable(syscall.EADDRNOTAVAIL) {
		t.Error("EADDRNOTAVAIL should be treated as stack-unavailable")
	}

	// net.Listen returns these wrapped in *net.OpError -> *os.SyscallError;
	// errors.Is must still unwrap to the errno.
	wrapped := &net.OpError{Op: "listen", Net: "tcp6", Err: os.NewSyscallError("bind", syscall.EAFNOSUPPORT)}
	if !stackUnavailable(wrapped) {
		t.Error("wrapped EAFNOSUPPORT (as net.Listen returns) should be stack-unavailable")
	}

	// A busy port (EADDRINUSE) or permission error is NOT stack-unavailable —
	// it means the port really can't be bound.
	if stackUnavailable(syscall.EADDRINUSE) {
		t.Error("EADDRINUSE means busy, not stack-unavailable")
	}
	if stackUnavailable(syscall.EACCES) {
		t.Error("EACCES means permission denied, not stack-unavailable")
	}
}
