package browser

import (
	"runtime"
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

func TestOpenCommand(t *testing.T) {
	cmd := openCommand()
	switch runtime.GOOS {
	case "darwin":
		if cmd != "open" {
			t.Errorf("openCommand() on darwin = %q, want %q", cmd, "open")
		}
	case "windows":
		if cmd != "rundll32" {
			t.Errorf("openCommand() on windows = %q, want %q", cmd, "rundll32")
		}
	default:
		if cmd != "xdg-open" {
			t.Errorf("openCommand() on %s = %q, want %q", runtime.GOOS, cmd, "xdg-open")
		}
	}
}
