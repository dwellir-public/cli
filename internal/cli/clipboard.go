package cli

import (
	"errors"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

type clipboardTool struct {
	name string
	args []string
}

var errNoClipboard = errors.New("no clipboard tool found")

// clipboardTools lists the copy commands to try, most specific first. Linux
// support depends on the display server, not the distribution: Wayland uses
// wl-copy, X11 uses xclip or xsel, WSL uses clip.exe and Termux uses
// termux-clipboard-set.
func clipboardTools(goos string, getenv func(string) string) []clipboardTool {
	switch goos {
	case "darwin":
		return []clipboardTool{{name: "pbcopy"}}
	case "windows":
		return []clipboardTool{{name: "clip.exe"}}
	}
	var tools []clipboardTool
	if getenv("WAYLAND_DISPLAY") != "" {
		tools = append(tools, clipboardTool{name: "wl-copy"})
	}
	if getenv("DISPLAY") != "" {
		tools = append(tools,
			clipboardTool{name: "xclip", args: []string{"-selection", "clipboard"}},
			clipboardTool{name: "xsel", args: []string{"--clipboard", "--input"}},
		)
	}
	return append(tools, clipboardTool{name: "clip.exe"}, clipboardTool{name: "termux-clipboard-set"})
}

// copyToClipboard sends value on stdin, never as an argument, because other
// users can read process arguments. It returns the tool that took the value.
func copyToClipboard(value string) (string, error) {
	err := errNoClipboard
	for _, tool := range clipboardTools(runtime.GOOS, os.Getenv) {
		path, lookErr := exec.LookPath(tool.name)
		if lookErr != nil {
			continue
		}
		cmd := exec.Command(path, tool.args...)
		cmd.Stdin = strings.NewReader(value)
		// Leave stdout and stderr unset. wl-copy and xclip keep a background
		// process alive to serve the clipboard, and a pipe would block Run
		// until that process exits.
		if err = cmd.Run(); err == nil {
			return tool.name, nil
		}
	}
	return "", err
}

const clipboardHelp = "Install wl-clipboard (Wayland), xclip or xsel (X11), or termux-api (Termux). " +
	"Over SSH or in CI, use 'dwellir keys env <name>' to write the key to .env instead."
