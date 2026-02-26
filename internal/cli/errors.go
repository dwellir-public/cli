package cli

import (
	"errors"
	"fmt"
	"strings"

	"github.com/dwellir-public/cli/internal/api"
)

func formatCommandError(err error) error {
	if err == nil {
		return nil
	}

	var apiErr *api.APIError
	if errors.As(err, &apiErr) {
		code := "api_error"
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
		message := fmt.Sprintf("Request failed with HTTP %d.", apiErr.StatusCode)
		help := strings.TrimSpace(apiErr.Body)
		return getFormatter().Error(code, message, help)
	}

	return getFormatter().Error("error", err.Error(), "")
}
