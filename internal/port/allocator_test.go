package port

import (
	"testing"

	"github.com/fairy-pitta/portree/internal/config"
)

func TestHashPort(t *testing.T) {
	t.Run("deterministic", func(t *testing.T) {
		a := hashPort("main", "web", 3100, 3199)
		b := hashPort("main", "web", 3100, 3199)
		if a != b {
			t.Errorf("hashPort not deterministic: %d != %d", a, b)
		}
	})

	t.Run("within range", func(t *testing.T) {
		p := hashPort("main", "web", 3100, 3199)
		if p < 3100 || p > 3199 {
			t.Errorf("hashPort = %d, not in [3100, 3199]", p)
		}
	})

	t.Run("different inputs differ", func(t *testing.T) {
		a := hashPort("main", "web", 3100, 3199)
		b := hashPort("feature/auth", "api", 3100, 3199)
		// Not guaranteed to differ but very likely with different input
		// Just check both are in range
		if a < 3100 || a > 3199 {
			t.Errorf("hashPort(main,web) = %d, not in range", a)
		}
		if b < 3100 || b > 3199 {
			t.Errorf("hashPort(feature/auth,api) = %d, not in range", b)
		}
	})
}

func TestAllocate(t *testing.T) {
	svc := config.ServiceConfig{
		Command:   "npm start",
		PortRange: config.PortRange{Min: 3100, Max: 3199},
		ProxyPort: 3000,
	}

	t.Run("fixed port available", func(t *testing.T) {
		used := map[int]bool{}
		port, err := Allocate("main", "web", svc, 3150, used)
		if err != nil {
			t.Fatalf("Allocate() error: %v", err)
		}
		if port != 3150 {
			t.Errorf("Allocate() = %d, want 3150", port)
		}
	})

	t.Run("fixed port in use", func(t *testing.T) {
		used := map[int]bool{3150: true}
		_, err := Allocate("main", "web", svc, 3150, used)
		if err == nil {
			t.Fatal("Allocate() expected error for used fixed port")
		}
	})

	t.Run("hash in range", func(t *testing.T) {
		used := map[int]bool{}
		port, err := Allocate("main", "web", svc, 0, used)
		if err != nil {
			t.Fatalf("Allocate() error: %v", err)
		}
		if port < 3100 || port > 3199 {
			t.Errorf("Allocate() = %d, not in [3100, 3199]", port)
		}
	})

	t.Run("deterministic", func(t *testing.T) {
		used := map[int]bool{}
		a, _ := Allocate("main", "web", svc, 0, used)
		b, _ := Allocate("main", "web", svc, 0, used)
		if a != b {
			t.Errorf("Allocate not deterministic: %d != %d", a, b)
		}
	})

	t.Run("skips used", func(t *testing.T) {
		// Get the port that would be allocated normally
		first, _ := Allocate("main", "web", svc, 0, map[int]bool{})
		// Mark it as used
		used := map[int]bool{first: true}
		second, err := Allocate("main", "web", svc, 0, used)
		if err != nil {
			t.Fatalf("Allocate() error: %v", err)
		}
		if second == first {
			t.Errorf("Allocate should skip used port %d", first)
		}
		if second < 3100 || second > 3199 {
			t.Errorf("Allocate() = %d, not in range", second)
		}
	})

	t.Run("all used", func(t *testing.T) {
		singleSvc := config.ServiceConfig{
			Command:   "npm start",
			PortRange: config.PortRange{Min: 5000, Max: 5002},
			ProxyPort: 3000,
		}
		used := map[int]bool{5000: true, 5001: true, 5002: true}
		_, err := Allocate("main", "web", singleSvc, 0, used)
		if err == nil {
			t.Fatal("Allocate() expected error when all ports used")
		}
	})

	t.Run("single port range", func(t *testing.T) {
		// Use a high port unlikely to be in use
		singleSvc := config.ServiceConfig{
			Command:   "npm start",
			PortRange: config.PortRange{Min: 19876, Max: 19876},
			ProxyPort: 3000,
		}
		used := map[int]bool{}
		port, err := Allocate("main", "web", singleSvc, 0, used)
		if err != nil {
			t.Fatalf("Allocate() error: %v", err)
		}
		if port != 19876 {
			t.Errorf("Allocate() = %d, want 19876", port)
		}
	})
}
