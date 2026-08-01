package browser

import (
	"fmt"
	"os/exec"
	"runtime"
)

// BuildURL constructs the proxy URL for a service.
func BuildURL(scheme, slug string, proxyPort int) string {
	return fmt.Sprintf("%s://%s.localhost:%d", scheme, slug, proxyPort)
}

// commandFor builds the browser-launching command for the given GOOS. It is
// split out from Open so tests can assert on the arguments of every platform
// without actually launching a browser.
func commandFor(goos, url string) *exec.Cmd {
	switch goos {
	case "darwin":
		return exec.Command("open", url)
	case "windows":
		// On Windows, "start" requires an empty title argument before the URL.
		return exec.Command("cmd", "/c", "start", "", url)
	default:
		return exec.Command("xdg-open", url)
	}
}

// Open opens the given URL in the default browser.
func Open(url string) error {
	return commandFor(runtime.GOOS, url).Start()
}
