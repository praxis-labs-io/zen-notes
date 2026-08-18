package app

import (
	"fmt"
	"net/url"
	"os/exec"
	"runtime"
	"strings"
)

// validWebLink accepts only absolute web URLs. Other schemes stay inert even
// if the operating system has registered a handler for them.
func validWebLink(target string) error {
	u, err := url.Parse(target)
	if err != nil || u.Host == "" || (!strings.EqualFold(u.Scheme, "http") && !strings.EqualFold(u.Scheme, "https")) {
		return fmt.Errorf("not an HTTP or HTTPS link")
	}
	return nil
}

func systemOpenLink(target string) error {
	cmd, err := linkCommand(runtime.GOOS, target)
	if err != nil {
		return err
	}
	return cmd.Run()
}

// linkCommand uses argument vectors rather than a shell, so link text can
// never be interpreted as a command.
func linkCommand(goos, target string) (*exec.Cmd, error) {
	switch goos {
	case "darwin":
		return exec.Command("open", target), nil
	case "linux":
		return exec.Command("xdg-open", target), nil
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", target), nil
	default:
		return nil, fmt.Errorf("opening links is not supported on %s", goos)
	}
}
