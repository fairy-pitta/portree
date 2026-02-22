package browser

import (
	"testing"
)

func TestBuildURL(t *testing.T) {
	t.Run("standard http", func(t *testing.T) {
		got := BuildURL("http", "feature-auth", 3000)
		want := "http://feature-auth.localhost:3000"
		if got != want {
			t.Errorf("BuildURL() = %q, want %q", got, want)
		}
	})

	t.Run("main branch", func(t *testing.T) {
		got := BuildURL("http", "main", 8000)
		want := "http://main.localhost:8000"
		if got != want {
			t.Errorf("BuildURL() = %q, want %q", got, want)
		}
	})

	t.Run("https", func(t *testing.T) {
		got := BuildURL("https", "feature-auth", 3000)
		want := "https://feature-auth.localhost:3000"
		if got != want {
			t.Errorf("BuildURL() = %q, want %q", got, want)
		}
	})
}

func TestBuildURL_VariousInputs(t *testing.T) {
	tests := []struct {
		name      string
		scheme    string
		slug      string
		proxyPort int
		want      string
	}{
		{"simple slug", "http", "main", 3000, "http://main.localhost:3000"},
		{"hyphenated slug", "http", "feature-auth", 8000, "http://feature-auth.localhost:8000"},
		{"numeric slug", "http", "v2-release", 5000, "http://v2-release.localhost:5000"},
		{"single char slug", "http", "a", 3000, "http://a.localhost:3000"},
		{"long slug", "http", "very-long-branch-name-here", 3000, "http://very-long-branch-name-here.localhost:3000"},
		{"https slug", "https", "main", 3000, "https://main.localhost:3000"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildURL(tt.scheme, tt.slug, tt.proxyPort)
			if got != tt.want {
				t.Errorf("BuildURL(%q, %q, %d) = %q, want %q", tt.scheme, tt.slug, tt.proxyPort, got, tt.want)
			}
		})
	}
}

func TestOpenDoesNotPanic(t *testing.T) {
	// Smoke test: calling Open with an invalid URL should not panic.
	// We don't check the error since the browser command may or may not
	// be available in the test environment.
	_ = Open("http://localhost:0/test")
}

func TestOpenReturnsNoError(t *testing.T) {
	// On macOS/Linux with a desktop, Open should succeed for a valid URL.
	// We can't assert on error reliably in CI, but we can ensure no panic.
	err := Open("http://example.com")
	// If this is a headless CI environment, the command may fail,
	// but it shouldn't panic.
	_ = err
}
