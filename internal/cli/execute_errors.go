package cli

import "strings"

func classifyExecutionError(err error) (code, message, help string) {
	raw := strings.TrimSpace(err.Error())
	if raw == "" {
		return "error", "Command failed.", "Run `dwellir --help` to view available commands."
	}

	if strings.Contains(raw, `unknown command "get" for "dwellir"`) {
		return "validation_error",
			"Unknown command `dwellir get`.",
			"Use endpoint subcommands instead.\nExample: dwellir endpoints get base\nTip: dwellir endpoints search <query>"
	}

	if strings.HasPrefix(raw, "unknown command ") {
		return "validation_error", raw, "Run `dwellir --help` to view available commands."
	}

	if strings.Contains(raw, "accepts") && strings.Contains(raw, "arg(s), received 0") {
		return "validation_error", "Missing required arguments.", raw + "\nRun the command with --help to see examples."
	}

	if strings.Contains(raw, "missing required argument") {
		return "validation_error", raw, ""
	}

	return "error", raw, "Run `dwellir --help` for usage."
}
