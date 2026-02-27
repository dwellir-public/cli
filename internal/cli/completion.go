package cli

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

var (
	completionInstall bool
	completionYes     bool
)

var completionCmd = &cobra.Command{
	Use:   "completion [bash|zsh|fish|powershell]",
	Short: "Generate or install shell completion script",
	Long: `Generate shell completion script or install it to a user-local path.

Examples:
  dwellir completion zsh
  dwellir completion zsh --install
  dwellir completion bash --install --yes`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		shell := strings.ToLower(strings.TrimSpace(args[0]))
		script, err := generateCompletionScript(shell)
		if err != nil {
			return getFormatter().Error(
				"validation_error",
				err.Error(),
				"Supported shells: bash, zsh, fish, powershell",
			)
		}

		if !completionInstall {
			_, writeErr := fmt.Fprint(cmd.OutOrStdout(), script)
			return writeErr
		}

		targetPath, sourceHint, err := completionInstallPath(shell)
		if err != nil {
			return formatCommandError(err)
		}

		if !completionYes {
			ok, confirmErr := confirmInstall(cmd, shell, targetPath)
			if confirmErr != nil {
				return formatCommandError(confirmErr)
			}
			if !ok {
				return getFormatter().Success("completion.install", map[string]string{
					"status": "cancelled",
				})
			}
		}

		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			return formatCommandError(err)
		}
		if err := os.WriteFile(targetPath, []byte(script), 0o644); err != nil {
			return formatCommandError(err)
		}

		return getFormatter().Success("completion.install", map[string]string{
			"status": "installed",
			"shell":  shell,
			"path":   targetPath,
			"hint":   sourceHint,
		})
	},
}

func init() {
	completionCmd.Flags().BoolVar(&completionInstall, "install", false, "Install completion script to a user-local path")
	completionCmd.Flags().BoolVarP(&completionYes, "yes", "y", false, "Skip confirmation prompt (non-interactive)")
	rootCmd.AddCommand(completionCmd)
}

func generateCompletionScript(shell string) (string, error) {
	var b bytes.Buffer
	switch shell {
	case "bash":
		if err := rootCmd.GenBashCompletion(&b); err != nil {
			return "", err
		}
	case "zsh":
		if err := rootCmd.GenZshCompletion(&b); err != nil {
			return "", err
		}
	case "fish":
		if err := rootCmd.GenFishCompletion(&b, true); err != nil {
			return "", err
		}
	case "powershell":
		if err := rootCmd.GenPowerShellCompletionWithDesc(&b); err != nil {
			return "", err
		}
	default:
		return "", fmt.Errorf("unsupported shell %q", shell)
	}
	return b.String(), nil
}

func completionInstallPath(shell string) (targetPath string, sourceHint string, err error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", "", err
	}

	switch shell {
	case "bash":
		targetPath = filepath.Join(home, ".local", "share", "bash-completion", "completions", "dwellir")
		sourceHint = "Restart your shell, or run: source ~/.local/share/bash-completion/completions/dwellir"
	case "zsh":
		targetPath = filepath.Join(home, ".zsh", "completions", "_dwellir")
		sourceHint = "Ensure ~/.zsh/completions is in $fpath, then run: autoload -U compinit && compinit"
	case "fish":
		targetPath = filepath.Join(home, ".config", "fish", "completions", "dwellir.fish")
		sourceHint = "Restart fish shell to load the completion."
	case "powershell":
		targetPath = filepath.Join(home, ".config", "powershell", "Completions", "dwellir.ps1")
		sourceHint = "Add the script to your PowerShell profile and reload the session."
	default:
		return "", "", fmt.Errorf("unsupported shell %q", shell)
	}

	return targetPath, sourceHint, nil
}

func confirmInstall(cmd *cobra.Command, shell, targetPath string) (bool, error) {
	_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Install %s completion to %s? [y/N]: ", shell, targetPath)
	reader := bufio.NewReader(cmd.InOrStdin())
	line, err := reader.ReadString('\n')
	if err != nil {
		return false, err
	}
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "y", "yes":
		return true, nil
	default:
		return false, nil
	}
}
