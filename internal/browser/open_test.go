package browser

import (
	"testing"
)

func TestBuildURL(t *testing.T) {
	t.Run("standard", func(t *testing.T) {
		got := BuildURL("feature-auth", 3000)
		want := "http://feature-auth.localhost:3000"
		if got != want {
			t.Errorf("BuildURL() = %q, want %q", got, want)
		}
	})

	t.Run("main branch", func(t *testing.T) {
		got := BuildURL("main", 8000)
		want := "http://main.localhost:8000"
		if got != want {
			t.Errorf("BuildURL() = %q, want %q", got, want)
		}
	})
}

func TestOpenDoesNotPanic(t *testing.T) {
	// Smoke test: calling Open with an invalid URL should not panic.
	// We don't check the error since the browser command may or may not
	// be available in the test environment.
	_ = Open("http://localhost:0/test")
}
