package port

import (
	"net"
	"testing"
)

func TestIsFree(t *testing.T) {
	t.Run("free port", func(t *testing.T) {
		// Grab a port the OS considers free, release it, then probe.
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		addr, ok := ln.Addr().(*net.TCPAddr)
		if !ok {
			t.Fatal("expected *net.TCPAddr")
		}
		p := addr.Port
		_ = ln.Close()
		if !IsFree(p) {
			t.Errorf("IsFree(%d) = false for a released port", p)
		}
	})

	t.Run("wildcard listener detected", func(t *testing.T) {
		// Services typically bind the wildcard (Node binds "::"). A probe on
		// 127.0.0.1 misses this on BSD/macOS; the wildcard probe must not.
		ln, err := net.Listen("tcp", ":0")
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = ln.Close() }()
		addr, ok := ln.Addr().(*net.TCPAddr)
		if !ok {
			t.Fatal("expected *net.TCPAddr")
		}
		p := addr.Port
		if IsFree(p) {
			t.Errorf("IsFree(%d) = true while a wildcard listener holds the port", p)
		}
	})

	t.Run("loopback listener detected", func(t *testing.T) {
		// Go listeners set SO_REUSEADDR, so a wildcard probe with default
		// options would coexist with a 127.0.0.1 bind on BSD/macOS. The probe
		// disables SO_REUSEADDR to conflict with specific-address listeners.
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = ln.Close() }()
		addr, ok := ln.Addr().(*net.TCPAddr)
		if !ok {
			t.Fatal("expected *net.TCPAddr")
		}
		p := addr.Port
		if IsFree(p) {
			t.Errorf("IsFree(%d) = true while a 127.0.0.1 listener holds the port", p)
		}
	})
}
