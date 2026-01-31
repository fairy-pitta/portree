package port

import (
	"fmt"
	"hash/fnv"
	"net"
	"strconv"

	"github.com/fairy-pitta/portree/internal/config"
)

// Allocate returns a port for the given branch and service using FNV32 hash.
// If fixedPort > 0, it is returned directly (after checking availability).
// Otherwise, hash-based allocation with linear probing is used.
func Allocate(branch, service string, svc config.ServiceConfig, fixedPort int, used map[int]bool) (int, error) {
	pr := svc.PortRange
	rangeSize := pr.Max - pr.Min + 1

	if fixedPort > 0 {
		if used[fixedPort] {
			return 0, fmt.Errorf("fixed port %d for %s/%s is already in use", fixedPort, branch, service)
		}
		return fixedPort, nil
	}

	base := hashPort(branch, service, pr.Min, pr.Max)

	for i := 0; i < rangeSize; i++ {
		candidate := pr.Min + (base-pr.Min+i)%rangeSize
		if !used[candidate] && isPortFree(candidate) {
			return candidate, nil
		}
	}

	return 0, fmt.Errorf("no available port in range [%d, %d] for %s/%s", pr.Min, pr.Max, branch, service)
}

// hashPort returns a port within [min, max] based on FNV32 of branch+service.
func hashPort(branch, service string, min, max int) int {
	h := fnv.New32a()
	h.Write([]byte(branch + ":" + service))
	rangeSize := max - min + 1
	return min + int(h.Sum32())%rangeSize
}

// isPortFree checks if a TCP port is available by attempting to listen on it.
// Note: there is an inherent TOCTOU (time-of-check-time-of-use) race between
// this check and the moment the child process actually binds the port. This is
// mitigated by (1) the file-level lock in state.FileStore serializing port
// allocation across concurrent portree invocations, and (2) a clear error
// message when the service fails to bind its assigned port.
func isPortFree(port int) bool {
	ln, err := net.Listen("tcp", ":"+strconv.Itoa(port))
	if err != nil {
		return false
	}
	ln.Close()
	return true
}
