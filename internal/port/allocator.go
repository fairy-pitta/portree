package port

import (
	"fmt"
	"hash/fnv"

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
		if !used[candidate] && IsFree(candidate) {
			return candidate, nil
		}
	}

	return 0, fmt.Errorf("no available port in range [%d, %d] for %s/%s; widen port_range for service %q in %s",
		pr.Min, pr.Max, branch, service, service, config.FileName)
}

// hashPort returns a port within [minPort, maxPort] based on FNV32 of branch+service.
func hashPort(branch, service string, minPort, maxPort int) int {
	h := fnv.New32a()
	h.Write([]byte(branch + ":" + service))
	rangeSize := maxPort - minPort + 1
	return minPort + int(h.Sum32())%rangeSize
}
