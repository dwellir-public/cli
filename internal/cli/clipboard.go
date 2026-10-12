package cli

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

type clipboardTool struct {
	name string
	args []string
}

var errNoClipboard = errors.New("no clipboard tool found")

// clipboardTools lists the copy commands to try, most specific first. Linux
// support depends on the display server, not the distribution: Wayland uses
// wl-copy, X11 uses xclip or xsel, WSL uses clip.exe and Termux uses
// termux-clipboard-set. WSL prefers clip.exe, because a WSLg display server
// copies only into the Linux side.
func clipboardTools(goos string, getenv func(string) string) []clipboardTool {
	switch goos {
	case "darwin":
		return []clipboardTool{{name: "pbcopy"}}
	case "windows":
		return []clipboardTool{{name: "clip.exe"}}
	}
	windowsClipboard := clipboardTool{name: "clip.exe"}
	wsl := getenv("WSL_DISTRO_NAME") != "" || getenv("WSL_INTEROP") != ""
	var tools []clipboardTool
	if wsl {
		tools = append(tools, windowsClipboard)
	}
	if getenv("WAYLAND_DISPLAY") != "" {
		tools = append(tools, clipboardTool{name: "wl-copy"})
	}
	if getenv("DISPLAY") != "" {
		tools = append(tools,
			clipboardTool{name: "xclip", args: []string{"-selection", "clipboard"}},
			clipboardTool{name: "xsel", args: []string{"--clipboard", "--input"}},
		)
	}
	if !wsl {
		tools = append(tools, windowsClipboard)
	}
	return append(tools, clipboardTool{name: "termux-clipboard-set"})
}

// clipboardTimeout bounds each tool, so a stalled display server cannot
// block the command or the next tool.
const clipboardTimeout = 5 * time.Second

// copyToClipboard sends value on stdin, never as an argument, because other
// users can read process arguments. It returns the tool that took the value.
func copyToClipboard(value string) (string, error) {
	err := errNoClipboard
	for _, tool := range clipboardTools(runtime.GOOS, os.Getenv) {
		path, lookErr := exec.LookPath(tool.name)
		if lookErr != nil {
			continue
		}
		ctx, cancel := context.WithTimeout(context.Background(), clipboardTimeout)
		cmd := exec.CommandContext(ctx, path, tool.args...)
		cmd.Stdin = strings.NewReader(value)
		cmd.WaitDelay = time.Second
		// Leave stdout and stderr unset. wl-copy and xclip keep a background
		// process alive to serve the clipboard, and a pipe would block Run
		// until that process exits.
		err = cmd.Run()
		cancel()
		if err == nil {
			return tool.name, nil
		}
	}
	return "", err
}

const clipboardHelp = "Install wl-clipboard (Wayland), xclip or xsel (X11), or termux-api (Termux). " +
	"Over SSH or in CI, use 'dwellir keys env <name>' to write the key to .env instead."
