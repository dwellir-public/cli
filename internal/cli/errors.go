package cli

import (
	"errors"
	"fmt"
	"strings"

	"github.com/dwellir-public/cli/internal/api"
	"github.com/dwellir-public/cli/internal/output"
)

func formatCommandError(err error) error {
	if err == nil {
		return nil
	}
	return getFormatter().Error(commandErrorParts(err))
}

// commandErrorParts classifies an API or network error into the error code,
// message and help that formatCommandError renders.
func commandErrorParts(err error) (code, message, help string) {
	var apiErr *api.APIError
	if errors.As(err, &apiErr) {
		code = "api_error"
		switch apiErr.StatusCode {
		case 401, 403:
			code = "forbidden"
		case 404:
			code = "not_found"
		case 422:
			code = "validation_error"
		case 429:
			code = "rate_limited"
		}
		message = fmt.Sprintf("Request failed with HTTP %d.", apiErr.StatusCode)
		help = output.DefaultHelp(code)
		if body := strings.TrimSpace(apiErr.Body); body != "" {
			help = "API response: " + body + "\n" + help
		}
		return code, message, help
	}

	if strings.HasPrefix(err.Error(), "request failed") {
		return "network_error", err.Error(),
			"Check the network connection and DWELLIR_API_URL, then retry. Run 'dwellir doctor --verify-api' to test the API."
	}
	return "error", err.Error(), ""
}
