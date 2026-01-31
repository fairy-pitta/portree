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
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", url).Start()
	case "windows":
		// On Windows, "start" requires an empty title argument before the URL.
		return exec.Command("cmd", "/c", "start", "", url).Start()
	default:
		return exec.Command("xdg-open", url).Start()
	}
}
