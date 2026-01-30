package browser

import (
	"fmt"
	"os/exec"
	"runtime"
)

// BuildURL constructs the proxy URL for a service.
func BuildURL(slug string, proxyPort int) string {
	return fmt.Sprintf("http://%s.localhost:%d", slug, proxyPort)
}

// Open opens the given URL in the default browser.
func Open(url string) error {
	cmd := openCommand()
	return exec.Command(cmd, url).Start()
}

func openCommand() string {
	switch runtime.GOOS {
	case "darwin":
		return "open"
	case "windows":
		return "rundll32"
	default:
		return "xdg-open"
	}
}
